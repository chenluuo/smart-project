"""ingest-service：文档向量化（消费 queue:doc.process）。

流程：收到通知（Go 文档上传/变更 → agent 入队）
  → 服务账号登录拿 JWT → 拉 GET /knowledge/docs（可用文档清单，含 downloadUrl）
  → 与 Milvus 已有向量对比（doc_id + version）
  → 缺失/版本变更 → downloadUrl 拉原文 → 切片 → embedding → 写知识 collection（幂等）
"""
from __future__ import annotations

import threading
import time
from typing import Any

from shared.config import get_config
from shared.embedding import embed
from shared.go_client import get_go_client
from shared.milvus_client import ensure_collections, upsert_documents

_token_lock = threading.Lock()
_cached_token: str = ""
_token_fetched_at = 0.0
_TOKEN_TTL = 1800  # 服务账号 JWT 缓存时长（秒）


def _service_token() -> str:
    """服务账号登录拿 JWT（缓存 TTL 内复用；Go /knowledge/docs 需要 JWT）。"""
    global _cached_token, _token_fetched_at
    now = time.time()
    with _token_lock:
        if _cached_token and now - _token_fetched_at < _TOKEN_TTL:
            return _cached_token
        cfg = get_config("ingest").get("service_account") or {}
        username, password = cfg.get("username"), cfg.get("password")
        if not username or not password:
            raise RuntimeError("ingest 未配置 service_account（config.yaml ingest.service_account）")
        _cached_token = get_go_client().login(username, password)
        _token_fetched_at = now
        return _cached_token


def _fetch(url: str) -> str:
    """按签名 URL 拉取文档原文（downloadUrl 已带 MinIO 签名）。"""
    import httpx

    with httpx.Client(timeout=60, trust_env=False) as client:
        resp = client.get(url)
        resp.raise_for_status()
        return resp.text


def _chunk_text(text: str, size: int, overlap: int) -> list[str]:
    """简单字符切片（生产可换语义切片器）。"""
    if len(text) <= size:
        return [text] if text.strip() else []
    chunks: list[str] = []
    start = 0
    while start < len(text):
        chunks.append(text[start : start + size])
        start += size - overlap
    return chunks


def process_doc_event(event: dict) -> dict:
    """处理一条文档加工事件。返回 {doc_id, version, chunks, status}。"""
    cfg = get_config("ingest")
    chunk_size = int(cfg.get("chunk_size", 800))
    chunk_overlap = int(cfg.get("chunk_overlap", 100))

    ensure_collections()

    doc_id = str(event.get("doc_id") or "")  # Go notify 入队的是数字 docId，Milvus doc_id 为 varchar
    # 服务账号拉可用文档清单，找到目标文档（含 downloadUrl/version）
    docs = get_go_client().get_knowledge_docs(authorization="Bearer " + _service_token())
    doc = next((d for d in docs if str(d.get("id")) == str(doc_id)), None)
    if doc is None:
        return {"doc_id": doc_id, "status": "skipped", "reason": "文档不可用或不存在"}

    download_url = doc.get("downloadUrl")
    if not download_url:
        return {"doc_id": doc_id, "status": "skipped", "reason": "缺少 downloadUrl"}

    # 拉原文（downloadUrl 为 MinIO 签名 URL，直连拉取）
    try:
        text = _fetch(download_url)
    except Exception as e:
        return {"doc_id": doc_id, "status": "failed", "reason": f"拉取原文失败: {e}"}

    # 切片 + embedding + 写入（doc_id + version 幂等）
    chunks = _chunk_text(text, chunk_size, chunk_overlap)
    if not chunks:
        return {"doc_id": doc_id, "status": "skipped", "reason": "无有效内容"}

    vectors = embed(chunks)
    rows = []
    for i, (chunk, vec) in enumerate(zip(chunks, vectors)):
        rows.append({
            "id": f"{doc_id}:v{doc.get('version', 1)}:{i}",
            "vector": vec,
            "doc_id": doc_id,
            "title": doc.get("title"),
            "version": doc.get("version"),
            "category": doc.get("category"),
            "source": doc.get("source") or download_url,
            "content": chunk,
        })
    upsert_documents(rows, kind="knowledge")
    return {"doc_id": doc_id, "version": doc.get("version"), "chunks": len(chunks), "status": "ok"}

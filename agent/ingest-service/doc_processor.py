"""ingest-service：文档向量化（消费 queue:doc.process）。

流程：收到通知（Go 文档上传/变更 → agent 入队）
  → 服务账号登录拿 JWT → 拉 GET /knowledge/docs（可用文档清单，含 downloadUrl）
  → 与 Milvus 已有向量对比（doc_id + version）
  → 缺失/版本变更 → downloadUrl 拉原文（二进制）
      → docling-serve 解析成 Markdown（PDF/DOCX/XLSX/PPTX/图片；纯文本降级 resp.text）
      → 切片 → embedding → 写知识 collection（幂等）
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


def _download(url: str) -> tuple[bytes, str]:
    """按签名 URL 下载文档原文（二进制 + content-type）。"""
    import httpx

    with httpx.Client(timeout=60, trust_env=False) as client:
        resp = client.get(url)
        resp.raise_for_status()
        return resp.content, resp.headers.get("content-type", "")


def _parse_with_docling(content: bytes, filename: str) -> str:
    """调 docling-serve（/v1/convert/file）把文档解析成 Markdown 文本。

    返回空串表示解析无可提取文本（非异常）；HTTP 失败抛异常由调用方降级。
    """
    import httpx

    cfg = get_config("docling")
    url = f"http://{cfg.get('host', 'localhost')}:{cfg.get('port', 5001)}/v1/convert/file"
    headers = {}
    api_key = cfg.get("api_key")
    if api_key:
        headers["X-Api-Key"] = api_key
    timeout = int(cfg.get("timeout_seconds", 180))
    files = {"files": (filename or "document", content, "application/octet-stream")}
    data = {"to_formats": ["md"]}  # 只要 Markdown（含表格）
    with httpx.Client(timeout=timeout, trust_env=False) as client:
        resp = client.post(url, files=files, data=data, headers=headers)
        resp.raise_for_status()
        payload = resp.json()
    # 新版 docling-server：document.{md_content,text_content}；旧版：顶层 markdown
    doc = payload.get("document") or {}
    md = (doc.get("md_content") or payload.get("markdown") or "").strip()
    if not md:  # 都没有 markdown 时兜底纯文本
        md = (doc.get("text_content") or "").strip()
    return md


def _extract_text(url: str, filename: str, content_type: str) -> str:
    """优先 docling 解析（支持 PDF/Office/图片）；纯文本格式降级直读。"""
    try:
        md = _parse_with_docling(_download(url)[0], filename)
        if md:
            return md
        # docling 返回空：说明文件本身无文本（如纯图片无 OCR），走降级
        raise ValueError("docling 未提取到文本")
    except Exception:
        # 纯文本（txt/md/csv 等）降级为直读原文；二进制格式解析失败则抛错
        from urllib.parse import urlparse

        url_ext = (urlparse(url).path or "").lower()
        is_text = (
            content_type.startswith("text/")
            or url_ext.endswith((".txt", ".md", ".markdown", ".csv"))
            or filename.lower().endswith((".txt", ".md", ".markdown", ".csv"))
        )
        if is_text:
            import httpx

            with httpx.Client(timeout=60, trust_env=False) as client:
                return client.get(url).text
        raise


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
        # 文档不在可用（ACTIVE）清单：未发布 / 已删除 / 已归档 → 清理该文档历史向量（幂等）。
        # 同一通道同时服务上传通知与删除通知：上传 DRAFT 时无向量、删除幂等，安全。
        try:
            from shared.milvus_client import delete_documents

            delete_documents(doc_id)
        except Exception as e:
            return {"doc_id": doc_id, "status": "failed", "reason": f"清理向量失败: {e}"}
        return {"doc_id": doc_id, "status": "deleted", "reason": "文档不可用或不存在，已清理向量"}

    download_url = doc.get("downloadUrl")
    if not download_url:
        return {"doc_id": doc_id, "status": "skipped", "reason": "缺少 downloadUrl"}

    # 拉原文（二进制）→ docling 解析成 Markdown（纯文本降级）
    try:
        content, content_type = _download(download_url)
        text = _extract_text(download_url, doc.get("title") or f"{doc_id}.bin", content_type)
    except Exception as e:
        return {"doc_id": doc_id, "status": "failed", "reason": f"拉取/解析原文失败: {e}"}

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

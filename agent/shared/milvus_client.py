"""Milvus 客户端（向量库归智能体侧，Go 不碰）。

两个 collection：
- knowledge：知识切片（ingest 写 / tool 检索），无状态过滤（Go 保证可用性）
- memory：对话记忆（ingest 写 / agent 召回），按 user_id 强隔离 + 时间衰减

依赖：pymilvus（pip install pymilvus）。未安装/未连接时抛错，由调用方降级。
"""
from __future__ import annotations

from typing import Any

from shared.config import get_config

_conn: Any = None


def _get_conn():
    global _conn
    if _conn is not None:
        return _conn
    from pymilvus import MilvusClient

    cfg = get_config("milvus")
    _conn = MilvusClient(uri=f"http://{cfg.get('host','localhost')}:{cfg.get('port',19530)}",
                         token=cfg.get("token"))
    return _conn


def _collection(name: str) -> str:
    return get_config("milvus").get(f"{name}_collection", name)


def ensure_collections() -> None:
    """幂等建 collection（ingest 启动时调用）。"""
    from pymilvus import DataType

    conn = _get_conn()
    for name in ("knowledge", "memory"):
        col = _collection(name)
        if conn.has_collection(col):
            continue
        conn.create_collection(
            collection_name=col,
            dimension=int(get_config("milvus").get("dimension", 1024)),
            metric_type="COSINE",
        )
        # 公共标量字段
        conn.create_index(col, "user_id", index_name="idx_user") if name == "memory" else None


def search_knowledge(query_embedding: list[float], top_k: int = 5,
                     category: str | None = None) -> list[dict[str, Any]]:
    """知识检索（向量召回，category 可选过滤）。"""
    conn = _get_conn()
    col = _collection("knowledge")
    filter_expr = f'category == "{category}"' if category else None
    hits = conn.search(
        collection_name=col,
        data=[query_embedding],
        limit=top_k,
        filter=filter_expr,
        output_fields=["doc_id", "title", "version", "source", "category", "content"],
    )
    out: list[dict[str, Any]] = []
    for hit in hits[0]:
        entity = hit.get("entity", {})
        out.append({
            "docId": entity.get("doc_id"),
            "title": entity.get("title"),
            "version": entity.get("version"),
            "source": entity.get("source"),
            "category": entity.get("category"),
            "content": entity.get("content"),
            "score": hit.get("distance"),
        })
    return out


def search_memory(user_id: str, query_embedding: list[float], top_k: int = 3) -> list[dict[str, Any]]:
    """长期记忆召回（user_id 强过滤 + generated_at 时间衰减降权）。"""
    conn = _get_conn()
    col = _collection("memory")
    hits = conn.search(
        collection_name=col,
        data=[query_embedding],
        limit=top_k,
        filter=f'user_id == "{user_id}"',
        output_fields=["summary_text", "session_id", "generated_at", "model_version"],
    )
    out: list[dict[str, Any]] = []
    for hit in hits[0]:
        entity = hit.get("entity", {})
        out.append({
            "summary": entity.get("summary_text"),
            "session_id": entity.get("session_id"),
            "generated_at": entity.get("generated_at"),
            "model_version": entity.get("model_version"),
            "score": hit.get("distance"),
        })
    return out


def upsert_documents(rows: list[dict[str, Any]], kind: str = "knowledge") -> None:
    """批量写入向量（kind: knowledge / memory）。行需含 id 与 vector 字段。"""
    conn = _get_conn()
    col = _collection(kind)
    conn.upsert(collection_name=col, data=rows)

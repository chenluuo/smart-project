"""共享：embedding 封装（OpenAI 兼容，可配置；DeepSeek 无 embedding，需配兼容端点或本地模型）。"""
from __future__ import annotations

from typing import Any

from shared.config import get_config

_client: Any = None


def _get_client():
    global _client
    if _client is not None:
        return _client
    from openai import OpenAI

    cfg = get_config("embedding")
    _client = OpenAI(
        base_url=cfg.get("base_url", "http://localhost:9997/v1"),
        api_key=cfg.get("api_key"),
    )
    return _client


def embed(texts: list[str]) -> list[list[float]]:
    """批量文本向量化。"""
    cfg = get_config("embedding")
    client = _get_client()
    resp = client.embeddings.create(model=cfg.get("model", "bge-m3"), input=texts)
    return [d.embedding for d in resp.data]

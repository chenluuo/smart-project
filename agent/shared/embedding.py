"""火山方舟多模态 embedding 封装（doubao-embedding-vision-250615）。

接口：POST /api/v3/embeddings/multimodal（OpenAI 兼容端口下的专用路径）
请求：{"model": "<ep-xxx>", "input": [{"type": "text", "text": "..."}]}
响应：{"data": {"embedding": [...]}}（每次调用产出一个向量，维度 2048）
实现：逐条调用，保证每条文本获得独立向量（multimodal 接口不支持一次多向量返回）。
"""
from __future__ import annotations

from typing import Any

import httpx

from shared.config import get_config

_client: httpx.Client | None = None


def _get_client() -> httpx.Client:
    global _client
    if _client is None:
        _client = httpx.Client(timeout=60)
    return _client


def embed(texts: list[str]) -> list[list[float]]:
    """批量文本向量化（逐条调用 multimodal 接口）。"""
    cfg = get_config("embedding")
    url = cfg.get("base_url", "https://ark.cn-beijing.volces.com/api/v3").rstrip("/") + "/embeddings/multimodal"
    headers = {
        "Authorization": f"Bearer {cfg.get('api_key')}",
        "Content-Type": "application/json",
    }
    model = cfg.get("model", "ep-20260621064553-q2m69")
    dimensions = cfg.get("dimensions")  # 可选：自定义输出维度（如 1024）

    out: list[list[float]] = []
    client = _get_client()
    for text in texts:
        payload: dict[str, Any] = {
            "model": model,
            "input": [{"type": "text", "text": text}],
        }
        if dimensions:
            payload["dimensions"] = int(dimensions)
        resp = client.post(url, headers=headers, json=payload)
        resp.raise_for_status()
        vec = resp.json()["data"]["embedding"]
        out.append(vec)
    return out

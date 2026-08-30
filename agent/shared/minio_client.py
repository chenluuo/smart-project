"""MinIO 客户端（知识文档对象写入；原文读取由 ingest 走签名 URL 下载）。"""
from __future__ import annotations

from io import BytesIO
from typing import Any

from shared.config import get_config

_client: Any = None


def _get_client():
    global _client
    if _client is not None:
        return _client
    from minio import Minio

    cfg = get_config("minio")
    _client = Minio(
        cfg.get("endpoint", "localhost:9000"),
        access_key=cfg.get("access_key", "minioadmin"),
        secret_key=cfg.get("secret_key"),
        secure=bool(cfg.get("secure", False)),
    )
    return _client


def put_bytes(object_key: str, data: bytes, content_type: str = "application/octet-stream") -> None:
    client = _get_client()
    bucket = get_config("minio").get("bucket", "knowledge")
    client.put_object(bucket, object_key, BytesIO(data), length=len(data), content_type=content_type)

"""docling-serve 客户端（文档 → Markdown 文本，共享给 ingest / tool 两处使用）。

服务：hwdsl2/docling-server（compose 内 docling 服务，端口 5001）。
接口：POST /v1/convert/file（multipart：files + to_formats=md），X-Api-Key 认证。
响应（新版）：{"document": {"md_content": "...", "text_content": "..."}, "status": "success"}
兼容旧版顶层 "markdown" 字段。

纯文本（txt/md/csv）自动降级直读原文，不依赖 docling。
"""
from __future__ import annotations

import httpx

from shared.config import get_config


def parse_bytes(content: bytes, filename: str) -> str:
    """调 docling-serve 把文档字节解析成 Markdown 文本。

    - 返回解析后的文本（可能为空串：文件无文本内容）；
    - HTTP/网络失败抛 httpx 异常，由调用方决定降级；
    - 纯文本格式（txt/md/csv）不调 docling，直接 decode 返回。
    """
    if _is_plain_text(filename):
        return content.decode("utf-8", errors="replace")

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
    text = (doc.get("md_content") or payload.get("markdown") or "").strip()
    if not text:  # 都没有 markdown 时兜底纯文本
        text = (doc.get("text_content") or "").strip()
    return text


def _is_plain_text(filename: str) -> bool:
    """判断是否为纯文本格式（不调 docling，直接 decode）。"""
    name = (filename or "").lower()
    return name.endswith((".txt", ".md", ".markdown", ".csv"))

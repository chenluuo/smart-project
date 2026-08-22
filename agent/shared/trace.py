"""trace_id 贯穿工具：生成/透传/读取。

约定：跨服务调用在 HTTP 头 `X-Trace-Id` 传递；进程内用 contextvar 传播。
"""
from __future__ import annotations

import contextvars
import uuid

_current: contextvars.ContextVar[str | None] = contextvars.ContextVar("trace_id", default=None)

HEADER = "X-Trace-Id"


def new_trace_id() -> str:
    return uuid.uuid4().hex


def set_trace_id(trace_id: str) -> None:
    _current.set(trace_id)


def get_trace_id() -> str | None:
    return _current.get()


def ensure_trace_id() -> str:
    """当前无 trace_id 则生成并设置，返回当前值。"""
    tid = get_trace_id()
    if not tid:
        tid = new_trace_id()
        set_trace_id(tid)
    return tid

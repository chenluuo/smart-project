"""ingest-service：worker 主循环（无对外 HTTP，消费 3 个 Redis Stream + 定时任务调度）。

- queue:doc.process      文档向量化
- queue:session.summary  会话摘要
- queue:session.activity LRU 逐出判定
- 定时任务调度：每 5 分钟检查到期的定时任务，触发 agent 处理（agent:task:schedule ZSET）
"""
from __future__ import annotations

import logging
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "ingest-service"))
sys.path.insert(0, str(ROOT / "agent-service"))  # 复用 llm 适配、task_trigger

from shared.config import get_config  # noqa: E402
from shared.observability import configure_logging  # noqa: E402
from shared.redis_client import get_redis  # noqa: E402
from shared.trace import bind_correlation_ids, normalize_correlation_id, reset_correlation_ids  # noqa: E402

from doc_processor import process_doc_event, reconcile_knowledge  # noqa: E402
from lru import handle_activity  # noqa: E402
from summarizer import build_summary  # noqa: E402

configure_logging()
log = logging.getLogger("ingest")

CONSUMER = "ingest-worker-1"

HANDLERS = {
    "doc.process": lambda ev: process_doc_event(ev),
    "session.summary": lambda ev: build_summary(ev.get("session_id", ""), ev.get("user_id", "")),
    "session.activity": lambda ev: handle_activity([ev]),
}

# 定时任务调度间隔（秒）：5 分钟轮询到期的定时任务
TASK_POLL_SECONDS = 300
_TASK_SCHEDULE_KEY = "agent:task:schedule"

# 向量对账间隔（秒）：默认每日一次；启动时额外跑一次（清理历史残留）。
# 对账兜底"删除事件清理失败被丢弃"导致的永久残留（见 doc_processor.reconcile_knowledge）。
RECONCILE_SECONDS = int(get_config("ingest").get("reconcile_interval_seconds", 86400))


def run_task_schedule() -> None:
    """检查到期的定时任务并触发 agent 处理。

    触发方式：调 agent-service 内部接口（agent-service 具备访问
    context/tool/go 的完整网络配置；ingest 容器只消费队列）。
    """
    try:
        redis = get_redis()
        now_ms = time.time() * 1000
        due_ids = redis.task_due(now_ms, limit=50)
        if not due_ids:
            return
        # 每个到期任务：查定义 → 更新下一次触发 → 调 agent-service 触发
        for task_id in due_ids:
            matched = _find_task_by_id(redis, task_id)
            if not matched:
                redis.client().zrem(_TASK_SCHEDULE_KEY, task_id)
                continue
            user_id, task = matched
            # 计算下一次触发时间
            next_run = _next_task_run(redis, user_id, task, now_ms)
            redis.task_update_next_run(user_id, task_id, next_run)
            # 触发 agent（内部接口，agent-service 负责编排）
            try:
                _notify_agent_service(task)
            except Exception as e:
                log.warning("触发定时任务失败", extra={"task_id": task_id, "error": str(e)})
    except Exception as e:
        log.warning("定时任务调度失败: %s", e)


def _notify_agent_service(task: dict) -> None:
    """调 agent-service 内部接口触发任务处理（复用内部密钥）。"""
    import httpx

    base = get_config("agent").get("base_url", "http://agent-service:8000")
    internal_key = get_config("go").get("internal_key", "")
    with httpx.Client(timeout=10, trust_env=False) as client:
        resp = client.post(
            f"{base}/internal/task/trigger",
            json={"task": task},
            headers={"X-Internal-Key": internal_key or ""},
        )
        resp.raise_for_status()


def _find_task_by_id(redis, task_id: str):
    """按 task_id 反查任务（scan 匹配 agent:task:*:{task_id}）。"""
    for key in redis.client().scan_iter(match="agent:task:*:*", count=100):
        if key.endswith(f":{task_id}"):
            raw = redis.client().hgetall(key)
            if raw:
                return raw.get("user_id"), raw
    return None


def _next_task_run(redis, user_id: str, task: dict, now_ms: float):
    """计算任务下一次触发时间（毫秒）；once 完成返回 None（删除调度）。"""
    from shared.scheduler import next_run

    trigger_type = task.get("trigger_type", "")
    trigger = task.get("trigger", "")
    last_run_at = task.get("last_run_at")
    try:
        import datetime

        return next_run(trigger_type, trigger, last_run_at=last_run_at,
                        now=datetime.datetime.fromtimestamp(now_ms / 1000, datetime.timezone.utc))
    except Exception:
        return None


def run_reconcile() -> None:
    """向量对账：清理 Milvus 有向量但 Go 不在 ACTIVE 清单的文档（兜底永久残留）。"""
    try:
        result = reconcile_knowledge()
        log.info(
            "知识向量对账完成",
            extra={
                "event": "knowledge_reconcile",
                "service": "ingest-service",
                "result": result.get("status"),
                "stale_count": len(result.get("stale") or []),
                "cleaned": result.get("cleaned", 0),
            },
        )
        if result.get("status") == "failed":
            log.warning("知识向量对账失败: %s", result.get("reason"))
        elif result.get("failed"):
            log.warning("知识向量对账部分失败: %s", result["failed"])
    except Exception as e:
        log.warning("知识向量对账异常: %s", e)


def run_once() -> None:
    r = get_redis()
    for stream in HANDLERS:
        try:
            events = r.xread_group(stream, CONSUMER, count=10, block_ms=100)
        except Exception as e:
            log.warning("读取 %s 失败: %s", stream, e)
            continue
        for ev in events:
            event_id = str(ev.get("id", ""))
            request_id = normalize_correlation_id(event_id)
            trace_id = normalize_correlation_id(ev.get("trace_id"), request_id)
            tokens = bind_correlation_ids(trace_id, request_id)
            started_at = time.perf_counter()
            try:
                result = HANDLERS[stream](ev)
                r.xack(stream, ev["id"])  # 处理成功才 ACK
                log.info(
                    "Stream event processed",
                    extra={
                        "event": "stream_event_processed",
                        "service": "ingest-service",
                        "stream": stream,
                        "event_id": event_id,
                        "trace_id": trace_id,
                        "request_id": request_id,
                        "result": "success",
                        "duration_ms": round((time.perf_counter() - started_at) * 1000, 3),
                    },
                )
            except Exception as e:
                # 失败重投（带重试计数，超限丢弃），失败消息不再丢失
                recovery_exception = None
                try:
                    requeued = r.retry_later(stream, ev, retry_key="retry", max_retries=5)
                    if requeued:
                        r.xack(stream, ev["id"])  # 原消息已重投，可 ACK
                        outcome = "requeued"
                    else:
                        r.xack(stream, ev["id"])
                        outcome = "discarded"
                except Exception as recovery_error:
                    recovery_exception = type(recovery_error).__name__
                    outcome = "recovery_failed"
                failure_record = {
                    "event": "stream_event_processed",
                    "service": "ingest-service",
                    "stream": stream,
                    "event_id": event_id,
                    "trace_id": trace_id,
                    "request_id": request_id,
                    "result": "failure",
                    "outcome": outcome,
                    "exception_type": type(e).__name__,
                    "duration_ms": round((time.perf_counter() - started_at) * 1000, 3),
                }
                if recovery_exception:
                    failure_record["recovery_exception_type"] = recovery_exception
                log.error(
                    "Stream event processing failed",
                    extra=failure_record,
                )
            finally:
                reset_correlation_ids(tokens)


def main() -> None:
    log.info("ingest-service 启动，消费队列: %s", list(HANDLERS))
    last_task_check = 0.0
    last_reconcile = 0.0
    # 启动即对账一次：清理历史遗留的永久残留（不等首个周期）
    run_reconcile()
    last_reconcile = time.time()
    while True:
        run_once()
        # 定时任务：每 TASK_POLL_SECONDS 检查一次
        if time.time() - last_task_check >= TASK_POLL_SECONDS:
            run_task_schedule()
            last_task_check = time.time()
        # 向量对账：每 RECONCILE_SECONDS（默认每日）一次
        if time.time() - last_reconcile >= RECONCILE_SECONDS:
            run_reconcile()
            last_reconcile = time.time()
        time.sleep(0.5)


if __name__ == "__main__":
    main()

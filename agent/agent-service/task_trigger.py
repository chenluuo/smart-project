"""定时任务触发：到点后以任务所属农户身份跑完整 agent 编排，结果存主动通知。

与告警主动推送（main.py _handle_alert_in_background）同构：
- issue_user_token(owner_id) 签发 JWT
- handle_question 完整编排（agent 自行决定查哪些数据、调什么工具）
- 结果 proactive_push 到 agent:proactive:{userId}（前端在线/上线拉取，读后清）
"""
from __future__ import annotations

import asyncio
import threading
import time
from typing import Any


def _build_task_question(task: dict[str, Any], last_summary: str | None) -> str:
    parts = [
        f"【定时任务·{task.get('name', '未命名任务')}】现在到触发时间了。",
    ]
    message = task.get("message")
    if message:
        parts.append(f"用户设定的任务内容：{message}")
    if last_summary:
        parts.append(f"上次执行结果：{last_summary}")
    parts.append("请以该农户的生产顾问身份，自主完成上述任务并汇报结果。")
    return "".join(parts)


def run_task_in_background(task: dict[str, Any]) -> None:
    """后台线程：以任务所属用户身份跑 agent 编排，结果存 proactive。"""
    from shared.jwt import issue_user_token
    from shared.redis_client import get_redis
    from orchestrator import handle_question

    user_id = str(task.get("user_id") or "")
    if not user_id:
        print(f"[task-trigger] 缺少 user_id，跳过任务 {task.get('task_id')}", flush=True)
        return
    try:
        token = issue_user_token(int(user_id))
    except Exception as e:
        print(f"[task-trigger] 签发用户 token 失败: {e}", flush=True)
        return

    collected: list[dict[str, Any]] = []
    try:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)

        async def run():
            async for ev in handle_question(
                user_id=user_id,
                question=_build_task_question(task, task.get("last_summary")),
                session_id=None,
                plot_id=None,
                authorization=f"Bearer {token}",
            ):
                collected.append(ev)

        loop.run_until_complete(run())
        loop.close()
    except Exception as e:
        print(f"[task-trigger] 主动处理失败: {e}", flush=True)

    final = "".join(ev.get("delta", "") for ev in collected if ev.get("type") == "answer")
    get_redis().proactive_push(user_id, {
        "ts": time.time(),
        "task": {"task_id": task.get("task_id"), "name": task.get("name"), "trigger_type": task.get("trigger_type")},
        "summary": final or "(agent 未生成摘要)",
    })
    print(f"[task-trigger] user={user_id} task={task.get('task_id')} 完成, summary_len={len(final)}", flush=True)


def trigger_task(task: dict[str, Any]) -> None:
    """同步触发（由 worker 调用）：后台线程执行，不阻塞调度循环。"""
    threading.Thread(target=run_task_in_background, args=(task,), daemon=True).start()

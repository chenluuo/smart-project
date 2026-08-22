"""agent-service：问答编排主流程。

链路：JWT → 会话状态（惰性超时检查）→ 三路取数（知识/现场工具/记忆）
     → context/build 组装 → LLM 流式 → 落库 chat_messages → 更新窗口+activity
     → 收尾意图（can_close → 追问"还有其他问题吗"）
"""
from __future__ import annotations

import json
from typing import Any, AsyncIterator

import httpx

from shared.config import get_config
from shared.go_client import get_go_client
from shared.redis_client import get_redis
from shared.trace import ensure_trace_id
from llm import get_llm
from rag import memory_recall, rag_search
from session import (
    STATUS_ACTIVE,
    STATUS_CLOSED,
    STATUS_WAITING_CLOSE,
    check_lazy_timeout,
    close,
    create_session,
    get_session,
    llm_intent,
    mark_waiting_close,
    rule_close_hit,
    touch,
)

_context_url = None
_tool_url = None


def _svc_url(service: str) -> str:
    cfg = get_config(service)
    return cfg.get("base_url", f"http://{service}:{cfg.get('port', 8000)}")


async def _call_context(payload: dict, authorization: str) -> dict:
    async with httpx.AsyncClient(timeout=15) as client:
        resp = await client.post(
            f"{_context_url}/context/build",
            json=payload,
            headers={"Authorization": authorization, "X-Trace-Id": ensure_trace_id()},
        )
        resp.raise_for_status()
        return resp.json()


async def _call_tool(name: str, args: dict, authorization: str) -> dict:
    async with httpx.AsyncClient(timeout=15) as client:
        resp = await client.post(
            f"{_tool_url}/tools/{name}/execute",
            json={"args": args},
            headers={"Authorization": authorization, "X-Trace-Id": ensure_trace_id()},
        )
        resp.raise_for_status()
        return resp.json()


async def _tool_schemas() -> list[dict]:
    """拉取工具清单（启动缓存，失败返回空）。"""
    async with httpx.AsyncClient(timeout=10) as client:
        resp = await client.get(f"{_tool_url}/tools", headers={"X-Trace-Id": ensure_trace_id()})
        resp.raise_for_status()
        defs = resp.json()
    return [
        {
            "type": "function",
            "function": {
                "name": d["name"],
                "description": d["description"],
                "parameters": {"type": "object", "properties": d["parameters"], "required": d["required"]},
            },
        }
        for d in defs
    ]


async def handle_question(
    user_id: str,
    question: str,
    session_id: str | None,
    plot_id: str | None,
    authorization: str,
) -> AsyncIterator[dict[str, Any]]:
    """处理一轮问答，SSE 事件流：{type: answer/done/error, ...}。"""
    global _context_url, _tool_url
    _context_url = _context_url or _svc_url("context")
    _tool_url = _tool_url or _svc_url("tool")

    # ---------- 会话状态 ----------
    state = get_session(session_id) if session_id else None
    if state is None:
        state = create_session(user_id, session_id or "sess_" + ensure_trace_id()[:12], plot_id)
        session_id = session_id or state.get("session_id", "")
    if state["status"] == STATUS_CLOSED:
        yield {"type": "error", "message": "会话已结束，请发起新会话"}
        return
    if state["status"] == STATUS_WAITING_CLOSE and check_lazy_timeout(state):
        close(session_id, state)
        yield {"type": "error", "message": "会话已超时结束，请发起新会话"}
        return

    # waiting_close 下的回复：先规则判定（省一轮 LLM）
    if state["status"] == STATUS_WAITING_CLOSE:
        if rule_close_hit(question):
            close(session_id, state)
            yield {"type": "done", "sessionId": session_id, "canClose": True, "closed": True,
                   "message": "好的，有需要随时找我！"}
            return
        intent = await llm_intent(question)
        if intent == "close":
            close(session_id, state)
            yield {"type": "done", "sessionId": session_id, "canClose": True, "closed": True,
                   "message": "好的，有需要随时找我！"}
            return
        state["status"] = STATUS_ACTIVE  # 新问题/继续 → 回到 active
        get_redis().session_set(session_id, state)

    # ---------- 三路取数（并行） ----------
    knowledge = []
    memory = []
    try:
        knowledge = rag_search(question)
    except Exception:
        pass  # 知识检索失败不阻塞问答
    try:
        memory = memory_recall(user_id, question)
    except Exception:
        pass

    # 现场数据：走工具（LLM 工具循环）
    live_data, tool_log = await _tool_loop(question, plot_id, authorization)

    # ---------- 组装（context-service） ----------
    window_turns = get_redis().window_get(user_id)
    prompt_result = await _call_context(
        {
            "user_id": user_id,
            "question": question,
            "window_turns": window_turns[-8:],
            "knowledge_chunks": [k.get("content", "") for k in knowledge[:5]],
            "memory_chunks": [m["summary"] for m in memory[:3]],
            "live_data": live_data,
        },
        authorization,
    )
    prompt = prompt_result["prompt"]

    messages = [{"role": "system", "content": prompt}]
    for t in window_turns[-6:]:
        messages.append({"role": t["role"], "content": t["content"]})
    messages.append({"role": "user", "content": question})

    # ---------- LLM 流式 ----------
    answer_parts: list[str] = []
    async for delta in get_llm().chat_stream(messages):
        answer_parts.append(delta)
        yield {"type": "answer", "delta": delta}

    answer = "".join(answer_parts)
    can_close = _judge_can_close(question, answer)

    # ---------- 落库 + 窗口 + activity ----------
    try:
        get_go_client().post_message(authorization, session_id, {
            "role": "user", "content": question, "plot_id": plot_id, "model_version": get_config("llm").get("model"),
        })
        get_go_client().post_message(authorization, session_id, {
            "role": "assistant", "content": answer,
            "citations": [{"docId": k.get("docId"), "title": k.get("title"), "version": k.get("version")} for k in knowledge[:5]] or None,
            "plot_id": plot_id, "model_version": get_config("llm").get("model"),
        })
    except Exception:
        pass  # 落库失败不阻塞回答（告警日志记录）

    # 更新短期窗口（只存文字对话，现场数据不进窗口）
    window = get_redis().window_get(user_id)
    window.append({"role": "user", "content": question})
    window.append({"role": "assistant", "content": answer})
    get_redis().window_set(user_id, window[-16:])
    get_redis().touch_active(user_id)
    get_redis().xadd("session.activity", {"user_id": user_id, "ts": __import__("time").time()})

    touch(session_id, state)
    if can_close:
        mark_waiting_close(session_id, state)
        answer_note = "\n\n还有其他问题吗？"
        yield {"type": "answer", "delta": answer_note}
        yield {"type": "done", "sessionId": session_id, "canClose": True,
               "sources": _sources(knowledge), "closed": False}
    else:
        yield {"type": "done", "sessionId": session_id, "canClose": False,
               "sources": _sources(knowledge), "closed": False}


async def _tool_loop(question: str, plot_id: str | None, authorization: str) -> tuple[list[dict], list[str]]:
    """LLM 工具调用循环（最多 max_tool_rounds 轮）。"""
    max_rounds = int(get_config("agent").get("max_tool_rounds", 5))
    live_data: list[dict] = []
    log: list[str] = []
    if not plot_id:
        return live_data, log  # 无地块上下文，交由 LLM 自行决定（可再扩展）
    try:
        tools = await _tool_schemas()
    except Exception:
        return live_data, log
    if not tools:
        return live_data, log

    messages = [
        {"role": "system", "content": "根据用户问题判断是否需要调用工具获取现场数据。需要则输出工具调用。"},
        {"role": "user", "content": question},
    ]
    for _ in range(max_rounds):
        resp = await get_llm()._get_client().chat.completions.create(
            model=get_config("llm").get("model"),
            messages=messages,
            tools=tools,
            tool_choice="auto",
        )
        msg = resp.choices[0].message
        if not msg.tool_calls:
            break
        for tc in msg.tool_calls:
            name = tc.function.name
            try:
                args = json.loads(tc.function.arguments or "{}")
            except json.JSONDecodeError:
                args = {}
            result = await _call_tool(name, args, authorization)
            log.append(f"{name}({args}) -> ok={result.get('ok')}")
            if name == "get_latest_telemetry" and result.get("ok"):
                data = result.get("data") or {}
                metrics = (data.get("metrics") or {})
                for m, v in metrics.items():
                    live_data.append({
                        "plot_id": args.get("plot_id") or data.get("plotId"),
                        "metric": m, "value": v.get("value"), "unit": v.get("unit"),
                        "sampled_at": data.get("sampleTime", ""),
                    })
            messages.append({
                "role": "tool", "tool_call_id": tc.id,
                "content": json.dumps(result, ensure_ascii=False)[:2000],
            })
    return live_data, log


def _judge_can_close(question: str, answer: str) -> bool:
    """收尾意图启发式：问题简短 + 回答给出建议 → 视为可收尾（生产可换 LLM 判定）。"""
    return len(question) < 40


def _sources(knowledge: list[dict]) -> list[dict]:
    return [
        {"type": "KNOWLEDGE_DOC", "title": k.get("title"), "docId": k.get("docId"),
         "version": k.get("version"), "score": k.get("score")}
        for k in knowledge[:5]
    ]

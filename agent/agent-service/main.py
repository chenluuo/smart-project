"""agent-service：FastAPI 入口（端口 8000）。

- POST /agent/chat                 问答入口（SSE 流式）
- POST /agent/chat/sessions/{id}/close  显式结束会话（前端按钮）
- POST /internal/knowledge/notify  Go 文档上传/变更通知（内部密钥）
"""
from __future__ import annotations

import asyncio
import json
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from fastapi import FastAPI, Header, HTTPException, Request  # noqa: E402
from fastapi.middleware.cors import CORSMiddleware  # noqa: E402
from fastapi.responses import StreamingResponse  # noqa: E402
from pydantic import BaseModel, ConfigDict, Field  # noqa: E402

from orchestrator import handle_question  # noqa: E402
from session import STATUS_CLOSED, close, get_session  # noqa: E402
from shared.config import get_config  # noqa: E402
from shared.go_client import get_go_client  # noqa: E402
from shared.jwt import JWTError, issue_user_token, user_id_from_token  # noqa: E402
from shared.observability import install_observability  # noqa: E402
from shared.redis_client import get_redis  # noqa: E402
from shared.trace import set_actor_id  # noqa: E402

app = FastAPI(title="agent-service", version="0.1.0")

# 测试期放开跨域（前端直连 agent/chat；生产收紧为白名单）
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

install_observability(app, "agent-service")

# 测试期放开跨域（极简前端直连 agent/chat；生产收紧为白名单）
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# ---------- 并发控制：最大并发对话数（config.yaml agent.max_concurrency） ----------
_sem: asyncio.Semaphore | None = None


def _get_sem() -> asyncio.Semaphore:
    global _sem
    if _sem is None:
        _sem = asyncio.Semaphore(int(get_config("agent").get("max_concurrency", 4)))
    return _sem


def _wait_seconds() -> float:
    return float(get_config("agent").get("concurrency_wait_seconds", 30))


class ChatRequest(BaseModel):
    session_id: str | None = None
    plot_id: str | None = None
    question: str = Field(..., min_length=1, max_length=2000)


def _require_user(authorization: str) -> str:
    if not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="缺少 JWT")
    try:
        return user_id_from_token(authorization[7:])
    except JWTError as e:
        raise HTTPException(status_code=401, detail=str(e)) from e


@app.get("/healthz")
def healthz() -> dict:
    sem = _get_sem()
    max_c = int(get_config("agent").get("max_concurrency", 4))
    waiters = sem._waiters or []
    return {
        "ok": True,
        "concurrency": {
            "max": max_c,
            "in_flight": max_c - sem._value,          # 已占用槽位
            "waiting": len(waiters),                  # 排队中的请求
            "available": sem._value,                  # 剩余槽位
        },
    }


@app.post("/agent/chat")
async def chat(req: ChatRequest, request: Request,
               authorization: str = Header(default=""),
               x_trace_id: str = Header(default="", alias="X-Trace-Id")) -> StreamingResponse:
    user_id = _require_user(authorization)
    set_actor_id(user_id)
    request.state.actor_id = user_id

    async def event_stream():
        # 先发占位事件：立即发出响应头，避免客户端长时间等待首 token（TTFT）
        yield "event: started\ndata: {\"type\": \"started\"}\n\n"
        # 并发控制：wait=0 时并发满立即拒绝（不排队）；wait>0 时排队等待
        sem = _get_sem()
        if _wait_seconds() <= 0:
            # 非阻塞：locked()=True 表示无可用槽位 → 立即繁忙（同事件循环内无竞争）
            if sem.locked():
                yield ("event: error\ndata: {\"type\": \"error\", "
                       "\"message\": \"系统繁忙，请稍后重试\"}\n\n")
                return
            await sem.acquire()
        else:
            try:
                await asyncio.wait_for(sem.acquire(), timeout=_wait_seconds())
            except asyncio.TimeoutError:
                yield ("event: error\ndata: {\"type\": \"error\", "
                       "\"message\": \"系统繁忙，请稍后重试\"}\n\n")
                return
        try:
            async for ev in handle_question(
                user_id=user_id,
                question=req.question,
                session_id=req.session_id,
                plot_id=req.plot_id,
                authorization=authorization,
            ):
                yield f"event: {ev['type']}\ndata: {json.dumps(ev, ensure_ascii=False)}\n\n"
        except Exception as e:  # 兜底，避免 SSE 中断无响应
            yield f"event: error\ndata: {json.dumps({'type':'error','message':str(e)}, ensure_ascii=False)}\n\n"
        finally:
            sem.release()

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


class CloseRequest(BaseModel):
    pass


@app.post("/agent/chat/sessions/{session_id}/close")
def close_session(session_id: str, request: Request, authorization: str = Header(default="")) -> dict:
    user_id = _require_user(authorization)
    set_actor_id(user_id)
    request.state.actor_id = user_id
    state = get_session(session_id)
    if state is None:
        raise HTTPException(status_code=404, detail="会话不存在")
    close(session_id, state)
    # 同步关闭 Go 侧会话（chat_xxx id；幂等，失败不阻塞本地状态机）
    try:
        get_go_client().close_session(authorization, session_id)
    except Exception:
        pass
    get_redis().xadd("session.summary", {"session_id": session_id, "user_id": state.get("user_id")})
    return {"session_id": session_id, "status": STATUS_CLOSED}


class NotifyRequest(BaseModel):
    # Go 端 payload 契约：{"docId": <数字>, "event": "UPLOADED|UPDATED|ARCHIVED", "version": n}
    model_config = ConfigDict(populate_by_name=True)
    doc_id: str | int = Field(alias="docId")
    event: str
    version: int | None = None


@app.post("/internal/knowledge/notify")
def knowledge_notify(req: NotifyRequest, x_internal_key: str = Header(default="", alias="X-Internal-Key"),
                     x_internal_service_key: str = Header(default="", alias="X-Internal-Service-Key")) -> dict:
    """Go 在文档上传/变更时调用；校验内部密钥后入队加工。"""
    expected = get_config("go").get("internal_key")
    if not expected or (x_internal_key != expected and x_internal_service_key != expected):
        raise HTTPException(status_code=401, detail="内部密钥无效")
    get_redis().xadd("doc.process", {
        "doc_id": req.doc_id, "event": req.event, "version": req.version,
    })
    return {"ok": True}


# ============================================================
# 告警主动推送：Go alert dispatcher → /internal/alerts/notify
# 收到告警后，以该用户身份"像用户发起会话一样"触发 agent 处理，
# 生成主动告警分析与建议，存入 Redis agent:proactive:{ownerId} 供前端/会话读取。
# ============================================================
class AlertNotifyRequest(BaseModel):
    alertId: int | None = None
    ownerId: int | None = None
    ruleId: int | None = None
    plotId: int | None = None
    deviceId: int | None = None
    triggerValue: float | None = None
    metric: str | None = None
    level: str | None = None
    status: str | None = None
    title: str | None = None
    traceId: str | None = None
    triggeredAt: str | None = None


def _build_alert_question(payload: dict) -> str:
    metric = payload.get("metric") or "未知指标"
    level = payload.get("level") or ""
    value = payload.get("triggerValue")
    plot = payload.get("plotId")
    parts = [f"【系统告警主动通知】地块 {plot} 触发{level}告警（{metric}）"]
    if value is not None:
        parts.append(f"，当前值 {value}")
    parts.append("。请以该用户生产顾问的身份，分析告警情况并给出处理建议。")
    return "".join(parts)


def _handle_alert_in_background(payload: dict) -> None:
    """后台线程：以告警所属用户身份跑完整 agent 编排，结果存 Redis 主动通知。"""
    import asyncio
    import threading

    owner_id = payload.get("ownerId") or payload.get("owner_id")
    if not owner_id:
        print(f"[alert-notify] 缺少 ownerId，跳过: {payload}", flush=True)
        return
    try:
        token = issue_user_token(owner_id)
    except Exception as e:
        print(f"[alert-notify] 签发用户 token 失败: {e}", flush=True)
        return

    collected: list[dict] = []
    try:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)

        async def run():
            async for ev in handle_question(
                user_id=str(owner_id),
                question=_build_alert_question(payload),
                session_id=None,
                plot_id=str(payload["plotId"]) if payload.get("plotId") else None,
                authorization=f"Bearer {token}",
            ):
                collected.append(ev)

        loop.run_until_complete(run())
        loop.close()
    except Exception as e:
        print(f"[alert-notify] 主动处理失败: {e}", flush=True)

    final = "".join(ev.get("delta", "") for ev in collected if ev.get("type") == "answer")
    get_redis().proactive_push(str(owner_id), {
        "ts": time.time(),
        "alert": payload,
        "summary": final or "(agent 未生成摘要)",
    })
    print(f"[alert-notify] owner={owner_id} 主动处理完成, summary_len={len(final)}", flush=True)


@app.post("/internal/alerts/notify")
def alert_notify(req: AlertNotifyRequest, x_internal_key: str = Header(default="", alias="X-Internal-Key"),
                 x_internal_service_key: str = Header(default="", alias="X-Internal-Service-Key")) -> dict:
    """Go alert dispatcher 推送告警：校验内部密钥后后台触发 agent 主动处理。"""
    import threading
    expected = get_config("go").get("internal_key")
    if not expected or (x_internal_key != expected and x_internal_service_key != expected):
        raise HTTPException(status_code=401, detail="内部密钥无效")
    payload = req.model_dump()
    threading.Thread(target=_handle_alert_in_background, args=(payload,), daemon=True).start()
    return {"ok": True, "accepted": True}

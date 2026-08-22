"""剩余功能测试：新问题回 active / 超时判定 / notify 密钥 / close 按钮 / LRU 逐出。

用法：python scripts/test_misc.py
前置：Redis(6379) + context(8001) + tool(8002 mock) + agent(8000) 已启动
"""
import datetime
import json
import os
import sys
import time
from pathlib import Path

import httpx

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "ingest-service"))

# 环境变量（config.yaml ${VAR} 占位需要；覆盖式避免继承旧值）
os.environ["LLM_API_KEY"] = "sk-9a563ed2c213414ab730b99cc8daee2a"
os.environ["GO_INTERNAL_KEY"] = "test-key"
os.environ["MINIO_SECRET_KEY"] = "minioadmin"
os.environ["JWT_SECRET"] = "test-secret"

JWT_SECRET = os.environ.get("JWT_SECRET", "test-secret")
AGENT_URL = os.environ.get("AGENT_URL", "http://127.0.0.1:8000")
GO_INTERNAL_KEY = os.environ.get("GO_INTERNAL_KEY", "test-key")


def make_token(user_id: str = "u_001") -> str:
    import jwt as pyjwt

    return pyjwt.encode({"user_id": user_id, "exp": int(time.time()) + 3600}, JWT_SECRET, algorithm="HS256")


def send_question(client: httpx.Client, token: str, session_id, question: str, plot_id="plot_a3") -> dict:
    payload = {"session_id": session_id, "plot_id": plot_id, "question": question}
    events = {"answer": 0, "done": 0, "error": 0}
    answer: list[str] = []
    result = {"session_id": None, "can_close": None, "closed": None, "error_msg": None}
    with client.stream("POST", f"{AGENT_URL}/agent/chat", json=payload,
                       headers={"Authorization": f"Bearer {token}"}) as resp:
        assert resp.status_code == 200, f"HTTP {resp.status_code}"
        ev_type = ""
        for line in resp.iter_lines():
            if line.startswith("event: "):
                ev_type = line[7:]
            elif line.startswith("data: "):
                data = json.loads(line[6:])
                if ev_type == "answer":
                    events["answer"] += 1
                    answer.append(data.get("delta", ""))
                elif ev_type == "done":
                    events["done"] += 1
                    result["session_id"] = data.get("sessionId")
                    result["can_close"] = data.get("canClose")
                    result["closed"] = data.get("closed")
                elif ev_type == "error":
                    events["error"] += 1
                    result["error_msg"] = data.get("message")
    result["events"] = events
    result["answer_len"] = len("".join(answer))
    return result


def test_new_question_back_to_active(client: httpx.Client, token: str) -> str:
    """waiting_close 后提出新问题 → 回到 active 正常回答。"""
    print("== 测试: 新问题回到 active")
    r1 = send_question(client, token, None, "A1 湿度多少？")
    assert r1["events"]["done"] == 1, "第一轮失败"
    sid = r1["session_id"]
    assert sid, "缺 session_id"
    assert r1["can_close"] is True, "第一轮应 waiting_close"

    r2 = send_question(client, token, sid, "那 B 地块的温度呢？")
    print(f"   第二轮 events={r2['events']} closed={r2['closed']} 字数={r2['answer_len']}")
    assert r2["events"]["done"] == 1, "第二轮应正常回答"
    assert r2["events"]["answer"] > 0, "新问题应回到 active 并回答"
    assert r2["closed"] is not True, "不应被误判结束"
    print("   通过")
    return sid


def test_timeout_close(client: httpx.Client, token: str) -> None:
    """waiting_close 后超时（5 分钟）→ 惰性检查置 closed。"""
    print("== 测试: waiting_close 超时判定")
    from shared.redis_client import get_redis

    r1 = send_question(client, token, None, "A2 湿度 25% 要紧吗？")
    sid = r1["session_id"]
    assert r1["can_close"] is True, "应进入 waiting_close"

    # 直接把 last_message_at 改为 6 分钟前（模拟超时）
    state = get_redis().session_get(sid)
    old = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(minutes=6)
    state["last_message_at"] = old.isoformat()
    get_redis().session_set(sid, state)

    r2 = send_question(client, token, sid, "在吗？")
    print(f"   回复后 events={r2['events']} error={r2['error_msg']}")
    assert r2["events"]["error"] == 1, "超时应返回 error（会话已结束）"
    assert "超时" in (r2["error_msg"] or ""), f"错误信息不符: {r2['error_msg']}"
    print("   通过")


def test_notify_auth(client: httpx.Client) -> None:
    """文档通知：内部密钥校验。"""
    print("== 测试: /internal/knowledge/notify 密钥校验")
    url = f"{AGENT_URL}/internal/knowledge/notify"

    r = client.post(url, json={"doc_id": "doc_001", "event": "uploaded"})
    assert r.status_code == 401, f"无密钥应 401，实际 {r.status_code}"
    print("   无密钥 → 401 OK")

    r = client.post(url, json={"doc_id": "doc_001", "event": "uploaded"},
                    headers={"X-Internal-Key": "wrong-key"})
    assert r.status_code == 401
    print("   错密钥 → 401 OK")

    r = client.post(url, json={"doc_id": "doc_001", "event": "uploaded", "version": 1},
                    headers={"X-Internal-Key": GO_INTERNAL_KEY})
    assert r.status_code == 200 and r.json() == {"ok": True}, r.text
    print("   正确密钥 → 200 ok=true OK")

    # 验证入队
    from shared.redis_client import get_redis

    rds = get_redis().client()
    length = rds.xlen("queue:doc.process")
    assert length >= 1, f"queue:doc.process 应至少 1 条，实际 {length}"
    print(f"   queue:doc.process 入队 OK (len={length})")


def test_close_button(client: httpx.Client, token: str) -> None:
    """显式关闭会话（按钮）。"""
    print("== 测试: 显式关闭会话 /close")
    from shared.redis_client import get_redis

    r = client.post(f"{AGENT_URL}/agent/chat/sessions/nonexist/close",
                    headers={"Authorization": f"Bearer {token}"})
    assert r.status_code == 404, "不存在会话应 404"
    print("   不存在会话 → 404 OK")

    r1 = send_question(client, token, None, "A3 需要浇水吗？")
    sid = r1["session_id"]
    r = client.post(f"{AGENT_URL}/agent/chat/sessions/{sid}/close",
                    headers={"Authorization": f"Bearer {token}"})
    assert r.status_code == 200 and r.json()["status"] == "closed", r.text
    state = get_redis().session_get(sid)
    assert state["status"] == "closed", "Redis 状态应为 closed"
    print(f"   会话 {sid} → closed OK")


def test_lru_eviction() -> None:
    """ingest LRU 判定：超限逐出最久未回用户（直接销毁，不写回）。"""
    print("== 测试: ingest LRU 逐出判定")
    import lru as ingest_service_lru  # ingest-service/lru.py

    # patch 配置：最大 3 用户、每批逐出 2
    _orig = ingest_service_lru.get_config
    ingest_service_lru.get_config = lambda service=None: (
        {"max_active_users": 3, "evict_batch": 2} if service == "ingest" else _orig(service)
    )
    from shared.redis_client import get_redis

    rds = get_redis().client()
    rds.delete("ctx:active")
    for i, uid in enumerate(["user_old1", "user_old2", "user_old3", "user_new"]):
        rds.zadd("ctx:active", {uid: 1000 + i})

    result = ingest_service_lru.handle_activity([{"user_id": "user_new"}])
    print(f"   结果: {result}")
    evicted = result["evicted"]
    assert "user_old1" in evicted and "user_old2" in evicted, f"应逐出最久未回: {evicted}"
    assert rds.zscore("ctx:active", "user_old1") is None, "被逐出用户应从 ZSET 移除"
    assert rds.exists("ctx:user_old1") == 0, "被逐出用户 ctx 应销毁"
    # 未逐出的保留
    assert rds.zscore("ctx:active", "user_old3") is not None or "user_old3" in evicted, "user_old3 状态异常"
    print("   通过（最久未回被逐出且缓存销毁）")


def main() -> int:
    token = make_token()
    with httpx.Client(timeout=120) as client:
        test_new_question_back_to_active(client, token)
        test_timeout_close(client, token)
        test_notify_auth(client)
        test_close_button(client, token)
    test_lru_eviction()
    print("\n=== 剩余功能测试全部通过 ===")
    return 0


if __name__ == "__main__":
    sys.exit(main())

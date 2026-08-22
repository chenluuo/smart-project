"""多轮对话 + 结束判定测试。

场景：
1. 第一轮提问 → 拿 session_id（waiting_close）
2. 同一会话回复"没有了" → 规则命中 → closed（不调 LLM）
"""
import json
import os
import sys
import time

import httpx

JWT_SECRET = os.environ.get("JWT_SECRET", "test-secret")
AGENT_URL = os.environ.get("AGENT_URL", "http://127.0.0.1:8000")


def make_token(user_id: str = "u_001") -> str:
    import jwt as pyjwt

    return pyjwt.encode({"user_id": user_id, "exp": int(time.time()) + 3600}, JWT_SECRET, algorithm="HS256")


def send_question(client: httpx.Client, token: str, session_id, question: str, plot_id="plot_a3") -> dict:
    payload = {"session_id": session_id, "plot_id": plot_id, "question": question}
    events = {"answer": 0, "done": 0, "error": 0}
    answer: list[str] = []
    result = {"session_id": None, "can_close": None, "closed": None}
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
                    print(f"  [error] {data.get('message')}")
    result["events"] = events
    result["answer_len"] = len("".join(answer))
    return result


def main() -> int:
    token = make_token()
    with httpx.Client(timeout=120) as client:
        # 第一轮
        print("== 第一轮: A1 湿度 28% 要灌溉吗？")
        r1 = send_question(client, token, None, "A1 地块土壤湿度 28%，需要灌溉吗？")
        print(f"   events={r1['events']} canClose={r1['can_close']} closed={r1['closed']} 字数={r1['answer_len']}")
        assert r1["events"]["done"] == 1 and r1["events"]["error"] == 0, "第一轮失败"
        assert r1["can_close"] is True, "第一轮应置 waiting_close"
        sid = r1["session_id"]
        assert sid, "缺少 session_id"

        # 第二轮：结束意图（规则命中，不调 LLM → 应无 answer 事件）
        print(f"== 第二轮(同会话 {sid}): 没有了")
        r2 = send_question(client, token, sid, "没有了")
        print(f"   events={r2['events']} canClose={r2['can_close']} closed={r2['closed']} 字数={r2['answer_len']}")
        assert r2["events"]["done"] == 1, "第二轮应 done"
        assert r2["closed"] is True, "规则命中应置 closed"
        assert r2["events"]["answer"] == 0, "规则命中不应再调 LLM（省一轮）"

    print("\n=== 多轮 + 结束判定测试通过 ===")
    return 0


if __name__ == "__main__":
    sys.exit(main())

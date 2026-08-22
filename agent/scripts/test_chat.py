"""智能体对话测试（真实 LLM：DeepSeek deepseek-v4-flash）。

用法：python scripts/test_chat.py ["问题"]
前置：Redis(6379) + context(8001) + tool(8002 mock) + agent(8000) 已启动
"""
import json
import os
import sys
import time

import httpx

JWT_SECRET = os.environ.get("JWT_SECRET", "test-secret")
AGENT_URL = os.environ.get("AGENT_URL", "http://127.0.0.1:8000")

QUESTION = sys.argv[1] if len(sys.argv) > 1 else "A1 地块土壤湿度 28%，需要灌溉吗？"


def make_token(user_id: str = "u_001") -> str:
    import jwt as pyjwt

    return pyjwt.encode({"user_id": user_id, "exp": int(time.time()) + 3600}, JWT_SECRET, algorithm="HS256")


def main() -> int:
    token = make_token()
    print(f"== 问题: {QUESTION}\n")

    payload = {"session_id": None, "plot_id": "plot_a3", "question": QUESTION}
    with httpx.Client(timeout=120) as client:
        with client.stream(
            "POST", f"{AGENT_URL}/agent/chat", json=payload,
            headers={"Authorization": f"Bearer {token}"},
        ) as resp:
            print(f"HTTP {resp.status_code}")
            if resp.status_code != 200:
                print(resp.text[:500])
                return 1
            full_answer: list[str] = []
            events = {"answer": 0, "done": 0, "error": 0}
            for line in resp.iter_lines():
                if not line:
                    continue
                if line.startswith("event: "):
                    ev_type = line[7:]
                elif line.startswith("data: "):
                    try:
                        data = json.loads(line[6:])
                    except json.JSONDecodeError:
                        continue
                    if ev_type == "answer":
                        events["answer"] += 1
                        full_answer.append(data.get("delta", ""))
                        print(data.get("delta", ""), end="", flush=True)
                    elif ev_type == "done":
                        events["done"] += 1
                        print(f"\n\n[done] canClose={data.get('canClose')} closed={data.get('closed')}")
                        print(f"[sources] {json.dumps(data.get('sources'), ensure_ascii=False)}")
                    elif ev_type == "error":
                        events["error"] += 1
                        print(f"\n[error] {data.get('message')}")

    print(f"\n== 事件统计: {events} | 回答字数: {len(''.join(full_answer))}")
    if events["done"] == 0 or events["error"] > 0:
        print("== 对话链路异常")
        return 1
    print("== 对话测试通过")
    return 0


if __name__ == "__main__":
    sys.exit(main())

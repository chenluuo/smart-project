"""Agent 循环测试：中间回复直出 / 工具调用 / final 标注 / 基于工具反馈的最终回答。

用法：python scripts/test_agent_loop.py
前置：三服务已启动 + Milvus（知识库有数据）
"""
import json
import os
import sys
import time
from pathlib import Path

import httpx

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

import jwt as pyjwt

TOKEN = pyjwt.encode({"user_id": "u_001", "exp": int(time.time()) + 3600}, "test-secret", algorithm="HS256")
AGENT = "http://127.0.0.1:8000"


def main() -> int:
    question = sys.argv[1] if len(sys.argv) > 1 else "A3 地块土壤湿度多少？需要灌溉吗？"
    print(f"== 问题: {question}\n")
    events = {"answer": 0, "tool_call": 0, "done": 0, "error": 0}
    answer_text: list[str] = []
    final_prefix_seen = False
    with httpx.Client(timeout=300, trust_env=False) as c:
        with c.stream("POST", f"{AGENT}/agent/chat",
                      json={"session_id": None, "plot_id": "plot_a3", "question": question},
                      headers={"Authorization": f"Bearer {TOKEN}"}) as resp:
            print(f"HTTP {resp.status_code}")
            ev = ""
            for line in resp.iter_lines():
                if line.startswith("event: "):
                    ev = line[7:]
                elif line.startswith("data: "):
                    d = json.loads(line[6:])
                    t = d.get("type")
                    events[t] = events.get(t, 0) + 1
                    if t == "answer":
                        delta = d.get("delta", "")
                        answer_text.append(delta)
                        if "【最终回复】" in delta:
                            final_prefix_seen = True
                        print(f"  [answer] {delta}", end="", flush=True)
                    elif t == "tool_call":
                        print(f"\n  >>> 工具调用: {json.dumps(d.get('tool_calls'), ensure_ascii=False)}")
                    elif t == "done":
                        print(f"\n  [done] canClose={d.get('canClose')} sources={len(d.get('sources') or [])}")
                    elif t == "error":
                        print(f"\n  [error] {d.get('message')}")
    print(f"\n== 事件统计: {events}")
    ok = (
        events["done"] >= 1
        and events["error"] == 0
        and final_prefix_seen  # 最终回复以"【最终回复】"前缀标注（代码判定）
        and len("".join(answer_text)) > 30
    )
    print("== Agent 循环测试", "通过" if ok else "失败")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

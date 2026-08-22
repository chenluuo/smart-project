"""验证 SSE 流式性：记录每条 delta 到达时间与间隔。"""
import json
import sys
import time
from pathlib import Path

import httpx

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

import jwt as pyjwt

token = pyjwt.encode({"user_id": "u_001", "exp": int(time.time()) + 3600}, "test-secret", algorithm="HS256")
t0 = time.time()
last = None

with httpx.Client(timeout=300) as c:
    with c.stream(
        "POST",
        "http://127.0.0.1:8000/agent/chat",
        json={"session_id": None, "plot_id": "plot_a3",
              "question": "番茄地湿度28%需要灌溉吗？给个简短建议。"},
        headers={"Authorization": f"Bearer {token}"},
    ) as r:
        ev = ""
        n = 0
        for line in r.iter_lines():
            if line.startswith("event: "):
                ev = line[7:]
            elif line.startswith("data: "):
                now = time.time()
                gap = (now - last) if last else 0
                last = now
                n += 1
                d = json.loads(line[6:])
                if ev == "answer":
                    delta = d.get("delta", "")[:30]
                    print(f"  +{now - t0:5.2f}s 间隔{gap:5.2f}s [{n:3d}] {delta}")
                elif ev == "done":
                    print(f"  +{now - t0:5.2f}s [done] canClose={d.get('canClose')}")
                elif ev == "error":
                    print(f"  [error] {d.get('message')}")
print(f"总耗时 {time.time() - t0:.2f}s, 共 {n} 条事件")

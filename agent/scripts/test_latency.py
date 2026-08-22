"""agent 侧单次请求耗时测试。

场景：
1. context/build 组装耗时（纯逻辑）
2. tool 执行耗时（mock 取数）
3. 单轮对话总耗时（SSE 完整流：JWT→会话→RAG→工具→组装→LLM→落库降级→done）
4. 对话分段时间：首 token 延迟（TTFT）、流式总时长

用法：python scripts/test_latency.py
前置：三服务已启动
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

os.environ["GO_INTERNAL_KEY"] = "test-key"
os.environ["MINIO_SECRET_KEY"] = "minioadmin"
os.environ["JWT_SECRET"] = "test-secret"

JWT_SECRET = "test-secret"
AGENT = "http://127.0.0.1:8000"
CTX = "http://127.0.0.1:8001"
TOOL = "http://127.0.0.1:8002"

ROUNDS = 2  # 每场景次数，取均值（对话场景耗时较长，2 次足够）


def make_token(user_id: str = "u_001") -> str:
    import jwt as pyjwt

    return pyjwt.encode({"user_id": user_id, "exp": int(time.time()) + 3600}, JWT_SECRET, algorithm="HS256")


def timing(name: str, fn, rounds: int = ROUNDS) -> None:
    times = []
    for _ in range(rounds):
        t0 = time.perf_counter()
        fn()
        times.append((time.perf_counter() - t0) * 1000)
    avg = sum(times) / len(times)
    series = "/".join(f"{t:.0f}" for t in times)
    print(f"  {name}: {series} ms ({rounds} 次) | 均值 {avg:.0f} ms")
    return avg


def main() -> int:
    token = make_token()
    c = httpx.Client(timeout=300)

    print(f"== 单次请求耗时（每场景 {ROUNDS} 次）==")

    # 1. context 组装
    def ctx_build():
        c.post(f"{CTX}/context/build",
               json={"user_id": "u_001", "question": "A1 土壤湿度 28% 需要灌溉吗？",
                     "live_data": [{"plot_id": "A1", "metric": "soilMoisture", "value": 28.0,
                                    "unit": "%", "sampled_at": "2026-08-22T08:00:00Z"}]},
               headers={"Authorization": f"Bearer {token}"})

    print("1) context/build 组装（纯逻辑+Go标签降级）")
    timing("context/build", ctx_build)

    # 2. tool 执行
    def tool_call():
        c.post(f"{TOOL}/tools/get_user_plots/execute", json={"args": {}})

    print("2) tool 执行（mock）")
    timing("get_user_plots", tool_call)

    # 3. 单轮对话（SSE 全链路）
    def chat_once():
        with c.stream("POST", f"{AGENT}/agent/chat",
                      json={"session_id": None, "plot_id": "plot_a3",
                            "question": "番茄地土壤湿度 28%，需要灌溉吗？"},
                      headers={"Authorization": f"Bearer {token}"}) as resp:
            for line in resp.iter_lines():
                pass

    print("3) 单轮对话总耗时（SSE 全链路：会话+RAG+工具+组装+LLM+落库降级）")
    timing("agent/chat 全链路", chat_once)

    # 4. 首 token 延迟（TTFT）+ 流式时长
    print("4) 首 token 延迟（TTFT）与流式时长")
    for i in range(ROUNDS):
        t_start = time.perf_counter()
        first_token = None
        t_first = None
        with c.stream("POST", f"{AGENT}/agent/chat",
                      json={"session_id": None, "plot_id": "plot_a3",
                            "question": "A1 地块现在适合灌溉吗？给个简短建议。"},
                      headers={"Authorization": f"Bearer {token}"}) as resp:
            ev = ""
            for line in resp.iter_lines():
                if line.startswith("event: "):
                    ev = line[7:]
                elif line.startswith("data: ") and ev == "answer" and t_first is None:
                    t_first = time.perf_counter()
                    first_token = line[6:][:60]
        ttft = (t_first - t_start) * 1000 if t_first else None
        total = (time.perf_counter() - t_start) * 1000
        print(f"  第{i+1}次: TTFT {ttft:.0f} ms | 总 {total:.0f} ms | 首token={first_token}")

    print("\n== 耗时测试完成 ==")
    return 0


if __name__ == "__main__":
    sys.exit(main())

"""完整测试套件：服务级 + 端到端 + 异常边界（前端未完成，全 API 级）。

用法：python scripts/test_full.py
前置：Redis(6379) + Milvus(19530) + context(8001) + tool(8002) + agent(8000)
"""
import json
import os
import sys
import time
from pathlib import Path

# Windows GBK 控制台兼容：强制 UTF-8 输出
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

import httpx

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

os.environ["GO_INTERNAL_KEY"] = "test-key"
os.environ["MINIO_SECRET_KEY"] = "minioadmin"
os.environ["JWT_SECRET"] = "test-secret"

JWT_SECRET = "test-secret"
AGENT = "http://127.0.0.1:8000"
CTX = "http://127.0.0.1:8001"
TOOL = "http://127.0.0.1:8002"

PASS = 0
FAIL = 0
FAILURES: list[str] = []


def check(name: str, cond: bool, detail: str = "") -> None:
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  [PASS] {name}")
    else:
        FAIL += 1
        FAILURES.append(f"{name}: {detail}")
        print(f"  [FAIL] {name}: {detail}")


def make_token(user_id: str = "u_001") -> str:
    import jwt as pyjwt

    return pyjwt.encode({"user_id": user_id, "exp": int(time.time()) + 3600}, JWT_SECRET, algorithm="HS256")


def sse_chat(question: str, session_id=None, token=None, plot_id="plot_a3", timeout=300) -> dict:
    token = token or make_token()
    payload = {"session_id": session_id, "plot_id": plot_id, "question": question}
    out = {"status": 0, "answer_len": 0, "done": 0, "error": 0, "session_id": None,
           "can_close": None, "closed": None, "sources": None, "error_msg": None}
    with httpx.Client(timeout=timeout) as client:
        with client.stream("POST", f"{AGENT}/agent/chat", json=payload,
                           headers={"Authorization": f"Bearer {token}"}) as resp:
            out["status"] = resp.status_code
            if resp.status_code != 200:
                return out
            ev = ""
            for line in resp.iter_lines():
                if line.startswith("event: "):
                    ev = line[7:]
                elif line.startswith("data: "):
                    d = json.loads(line[6:])
                    if ev == "answer":
                        out["answer_len"] += len(d.get("delta", ""))
                    elif ev == "done":
                        out["done"] += 1
                        out["session_id"] = d.get("sessionId")
                        out["can_close"] = d.get("canClose")
                        out["closed"] = d.get("closed")
                        out["sources"] = d.get("sources")
                    elif ev == "error":
                        out["error"] += 1
                        out["error_msg"] = d.get("message")
    return out


# ============================================================
# 一、服务级
# ============================================================

def test_service_level() -> None:
    print("\n== 一、服务级测试 ==")
    c = httpx.Client(timeout=10)

    for p, name in [(8000, "agent"), (8001, "context"), (8002, "tool")]:
        r = c.get(f"http://127.0.0.1:{p}/healthz")
        check(f"{name}-service healthz", r.status_code == 200 and r.json() == {"ok": True}, r.text)

    # context 组装
    r = c.post(f"{CTX}/context/build",
               json={"user_id": "u_001", "question": "A1 湿度多少？",
                     "live_data": [{"plot_id": "A1", "metric": "soilMoisture", "value": 28.0,
                                    "unit": "%", "sampled_at": "2026-08-22T08:00:00Z"}]},
               headers={"Authorization": f"Bearer {make_token()}"})
    check("context/build 组装", r.status_code == 200 and "[现场数据]" in r.json()["prompt"], r.text[:200])

    # tool 清单 + mock 执行
    r = c.get(f"{TOOL}/tools")
    check("tool 清单 12 个", r.status_code == 200 and len(r.json()) == 12, r.text[:100])
    r = c.post(f"{TOOL}/tools/get_user_plots/execute", json={"args": {}})
    check("tool get_user_plots(mock)", r.status_code == 200 and r.json()["ok"], r.text[:200])
    r = c.post(f"{TOOL}/tools/get_latest_telemetry/execute", json={"args": {"plot_id": "plot_a3"}})
    check("tool telemetry(mock)", r.status_code == 200 and r.json()["ok"], r.text[:200])


# ============================================================
# 二、端到端对话
# ============================================================

def test_e2e_chat() -> None:
    print("\n== 二、端到端对话 ==")
    # 1. 单轮：知识引用（Milvus 里已有番茄知识）
    r1 = sse_chat("番茄地土壤湿度 28%，需要灌溉吗？")
    check("单轮对话 200", r1["status"] == 200, str(r1["status"]))
    check("单轮 done + 有回答", r1["done"] == 1 and r1["answer_len"] > 50, str(r1))
    check("单轮知识引用", r1["sources"] and any(s["type"] == "KNOWLEDGE_DOC" for s in r1["sources"]),
          json.dumps(r1["sources"], ensure_ascii=False))

    # 2. 多轮：新问题回 active
    sid = r1["session_id"]
    r2 = sse_chat("那 B 地块温度多少？", session_id=sid)
    check("多轮新问题回 active", r2["done"] == 1 and r2["answer_len"] > 20 and not r2["closed"], str(r2))

    # 3. 结束判定：规则命中
    r3 = sse_chat("没有了", session_id=sid)
    check("结束意图 closed（省 LLM）", r3["closed"] is True and r3["answer_len"] == 0, str(r3))


# ============================================================
# 三、异常/边界
# ============================================================

def test_edge_cases() -> None:
    print("\n== 三、异常/边界 ==")
    c = httpx.Client(timeout=60)

    # 无效 JWT
    r = c.post(f"{AGENT}/agent/chat", json={"question": "hi"},
               headers={"Authorization": "Bearer bad.token.here"})
    check("无效 JWT → 401", r.status_code == 401, str(r.status_code))

    # 无 JWT
    r = c.post(f"{AGENT}/agent/chat", json={"question": "hi"})
    check("无 JWT → 401", r.status_code == 401, str(r.status_code))

    # 空问题
    r = c.post(f"{AGENT}/agent/chat", json={"question": ""},
               headers={"Authorization": f"Bearer {make_token()}"})
    check("空问题 → 422", r.status_code == 422, str(r.status_code))

    # notify 密钥
    r = c.post(f"{AGENT}/internal/knowledge/notify", json={"doc_id": "x", "event": "uploaded"})
    check("notify 无密钥 → 401", r.status_code == 401, str(r.status_code))
    r = c.post(f"{AGENT}/internal/knowledge/notify", json={"doc_id": "x", "event": "uploaded"},
               headers={"X-Internal-Key": "test-key"})
    check("notify 正确密钥 → 200", r.status_code == 200 and r.json() == {"ok": True}, r.text)

    # notify 非法 event
    r = c.post(f"{AGENT}/internal/knowledge/notify", json={"doc_id": "x", "event": "hack"},
               headers={"X-Internal-Key": "test-key"})
    check("notify 非法 event → 422", r.status_code == 422, str(r.status_code))

    # tool 未知工具
    r = c.post(f"{TOOL}/tools/nope/execute", json={"args": {}})
    check("未知工具 → 404", r.status_code == 404, str(r.status_code))

    # tool 越权参数（additionalProperties:false）
    r = c.post(f"{TOOL}/tools/get_user_plots/execute", json={"args": {"evil": 1}})
    check("非法参数 → 422", r.status_code == 422, str(r.status_code))

    # close 不存在会话
    r = c.post(f"{AGENT}/agent/chat/sessions/nonexist/close",
               headers={"Authorization": f"Bearer {make_token()}"})
    check("close 不存在会话 → 404", r.status_code == 404, str(r.status_code))


# ============================================================

def main() -> int:
    test_service_level()
    test_e2e_chat()
    test_edge_cases()
    print(f"\n=== 完整测试: {PASS} 通过, {FAIL} 失败 ===")
    if FAILURES:
        print("失败项:")
        for f in FAILURES:
            print(f"  - {f}")
    return 1 if FAIL else 0


if __name__ == "__main__":
    sys.exit(main())

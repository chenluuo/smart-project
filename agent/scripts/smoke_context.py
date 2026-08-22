"""context-service 冒烟测试（TestClient，不依赖 Redis/Docker；Go 拉标签失败自动降级）。

用法：python scripts/smoke_context.py
"""
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "context-service"))

# 环境变量占位（config.yaml ${VAR}；Go 不可达时降级默认标签）
os.environ["REDIS_PASSWORD"] = ""
os.environ["GO_INTERNAL_KEY"] = "smoke"
os.environ["JWT_SECRET"] = "smoke"
os.environ["LLM_API_KEY"] = "smoke"
os.environ["MILVUS_TOKEN"] = "smoke"
os.environ["MINIO_SECRET_KEY"] = "smoke"

from fastapi.testclient import TestClient  # noqa: E402

from main import app  # noqa: E402

client = TestClient(app)


def main() -> int:
    # 1. 健康检查
    r = client.get("/healthz")
    assert r.status_code == 200 and r.json() == {"ok": True}
    print("healthz OK")

    # 2. 组装（无 JWT → 401）
    r = client.post("/context/build", json={"user_id": "u_001", "question": "A1 湿度多少？"})
    assert r.status_code == 401, f"无 JWT 应 401，实际 {r.status_code}"
    print("无 JWT → 401 OK")

    # 3. 组装（带 JWT，Go 不可达 → 默认标签；验证 System 段 + 预算裁剪结构）
    r = client.post(
        "/context/build",
        json={
            "user_id": "u_001",
            "question": "A1 土壤湿度 28%，需要灌溉吗？",
            "window_turns": [{"role": "user", "content": "昨天问过温度"}],
            "knowledge_chunks": ["番茄灌溉手册：见干见湿原则。"],
            "memory_chunks": ["用户上个月咨询过黄瓜施肥。"],
            "live_data": [
                {"plot_id": "A1", "metric": "soilMoisture", "value": 28.0, "unit": "%",
                 "sampled_at": "2026-08-22T08:00:00Z"}
            ],
            "budget_tokens": 4000,
        },
        headers={"Authorization": "Bearer test-token"},
    )
    assert r.status_code == 200, f"build 失败: {r.text[:300]}"
    body = r.json()
    prompt = body["prompt"]
    for marker in ("[现场数据]", "[知识库]", "[对话记录]", "[历史对话参考", "[当前问题]"):
        assert marker in prompt, f"缺少段: {marker}"
    assert "番茄灌溉手册" in prompt
    assert "采样 2026-08-22T08:00:00Z" in prompt
    print(f"build OK: used_tokens={body['used_tokens']}, trimmed={body['trimmed']}")
    print("--- prompt 前 300 字 ---")
    print(prompt[:300])

    print("\n=== context-service 冒烟通过 ===")
    return 0


if __name__ == "__main__":
    sys.exit(main())

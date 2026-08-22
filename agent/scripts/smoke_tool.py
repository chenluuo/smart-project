"""tool-service 冒烟测试（mock 模式，不依赖 Go/Redis/Milvus/MinIO）。

用法：python scripts/smoke_tool.py
先设环境变量（config.yaml 的 ${VAR} 占位需要非空值）：
  $env:LLM_API_KEY='x'; $env:REDIS_PASSWORD=''; $env:MILVUS_TOKEN='';
  $env:MINIO_SECRET_KEY='x'; $env:JWT_SECRET='x'; $env:GO_INTERNAL_KEY='x'
"""
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "tool-service"))

# mock 开关（config.yaml tool.mock_go）
os.environ["LLM_API_KEY"] = "smoke"
os.environ["REDIS_PASSWORD"] = ""
os.environ["MILVUS_TOKEN"] = "smoke"
os.environ["MINIO_SECRET_KEY"] = "smoke"
os.environ["JWT_SECRET"] = "smoke"
os.environ["GO_INTERNAL_KEY"] = "smoke"

from fastapi.testclient import TestClient  # noqa: E402

from main import app  # noqa: E402
import tools.impl as impl_mod  # noqa: E402

# mock 开关：patch tools.impl.get_config，令 tool.mock_go=True（不依赖改 config.yaml）
_orig_tool_config = impl_mod.get_config


def _mock_tool_config(service=None):
    cfg = _orig_tool_config(service)
    if service == "tool":
        cfg = dict(cfg)
        cfg["mock_go"] = True
    return cfg


impl_mod.get_config = _mock_tool_config

client = TestClient(app)


def main() -> int:
    # 1. 健康检查
    r = client.get("/healthz")
    assert r.status_code == 200 and r.json() == {"ok": True}, f"healthz: {r.text}"
    print("healthz OK")

    # 2. 工具清单：9 个
    r = client.get("/tools")
    assert r.status_code == 200
    tools = r.json()
    assert len(tools) == 9, f"工具数量 {len(tools)} != 9"
    names = [t["name"] for t in tools]
    print(f"工具清单 OK: {names}")

    # 3. 执行 get_user_plots（mock 数据）
    r = client.post("/tools/get_user_plots/execute", json={"args": {"keyword": "东"}})
    assert r.status_code == 200, f"get_user_plots status={r.status_code} body={r.text[:400]}"
    body = r.json()
    assert body["ok"] is True, body
    plots = body["data"]["plots"]
    assert len(plots) == 1 and plots[0]["name"] == "东侧棚", plots
    print("get_user_plots(mock) OK:", body["data"])

    # 4. 执行 get_latest_telemetry（mock）
    r = client.post("/tools/get_latest_telemetry/execute", json={"args": {"plot_id": "plot_a3"}})
    assert r.status_code == 200 and r.json()["ok"] is True
    print("get_latest_telemetry(mock) OK:", r.json()["data"]["metrics"])

    # 5. 参数校验：缺必填 plot_id → 422
    r = client.post("/tools/get_latest_telemetry/execute", json={"args": {}})
    assert r.status_code == 422, f"缺少必填参数应 422，实际 {r.status_code}"
    print("参数校验 OK（缺 plot_id → 422）")

    # 6. 未知工具 → 404
    r = client.post("/tools/no_such_tool/execute", json={"args": {}})
    assert r.status_code == 404
    print("未知工具 OK（→ 404）")

    # 7. 非法参数（additionalProperties: false）→ 422
    r = client.post("/tools/get_user_plots/execute", json={"args": {"evil": 1}})
    assert r.status_code == 422
    print("非法参数 OK（→ 422）")

    print("\n=== tool-service 冒烟全部通过 ===")
    return 0


if __name__ == "__main__":
    sys.exit(main())

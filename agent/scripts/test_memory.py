"""记忆召回链路测试：摘要写入（Milvus memory）→ 按 user_id 召回 → 隔离验证。

用法：python scripts/test_memory.py
前置：Milvus(19530) + Redis(6379) 已启动
"""
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "ingest-service"))
sys.path.insert(0, str(ROOT / "agent-service"))

os.environ["GO_INTERNAL_KEY"] = "test-key"
os.environ["MINIO_SECRET_KEY"] = "minioadmin"
os.environ["JWT_SECRET"] = "test-secret"

from shared.milvus_client import _get_conn, ensure_collections  # noqa: E402
from shared.redis_client import get_redis  # noqa: E402
from summarizer import build_summary  # noqa: E402
from rag import memory_recall  # noqa: E402


def main() -> int:
    ensure_collections()
    rds = get_redis().client()

    # 准备两个用户的窗口数据（user_a 有番茄记忆，user_b 有黄瓜记忆）
    user_a, user_b = "user_a", "user_b"
    rds.set("ctx:" + user_a, __import__("json").dumps([
        {"role": "user", "content": "我的番茄地湿度低，我打算浇水 10 分钟"},
        {"role": "assistant", "content": "好的，番茄见干见湿，浇水后注意观察"},
    ], ensure_ascii=False))
    rds.set("ctx:" + user_b, __import__("json").dumps([
        {"role": "user", "content": "黄瓜需要每天浇水吗？"},
        {"role": "assistant", "content": "黄瓜定植后每天浇水一次"},
    ], ensure_ascii=False))

    # 1. 生成摘要并写入 memory collection
    r_a = build_summary("sess_a1", user_a)
    r_b = build_summary("sess_b1", user_b)
    print("摘要生成:", r_a["status"], "|", r_b["status"])
    assert r_a["status"] == "ok" and r_b["status"] == "ok", (r_a, r_b)

    # 2. 按 user_id 召回（user_a 问番茄）
    hits = memory_recall(user_a, "我上次说要给番茄地做什么？")
    print(f"user_a 召回 {len(hits)} 条:")
    for h in hits:
        print(f"  [{h['score']:.4f}] {h['summary'][:40]}")
    assert hits, "user_a 应召回记忆"
    assert "番茄" in hits[0]["summary"], "user_a 应命中番茄记忆"

    # 3. 隔离验证：user_a 检索黄瓜问题，不应召回 user_b 的黄瓜记忆
    hits_b = memory_recall(user_a, "黄瓜怎么浇水？")
    for h in hits_b:
        assert "黄瓜" not in h["summary"], f"越权召回 user_b 记忆: {h['summary']}"
    print(f"隔离验证 OK：user_a 查黄瓜问题未召回 user_b 记忆（{len(hits_b)} 条均非黄瓜）")

    print("\n=== 记忆召回链路测试通过 ===")
    return 0


if __name__ == "__main__":
    sys.exit(main())

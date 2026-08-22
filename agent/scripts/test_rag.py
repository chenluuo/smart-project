"""RAG 完整链路测试：知识写入（embedding → Milvus）→ 检索。

用法：python scripts/test_rag.py
前置：Milvus(19530) 已启动 + config 环境变量
"""
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

os.environ["GO_INTERNAL_KEY"] = "test-key"
os.environ["MINIO_SECRET_KEY"] = "minioadmin"
os.environ["JWT_SECRET"] = "test-secret"

from shared.embedding import embed  # noqa: E402
from shared.milvus_client import _get_conn, ensure_collections, search_knowledge, upsert_documents  # noqa: E402


def main() -> int:
    # 删除旧的错误 schema collection（int64 主键），重建为 VARCHAR 主键
    conn = _get_conn()
    for name in ("knowledge", "memory"):
        if conn.has_collection(name):
            conn.drop_collection(name)
    ensure_collections()

    # 1. 写入 2 条知识（embedding → Milvus knowledge collection）
    texts = [
        "番茄灌溉原则：见干见湿，土壤湿度低于 30% 时建议灌溉 10 分钟。",
        "黄瓜种植标准：定植后每天浇水一次，追施复合肥。",
    ]
    vecs = embed(texts)
    rows = [
        {
            "id": f"doc_001:v1:{i}",
            "vector": vecs[i],
            "doc_id": "doc_001",
            "title": "番茄温室灌溉建议",
            "version": 1,
            "category": "灌溉",
            "source": "knowledge/tomato.pdf",
            "content": texts[i],
        }
        for i in range(len(texts))
    ]
    upsert_documents(rows, kind="knowledge")
    print("写入 2 条知识 OK")

    # 2. 检索（相关性问题）
    q_vec = embed(["番茄地什么时候浇水？"])[0]
    hits = search_knowledge(q_vec, top_k=2)
    assert hits, "检索无结果"
    for h in hits:
        print(f"  [{h['score']:.4f}] {h['title']}: {h['content'][:35]}")

    # 3. 相关性断言：番茄问题应命中番茄文档（score 更高）
    assert "番茄" in hits[0]["content"], "最相关应是番茄文档"
    print("RAG 检索链路 OK（最相关命中）")
    return 0


if __name__ == "__main__":
    sys.exit(main())

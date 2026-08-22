"""assemble.assemble 单元测试：预算裁剪顺序（现场>知识>短期>记忆）。"""
import sys
import unittest
from pathlib import Path

CONTEXT_DIR = Path(__file__).resolve().parent.parent / "context-service"
sys.path.insert(0, str(CONTEXT_DIR))

from assemble import _approx_tokens, assemble  # noqa: E402
from prompts import build_system_segment  # noqa: E402


def _system_tokens() -> int:
    """按实际 System 段文本估算 token（测试自洽，不依赖硬编码值）。"""
    return _approx_tokens(build_system_segment({}))


def _long_text(n: int) -> str:
    """生成约 n 字的文本（用于撑预算）。"""
    return "田" * n


class TestAssemble(unittest.TestCase):
    def _sections_present(self, prompt: str) -> dict[str, bool]:
        return {
            "live": "[现场数据]" in prompt,
            "knowledge": "[知识库]" in prompt,
            "window": "[对话记录]" in prompt,
            "memory": "[历史对话参考" in prompt,
        }

    def test_budget_enough_keeps_all(self):
        result = assemble(
            profile={},
            window_turns=[{"role": "user", "content": "今天要灌溉吗"}],
            knowledge_chunks=["番茄灌溉手册：见干见湿。"],
            memory_chunks=["用户上个月问过黄瓜施肥。"],
            live_data=[{"plot_id": "A1", "metric": "soilMoisture", "value": 28, "unit": "%", "sampled_at": "2026-08-22T08:00:00Z"}],
            question="A1 需要灌溉吗？",
            budget_tokens=10000,
        )
        present = self._sections_present(result["prompt"])
        self.assertTrue(all(present.values()), present)
        self.assertEqual(result["trimmed"], [])
        self.assertIn("当前问题", result["prompt"])

    def test_memory_trimmed_first_when_budget_tight(self):
        # 预算 = System + 现场 + 知识 + 短期窗口 + 记忆 - 1（挤掉长期记忆）
        # → 只有 memory 被裁，验证"长期记忆最低优先"
        sys_t = _system_tokens()
        live = [{"plot_id": "A1", "metric": "soilMoisture", "value": 28, "unit": "%", "sampled_at": "2026-08-22T08:00:00Z"}]
        window = [{"role": "user", "content": _long_text(200)}]
        kb = [_long_text(200)]
        memory = [_long_text(200)]

        # 按各块实际文本估算 token（与 assemble 内部算法一致，测试自洽）
        live_t = _approx_tokens("[现场数据]\nA1 soilMoisture=28.0%（采样 2026-08-22T08:00:00Z）")
        kb_t = _approx_tokens("[知识库]\n" + kb[0])
        win_t = _approx_tokens("[对话记录]\nuser: " + window[0]["content"])
        mem_t = _approx_tokens("[历史对话参考（非当前事实，仅供参考）]\n" + memory[0])
        budget = sys_t + live_t + kb_t + win_t + mem_t - 1  # 只差 1 token，挤掉 memory

        result = assemble(
            profile={},
            window_turns=window,
            knowledge_chunks=kb,
            memory_chunks=memory,
            live_data=live,
            question="A1 需要灌溉吗？",
            budget_tokens=budget,
        )
        present = self._sections_present(result["prompt"])
        self.assertTrue(present["live"], "现场数据最高优先")
        self.assertTrue(present["knowledge"], "知识第二优先")
        self.assertTrue(present["window"], "短期窗口第三优先")
        self.assertFalse(present["memory"], "长期记忆最低优先，应最先被裁")
        self.assertIn("memory", result["trimmed"])

    def test_live_data_kept_when_everything_else_trimmed(self):
        # 预算只够 System + 现场
        result = assemble(
            profile={},
            window_turns=[{"role": "user", "content": _long_text(500)}],
            knowledge_chunks=[_long_text(500)],
            memory_chunks=[_long_text(500)],
            live_data=[{"plot_id": "A1", "metric": "soilMoisture", "value": 28, "unit": "%", "sampled_at": "2026-08-22T08:00:00Z"}],
            question="A1 需要灌溉吗？",
            budget_tokens=600,
        )
        present = self._sections_present(result["prompt"])
        self.assertTrue(present["live"], "现场数据最高优先，必须保留")
        self.assertIn("memory", result["trimmed"])
        self.assertIn("window", result["trimmed"])

    def test_knowledge_chunk_truncated_when_oversize(self):
        # 知识片段超大：预算不足时截断保开头，而不是整体丢弃
        result = assemble(
            profile={},
            window_turns=[],
            knowledge_chunks=[_long_text(3000)],
            memory_chunks=[],
            live_data=[],
            question="灌溉标准？",
            budget_tokens=2000,
        )
        self.assertIn("knowledge", result["trimmed"])
        self.assertIn("[知识库]", result["prompt"])

    def test_live_data_format_carries_sampled_at(self):
        result = assemble(
            profile={},
            window_turns=[],
            knowledge_chunks=[],
            memory_chunks=[],
            live_data=[{"plot_id": "A1", "metric": "soilMoisture", "value": 28, "unit": "%", "sampled_at": "2026-08-22T08:00:00Z"}],
            question="湿度多少？",
            budget_tokens=10000,
        )
        self.assertIn("采样 2026-08-22T08:00:00Z", result["prompt"])


if __name__ == "__main__":
    unittest.main()

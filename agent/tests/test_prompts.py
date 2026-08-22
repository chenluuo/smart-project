"""prompts.build_system_segment 单元测试：case 选取 + 默认兜底。"""
import sys
import unittest
from pathlib import Path

CONTEXT_DIR = Path(__file__).resolve().parent.parent / "context-service"
sys.path.insert(0, str(CONTEXT_DIR))

from prompts import (  # noqa: E402
    BASE_SYSTEM,
    RELIANCE_DATA,
    RELIANCE_DOCUMENT,
    RELIANCE_EXPERIENCE,
    STYLE_CASUAL,
    STYLE_PLAIN,
    STYLE_PROFESSIONAL,
    build_system_segment,
)


class TestBuildSystemSegment(unittest.TestCase):
    def test_empty_profile_uses_defaults(self):
        seg = build_system_segment({})
        self.assertIn(STYLE_CASUAL, seg)
        self.assertIn(RELIANCE_DOCUMENT, seg)

    def test_none_tags_use_defaults(self):
        seg = build_system_segment(
            {"interaction_style": None, "knowledge_reliance": None}
        )
        self.assertIn(STYLE_CASUAL, seg)
        self.assertIn(RELIANCE_DOCUMENT, seg)

    def test_style_plain_selected(self):
        seg = build_system_segment(
            {"interaction_style": "plain", "knowledge_reliance": "data"}
        )
        self.assertIn(STYLE_PLAIN, seg)
        self.assertNotIn(STYLE_CASUAL, seg)
        self.assertNotIn(STYLE_PROFESSIONAL, seg)

    def test_style_professional_selected(self):
        seg = build_system_segment(
            {"interaction_style": "professional", "knowledge_reliance": "experience"}
        )
        self.assertIn(STYLE_PROFESSIONAL, seg)
        self.assertNotIn(STYLE_PLAIN, seg)

    def test_reliance_data_selected(self):
        seg = build_system_segment(
            {"interaction_style": "casual", "knowledge_reliance": "data"}
        )
        self.assertIn(RELIANCE_DATA, seg)
        self.assertNotIn(RELIANCE_DOCUMENT, seg)
        self.assertNotIn(RELIANCE_EXPERIENCE, seg)

    def test_unknown_values_fallback_to_default(self):
        seg = build_system_segment(
            {"interaction_style": "weird", "knowledge_reliance": "unknown"}
        )
        self.assertIn(STYLE_CASUAL, seg)  # 未知风格 → 默认 casual
        self.assertIn(RELIANCE_DOCUMENT, seg)  # 未知依据 → 默认 document

    def test_base_system_always_present(self):
        seg = build_system_segment({"interaction_style": "plain"})
        self.assertIn(BASE_SYSTEM, seg)

    def test_does_not_reference_persona(self):
        # persona 已移除（不参考 05）：不应出现经营身份/订单/托管等字样
        seg = build_system_segment({})
        for banned in ("订单", "托管", "经营身份", "persona"):
            self.assertNotIn(banned, seg)


if __name__ == "__main__":
    unittest.main()

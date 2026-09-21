import json
import os
import tempfile
import unittest

from komodo import config


class ConfigTests(unittest.TestCase):
    def test_defaults_load_without_files(self):
        with tempfile.TemporaryDirectory() as root:
            cfg = config.Config.load(root)
            self.assertEqual(cfg.role("builder")["provider"], "claude")
            self.assertTrue(cfg.is_protected("main"))
            self.assertTrue(cfg.is_protected("release/1.2"))
            self.assertTrue(cfg.is_protected("refs/heads/master"))
            self.assertFalse(cfg.is_protected("feat/x"))

    def test_team_then_local_overlay(self):
        with tempfile.TemporaryDirectory() as root:
            with open(os.path.join(root, "komodo.json"), "w") as handle:
                json.dump({"profile": "thinking", "severity_floor": "medium"}, handle)
            os.makedirs(os.path.join(root, ".komodo"))
            with open(os.path.join(root, ".komodo", "local.json"), "w") as handle:
                json.dump({"profiles": {"thinking": {"roles": {"builder": {"model": "haiku"}}}}}, handle)
            cfg = config.Config.load(root)
            self.assertEqual(cfg.get("severity_floor"), "medium")
            self.assertEqual(cfg.role("builder")["model"], "haiku")
            self.assertEqual(cfg.role("builder")["provider"], "claude")
            self.assertEqual(cfg.role("builder", "fast")["model"], "sonnet")

    def test_unknown_provider_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            with open(os.path.join(root, "komodo.json"), "w") as handle:
                json.dump({"profiles": {"fast": {"tiers": {"standard": {"provider": "gemini"}}}}}, handle)
            with self.assertRaises(config.ConfigError):
                config.Config.load(root)

    def test_dotted_get(self):
        with tempfile.TemporaryDirectory() as root:
            cfg = config.Config.load(root)
            self.assertEqual(cfg.get("comments.trivial_lines"), 8)
            self.assertIsNone(cfg.get("nope.nothing"))

    def test_tiers_and_role_overrides(self):
        with tempfile.TemporaryDirectory() as root:
            cfg = config.Config.load(root, {"account": {"detect": False, "plan": "max"}})
            self.assertEqual(cfg.role("scout")["model"], "haiku")
            self.assertEqual(cfg.role("reviewer", "thinking")["model"], "opus")
            self.assertEqual(cfg.role("reviewer")["min_diff_lines"], 150)
            self.assertEqual(cfg.role("summarizer")["provider"], "ollama")
            self.assertEqual(cfg.role("builder", "local")["provider"], "ollama")
            self.assertEqual(cfg.profile_names(), ["fast", "local", "thinking"])

    def test_plan_sets_the_model_ceiling_and_the_turn_cap(self):
        with tempfile.TemporaryDirectory() as root:
            pro = config.Config.load(root, {"account": {"detect": False, "plan": "pro"}})
            big = config.Config.load(root, {"account": {"detect": False, "plan": "max"}})
            self.assertEqual(pro.role("reviewer", "thinking")["model"], "sonnet")
            self.assertEqual(big.role("reviewer", "thinking")["model"], "opus")
            self.assertLess(pro.role("builder")["max_turns"], big.role("builder")["max_turns"])
            self.assertGreater(big.worker_timeout(400_000), pro.worker_timeout(400_000))

    def test_explicit_turn_cap_survives_derivation(self):
        with tempfile.TemporaryDirectory() as root:
            overrides = {"account": {"detect": False, "plan": "max"}, "profiles": {"fast": {"tiers": {"standard": {"max_turns": 9}}}}}
            cfg = config.Config.load(root, overrides)
            self.assertEqual(cfg.role("builder")["max_turns"], 9)

    def test_model_ceiling_can_be_switched_off(self):
        with tempfile.TemporaryDirectory() as root:
            cfg = config.Config.load(root, {"account": {"detect": False, "plan": "pro", "model_ceiling": False}})
            self.assertEqual(cfg.role("reviewer", "thinking")["model"], "opus")


if __name__ == "__main__":
    unittest.main()

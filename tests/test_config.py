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
                json.dump({"profiles": {"thinking": {"builder": {"model": "haiku"}}}}, handle)
            cfg = config.Config.load(root)
            self.assertEqual(cfg.get("severity_floor"), "medium")
            self.assertEqual(cfg.role("builder")["model"], "haiku")
            self.assertEqual(cfg.role("builder")["provider"], "claude")
            self.assertEqual(cfg.role("builder", "fast")["model"], "sonnet")

    def test_unknown_provider_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            with open(os.path.join(root, "komodo.json"), "w") as handle:
                json.dump({"profiles": {"fast": {"builder": {"provider": "gemini"}}}}, handle)
            with self.assertRaises(config.ConfigError):
                config.Config.load(root)

    def test_dotted_get(self):
        with tempfile.TemporaryDirectory() as root:
            cfg = config.Config.load(root)
            self.assertEqual(cfg.get("comments.trivial_lines"), 8)
            self.assertIsNone(cfg.get("nope.nothing"))


if __name__ == "__main__":
    unittest.main()

import json
import os
import tempfile
import unittest

from komodo import doctor, install, standards
from komodo.adapters import claude

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


class DoctorTests(unittest.TestCase):
    def test_repo_is_clean(self):
        problems = doctor.check_references(REPO) + doctor.check_policy(REPO) + doctor.check_roles(REPO)
        self.assertEqual(problems, [])

    def test_planted_dangling_reference(self):
        with tempfile.TemporaryDirectory() as root:
            with open(os.path.join(root, "README.md"), "w") as handle:
                handle.write("See `komodo/nothing.py` and run /workflow-loop today.\n")
            problems = doctor.check_references(root)
            self.assertTrue(any("komodo/nothing.py" in p for p in problems))
            self.assertTrue(any("workflow-loop" in p for p in problems))

    def test_planted_personal_key_in_policy(self):
        with tempfile.TemporaryDirectory() as root:
            os.makedirs(os.path.join(root, "komodo", "adapters", "claude", "hooks"))
            with open(os.path.join(root, "komodo", "adapters", "claude", "settings.policy.json"), "w") as handle:
                json.dump({"effortLevel": "high", "hooks": {"PreToolUse": [{"hooks": [{"command": "python3 ~/.claude/hooks/missing.py"}]}]}}, handle)
            problems = doctor.check_policy(root)
            self.assertTrue(any("personal key" in p for p in problems))
            self.assertTrue(any("missing.py" in p for p in problems))

    def test_checked_in_claude_code_is_flagged(self):
        with tempfile.TemporaryDirectory() as root:
            os.makedirs(os.path.join(root, "komodo", "adapters", "claude", "hooks"))
            with open(os.path.join(root, "komodo", "adapters", "claude", "settings.policy.json"), "w") as handle:
                json.dump({"hooks": {}}, handle)
            os.makedirs(os.path.join(root, "claude-code"))
            self.assertTrue(any("claude-code/ exists" in p for p in doctor.check_policy(root)))


class InstallTests(unittest.TestCase):
    def test_merge_keeps_personal_keys(self):
        existing = {"effortLevel": "medium", "permissions": {"allow": ["old"]}, "theme": "dark"}
        policy = {"permissions": {"allow": ["new"]}, "hooks": {"PreToolUse": []}}
        merged = install.merge_settings(existing, policy)
        self.assertEqual(merged["permissions"], {"allow": ["new"]})
        self.assertEqual(merged["effortLevel"], "medium")
        self.assertEqual(merged["theme"], "dark")
        self.assertIn("hooks", merged)

    def test_rewrite_hook_commands(self):
        policy = {"hooks": {"PreToolUse": [{"hooks": [{"command": "python3 ~/.claude/hooks/guard.py"}]}]}}
        out = install.rewrite_hook_commands(policy, "/x/hooks", ["py", "-3"])
        command = out["hooks"]["PreToolUse"][0]["hooks"][0]["command"]
        self.assertTrue(command.startswith("py -3 "), command)
        self.assertTrue(command.rstrip("'\"").endswith("guard.py"), command)
        self.assertNotIn("~", command)

    def test_install_dry_run_into_temp(self):
        with tempfile.TemporaryDirectory() as target:
            logs = []
            code = install.install(target, dry_run=True, log=logs.append)
            self.assertEqual(code, 0)
            self.assertEqual(os.listdir(target), [])
            self.assertTrue(any("settings.json" in line for line in logs))

    def test_rewrite_leaves_a_binary_hook_uninterpreted(self):
        policy = {"hooks": {"PreToolUse": [{"hooks": [{"command": "~/.claude/hooks/guard"}]}]}}
        out = install.rewrite_hook_commands(policy, "/x/hooks", ["python3"])
        command = out["hooks"]["PreToolUse"][0]["hooks"][0]["command"]
        self.assertEqual(command, "/x/hooks/guard")

    def test_install_real_into_temp_preserves_personal(self):
        with tempfile.TemporaryDirectory() as target:
            with open(os.path.join(target, "settings.json"), "w") as handle:
                json.dump({"effortLevel": "high", "permissions": {"allow": []}}, handle)
            code = install.install(target, dry_run=False, log=lambda line: None)
            self.assertEqual(code, 0)
            with open(os.path.join(target, "settings.json")) as handle:
                settings = json.load(handle)
            self.assertEqual(settings["effortLevel"], "high")
            guard = settings["hooks"]["PreToolUse"][0]["hooks"][0]["command"]
            expected = "guard" if claude.host_guard() else "guard.py"
            self.assertTrue(guard.endswith(expected), guard)
            self.assertEqual(os.path.isfile(os.path.join(target, "hooks", "guard")), bool(claude.host_guard()))
            self.assertTrue(os.path.isfile(os.path.join(target, "AGENTS.md")))
            self.assertTrue(os.path.isfile(os.path.join(target, "standards", "go.md")))
            self.assertTrue(os.path.isdir(os.path.join(target, "skills", "komodo")))
            self.assertTrue(os.path.isfile(os.path.join(target, "agents", "builder.md")))
            self.assertTrue(os.path.isfile(os.path.join(target, "CLAUDE.local.md")))
            self.assertFalse(os.path.isdir(os.path.join(target, "seeds")))
            self.assertFalse(os.path.islink(os.path.join(target, "hooks")))


class StandardsTests(unittest.TestCase):
    def test_names_for_paths(self):
        names = standards.names_for(["internal/api/handler.go", "web/src/App.tsx", "Dockerfile"], "builder")
        for expected in ("go", "api-design", "api-security", "typescript", "react", "ui-web", "docker", "comments"):
            self.assertIn(expected, names)
        self.assertIn("rust", standards.names_for(["src/main.rs"]))
        self.assertIn("zig", standards.names_for(["build.zig"]))

    def test_load_clips_and_falls_back(self):
        text = standards.load(["go"], cap_chars=500)
        self.assertIn("truncated", text)
        self.assertIn("No language standard", standards.load([]))

    def test_comment_convention(self):
        self.assertIn("godoc", standards.comment_convention(["a.go", "b.go", "c.py"]))
        self.assertIn("public function", standards.comment_convention(["x.txt"]))


if __name__ == "__main__":
    unittest.main()

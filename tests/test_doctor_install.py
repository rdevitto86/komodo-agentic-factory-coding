import json
import os
import tempfile
import unittest

from komodo import doctor, install, standards

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


class DoctorTests(unittest.TestCase):
    def test_repo_is_clean(self):
        problems = doctor.check_references(REPO) + doctor.check_policy(REPO) + doctor.check_skills(REPO)
        self.assertEqual(problems, [])

    def test_planted_dangling_reference(self):
        with tempfile.TemporaryDirectory() as root:
            os.makedirs(os.path.join(root, "claude-code", "skills", "komodo"))
            with open(os.path.join(root, "claude-code", "skills", "komodo", "SKILL.md"), "w") as handle:
                handle.write("---\nname: komodo\n---\n")
            with open(os.path.join(root, "README.md"), "w") as handle:
                handle.write("See `komodo/nothing.py` and run /workflow-loop today.\n")
            problems = doctor.check_references(root)
            self.assertTrue(any("komodo/nothing.py" in p for p in problems))
            self.assertTrue(any("workflow-loop" in p for p in problems))

    def test_planted_personal_key_in_policy(self):
        with tempfile.TemporaryDirectory() as root:
            os.makedirs(os.path.join(root, "claude-code", "hooks"))
            with open(os.path.join(root, "claude-code", "settings.policy.json"), "w") as handle:
                json.dump({"effortLevel": "high", "hooks": {"PreToolUse": [{"hooks": [{"command": "python3 ~/.claude/hooks/missing.py"}]}]}}, handle)
            problems = doctor.check_policy(root)
            self.assertTrue(any("personal key" in p for p in problems))
            self.assertTrue(any("missing.py" in p for p in problems))

    def test_skill_checks(self):
        with tempfile.TemporaryDirectory() as root:
            folder = os.path.join(root, "claude-code", "skills", "bad")
            os.makedirs(folder)
            open(os.path.join(folder, "SKILL.md.off"), "w").close()
            with open(os.path.join(folder, "SKILL.md"), "w") as handle:
                handle.write("---\nname: other\nweird: 1\n---\n")
            problems = doctor.check_skills(root)
            self.assertTrue(any(".off" in p for p in problems))
            self.assertTrue(any("unknown frontmatter" in p for p in problems))
            self.assertTrue(any("does not match" in p for p in problems))


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

    def test_install_real_into_temp_preserves_personal(self):
        with tempfile.TemporaryDirectory() as target:
            with open(os.path.join(target, "settings.json"), "w") as handle:
                json.dump({"effortLevel": "high", "permissions": {"allow": []}}, handle)
            code = install.install(target, dry_run=False, log=lambda line: None)
            self.assertEqual(code, 0)
            settings = json.load(open(os.path.join(target, "settings.json")))
            self.assertEqual(settings["effortLevel"], "high")
            self.assertIn("guard.py", settings["hooks"]["PreToolUse"][0]["hooks"][0]["command"])
            self.assertTrue(os.path.isfile(os.path.join(target, "AGENTS.md")))
            self.assertTrue(os.path.isfile(os.path.join(target, "standards", "go.md")))
            self.assertTrue(os.path.isdir(os.path.join(target, "skills", "komodo")))
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

    def test_every_standard_has_a_skill_and_vice_versa(self):
        skills = {name[len("standards-"):] for name in os.listdir(os.path.join(REPO, "claude-code", "skills")) if name.startswith("standards-")}
        self.assertEqual(skills, set(standards.available()))

    def test_comment_convention(self):
        self.assertIn("godoc", standards.comment_convention(["a.go", "b.go", "c.py"]))
        self.assertIn("public function", standards.comment_convention(["x.txt"]))


if __name__ == "__main__":
    unittest.main()

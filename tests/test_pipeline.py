import io
import json
import os
import subprocess
import tempfile
import unittest

from komodo import pipeline, tasks
from komodo.config import Config
from komodo.workers import Result, Worker

BACKLOG = """# Backlog

### [TG-01.1] Greeting module
```yaml
type: feat
```

#### [TSK-01.1.1] Write the greeting [P: H] [TODO]
```yaml
files: [pkg/greet.py]
done_when:
  - python3 -c "import pkg.greet as g; assert g.greet('x') == 'hello x'"
```

#### [TSK-01.1.2] Write the farewell [P: M] [TODO]
```yaml
files: [other/bye.py]
done_when:
  - python3 -c "import other.bye as b; assert b.bye('x') == 'bye x'"
```

#### [TSK-01.1.3] Wire both [P: M] [TODO]
```yaml
files: [app.py]
done_when:
  - python3 app.py
depends_on: [TSK-01.1.1, TSK-01.1.2]
```
"""

FILES = {
    "TSK-01.1.1": ("pkg/greet.py", '"""Greetings."""\n\n\ndef greet(name):\n    """Returns a greeting for name."""\n    return "hello " + name\n'),
    "TSK-01.1.2": ("other/bye.py", '"""Farewells."""\n\n\ndef bye(name):\n    """Returns a farewell for name."""\n    return "bye " + name\n'),
    "TSK-01.1.3": ("app.py", '"""Entry point."""\nimport pkg.greet, other.bye\nprint(pkg.greet.greet("a"), other.bye.bye("b"))\n'),
}


class FakeBuilder(Worker):
    """Writes the file a task expects, like a builder would, and returns a DONE envelope."""

    name = "fake"
    calls = []

    def invoke(self, brief):
        FakeBuilder.calls.append(brief)
        if brief.role == "reviewer":
            return Result(ok=True, data={"summary": "fine", "findings": [{"severity": "low", "class": "simplify", "file": "app.py", "line": 1, "title": "could inline", "detail": "one-liner", "fix": "inline"}]}, cost_usd=0.01)
        task_id = brief.task_id
        if task_id in FILES:
            path, body = FILES[task_id]
            full = os.path.join(brief.cwd, path)
            os.makedirs(os.path.dirname(full) or brief.cwd, exist_ok=True)
            for pkg in ("pkg", "other"):
                init = os.path.join(brief.cwd, pkg, "__init__.py")
                if os.path.isdir(os.path.dirname(init)) and not os.path.exists(init):
                    open(init, "w").close()
            with open(full, "w") as handle:
                handle.write(body)
            return Result(ok=True, data={"result": "DONE", "summary": "wrote %s" % path, "changed": [{"path": path, "what": "created"}], "verified": [], "notes": []}, cost_usd=0.02)
        return Result(ok=False, error="unknown task")


def make_repo(root):
    def git(*args):
        subprocess.run(["git", *args], cwd=root, check=True, capture_output=True)

    git("init", "-q", "-b", "main")
    git("config", "user.email", "t@example.com")
    git("config", "user.name", "t")
    git("config", "commit.gpgsign", "false")
    with open(os.path.join(root, "BACKLOG.md"), "w") as handle:
        handle.write(BACKLOG)
    os.makedirs(os.path.join(root, "pkg"))
    os.makedirs(os.path.join(root, "other"))
    open(os.path.join(root, "pkg", "__init__.py"), "w").close()
    open(os.path.join(root, "other", "__init__.py"), "w").close()
    with open(os.path.join(root, "komodo.json"), "w") as handle:
        json.dump({"profile": "fast", "worker_timeout_s": 60, "severity_floor": "high", "profiles": {"fast": {"reviewer": {"min_diff_lines": 0}}}}, handle)
    git("add", "-A")
    git("commit", "-q", "-m", "init")


class PipelineTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        make_repo(self.tmp.name)
        FakeBuilder.calls = []
        self.logs = []

    def tearDown(self):
        self.tmp.cleanup()

    def test_dry_run_spawns_nothing_and_plans_waves(self):
        config = Config.load(self.tmp.name)
        run = pipeline.Pipeline(self.tmp.name, config, dry_run=True, log=self.logs.append, worker_factory=lambda *a, **k: FakeBuilder())
        state = run.run()
        self.assertEqual(FakeBuilder.calls, [])
        joined = "\n".join(self.logs)
        self.assertIn("wave 1: TSK-01.1.1 [pkg], TSK-01.1.2 [other]", joined)
        self.assertIn("wave 2: TSK-01.1.3", joined)
        self.assertIn("dry-run builder for TSK-01.1.1", joined)
        self.assertEqual(state.branch, "feat/greeting-module")
        self.assertFalse(os.path.exists(os.path.join(self.tmp.name, ".komodo", "runs")))

    def test_end_to_end_with_fake_workers(self):
        config = Config.load(self.tmp.name)
        run = pipeline.Pipeline(self.tmp.name, config, log=self.logs.append, worker_factory=lambda *a, **k: FakeBuilder())
        state = run.run()
        statuses = {tid: record.status for tid, record in state.tasks.items()}
        self.assertEqual(statuses, {"TSK-01.1.1": "DONE", "TSK-01.1.2": "DONE", "TSK-01.1.3": "DONE"})
        roles = [brief.role for brief in FakeBuilder.calls]
        self.assertEqual(roles.count("builder"), 3)
        self.assertEqual(roles.count("reviewer"), 1)
        git = run.git
        self.assertEqual(git.current_branch(), "feat/greeting-module")
        subjects = git.log_subjects("main")
        self.assertIn("feat: Write the greeting", subjects)
        self.assertIn("chore: close out TG-01.1", subjects)
        self.assertEqual(git.worktrees(), [])
        backlog = tasks.load(os.path.join(self.tmp.name, "BACKLOG.md"))
        self.assertEqual(backlog.task("TSK-01.1.1").status, "DONE")
        self.assertEqual(len([t for t in backlog.group("TG-01.1").tasks if t.status == "TODO"]), 1, "low finding filed as a new TODO task")
        changelog = open(os.path.join(self.tmp.name, "CHANGELOG.md")).read()
        self.assertIn("### Added", changelog)
        self.assertIn("Write the greeting", changelog)
        report = open(run.store.report_path(state.run_id)).read()
        self.assertIn("## ✅ Successful Changes", report)
        self.assertTrue(any("unpushed" in note or "gh is not" in note or "nothing landed" in note or "pushed" in note for note in state.notes) or state.pr_url == "")
        self.assertGreater(state.cost_usd, 0)
        for brief in FakeBuilder.calls:
            if brief.role == "builder":
                self.assertNotIn("GH_TOKEN", brief.env or {})
                self.assertIn("done_when", brief.prompt)

    def test_refuses_dirty_tree_and_unfinished_run(self):
        config = Config.load(self.tmp.name)
        with open(os.path.join(self.tmp.name, "dirty.txt"), "w") as handle:
            handle.write("x")
        run = pipeline.Pipeline(self.tmp.name, config, log=self.logs.append, worker_factory=lambda *a, **k: FakeBuilder())
        with self.assertRaises(pipeline.PipelineError):
            run.run()

    def test_blocked_task_blocks_dependents(self):
        class FailingBuilder(FakeBuilder):
            def invoke(self, brief):
                if brief.task_id == "TSK-01.1.1":
                    return Result(ok=False, error="boom")
                return super().invoke(brief)

        config = Config.load(self.tmp.name)
        run = pipeline.Pipeline(self.tmp.name, config, log=self.logs.append, worker_factory=lambda *a, **k: FailingBuilder())
        state = run.run()
        self.assertEqual(state.tasks["TSK-01.1.1"].status, "BLOCKED")
        self.assertEqual(state.tasks["TSK-01.1.3"].status, "BLOCKED")
        self.assertEqual(state.tasks["TSK-01.1.2"].status, "DONE")
        self.assertIn("TSK-01.1.3", state.blocked)


class ChangelogTests(unittest.TestCase):
    def test_write_changelog_creates_and_appends(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            pipeline._write_changelog(path, "feat", ["First thing"])
            pipeline._write_changelog(path, "fix", ["Second thing"])
            pipeline._write_changelog(path, "feat", ["Third thing"])
            text = open(path).read()
            self.assertEqual(text.count("## [Unreleased]"), 1)
            self.assertEqual(text.count("### Added"), 1)
            self.assertIn("- First thing\n- Third thing", text)
            self.assertIn("### Fixed\n- Second thing", text)

    def test_section_anchor(self):
        text = "# Doc\n\n## Refunds\nbody\n\n## Other\nx\n"
        self.assertEqual(pipeline._section(text, "refunds"), "## Refunds\nbody\n")
        self.assertEqual(pipeline._section(text, "missing"), "")


if __name__ == "__main__":
    unittest.main()

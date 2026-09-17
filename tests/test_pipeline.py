import json
import os
import shlex
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

from komodo import pipeline, render, tasks
from komodo.config import Config
from komodo.workers import Result, Worker

PY = sys.executable if " " not in sys.executable else (shlex.quote(sys.executable) if os.name != "nt" else '"%s"' % sys.executable)

BACKLOG_TEMPLATE = """# Backlog

### [TG-01.1] Greeting module
```yaml
type: feat
```

#### [TSK-01.1.1] Write the greeting [P: H] [TODO]
```yaml
files: [pkg/greet.py]
done_when:
  - PYEXE -c "import pkg.greet as g; assert g.greet('x') == 'hello x'"
```

#### [TSK-01.1.2] Write the farewell [P: M] [TODO]
```yaml
files: [other/bye.py]
done_when:
  - PYEXE -c "import other.bye as b; assert b.bye('x') == 'bye x'"
```

#### [TSK-01.1.3] Wire both [P: M] [TODO]
```yaml
files: [app.py]
done_when:
  - PYEXE app.py
depends_on: [TSK-01.1.1, TSK-01.1.2]
```
"""

BACKLOG = BACKLOG_TEMPLATE.replace("PYEXE", PY)

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
            return Result(ok=True, data={"summary": "fine", "blast_radius": "low-med", "blast_radius_why": "one module, nothing imports it", "findings": [{"severity": "low", "class": "simplify", "file": "app.py", "line": 1, "title": "could inline", "detail": "one-liner", "fix": "inline"}]}, cost_usd=0.01)
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
    with open(os.path.join(root, "CHANGELOG.md"), "w") as handle:
        handle.write("# Changelog\n\n## [Unreleased]\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- the first thing\n")
    os.makedirs(os.path.join(root, "pkg"))
    os.makedirs(os.path.join(root, "other"))
    open(os.path.join(root, "pkg", "__init__.py"), "w").close()
    open(os.path.join(root, "other", "__init__.py"), "w").close()
    with open(os.path.join(root, "komodo.json"), "w") as handle:
        json.dump({"profile": "fast", "worker_timeout_s": 60, "severity_floor": "high", "profiles": {"fast": {"roles": {"reviewer": {"min_diff_lines": 0}}}}}, handle)
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
        self.assertEqual(state.blast_radius, "low-med")
        self.assertIn("low-med", render.report(state, "Greeting", {}, []))
        git = run.git
        self.assertEqual(git.current_branch(), "feat/greeting-module")
        subjects = git.log_subjects("main")
        self.assertIn("feat: Write the greeting", subjects)
        self.assertIn("chore: close out TG-01.1", subjects)
        self.assertEqual(git.worktrees(), [])
        backlog = tasks.load(os.path.join(self.tmp.name, "BACKLOG.md"))
        self.assertIsNone(backlog.task("TSK-01.1.1"), "a completed task is dropped from the backlog")
        self.assertEqual(len([t for t in backlog.group("TG-01.1").tasks if t.status == "TODO"]), 1, "low finding filed as a new TODO task")
        self.assertEqual([t.id for t in backlog.tasks if t.id.startswith("TSK-01.1.")], [t.id for t in backlog.tasks])
        with open(os.path.join(self.tmp.name, "CHANGELOG.md"), encoding="utf-8") as handle:
            changelog = handle.read()
        self.assertIn("### Added", changelog)
        self.assertIn("Write the greeting", changelog)
        self.assertIn("## [0.2.0] \u2014 ", changelog, "a feat group cuts a minor version")
        self.assertNotIn("- Write the greeting", pipeline._section(changelog, "unreleased"))
        with open(run.store.report_path(state.run_id), encoding="utf-8") as handle:
            report = handle.read()
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

    def _pipeline(self):
        return pipeline.Pipeline(self.tmp.name, Config.load(self.tmp.name), log=self.logs.append,
                                 worker_factory=lambda *a, **k: FakeBuilder())

    def test_tag_pending_tags_the_merged_version_on_base(self):
        run = self._pipeline()
        run._tag_pending("main")
        self.assertTrue(run.git.tag_exists("v0.1.0"))
        self.assertIn("tagged v0.1.0 on main", "\n".join(self.logs))
        self.logs = []
        run._tag_pending("main")
        self.assertEqual(self.logs, [], "a second call is a no-op once the tag exists")

    def test_tag_pending_tags_a_gap_below_the_newest_heading(self):
        path = os.path.join(self.tmp.name, "CHANGELOG.md")
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("# Changelog\n\n## [Unreleased]\n\n## [0.3.0] \u2014 2026-03-01\n\n### Added\n- third\n\n## [0.2.0] \u2014 2026-02-01\n\n### Added\n- second\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
        run = self._pipeline()
        run.git.run("add", "-A")
        run.git.run("commit", "-q", "-m", "changelog")
        run.git.run("tag", "-a", "v0.3.0", "-m", "release 0.3.0")
        run._tag_pending("main")
        self.assertTrue(run.git.tag_exists("v0.2.0"), "a gap below the newest heading is tagged")
        self.assertTrue(run.git.tag_exists("v0.1.0"))

    def test_tag_pending_skips_a_never_released_version(self):
        path = os.path.join(self.tmp.name, "CHANGELOG.md")
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("# Changelog\n\n## [Unreleased]\n\n## [0.2.0] \u2014 2026-02-01\n\n> Never released as its own tag; superseded by 0.3.0.\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
        run = self._pipeline()
        run.git.run("add", "-A")
        run.git.run("commit", "-q", "-m", "changelog")
        run._tag_pending("main")
        self.assertFalse(run.git.tag_exists("v0.2.0"))
        self.assertTrue(run.git.tag_exists("v0.1.0"))

    def test_tag_pending_is_a_no_op_off_base(self):
        run = self._pipeline()
        run.git.create_branch("feat/somewhere-else")
        run._tag_pending("main")
        self.assertFalse(run.git.tag_exists("v0.1.0"))
        self.assertEqual(self.logs, [])

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
            with open(path, encoding="utf-8") as handle:
                text = handle.read()
            self.assertEqual(text.count("## [Unreleased]"), 1)
            self.assertEqual(text.count("### Added"), 1)
            self.assertIn("- First thing\n- Third thing", text)
            self.assertIn("### Fixed\n- Second thing", text)

    def test_cut_version_bumps_patch_and_opens_a_fresh_unreleased(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("# Changelog\n\n## [Unreleased]\n\n### Fixed\n- a fix\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
            cut = pipeline._cut_version(path)
            with open(path, encoding="utf-8") as handle:
                text = handle.read()
            self.assertTrue(cut.startswith("0.1.1 (patch bump:"), cut)
            self.assertIn("## [0.1.1] \u2014 ", text)
            self.assertEqual(text.count("## [Unreleased]"), 1)
            self.assertFalse(render.has_unreleased_entries(text))

    def test_cut_version_takes_a_major_from_a_breaking_bullet(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("# Changelog\n\n## [Unreleased]\n\n### Changed\n- **Breaking** the flag is gone\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
            cut = pipeline._cut_version(path)
            self.assertTrue(cut.startswith("1.0.0 (major bump:"), cut)

    def test_write_changelog_refuses_to_drop_a_released_version(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("# Changelog\n\n## [Unreleased]\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
            with mock.patch.object(pipeline, "_write") as writer:
                with mock.patch.object(pipeline.render, "assert_preserved", side_effect=ValueError("would drop 0.1.0")):
                    with self.assertRaises(ValueError):
                        pipeline._write_changelog(path, "fix", ["a fix"])
            writer.assert_not_called()

    def test_write_preserving_refuses_a_write_that_drops_a_version(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            before = "# Changelog\n\n## [Unreleased]\n\n## [0.2.0] \u2014 2026-02-01\n\n### Added\n- two\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- one\n"
            with open(path, "w", encoding="utf-8") as handle:
                handle.write(before)
            with self.assertRaises(ValueError) as caught:
                pipeline._write_preserving(path, before, ["# Changelog", "", "## [Unreleased]", "", "## [0.2.0] \u2014 2026-02-01"])
            self.assertIn("0.1.0", str(caught.exception))
            with open(path, encoding="utf-8") as handle:
                self.assertEqual(handle.read(), before, "the file is untouched when the write is refused")

    def test_cut_version_keeps_every_released_version(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("# Changelog\n\n## [Unreleased]\n\n### Fixed\n- a fix\n\n## [0.2.0] \u2014 2026-02-01\n\n### Added\n- two\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- one\n")
            pipeline._cut_version(path)
            with open(path, encoding="utf-8") as handle:
                self.assertEqual(render.released_versions(handle.read())[1:], ["0.2.0", "0.1.0"])

    def test_section_anchor(self):
        text = "# Doc\n\n## Refunds\nbody\n\n## Other\nx\n"
        self.assertEqual(pipeline._section(text, "refunds"), "## Refunds\nbody\n")
        self.assertEqual(pipeline._section(text, "missing"), "")


if __name__ == "__main__":
    unittest.main()

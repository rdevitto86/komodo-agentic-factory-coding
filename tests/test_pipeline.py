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
version: 1.1.0
```

#### [TSK-01.1.1] Write the greeting [P: H] [READY]
```yaml
files: [pkg/greet.py]
done_when:
  - PYEXE -c "import pkg.greet as g; assert g.greet('x') == 'hello x'"
```

#### [TSK-01.1.2] Write the farewell [P: M] [READY]
```yaml
files: [other/bye.py]
done_when:
  - PYEXE -c "import other.bye as b; assert b.bye('x') == 'bye x'"
```

#### [TSK-01.1.3] Wire both [P: M] [READY]
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
    with open(os.path.join(root, ".gitignore"), "w") as handle:
        handle.write(".komodo/\n")
    with open(os.path.join(root, "BACKLOG.md"), "w") as handle:
        handle.write(BACKLOG)
    with open(os.path.join(root, "CHANGELOG.md"), "w") as handle:
        handle.write("# Changelog\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- the first thing\n")
    os.makedirs(os.path.join(root, "pkg"))
    os.makedirs(os.path.join(root, "other"))
    open(os.path.join(root, "pkg", "__init__.py"), "w").close()
    open(os.path.join(root, "other", "__init__.py"), "w").close()
    os.makedirs(os.path.join(root, ".komodo"), exist_ok=True)
    with open(os.path.join(root, ".komodo", "config.json"), "w") as handle:
        json.dump({"profile": "fast", "worker_timeout_s": 60, "severity_floor": "high", "account": {"detect": False, "plan": "max"}, "profiles": {"fast": {"roles": {"reviewer": {"min_diff_lines": 0}}}}}, handle)
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

    def _pipeline(self, **overrides):
        """A pipeline over the temp repo, with config overrides applied."""
        config = Config.load(self.tmp.name, overrides or None)
        return pipeline.Pipeline(self.tmp.name, config, log=self.logs.append, worker_factory=lambda *a, **k: FakeBuilder())

    def test_a_spent_usage_window_stops_the_next_wave(self):
        line = self._pipeline()
        line.state = pipeline.RunState(run_id="r", group_id="TG-01.1", branch="feat/x", base="main", profile="fast", started=0.0)
        line.state.rate_limit = {"unifiedWindows": {"five_hour": {"utilization": 0.97, "resetsAt": 1_790_017_800}}}
        hold = line.rate_limit_hold()
        self.assertIn("five_hour", hold)
        self.assertIn("97%", hold)
        line.state.rate_limit = {"unifiedWindows": {"five_hour": {"utilization": 0.5, "resetsAt": 1}}}
        self.assertEqual(line.rate_limit_hold(), "")

    def test_an_overage_account_is_never_held_back(self):
        line = self._pipeline()
        line.state = pipeline.RunState(run_id="r", group_id="TG-01.1", branch="feat/x", base="main", profile="fast", started=0.0)
        line.state.rate_limit = {"isUsingOverage": True, "unifiedWindows": {"seven_day": {"utilization": 0.99, "resetsAt": 1}}}
        self.assertEqual(line.rate_limit_hold(), "")

    def test_each_file_gets_its_own_budget_rather_than_a_share_of_one(self):
        backlog = tasks.load(os.path.join(self.tmp.name, "BACKLOG.md"))
        task = backlog.group("TG-01.1").tasks[0]
        task.fields["files"] = ["pkg/greet.py", "other/bye.py", "app.py"]
        body = "x" * 9000
        for path in task.files:
            full = os.path.join(self.tmp.name, path)
            os.makedirs(os.path.dirname(full) or self.tmp.name, exist_ok=True)
            with open(full, "w") as handle:
                handle.write(body)
        line = self._pipeline()
        line.backlog = backlog
        slots = line.task_slots(task, self.tmp.name)
        self.assertNotIn("truncated", slots["files"])

    def test_the_repair_attempt_is_handed_what_the_first_attempt_wrote(self):
        line = self._pipeline()
        with open(os.path.join(self.tmp.name, "pkg", "greet.py"), "w") as handle:
            handle.write("# half a greeting\n")
        diff = line._attempt_diff(self.tmp.name)
        self.assertIn("half a greeting", diff)
        self.assertIn("What the previous attempt already changed", diff)

    def test_the_worker_timeout_grows_with_the_task(self):
        line = self._pipeline()
        self.assertGreater(line.config.worker_timeout(500_000), line.config.worker_timeout(0))

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
        with self.assertRaises(pipeline.PipelineError):
            run.run()  # the fixture has no remote, so publish reports it instead of passing silently
        state = run.state
        assert state is not None
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
        self.assertEqual(len([t for t in backlog.group("TG-01.1").tasks if t.status == "READY"]), 1, "low finding filed as a new READY task")
        self.assertEqual([t.id for t in backlog.tasks if t.id.startswith("TSK-01.1.")], [t.id for t in backlog.tasks])
        with open(os.path.join(self.tmp.name, "CHANGELOG.md"), encoding="utf-8") as handle:
            changelog = handle.read()
        self.assertIn("### Added", changelog)
        self.assertIn("Write the greeting", changelog)
        declared = tasks.load(os.path.join(self.tmp.name, "BACKLOG.md")).group("TG-01.1").version
        self.assertEqual(declared, "1.1.0")
        self.assertIn("## [%s] \u2014 " % declared, changelog, "the heading is the version the backlog declared")
        self.assertNotIn("Unreleased", changelog)
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
            handle.write("# Changelog\n\n## [0.3.0] \u2014 2026-03-01\n\n### Added\n- third\n\n## [0.2.0] \u2014 2026-02-01\n\n### Added\n- second\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
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
            handle.write("# Changelog\n\n## [0.2.0] \u2014 2026-02-01\n\n> Never released as its own tag; superseded by 0.3.0.\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
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
        with self.assertRaises(pipeline.PipelineError):
            run.run()
        state = run.state
        assert state is not None
        self.assertEqual(state.tasks["TSK-01.1.1"].status, "BLOCKED")
        self.assertEqual(state.tasks["TSK-01.1.3"].status, "BLOCKED")
        self.assertEqual(state.tasks["TSK-01.1.2"].status, "DONE")
        self.assertIn("TSK-01.1.3", state.blocked)


class VersionSyncTests(unittest.TestCase):
    """The declared version, the changelog heading, and the git tag are one value copied, never three guesses."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        make_repo(self.tmp.name)
        FakeBuilder.calls = []
        self.logs = []

    def tearDown(self):
        self.tmp.cleanup()

    def _git(self, *args):
        return subprocess.run(["git", *args], cwd=self.tmp.name, check=True, capture_output=True, text=True).stdout.strip()

    def test_backlog_version_reaches_the_changelog_and_the_tag_unchanged(self):
        declared = tasks.load(os.path.join(self.tmp.name, "BACKLOG.md")).group("TG-01.1").version
        config = Config.load(self.tmp.name)
        run = pipeline.Pipeline(self.tmp.name, config, log=self.logs.append, worker_factory=lambda *a, **k: FakeBuilder())
        with self.assertRaises(pipeline.PipelineError):
            run.run()  # no remote in the fixture; the close-out commit still landed

        changelog = os.path.join(self.tmp.name, "CHANGELOG.md")
        with open(changelog, encoding="utf-8") as handle:
            text = handle.read()
        self.assertEqual(render.newest_version(text), declared, "close-out wrote the declared version")
        self.assertNotIn("Unreleased", text)

        # the human merges, then preflight tags whatever merged
        branch = run.state.branch
        self._git("switch", "-q", "main")
        self._git("merge", "--no-ff", "-q", "-m", "merge %s" % branch, branch)
        second = pipeline.Pipeline(self.tmp.name, Config.load(self.tmp.name), log=self.logs.append,
                                   worker_factory=lambda *a, **k: FakeBuilder())
        second._tag_pending("main")

        self.assertTrue(second.git.tag_exists("v" + declared), "preflight tagged the merged version")
        tagged = self._git("rev-list", "-n1", "v" + declared)
        self.assertEqual(tagged, self._git("rev-parse", "HEAD"), "the tag points at what merged")
        with open(changelog, encoding="utf-8") as handle:
            merged_text = handle.read()
        self.assertEqual(render.newest_version(merged_text), declared)
        tags = self._git("tag", "--list", "v*").split()
        self.assertEqual(render.changelog_drift(merged_text, tags), [], "no drift between the changelog and the tags")

    def test_a_second_group_on_the_same_version_shares_one_heading(self):
        changelog = os.path.join(self.tmp.name, "CHANGELOG.md")
        pipeline._write_changelog(changelog, "feat", ["One"], "1.1.0")
        pipeline._write_changelog(changelog, "fix", ["Two"], "1.1.0")
        with open(changelog, encoding="utf-8") as handle:
            text = handle.read()
        self.assertEqual(render.released_versions(text).count("1.1.0"), 1)
        self.assertEqual(render.newest_version(text), "1.1.0")

class PublishBlockerTests(unittest.TestCase):
    """Every publish path that ends without a pull request names itself and fails the run."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        make_repo(self.tmp.name)
        self.remote = tempfile.TemporaryDirectory()
        subprocess.run(["git", "init", "-q", "--bare", self.remote.name], check=True, capture_output=True)
        FakeBuilder.calls = []
        self.logs = []

    def tearDown(self):
        self.tmp.cleanup()
        self.remote.cleanup()

    def _pipeline(self):
        return pipeline.Pipeline(self.tmp.name, Config.load(self.tmp.name), log=self.logs.append,
                                 worker_factory=lambda *a, **k: FakeBuilder())

    def _add_remote(self):
        subprocess.run(["git", "remote", "add", "origin", self.remote.name], cwd=self.tmp.name, check=True, capture_output=True)

    def _head(self):
        out = subprocess.run(["git", "rev-parse", "HEAD"], cwd=self.tmp.name, check=True, capture_output=True, text=True)
        return out.stdout.strip()

    def test_no_remote_fails_the_run_and_names_the_reason(self):
        run = self._pipeline()
        with self.assertRaises(pipeline.PipelineError) as caught:
            run.run()
        self.assertIn("no remote 'origin'", str(caught.exception))
        self.assertEqual(run.state.publish_blocker, str(caught.exception))
        self.assertEqual(run.state.pr_url, "")

    def test_unauthenticated_gh_pushes_then_fails_the_run(self):
        self._add_remote()
        run = self._pipeline()
        with mock.patch.object(pipeline.pr, "available", return_value=False):
            with self.assertRaises(pipeline.PipelineError) as caught:
                run.run()
        self.assertIn("gh is not authenticated", str(caught.exception))
        self.assertEqual(run.state.pr_url, "")
        pushed = subprocess.run(["git", "branch", "--list", "feat/greeting-module"], cwd=self.remote.name, capture_output=True, text=True)
        self.assertIn("feat/greeting-module", pushed.stdout, "the branch reached the remote before the run failed")

    def test_failed_verify_leaves_no_close_out_commit(self):
        run = self._pipeline()
        self._add_remote()
        with mock.patch.object(pipeline.Pipeline, "verify", return_value=False):
            with self.assertRaises(pipeline.PipelineError) as caught:
                run.run()
        self.assertIn("verify failed", str(caught.exception))
        subjects = run.git.log_subjects("main")
        self.assertNotIn("chore: close out TG-01.1", subjects, "a failed verify never writes a close-out commit")

    def test_nothing_landed_fails_the_run(self):
        class DeadBuilder(FakeBuilder):
            def invoke(self, brief):
                if brief.role == "reviewer":
                    return super().invoke(brief)
                return Result(ok=False, error="boom")

        self._add_remote()
        run = pipeline.Pipeline(self.tmp.name, Config.load(self.tmp.name), log=self.logs.append,
                                worker_factory=lambda *a, **k: DeadBuilder())
        with self.assertRaises(pipeline.PipelineError) as caught:
            run.run()
        self.assertIn("nothing to publish", str(caught.exception))

    def test_the_happy_path_still_opens_a_pull_request(self):
        self._add_remote()
        run = self._pipeline()
        with mock.patch.object(pipeline.pr, "available", return_value=True), \
             mock.patch.object(pipeline.pr, "view", return_value=None), \
             mock.patch.object(pipeline.pr, "existing_labels", return_value=[]), \
             mock.patch.object(pipeline.pr, "create", return_value="https://example.invalid/pr/1") as created:
            state = run.run()
        self.assertEqual(state.pr_url, "https://example.invalid/pr/1")
        self.assertEqual(state.publish_blocker, "")
        self.assertEqual(created.call_count, 1)
        self.assertIn("chore: close out TG-01.1", run.git.log_subjects("main"))

    def test_the_report_names_the_missing_pull_request(self):
        run = self._pipeline()
        with self.assertRaises(pipeline.PipelineError):
            run.run()
        self.assertIn("**No pull request.**", render.report(run.state, "Greeting", {}, []))

class ChangelogTests(unittest.TestCase):
    """The declared version is copied into the changelog, never inferred from the bullets."""

    def test_write_changelog_opens_the_declared_version_and_appends_into_it(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            pipeline._write_changelog(path, "feat", ["First thing"], "1.1.0")
            pipeline._write_changelog(path, "fix", ["Second thing"], "1.1.0")
            pipeline._write_changelog(path, "feat", ["Third thing"], "1.1.0")
            with open(path, encoding="utf-8") as handle:
                text = handle.read()
            self.assertEqual(text.count("## [1.1.0]"), 1, "three groups on one version share one heading")
            self.assertNotIn("Unreleased", text)
            self.assertEqual(text.count("### Added"), 1)
            self.assertIn("- First thing\n- Third thing", text)
            self.assertIn("### Fixed\n- Second thing", text)

    def test_a_new_version_opens_above_the_released_ones(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("# Changelog\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
            pipeline._write_changelog(path, "fix", ["a fix"], "0.1.1")
            with open(path, encoding="utf-8") as handle:
                text = handle.read()
            self.assertLess(text.index("## [0.1.1]"), text.index("## [0.1.0]"))
            self.assertIn("### Added\n- first", text, "the released version survives")

    def test_the_date_separator_matches_the_changelog_and_never_drifts(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("# Changelog\n\n## [0.1.0] - 2026-01-01\n\n### Added\n- first\n")
            pipeline._write_changelog(path, "fix", ["a fix"], "0.1.1")
            with open(path, encoding="utf-8") as handle:
                text = handle.read()
            self.assertIn("## [0.1.1] - ", text)
            self.assertEqual(render.changelog_drift(text, ["v0.1.0", "v0.1.1"]), [])

    def test_write_changelog_refuses_to_drop_a_released_version(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("# Changelog\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- first\n")
            with mock.patch.object(pipeline, "_write") as writer:
                with mock.patch.object(pipeline.render, "assert_preserved", side_effect=ValueError("would drop 0.1.0")):
                    with self.assertRaises(ValueError):
                        pipeline._write_changelog(path, "fix", ["a fix"], "0.1.1")
            writer.assert_not_called()

    def test_write_preserving_refuses_a_write_that_drops_a_version(self):
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "CHANGELOG.md")
            before = "# Changelog\n\n## [0.2.0] \u2014 2026-02-01\n\n### Added\n- two\n\n## [0.1.0] \u2014 2026-01-01\n\n### Added\n- one\n"
            with open(path, "w", encoding="utf-8") as handle:
                handle.write(before)
            with self.assertRaises(ValueError) as caught:
                pipeline._write_preserving(path, before, ["# Changelog", "", "## [0.2.0] \u2014 2026-02-01"])
            self.assertIn("0.1.0", str(caught.exception))
            with open(path, encoding="utf-8") as handle:
                self.assertEqual(handle.read(), before, "the file is untouched when the write is refused")


import contextlib
import io
import os
import subprocess
import tempfile
import unittest

from komodo import __main__ as cli

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def make_repo(root):
    """Inits a git repo at root with an initial commit on main."""
    def git(*args):
        subprocess.run(["git", *args], cwd=root, check=True, capture_output=True)

    git("init", "-q", "-b", "main")
    git("config", "user.email", "t@example.com")
    git("config", "user.name", "t")
    with open(os.path.join(root, "a.txt"), "w") as handle:
        handle.write("a\n")
    git("add", "a.txt")
    git("commit", "-q", "-m", "init")
    return git


def write_backlog(root, extra=""):
    """A minimal, valid BACKLOG.md under root."""
    with open(os.path.join(root, "BACKLOG.md"), "w") as handle:
        handle.write(
            "# Backlog\n\n"
            "## [EPIC-01] Now\n*Goal*\n\n"
            "### [TG-01.1] Refunds\n"
            "```yaml\ntype: feat\n```\n\n"
            "#### [TSK-01.1.1] Add refund handler [P: H] [TODO]\n"
            "```yaml\nfiles: [a.py]\ndone_when: [python3 -c 'pass']\n```\n"
            + extra
        )


@contextlib.contextmanager
def chdir(path):
    """Restores the cwd on exit."""
    previous = os.getcwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(previous)


class RepoRootTests(unittest.TestCase):
    def test_git_repo_returns_toplevel(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            nested = os.path.join(root, "sub")
            os.makedirs(nested)
            self.assertEqual(os.path.realpath(cli.repo_root(nested)), os.path.realpath(root))

    def test_non_repo_falls_back_to_cwd(self):
        with tempfile.TemporaryDirectory() as root:
            self.assertEqual(os.path.realpath(cli.repo_root(root)), os.path.realpath(root))


class LoadConfigTests(unittest.TestCase):
    def test_defaults_when_no_komodo_json(self):
        with tempfile.TemporaryDirectory() as root:
            config = cli.load_config(root)
            self.assertEqual(config.data["profile"], "fast")

    def test_profile_override(self):
        with tempfile.TemporaryDirectory() as root:
            config = cli.load_config(root, "thinking")
            self.assertEqual(config.data["profile"], "thinking")


class TasksCommandTests(unittest.TestCase):
    def test_lint_reports_no_problems_on_a_clean_backlog(self):
        with tempfile.TemporaryDirectory() as root:
            write_backlog(root)
            with chdir(root):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = cli.main(["tasks", "lint"])
            self.assertEqual(code, 0)
            self.assertIn("0 problem(s)", out.getvalue())

    def test_list_prints_groups_and_tasks(self):
        with tempfile.TemporaryDirectory() as root:
            write_backlog(root)
            with chdir(root):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = cli.main(["tasks", "list"])
            self.assertEqual(code, 0)
            self.assertIn("TG-01.1", out.getvalue())
            self.assertIn("TSK-01.1.1", out.getvalue())

    def test_add_appends_a_task_and_prints_its_id(self):
        with tempfile.TemporaryDirectory() as root:
            write_backlog(root)
            with chdir(root):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = cli.main(["tasks", "add", "TG-01.1", "Wire refund route", "--files", "b.py", "--done-when", "python3 -c 'pass'"])
            self.assertEqual(code, 0)
            self.assertIn("TSK-01.1.2", out.getvalue())
            with open(os.path.join(root, "BACKLOG.md")) as handle:
                self.assertIn("Wire refund route", handle.read())

    def test_migrate_prints_to_stdout_without_write(self):
        with tempfile.TemporaryDirectory() as root:
            write_backlog(root)
            with chdir(root):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = cli.main(["tasks", "migrate"])
            self.assertEqual(code, 0)
            self.assertIn("Refunds", out.getvalue())
            with open(os.path.join(root, "BACKLOG.md")) as handle:
                self.assertNotIn("done_when", handle.read().split("TSK-01.1.1")[0])

    def test_missing_backlog_exits_with_usage_error(self):
        with tempfile.TemporaryDirectory() as root:
            with chdir(root):
                with self.assertRaises(SystemExit) as ctx:
                    cli.main(["tasks", "lint"])
            self.assertEqual(ctx.exception.code, 2)


class CommentsCommandTests(unittest.TestCase):
    def test_clean_file_has_no_findings(self):
        with tempfile.TemporaryDirectory() as root:
            with open(os.path.join(root, "ok.py"), "w") as handle:
                handle.write('"""Module."""\n\n\ndef public(a):\n    """Returns a."""\n    return a\n')
            with chdir(root):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = cli.main(["comments", "check", "ok.py", "--all"])
            self.assertEqual(code, 0)


class DoctorCommandTests(unittest.TestCase):
    def test_this_repo_reports_zero_problems(self):
        with chdir(REPO):
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                code = cli.main(["doctor", "--no-git"])
        self.assertEqual(code, 0)
        self.assertIn("0 problem(s)", out.getvalue())


class HooksCommandTests(unittest.TestCase):
    def test_status_reports_unset_hooks_path(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                code = cli.main(["hooks", "status", root])
            self.assertEqual(code, 0)
            self.assertIn("hooksPath=<unset>", out.getvalue())


class StatusCommandTests(unittest.TestCase):
    def test_no_runs_on_disk(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            with chdir(root):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = cli.main(["status"])
            self.assertEqual(code, 0)
            self.assertIn("runs: 0 on disk", out.getvalue())


class ReleaseCommandTests(unittest.TestCase):
    def test_no_changelog_is_an_error(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            with chdir(root):
                code = cli.main(["release", "--dry-run"])
            self.assertEqual(code, 2)

    def test_dry_run_prints_the_tag_it_would_push(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            with open(os.path.join(root, "CHANGELOG.md"), "w") as handle:
                handle.write("# Changelog\n\n## [1.2.0] - 2026-01-01\n\n### Fixed\n\n- x\n")
            with chdir(root):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = cli.main(["release", "--dry-run"])
            self.assertEqual(code, 0)
            self.assertIn("would tag v1.2.0", out.getvalue())


class InstallCommandTests(unittest.TestCase):
    def test_dry_run_into_temp_target(self):
        with tempfile.TemporaryDirectory() as target:
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                code = cli.main(["install", "--target", target, "--dry-run"])
            self.assertEqual(code, 0)
            self.assertEqual(os.listdir(target), [])


class MainTests(unittest.TestCase):
    def test_version_flag_exits_zero(self):
        with self.assertRaises(SystemExit) as ctx:
            cli.main(["--version"])
        self.assertEqual(ctx.exception.code, 0)

    def test_no_command_is_a_usage_error(self):
        with self.assertRaises(SystemExit) as ctx:
            cli.main([])
        self.assertEqual(ctx.exception.code, 2)

    def test_config_error_is_caught_and_reported(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            with open(os.path.join(root, "komodo.json"), "w") as handle:
                handle.write('{"profile": "nope"}')
            with chdir(root):
                code = cli.main(["status"])
            self.assertEqual(code, 2)


if __name__ == "__main__":
    unittest.main()

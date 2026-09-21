import os
import subprocess
import tempfile
import unittest

from komodo import gitops

PROTECTED = ["main", "master", "release/*"]


def make_repo(root):
    def git(*args):
        subprocess.run(["git", *args], cwd=root, check=True, capture_output=True)

    git("init", "-q", "-b", "main")
    git("config", "user.email", "t@example.com")
    git("config", "user.name", "t")
    git("config", "commit.gpgsign", "false")
    with open(os.path.join(root, "README.md"), "w") as handle:
        handle.write("hi\n")
    git("add", "README.md")
    git("commit", "-q", "-m", "init")


class RefusalTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        make_repo(self.tmp.name)
        self.git = gitops.Git(self.tmp.name, PROTECTED)

    def tearDown(self):
        self.tmp.cleanup()

    def test_commit_on_protected_branch_refused(self):
        with open(os.path.join(self.tmp.name, "a.txt"), "w") as handle:
            handle.write("x")
        with self.assertRaises(gitops.GitRefused):
            self.git.commit("feat: x")

    def test_trailer_refused(self):
        self.git.create_branch("feat/x")
        with self.assertRaises(gitops.GitRefused):
            self.git.commit("feat: x\n\nCo-Authored-By: Bot <b@x>")
        with self.assertRaises(gitops.GitRefused):
            self.git.commit("feat: x\n\n\U0001F916 Generated with Tool")

    def test_bad_branch_names_refused(self):
        for name in ("main", "release/1", "Feature/x", "feat/Bad_Name", "nope"):
            with self.assertRaises(gitops.GitRefused):
                self.git.create_branch(name)

    def test_push_refusals_never_reach_git(self):
        for spec in ("main", "release/2", "+feat/x", ":feat/x", "feat/x:main", "refs/heads/master"):
            with self.assertRaises(gitops.GitRefused):
                self.git.push(spec)

    def test_merge_into_protected_refused(self):
        with self.assertRaises(gitops.GitRefused):
            self.git.merge("feat/anything")

    def test_delete_protected_refused(self):
        with self.assertRaises(gitops.GitRefused):
            self.git.delete_branch("main")


class FlowTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        make_repo(self.tmp.name)
        self.git = gitops.Git(self.tmp.name, PROTECTED)

    def tearDown(self):
        self.tmp.cleanup()

    def test_branch_commit_and_worktree_merge(self):
        self.git.create_branch("feat/group")
        base = self.git.head()
        wt = os.path.join(self.tmp.name, ".komodo", "wt", "task-a")
        self.git.worktree_add(wt, "feat/group-task-a", "feat/group")
        with open(os.path.join(wt, "a.txt"), "w") as handle:
            handle.write("a\n")
        sha = self.git.commit(gitops.commit_message("feat", "add a", ["wrote a.txt"]), cwd=wt)
        self.assertTrue(sha)
        self.assertTrue(self.git.merge("feat/group-task-a"))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp.name, "a.txt")))
        self.git.worktree_remove(wt, "feat/group-task-a")
        self.assertFalse(os.path.isdir(wt))
        self.assertFalse(self.git.branch_exists("feat/group-task-a"))
        self.assertEqual(self.git.worktrees(), [])
        self.assertEqual(self.git.log_subjects("main"), ["feat: add a"])
        self.assertGreater(self.git.diff_lines("main"), 0)
        self.assertEqual(self.git.changed_files("main"), ["a.txt"])
        self.assertNotEqual(base, self.git.head())

    def test_commit_returns_none_when_nothing_changed(self):
        self.git.create_branch("feat/empty")
        self.assertIsNone(self.git.commit("feat: nothing"))

    def test_conflict_reports_files(self):
        self.git.create_branch("feat/left")
        with open(os.path.join(self.tmp.name, "README.md"), "w") as handle:
            handle.write("left\n")
        self.git.commit("feat: left")
        self.git.switch("main")
        self.git.run("switch", "-c", "feat/right")
        with open(os.path.join(self.tmp.name, "README.md"), "w") as handle:
            handle.write("right\n")
        self.git.commit("feat: right")
        self.assertFalse(self.git.merge("feat/left"))
        self.assertEqual(self.git.conflicted_files(), ["README.md"])
        self.assertEqual(self.git.files_with_markers(["README.md"]), ["README.md"])
        self.git.merge_abort()
        self.assertTrue(self.git.is_clean())

    def test_merged_branches_and_prune(self):
        self.git.create_branch("feat/done")
        with open(os.path.join(self.tmp.name, "b.txt"), "w") as handle:
            handle.write("b\n")
        self.git.commit("feat: b")
        self.git.switch("main")
        self.git.run("merge", "--ff", "feat/done")
        self.assertEqual(self.git.merged_branches("main"), ["feat/done"])
        self.git.delete_branch("feat/done")
        self.assertFalse(self.git.branch_exists("feat/done"))


class EnvTests(unittest.TestCase):
    def test_worker_env_strips_credentials(self):
        env = gitops.worker_env({"GH_TOKEN": "x", "GITHUB_TOKEN": "y", "PATH": "/bin", "SSH_AUTH_SOCK": "/tmp/s"})
        self.assertNotIn("GH_TOKEN", env)
        self.assertNotIn("GITHUB_TOKEN", env)
        self.assertNotIn("SSH_AUTH_SOCK", env)
        self.assertEqual(env["GIT_CONFIG_KEY_0"], "credential.helper")
        self.assertEqual(env["GIT_CONFIG_VALUE_0"], "")
        self.assertEqual(env["GIT_TERMINAL_PROMPT"], "0")
        self.assertIn("BatchMode=yes", env["GIT_SSH_COMMAND"])
        self.assertTrue(os.path.isdir(env["GH_CONFIG_DIR"]))

    def test_commit_message_shape(self):
        message = gitops.commit_message("fix", "Stop the crash.", ["guard nil", "  ", "add test"])
        self.assertEqual(message, "fix: Stop the crash\n\n- guard nil\n- add test")
        self.assertLessEqual(len(gitops.commit_message("feat", "x" * 100).splitlines()[0]), 72)


if __name__ == "__main__":
    unittest.main()


class CommitHookTests(unittest.TestCase):
    def test_a_refused_commit_keeps_the_work_staged(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            hooks = os.path.join(root, ".git", "hooks")
            os.makedirs(hooks, exist_ok=True)
            hook = os.path.join(hooks, "pre-commit")
            with open(hook, "w") as handle:
                handle.write("#!/bin/sh\necho 'OVER_LINES: comment too long' >&2\nexit 1\n")
            os.chmod(hook, 0o755)
            git = gitops.Git(root, PROTECTED)
            git.run("switch", "-c", "fix/hooked")
            with open(os.path.join(root, "new.py"), "w") as handle:
                handle.write("x = 1\n")
            with self.assertRaises(gitops.CommitRejected) as caught:
                git.commit("fix: add new.py")
            self.assertIn("OVER_LINES", caught.exception.output)
            self.assertIn("new.py", git.run("diff", "--cached", "--name-only"))

    def test_a_linked_worktree_is_not_its_own_stale_worktree(self):
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as elsewhere:
            make_repo(root)
            linked = os.path.join(elsewhere, "wt")
            gitops.Git(root, PROTECTED).run("worktree", "add", "-q", "-b", "fix/linked", linked, "main")
            from_main = gitops.Git(root, PROTECTED)
            from_linked = gitops.Git(linked, PROTECTED)
            self.assertEqual(from_main.main_checkout(), os.path.realpath(root))
            self.assertEqual(from_linked.main_checkout(), os.path.realpath(root))
            self.assertEqual([os.path.realpath(p) for p in from_main.worktrees()], [os.path.realpath(linked)])
            self.assertEqual(from_linked.worktrees(), [])

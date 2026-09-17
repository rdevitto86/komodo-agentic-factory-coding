import os
import subprocess
import sys
import tempfile
import unittest

from komodo.adapters import claude

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOOKS = os.path.join(REPO, "komodo", "hooks")
PRE_COMMIT = os.path.join(HOOKS, "pre-commit.py")
PRE_PUSH = os.path.join(HOOKS, "pre-push.py")
GUARD = os.path.join(REPO, "komodo", "adapters", "claude", "hooks", "guard.py")


def make_repo(root):
    def git(*args):
        subprocess.run(["git", *args], cwd=root, check=True, capture_output=True)

    git("init", "-q", "-b", "main")
    git("config", "user.email", "t@example.com")
    git("config", "user.name", "t")
    git("config", "commit.gpgsign", "false")
    with open(os.path.join(root, "a.txt"), "w") as handle:
        handle.write("a\n")
    git("add", "a.txt")
    git("commit", "-q", "-m", "init")
    return git


def run_hook(script, cwd, stdin="", env=None):
    merged = dict(os.environ)
    merged["PYTHONPATH"] = REPO
    if env:
        merged.update(env)
    return subprocess.run([sys.executable, script], cwd=cwd, input=stdin, capture_output=True, text=True, env=merged)


class PreCommitTests(unittest.TestCase):
    def test_refuses_protected_branch(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            with open(os.path.join(root, "b.txt"), "w") as handle:
                handle.write("b\n")
            git("add", "b.txt")
            result = run_hook(PRE_COMMIT, root)
            self.assertEqual(result.returncode, 1)
            self.assertIn("protected branch", result.stderr)

    def test_refuses_trailer(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            git("switch", "-q", "-c", "feat/x")
            message = os.path.join(root, "msg.txt")
            with open(message, "w") as handle:
                handle.write("feat: x\n\nCo-Authored-By: Bot <b@x>\n")
            result = run_hook(PRE_COMMIT, root, env={"KOMODO_COMMIT_MSG_FILE": message})
            self.assertEqual(result.returncode, 1)
            self.assertIn("trailer", result.stderr)

    def test_passes_clean_feature_commit(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            git("switch", "-q", "-c", "feat/x")
            with open(os.path.join(root, "ok.py"), "w") as handle:
                handle.write('"""Module."""\n\n\ndef public(a):\n    """Returns a."""\n    return a\n')
            git("add", "ok.py")
            result = run_hook(PRE_COMMIT, root)
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_comment_lint_on_staged_lines(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            git("switch", "-q", "-c", "feat/x")
            with open(os.path.join(root, "bad.py"), "w") as handle:
                handle.write('"""Module."""\n\n\ndef public(a):\n    b = a\n    c = b\n    return c\n')
            git("add", "bad.py")
            result = run_hook(PRE_COMMIT, root)
            self.assertEqual(result.returncode, 1)
            self.assertIn("FUNC_UNDOCUMENTED", result.stderr)


class PrePushTests(unittest.TestCase):
    def test_refuses_protected_ref_and_delete(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            sha = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True).stdout.strip()
            stdin = "refs/heads/main %s refs/heads/main %s\n" % (sha, "0" * 40)
            result = run_hook(PRE_PUSH, root, stdin)
            self.assertEqual(result.returncode, 1)
            self.assertIn("protected ref", result.stderr)
            stdin = "(delete) %s refs/heads/feat/x %s\n" % ("0" * 40, sha)
            result = run_hook(PRE_PUSH, root, stdin)
            self.assertEqual(result.returncode, 1)
            self.assertIn("deleting", result.stderr)

    def test_refuses_non_fast_forward(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            first = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True).stdout.strip()
            git("switch", "-q", "-c", "feat/x")
            with open(os.path.join(root, "c.txt"), "w") as handle:
                handle.write("c\n")
            git("add", "c.txt")
            git("commit", "-q", "-m", "c")
            second = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True).stdout.strip()
            ok = run_hook(PRE_PUSH, root, "refs/heads/feat/x %s refs/heads/feat/x %s\n" % (second, first))
            self.assertEqual(ok.returncode, 0, ok.stderr)
            bad = run_hook(PRE_PUSH, root, "refs/heads/feat/x %s refs/heads/feat/x %s\n" % (first, second))
            self.assertEqual(bad.returncode, 1)
            self.assertIn("non-fast-forward", bad.stderr)

    def test_runs_verify_gate(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            os.makedirs(os.path.join(root, "scripts"))
            with open(os.path.join(root, "scripts", "verify.py"), "w") as handle:
                handle.write("import sys; sys.exit(1)\n")
            sha = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True).stdout.strip()
            result = run_hook(PRE_PUSH, root, "refs/heads/feat/x %s refs/heads/feat/x %s\n" % (sha, "0" * 40))
            self.assertEqual(result.returncode, 1)
            self.assertIn("failed", result.stderr)


class GuardTests(unittest.TestCase):
    def argv(self):
        return [sys.executable, GUARD]

    def probe(self, command, cwd=REPO):
        import json

        payload = json.dumps({"tool_name": "Bash", "tool_input": {"command": command}, "cwd": cwd})
        result = subprocess.run(self.argv(), input=payload, capture_output=True, text=True)
        return "deny" if result.stdout.strip() else "allow"

    def test_denies_the_never_right_set(self):
        for command in ("git push -f origin main", "git push origin feat/x:main", "git rebase main", "git reset --hard", "rm -rf x", "sudo ls", "gh pr merge 1", "git commit --amend", "git commit -m 'x\n\nCo-Authored-By: a <b>'"):
            self.assertEqual(self.probe(command), "deny", command)

    def test_allows_everyday_git(self):
        for command in ("git merge -m sync main", "git branch --list 'feat/*'", "git reflog", "git push origin feat/x", "git status && ls", "python3 -m komodo run --dry-run", "git stash list"):
            self.assertEqual(self.probe(command), "allow", command)

    def test_fails_open_on_garbage(self):
        result = subprocess.run(self.argv(), input="not json", capture_output=True, text=True)
        self.assertEqual(result.stdout.strip(), "")
        self.assertEqual(result.returncode, 0)


# The compiled guard inherits every case above, so the two implementations can never drift apart silently.
@unittest.skipUnless(claude.host_guard(), "no prebuilt guard binary for this platform")
class GuardBinaryTests(GuardTests):
    def argv(self):
        return [claude.host_guard()]


if __name__ == "__main__":
    unittest.main()

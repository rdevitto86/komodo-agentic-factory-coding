import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

from komodo.adapters import claude

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOOKS = os.path.join(REPO, "komodo", "hooks")
PRE_COMMIT = os.path.join(HOOKS, "pre-commit.py")
PRE_PUSH = os.path.join(HOOKS, "pre-push.py")
CLAUDE_HOOKS = os.path.join(REPO, "komodo", "adapters", "claude", "hooks")
GUARD = os.path.join(CLAUDE_HOOKS, "guard.py")
GUARD_GO = os.path.join(HOOKS, "src", "guard.go")
POLICY = os.path.join(REPO, "komodo", "adapters", "claude", "settings.policy.json")
INJECTOR = os.path.join(CLAUDE_HOOKS, "context_injector.py")


def load_guard():
    """Imports the guard hook from its path so its tables can be read."""
    spec = importlib.util.spec_from_file_location("komodo_guard", GUARD)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


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


def run_hook(argv, cwd, stdin="", env=None):
    merged = dict(os.environ)
    merged["PYTHONPATH"] = REPO
    if env:
        merged.update(env)
    return subprocess.run(argv, cwd=cwd, input=stdin, capture_output=True, text=True, env=merged)


class PreCommitTests(unittest.TestCase):
    def argv(self):
        return [sys.executable, PRE_COMMIT]

    def test_refuses_protected_branch(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            with open(os.path.join(root, "b.txt"), "w") as handle:
                handle.write("b\n")
            git("add", "b.txt")
            result = run_hook(self.argv(), root)
            self.assertEqual(result.returncode, 1)
            self.assertIn("protected branch", result.stderr)

    def test_refuses_trailer(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            git("switch", "-q", "-c", "feat/x")
            message = os.path.join(root, "msg.txt")
            with open(message, "w") as handle:
                handle.write("feat: x\n\nCo-Authored-By: Bot <b@x>\n")
            result = run_hook(self.argv(), root, env={"KOMODO_COMMIT_MSG_FILE": message})
            self.assertEqual(result.returncode, 1)
            self.assertIn("trailer", result.stderr)

    def test_passes_clean_feature_commit(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            git("switch", "-q", "-c", "feat/x")
            with open(os.path.join(root, "ok.py"), "w") as handle:
                handle.write('"""Module."""\n\n\ndef public(a):\n    """Returns a."""\n    return a\n')
            git("add", "ok.py")
            result = run_hook(self.argv(), root)
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_refuses_unformatted_go(self):
        if not shutil.which("gofmt"):
            self.skipTest("gofmt is not installed")
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            git("switch", "-q", "-c", "feat/x")
            with open(os.path.join(root, "bad.go"), "w") as handle:
                handle.write("package main\n\n// x does nothing.\nfunc  x()  {}\n")
            git("add", "bad.go")
            result = run_hook(self.argv(), root)
            self.assertEqual(result.returncode, 1)
            self.assertIn("not gofmt-clean", result.stderr)

    def test_comment_lint_on_staged_lines(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            git("switch", "-q", "-c", "feat/x")
            with open(os.path.join(root, "bad.py"), "w") as handle:
                handle.write('"""Module."""\n\n\ndef public(a):\n    b = a\n    c = b\n    return c\n')
            git("add", "bad.py")
            result = run_hook(self.argv(), root)
            self.assertEqual(result.returncode, 1)
            self.assertIn("FUNC_UNDOCUMENTED", result.stderr)


class PrePushTests(unittest.TestCase):
    def argv(self):
        return [sys.executable, PRE_PUSH]

    def test_refuses_protected_ref_and_delete(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            sha = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True).stdout.strip()
            stdin = "refs/heads/main %s refs/heads/main %s\n" % (sha, "0" * 40)
            result = run_hook(self.argv(), root, stdin)
            self.assertEqual(result.returncode, 1)
            self.assertIn("protected ref", result.stderr)
            stdin = "(delete) %s refs/heads/feat/x %s\n" % ("0" * 40, sha)
            result = run_hook(self.argv(), root, stdin)
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
            ok = run_hook(self.argv(), root, "refs/heads/feat/x %s refs/heads/feat/x %s\n" % (second, first))
            self.assertEqual(ok.returncode, 0, ok.stderr)
            bad = run_hook(self.argv(), root, "refs/heads/feat/x %s refs/heads/feat/x %s\n" % (first, second))
            self.assertEqual(bad.returncode, 1)
            self.assertIn("non-fast-forward", bad.stderr)

    def test_runs_verify_gate(self):
        with tempfile.TemporaryDirectory() as root:
            git = make_repo(root)
            os.makedirs(os.path.join(root, "scripts"))
            with open(os.path.join(root, "scripts", "verify.py"), "w") as handle:
                handle.write("import sys; sys.exit(1)\n")
            sha = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True).stdout.strip()
            result = run_hook(self.argv(), root, "refs/heads/feat/x %s refs/heads/feat/x %s\n" % (sha, "0" * 40))
            self.assertEqual(result.returncode, 1)
            self.assertIn("failed", result.stderr)


class GuardTests(unittest.TestCase):
    def argv(self):
        return [sys.executable, GUARD]

    def probe(self, command, cwd=REPO):
        payload = json.dumps({"tool_name": "Bash", "tool_input": {"command": command}, "cwd": cwd})
        result = subprocess.run(self.argv(), input=payload, capture_output=True, text=True)
        return "deny" if result.stdout.strip() else "allow"

    def test_denies_the_never_right_set(self):
        for command in ("git push -f origin main", "git push origin feat/x:main", "git rebase main", "git reset --hard", "rm -rf x", "sudo ls", "gh pr merge 1", "git commit --amend", "git commit -m 'x\n\nCo-Authored-By: a <b>'", "git checkout -- .", "git restore src/", "git switch --discard-changes main"):
            self.assertEqual(self.probe(command), "deny", command)

    def test_allows_everyday_git(self):
        for command in ("git branch --list 'feat/*'", "git reflog", "git push origin feat/x", "git status && ls", "python3 -m komodo run --dry-run", "git stash list", "git switch main"):
            self.assertEqual(self.probe(command), "allow", command)

    def test_merge_follows_the_branch_it_lands_on(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            self.assertEqual(self.probe("git merge feat/x", root), "deny")
            subprocess.run(["git", "switch", "-q", "-c", "feat/x"], cwd=root, check=True, capture_output=True)
            self.assertEqual(self.probe("git merge -m sync main", root), "allow")

    def test_the_policy_and_the_go_twin_carry_every_destructive_verb(self):
        with open(POLICY, encoding="utf-8") as handle:
            deny = set(json.load(handle)["permissions"]["deny"])
        with open(GUARD_GO, encoding="utf-8") as handle:
            go_source = handle.read()
        for sub in load_guard().DESTRUCTIVE:
            self.assertIn("Bash(git %s:*)" % sub, deny, sub)
            self.assertIn('"%s":' % sub, go_source, sub)

    def test_fails_open_on_garbage(self):
        result = subprocess.run(self.argv(), input="not json", capture_output=True, text=True)
        self.assertEqual(result.stdout.strip(), "")
        self.assertEqual(result.returncode, 0)


class InjectorTests(unittest.TestCase):
    def argv(self):
        return [sys.executable, INJECTOR]

    def summary(self, root):
        return subprocess.run(self.argv(), cwd=root, capture_output=True, text=True).stdout

    def test_silent_outside_a_repo(self):
        with tempfile.TemporaryDirectory() as root:
            self.assertEqual(self.summary(root), "")

    def test_reports_backlog_counts_and_version(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            with open(os.path.join(root, "BACKLOG.md"), "w") as handle:
                handle.write(
                    "### [TG-01.1] First\n"
                    "#### [TSK-01.1.1] done work [P: H] [DONE]\n"
                    "#### [TSK-01.1.2] open work [P: H] [READY]\n"
                    "#### [TSK-01.1.3] stuck work [P: M] [BLOCKED]\n"
                    "#### [TSK-01.1.4] live work [P: M] [IN_PROGRESS]\n"
                    "#### [TSK-01.1.5] unplanned work [P: L] [REFINEMENT]\n"
                )
            with open(os.path.join(root, "CHANGELOG.md"), "w") as handle:
                handle.write("## [2.1.0] - 2026-01-01\n")
            out = self.summary(root)
            self.assertIn("In progress: TSK-01.1.4 live work", out)
            self.assertIn("Backlog: 4 open, 1 blocked, 1 in refinement. Next group: TG-01.1.", out)
            self.assertIn("Released version: 2.1.0.", out)
            self.assertIn("No verify gate declared.", out)

    def test_reports_a_missing_backlog_and_a_verify_gate(self):
        with tempfile.TemporaryDirectory() as root:
            make_repo(root)
            os.makedirs(os.path.join(root, "scripts"))
            with open(os.path.join(root, "scripts", "verify.py"), "w") as handle:
                handle.write("")
            out = self.summary(root)
            self.assertIn("No BACKLOG.md; the harness has nothing to run here.", out)
            self.assertIn("Verify gate: scripts/verify.py.", out)


# The compiled hooks inherit every case above, so the two implementations can never drift apart silently.
@unittest.skipUnless(claude.host_binary(), "no prebuilt hook binary for this platform")
class GuardBinaryTests(GuardTests):
    def argv(self):
        return [claude.host_binary(), "guard"]


@unittest.skipUnless(claude.host_binary(), "no prebuilt hook binary for this platform")
class InjectorBinaryTests(InjectorTests):
    def argv(self):
        return [claude.host_binary(), "inject"]


@unittest.skipUnless(claude.host_binary(), "no prebuilt hook binary for this platform")
class PreCommitBinaryTests(PreCommitTests):
    def argv(self):
        return [claude.host_binary(), "precommit"]


@unittest.skipUnless(claude.host_binary(), "no prebuilt hook binary for this platform")
class PrePushBinaryTests(PrePushTests):
    def argv(self):
        return [claude.host_binary(), "prepush"]


if __name__ == "__main__":
    unittest.main()

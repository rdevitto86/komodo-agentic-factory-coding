#!/usr/bin/env python3
from __future__ import annotations

import difflib
import os
import re
import shutil
import subprocess
import sys
import tempfile

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
INSTALL = os.path.join(REPO_ROOT, "scripts", "install.py")
SETUP = os.path.join(REPO_ROOT, "setup.sh")
GIT_INSTALL = os.path.join(REPO_ROOT, "scripts", "hooks", "git", "install.sh")
LIVE_HOOK_DIR = os.path.join(REPO_ROOT, "scripts", "hooks", "git")

METACHAR_COMMAND = re.compile(
    r"\"command\": \"[a-zA-Z0-9_]+ '[^']*home6\$\(evil\)[^']*git_guard\.py'"
)

PASSED = [0]
FAILED = [0]


def emit(text: str) -> None:
    sys.stdout.write(text)
    sys.stdout.flush()


def passed(label: str) -> None:
    emit("  PASS  %s\n" % label)
    PASSED[0] += 1


def failed(label: str, reason: str) -> None:
    emit("  FAIL  %s\n        %s\n" % (label, reason))
    FAILED[0] += 1


def record(label: str, problem: str) -> None:
    if problem:
        failed(label, problem)
    else:
        passed(label)


def capture(args: list, env: dict = None, cwd: str = None):
    result = subprocess.run(
        args, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=env, cwd=cwd
    )
    return result.returncode, result.stdout.decode("utf-8", "replace")


def git(args: list, repo: str):
    return capture(["git", "-C", repo] + args)


def read_text(path: str) -> str:
    try:
        with open(path, encoding="utf-8", errors="replace") as handle:
            return handle.read()
    except OSError:
        return ""


def env_with(**overrides) -> dict:
    env = dict(os.environ)
    for key, value in overrides.items():
        env[key] = value
    return env


def check_install(workdir: str, python3: str) -> None:
    emit("\ninstall\n\n")

    stubbin = os.path.join(workdir, "stubbin-python-only")
    os.makedirs(stubbin)
    os.symlink(python3, os.path.join(stubbin, "python"))

    label = "I1 resolves python when python3 is absent from PATH"
    target1 = os.path.join(workdir, "home1", ".claude")
    rc, out = capture(
        [os.path.join(stubbin, "python"), INSTALL, "--target", target1],
        env=env_with(PATH=stubbin),
    )
    if rc != 0:
        failed(label, "exit %d: %s" % (rc, out))
    elif '"command": "python /' not in read_text(os.path.join(target1, "settings.json")):
        failed(label, "settings.json did not use the resolved 'python' interpreter")
    else:
        passed(label)

    target2 = os.path.join(workdir, "home2", ".claude")
    rc, out = capture([python3, INSTALL, "--target", target2])
    if rc != 0:
        failed("I2 install with the real python3 exits 0", "exit %d: %s" % (rc, out))
    elif "~" in read_text(os.path.join(target2, "settings.json")):
        failed("I2 no literal tilde in generated settings.json", "tilde found")
    else:
        passed("I2 no literal tilde in generated settings.json")

    label = "I3 forced symlink failure falls back to copy and prints guidance"
    target3 = os.path.join(workdir, "home3", ".claude")
    rc, out = capture([python3, INSTALL, "--target", target3, "--force-copy"])
    hooks3 = os.path.join(target3, "hooks")
    problem = ""
    if rc != 0:
        problem = "exit %d: %s" % (rc, out)
    if not problem and os.path.islink(hooks3):
        problem = "hooks was symlinked, not copied"
    if not problem and not os.path.isdir(hooks3):
        problem = "hooks directory missing after copy fallback"
    if not problem and not os.path.isfile(os.path.join(hooks3, "git_guard.py")):
        problem = "hooks/git_guard.py missing after copy fallback"
    if not problem and "Developer Mode" not in out:
        problem = "missing Developer Mode guidance"
    if not problem and "install.py" not in out:
        problem = "missing re-sync command"
    record(label, problem)

    label = "I4 a normal install still symlinks (macOS/Linux path unaffected)"
    target4 = os.path.join(workdir, "home4", ".claude")
    capture([python3, INSTALL, "--target", target4])
    hooks4 = os.path.join(target4, "hooks")
    if os.path.islink(hooks4) and os.path.isfile(os.path.join(hooks4, "git_guard.py")):
        passed(label)
    else:
        failed(label, "hooks was not a live symlink")

    label = "I5 symlinked directory entry is created with directory semantics"
    target5 = os.path.join(workdir, "home5", ".claude")
    capture([python3, INSTALL, "--target", target5])
    hooks5 = os.path.join(target5, "hooks")
    problem = ""
    if not os.path.islink(hooks5):
        problem = "hooks was not a symlink"
    if not problem and not os.path.isdir(hooks5):
        problem = "symlinked hooks does not resolve to a directory"
    if not problem and not os.path.isfile(os.path.join(hooks5, "git_guard.py")):
        problem = "hooks/git_guard.py not reachable through symlinked directory"
    if not problem and "target_is_directory" not in read_text(INSTALL):
        problem = "install.py no longer requests target_is_directory"
    record(label, problem)

    label = "I6 hook_path with a shell metacharacter is single-quoted, not left bare"
    target6 = os.path.join(workdir, "home6$(evil)", ".claude")
    os.makedirs(os.path.dirname(target6))
    rc, out = capture([python3, INSTALL, "--target", target6])
    settings6 = read_text(os.path.join(target6, "settings.json"))
    if rc != 0:
        failed(label, "exit %d: %s" % (rc, out))
    elif not METACHAR_COMMAND.search(settings6):
        first = ""
        for line in settings6.splitlines():
            if '"command"' in line:
                first = line
                break
        failed(
            label,
            "generated command did not wrap the metacharacter-bearing path in single quotes: %s" % first,
        )
    else:
        passed(label)


def assert_guard_rejected(label: str, clone: str, target: str, args: list) -> None:
    _, head_before = git(["rev-parse", "--abbrev-ref", "HEAD"], clone)
    rc, out = capture(
        ["bash", os.path.join(clone, "setup.sh")] + args, env=env_with(AGENT_HOME=target)
    )
    _, head_after = git(["rev-parse", "--abbrev-ref", "HEAD"], clone)
    problem = ""
    if rc == 0:
        problem = "exited 0: %s" % out
    if not problem and head_before != head_after:
        problem = "HEAD moved"
    if not problem and os.path.exists(target):
        problem = "target was created before the guard failed"
    record(label, problem)


def check_setup_ref(workdir: str) -> None:
    emit("\nsetup.sh --ref\n\n")

    clone = os.path.join(workdir, "clone")
    capture(["git", "clone", "--quiet", "--local", REPO_ROOT, clone])
    _, tag_out = git(["tag"], clone)
    tag_lines = tag_out.splitlines()
    tag = tag_lines[-1] if tag_lines else ""

    label = "S4 plain --dry-run output is unchanged by the --ref addition"
    target_a = os.path.join(workdir, "home-s4a", ".claude")
    target_b = os.path.join(workdir, "home-s4b", ".claude")
    _, baseline = capture(
        ["bash", os.path.join(clone, "setup.sh"), "--dry-run"], env=env_with(AGENT_HOME=target_a)
    )
    _, current = capture(["bash", SETUP, "--dry-run"], env=env_with(AGENT_HOME=target_b))
    baseline = baseline.replace(target_a, "TARGET").replace(clone, "REPO")
    current = current.replace(target_b, "TARGET").replace(REPO_ROOT, "REPO")
    if baseline == current:
        passed(label)
    else:
        diff = "".join(
            difflib.unified_diff(
                baseline.splitlines(True), current.splitlines(True), "baseline", "current"
            )
        )
        failed(label, "diff:\n%s" % diff)

    shutil.copyfile(SETUP, os.path.join(clone, "setup.sh"))
    _, commit_out = capture(
        [
            "git", "-C", clone,
            "-c", "user.email=test@example.com",
            "-c", "user.name=test",
            "commit", "--quiet", "-am",
            "bring in the working copy's setup.sh for --ref testing",
        ]
    )
    emit(commit_out)

    label_s1 = "S1 --ref TAG --dry-run previews the checkout and exits 0"
    label_s3 = "S3 a dirty tree exits non-zero before touching HEAD or the target"
    if not tag:
        failed(label_s1, "no tag found in the clone to pin to")
        failed(label_s3, "no tag found in the clone to pin to")
    else:
        _, commit_line = git(["rev-parse", "--verify", "--quiet", tag + "^{commit}"], clone)
        tag_commit = commit_line.strip()
        target_s1 = os.path.join(workdir, "home-s1", ".claude")
        rc, out = capture(
            ["bash", os.path.join(clone, "setup.sh"), "--ref", tag, "--dry-run"],
            env=env_with(AGENT_HOME=target_s1),
        )
        preview = "would: git -C %s checkout --detach %s" % (clone, tag_commit)
        if rc != 0:
            failed(label_s1, "exit %d: %s" % (rc, out))
        elif preview not in out:
            failed(label_s1, "did not preview the detach: %s" % out)
        else:
            passed(label_s1)

    assert_guard_rejected(
        "S2 an unknown ref exits non-zero before touching HEAD or the target",
        clone,
        os.path.join(workdir, "home-s2", ".claude"),
        ["--ref", "no-such-tag-xyz"],
    )
    assert_guard_rejected(
        "S5 an empty --ref is rejected rather than silently installing unpinned",
        clone,
        os.path.join(workdir, "home-s5", ".claude"),
        ["--ref="],
    )
    assert_guard_rejected(
        "S6 a leading-dash ref is rejected before reaching git",
        clone,
        os.path.join(workdir, "home-s6", ".claude"),
        ["--ref", "--orphan=x"],
    )

    with open(os.path.join(clone, "README.md"), "a", encoding="utf-8") as handle:
        handle.write("dirty\n")
    if tag:
        assert_guard_rejected(
            label_s3, clone, os.path.join(workdir, "home-s3", ".claude"), ["--ref", tag]
        )


def make_git_repo(path: str) -> None:
    os.makedirs(path)
    git(["init", "--quiet"], path)


def check_git_install(workdir: str) -> None:
    emit("\nscripts/hooks/git/install.sh\n\n")

    grepo1 = os.path.join(workdir, "grepo1")
    make_git_repo(grepo1)
    git(["config", "--local", "core.hooksPath", "no-such-dir/hooks"], grepo1)

    label = "G1 --status marks a missing core.hooksPath stale"
    rc, out = capture(["bash", GIT_INSTALL, "--status", grepo1])
    if rc != 0:
        failed(label, "exit %d: %s" % (rc, out))
    elif "stale" not in out:
        failed(label, "no stale marker in: %s" % out)
    else:
        passed(label)

    label = "G2 installing over a stale core.hooksPath notes hooks had not been running"
    rc, out = capture(["bash", GIT_INSTALL, grepo1])
    _, current = git(["config", "--local", "--get", "core.hooksPath"], grepo1)
    current = current.strip()
    problem = ""
    if rc != 0:
        problem = "exit %d: %s" % (rc, out)
    if not problem and "had not been running" not in out:
        problem = "no note that hooks had not been running: %s" % out
    if not problem and current != LIVE_HOOK_DIR:
        problem = "core.hooksPath was not rewritten to the live directory: %s" % current
    record(label, problem)

    label = "G3 a repo already pointing at the live hooks dir is left alone"
    rc, out = capture(["bash", GIT_INSTALL, grepo1])
    if rc != 0:
        failed(label, "exit %d: %s" % (rc, out))
    elif "already installed" not in out:
        failed(label, "missing 'already installed': %s" % out)
    else:
        passed(label)

    label = "G4 an orphaned .git/hooks file is still reported, unset stays unset"
    grepo2 = os.path.join(workdir, "grepo2")
    make_git_repo(grepo2)
    orphan = os.path.join(grepo2, ".git", "hooks", "pre-commit")
    if not os.path.isdir(os.path.dirname(orphan)):
        os.makedirs(os.path.dirname(orphan))
    with open(orphan, "w", encoding="utf-8") as handle:
        handle.write("#!/bin/sh\n")
    os.chmod(orphan, 0o755)

    rc, out = capture(["bash", GIT_INSTALL, grepo2])
    problem = ""
    if rc != 0:
        problem = "exit %d: %s" % (rc, out)
    if not problem and ".git/hooks files stop running" not in out:
        problem = "missing orphan-hooks note: %s" % out
    if not problem and "had not been running" in out:
        problem = "unset core.hooksPath was misclassified as stale: %s" % out
    record(label, problem)


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")

    python3 = sys.executable or shutil.which("python3")
    workdir = tempfile.mkdtemp()
    try:
        check_install(workdir, python3)
        check_setup_ref(workdir)
        check_git_install(workdir)
    finally:
        shutil.rmtree(workdir, ignore_errors=True)

    emit("\n  %d passed, %d failed\n\n" % (PASSED[0], FAILED[0]))
    return 1 if FAILED[0] else 0


if __name__ == "__main__":
    sys.exit(main())

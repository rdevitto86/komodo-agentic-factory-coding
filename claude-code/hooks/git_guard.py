#!/usr/bin/env python3

import json
import os
import re
import shlex
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from comment_guard import EXTENSION_FAMILY, FILENAME_FAMILY

READ_ONLY_GIT = {
    "annotate",
    "blame",
    "cat-file",
    "check-ignore",
    "cherry",
    "config",
    "count-objects",
    "describe",
    "diff",
    "diff-index",
    "diff-tree",
    "difftool",
    "fetch",
    "for-each-ref",
    "grep",
    "help",
    "log",
    "ls-files",
    "ls-remote",
    "ls-tree",
    "merge-base",
    "name-rev",
    "remote",
    "rev-list",
    "rev-parse",
    "shortlog",
    "show",
    "show-branch",
    "show-ref",
    "status",
    "tag",
    "var",
    "verify-commit",
    "version",
    "whatchanged",
}

MUTATING_FLAGS = {
    "branch": ("-d", "-D", "-m", "-M", "-c", "-C", "--delete", "--move", "--copy", "--edit-description", "--set-upstream-to", "--unset-upstream"),
    "tag": ("-d", "-D", "--delete", "-f", "--force"),
    "remote": ("add", "remove", "rm", "rename", "set-url", "set-head", "set-branches", "prune"),
    "config": ("--unset", "--unset-all", "--add", "--replace-all", "--rename-section", "--remove-section", "--edit", "-e"),
}

GIT_GLOBAL_FLAGS_WITH_VALUE = ("-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path")

# --- Publishing policy -------------------------------------------------
#
# The agent branches, commits, pushes its own branch, and opens a PR.
# It never reaches a protected ref, never rewrites history, and never
# merges. Everything below is the deterministic half of that rule; the
# `rules-source-control` skill carries the reasoning.

# Flip to False to retire the whole capability: every state-changing verb
# below goes back to a blanket deny, and no other file needs reverting.
# The gh restrictions are strictly tighter than what shipped before this,
# so they stay on either way.
PUBLISH_ENABLED = True

PUBLISH_VERBS = ("push", "commit", "switch", "add", "merge", "pull")

PROTECTED_BRANCHES = {"main", "master", "trunk", "prod", "production"}
PROTECTED_PREFIXES = ("release/", "hotfix/")

BRANCH_NAME = re.compile(r"^(feat|fix|chore|docs|test|refactor|perf|build|ci)/[a-z0-9][a-z0-9-]*$")

PUSH_FORCE_FLAGS = ("-f", "--force", "--force-with-lease", "--force-if-includes")
PUSH_BANNED_FLAGS = ("--mirror", "--all", "--tags", "--follow-tags", "--delete", "-d", "--prune")
COMMIT_BANNED_FLAGS = ("-n", "--no-verify", "--amend", "--no-gpg-sign")
ADD_BANNED_FLAGS = ("-i", "--interactive", "-p", "--patch")
SWITCH_BANNED_FLAGS = ("-C", "--force-create", "--orphan", "-d", "--detach", "--discard-changes")

BANNED_TRAILER = re.compile(r"co-authored-by\s*:|generated with|\U0001F916", re.I)
TRAILER_FINDING = "the commit message carries a co-author or generated-by trailer"

# A -m body spans newlines, and SEGMENT_SPLIT breaks on them — so a trailer
# on the message's third line never reaches the tokenized args. The raw
# command is the only place the whole message is still intact.
GIT_COMMIT = re.compile(r"\bgit\b[^\n;|&]*\bcommit\b")

GH_GLOBAL_VALUE_FLAGS = ("-R", "--repo")
GH_ALLOWED = {
    "pr": {"create", "view", "list", "diff", "status", "checks", "comment", "edit", "ready"},
    "issue": {"view", "list", "create", "comment"},
    "repo": {"view"},
    "run": {"list", "view", "watch"},
    "label": {"list", "create"},
    "auth": {"status"},
    "stack": None,
    "search": None,
    "browse": None,
    "status": None,
    "version": None,
    "extension": {"list"},
}
GH_API_WRITE_FLAGS = ("-X", "--method", "-f", "--raw-field", "-F", "--field", "--input")

SHELL_WRAPPERS = ("sh", "bash", "zsh", "dash", "ksh", "env")
PASSTHROUGH_WRAPPERS = ("time", "command", "nohup", "xargs")
PASSTHROUGH_VALUE_FLAGS = {
    "time": ("-f", "-o"),
    "command": (),
    "nohup": (),
    "xargs": ("-I", "-n", "-P", "-L", "-s", "-a", "-d", "-E"),
}

# Names strip_leading_flags treats as "this must be the wrapped command,
# not a flag's value" when it meets an option it doesn't recognize.
MONITORED_COMMANDS = SHELL_WRAPPERS + PASSTHROUGH_WRAPPERS + ("git", "gh", "cp", "mv", "tee", "eval")

CP_MV_TARGET_FLAGS = ("-t", "--target-directory")

SEGMENT_SPLIT = re.compile(r"&&|\|\||[;\n|]")
REDIRECT = re.compile(r"(?<![-=<0-9&])>>?\s*([^\s;&|>]+)")
HEREDOC = re.compile(r"(<<-?\s*['\"]?(\w+)['\"]?[^\n]*\n)(.*?)(^\s*\2\s*$)", re.S | re.M)
QUOTED = re.compile(r"'[^']*'|\"[^\"]*\"")
INPLACE_SED = re.compile(r"\bsed\b[^;|&]*?(?:\s-[a-zA-Z]*i\b|\s--in-place\b)")
INPLACE_PERL = re.compile(r"\bperl\b[^;|&]*?\s-[a-zA-Z]*i\b")
PYTHON_WRITE = re.compile(r"\bpython3?\b[^;|&]*?-c\b.*?open\s*\([^)]*['\"][wa]")


def parse_cp_mv_target(tokens):
    # cp/mv -t DIR (or --target-directory[=DIR]) names the real
    # destination out of order; everything else stays positional.
    index = 0
    target_dir = None
    positional = []
    while index < len(tokens):
        token = tokens[index]
        if token in CP_MV_TARGET_FLAGS:
            if index + 1 < len(tokens):
                target_dir = tokens[index + 1]
            index += 2
            continue
        if token.startswith("--target-directory="):
            target_dir = token.split("=", 1)[1]
            index += 1
            continue
        if token.startswith("-") and token != "-":
            index += 1
            continue
        positional.append(token)
        index += 1
    return target_dir, positional


def is_code_path(token):
    cleaned = token.strip("\"'")
    base = os.path.basename(cleaned)
    if base in FILENAME_FAMILY:
        return True
    _, ext = os.path.splitext(base)
    return ext.lower() in EXTENSION_FAMILY


def blank(text):
    return re.sub(r"[^\n]", " ", text)


def mask_data(command):
    # heredoc bodies and quoted spans are data, not shell syntax
    masked = HEREDOC.sub(lambda m: m.group(1) + blank(m.group(3)) + m.group(4), command)
    return QUOTED.sub(lambda m: blank(m.group(0)), masked)


def tokenize(segment):
    try:
        return shlex.split(segment)
    except ValueError:
        return segment.split()


def strip_env_assignments(tokens):
    index = 0
    while index < len(tokens) and re.match(r"^[A-Za-z_][A-Za-z0-9_]*=", tokens[index]):
        index += 1
    return tokens[index:]


def strip_leading_flags(tokens, value_flags):
    index = 0
    while index < len(tokens) and tokens[index].startswith("-"):
        flag = tokens[index]
        index += 1
        if flag in value_flags:
            index += 1
            continue
        if "=" in flag:
            continue
        # An option we don't have on record: if what follows doesn't look
        # like the wrapped command itself, assume it's this flag's value
        # rather than mis-starting the inner command at that token.
        if index < len(tokens):
            nxt = tokens[index]
            if not nxt.startswith("-") and os.path.basename(nxt) not in MONITORED_COMMANDS:
                index += 1
    return tokens[index:]


def subcommand_of(tokens, value_flags):
    index = 1
    while index < len(tokens):
        token = tokens[index]
        if token in value_flags:
            index += 2
            continue
        if token.startswith("-"):
            index += 1
            continue
        return token, tokens[index + 1:]
    return None, []


def leading_word(segment):
    tokens = strip_env_assignments(tokenize(segment))
    return os.path.basename(tokens[0]) if tokens else ""


def current_branch(cwd):
    if not cwd or not os.path.isdir(cwd):
        return None
    try:
        result = subprocess.run(
            ["git", "-C", cwd, "symbolic-ref", "--short", "HEAD"],
            capture_output=True, text=True, timeout=5,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None
    return result.stdout.strip() or None


def is_protected(ref):
    ref = ref.strip().strip("\"'")
    if ref.startswith("refs/heads/"):
        ref = ref[len("refs/heads/"):]
    if ref in PROTECTED_BRANCHES:
        return True
    return ref.startswith(PROTECTED_PREFIXES)


def push_violation(args, cwd):
    positional = []
    for arg in args:
        if arg in PUSH_FORCE_FLAGS or arg.startswith("--force-with-lease=") or arg.startswith("--force-if-includes="):
            return "git push %s rewrites published history" % arg
        if arg in PUSH_BANNED_FLAGS:
            return "git push %s is denied" % arg
        if not arg.startswith("-"):
            positional.append(arg)
    if len(positional) < 2:
        return "git push needs an explicit remote and branch — git push -u origin <branch>"
    for spec in positional[1:]:
        if spec.startswith("+"):
            return "a + refspec force-pushes; push without it"
        if spec.startswith(":"):
            return "git push :<ref> deletes a remote branch"
        source, _, destination = spec.partition(":")
        destination = destination or source
        if destination in ("HEAD", ""):
            destination = current_branch(cwd) or ""
            if not destination:
                return "cannot resolve HEAD to a branch — name the destination explicitly"
        if is_protected(destination):
            return "git push to %s is denied — open a pull request instead" % destination
    return None


def commit_violation(args, cwd, has_cd):
    for arg in args:
        if arg in COMMIT_BANNED_FLAGS:
            return "git commit %s skips a gate or rewrites a commit" % arg
    if BANNED_TRAILER.search(" ".join(args)):
        return TRAILER_FINDING
    if has_cd:
        return "a cd earlier in this command makes the current branch unknowable"
    branch = current_branch(cwd)
    if branch is None:
        return "cannot resolve the current branch — refusing to commit"
    if is_protected(branch):
        return "git commit on %s is denied — create a branch first" % branch
    return None


def switch_violation(args):
    created = None
    index = 0
    while index < len(args):
        arg = args[index]
        if arg in SWITCH_BANNED_FLAGS:
            return "git switch %s is denied" % arg
        if arg in ("-c", "--create"):
            created = args[index + 1] if index + 1 < len(args) else ""
            index += 2
            continue
        index += 1
    if created is not None and not BRANCH_NAME.match(created):
        return "branch name '%s' must match <type>/<kebab-case>" % created
    return None


def git_violation(subcommand, args, cwd, has_cd):
    if subcommand is None:
        return None
    if not PUBLISH_ENABLED:
        if subcommand in PUBLISH_VERBS:
            return "git %s changes repository state" % subcommand
        if subcommand == "branch":
            positional = [arg for arg in args if not arg.startswith("-")]
            if positional:
                return "git branch %s creates a branch" % positional[0]
    if subcommand == "push":
        return push_violation(args, cwd)
    if subcommand == "commit":
        return commit_violation(args, cwd, has_cd)
    if subcommand == "switch":
        return switch_violation(args)
    if subcommand == "add":
        for arg in args:
            if arg in ADD_BANNED_FLAGS:
                return "git add %s is interactive and cannot complete here" % arg
        return None
    if subcommand in ("merge", "pull"):
        if "--ff-only" in args:
            return None
        if subcommand == "pull":
            return "git pull is allowed only with --ff-only — use git fetch + git merge for a real sync"
        if "--abort" in args or "--continue" in args:
            return None
        for flag in ("-X", "--strategy-option", "-s", "--strategy", "--squash"):
            if flag in args:
                return "git merge %s auto-resolves without a visible conflict" % flag
        branch = current_branch(cwd)
        if branch and is_protected(branch):
            return "git merge on %s, a protected branch — landing into it stays the user's job" % branch
        positional = [arg for arg in args if not arg.startswith("-")]
        if len(positional) != 1:
            return "git merge only takes one target: the protected base branch, to sync before a human merges the PR"
        target = positional[0]
        bare = target[len("origin/"):] if target.startswith("origin/") else target
        if not is_protected(bare):
            return "git merge %s isn't the protected base branch" % target
        return None
    if subcommand == "branch":
        positional = [arg for arg in args if not arg.startswith("-")]
        for flag in MUTATING_FLAGS["branch"]:
            if flag in args:
                return "git branch %s changes repository state" % flag
        if positional and not BRANCH_NAME.match(positional[0]):
            return "branch name '%s' must match <type>/<kebab-case>" % positional[0]
        return None
    if subcommand not in READ_ONLY_GIT:
        return "git %s changes repository state" % subcommand
    if subcommand == "config":
        positional = [arg for arg in args if not arg.startswith("-")]
        if len(positional) >= 2:
            return "git config %s sets a value" % positional[0]
    mutators = MUTATING_FLAGS.get(subcommand)
    if not mutators:
        return None
    for arg in args:
        if arg in mutators:
            return "git %s %s changes repository state" % (subcommand, arg)
    return None


def gh_violation(tokens):
    group, rest = subcommand_of(tokens, GH_GLOBAL_VALUE_FLAGS)
    if group is None:
        return None
    if group == "api":
        for token in tokens:
            if token in GH_API_WRITE_FLAGS or token.startswith("--method="):
                return "gh api write requests are denied"
        return None
    if group not in GH_ALLOWED:
        return "gh %s is denied" % group
    allowed = GH_ALLOWED[group]
    if allowed is None:
        return None
    subcommand = next((arg for arg in rest if not arg.startswith("-")), "")
    if subcommand not in allowed:
        return "gh %s %s is denied" % (group, subcommand or "<none>")
    return None


def scan_segment(segment, findings, cwd, has_cd):
    tokens = strip_env_assignments(tokenize(segment))
    if not tokens:
        return
    command = os.path.basename(tokens[0])
    if command in SHELL_WRAPPERS:
        for index, token in enumerate(tokens):
            if token == "-c" and index + 1 < len(tokens):
                scan_command(tokens[index + 1], findings, cwd)
        return
    if command == "eval":
        inner = tokens[1:]
        if inner:
            scan_command(" ".join(inner), findings, cwd)
        return
    if command in PASSTHROUGH_WRAPPERS:
        inner = strip_leading_flags(tokens[1:], PASSTHROUGH_VALUE_FLAGS[command])
        if inner:
            scan_command(" ".join(inner), findings, cwd)
        return
    if command == "tee":
        for token in tokens[1:]:
            if is_code_path(token):
                findings.append("tee writing to %s bypasses the comment guard" % token)
                break
        return
    if command in ("cp", "mv"):
        target_dir, positional = parse_cp_mv_target(tokens[1:])
        if target_dir is not None:
            if is_code_path(target_dir):
                findings.append("%s writing to %s bypasses the comment guard" % (command, target_dir))
                return
            for source in positional:
                if is_code_path(source):
                    findings.append("%s writing to %s bypasses the comment guard" % (command, target_dir))
                    break
            return
        if not positional:
            return
        destination = positional[-1]
        sources = positional[:-1]
        dest_clean = destination.strip("\"'")
        stripped = dest_clean.rstrip("/")
        _, dest_ext = os.path.splitext(stripped)
        dest_is_dir = (
            dest_clean.endswith("/")
            or (bool(stripped) and os.path.isdir(stripped))
            or (bool(stripped) and not dest_ext and not os.path.isfile(stripped))
        )
        if dest_is_dir:
            for source in sources:
                if is_code_path(source):
                    findings.append("%s writing to %s bypasses the comment guard" % (command, destination))
                    break
        elif is_code_path(destination):
            findings.append("%s writing to %s bypasses the comment guard" % (command, destination))
        return
    if command == "git":
        subcommand, args = subcommand_of(tokens, GIT_GLOBAL_FLAGS_WITH_VALUE)
        scoped = cwd
        for index, token in enumerate(tokens):
            if token == "-C" and index + 1 < len(tokens):
                scoped = tokens[index + 1]
        violation = git_violation(subcommand, args, scoped, has_cd)
        if violation:
            findings.append(violation)
        return
    if command == "gh":
        violation = gh_violation(tokens)
        if violation:
            findings.append(violation)
        return


def scan_command(command, findings, cwd):
    segments = [segment.strip() for segment in SEGMENT_SPLIT.split(command)]
    segments = [segment for segment in segments if segment]
    has_cd = any(leading_word(segment) == "cd" for segment in segments)
    for segment in segments:
        scan_segment(segment, findings, cwd, has_cd)

    if GIT_COMMIT.search(command) and BANNED_TRAILER.search(command):
        findings.append(TRAILER_FINDING)
    if INPLACE_SED.search(command):
        findings.append("sed -i rewrites files in place, bypassing the comment guard")
    if INPLACE_PERL.search(command):
        findings.append("perl -i rewrites files in place, bypassing the comment guard")
    if PYTHON_WRITE.search(command):
        findings.append("python -c opening a file for writing bypasses the comment guard")

    for target in REDIRECT.findall(mask_data(command)):
        if is_code_path(target):
            findings.append("redirecting output into %s bypasses the comment guard" % target)


def respond_deny(reason):
    payload = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }
    sys.stdout.write(json.dumps(payload))
    sys.exit(0)


def main():
    payload = json.loads(sys.stdin.read())
    if payload.get("tool_name") != "Bash":
        sys.exit(0)
    command = (payload.get("tool_input") or {}).get("command", "")
    if not command:
        sys.exit(0)

    findings = []
    scan_command(command, findings, payload.get("cwd") or os.getcwd())
    if not findings:
        sys.exit(0)

    unique = []
    for item in findings:
        if item not in unique:
            unique.append(item)

    lines = ["BLOCKED. Nothing ran.", ""]
    lines.extend("    %s" % item for item in unique)
    lines.extend([
        "",
        "You may branch, commit, push your own branch, and open a pull request.",
        "You may never reach a protected ref, rewrite history, or merge.",
        "Reading history is always fine: log, diff, show, status, blame, rev-parse.",
        "In-place file rewrites must go through Edit or Write so the comment",
        "guard can see them. Run this yourself if you intended it.",
    ])
    respond_deny("\n".join(lines))


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException as error:
        respond_deny(
            "git guard failed to evaluate this command: %s: %s\nFailing closed."
            % (type(error).__name__, error)
        )

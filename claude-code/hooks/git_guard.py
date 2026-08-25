#!/usr/bin/env python3

import json
import os
import re
import shlex
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from comment_guard import EXTENSION_FAMILY, FILENAME_FAMILY

READ_ONLY_GIT = {
    "annotate",
    "blame",
    "branch",
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
    "stash",
    "status",
    "tag",
    "var",
    "verify-commit",
    "version",
    "whatchanged",
}

MUTATING_FLAGS = {
    "branch": ("-d", "-D", "-m", "-M", "-c", "-C", "--delete", "--move", "--copy", "--edit-description", "--set-upstream-to", "-u", "--unset-upstream"),
    "tag": ("-d", "-D", "--delete", "-a", "-s", "-f", "--force", "-m", "--annotate", "--sign"),
    "remote": ("add", "remove", "rm", "rename", "set-url", "set-head", "set-branches", "prune"),
    "config": ("--unset", "--unset-all", "--add", "--replace-all", "--rename-section", "--remove-section", "--edit", "-e"),
}

READ_ONLY_MODES = {"stash": ("list", "show")}

GIT_GLOBAL_FLAGS_WITH_VALUE = ("-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path")

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
MONITORED_COMMANDS = SHELL_WRAPPERS + PASSTHROUGH_WRAPPERS + ("git", "cp", "mv", "tee", "eval")

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


def git_subcommand(tokens):
    index = 1
    while index < len(tokens):
        token = tokens[index]
        if token in GIT_GLOBAL_FLAGS_WITH_VALUE:
            index += 2
            continue
        if token.startswith("-"):
            index += 1
            continue
        return token, tokens[index + 1:]
    return None, []


def git_violation(subcommand, args):
    if subcommand is None:
        return None
    if subcommand not in READ_ONLY_GIT:
        return "git %s changes repository state" % subcommand
    allowed_modes = READ_ONLY_MODES.get(subcommand)
    if allowed_modes is not None:
        positional = [arg for arg in args if not arg.startswith("-")]
        if not positional or positional[0] not in allowed_modes:
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
    if subcommand == "branch":
        positional = [arg for arg in args if not arg.startswith("-")]
        if positional:
            return "git branch %s creates a branch" % positional[0]
    if subcommand == "tag":
        positional = [arg for arg in args if not arg.startswith("-")]
        if positional:
            return "git tag %s creates a tag" % positional[0]
    return None


def scan_segment(segment, findings):
    tokens = strip_env_assignments(tokenize(segment))
    if not tokens:
        return
    command = os.path.basename(tokens[0])
    if command in SHELL_WRAPPERS:
        for index, token in enumerate(tokens):
            if token == "-c" and index + 1 < len(tokens):
                scan_command(tokens[index + 1], findings)
        return
    if command == "eval":
        inner = tokens[1:]
        if inner:
            scan_command(" ".join(inner), findings)
        return
    if command in PASSTHROUGH_WRAPPERS:
        inner = strip_leading_flags(tokens[1:], PASSTHROUGH_VALUE_FLAGS[command])
        if inner:
            scan_command(" ".join(inner), findings)
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
        subcommand, args = git_subcommand(tokens)
        violation = git_violation(subcommand, args)
        if violation:
            findings.append(violation)
        return


def scan_command(command, findings):
    for segment in SEGMENT_SPLIT.split(command):
        segment = segment.strip()
        if segment:
            scan_segment(segment, findings)

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
    scan_command(command, findings)
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
        "Only you commit, push, branch, merge, or otherwise change git state.",
        "Reading history is fine: log, diff, show, status, blame, rev-parse.",
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

#!/usr/bin/env python3
#
# install.py - install this repo into ~/.claude.
#
# Run:     python3 scripts/install.py                link, then test and validate
#          python3 scripts/install.py --dry-run      print every action, change nothing
#          python3 scripts/install.py --skip-verify  link only, skip the trailing
#                                                    test/validate run (CI runs its
#                                                    own `make verify` right after)
#          python3 scripts/install.py --ref TAG      detach the clone at TAG first, so
#                                                    the symlinks pin to a release
#                                                    instead of the working tree
#          python3 scripts/install.py --force-copy   skip symlinks, copy directly
#          python3 scripts/install.py --target DIR   install into DIR instead
#
# Everything is a symlink, not a copy: ~/.claude/<name> points back into this
# repo, so editing a file here takes effect in the next session with no
# reinstall. An existing real file is moved to <name>.bak-<timestamp>.
# settings.local.json and CLAUDE.local.md are the exceptions - each is a real,
# untracked copy so personal prefs never land in this repo.
#
# settings.json is linked where the static "python3 ~/.claude/hooks/x.py" hook
# command works, and generated with a resolved interpreter and an absolute,
# tilde-free path where it does not. --settings link|generate overrides that.

from __future__ import annotations

import argparse
import datetime
import json
import os
import platform
import shlex
import shutil
import subprocess
import sys

HOOK_NAMES = ("git_guard", "comments", "auto_format", "context_injector")
STALE_LINKS = ("standards", "modes", "templates", "profile", "orchestration", "docs")
PYTHON_FLOOR = (3, 7)
PYTHON_FLOOR_TEXT = "3.7"
VERSION_PROBE = 'import sys; print("%d.%d.%d" % sys.version_info[:3])'


def detect_os() -> str:
    system = platform.system().lower()
    if system == "darwin":
        return "darwin"
    if system == "windows":
        return "windows"
    return "linux"


def interpreter_works(candidate: list) -> bool:
    try:
        result = subprocess.run(candidate + ["--version"], capture_output=True, timeout=5)
    except (OSError, subprocess.SubprocessError):
        return False
    return result.returncode == 0


def resolve_interpreter() -> list:
    for candidate in (["python3"], ["python"], ["py", "-3"]):
        if shutil.which(candidate[0]) is None:
            continue
        if interpreter_works(candidate):
            return candidate
    return []


def interpreter_version(interpreter: list):
    try:
        result = subprocess.run(
            interpreter + ["-c", VERSION_PROBE], capture_output=True, timeout=10
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None
    text = result.stdout.decode("utf-8", "replace").strip()
    parts = text.split(".")
    if len(parts) < 3:
        return None
    try:
        numbers = tuple(int(part) for part in parts[:3])
    except ValueError:
        return None
    return text, numbers


def check_python():
    print("")
    print("checking python3")

    interpreter = resolve_interpreter()
    if not interpreter:
        print("  python3 not found on PATH (every hook in claude-code/hooks/ and")
        print("  scripts/hooks/git/ shells out to it; floor: %s)" % PYTHON_FLOOR_TEXT)
        return None

    probed = interpreter_version(interpreter)
    if probed is None:
        print("  could not determine python3 version")
        return None

    text, numbers = probed
    name = " ".join(interpreter)
    print("  found %s %s (floor: %s)" % (name, text, PYTHON_FLOOR_TEXT))
    if numbers < PYTHON_FLOOR:
        print("  %s %s is below the required floor %s" % (name, text, PYTHON_FLOOR_TEXT))
        return None

    if interpreter != ["python3"]:
        print("  python3 itself is not on PATH: hook commands will be generated with")
        print("  %s, but scripts/hooks/git/ dispatchers still require python3" % name)

    return interpreter


def hook_command(interpreter: list, hook_path: str) -> str:
    return " ".join(shlex.quote(part) for part in interpreter + [hook_path])


def build_settings(source_settings_path: str, hooks_dir: str, interpreter: list) -> dict:
    with open(source_settings_path, "r", encoding="utf-8") as fh:
        settings = json.load(fh)

    for event_hooks in settings.get("hooks", {}).values():
        for entry in event_hooks:
            for hook in entry.get("hooks", []):
                command = hook.get("command", "")
                name = next((n for n in HOOK_NAMES if command.endswith(n + ".py")), None)
                if name is None:
                    continue
                hook_path = os.path.join(hooks_dir, name + ".py")
                hook["command"] = hook_command(interpreter, hook_path)

    return settings


def settings_strategy(mode: str, interpreter: list, force_copy: bool) -> str:
    if mode != "auto":
        return mode
    if force_copy or detect_os() == "windows" or interpreter != ["python3"]:
        return "generate"
    return "link"


def git_output(repo_root: str, args: list):
    try:
        result = subprocess.run(
            ["git", "-C", repo_root] + args, capture_output=True, timeout=60
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None
    return result.stdout.decode("utf-8", "replace").strip()


def check_ref(repo_root: str, ref: str):
    print("")
    print("checking --ref %s" % ref)
    if git_output(repo_root, ["rev-parse", "--git-dir"]) is None:
        print("  %s is not a git repository, cannot pin to a ref" % repo_root)
        return 1, ""
    if ref.startswith("-"):
        print("  ref '%s' starts with '-', refusing to pass it to git" % ref)
        return 2, ""
    commit = git_output(repo_root, ["rev-parse", "--verify", "--quiet", ref + "^{commit}"])
    if not commit:
        print("  ref '%s' does not resolve to a commit" % ref)
        return 1, ""
    if git_output(repo_root, ["status", "--porcelain"]):
        print("  working tree is dirty, refusing to pin to a ref (commit or stash first)")
        return 1, ""
    print("  ref resolves, working tree is clean")
    return 0, commit


def resolve_version(repo_root: str, ref: str) -> str:
    if ref:
        return "%s (pinned)" % ref
    described = git_output(repo_root, ["describe", "--tags", "--abbrev=0"])
    if described:
        return described
    short = git_output(repo_root, ["rev-parse", "--short", "HEAD"])
    if short:
        return short
    return "unknown"


def free_backup_path(dest: str, stamp: str) -> str:
    candidate = dest + ".bak-" + stamp
    suffix = 2
    while os.path.lexists(candidate):
        candidate = "%s.bak-%s.%d" % (dest, stamp, suffix)
        suffix += 1
    return candidate


def backup_existing(dest: str) -> None:
    if os.path.islink(dest) or not os.path.exists(dest):
        return
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    os.rename(dest, free_backup_path(dest, stamp))


def prepare_dest(dest: str) -> None:
    if os.path.islink(dest):
        os.unlink(dest)
    else:
        backup_existing(dest)


def points_into(path: str, root: str) -> bool:
    resolved = os.path.realpath(path)
    root = os.path.realpath(root)
    return resolved == root or resolved.startswith(root + os.sep)


def prune_stale(repo_root: str, target: str, dry_run: bool) -> None:
    print("")
    print("  pruning stale links from the previous layout")
    for name in STALE_LINKS:
        path = os.path.join(target, name)
        if os.path.islink(path):
            if not points_into(path, repo_root):
                print("    kept   %s (symlink outside this repo, left alone)" % name)
                continue
            print("    unlink %s" % name)
            if not dry_run:
                os.unlink(path)
        elif os.path.exists(path):
            print("    kept   %s (real directory, left alone)" % name)


def link_entry(entry: str, dest: str, force_copy: bool) -> bool:
    prepare_dest(dest)
    is_dir = os.path.isdir(entry)
    try:
        if force_copy:
            raise OSError("symlink disabled by --force-copy")
        os.symlink(entry, dest, target_is_directory=is_dir)
    except OSError:
        if is_dir:
            shutil.copytree(entry, dest)
        else:
            shutil.copy2(entry, dest)
        return True
    return False


def link_entries(source: str, target: str, force_copy: bool, skip: tuple, dry_run: bool) -> bool:
    fallback_used = False
    print("")
    print("  linking")
    for name in sorted(os.listdir(source)):
        if name in skip:
            continue
        entry = os.path.join(source, name)
        if not os.path.exists(entry):
            continue
        dest = os.path.join(target, name)
        print("    link   %s" % name)
        if dry_run:
            continue
        if link_entry(entry, dest, force_copy):
            fallback_used = True
    return fallback_used


def write_settings(target: str, settings: dict) -> None:
    dest = os.path.join(target, "settings.json")
    prepare_dest(dest)
    with open(dest, "w", encoding="utf-8") as fh:
        json.dump(settings, fh, indent=2)
        fh.write("\n")


def install_settings(source: str, target: str, interpreter: list, strategy: str, force_copy: bool, dry_run: bool) -> bool:
    entry = os.path.join(source, "settings.json")
    dest = os.path.join(target, "settings.json")

    if strategy == "link":
        print("    link   settings.json")
        if dry_run:
            return False
        if not link_entry(entry, dest, force_copy):
            return False
        os.remove(dest)
        print("    write  settings.json (symlink refused, generated instead)")
        write_settings(target, build_settings(entry, os.path.join(target, "hooks"), interpreter))
        return True

    print("    write  settings.json (generated for %s)" % " ".join(interpreter))
    if dry_run:
        return False
    write_settings(target, build_settings(entry, os.path.join(target, "hooks"), interpreter))
    return True


def overlay(source: str, target: str, dry_run: bool) -> None:
    print("")
    print("  personal overlay")
    for name in ("settings.local.json", "CLAUDE.local.md"):
        dest = os.path.join(target, name)
        tmpl = os.path.join(source, name + ".tmpl")
        if os.path.exists(dest):
            print("    kept   %s (already present)" % name)
            continue
        print("    copy   %s.tmpl -> %s" % (name, name))
        if not dry_run and os.path.exists(tmpl):
            shutil.copy2(tmpl, dest)


def print_copy_fallback_notice(target: str, interpreter: list, repo_root: str) -> None:
    script = os.path.relpath(os.path.abspath(__file__), start=repo_root)
    resync = " ".join(interpreter + [script])
    default_target = os.path.join(os.path.expanduser("~"), ".claude")
    if os.path.normcase(target) != os.path.normcase(default_target):
        resync += " --target " + target
    print("")
    print("  fell back to copy mode: symlink creation failed")
    print("")
    print("  on Windows, enable Developer Mode to get real symlinks next time:")
    print("    Settings > Privacy & security > For developers > Developer Mode")
    print("")
    print("  claude-code/ is now a plain copy, not a live link: after editing")
    print("  it again, re-sync with:")
    print("    " + resync)
    print("")


def run_verify(repo_root: str, interpreter: list) -> int:
    print("")
    for script in ("test_hooks.py", "validate.py"):
        result = subprocess.run(interpreter + [os.path.join(repo_root, "scripts", script)])
        if result.returncode != 0:
            return result.returncode
    return 0


def parse_args(argv):
    default_target = os.environ.get("AGENT_HOME") or os.path.join(os.path.expanduser("~"), ".claude")
    parser = argparse.ArgumentParser(description="install claude-code/ into a Claude Code config directory")
    parser.add_argument("--target", default=default_target, help="install into DIR instead of %s" % default_target)
    parser.add_argument("--dry-run", action="store_true", help="print every action, change nothing")
    parser.add_argument("--skip-verify", action="store_true", help="link and overlay only, skip the trailing test/validate run")
    parser.add_argument("--ref", default=None, help="detach the clone at TAG before linking (ref must resolve, working tree must be clean)")
    parser.add_argument(
        "--settings",
        choices=("auto", "link", "generate"),
        default="auto",
        help="link settings.json live, generate it for the resolved interpreter, or decide by platform",
    )
    parser.add_argument(
        "--force-copy",
        action="store_true",
        help="skip symlink attempts and copy directly (CI, restricted permissions, or Windows without Developer Mode)",
    )
    return parser.parse_args(argv)


def main(argv=None) -> int:
    args = parse_args(argv)
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    source = os.path.join(repo_root, "claude-code")

    if not args.target:
        print("install.py: empty target", file=sys.stderr)
        return 2
    if args.ref is not None and not args.ref:
        print("install.py: --ref requires a non-empty ref", file=sys.stderr)
        return 2

    target = os.path.abspath(os.path.expanduser(args.target))

    if not os.path.isdir(source):
        print("install.py: missing " + source, file=sys.stderr)
        return 1

    interpreter = check_python()
    if interpreter is None:
        return 1

    commit = ""
    if args.ref:
        code, commit = check_ref(repo_root, args.ref)
        if code:
            return code

    strategy = settings_strategy(args.settings, interpreter, args.force_copy)
    if strategy == "link" and interpreter != ["python3"]:
        print("install.py: --settings link needs python3 on PATH", file=sys.stderr)
        return 2

    print("")
    print("installing agent config")
    print("  from  " + source)
    print("  into  " + target)
    print("  os    " + detect_os())
    print("  using " + " ".join(interpreter))

    if args.ref:
        print("")
        print("  detaching at %s (%s)" % (args.ref, commit))
        if args.dry_run:
            print("    would: git -C %s checkout --detach %s" % (repo_root, commit))
        else:
            result = subprocess.run(["git", "-C", repo_root, "checkout", "--detach", commit])
            if result.returncode != 0:
                return result.returncode

    if not args.dry_run:
        os.makedirs(target, exist_ok=True)
        os.environ["AGENT_HOME"] = target

    prune_stale(repo_root, target, args.dry_run)
    fallback_used = link_entries(source, target, args.force_copy, ("settings.json",), args.dry_run)
    if install_settings(source, target, interpreter, strategy, args.force_copy, args.dry_run):
        fallback_used = True
    overlay(source, target, args.dry_run)

    if args.dry_run:
        print("")
        print("dry run complete, nothing changed")
        return 0

    if fallback_used:
        print_copy_fallback_notice(target, interpreter, repo_root)

    if not args.skip_verify:
        code = run_verify(repo_root, interpreter)
        if code:
            return code

    print("installed %s into %s" % (resolve_version(repo_root, args.ref or ""), target))
    print("restart Claude Code to pick up settings.json and hooks")
    print("")
    return 0


if __name__ == "__main__":
    sys.exit(main())

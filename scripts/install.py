#!/usr/bin/env python3
#
# install.py - cross-platform installer for claude-code/ into ~/.claude/.
#
# Run:     python3 scripts/install.py              symlink, then generate
#          python3 scripts/install.py --dry-run     print every action, change nothing
#          python3 scripts/install.py --force-copy  skip symlinks, copy directly
#          python3 scripts/install.py --target DIR  install into DIR instead
#
# setup.sh remains the macOS/Linux bash entry point. This is the
# entry point for Windows, and for any shell where bash + `ln -s`
# are not both guaranteed: it detects which of python3/python/py -3
# actually resolves, then writes every hook's "command" field in
# settings.json with that interpreter and an absolute, tilde-free
# path instead of the static "python3 ~/.claude/hooks/x.py" string.
# Every claude-code/* entry is symlinked into the target the same
# way setup.sh does; a target where symlink creation fails (e.g.
# Windows without Developer Mode/admin) falls back to a plain copy.

import argparse
import datetime
import json
import os
import platform
import shlex
import shutil
import subprocess
import sys

HOOK_NAMES = ("git_guard", "comment_guard", "auto_format", "context_injector")


def detect_os():
    system = platform.system().lower()
    if system == "darwin":
        return "darwin"
    if system == "windows":
        return "windows"
    return "linux"


def interpreter_works(candidate):
    try:
        result = subprocess.run(candidate + ["--version"], capture_output=True, timeout=5)
    except (OSError, subprocess.SubprocessError):
        return False
    return result.returncode == 0


def resolve_interpreter():
    for candidate in (["python3"], ["python"], ["py", "-3"]):
        if shutil.which(candidate[0]) is None:
            continue
        if interpreter_works(candidate):
            return candidate
    raise RuntimeError("no working python3, python, or py -3 interpreter found on PATH")


def hook_command(interpreter, hook_path):
    # WHY: shlex.quote is POSIX-only; PowerShell-only dispatch is a known residual gap.
    return " ".join(shlex.quote(part) for part in interpreter + [hook_path])


def build_settings(source_settings_path, hooks_dir, interpreter):
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


def backup_existing(dest):
    if os.path.islink(dest) or not os.path.exists(dest):
        return
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    os.rename(dest, dest + ".bak-" + stamp)


def prepare_dest(dest):
    if os.path.islink(dest):
        os.unlink(dest)
    else:
        backup_existing(dest)


def link_or_copy_entries(source, target, force_copy):
    fallback_used = False
    for name in sorted(os.listdir(source)):
        if name == "settings.json":
            continue
        entry = os.path.join(source, name)
        dest = os.path.join(target, name)

        prepare_dest(dest)

        is_dir = os.path.isdir(entry)
        try:
            if force_copy:
                raise OSError("symlink disabled by --force-copy")
            os.symlink(entry, dest, target_is_directory=is_dir)
        except OSError:
            fallback_used = True
            if is_dir:
                shutil.copytree(entry, dest)
            else:
                shutil.copy2(entry, dest)

    return fallback_used


def write_settings(target, settings):
    dest = os.path.join(target, "settings.json")
    prepare_dest(dest)
    with open(dest, "w", encoding="utf-8") as fh:
        json.dump(settings, fh, indent=2)
        fh.write("\n")


def overlay(source, target):
    for name in ("settings.local.json", "CLAUDE.local.md"):
        dest = os.path.join(target, name)
        tmpl = os.path.join(source, name + ".tmpl")
        if os.path.exists(dest) or not os.path.exists(tmpl):
            continue
        shutil.copy2(tmpl, dest)


def print_copy_fallback_notice(target, interpreter, repo_root):
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


def parse_args(argv):
    default_target = os.environ.get("AGENT_HOME") or os.path.join(os.path.expanduser("~"), ".claude")
    parser = argparse.ArgumentParser(description="install claude-code/ into a Claude Code config directory")
    parser.add_argument("--target", default=default_target, help="install into DIR instead of %s" % default_target)
    parser.add_argument("--dry-run", action="store_true", help="print every action, change nothing")
    parser.add_argument(
        "--force-copy",
        action="store_true",
        help="skip symlink attempts and copy directly (CI, restricted permissions, or Windows without Developer Mode)",
    )
    return parser.parse_args(argv)


def main(argv=None):
    args = parse_args(argv)
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    source = os.path.join(repo_root, "claude-code")
    target = os.path.abspath(os.path.expanduser(args.target))

    if not os.path.isdir(source):
        print("install.py: missing " + source, file=sys.stderr)
        return 1

    try:
        interpreter = resolve_interpreter()
    except RuntimeError as err:
        print("install.py: " + str(err), file=sys.stderr)
        return 1

    os_name = detect_os()

    print("")
    print("installing agent config")
    print("  from  " + source)
    print("  into  " + target)
    print("  os    " + os_name)
    print("  using " + " ".join(interpreter))
    print("")

    if args.dry_run:
        print("dry run complete, nothing changed")
        return 0

    os.makedirs(target, exist_ok=True)

    fallback_used = link_or_copy_entries(source, target, args.force_copy)

    settings = build_settings(os.path.join(source, "settings.json"), os.path.join(target, "hooks"), interpreter)
    write_settings(target, settings)

    overlay(source, target)

    if fallback_used:
        print_copy_fallback_notice(target, interpreter, repo_root)

    print("installed into " + target)
    print("restart Claude Code to pick up settings.json and hooks")
    print("")
    return 0


if __name__ == "__main__":
    sys.exit(main())

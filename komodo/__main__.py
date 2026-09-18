"""komodo: the command line for the harness."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
import time
import uuid
from typing import List, Optional, Sequence, Tuple

from . import __version__, comments, doctor, gates, gitops, pr, tasks
from .config import Config, ConfigError
from .state import Store

SCRATCH_TIMEOUT_S = 60
BROKEN_COMMAND_CODES = (9009,) if os.name == "nt" else (126, 127)
# cmd.exe reports an unknown command through this text, sometimes with exit 1 rather than 9009.
WINDOWS_NOT_FOUND_RE = re.compile(r"is not recognized as an internal or external command|cannot find the (path|file) specified", re.IGNORECASE)
# Network and destructive tools a planner-authored done_when must never invoke unattended.
UNSAFE_DONE_WHEN_RE = re.compile(
    r"\b(rm\s+-[a-z]*r[a-z]*f|sudo|curl|wget|nc|ncat|netcat|ssh|scp|sftp|rsync|dd|mkfs|chmod|chown|kill(all)?"
    r"|shutdown|reboot|telnet|base64\s+-d)\b|\|\s*(sh|bash|zsh)\b",
    re.IGNORECASE,
)


def repo_root(start: Optional[str] = None) -> str:
    """The git toplevel for the cwd, or the cwd itself."""
    result = subprocess.run(["git", "rev-parse", "--show-toplevel"], cwd=start or os.getcwd(), capture_output=True, text=True)
    return result.stdout.strip() or os.path.abspath(start or os.getcwd())


def load_config(root: str, profile: Optional[str] = None) -> Config:
    """Config for the repo, with an optional profile override."""
    overrides = {"profile": profile} if profile else None
    return Config.load(root, overrides)


def cmd_run(args: argparse.Namespace) -> int:
    """komodo run [group] [--profile fast|thinking] [--dry-run] [--resume]"""
    from .pipeline import Pipeline, PipelineError

    root = repo_root()
    config = load_config(root, args.profile)
    pipeline = Pipeline(root, config, profile=args.profile, dry_run=args.dry_run)
    try:
        state = pipeline.run(args.group, resume=args.resume)
    except PipelineError as error:
        print("komodo run: %s" % error, file=sys.stderr)
        return 2
    except gitops.GitRefused as error:
        print("komodo run: refused: %s" % error, file=sys.stderr)
        return 3
    if args.dry_run:
        return 0
    return 1 if state.blocked or any("no PR opened" in note or "unpushed" in note for note in state.notes) else 0


def cmd_status(args: argparse.Namespace) -> int:
    """komodo status [--prune]: open runs, stale worktrees, merged branches."""
    root = repo_root()
    config = load_config(root)
    store = Store(root)
    git = gitops.Git(root, config.protected, config.remote)
    runs = store.list()
    print("runs: %d on disk" % len(runs))
    for run_id in runs[:5]:
        try:
            state = store.load(run_id)
        except (OSError, ValueError, TypeError):
            continue
        status = "finished" if state.finished else "open (%s)" % state.phase
        print("  %s  %s  %s  $%.2f  %s" % (run_id, state.group_id, status, state.cost_usd, state.pr_url or state.branch))
    worktrees = git.worktrees()
    for path in worktrees:
        print("stale worktree: %s" % path)
    try:
        base = str(config.get("base") or "") or git.default_base()
        merged = [name for name in git.merged_branches(base) if name != git.current_branch()]
    except gitops.GitError as error:
        print("git: %s" % error)
        merged = []
    if merged:
        if pr.available(root):
            merged = pr.merged_prs_for(root, merged) or merged
        for name in merged:
            print("merged branch: %s" % name)
    if args.prune:
        for path in worktrees:
            git.worktree_remove(path)
            print("removed worktree %s" % path)
        for name in merged:
            git.delete_branch(name)
            print("deleted branch %s" % name)
    if args.json:
        print(json.dumps({"runs": runs, "worktrees": worktrees, "merged": merged}))
    return 0


def _backlog_path(root: str) -> str:
    """The backlog path or a usage error."""
    path = tasks.find_backlog(root)
    if path is None:
        print("no BACKLOG.md found under %s" % root, file=sys.stderr)
        sys.exit(2)
    return path


def cmd_tasks(args: argparse.Namespace) -> int:
    """komodo tasks lint|list|migrate|add|plan"""
    root = repo_root()
    if args.action == "lint":
        backlog = tasks.load(_backlog_path(root))
        problems = tasks.lint(backlog)
        for problem in problems:
            print(problem)
        print("%d task(s), %d group(s), %d problem(s)" % (len(backlog.tasks), len(backlog.groups), len(problems)))
        return 1 if problems else 0
    if args.action == "list":
        backlog = tasks.load(_backlog_path(root))
        for group in backlog.groups:
            print("%s  %s  [%s]  %d open" % (group.id, group.title, group.mode, len(group.open_tasks)))
            for task in group.tasks:
                print("  %s  [%s] [%s]  %s" % (task.id, task.priority, task.status, task.title))
        return 0
    if args.action == "migrate":
        path = _backlog_path(root)
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
        new_text, report = tasks.migrate(text)
        if args.write:
            with open(path, "w", encoding="utf-8") as handle:
                handle.write(new_text)
            print("wrote %s" % path)
        else:
            sys.stdout.write(new_text)
        for line in report:
            print(line, file=sys.stderr)
        return 0
    if args.action == "add":
        path = _backlog_path(root)
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
        fields = {"files": args.files or [], "done_when": args.done_when or []}
        if args.depends_on:
            fields["depends_on"] = args.depends_on
        if args.context:
            fields["context"] = args.context
        if args.type:
            fields["type"] = args.type
        if args.owner and args.owner != "agent":
            fields["owner"] = args.owner
        new_text, task_id = tasks.append_task(text, args.group, args.title, fields, args.priority)
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(new_text)
        print(task_id)
        return 0
    if args.action == "plan":
        return cmd_plan(args, root)
    return 2


def _cannot_execute(result: gates.CommandResult) -> bool:
    """Whether the shell could not run the command at all, as opposed to running it and getting a failure."""
    if result.returncode in BROKEN_COMMAND_CODES:
        return True
    return os.name == "nt" and result.returncode != 0 and bool(WINDOWS_NOT_FOUND_RE.search(result.output))


def validate_done_when(root: str, config: Config, commands: Sequence[str]) -> Tuple[List[str], List[str]]:
    """Runs each command once in a scratch worktree, dropping any that can't even execute."""
    if not commands:
        return [], []
    git = gitops.Git(root, config.protected, config.remote)
    scratch = tempfile.mkdtemp(prefix="komodo-plan-")
    branch = "chore/plan-validate-%s" % uuid.uuid4().hex[:8]
    kept: List[str] = []
    problems: List[str] = []
    try:
        git.worktree_add(scratch, branch, "HEAD")
        for command in commands:
            if UNSAFE_DONE_WHEN_RE.search(command):
                problems.append("done_when %r not run: matches a network or destructive tool, needs human confirmation" % command)
                continue
            result = gates.run_command(command, scratch, SCRATCH_TIMEOUT_S)
            if _cannot_execute(result):
                problems.append("done_when %r did not run: %s" % (command, result.output.strip().splitlines()[-1] if result.output.strip() else "exit %d" % result.returncode))
            else:
                kept.append(command)
    finally:
        git.worktree_remove(scratch, branch)
    return kept, problems


def cmd_plan(args: argparse.Namespace, root: str) -> int:
    """Runs the planner worker over a goal and appends the tasks it proposes to a group."""
    from . import briefs, standards
    from .workers import Brief, worker_for

    config = load_config(root, args.profile)
    path = _backlog_path(root)
    with open(path, encoding="utf-8") as handle:
        text = handle.read()
    backlog = tasks.parse(text)
    group = backlog.group(args.group)
    if group is None:
        print("group %s not found" % args.group, file=sys.stderr)
        return 2
    spec_parts = []
    for relative in ("docs/spec/SDD.md", "docs/spec/PRD.md"):
        full = os.path.join(root, relative)
        if os.path.isfile(full):
            with open(full, encoding="utf-8", errors="replace") as handle:
                spec_parts.append("### %s\n%s" % (relative, briefs.clip(handle.read(), 20000, relative)))
    layout = subprocess.run(["git", "ls-files"], cwd=root, capture_output=True, text=True).stdout
    slots = {
        "goal": args.goal, "layout": briefs.clip(layout, 8000, "file list"),
        "spec": "\n\n".join(spec_parts) or "No spec files under docs/spec/.",
        "existing": "\n".join("- %s %s" % (task.id, task.title) for task in group.tasks) or "none",
    }
    system, prompt = briefs.render("planner", slots)
    spec = config.role("planner", args.profile)
    brief = Brief(role="planner", system=system, prompt=prompt, cwd=root, spec=spec, schema=briefs.schema_for("planner"), tools=briefs.tools_for("planner"), timeout_s=int(config.get("worker_timeout_s", 900)), env=gitops.worker_env())
    result = worker_for(str(spec["provider"]), config).run(brief)
    if not result.ok or not result.data:
        print("planner failed: %s" % result.error, file=sys.stderr)
        return 1
    proposed = result.data.get("tasks", [])
    ids: List[str] = []
    gaps: List[str] = list(result.data.get("gaps", []))
    for index, item in enumerate(proposed):
        done_when, broken = validate_done_when(root, config, [str(c) for c in item.get("done_when", [])])
        gaps.extend(broken)
        fields = {"files": item.get("files", []), "done_when": done_when}
        deps = [ids[i] for i in item.get("depends_on", []) if isinstance(i, int) and 0 <= i < len(ids)]
        if deps:
            fields["depends_on"] = deps
        if item.get("context"):
            fields["context"] = item["context"]
        if item.get("type") in tasks.TYPES:
            fields["type"] = item["type"]
        text, task_id = tasks.append_task(text, group.id, str(item.get("title", "task %d" % index)), fields, str(item.get("priority", "M")))
        ids.append(task_id)
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(text)
    print("added %d task(s) to %s: %s" % (len(ids), group.id, ", ".join(ids)))
    for gap in gaps:
        print("gap: %s" % gap)
    problems = tasks.lint(tasks.parse(text))
    for problem in problems:
        print("lint: %s" % problem)
    print("cost $%.2f in %ss" % (result.cost_usd, result.seconds))
    return 1 if problems else 0


def cmd_comments(args: argparse.Namespace) -> int:
    """komodo comments check [paths] [--base REF] [--all] [--staged]"""
    root = repo_root()
    config = load_config(root)
    findings = comments.check(root, args.paths, base=args.base, all_lines=args.all, staged=args.staged,
                              require=str(config.get("comments.require", "nonobvious")), trivial_lines=int(config.get("comments.trivial_lines", 8)))
    if args.json:
        print(json.dumps({"findings": findings}, indent=2))
    else:
        print(comments.format_findings(findings))
    return 1 if findings else 0


def cmd_hooks(args: argparse.Namespace) -> int:
    """komodo hooks install|status <repo>..."""
    from . import hooks

    repos = [os.path.abspath(path) for path in (args.repos or [repo_root()])]
    for repo, outcome in hooks.install(repos, status_only=args.action == "status"):
        print("%-50s %s" % (os.path.basename(repo.rstrip(os.sep)), outcome))
    return 0


def cmd_install(args: argparse.Namespace) -> int:
    """komodo install [--target DIR] [--dry-run]"""
    from . import install

    config = None
    try:
        config = load_config(repo_root())
    except ConfigError:
        pass
    return install.install(args.target, args.dry_run, adapter=args.adapter, config=config)


def cmd_doctor(args: argparse.Namespace) -> int:
    """komodo doctor: dangling references, policy leaks, skill frontmatter, git leftovers."""
    root = repo_root()
    try:
        protected = load_config(root).protected
    except ConfigError as error:
        print("config: %s" % error)
        protected = ["main", "master"]
    problems = doctor.run(root, protected, git_checks=not args.no_git)
    for problem in problems:
        print(problem)
    print("%d problem(s)" % len(problems))
    return 1 if problems else 0


def cmd_pr(args: argparse.Namespace) -> int:
    """komodo pr label|comment|reply|threads|sync|respond"""
    from . import pr_actions

    root = repo_root()
    config = load_config(root, getattr(args, "profile", None))
    return pr_actions.dispatch(args, root, config)


def _read(path: str) -> str:
    """File contents as text."""
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def release_check(root: str, path: str) -> int:
    """Reports changelog and tag drift read-only, without touching the repo or the remote."""
    from . import render

    text = _read(path)
    tags = subprocess.run(["git", "tag", "--list", "v*"], cwd=root, capture_output=True, text=True).stdout.split()
    problems = render.changelog_drift(text, tags)
    for version in render.never_released_versions(text):
        print("note: %s is marked never released, so no tag is expected" % version)
    for problem in problems:
        print(problem)
    print("release check: %d problem(s)" % len(problems))
    return 1 if problems else 0


def cmd_release(args: argparse.Namespace) -> int:
    """komodo release [check]: audit changelog and tag drift, or tag the newest version."""
    root = repo_root()
    config = load_config(root)
    path = os.path.join(root, str(config.get("changelog", "CHANGELOG.md")))
    if not os.path.isfile(path):
        print("no changelog at %s" % path, file=sys.stderr)
        return 2
    if getattr(args, "action", None) == "check":
        return release_check(root, path)
    version = None
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            if line.startswith("## ["):
                version = line.split("[", 1)[1].split("]", 1)[0].strip()
                break
    if not version:
        print("no released version heading in the changelog", file=sys.stderr)
        return 2
    tag = "v" + version
    git = gitops.Git(root, config.protected, config.remote)
    if subprocess.run(["git", "rev-parse", "-q", "--verify", "refs/tags/" + tag], cwd=root, capture_output=True).returncode == 0:
        print("%s already exists" % tag)
        return 0
    if args.dry_run:
        print("would tag %s at HEAD and push it" % tag)
        return 0
    git.run("tag", "-a", tag, "-m", "release %s" % version)
    git.run("push", config.remote, tag)
    print("tagged and pushed %s" % tag)
    return 0


def build_parser() -> argparse.ArgumentParser:
    """The CLI surface."""
    parser = argparse.ArgumentParser(prog="komodo", description="Run task groups through workers with the guarantees in code.")
    parser.add_argument("--version", action="version", version="komodo %s" % __version__)
    sub = parser.add_subparsers(dest="command", required=True)

    run = sub.add_parser("run", help="run one task group end to end")
    run.add_argument("group", nargs="?", help="TG id or a substring of its id or title; default: first open group")
    run.add_argument("--profile", choices=None, help="fast or thinking (or any profile in komodo.json)")
    run.add_argument("--dry-run", action="store_true", help="print waves, briefs, and token estimates; spawn nothing")
    run.add_argument("--resume", action="store_true", help="continue the unfinished run for this group")
    run.set_defaults(func=cmd_run)

    status = sub.add_parser("status", help="runs on disk, stale worktrees, merged branches")
    status.add_argument("--prune", action="store_true", help="remove stale worktrees and delete merged branches")
    status.add_argument("--json", action="store_true")
    status.set_defaults(func=cmd_status)

    task_parser = sub.add_parser("tasks", help="lint, list, migrate, add, or plan backlog tasks")
    task_sub = task_parser.add_subparsers(dest="action", required=True)
    task_sub.add_parser("lint")
    task_sub.add_parser("list")
    migrate = task_sub.add_parser("migrate")
    migrate.add_argument("--write", action="store_true", help="rewrite BACKLOG.md in place instead of printing")
    add = task_sub.add_parser("add")
    add.add_argument("group")
    add.add_argument("title")
    add.add_argument("--files", nargs="+")
    add.add_argument("--done-when", nargs="+", dest="done_when")
    add.add_argument("--depends-on", nargs="*", dest="depends_on")
    add.add_argument("--context", nargs="*")
    add.add_argument("--priority", default="M", choices=list(tasks.PRIORITIES))
    add.add_argument("--type", choices=list(tasks.TYPES))
    add.add_argument("--owner", default="agent", choices=list(tasks.OWNERS))
    plan = task_sub.add_parser("plan")
    plan.add_argument("group")
    plan.add_argument("goal")
    plan.add_argument("--profile")
    task_parser.set_defaults(func=cmd_tasks)

    comment = sub.add_parser("comments", help="comment lint")
    comment_sub = comment.add_subparsers(dest="action", required=True)
    check = comment_sub.add_parser("check")
    check.add_argument("paths", nargs="*")
    check.add_argument("--base", default="HEAD")
    check.add_argument("--all", action="store_true")
    check.add_argument("--staged", action="store_true")
    check.add_argument("--json", action="store_true")
    comment.set_defaults(func=cmd_comments)

    hooks = sub.add_parser("hooks", help="install the git hooks into repos")
    hooks.add_argument("action", choices=("install", "status"))
    hooks.add_argument("repos", nargs="*")
    hooks.set_defaults(func=cmd_hooks)

    install = sub.add_parser("install", help="render an adapter and install it by copy (default: claude into ~/.claude)")
    install.add_argument("--adapter", default="claude", choices=("claude",))
    install.add_argument("--target")
    install.add_argument("--dry-run", action="store_true")
    install.set_defaults(func=cmd_install)

    doc = sub.add_parser("doctor", help="find fragments: dangling references, policy leaks, leftovers")
    doc.add_argument("--no-git", action="store_true", help="skip worktree and branch checks")
    doc.set_defaults(func=cmd_doctor)

    pr_parser = sub.add_parser("pr", help="pull request actions on the current branch's own PR")
    pr_sub = pr_parser.add_subparsers(dest="action", required=True)
    pr_sub.add_parser("threads", help="list unresolved review threads")
    label = pr_sub.add_parser("label")
    label.add_argument("labels", nargs="*")
    label.add_argument("--auto", action="store_true", help="derive labels from the PR's commit type and komodo.json")
    comment_pr = pr_sub.add_parser("comment")
    comment_pr.add_argument("body")
    reply = pr_sub.add_parser("reply")
    reply.add_argument("comment_id", type=int)
    reply.add_argument("body")
    sync = pr_sub.add_parser("sync", help="merge the base into this branch, resolving conflicts with a worker")
    sync.add_argument("--profile")
    respond = pr_sub.add_parser("respond", help="answer every unresolved review thread with a worker")
    respond.add_argument("--profile")
    respond.add_argument("--dry-run", action="store_true")
    pr_parser.set_defaults(func=cmd_pr)

    release = sub.add_parser("release", help="audit changelog and tag drift, or tag the newest version")
    release.add_argument("action", nargs="?", choices=("check",), help="check: report changelog and tag drift read-only, exiting non-zero on any")
    release.add_argument("--dry-run", action="store_true")
    release.set_defaults(func=cmd_release)
    return parser


def main(argv: Optional[List[str]] = None) -> int:
    """Entry point."""
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return int(args.func(args))
    except ConfigError as error:
        print("komodo: config: %s" % error, file=sys.stderr)
        return 2
    except gitops.GitError as error:
        print("komodo: %s" % error, file=sys.stderr)
        return 3


if __name__ == "__main__":
    sys.exit(main())

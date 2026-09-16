"""PR subcommands: label, comment, reply, threads, sync (conflict resolution), respond (review threads)."""

from __future__ import annotations

import argparse
import os
import sys
from typing import List

from . import briefs, gates, gitops, pr, tasks
from .config import Config
from .workers import Brief, worker_for


def _own_pr(root: str) -> dict:
    """The current branch's PR or a usage error."""
    info = pr.view(root)
    if not info:
        print("no pull request for the current branch", file=sys.stderr)
        sys.exit(2)
    return info


def _worker_call(config: Config, role: str, system: str, prompt: str, cwd: str, profile: str = None):
    """One worker invocation for a PR action."""
    spec = config.role(role, profile)
    brief = Brief(role=role, system=system, prompt=prompt, cwd=cwd, spec=spec, schema=briefs.schema_for(role),
                  tools=briefs.tools_for(role), timeout_s=int(config.get("worker_timeout_s", 900)), env=gitops.worker_env())
    return worker_for(str(spec["provider"]), config).run(brief)


def _verify_commands(root: str) -> List[str]:
    """The repo verify command as a list, or empty."""
    command = gates.resolve_verify(root)
    return [command] if command else []


def sync(root: str, config: Config, profile: str = None) -> int:
    """Merges the base branch into the current branch; a merger worker resolves conflicts, the orchestrator verifies and commits."""
    git = gitops.Git(root, config.protected, config.remote)
    branch = git.current_branch()
    if branch is None or git.is_protected(branch):
        print("sync runs on an unprotected feature branch, not %r" % branch, file=sys.stderr)
        return 2
    if not git.is_clean():
        print("working tree is not clean", file=sys.stderr)
        return 2
    git.fetch()
    base = str(config.get("base") or "") or git.default_base()
    incoming = "%s/%s" % (config.remote, base)
    if git.merge(incoming):
        print("merged %s into %s cleanly" % (incoming, branch))
        return 0
    conflicted = git.conflicted_files()
    print("conflicts in %d file(s): %s" % (len(conflicted), ", ".join(conflicted)))
    ours = git.run("log", "--oneline", "-10", "%s..%s" % (incoming, branch))
    theirs = git.run("log", "--oneline", "-10", "%s..%s" % (branch, incoming))
    system, prompt = briefs.render("merger", {"incoming": incoming, "branch": branch, "files": "\n".join("- " + path for path in conflicted), "ours": ours or "(none)", "theirs": theirs or "(none)"})
    result = _worker_call(config, "builder", system, prompt, root, profile)
    escalate = [item for item in ((result.data or {}).get("escalate") or [])] if result.ok else [{"file": path, "reason": result.error} for path in conflicted]
    remaining = git.files_with_markers(conflicted)
    if escalate or remaining:
        git.merge_abort()
        for item in escalate:
            print("escalate %s: %s" % (item.get("file"), item.get("reason")))
        for path in remaining:
            print("markers remain in %s" % path)
        print("merge aborted; resolve by hand")
        return 1
    verify = gates.run_gate("verify", _verify_commands(root), root, int(config.get("worker_timeout_s", 900)))
    if not verify.ok:
        git.merge_abort()
        print(verify.failures()[0].output)
        print("verify failed after resolution; merge aborted")
        return 1
    git.run("add", "-A")
    git.run("commit", "-q", "--no-edit")
    print("resolved %d file(s) and committed the merge; cost $%.2f" % (len(conflicted), result.cost_usd))
    return 0


def respond(root: str, config: Config, profile: str = None, dry_run: bool = False) -> int:
    """Answers each unresolved review thread with a responder worker, committing code changes and replying."""
    git = gitops.Git(root, config.protected, config.remote)
    info = _own_pr(root)
    number = int(info["number"])
    threads = pr.review_threads(root, number)
    if not threads:
        print("no unresolved review threads on #%d" % number)
        return 0
    branch = git.current_branch()
    if branch is None or git.is_protected(branch):
        print("respond runs on the PR's own branch", file=sys.stderr)
        return 2
    backlog = tasks.load(tasks.find_backlog(root)) if tasks.find_backlog(root) else None
    done_when = _verify_commands(root)
    replied = 0
    for thread in threads:
        path = thread["path"]
        line = int(thread.get("line") or 0)
        excerpt = ""
        full = os.path.join(root, path)
        if os.path.isfile(full):
            with open(full, encoding="utf-8", errors="replace") as handle:
                lines = handle.read().splitlines()
            start, end = max(0, line - 15), min(len(lines), line + 15)
            excerpt = "\n".join("%d: %s" % (index + 1, text) for index, text in enumerate(lines[start:end], start))
        conversation = "\n\n".join("**%s** (%s):\n%s" % (c["author"], c["at"], c["body"]) for c in thread["comments"])
        if backlog:
            for task in backlog.tasks:
                if path in task.files:
                    done_when = task.done_when + [cmd for cmd in done_when if cmd not in task.done_when]
                    break
        system, prompt = briefs.render("responder", {"pr_number": number, "path": path, "line": line, "thread": conversation, "excerpt": excerpt or "(file not found)", "done_when": "\n".join("- `%s`" % c for c in done_when) or "- none declared"})
        if dry_run:
            print("would answer thread on %s:%d (%d comment(s))" % (path, line, len(thread["comments"])))
            continue
        result = _worker_call(config, "builder", system, prompt, root, profile)
        if not result.ok or not result.data:
            print("thread %s:%d: worker failed: %s" % (path, line, result.error))
            continue
        data = result.data
        if data.get("result") == "CHANGED":
            gate = gates.run_gate("done_when", done_when, root, int(config.get("worker_timeout_s", 900)))
            if not gate.ok:
                git.run("checkout", "--", ".")
                print("thread %s:%d: change failed verification; reverted" % (path, line))
                continue
            git.commit(gitops.commit_message("fix", "address review on %s" % os.path.basename(path), [str(item.get("what", "")) for item in data.get("changed", [])]))
        last_id = thread["comments"][-1]["id"] if thread["comments"] else None
        if last_id:
            pr.reply(root, number, int(last_id), str(data.get("reply", "")).strip())
            replied += 1
        print("thread %s:%d: %s" % (path, line, data.get("result")))
    if replied and not dry_run:
        try:
            git.push(branch)
        except gitops.GitRefused as error:
            print("push refused: %s" % error)
    print("answered %d thread(s)" % replied)
    return 0


def dispatch(args: argparse.Namespace, root: str, config: Config) -> int:
    """Routes a pr subcommand."""
    if args.action == "threads":
        info = _own_pr(root)
        for thread in pr.review_threads(root, int(info["number"])):
            print("%s:%s  %d comment(s)  last by %s" % (thread["path"], thread["line"], len(thread["comments"]), thread["comments"][-1]["author"] if thread["comments"] else "?"))
        return 0
    if args.action == "label":
        info = _own_pr(root)
        defined = pr.existing_labels(root)
        unknown = [label for label in args.labels if label not in defined]
        if unknown:
            print("label(s) not defined in the repo: %s" % ", ".join(unknown), file=sys.stderr)
            return 2
        pr.edit(root, int(info["number"]), add_labels=args.labels)
        print("labelled #%d: %s" % (info["number"], ", ".join(args.labels)))
        return 0
    if args.action == "comment":
        info = _own_pr(root)
        pr.comment(root, int(info["number"]), args.body)
        return 0
    if args.action == "reply":
        info = _own_pr(root)
        pr.reply(root, int(info["number"]), args.comment_id, args.body)
        return 0
    if args.action == "sync":
        return sync(root, config, args.profile)
    if args.action == "respond":
        return respond(root, config, args.profile, args.dry_run)
    return 2

"""The assembly line: preflight, branch, build waves, verify, review, publish, report."""

from __future__ import annotations

import concurrent.futures
import os
import re
import shutil
import time
from typing import Any, Callable, Dict, List, Optional, Sequence

from . import briefs, dag, gates, gitops, pr, render, standards, tasks
from .config import Config
from .state import RunState, Store, WorkerRecord
from .tasks import Backlog, Group, Task
from .workers import Brief, Result, worker_for

SEVERITY_RANK = {"low": 0, "medium": 1, "high": 2, "critical": 3}
WORKTREE_DIR = os.path.join(".komodo", "wt")


class PipelineError(RuntimeError):
    """A condition that stops the run before or between phases, with a message for the human."""


class Pipeline:
    """Runs one task group end to end; every model call goes through a worker, every git write through gitops."""

    def __init__(self, root: str, config: Config, profile: Optional[str] = None, dry_run: bool = False,
                 log: Callable[[str], None] = print, worker_factory: Callable[..., Any] = worker_for):
        self.root = os.path.abspath(root)
        self.config = config
        self.profile = profile or str(config.get("profile"))
        self.dry_run = dry_run
        self.log = log
        self.worker_factory = worker_factory
        self.git = gitops.Git(self.root, config.protected, config.remote)
        self.store = Store(self.root)
        self.backlog_path = tasks.find_backlog(self.root)
        self.backlog: Optional[Backlog] = None
        self.group: Optional[Group] = None
        self.state: Optional[RunState] = None
        self.started = time.time()

    # ----- helpers -----------------------------------------------------------------

    def spec(self, role: str) -> Dict[str, Any]:
        """The role spec under the active profile."""
        return self.config.role(role, self.profile)

    def record(self, role: str, task_id: str, result: Result) -> None:
        """Appends a worker's cost and outcome to the run state."""
        assert self.state is not None
        self.state.workers.append(WorkerRecord(
            role=role, task_id=task_id, provider=result.provider, model=result.model, ok=result.ok,
            seconds=result.seconds, cost_usd=result.cost_usd, input_tokens=result.input_tokens,
            output_tokens=result.output_tokens, turns=result.turns, error=result.error[:300],
        ))
        self.store.save(self.state)

    def call(self, role: str, task_id: str, system: str, prompt: str, cwd: str) -> Result:
        """Runs one worker for a role and records it."""
        spec = self.spec(role)
        brief = Brief(
            role=role, system=system, prompt=prompt, cwd=cwd, spec=spec, schema=briefs.schema_for(role),
            tools=briefs.tools_for(role), timeout_s=int(self.config.get("worker_timeout_s", 900)),
            env=gitops.worker_env(), task_id=task_id,
        )
        if self.dry_run:
            estimate = brief.estimated_tokens
            self.log("  dry-run %s for %s: %s/%s effort=%s ~%d system + %d prompt tokens" % (
                role, task_id, spec.get("provider"), spec.get("model"), spec.get("effort", "-"), estimate["system"], estimate["prompt"]))
            return Result(ok=True, data=None, provider=str(spec.get("provider")), model=str(spec.get("model")))
        worker = self.worker_factory(str(spec["provider"]), self.config)
        result = worker.run(brief)
        self.record(role, task_id, result)
        return result

    def over_budget(self) -> bool:
        """Whether the group's wall-clock budget is spent."""
        return time.time() - self.started > float(self.config.get("group_budget_s", 3600))

    def read(self, path: str, cap: int) -> str:
        """A file's contents clipped, or a marker when absent."""
        try:
            with open(path, encoding="utf-8", errors="replace") as handle:
                return briefs.clip(handle.read(), cap, os.path.basename(path))
        except OSError:
            return "[%s does not exist yet]" % os.path.relpath(path, self.root)

    def repo_rules(self) -> str:
        """The repo's own AGENTS.md, clipped, or a one-line default."""
        for name in ("AGENTS.md", "CLAUDE.md"):
            path = os.path.join(self.root, name)
            if os.path.isfile(path):
                return self.read(path, 6000)
        return "No repo-level rules file. Follow the standards below and the code's existing idioms."

    def task_slots(self, task: Task, cwd: str, failure: str = "") -> Dict[str, Any]:
        """Every slot the builder brief needs for one task, read from the worktree."""
        file_cap = int(self.config.get("context.file_chars", 24000))
        per_file = max(2000, file_cap // max(1, len(task.files)))
        files = "\n\n".join("### %s\n```\n%s\n```" % (path, self.read(os.path.join(cwd, path), per_file)) for path in task.files) or "No files listed."
        context_parts = []
        for ref in task.context:
            path, _, anchor = ref.partition("#")
            text = self.read(os.path.join(cwd, path), 8000)
            if anchor:
                text = _section(text, anchor) or text
            context_parts.append("### %s\n%s" % (ref, text))
        names = standards.names_for(task.files, "builder")
        return {
            "task_id": task.id, "title": task.title,
            "task_block": "\n".join(self.backlog.lines[task.block_start + 1: task.block_end]) if self.backlog and task.block_start >= 0 else "",
            "repo_rules": self.repo_rules(),
            "context": "\n\n".join(context_parts) or "None beyond the files below.",
            "files": files,
            "standards": standards.load(names, int(self.config.get("context.standards_chars", 6000))),
            "done_when": "\n".join("- `%s`" % command for command in task.done_when),
            "failure": ("\n# Previous attempt failed\nFix the cause. Never weaken the check.\n```\n%s\n```" % failure) if failure else "",
            "comment_convention": standards.comment_convention(task.files),
        }

    # ----- phases ------------------------------------------------------------------

    def preflight(self, needle: Optional[str], resume: bool) -> None:
        """Reads the backlog, picks the group, checks the tree, and creates or resumes state."""
        if self.backlog_path is None:
            raise PipelineError("no BACKLOG.md in %s; write one in the task grammar first" % self.root)
        self.backlog = tasks.load(self.backlog_path)
        problems = tasks.lint(self.backlog)
        if problems:
            raise PipelineError("BACKLOG.md fails lint:\n  " + "\n  ".join(problems[:20]))
        self.group = self.backlog.group(needle) if needle else self.backlog.next_group()
        if self.group is None:
            raise PipelineError("no task group with open agent work" + (" matching %r" % needle if needle else ""))
        open_tasks = [task for task in self.group.open_tasks if task.owner == "agent"]
        if not open_tasks:
            raise PipelineError("%s has no open agent tasks" % self.group.id)
        try:
            self.waves = dag.waves(open_tasks) if self.group.mode == "parallel" else [[task] for task in dag.topological(open_tasks)]
        except dag.CycleError as error:
            raise PipelineError("dependency cycle in %s: %s" % (self.group.id, error))
        base = str(self.config.get("base") or "") or self.git.default_base()
        branch = "%s/%s" % (self.group.type, self.group.slug)
        existing = self.store.latest_for_group(self.group.id)
        if resume and existing:
            self.state = existing
            self.log("resuming run %s on %s" % (existing.run_id, existing.branch))
        else:
            if existing and not self.dry_run:
                raise PipelineError("run %s for %s is unfinished; pass --resume or delete %s" % (existing.run_id, self.group.id, self.store.path(existing.run_id)))
            if not self.dry_run and not self.git.is_clean():
                raise PipelineError("working tree is not clean; commit or stash first")
            if not self.dry_run and self.git.branch_exists(branch):
                raise PipelineError("branch %s already exists with no run state; delete it or pass a different group" % branch)
            leftovers = self.git.worktrees() if not self.dry_run else []
            if leftovers:
                raise PipelineError("stale worktrees: %s; run `komodo status --prune`" % ", ".join(leftovers))
            self.state = RunState(run_id=self.store.new_id(self.group.id), group_id=self.group.id, branch=branch, base=base, profile=self.profile, started=time.time())
            for task in open_tasks:
                self.state.task(task.id)
        self.state.mark_phase("preflight")
        if gates.resolve_verify(self.root) is None:
            self.state.notes.append("repo declares no verify gate; only done_when commands prove the work")
        self.log("group %s: %s | %d task(s) in %d wave(s) | branch %s from %s | profile %s" % (
            self.group.id, self.group.title, len(open_tasks), len(self.waves), branch, base, self.profile))
        for index, wave in enumerate(self.waves, 1):
            self.log("  wave %d: %s" % (index, ", ".join("%s [%s]" % (task.id, ",".join(task.dirs)) for task in wave)))
        if not self.dry_run:
            self.store.save(self.state)

    def branch(self) -> None:
        """Creates the run branch, or checks out the existing one on resume."""
        assert self.state is not None
        self.state.mark_phase("branch")
        if self.dry_run:
            return
        if self.git.branch_exists(self.state.branch):
            self.git.switch(self.state.branch)
        else:
            self.git.create_branch(self.state.branch, self.state.base)
        self.store.save(self.state)

    def build(self) -> None:
        """Runs every wave: one builder per task in its own worktree, merged back in order, compile gate between waves."""
        assert self.state is not None and self.group is not None
        self.state.mark_phase("build")
        if self.group.mode == "single":
            self._build_single()
            return
        max_parallel = max(1, int(self.config.get("max_parallel", 3)))
        for index, wave in enumerate(self.waves, 1):
            pending = [task for task in wave if self.state.task(task.id).status not in ("DONE", "BLOCKED")]
            if not pending:
                continue
            if self.over_budget():
                self.state.notes.append("group budget exhausted before wave %d; %d task(s) left unstarted" % (index, len(pending)))
                break
            self.log("wave %d: %s" % (index, ", ".join(task.id for task in pending)))
            if self.dry_run:
                for task in pending:
                    slots = self.task_slots(task, self.root)
                    system, prompt = briefs.render("builder", slots)
                    self.call("builder", task.id, system, prompt, self.root)
                continue
            worktrees = {task.id: self._worktree_for(task) for task in pending}
            with concurrent.futures.ThreadPoolExecutor(max_workers=min(max_parallel, len(pending))) as pool:
                futures = {pool.submit(self._build_task, task, worktrees[task.id]): task for task in pending}
                for future in concurrent.futures.as_completed(futures):
                    task = futures[future]
                    try:
                        future.result()
                    except Exception as error:  # a crashed task blocks itself, never the run
                        self._block(task, "builder crashed: %s" % error)
            for task in pending:
                self._merge_task(task, worktrees[task.id])
            self._compile_gate(index, pending)
            self.store.save(self.state)

    def _worktree_for(self, task: Task) -> str:
        """Creates the worktree and branch one task builds in."""
        assert self.state is not None
        path = os.path.join(self.root, WORKTREE_DIR, task.id.lower())
        branch = "%s-%s" % (self.state.branch, task.id.lower().replace(".", "-"))
        if os.path.isdir(path):
            self.git.worktree_remove(path, branch)
        self.git.worktree_add(path, branch, self.state.branch)
        return path

    def _build_task(self, task: Task, cwd: str) -> None:
        """One builder pass plus at most one repair, with done_when re-run by the orchestrator."""
        assert self.state is not None
        record = self.state.task(task.id)
        record.status = "IN_PROGRESS"
        failure = ""
        for attempt in (1, 2):
            record.attempts = attempt
            system, prompt = briefs.render("builder", self.task_slots(task, cwd, failure))
            result = self.call("builder", task.id, system, prompt, cwd)
            if not result.ok:
                failure = result.error or "worker failed"
                continue
            data = result.data or {}
            if data.get("result") == "BLOCKED" and attempt == 2:
                failure = str(data.get("summary", "worker reported BLOCKED"))
                break
            gate = gates.run_gate("done_when", task.done_when, cwd, int(self.config.get("worker_timeout_s", 900)))
            if gate.ok:
                changed = [str(item.get("path", "")) for item in data.get("changed", []) if isinstance(item, dict)]
                record.changed = changed
                bullets = [str(item.get("what", "")) for item in data.get("changed", []) if isinstance(item, dict)][:8]
                message = gitops.commit_message(task.type, task.title, bullets)
                record.commit = self.git.commit(message, cwd=cwd) or ""
                record.status = "DONE"
                record.note = str(data.get("summary", ""))[:300]
                record.comment_lines = _comment_lines_added(self.git, cwd, self.state.branch)
                self.log("  %s done in %d attempt(s)" % (task.id, attempt))
                return
            failed = gate.failures()[0]
            failure = "$ %s\n%s" % (failed.command, failed.output)
        self._block(task, failure or "done_when never passed")

    def _block(self, task: Task, note: str) -> None:
        """Marks a task blocked, and its dependents with it."""
        assert self.state is not None and self.group is not None
        record = self.state.task(task.id)
        record.status = "BLOCKED"
        record.note = note[:600]
        self.state.blocked.append(task.id)
        for dependent in dag.blocked_by(self.group.tasks, task.id):
            dep_record = self.state.task(dependent)
            if dep_record.status not in ("DONE",):
                dep_record.status = "BLOCKED"
                dep_record.note = "depends on blocked %s" % task.id
                if dependent not in self.state.blocked:
                    self.state.blocked.append(dependent)
        self.log("  %s BLOCKED: %s" % (task.id, note.splitlines()[0][:120] if note else ""))

    def _merge_task(self, task: Task, cwd: str) -> None:
        """Merges a finished task's worktree branch into the run branch and removes the worktree."""
        assert self.state is not None
        record = self.state.task(task.id)
        branch = "%s-%s" % (self.state.branch, task.id.lower().replace(".", "-"))
        if record.status == "DONE" and record.commit:
            if not self.git.merge(branch):
                conflicted = self.git.conflicted_files()
                self.git.merge_abort()
                self._block(task, "merge conflict with the run branch in %s" % ", ".join(conflicted))
                record.status = "BLOCKED"
        self.git.worktree_remove(cwd, branch)

    def _compile_gate(self, wave_index: int, wave: Sequence[Task]) -> None:
        """A no-model compile or typecheck after a wave, with one repair pass on failure."""
        assert self.state is not None
        commands = gates.compile_commands(self.root)
        if not commands:
            return
        gate = gates.run_gate("compile", commands, self.root, int(self.config.get("worker_timeout_s", 900)))
        if gate.ok:
            return
        failed = gate.failures()[0]
        self.log("  wave %d compile gate failed; running one repair" % wave_index)
        files = sorted({path for task in wave for path in task.files})
        if self._repair(files, "$ %s\n%s" % (failed.command, failed.output), "compile-wave-%d" % wave_index, commands):
            return
        self.state.notes.append("wave %d left the tree failing `%s`; later waves skipped" % (wave_index, failed.command))
        for task in [t for later in self.waves[wave_index:] for t in later]:
            if self.state.task(task.id).status not in ("DONE", "BLOCKED"):
                self._block(task, "skipped: earlier wave broke the build")

    def _repair(self, files: List[str], failure: str, label: str, done_when: List[str]) -> bool:
        """One builder pass in a fresh worktree against a synthetic repair task; returns whether the gate now passes."""
        assert self.state is not None
        synthetic = Task(id=label.upper(), title="Repair the failing check", priority="H", status="TODO",
                         fields={"files": files, "done_when": done_when, "type": "fix"})
        cwd = self._worktree_for(synthetic)
        branch = "%s-%s" % (self.state.branch, synthetic.id.lower().replace(".", "-"))
        try:
            system, prompt = briefs.render("builder", self.task_slots(synthetic, cwd, failure))
            result = self.call("builder", synthetic.id, system, prompt, cwd)
            if not result.ok:
                return False
            gate = gates.run_gate("repair", done_when, cwd, int(self.config.get("worker_timeout_s", 900)))
            if not gate.ok:
                return False
            summary = str((result.data or {}).get("summary", "repair"))[:60]
            if self.git.commit(gitops.commit_message("fix", summary or "repair failing check"), cwd=cwd):
                if not self.git.merge(branch):
                    self.git.merge_abort()
                    return False
            return True
        finally:
            self.git.worktree_remove(cwd, branch)

    def _build_single(self) -> None:
        """mode: single. One builder takes the whole group in one worktree and one commit."""
        assert self.state is not None and self.group is not None
        open_tasks = [task for task in self.group.open_tasks if task.owner == "agent"]
        files = sorted({path for task in open_tasks for path in task.files})
        done_when = []
        for task in open_tasks:
            for command in task.done_when:
                if command not in done_when:
                    done_when.append(command)
        combined = Task(id=self.group.id, title=self.group.title, priority="H", status="TODO",
                        fields={"files": files, "done_when": done_when, "type": self.group.type,
                                "context": sorted({ref for task in open_tasks for ref in task.context})})
        if self.backlog:
            block_lines = []
            for task in open_tasks:
                block_lines.append("# %s: %s" % (task.id, task.title))
                block_lines.extend(self.backlog.lines[task.block_start + 1: task.block_end])
            combined_block = "\n".join(block_lines)
        else:
            combined_block = ""
        if self.dry_run:
            slots = self.task_slots(combined, self.root)
            slots["task_block"] = combined_block
            system, prompt = briefs.render("builder", slots)
            self.call("builder", self.group.id, system, prompt, self.root)
            return
        cwd = self._worktree_for(combined)
        branch = "%s-%s" % (self.state.branch, combined.id.lower().replace(".", "-"))
        failure = ""
        for attempt in (1, 2):
            slots = self.task_slots(combined, cwd, failure)
            slots["task_block"] = combined_block
            system, prompt = briefs.render("builder", slots)
            result = self.call("builder", self.group.id, system, prompt, cwd)
            if not result.ok:
                failure = result.error
                continue
            gate = gates.run_gate("done_when", done_when, cwd, int(self.config.get("worker_timeout_s", 900)))
            if gate.ok:
                data = result.data or {}
                bullets = [str(item.get("what", "")) for item in data.get("changed", []) if isinstance(item, dict)][:8]
                commit = self.git.commit(gitops.commit_message(self.group.type, self.group.title, bullets), cwd=cwd) or ""
                for task in open_tasks:
                    record = self.state.task(task.id)
                    record.status, record.commit, record.attempts = "DONE", commit, attempt
                break
            failed = gate.failures()[0]
            failure = "$ %s\n%s" % (failed.command, failed.output)
        else:
            for task in open_tasks:
                self._block(task, failure or "done_when never passed")
        if any(self.state.task(task.id).status == "DONE" for task in open_tasks):
            if not self.git.merge(branch):
                self.git.merge_abort()
                for task in open_tasks:
                    self._block(task, "merge conflict merging the single-mode worktree")
        self.git.worktree_remove(cwd, branch)
        self.store.save(self.state)

    def verify(self) -> bool:
        """The repo gate once on the merged branch, with one repair pass; returns whether it passed."""
        assert self.state is not None
        self.state.mark_phase("verify")
        command = gates.resolve_verify(self.root)
        if command is None or self.dry_run:
            if self.dry_run:
                self.log("  dry-run verify: %s" % (command or "no gate"))
            return True
        if not any(record.status == "DONE" for record in self.state.tasks.values()):
            return True
        result = gates.run_command(command, self.root, int(self.config.get("worker_timeout_s", 900)))
        if result.ok:
            self.log("  verify passed (%ss)" % result.seconds)
            return True
        self.log("  verify failed; running one repair")
        files = self.git.changed_files(self.state.base)
        if self._repair(files, "$ %s\n%s" % (result.command, result.output), "verify-repair", [command]):
            self.log("  verify passed after repair")
            return True
        self.state.notes.append("verify gate `%s` still failing after one repair; no PR opened" % command)
        return False

    def review(self) -> None:
        """One reviewer pass over the group diff; floor findings get a repair, the rest are filed to the backlog."""
        assert self.state is not None and self.group is not None
        self.state.mark_phase("review")
        spec = self.spec("reviewer")
        lines = 0 if self.dry_run else self.git.diff_lines(self.state.base)
        if not self.dry_run and lines < int(spec.get("min_diff_lines", 0) or 0):
            self.state.notes.append("review skipped: diff is %d lines, under the %s-line floor for profile %s" % (lines, spec.get("min_diff_lines"), self.profile))
            return
        diff = "" if self.dry_run else briefs.clip(self.git.diff(self.state.base), int(self.config.get("context.diff_chars", 80000)), "diff")
        changed = [] if self.dry_run else self.git.changed_files(self.state.base)
        slots = {
            "group_id": self.group.id, "title": self.group.title,
            "tasks": "\n".join("- %s: %s (done_when: %s)" % (task.id, task.title, "; ".join(task.done_when)) for task in self.group.tasks if self.state.task(task.id).status == "DONE") or "- none landed",
            "standards": standards.load(standards.names_for(changed, "reviewer"), int(self.config.get("context.standards_chars", 6000))),
            "base": self.state.base, "diff": diff or "[dry run: diff omitted]",
        }
        system, prompt = briefs.render("reviewer", slots)
        result = self.call("reviewer", self.group.id, system, prompt, self.root)
        if self.dry_run or not result.ok:
            if not self.dry_run:
                self.state.notes.append("review worker failed: %s" % result.error[:200])
            return
        findings = [item for item in (result.data or {}).get("findings", []) if isinstance(item, dict)]
        self.state.blast_radius = str((result.data or {}).get("blast_radius", ""))
        self.state.blast_radius_why = str((result.data or {}).get("blast_radius_why", ""))[:300]
        floor = SEVERITY_RANK.get(str(self.config.get("severity_floor", "high")), 2)
        to_fix = [f for f in findings if SEVERITY_RANK.get(str(f.get("severity")), 0) >= floor]
        to_file = [f for f in findings if f not in to_fix]
        for finding in findings:
            self.state.findings.append({"title": "%s: %s" % (finding.get("severity"), finding.get("title")), "file": finding.get("file"), "severity": finding.get("severity"), "class": finding.get("class"), "fixed": False, "filed": False})
        self.log("  review: %d finding(s), %d at or above %s; blast radius %s" % (len(findings), len(to_fix), self.config.get("severity_floor"), self.state.blast_radius or "unscored"))
        if to_fix:
            failure = "Review findings to fix:\n" + "\n".join("- [%s/%s] %s:%s %s. %s Fix: %s" % (f.get("severity"), f.get("class"), f.get("file"), f.get("line", "?"), f.get("title"), f.get("detail"), f.get("fix", "")) for f in to_fix)
            files = sorted({str(f.get("file")) for f in to_fix if f.get("file")}) or changed
            command = gates.resolve_verify(self.root)
            if self._repair(files, failure, "review-repair", [command] if command else []):
                for entry in self.state.findings:
                    if SEVERITY_RANK.get(str(entry.get("severity")), 0) >= floor:
                        entry["fixed"] = True
            else:
                self.state.notes.append("%d review finding(s) at or above the floor could not be repaired automatically" % len(to_fix))
                to_file = findings
        if to_file and self.backlog_path:
            text = _read_text(self.backlog_path)
            command = gates.resolve_verify(self.root)
            for finding in to_file:
                fields = {
                    "files": [str(finding.get("file"))] if finding.get("file") else [],
                    "done_when": [command] if command else [],
                    "type": "fix",
                    "context": ["review finding from %s: %s" % (self.state.run_id, str(finding.get("detail", ""))[:160])],
                }
                priority = {"critical": "C", "high": "H", "medium": "M", "low": "L"}.get(str(finding.get("severity")), "M")
                text, _ = tasks.append_task(text, self.group.id, "%s: %s" % (finding.get("class", "review"), finding.get("title", "finding")), fields, priority)
            with open(self.backlog_path, "w", encoding="utf-8") as handle:
                handle.write(text)
            for entry in self.state.findings:
                if not entry.get("fixed"):
                    entry["filed"] = True
        self.store.save(self.state)

    def publish(self, verified: bool) -> None:
        """Updates backlog statuses and the changelog, commits, pushes, and opens the PR when the branch verified."""
        assert self.state is not None and self.group is not None and self.backlog_path is not None
        self.state.mark_phase("publish")
        if self.dry_run:
            self.log("  dry-run publish: would push %s and open a PR against %s" % (self.state.branch, self.state.base))
            return
        text = _read_text(self.backlog_path)
        for task_id, record in self.state.tasks.items():
            if record.status in ("DONE", "BLOCKED") and self.backlog and self.backlog.task(task_id):
                text = tasks.set_status(text, task_id, record.status)
        with open(self.backlog_path, "w", encoding="utf-8") as handle:
            handle.write(text)
        done_titles = [self.backlog.task(tid).title for tid, record in self.state.tasks.items() if record.status == "DONE" and self.backlog and self.backlog.task(tid)]
        if done_titles:
            _write_changelog(os.path.join(self.root, str(self.config.get("changelog", "CHANGELOG.md"))), self.group.type, done_titles)
        self.git.commit(gitops.commit_message("chore", "close out %s" % self.group.id, ["backlog statuses", "changelog entry"]))
        if not verified:
            self.state.notes.append("branch %s left local and unpushed because verify failed" % self.state.branch)
            return
        if not done_titles:
            self.state.notes.append("nothing landed; branch not pushed")
            return
        if not self.git.has_remote():
            self.state.notes.append("no remote %r configured; branch %s left local" % (self.config.remote, self.state.branch))
            return
        self.git.push(self.state.branch)
        if not pr.available(self.root):
            self.state.notes.append("gh is not authenticated; pushed %s without opening a PR" % self.state.branch)
            return
        titles = {task.id: task.title for task in self.group.tasks}
        template = None
        for candidate in (".github/PULL_REQUEST_TEMPLATE.md", ".github/pull_request_template.md"):
            path = os.path.join(self.root, candidate)
            if os.path.isfile(path):
                template = _read_text(path)
        body = render.pr_body(self.state, self.group.title, titles, self.git.log_subjects(self.state.base), template)
        labels = pr.pick_labels(self.group.type, dict(self.config.get("labels", {})), pr.existing_labels(self.root))
        title = "%s: %s" % (self.group.type, self.group.title)
        existing = pr.view(self.root)
        if existing and existing.get("state") == "OPEN":
            pr.edit(self.root, int(existing["number"]), body=body, add_labels=labels)
            self.state.pr_url = str(existing.get("url", ""))
        else:
            self.state.pr_url = pr.create(self.root, title[:72], body, self.state.base, labels)
        self.log("  PR: %s" % self.state.pr_url)

    def finish(self) -> str:
        """Writes the report and returns its path."""
        assert self.state is not None and self.group is not None
        self.state.finished = time.time()
        titles = {task.id: task.title for task in self.group.tasks}
        commits = [] if self.dry_run else self.git.log_subjects(self.state.base)
        text = render.report(self.state, self.group.title, titles, commits)
        path = self.store.report_path(self.state.run_id)
        if not self.dry_run:
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w", encoding="utf-8") as handle:
                handle.write(text)
            self.store.save(self.state)
        self.log(text)
        return path

    def run(self, needle: Optional[str] = None, resume: bool = False) -> RunState:
        """The whole line, in order."""
        self.preflight(needle, resume)
        self.branch()
        self.build()
        verified = self.verify()
        if verified:
            self.review()
            verified = self.verify() if any(f.get("fixed") for f in (self.state.findings if self.state else [])) else verified
        self.publish(verified)
        self.finish()
        assert self.state is not None
        return self.state


def _section(text: str, anchor: str) -> str:
    """The markdown section whose heading slug matches anchor, or empty."""
    slug = re.sub(r"[^a-z0-9]+", "-", anchor.lower()).strip("-")
    lines = text.splitlines()
    for index, line in enumerate(lines):
        if line.startswith("#") and re.sub(r"[^a-z0-9]+", "-", line.lstrip("#").strip().lower()).strip("-") == slug:
            level = len(line) - len(line.lstrip("#"))
            end = index + 1
            while end < len(lines) and not (lines[end].startswith("#") and len(lines[end]) - len(lines[end].lstrip("#")) <= level):
                end += 1
            return "\n".join(lines[index:end])
    return ""


def _comment_lines_added(git: gitops.Git, cwd: str, base: str) -> int:
    """Added lines that are whole-line comments, from the worktree's diff against the run branch."""
    try:
        diff = git.run("diff", base, "--unified=0", cwd=cwd)
    except gitops.GitError:
        return 0
    count = 0
    for line in diff.splitlines():
        if line.startswith("+") and not line.startswith("+++"):
            body = line[1:].strip()
            if body.startswith(("//", "#", "--", "/*", "*", '"""')) and not body.startswith("#!"):
                count += 1
    return count


def _write_changelog(path: str, kind: str, titles: Sequence[str]) -> None:
    """Appends bullets under [Unreleased] in the Keep a Changelog shape, creating the file if needed."""
    heading = render.changelog_heading(kind)
    bullets = render.changelog_entry(kind, titles)
    if not os.path.isfile(path):
        text = "# Changelog\n\n## [Unreleased]\n\n### %s\n%s\n" % (heading, "\n".join(bullets))
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(text)
        return
    text = _read_text(path)
    lines = text.splitlines()
    try:
        start = next(index for index, line in enumerate(lines) if line.lower().startswith("## [unreleased]"))
    except StopIteration:
        insert_at = next((index for index, line in enumerate(lines) if line.startswith("## ")), len(lines))
        lines[insert_at:insert_at] = ["## [Unreleased]", "", "### %s" % heading] + bullets + [""]
        _write(path, lines)
        return
    end = next((index for index in range(start + 1, len(lines)) if lines[index].startswith("## ")), len(lines))
    section = lines[start:end]
    for index, line in enumerate(section):
        if line.strip().lower() == ("### %s" % heading).lower():
            insert_at = index + 1
            while insert_at < len(section) and section[insert_at].startswith("- "):
                insert_at += 1
            section[insert_at:insert_at] = bullets
            break
    else:
        while section and not section[-1].strip():
            section.pop()
        section += ["", "### %s" % heading] + bullets
    lines[start:end] = section + ([""] if end < len(lines) and lines[end].startswith("## ") and section[-1].strip() else [])
    _write(path, lines)


def _read_text(path: str) -> str:
    """A file's text, closed on return."""
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def _write(path: str, lines: List[str]) -> None:
    """Writes lines back with a trailing newline."""
    with open(path, "w", encoding="utf-8") as handle:
        handle.write("\n".join(lines).rstrip("\n") + "\n")

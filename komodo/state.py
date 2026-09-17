"""Run state on disk under .komodo/runs/<id>/, so a run can resume and report."""

from __future__ import annotations

import json
import os
import threading
import time
from dataclasses import asdict, dataclass, field
from typing import Any, Dict, List, Optional

RUNS_DIR = os.path.join(".komodo", "runs")


@dataclass
class WorkerRecord:
    """What one worker invocation cost and returned."""

    role: str
    task_id: str
    provider: str
    model: str
    ok: bool
    seconds: float
    cost_usd: float = 0.0
    input_tokens: int = 0
    output_tokens: int = 0
    turns: int = 0
    error: str = ""


@dataclass
class TaskRecord:
    """Where one task stands inside the run."""

    id: str
    status: str = "TODO"
    commit: str = ""
    attempts: int = 0
    note: str = ""
    changed: List[str] = field(default_factory=list)
    comment_lines: int = 0


@dataclass
class RunState:
    """Everything a resumed run needs, persisted after every phase."""

    run_id: str
    group_id: str
    branch: str
    base: str
    profile: str
    started: float
    phase: str = "preflight"
    tasks: Dict[str, TaskRecord] = field(default_factory=dict)
    workers: List[WorkerRecord] = field(default_factory=list)
    phases: Dict[str, float] = field(default_factory=dict)
    findings: List[Dict[str, Any]] = field(default_factory=list)
    blast_radius: str = ""
    blast_radius_why: str = ""
    pr_url: str = ""
    blocked: List[str] = field(default_factory=list)
    notes: List[str] = field(default_factory=list)
    finished: float = 0.0

    @property
    def elapsed(self) -> float:
        """Seconds since the run started, or its total once finished."""
        end = self.finished or time.time()
        return round(end - self.started, 1)

    @property
    def cost_usd(self) -> float:
        """Total worker spend."""
        return round(sum(record.cost_usd for record in self.workers), 4)

    def task(self, task_id: str) -> TaskRecord:
        """The record for a task, created on first access."""
        if task_id not in self.tasks:
            self.tasks[task_id] = TaskRecord(id=task_id)
        return self.tasks[task_id]

    def mark_phase(self, name: str) -> None:
        """Records the wall-clock at which a phase began."""
        self.phase = name
        self.phases[name] = time.time()

    def to_json(self) -> Dict[str, Any]:
        """A plain dict for JSON."""
        data = asdict(self)
        return data

    @classmethod
    def from_json(cls, data: Dict[str, Any]) -> "RunState":
        """Rebuilds a state from its JSON form."""
        tasks = {key: TaskRecord(**value) for key, value in data.get("tasks", {}).items()}
        workers = [WorkerRecord(**value) for value in data.get("workers", [])]
        rest = {key: value for key, value in data.items() if key not in ("tasks", "workers")}
        state = cls(**rest)
        state.tasks = tasks
        state.workers = workers
        return state


class Store:
    """Reads and writes run states under a repo's .komodo/runs/."""

    def __init__(self, root: str):
        self.root = root
        self.dir = os.path.join(root, RUNS_DIR)
        self._lock = threading.Lock()

    def path(self, run_id: str) -> str:
        """The state file for a run."""
        return os.path.join(self.dir, run_id, "state.json")

    def report_path(self, run_id: str) -> str:
        """The rendered report for a run."""
        return os.path.join(self.dir, run_id, "report.md")

    def save(self, state: RunState) -> None:
        """Writes the state atomically; parallel builders share one store, so the write is serialized."""
        target = self.path(state.run_id)
        with self._lock:
            os.makedirs(os.path.dirname(target), exist_ok=True)
            temp = "%s.%d.tmp" % (target, threading.get_ident())
            with open(temp, "w", encoding="utf-8") as handle:
                json.dump(state.to_json(), handle, indent=2)
            os.replace(temp, target)

    def load(self, run_id: str) -> RunState:
        """Reads a saved state."""
        with open(self.path(run_id), encoding="utf-8") as handle:
            return RunState.from_json(json.load(handle))

    def list(self) -> List[str]:
        """Run ids on disk, newest first."""
        if not os.path.isdir(self.dir):
            return []
        return sorted(os.listdir(self.dir), reverse=True)

    def latest_for_group(self, group_id: str) -> Optional[RunState]:
        """The most recent state for a group that has not finished, or None."""
        for run_id in self.list():
            try:
                state = self.load(run_id)
            except (OSError, ValueError, TypeError):
                continue
            if state.group_id == group_id and not state.finished:
                return state
        return None

    def new_id(self, group_id: str) -> str:
        """A sortable run id from the clock and the group."""
        return time.strftime("%Y%m%d-%H%M%S") + "-" + group_id.lower().replace(".", "-")

"""Dependency ordering and wave scheduling over a group's open tasks."""

from __future__ import annotations

from typing import Dict, List, Sequence

from .tasks import Task


class CycleError(ValueError):
    """Raised when depends_on edges form a loop."""


def topological(tasks: Sequence[Task]) -> List[Task]:
    """Tasks ordered so every dependency precedes its dependents, file order breaking ties."""
    by_id = {task.id: task for task in tasks}
    state: Dict[str, int] = {}
    ordered: List[Task] = []

    def visit(task: Task, trail: List[str]) -> None:
        """Depth-first walk that records a task after its dependencies."""
        mark = state.get(task.id, 0)
        if mark == 2:
            return
        if mark == 1:
            raise CycleError(" -> ".join(trail + [task.id]))
        state[task.id] = 1
        for dep in task.depends_on:
            if dep in by_id:
                visit(by_id[dep], trail + [task.id])
        state[task.id] = 2
        ordered.append(task)

    for task in tasks:
        visit(task, [])
    return ordered


def _dirs_overlap(left: Task, right: Task) -> bool:
    """Whether two tasks claim the same directory, or one claims a parent of the other's."""
    for a in left.dirs:
        for b in right.dirs:
            if a == b or a.startswith(b + "/") or b.startswith(a + "/"):
                return True
    return False


def waves(tasks: Sequence[Task], done: Sequence[str] = ()) -> List[List[Task]]:
    """Groups tasks into waves whose members share no directory and whose dependencies are all satisfied earlier."""
    ordered = [task for task in topological(tasks) if task.id not in done]
    finished = set(done)
    result: List[List[Task]] = []
    pending = list(ordered)
    while pending:
        wave: List[Task] = []
        remaining: List[Task] = []
        for task in pending:
            ready = all(dep in finished or dep not in {t.id for t in tasks} for dep in task.depends_on)
            if ready and not any(_dirs_overlap(task, member) for member in wave):
                wave.append(task)
            else:
                remaining.append(task)
        if not wave:
            raise CycleError("no runnable task among: " + ", ".join(task.id for task in remaining))
        result.append(wave)
        finished.update(task.id for task in wave)
        pending = remaining
    return result


def blocked_by(tasks: Sequence[Task], failed: str) -> List[str]:
    """Ids of every task that transitively depends on the failed one."""
    dependents: Dict[str, List[str]] = {}
    for task in tasks:
        for dep in task.depends_on:
            dependents.setdefault(dep, []).append(task.id)
    out: List[str] = []
    frontier = [failed]
    while frontier:
        current = frontier.pop()
        for child in dependents.get(current, []):
            if child not in out:
                out.append(child)
                frontier.append(child)
    return out

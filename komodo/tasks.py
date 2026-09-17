"""BACKLOG.md grammar: parse, lint, rewrite status, append tasks, migrate the old table shape."""

from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Tuple

from . import yamlite

STATUSES = ("TODO", "IN_PROGRESS", "BLOCKED", "DONE")
PRIORITIES = ("C", "H", "M", "L")
TYPES = ("feat", "fix", "chore", "docs", "test", "refactor", "perf", "build", "ci")
OWNERS = ("agent", "human")
MODES = ("parallel", "single")

EPIC_HEADING = re.compile(r"^##\s+\[(EPIC-[\w.]+)\]\s*(.*?)\s*$")
GROUP_HEADING = re.compile(r"^###\s+\[(TG-[\w.]+)\]\s*(.*?)\s*$")
TASK_HEADING = re.compile(
    r"^####\s+\[(TSK-[\w.]+)\]\s+(.+?)\s*\[P:\s*([A-Z])\]\s*\[([A-Z_]+)\]\s*$"
)
FENCE_OPEN = re.compile(r"^```(?:yaml|yml)\s*$")
FENCE_CLOSE = re.compile(r"^```\s*$")
LEGACY_STATUS = {"WIP": "IN_PROGRESS"}
COMMAND_HINT = re.compile(r"^(go|npm|pnpm|bun|npx|python3?|py\b|pytest|make|task|just|cdk|tsc|cargo|dotnet|mvn|gradle|zig|swift|bash|sh|\./|/|\"|'|[A-Za-z]:[\\/]|test\b|git\b)")


@dataclass
class Task:
    """One task heading plus its fenced block, with line positions for rewrites."""

    id: str
    title: str
    priority: str
    status: str
    fields: Dict[str, object] = field(default_factory=dict)
    heading_line: int = 0
    block_start: int = -1
    block_end: int = -1
    group_id: str = ""

    @property
    def files(self) -> List[str]:
        """The files this task declares it will touch."""
        return [str(item) for item in self.fields.get("files") or []]

    @property
    def dirs(self) -> List[str]:
        """Ownership units: the directory of every declared file, deduplicated."""
        seen: List[str] = []
        for path in self.files:
            directory = os.path.dirname(path.replace("\\", "/")) or "."
            if directory not in seen:
                seen.append(directory)
        return seen

    @property
    def done_when(self) -> List[str]:
        """The shell commands whose zero exit proves the task done."""
        return [str(item) for item in self.fields.get("done_when") or []]

    @property
    def depends_on(self) -> List[str]:
        """Task ids that must be DONE before this one may start."""
        return [str(item) for item in self.fields.get("depends_on") or []]

    @property
    def context(self) -> List[str]:
        """Paths, with optional #anchor, a worker reads before starting."""
        return [str(item) for item in self.fields.get("context") or []]

    @property
    def owner(self) -> str:
        """Who executes this task: agent by default, human when a person must act."""
        return str(self.fields.get("owner") or "agent")

    @property
    def type(self) -> str:
        """The conventional-commit type, defaulting to feat."""
        return str(self.fields.get("type") or "feat")

    @property
    def open(self) -> bool:
        """Whether the task still has work to run."""
        return self.status not in ("DONE", "BLOCKED")


@dataclass
class Group:
    """A task group heading, its optional block, and its tasks in file order."""

    id: str
    title: str
    fields: Dict[str, object] = field(default_factory=dict)
    tasks: List[Task] = field(default_factory=list)
    heading_line: int = 0
    epic_id: str = ""

    @property
    def mode(self) -> str:
        """parallel by default; single runs one builder over the whole group."""
        return str(self.fields.get("mode") or "parallel")

    @property
    def type(self) -> str:
        """The branch and commit type for the group, defaulting to feat."""
        return str(self.fields.get("type") or "feat")

    @property
    def slug(self) -> str:
        """A kebab-case branch fragment derived from the group title."""
        text = re.sub(r"[^a-z0-9]+", "-", self.title.lower()).strip("-")
        return text[:40].rstrip("-") or self.id.lower()

    @property
    def open_tasks(self) -> List[Task]:
        """Tasks that are neither DONE nor BLOCKED."""
        return [task for task in self.tasks if task.open]


@dataclass
class Backlog:
    """The whole parsed file: groups in order, plus every problem the parser saw."""

    groups: List[Group] = field(default_factory=list)
    problems: List[str] = field(default_factory=list)
    lines: List[str] = field(default_factory=list)

    @property
    def tasks(self) -> List[Task]:
        """Every task across every group, in file order."""
        return [task for group in self.groups for task in group.tasks]

    def task(self, task_id: str) -> Optional[Task]:
        """The task with this exact id, or None."""
        for task in self.tasks:
            if task.id == task_id:
                return task
        return None

    def group(self, needle: str) -> Optional[Group]:
        """The group whose id matches exactly, else the first whose id or title contains the needle."""
        for group in self.groups:
            if group.id == needle:
                return group
        lowered = needle.lower()
        for group in self.groups:
            if lowered in group.id.lower() or lowered in group.title.lower():
                return group
        return None

    def next_group(self) -> Optional[Group]:
        """The first group in file order holding at least one open agent task."""
        for group in self.groups:
            if any(task.open and task.owner == "agent" for task in group.tasks):
                return group
        return None


def _read_block(lines: List[str], start: int) -> Tuple[Optional[Dict[str, object]], int, int, Optional[str]]:
    """Reads the fenced yaml block that directly follows a heading; returns fields, block bounds, error."""
    index = start
    while index < len(lines) and not lines[index].strip():
        index += 1
    if index >= len(lines) or not FENCE_OPEN.match(lines[index]):
        return None, -1, -1, None
    open_line = index
    index += 1
    body = []
    while index < len(lines) and not FENCE_CLOSE.match(lines[index]):
        body.append(lines[index])
        index += 1
    if index >= len(lines):
        return None, open_line, -1, "unterminated yaml block opened at line %d" % (open_line + 1)
    try:
        return yamlite.loads("\n".join(body)), open_line, index, None
    except yamlite.YamliteError as error:
        return None, open_line, index, "line %d: %s" % (open_line + 1, error)


def parse(text: str) -> Backlog:
    """Parses BACKLOG.md into groups and tasks without judging their content."""
    lines = text.splitlines()
    backlog = Backlog(lines=lines)
    epic_id = ""
    group: Optional[Group] = None
    index = 0
    while index < len(lines):
        line = lines[index]
        epic = EPIC_HEADING.match(line)
        if epic:
            epic_id = epic.group(1)
            index += 1
            continue
        heading = GROUP_HEADING.match(line)
        if heading:
            group = Group(id=heading.group(1), title=heading.group(2), heading_line=index, epic_id=epic_id)
            fields, _, end, error = _read_block(lines, index + 1)
            if error:
                backlog.problems.append(error)
            if fields is not None:
                group.fields = fields
                index = end + 1
            else:
                index += 1
            backlog.groups.append(group)
            continue
        task_match = TASK_HEADING.match(line)
        if task_match:
            status = task_match.group(4).upper()
            status = LEGACY_STATUS.get(status, status)
            task = Task(
                id=task_match.group(1),
                title=task_match.group(2).strip(),
                priority=task_match.group(3),
                status=status,
                heading_line=index,
                group_id=group.id if group else "",
            )
            fields, start, end, error = _read_block(lines, index + 1)
            if error:
                backlog.problems.append(error)
            if fields is not None:
                task.fields = fields
                task.block_start, task.block_end = start, end
                index = end + 1
            else:
                index += 1
            if group is None:
                backlog.problems.append("line %d: task %s appears before any ### [TG-] heading" % (task.heading_line + 1, task.id))
                group = Group(id="TG-ORPHAN", title="Orphaned tasks", heading_line=task.heading_line)
                backlog.groups.append(group)
            task.group_id = group.id
            group.tasks.append(task)
            continue
        index += 1
    return backlog


def lint(backlog: Backlog) -> List[str]:
    """Every problem that would stop the orchestrator from running this backlog deterministically."""
    problems = list(backlog.problems)
    seen: Dict[str, int] = {}
    ids = {task.id for task in backlog.tasks}
    for group in backlog.groups:
        if group.mode not in MODES:
            problems.append("%s: mode must be one of %s" % (group.id, "|".join(MODES)))
        if group.type not in TYPES:
            problems.append("%s: type must be one of %s" % (group.id, "|".join(TYPES)))
        if group.id in seen:
            problems.append("%s: duplicate group id (lines %d and %d)" % (group.id, seen[group.id] + 1, group.heading_line + 1))
        seen[group.id] = group.heading_line
    for task in backlog.tasks:
        where = "%s (line %d)" % (task.id, task.heading_line + 1)
        if task.id in seen:
            problems.append("%s: duplicate task id" % where)
        seen[task.id] = task.heading_line
        if task.status not in STATUSES:
            problems.append("%s: status must be one of %s" % (where, "|".join(STATUSES)))
        if task.priority not in PRIORITIES:
            problems.append("%s: priority must be one of %s" % (where, "|".join(PRIORITIES)))
        if task.owner not in OWNERS:
            problems.append("%s: owner must be agent or human" % where)
        if task.type not in TYPES:
            problems.append("%s: type must be one of %s" % (where, "|".join(TYPES)))
        for dep in task.depends_on:
            if dep not in ids:
                problems.append("%s: depends_on names unknown task %s" % (where, dep))
        if task.owner != "agent" or not task.open:
            continue
        if task.block_start < 0:
            problems.append("%s: agent task has no yaml block" % where)
            continue
        if not task.files:
            problems.append("%s: agent task declares no files" % where)
        if not task.done_when:
            problems.append("%s: agent task declares no done_when commands" % where)
        for command in task.done_when:
            if not COMMAND_HINT.match(command.strip()):
                problems.append("%s: done_when entry does not look like a command: %r" % (where, command))
    return problems


def set_status(text: str, task_id: str, status: str) -> str:
    """Rewrites one task heading's status token, leaving every other byte alone."""
    if status not in STATUSES:
        raise ValueError("unknown status %r" % status)
    lines = text.splitlines(keepends=True)
    for index, line in enumerate(lines):
        match = TASK_HEADING.match(line.rstrip("\r\n"))
        if match and match.group(1) == task_id:
            newline = line[len(line.rstrip("\r\n")):]
            lines[index] = re.sub(r"\[[A-Z_]+\]\s*$", "[%s]" % status, line.rstrip("\r\n")) + newline
            return "".join(lines)
    raise KeyError("task %s not found" % task_id)


def _block_end(lines: List[str], start: int) -> int:
    """The index after a task's heading, its fenced block, and the blank lines trailing it."""
    index = start + 1
    if index < len(lines) and FENCE_OPEN.match(lines[index].rstrip("\r\n")):
        index += 1
        while index < len(lines) and not FENCE_CLOSE.match(lines[index].rstrip("\r\n")):
            index += 1
        index += 1
    while index < len(lines) and not lines[index].strip():
        index += 1
    return index


def _drop_dependency(lines: List[str], task_id: str) -> List[str]:
    """Removes a task id from every depends_on list, inline or dashed."""
    kept = []
    for line in lines:
        bare = line.strip()
        if bare in ("- %s" % task_id, "- \"%s\"" % task_id, "- '%s'" % task_id):
            continue
        if "depends_on:" in line and task_id in line:
            head, _, rest = line.partition("depends_on:")
            items = [item.strip() for item in rest.strip().strip("[]").split(",")]
            items = [item for item in items if item.strip("\"'") != task_id and item]
            line = "%sdepends_on: [%s]\n" % (head, ", ".join(items)) if items else ""
            if not line:
                continue
        kept.append(line)
    return kept


def remove_task(text: str, task_id: str) -> str:
    """Deletes a task, its fenced block, any group left empty, and every depends_on naming it."""
    lines = text.splitlines(keepends=True)
    for index, line in enumerate(lines):
        match = TASK_HEADING.match(line.rstrip("\r\n"))
        if not match or match.group(1) != task_id:
            continue
        end = _block_end(lines, index)
        start = index
        if not _group_keeps_tasks(lines, index, end):
            start = _group_start(lines, index)
        remaining = _drop_dependency(lines[:start] + lines[end:], task_id)
        return "".join(remaining)
    raise KeyError("task %s not found" % task_id)


def _group_start(lines: List[str], task_index: int) -> int:
    """The index of the group heading above a task, including its fenced block."""
    for index in range(task_index - 1, -1, -1):
        if GROUP_HEADING.match(lines[index].rstrip("\r\n")):
            return index
    return task_index


def _group_keeps_tasks(lines: List[str], task_index: int, end: int) -> bool:
    """Whether the task's group still holds another task once this one goes."""
    start = _group_start(lines, task_index)
    if start == task_index:
        return True
    stop = len(lines)
    for index in range(start + 1, len(lines)):
        stripped = lines[index].rstrip("\r\n")
        if GROUP_HEADING.match(stripped) or EPIC_HEADING.match(stripped):
            stop = index
            break
    for index in range(start + 1, stop):
        if task_index <= index < end:
            continue
        if TASK_HEADING.match(lines[index].rstrip("\r\n")):
            return True
    return False


def render_task(task_id: str, title: str, priority: str, status: str, fields: Dict[str, object]) -> str:
    """Renders a heading plus fenced block in the grammar."""
    heading = "#### [%s] %s [P: %s] [%s]" % (task_id, title.strip(), priority, status)
    return heading + "\n```yaml\n" + yamlite.dumps(fields) + "```\n"


def next_task_id(group: Group) -> str:
    """The next free TSK id under a group, counting from the highest existing suffix."""
    base = group.id[len("TG-"):]
    highest = 0
    for task in group.tasks:
        suffix = task.id.rsplit(".", 1)[-1]
        if suffix.isdigit():
            highest = max(highest, int(suffix))
    return "TSK-%s.%d" % (base, highest + 1)


def append_task(text: str, group_id: str, title: str, fields: Dict[str, object], priority: str = "M", status: str = "TODO") -> Tuple[str, str]:
    """Appends a task at the end of a group and returns the new text and the task id."""
    backlog = parse(text)
    group = backlog.group(group_id)
    if group is None:
        raise KeyError("group %s not found" % group_id)
    task_id = next_task_id(group)
    lines = text.splitlines(keepends=True)
    position = group.heading_line + 1
    for candidate in backlog.groups:
        if candidate.heading_line > group.heading_line:
            position = candidate.heading_line
            break
    else:
        position = len(lines)
    while position > group.heading_line + 1 and not lines[position - 1].strip():
        position -= 1
    insert = render_task(task_id, title, priority, status, fields)
    block = "\n" + insert if position and lines[position - 1].strip() else insert
    if position < len(lines):
        block = block + "\n"
    lines[position:position] = [block]
    return "".join(lines), task_id


LEGACY_TASK = re.compile(r"^####\s+\[(TSK-[\w.]+)\]\s+(.+?)\s*\[P:\s*([A-Z])\]\s*\[([A-Z_]+)\]\s*$")
LEGACY_ROW = re.compile(r"^\|\s*`?(SUB-[\w.]+)`?\s*\|(.*?)\|(.*?)\|\s*$")
LEGACY_FIELD = re.compile(r"^\|\s*([^|]+?)\s*\|\s*(.*?)\s*\|\s*$")


def migrate(text: str) -> Tuple[str, List[str]]:
    """Converts the old table-based task shape into fenced blocks, reporting what needed a human."""
    lines = text.splitlines()
    out: List[str] = []
    report: List[str] = []
    index = 0
    while index < len(lines):
        line = lines[index]
        match = LEGACY_TASK.match(line)
        if not match:
            out.append(line)
            index += 1
            continue
        task_id, title, priority, status = match.groups()
        status = LEGACY_STATUS.get(status.upper(), status.upper())
        peek = index + 1
        while peek < len(lines) and not lines[peek].strip():
            peek += 1
        if peek < len(lines) and FENCE_OPEN.match(lines[peek]):
            out.append(line)
            index += 1
            continue
        index += 1
        fields: Dict[str, object] = {"files": [], "done_when": []}
        prose: List[str] = []
        depends: List[str] = []
        while index < len(lines) and not lines[index].startswith("#"):
            row = lines[index]
            sub = LEGACY_ROW.match(row)
            if sub:
                done = sub.group(3).strip().strip("`")
                if done and done.lower() not in ("done when", "---"):
                    if COMMAND_HINT.match(done):
                        fields["done_when"].append(done)
                    else:
                        prose.append(done)
            else:
                pair = LEGACY_FIELD.match(row)
                if pair and pair.group(1).lower() in ("depends on", "after", "blocked by"):
                    depends.extend(re.findall(r"TSK-[\w.]+", pair.group(2)))
            index += 1
        if depends:
            fields["depends_on"] = depends
        if prose:
            fields["done_when_prose"] = prose
            report.append("%s: %d done-when cell(s) are prose, not commands; rewrite done_when" % (task_id, len(prose)))
        if not fields["done_when"]:
            report.append("%s: no executable done_when found" % task_id)
        report.append("%s: files list is empty; declare the paths this task touches" % task_id)
        out.append("#### [%s] %s [P: %s] [%s]" % (task_id, title.strip(), priority, status))
        out.append("```yaml")
        out.append(yamlite.dumps(fields).rstrip("\n"))
        out.append("```")
        out.append("")
    return "\n".join(out) + ("\n" if text.endswith("\n") else ""), report


def load(path: str) -> Backlog:
    """Parses the backlog file at path."""
    with open(path, encoding="utf-8") as handle:
        return parse(handle.read())


def find_backlog(root: str) -> Optional[str]:
    """The backlog path for a repo, checking the root then docs/."""
    for relative in ("BACKLOG.md", os.path.join("docs", "BACKLOG.md")):
        candidate = os.path.join(root, relative)
        if os.path.isfile(candidate):
            return candidate
    return None

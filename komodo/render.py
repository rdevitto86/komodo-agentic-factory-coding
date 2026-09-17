"""Human-facing text the orchestrator writes: run reports, PR bodies, changelog bullets, under fixed density caps."""

from __future__ import annotations

import re
import time
from typing import Dict, Iterable, List, Optional, Sequence, Tuple

from .state import RunState

MAX_BULLETS = 5
MAX_WORDS = 20


def bullets(items: Iterable[str], cap: int = MAX_BULLETS) -> List[str]:
    """At most cap bullets; the rest collapse into one counted line."""
    listed = [item.strip() for item in items if item and item.strip()]
    if len(listed) <= cap:
        return ["- " + item for item in listed]
    shown = ["- " + item for item in listed[: cap - 1]]
    shown.append("- and %d more" % (len(listed) - cap + 1))
    return shown


def clip_sentence(text: str, words: int = MAX_WORDS) -> str:
    """Cuts a sentence at the word cap with an ellipsis."""
    parts = text.split()
    if len(parts) <= words:
        return text.strip()
    return " ".join(parts[:words]).rstrip(",;:") + "..."


def duration(seconds: float) -> str:
    """Seconds as 'Xm Ys'."""
    total = int(round(seconds))
    if total < 60:
        return "%ds" % total
    return "%dm %02ds" % divmod(total, 60)


def phase_table(state: RunState) -> List[str]:
    """Wall-clock per phase from the recorded start times."""
    names = list(state.phases.keys())
    rows = ["| Phase | Time |", "|---|---|"]
    for index, name in enumerate(names):
        start = state.phases[name]
        end = state.phases[names[index + 1]] if index + 1 < len(names) else (state.finished or time.time())
        rows.append("| %s | %s |" % (name, duration(end - start)))
    return rows[:8]


def worker_table(state: RunState) -> List[str]:
    """Cost and tokens per role, aggregated."""
    totals: Dict[str, Dict[str, float]] = {}
    for record in state.workers:
        row = totals.setdefault(record.role, {"calls": 0, "cost": 0.0, "tokens": 0})
        row["calls"] += 1
        row["cost"] += record.cost_usd
        row["tokens"] += record.input_tokens + record.output_tokens
    rows = ["| Role | Calls | Cost |", "|---|---|---|"]
    for role, row in sorted(totals.items()):
        rows.append("| %s | %d | $%.2f |" % (role, row["calls"], row["cost"]))
    return rows[:8]


def summary_buckets(state: RunState, task_titles: Dict[str, str]) -> List[str]:
    """The fixed three-bucket turn-end summary."""
    done = [task_titles.get(tid, tid) for tid, record in state.tasks.items() if record.status == "DONE"]
    blocked = ["%s: %s" % (task_titles.get(tid, tid), record.note) for tid, record in state.tasks.items() if record.status == "BLOCKED"]
    flagged = [f["title"] for f in state.findings if f.get("filed")] + state.notes
    out: List[str] = []
    if done:
        out += ["## ✅ Successful Changes", ""] + bullets(done) + [""]
    if blocked:
        out += ["## ❌ Blocked Changes", ""] + bullets(blocked) + [""]
    if flagged:
        out += ["## ⚠️ Flagged Changes", ""] + bullets(clip_sentence(item) for item in flagged) + [""]
    return out


def report(state: RunState, group_title: str, task_titles: Dict[str, str], commits: Sequence[str]) -> str:
    """The full run report, written to .komodo/runs/<id>/report.md and used as the PR body."""
    lines = ["# %s: %s" % (state.group_id, group_title), ""]
    status = "blocked" if state.blocked else "complete"
    lines.append("**Run %s** in %s for $%.2f, %d worker calls, profile `%s`." % (status, duration(state.elapsed), state.cost_usd, len(state.workers), state.profile))
    if state.blast_radius:
        lines.append("**Blast radius %s.** %s" % (state.blast_radius, state.blast_radius_why))
    lines.append("")
    lines += summary_buckets(state, task_titles)
    if commits:
        lines += ["## Commits", ""] + bullets(commits) + [""]
    lines += ["## Timing", ""] + phase_table(state) + [""]
    if state.workers:
        lines += ["## Spend", ""] + worker_table(state) + [""]
    return "\n".join(lines).rstrip() + "\n"


HTML_COMMENT = re.compile(r"<!--.*?-->", re.DOTALL)


def pr_sections(state: RunState, group_title: str, task_titles: Dict[str, str]) -> List[tuple]:
    """Each PR body section in order, as a heading and its body lines."""
    done = [task_titles.get(tid, tid) for tid, record in state.tasks.items() if record.status == "DONE"]
    blocked = [task_titles.get(tid, tid) for tid, record in state.tasks.items() if record.status == "BLOCKED"]
    summary = "%s: %s. %d of %d tasks landed." % (state.group_id, group_title, len(done), len(state.tasks))
    validation = ["- Every task's `done_when` re-run by the orchestrator, then the repo verify gate once on the merged branch."]
    if state.findings:
        fixed = sum(1 for f in state.findings if f.get("fixed"))
        filed = sum(1 for f in state.findings if f.get("filed"))
        validation.append("- Review: %d finding(s) fixed in-branch, %d filed to the backlog." % (fixed, filed))
    if state.blast_radius:
        validation.append("- Blast radius **%s**: %s" % (state.blast_radius, state.blast_radius_why))
    sections = [("Summary", [summary]), ("Changes", bullets(done))]
    if blocked:
        sections.append(("Blocked", bullets(blocked)))
    sections.append(("Validation", validation))
    return sections


def pr_body(state: RunState, group_title: str, task_titles: Dict[str, str], commits: Sequence[str], template: Optional[str] = None) -> str:
    """A PR description: the repo's template filled when present, else the sections on their own."""
    sections = pr_sections(state, group_title, task_titles)
    if template and "## Summary" in template:
        return fill_template(template, sections)
    lines: List[str] = []
    for heading, body in sections:
        lines += ["## " + heading, ""] + body + [""]
    return "\n".join(lines).rstrip() + "\n"


def fill_template(template: str, sections: Sequence[tuple]) -> str:
    """The template with each heading's content substituted, its comments stripped, and every empty section dropped."""
    content = dict(sections)
    blocks: List[tuple] = [(None, [])]
    for line in HTML_COMMENT.sub("", template).splitlines():
        if line.startswith("## "):
            blocks.append((line[3:].strip(), []))
        else:
            blocks[-1][1].append(line)
    lines: List[str] = []
    for heading, buffered in blocks:
        if heading is None:
            lines += [line for line in buffered if line.strip()]
            continue
        body = content.get(heading) or [line for line in buffered if line.strip()]
        if body:
            lines += ["## " + heading, ""] + body + [""]
    known = set(name for name, _ in blocks if name)
    for heading, body in sections:
        if heading in known or not body:
            continue
        lines += ["## " + heading, ""] + body + [""]
    return "\n".join(lines).rstrip() + "\n"


def next_version(current: str, bump: str) -> str:
    """The semantic version after a major, minor, or patch bump."""
    parts = [int(piece) for piece in current.split(".")[:3]]
    while len(parts) < 3:
        parts.append(0)
    major, minor, patch = parts
    if bump == "major":
        return "%d.0.0" % (major + 1)
    if bump == "minor":
        return "%d.%d.0" % (major, minor + 1)
    return "%d.%d.%d" % (major, minor, patch + 1)


def newest_version(text: str) -> Optional[str]:
    """The version in the first numbered changelog heading, or None when nothing is released."""
    for line in text.splitlines():
        if line.startswith("## ["):
            return line.split("[", 1)[1].split("]", 1)[0].strip()
    return None


VERSION_HEADING = re.compile(r"^## \[(\d+\.\d+\.\d+[^\]]*)\][ \t]*(\S+)?[ \t]*(\d{4}-\d{2}-\d{2})?", re.M)
NEVER_RELEASED = re.compile(r"never released", re.IGNORECASE)
KEEP_A_CHANGELOG_SEPARATOR = "-"


def released_versions(text: str) -> List[str]:
    """Every version carrying a numbered changelog heading, newest heading first."""
    return [match.group(1).strip() for match in VERSION_HEADING.finditer(text)]


def assert_preserved(before: str, after: str) -> None:
    """Raises when a changelog write would drop a released version heading."""
    kept = set(released_versions(after))
    lost = [version for version in released_versions(before) if version not in kept]
    if lost:
        raise ValueError("refusing a changelog write that drops released version(s): %s" % ", ".join(lost))


def _heading_sections(text: str) -> List[Tuple[str, str, str]]:
    """Each numbered heading as its version, its own line, and the lines under it up to the next heading."""
    lines = text.splitlines()
    starts = [index for index, line in enumerate(lines) if VERSION_HEADING.match(line)]
    out: List[Tuple[str, str, str]] = []
    for position, index in enumerate(starts):
        end = starts[position + 1] if position + 1 < len(starts) else len(lines)
        match = VERSION_HEADING.match(lines[index])
        out.append((match.group(1).strip(), lines[index], "\n".join(lines[index + 1: end])))
    return out


def _is_never_released(line: str, body: str) -> bool:
    """Whether a heading or the blockquote under it marks the version as never released."""
    if NEVER_RELEASED.search(line):
        return True
    for row in body.splitlines():
        if not row.strip():
            continue
        return row.strip().startswith(">") and bool(NEVER_RELEASED.search(row))
    return False


def taggable_versions(text: str) -> List[str]:
    """Versions with a numbered heading that is not annotated as never released."""
    return [version for version, line, body in _heading_sections(text) if not _is_never_released(line, body)]


def never_released_versions(text: str) -> List[str]:
    """Versions whose heading or blockquote says they were never released, so no tag is expected."""
    return [version for version, line, body in _heading_sections(text) if _is_never_released(line, body)]


def changelog_drift(text: str, tags: Sequence[str]) -> List[str]:
    """Every release-integrity problem: an untagged entry, a tagged version with no entry, a separator mismatch."""
    problems: List[str] = []
    tagged = {name[1:] for name in tags if name.startswith("v")}
    entries = released_versions(text)
    for version in taggable_versions(text):
        if version not in tagged:
            problems.append("%s has a changelog entry and no v%s tag; tag it with `git tag -a v%s -m \"release %s\"`" % (version, version, version, version))
    for version in sorted(tagged - set(entries)):
        problems.append("v%s is tagged and has no changelog entry; recover it with `git show v%s:CHANGELOG.md`" % (version, version))
    separators = [match.group(2) for match in (VERSION_HEADING.match(line) for _, line, _ in _heading_sections(text)) if match.group(2) and match.group(3)]
    if separators:
        dominant = max(set(separators), key=separators.count)
        odd = sorted({separator for separator in separators if separator != dominant})
        if odd:
            problems.append("date separator mismatch: %d heading(s) use %r, %s also appear(s); Keep a Changelog specifies %r" % (
                separators.count(dominant), dominant, " and ".join(repr(item) for item in odd), KEEP_A_CHANGELOG_SEPARATOR))
    return problems


def changelog_entry(kind: str, titles: Sequence[str]) -> List[str]:
    """Bullets for the changelog under the heading kind maps to."""
    return ["- " + clip_sentence(title) for title in titles]


def changelog_heading(kind: str) -> str:
    """Keep a Changelog heading for a conventional-commit type."""
    return {"fix": "Fixed", "feat": "Added", "perf": "Changed", "refactor": "Changed", "docs": "Changed", "chore": "Changed", "build": "Changed", "ci": "Changed", "test": "Changed"}.get(kind, "Changed")

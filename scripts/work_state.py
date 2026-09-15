#!/usr/bin/env python3
"""Emit a repo's whole work state as a position-independent JSON graph.

Reads BACKLOG.md (what is planned and open) and CHANGELOG.md (what shipped)
and prints one JSON document to stdout. Standard library only; the floor is
Python 3.7, matching the rest of this repo's hook layer.

    python3 scripts/work_state.py [--repo-root DIR] [--releases N]

The data model is the contract; the renderer is swappable. A node carries its
identity, level, state, counts, and edges, and NEVER a coordinate -- layout is
the renderer's job, which is what lets a 2D and a 3D renderer consume the same
output unchanged.

Node kinds:

    epic        an EPIC-XX heading in BACKLOG.md
    task_group  a TG-XX.Y heading in BACKLOG.md, with per-state task counts
    release     a version section in CHANGELOG.md ([Unreleased] included)

Nodes are epics, task groups, and releases only. A task or subtask is never a
node; a task group's `counts` is how open, blocked, and shipped work becomes
visible at group level.

KNOWN LIMITATION -- shipped work is not linked to a task group. When a band
closes out, its finished tasks are DELETED from BACKLOG.md and survive only as
CHANGELOG.md prose, which carries no TG- or TSK- identifiers. So releases form
a separate chronological spine (`follows` edges between consecutive versions)
rather than attaching to the task groups that produced them. Any per-group
linkage would have to be guessed from prose similarity, which is invention, not
parsing. The spine is what the data actually supports.

Edges:

    contains  epic -> task_group, and epic -> nothing else
    after     task_group -> task_group, resolved from a task's
              (after: "<title substring>") annotation; `intra_group` marks a
              pair of tasks that share one group
    follows   release -> the release published before it

An `(after:)` target naming a task that has already shipped (and so has been
deleted from BACKLOG.md) cannot be resolved and is reported under `warnings`
rather than dropped silently or matched fuzzily.

A missing or malformed source file is reported under `warnings` and parsing
continues. The exit status is 1 only when neither source file could be read.
"""

import argparse
import json
import os
import re
import sys

SCHEMA = "komodo.work-state/1"

EPIC_RE = re.compile(r"^##\s+\[(EPIC-\d+)\]\s*(.*?)\s*$")
GROUP_RE = re.compile(r"^###\s+\[(TG-\d+\.\d+)\]\s*(.*?)\s*$")
TASK_RE = re.compile(r"^####\s+\[(TSK-\d+\.\d+\.\d+)\]\s*(.*?)\s*$")
TARGET_RE = re.compile(r"^\*\s+\*\*Target Release:\*\*\s*(.+?)\s*$")
AC_RE = re.compile(r"^-\s+\[([ xX])\]\s+\*\*AC-")
AFTER_RE = re.compile(r"\(after:\s*\"(.+?)\"\s*\)")
STATUS_RE = re.compile(r"\[(TODO|IN_PROGRESS|BLOCKED|DONE)\]\s*$")
PRIORITY_RE = re.compile(r"\[P:\s*([CHML])\]")
ARCHIVE_RE = re.compile(r"^##\s+Archive\s*$")

UNRELEASED_RE = re.compile(r"^##\s+\[Unreleased\]\s*$", re.IGNORECASE)
VERSION_RE = re.compile(r"^##\s+\[(\d+\.\d+\.\d+)\]\s*(?:[-\u2013\u2014]\s*(.+?))?\s*$")
CHANGE_GROUP_RE = re.compile(r"^###\s+(Added|Changed|Fixed|Removed|Security)\s*$")
CHANGE_ITEM_RE = re.compile(r"^-\s+\S")

STATUSES = ("todo", "in_progress", "blocked", "done")
PRIORITIES = ("C", "H", "M", "L")
CHANGE_GROUPS = ("Added", "Changed", "Fixed", "Removed", "Security")


def read_lines(path, warnings, label):
    if not os.path.isfile(path):
        warnings.append({"kind": "missing_source", "source": label, "detail": path})
        return None
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as handle:
            return handle.read().splitlines()
    except OSError as exc:
        warnings.append({"kind": "unreadable_source", "source": label, "detail": str(exc)})
        return None


def empty_counts():
    counts = dict((status, 0) for status in STATUSES)
    counts["total"] = 0
    return counts


def empty_priorities():
    return dict((priority, 0) for priority in PRIORITIES)


def empty_changes():
    return dict((name, 0) for name in CHANGE_GROUPS)


def add_counts(target, source):
    for key in target:
        target[key] += source[key]


def strip_annotations(title):
    text = AFTER_RE.sub("", title)
    text = PRIORITY_RE.sub("", text)
    text = STATUS_RE.sub("", text)
    return re.sub(r"\s{2,}", " ", text).strip(" -\u2013\u2014")


def derive_state(counts):
    if counts["total"] == 0:
        return "empty"
    if counts["done"] == counts["total"]:
        return "done"
    if counts["in_progress"] > 0:
        return "in_progress"
    if counts["blocked"] > 0 and counts["blocked"] + counts["done"] == counts["total"]:
        return "blocked"
    return "open"


def parse_backlog(lines, warnings):
    epics = []
    groups = []
    tasks = []
    epic = None
    group = None
    task = None

    for number, line in enumerate(lines, 1):
        if ARCHIVE_RE.match(line):
            break

        match = EPIC_RE.match(line)
        if match:
            epic = {
                "id": match.group(1),
                "kind": "epic",
                "level": "epic",
                "title": match.group(2).strip() or match.group(1),
                "parent": None,
                "counts": empty_counts(),
                "priorities": empty_priorities(),
                "acceptance": {"checked": 0, "total": 0},
                "line": number,
            }
            epics.append(epic)
            group = None
            task = None
            continue

        match = GROUP_RE.match(line)
        if match:
            if epic is None:
                warnings.append({
                    "kind": "orphan_task_group",
                    "source": "backlog",
                    "detail": "%s at line %d precedes any EPIC heading" % (match.group(1), number),
                })
            group = {
                "id": match.group(1),
                "kind": "task_group",
                "level": "task_group",
                "title": match.group(2).strip() or match.group(1),
                "parent": epic["id"] if epic else None,
                "target_release": None,
                "counts": empty_counts(),
                "priorities": empty_priorities(),
                "acceptance": {"checked": 0, "total": 0},
                "line": number,
            }
            groups.append(group)
            task = None
            continue

        match = TASK_RE.match(line)
        if match:
            task = None
            if group is None:
                warnings.append({
                    "kind": "orphan_task",
                    "source": "backlog",
                    "detail": "%s at line %d precedes any TG heading" % (match.group(1), number),
                })
                continue
            raw = match.group(2)
            status_match = STATUS_RE.search(raw)
            if status_match is None:
                warnings.append({
                    "kind": "missing_status",
                    "source": "backlog",
                    "detail": "%s at line %d carries no [TODO|IN_PROGRESS|BLOCKED|DONE]" % (match.group(1), number),
                })
                status = "todo"
            else:
                status = status_match.group(1).lower()
            after = AFTER_RE.search(raw)
            priority_match = PRIORITY_RE.search(raw)
            task = {
                "id": match.group(1),
                "group": group["id"],
                "title": strip_annotations(raw),
                "status": status,
                "after": after.group(1) if after else None,
                "line": number,
            }
            tasks.append(task)
            group["counts"][status] += 1
            group["counts"]["total"] += 1
            if priority_match:
                group["priorities"][priority_match.group(1)] += 1
            continue

        if group is not None and group["target_release"] is None:
            match = TARGET_RE.match(line)
            if match:
                group["target_release"] = match.group(1)
                continue

        if task is not None and group is not None:
            match = AC_RE.match(line)
            if match:
                group["acceptance"]["total"] += 1
                if match.group(1) in ("x", "X"):
                    group["acceptance"]["checked"] += 1

    by_id = dict((item["id"], item) for item in epics)
    for item in groups:
        parent = by_id.get(item["parent"])
        if parent is not None:
            add_counts(parent["counts"], item["counts"])
            for priority in PRIORITIES:
                parent["priorities"][priority] += item["priorities"][priority]
            parent["acceptance"]["checked"] += item["acceptance"]["checked"]
            parent["acceptance"]["total"] += item["acceptance"]["total"]

    for item in epics + groups:
        item["state"] = derive_state(item["counts"])

    return epics, groups, tasks


def resolve_after_edges(tasks, warnings):
    edges = []
    for task in tasks:
        if not task["after"]:
            continue
        needle = task["after"].lower()
        matches = [other for other in tasks
                   if other["id"] != task["id"] and needle in other["title"].lower()]
        if len(matches) != 1:
            warnings.append({
                "kind": "unresolved_after" if not matches else "ambiguous_after",
                "source": "backlog",
                "detail": "%s depends on \"%s\", which matches %d open tasks "
                          "(a shipped task is deleted from BACKLOG.md, so it cannot be matched)"
                          % (task["id"], task["after"], len(matches)),
            })
            continue
        target = matches[0]
        edges.append({
            "type": "after",
            "from": target["group"],
            "to": task["group"],
            "from_task": target["id"],
            "to_task": task["id"],
            "intra_group": target["group"] == task["group"],
        })
    return edges


def parse_changelog(lines, warnings, limit):
    releases = []
    current = None
    group = None

    for number, line in enumerate(lines, 1):
        if UNRELEASED_RE.match(line):
            current = {
                "id": "REL-unreleased",
                "kind": "release",
                "level": "release",
                "title": "Unreleased",
                "version": None,
                "date": None,
                "released": False,
                "state": "unreleased",
                "parent": None,
                "changes": empty_changes(),
                "change_count": 0,
                "line": number,
            }
            releases.append(current)
            group = None
            continue

        match = VERSION_RE.match(line)
        if match:
            current = {
                "id": "REL-" + match.group(1),
                "kind": "release",
                "level": "release",
                "title": match.group(1),
                "version": match.group(1),
                "date": match.group(2),
                "released": True,
                "state": "shipped",
                "parent": None,
                "changes": empty_changes(),
                "change_count": 0,
                "line": number,
            }
            if match.group(2) is None:
                warnings.append({
                    "kind": "missing_release_date",
                    "source": "changelog",
                    "detail": "version %s at line %d has no date" % (match.group(1), number),
                })
            releases.append(current)
            group = None
            continue

        if line.startswith("## ") and current is not None:
            current = None
            group = None
            continue

        match = CHANGE_GROUP_RE.match(line)
        if match:
            group = match.group(1) if current is not None else None
            continue

        if current is not None and group is not None and CHANGE_ITEM_RE.match(line):
            current["changes"][group] += 1
            current["change_count"] += 1

    released = [item for item in releases if item["released"]]
    for index, item in enumerate(released):
        item["current"] = index == 0
    version = released[0]["version"] if released else None
    if version is None:
        warnings.append({
            "kind": "no_released_version",
            "source": "changelog",
            "detail": "no [X.Y.Z] heading found below [Unreleased]",
        })

    kept = releases
    if limit > 0:
        kept = [item for item in releases if not item["released"]] + released[:limit]
    return kept, version, len(released)


def release_edges(releases):
    ordered = [item for item in releases if not item["released"]]
    ordered.extend([item for item in releases if item["released"]])
    edges = []
    for index in range(len(ordered) - 1):
        edges.append({"type": "follows", "from": ordered[index]["id"], "to": ordered[index + 1]["id"]})
    return edges


def build(repo_root, backlog_path, changelog_path, release_limit):
    warnings = []
    nodes = []
    edges = []

    backlog_lines = read_lines(backlog_path, warnings, "backlog")
    if backlog_lines is None and os.path.basename(backlog_path) == "BACKLOG.md":
        fallback = os.path.join(repo_root, "docs", "BACKLOG.md")
        if os.path.isfile(fallback):
            backlog_path = fallback
            warnings.pop()
            backlog_lines = read_lines(fallback, warnings, "backlog")

    epics = []
    groups = []
    if backlog_lines is not None:
        epics, groups, tasks = parse_backlog(backlog_lines, warnings)
        nodes.extend(epics)
        nodes.extend(groups)
        for group in groups:
            if group["parent"]:
                edges.append({"type": "contains", "from": group["parent"], "to": group["id"]})
        edges.extend(resolve_after_edges(tasks, warnings))

    changelog_lines = read_lines(changelog_path, warnings, "changelog")
    released_version = None
    release_total = 0
    if changelog_lines is not None:
        releases, released_version, release_total = parse_changelog(
            changelog_lines, warnings, release_limit)
        nodes.extend(releases)
        edges.extend(release_edges(releases))

    totals = empty_counts()
    for epic in epics:
        add_counts(totals, epic["counts"])

    return {
        "schema": SCHEMA,
        "repo_root": repo_root,
        "sources": {
            "backlog": os.path.relpath(backlog_path, repo_root),
            "changelog": os.path.relpath(changelog_path, repo_root),
        },
        "released_version": released_version,
        "release_count": release_total,
        "releases_shown": len([n for n in nodes if n["kind"] == "release"]),
        "totals": totals,
        "counts": {"epics": len(epics), "task_groups": len(groups), "nodes": len(nodes), "edges": len(edges)},
        "nodes": nodes,
        "edges": edges,
        "warnings": warnings,
        "read_only": True,
    }, (backlog_lines is None and changelog_lines is None)


def main(argv=None):
    parser = argparse.ArgumentParser(description="Emit a repo's work state as a JSON graph.")
    parser.add_argument("--repo-root", default=os.getcwd())
    parser.add_argument("--backlog", default=None)
    parser.add_argument("--changelog", default=None)
    parser.add_argument("--releases", type=int, default=8,
                        help="how many released versions to include, newest first; 0 for all")
    parser.add_argument("--indent", type=int, default=2)
    args = parser.parse_args(argv)

    repo_root = os.path.abspath(args.repo_root)
    backlog = os.path.abspath(args.backlog or os.path.join(repo_root, "BACKLOG.md"))
    changelog = os.path.abspath(args.changelog or os.path.join(repo_root, "CHANGELOG.md"))

    graph, both_missing = build(repo_root, backlog, changelog, max(0, args.releases))
    indent = args.indent if args.indent > 0 else None
    sys.stdout.write(json.dumps(graph, indent=indent, sort_keys=False) + "\n")

    if both_missing:
        sys.stderr.write("work_state: neither BACKLOG.md nor CHANGELOG.md could be read\n")
        return 1
    for warning in graph["warnings"]:
        sys.stderr.write("work_state: %s: %s\n" % (warning["kind"], warning["detail"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())

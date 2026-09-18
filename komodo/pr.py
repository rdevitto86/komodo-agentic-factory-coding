"""GitHub pull request actions through the gh CLI, each scoped to one branch's own PR."""

from __future__ import annotations

import json
import re
import subprocess
from typing import Any, Dict, List, Optional


class GhError(RuntimeError):
    """gh exited non-zero or returned something unparseable."""


def gh(*args: str, cwd: str, timeout: int = 120, check: bool = True) -> str:
    """Runs gh and returns stdout."""
    result = subprocess.run(["gh", *args], cwd=cwd, capture_output=True, text=True, timeout=timeout)
    if check and result.returncode != 0:
        raise GhError("gh %s failed: %s" % (" ".join(args[:3]), result.stderr.strip()[:600]))
    return result.stdout


def gh_json(*args: str, cwd: str) -> Any:
    """Runs gh and parses its JSON output."""
    out = gh(*args, cwd=cwd)
    try:
        return json.loads(out) if out.strip() else None
    except ValueError:
        raise GhError("gh returned non-JSON for %s" % " ".join(args[:3]))


def available(cwd: str) -> bool:
    """Whether gh is installed and authenticated."""
    try:
        return subprocess.run(["gh", "auth", "status"], cwd=cwd, capture_output=True, timeout=30).returncode == 0
    except (OSError, subprocess.SubprocessError):
        return False


def repo_slug(cwd: str) -> str:
    """owner/name for the current repo."""
    data = gh_json("repo", "view", "--json", "nameWithOwner", cwd=cwd)
    return str(data["nameWithOwner"])


def view(cwd: str, branch: Optional[str] = None) -> Optional[Dict[str, Any]]:
    """The PR for a branch (default: current), or None when there is none."""
    args = ["pr", "view", "--json", "number,url,state,title,labels,headRefName,baseRefName,mergedAt"]
    if branch:
        args.append(branch)
    try:
        return gh_json(*args, cwd=cwd)
    except GhError:
        return None


def create(cwd: str, title: str, body: str, base: str, labels: List[str], draft: bool = False) -> str:
    """Opens a PR for the current branch and returns its URL."""
    args = ["pr", "create", "--title", title, "--body", body, "--base", base]
    for label in labels:
        args += ["--label", label]
    if draft:
        args.append("--draft")
    return gh(*args, cwd=cwd, timeout=180).strip()


def edit(cwd: str, number: int, body: Optional[str] = None, title: Optional[str] = None, add_labels: List[str] = ()) -> None:
    """Updates the body, title, or labels of a PR the branch owns."""
    args = ["pr", "edit", str(number)]
    if body is not None:
        args += ["--body", body]
    if title is not None:
        args += ["--title", title]
    for label in add_labels:
        args += ["--add-label", label]
    gh(*args, cwd=cwd)


def existing_labels(cwd: str) -> List[str]:
    """Label names the repo defines."""
    data = gh_json("label", "list", "--json", "name", "--limit", "200", cwd=cwd) or []
    return [str(item["name"]) for item in data]


# A conventional-commit subject: the type, an optional scope, then the colon.
CONVENTIONAL = re.compile(r"^([a-z]+)(\([^)]*\))?!?:")


def kind_of(title: str, branch: str) -> str:
    """The commit type a PR carries, read from its title, then from its branch prefix."""
    match = CONVENTIONAL.match(title.strip())
    if match:
        return match.group(1)
    head = branch.split("/", 1)[0].strip().lower()
    return head if head and head != branch else ""


def pick_labels(kind: str, mapping: Dict[str, str], defined: List[str]) -> List[str]:
    """Category and authorship labels that exist in the repo, never inventing one."""
    chosen = []
    category = mapping.get(kind)
    if category and category in defined:
        chosen.append(category)
    agent = mapping.get("agent")
    if agent and agent in defined:
        chosen.append(agent)
    return chosen


def comment(cwd: str, number: int, body: str) -> None:
    """Posts a top-level PR comment."""
    gh("pr", "comment", str(number), "--body", body, cwd=cwd)


def reply(cwd: str, number: int, comment_id: int, body: str) -> None:
    """Replies inside a review thread; gh pr comment has no thread mode, so this is the one raw API write."""
    slug = repo_slug(cwd)
    gh("api", "-X", "POST", "repos/%s/pulls/%d/comments/%d/replies" % (slug, number, comment_id), "-f", "body=%s" % body, cwd=cwd)


def review_threads(cwd: str, number: int) -> List[Dict[str, Any]]:
    """Unresolved review threads with their comments, via GraphQL."""
    slug = repo_slug(cwd)
    owner, name = slug.split("/", 1)
    query = (
        "query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){pullRequest(number:$number){"
        "reviewThreads(first:50){nodes{id isResolved path line comments(first:20){nodes{databaseId author{login} body createdAt}}}}}}}"
    )
    data = gh_json("api", "graphql", "-f", "query=%s" % query, "-F", "owner=%s" % owner, "-F", "name=%s" % name, "-F", "number=%d" % number, cwd=cwd)
    nodes = (((data or {}).get("data") or {}).get("repository") or {}).get("pullRequest", {}).get("reviewThreads", {}).get("nodes", [])
    threads = []
    for node in nodes:
        if node.get("isResolved"):
            continue
        comments = [
            {"id": item.get("databaseId"), "author": (item.get("author") or {}).get("login", ""), "body": item.get("body", ""), "at": item.get("createdAt", "")}
            for item in node.get("comments", {}).get("nodes", [])
        ]
        threads.append({"id": node.get("id"), "path": node.get("path") or "", "line": node.get("line") or 0, "comments": comments})
    return threads


def merged_prs_for(cwd: str, branches: List[str]) -> List[str]:
    """Which of the given branches have a merged PR."""
    merged = []
    for branch in branches:
        info = view(cwd, branch)
        if info and info.get("state") == "MERGED":
            merged.append(branch)
    return merged

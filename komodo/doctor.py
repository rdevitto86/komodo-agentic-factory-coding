"""Checks that catch fragments: dangling references, personal keys in policy, stale branches and worktrees."""

from __future__ import annotations

import json
import os
import re
import subprocess
from typing import Dict, List, Optional

from . import gitops, standards

PERSONAL_KEYS = ("effortLevel", "model", "modelOverrides", "agentPushNotifEnabled", "theme", "editorMode", "autoUpdates", "preferredNotifChannel", "statusLine")
BACKTICK = re.compile(r"`([^`\n]+)`")
SKILL_REF = re.compile(r"(?<![\w/])/([a-z][a-z0-9-]+)\b")
PATH_LIKE = re.compile(r"^(?:[\w.-]+/)+[\w.-]+$|^[\w.-]+\.(?:py|md|json|yml|yaml|sh|toml|txt)$")
SKIP_DIRS = (".git", "node_modules", ".komodo", "vendor", "__pycache__", "synced")
KNOWN_ABSENT = ("claude-code/settings.json", ".komodo/local.json", "settings.local.json", "CLAUDE.local.md")
FOREIGN_PREFIXES = (".claude/", ".komodo/", "docs/spec/", "cdk/", "internal/", "src/", "pkg/", "cmd/", "web/")


def _walk_markdown(root: str) -> List[str]:
    """Every markdown file under root outside skipped directories."""
    found = []
    for directory, dirs, names in os.walk(root):
        dirs[:] = [name for name in dirs if name not in SKIP_DIRS and (not name.startswith(".") or name == ".github")]
        for name in names:
            if name.endswith(".md"):
                found.append(os.path.join(directory, name))
    return found


def _basenames(root: str) -> set:
    """Every file basename in the tree, so a bare `guard.py` mention resolves wherever the file lives."""
    names = set()
    for directory, dirs, files in os.walk(root):
        dirs[:] = [name for name in dirs if name not in SKIP_DIRS and not name.startswith(".")]
        names.update(files)
    return names


def _resolves(root: str, doc_dir: str, candidate: str, basenames: set) -> bool:
    """Whether a backticked path points at something real, by root, by the doc's directory, by komodo/, or by bare name."""
    if candidate in KNOWN_ABSENT or candidate.startswith(FOREIGN_PREFIXES):
        return True
    for base in (root, doc_dir, os.path.join(root, "komodo"), os.path.join(root, "claude-code")):
        if os.path.exists(os.path.join(base, candidate)):
            return True
    return "/" not in candidate and candidate in basenames


def _known_names(root: str) -> Dict[str, set]:
    """Skill, agent, standard, and CLI subcommand names that a reference may legitimately point at."""
    skills = set()
    skills_dir = os.path.join(root, "claude-code", "skills")
    if os.path.isdir(skills_dir):
        skills = {name for name in os.listdir(skills_dir) if os.path.isfile(os.path.join(skills_dir, name, "SKILL.md"))}
    agents = set()
    agents_dir = os.path.join(root, "claude-code", "agents")
    if os.path.isdir(agents_dir):
        agents = {name[:-3] for name in os.listdir(agents_dir) if name.endswith(".md")}
    return {"skills": skills, "agents": agents, "standards": set(standards.available())}


def check_references(root: str) -> List[str]:
    """Backticked repo paths and /skill names in markdown that no longer resolve."""
    problems: List[str] = []
    known = _known_names(root)
    basenames = _basenames(root)
    for path in _walk_markdown(root):
        relative = os.path.relpath(path, root).replace("\\", "/")
        if relative.startswith(("docs/design-decisions", "CHANGELOG", "komodo/standards/")):
            continue
        try:
            text = open(path, encoding="utf-8", errors="ignore").read()
        except OSError:
            continue
        in_fence = False
        for number, line in enumerate(text.splitlines(), 1):
            if line.strip().startswith("```"):
                in_fence = not in_fence
                continue
            if in_fence:
                continue
            for token in BACKTICK.findall(line):
                candidate = token.strip()
                if candidate.startswith(("~", "$", "<", "http", "-")) or "*" in candidate or "{" in candidate or " " in candidate:
                    continue
                if PATH_LIKE.match(candidate) and not _resolves(root, os.path.dirname(path), candidate, basenames):
                    if candidate.split("/")[0] in ("komodo", "claude-code", "scripts", "tests", "docs", "templates", ".github", "workers", "briefs", "standards", "hooks") or candidate.endswith(".py"):
                        problems.append("%s:%d: `%s` does not exist" % (relative, number, candidate))
            for name in SKILL_REF.findall(line):
                if name in ("dev", "tmp", "usr", "bin", "etc", "var", "home", "api", "v1", "v2", "orders", "items") or "/" + name + "/" in line:
                    continue
                if known["skills"] and name not in known["skills"] and name in _historic_skill_names():
                    problems.append("%s:%d: /%s names a skill that no longer exists" % (relative, number, name))
    return problems


def _historic_skill_names() -> set:
    """Skill names the toolkit once shipped, so a lingering reference is caught by name."""
    return {
        "workflow-loop", "workflow-implement", "workflow-decompose", "workflow-consolidate", "workflow-complete", "workflow-debug",
        "backlog-modify", "backlog-plan", "backlog-audit", "backlog-prioritize", "assess-bugs", "assess-security", "assess-simplify",
        "assess-performance", "assess-code-quality", "assess-testing", "assess-readiness", "assess-dependencies", "assess-vulnerabilities",
        "assess-change-risk", "assess-code-conventions", "changelog-write", "changelog-audit", "readme-modify", "readme-audit",
        "git-pr-create", "git-commit-message", "git-commit-tag", "git-repo-init", "git-pr-review", "git-pr-comment", "git-issue-create",
        "git-issue-review", "git-merge-conflict", "git-branching-strategy", "write-comments", "work-state-map", "repo-assess",
        "config-accessibility", "standards-comments", "standards-worklog", "sdd", "prd", "adr", "runbook",
    }


def check_policy(root: str) -> List[str]:
    """Personal preference keys that leaked into the shipped settings policy, and hook commands naming missing files."""
    problems: List[str] = []
    policy = os.path.join(root, "claude-code", "settings.policy.json")
    if not os.path.isfile(policy):
        return ["claude-code/settings.policy.json is missing"]
    try:
        data = json.load(open(policy, encoding="utf-8"))
    except ValueError as error:
        return ["claude-code/settings.policy.json: %s" % error]
    for key in PERSONAL_KEYS:
        if key in data:
            problems.append("settings.policy.json carries personal key %r; it belongs in ~/.claude/settings.json" % key)
    for event, entries in (data.get("hooks") or {}).items():
        for entry in entries:
            for hook in entry.get("hooks", []):
                command = str(hook.get("command", ""))
                for token in command.split():
                    if token.endswith(".py"):
                        expected = os.path.join(root, "claude-code", "hooks", os.path.basename(token))
                        if not os.path.isfile(expected):
                            problems.append("settings.policy.json %s hook names %s, which is not in claude-code/hooks/" % (event, os.path.basename(token)))
    tracked = subprocess.run(["git", "ls-files", "claude-code/settings.json"], cwd=root, capture_output=True, text=True).stdout.strip()
    if tracked:
        problems.append("claude-code/settings.json is tracked; only settings.policy.json ships")
    return problems


def check_git_leftovers(root: str, protected: List[str], base: Optional[str] = None) -> List[str]:
    """Worktrees the harness left behind and local branches whose work already merged."""
    problems: List[str] = []
    git = gitops.Git(root, protected)
    try:
        for path in git.worktrees():
            problems.append("stale worktree %s" % path)
        base = base or git.default_base()
        for name in git.merged_branches(base):
            if name != git.current_branch():
                problems.append("branch %s is merged into %s and can be deleted" % (name, base))
    except gitops.GitError as error:
        problems.append("git: %s" % error)
    return problems


def check_skills(root: str) -> List[str]:
    """Skill files with disabled suffixes, missing frontmatter, or unknown frontmatter keys."""
    allowed = {"name", "description", "when_to_use", "model", "effort", "allowed-tools", "disallowed-tools", "argument-hint", "disable-model-invocation", "user-invocable", "paths", "context", "agent", "background", "hooks", "metadata", "shell", "license", "compatibility"}
    problems: List[str] = []
    skills_dir = os.path.join(root, "claude-code", "skills")
    if not os.path.isdir(skills_dir):
        return problems
    for name in sorted(os.listdir(skills_dir)):
        folder = os.path.join(skills_dir, name)
        if not os.path.isdir(folder) or name == "synced":
            continue
        for entry in os.listdir(folder):
            if entry.endswith(".off"):
                problems.append("claude-code/skills/%s/%s: disabled file lingering; delete it" % (name, entry))
        skill = os.path.join(folder, "SKILL.md")
        if not os.path.isfile(skill):
            problems.append("claude-code/skills/%s has no SKILL.md" % name)
            continue
        text = open(skill, encoding="utf-8").read()
        if not text.startswith("---"):
            problems.append("claude-code/skills/%s/SKILL.md lacks frontmatter" % name)
            continue
        front = text.split("---", 2)[1]
        for line in front.splitlines():
            if ":" in line and not line.startswith((" ", "\t")):
                key = line.split(":", 1)[0].strip()
                if key not in allowed:
                    problems.append("claude-code/skills/%s/SKILL.md: unknown frontmatter key %r" % (name, key))
        if "name: %s" % name not in front:
            problems.append("claude-code/skills/%s/SKILL.md: name does not match its directory" % name)
    return problems


def run(root: str, protected: Optional[List[str]] = None, git_checks: bool = True) -> List[str]:
    """Every doctor check, concatenated."""
    problems = check_references(root) + check_policy(root) + check_skills(root)
    if git_checks:
        problems += check_git_leftovers(root, protected or ["main", "master"])
    return problems

#!/usr/bin/env python3
"""Validates the Claude Code adapter: frontmatter keys, the always-on token budget, agent profiles, and briefs."""

from __future__ import annotations

import json
import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ADAPTER = os.path.join(REPO_ROOT, "claude-code")
BUDGET_TOKENS = 1500
SKILL_KEYS = {"name", "description", "when_to_use", "model", "effort", "allowed-tools", "disallowed-tools", "argument-hint", "disable-model-invocation", "user-invocable", "paths", "context", "agent", "background", "hooks", "metadata", "shell", "license", "compatibility"}
AGENT_KEYS = {"name", "description", "tools", "disallowedTools", "model", "permissionMode", "maxTurns", "skills", "mcpServers", "hooks", "memory", "background", "effort", "isolation", "color", "initialPrompt"}
REQUIRED_AGENT = {"name", "description", "tools", "model", "effort", "maxTurns"}


def frontmatter(text: str):
    """The frontmatter as a dict of top-level keys, or None."""
    if not text.startswith("---"):
        return None
    parts = text.split("---", 2)
    if len(parts) < 3:
        return None
    keys = {}
    for line in parts[1].splitlines():
        if line and not line[0].isspace() and ":" in line:
            key, value = line.split(":", 1)
            keys[key.strip()] = value.strip()
    return keys


def tokens(text: str) -> int:
    """Rough token count."""
    return len(text) // 4


def main() -> int:
    """Runs every validation and prints each problem."""
    problems = []
    overrides = {}
    policy_path = os.path.join(ADAPTER, "settings.policy.json")
    try:
        overrides = json.load(open(policy_path, encoding="utf-8")).get("skillOverrides", {})
    except (OSError, ValueError) as error:
        problems.append("settings.policy.json: %s" % error)

    always_on = tokens(open(os.path.join(ADAPTER, "AGENTS.md"), encoding="utf-8").read())
    skills_dir = os.path.join(ADAPTER, "skills")
    for name in sorted(os.listdir(skills_dir)):
        folder = os.path.join(skills_dir, name)
        skill = os.path.join(folder, "SKILL.md")
        if not os.path.isdir(folder) or name == "synced":
            continue
        if not os.path.isfile(skill):
            problems.append("skills/%s: no SKILL.md" % name)
            continue
        text = open(skill, encoding="utf-8").read()
        keys = frontmatter(text)
        if keys is None:
            problems.append("skills/%s: missing frontmatter" % name)
            continue
        for key in keys:
            if key not in SKILL_KEYS:
                problems.append("skills/%s: unknown frontmatter key %r" % (name, key))
        if keys.get("name") != name:
            problems.append("skills/%s: name %r does not match directory" % (name, keys.get("name")))
        if "paths" in keys and not keys["paths"].startswith('"'):
            problems.append("skills/%s: paths value must be quoted" % name)
        if keys.get("disable-model-invocation", "").lower() in ("true", "yes"):
            continue
        cost = tokens(name) + 2 if overrides.get(name) == "name-only" else tokens(name + keys.get("description", "")) + 6
        always_on += cost

    if always_on > BUDGET_TOKENS:
        problems.append("always-on context is ~%d tokens, over the %d budget" % (always_on, BUDGET_TOKENS))

    agents_dir = os.path.join(ADAPTER, "agents")
    for name in sorted(os.listdir(agents_dir)):
        if not name.endswith(".md"):
            continue
        keys = frontmatter(open(os.path.join(agents_dir, name), encoding="utf-8").read())
        if keys is None:
            problems.append("agents/%s: missing frontmatter" % name)
            continue
        for key in keys:
            if key not in AGENT_KEYS:
                problems.append("agents/%s: unknown frontmatter key %r" % (name, key))
        for key in REQUIRED_AGENT - set(keys):
            problems.append("agents/%s: missing %r" % (name, key))
        if keys.get("name") != name[:-3]:
            problems.append("agents/%s: name does not match filename" % name)

    briefs_dir = os.path.join(REPO_ROOT, "komodo", "briefs")
    roles = {name.split(".")[0] for name in os.listdir(briefs_dir) if name.endswith(".md")}
    for role in sorted(roles):
        for part in ("system", "prompt"):
            if not os.path.isfile(os.path.join(briefs_dir, "%s.%s.md" % (role, part))):
                problems.append("briefs/%s.%s.md is missing" % (role, part))
    for stale in ("settings.json", "CLAUDE.local.md.tmpl.bak"):
        if os.path.isfile(os.path.join(ADAPTER, stale)) and stale == "settings.json":
            problems.append("claude-code/settings.json must not exist; policy lives in settings.policy.json")

    for problem in problems:
        print(problem)
    print("always-on context: ~%d tokens (budget %d); %d problem(s)" % (always_on, BUDGET_TOKENS, len(problems)))
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())

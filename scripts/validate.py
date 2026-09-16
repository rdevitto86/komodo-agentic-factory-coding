#!/usr/bin/env python3
"""Validates the global layer and the rendered Claude adapter: roles, frontmatter keys, the always-on budget, brief templates."""

from __future__ import annotations

import json
import os
import sys
import tempfile

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, REPO_ROOT)

from komodo import adapters, roles  # noqa: E402

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


def read(path: str) -> str:
    """File text."""
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def check_roles(problems: list) -> None:
    """Every role parses, and every worker role has a prompt template and a worker output contract."""
    briefs_dir = os.path.join(REPO_ROOT, "komodo", "briefs")
    templates = {name[: -len(".prompt.md")] for name in os.listdir(briefs_dir) if name.endswith(".prompt.md")}
    for name in roles.available():
        try:
            role = roles.load(name)
        except roles.RoleError as error:
            problems.append(str(error))
            continue
        if not role.purpose:
            problems.append("roles/%s.md: missing purpose" % name)
        if name in templates and not role.worker_output:
            problems.append("roles/%s.md: has a prompt template but no '## Worker output' section" % name)
        if role.session and not role.session_output:
            problems.append("roles/%s.md: session role without a '## Session output' section" % name)
    for name in templates - set(roles.available()):
        problems.append("briefs/%s.prompt.md has no role file" % name)


def check_rendered(problems: list) -> int:
    """Renders the Claude adapter into a scratch dir and checks frontmatter and the always-on budget."""
    with tempfile.TemporaryDirectory() as target:
        adapters.render("claude", target)
        with open(os.path.join(target, "settings.policy.json"), encoding="utf-8") as handle:
            overrides = json.load(handle).get("skillOverrides", {})
        always_on = tokens(read(os.path.join(target, "AGENTS.md")))
        skills_dir = os.path.join(target, "skills")
        for name in sorted(os.listdir(skills_dir)):
            keys = frontmatter(read(os.path.join(skills_dir, name, "SKILL.md")))
            if keys is None:
                problems.append("rendered skill %s: missing frontmatter" % name)
                continue
            for key in keys:
                if key not in SKILL_KEYS:
                    problems.append("rendered skill %s: unknown frontmatter key %r" % (name, key))
            if keys.get("name") != name:
                problems.append("rendered skill %s: name mismatch" % name)
            if "paths" in keys and not keys["paths"].startswith('"'):
                problems.append("rendered skill %s: paths value must be quoted" % name)
            always_on += tokens(name) + 2 if overrides.get(name) == "name-only" else tokens(name + keys.get("description", "")) + 6
        agents_dir = os.path.join(target, "agents")
        for name in sorted(os.listdir(agents_dir)):
            keys = frontmatter(read(os.path.join(agents_dir, name)))
            if keys is None:
                problems.append("rendered agent %s: missing frontmatter" % name)
                continue
            for key in keys:
                if key not in AGENT_KEYS:
                    problems.append("rendered agent %s: unknown frontmatter key %r" % (name, key))
            for key in REQUIRED_AGENT - set(keys):
                problems.append("rendered agent %s: missing %r" % (name, key))
        if always_on > BUDGET_TOKENS:
            problems.append("always-on context is ~%d tokens, over the %d budget" % (always_on, BUDGET_TOKENS))
        return always_on


def main() -> int:
    """Runs every validation and prints each problem."""
    problems: list = []
    check_roles(problems)
    always_on = check_rendered(problems)
    if os.path.isdir(os.path.join(REPO_ROOT, "claude-code")):
        problems.append("claude-code/ must not exist; the Claude layer is rendered from komodo/adapters/claude")
    for problem in problems:
        print(problem)
    print("always-on context: ~%d tokens (budget %d); %d problem(s)" % (always_on, BUDGET_TOKENS, len(problems)))
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())

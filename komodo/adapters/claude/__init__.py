"""Claude Code adapter: renders ~/.claude from the global rules, roles, and standards, plus its own hooks and policy."""

from __future__ import annotations

import json
import os
import platform
import shutil
from typing import Any, Dict, List, Optional

from ... import roles, standards
from ...briefs import TOOLS_BY_ACCESS

HERE = os.path.dirname(os.path.abspath(__file__))
KOMODO = os.path.dirname(os.path.dirname(HERE))
RULES = os.path.join(KOMODO, "rules")
POLICY = os.path.join(HERE, "settings.policy.json")
HOOKS = os.path.join(HERE, "hooks")
BIN = os.path.join(HOOKS, "bin")
SEEDS = os.path.join(HERE, "seeds")
WEB_TOOLS = ["WebFetch", "WebSearch"]
GUARD_BINARY = "guard"

ARCH_ALIASES = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}

STANDARD_PATHS: Dict[str, str] = {
    "go": "**/*.go, **/go.mod, **/go.sum",
    "typescript": "**/*.ts, **/*.tsx, **/*.js, **/*.jsx, **/*.mjs, **/package.json, **/tsconfig.json",
    "python": "**/*.py, **/*.pyi, **/pyproject.toml",
    "shell": "**/*.sh, **/*.bash",
    "cicd": "cicd.yaml, .github/workflows/**, **/Makefile, **/Taskfile*, **/Jenkinsfile, **/.gitlab-ci.yml",
    "database": "**/*.sql, **/migrations/**, **/schema/**",
    "api-design": "**/api/**, **/routes/**, **/controllers/**, **/handlers/**, **/*.proto, **/openapi*, **/graphql/**",
    "api-security": "**/api/**, **/routes/**, **/handlers/**, **/auth/**, **/*auth*, **/middleware/**, **/session*, **/token*, **/crypto/**, **/secrets/**, **/*.sql, **/*.tf",
    "aws": "**/*.tf, **/cdk/**, **/infra/**, **/infrastructure/**",
    "cdk": "**/cdk/**, **/infra/**, **/infrastructure/**, **/*-stack.ts, **/*.stack.ts, **/cdk.json",
    "docker": "**/Dockerfile*, **/docker-compose*, **/*.dockerfile",
    "observability": "**/logging/**, **/logger*, **/telemetry/**, **/metrics/**, **/tracing/**, **/otel*",
    "react": "**/*.tsx, **/*.jsx",
    "vue": "**/*.vue, **/stores/**",
    "svelte": "**/*.svelte, **/+page.ts, **/+page.server.ts, **/+layout.ts, **/+server.ts",
    "ui-web": "**/*.css, **/*.svelte, **/*.vue, **/*.tsx, **/*.jsx, **/*.html",
    "ui-mobile": "**/*.swift, **/*.kt, **/*.m, **/*.mm, **/*.storyboard, **/*.xib, **/AndroidManifest.xml, **/Info.plist",
    "ui-desktop": "**/tauri.conf.json, **/electron-builder.yml, **/electron-builder.json, **/forge.config.*, **/*.desktop, **/*.appxmanifest",
    "specs": "**/docs/spec/**",
    "sdlc": "**/*_test.*, **/*.test.*, **/*.spec.*, **/test/**, **/tests/**, **/__tests__/**, **/e2e/**",
    "comments": "**/*.go, **/*.py, **/*.ts, **/*.tsx, **/*.js, **/*.jsx, **/*.rs, **/*.java, **/*.kt, **/*.swift, **/*.cs, **/*.c, **/*.h, **/*.cpp, **/*.zig, **/*.sh, **/*.sql, **/*.vue, **/*.svelte",
    "rust": "**/*.rs, **/Cargo.toml",
    "zig": "**/*.zig, **/build.zig, **/build.zig.zon",
    "swift": "**/*.swift, **/Package.swift",
    "kotlin": "**/*.kt, **/*.kts",
    "c": "**/*.c, **/*.h",
    "cpp": "**/*.cc, **/*.cpp, **/*.hpp, **/*.ino",
    "csharp": "**/*.cs, **/*.csproj",
    "java": "**/*.java, **/pom.xml, **/build.gradle, **/build.gradle.kts",
    "dotnet": "**/*.csproj, **/*.sln, **/Directory.Build.props, **/appsettings*.json, **/global.json, **/Program.cs",
    "azure": "**/*.bicep, **/azure-pipelines*.yml, **/*.azure.*",
    "gcp": "**/*.gcp.*, **/cloudbuild*.yaml, **/app.yaml",
    "hardware": "**/*.kicad_sch, **/*.kicad_pcb, **/*.sch, **/*.brd, **/bom*.csv",
}

PROCEDURE_SKILLS = {
    "komodo": ("Drive the Komodo harness from a session: run a task group, check status, plan tasks, lint the backlog, act on a PR. Use when asked to build, ship, or run work end to end.", "cli.md", "[run <group> | status | tasks lint | pr respond]", None),
    "backlog": ("The BACKLOG.md task grammar the harness parses, and how to add or change a task. Use before editing BACKLOG.md or when asked to add work.", "backlog.md", None, '"**/BACKLOG.md"'),
}


def _write(target: str, relative: str, text: str, written: List[str]) -> None:
    """Writes one rendered file under target."""
    path = os.path.join(target, relative)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(text)
    written.append(relative)


def host_target() -> Optional[str]:
    """The goos-goarch pair this machine runs, or None when the platform is not one we name."""
    goos = platform.system().lower()
    goarch = ARCH_ALIASES.get(platform.machine().lower())
    if goos not in ("darwin", "linux", "windows") or goarch is None:
        return None
    return "%s-%s" % (goos, goarch)


def host_guard() -> Optional[str]:
    """Path to the prebuilt guard binary for this machine, or None when no committed target matches."""
    target = host_target()
    if target is None:
        return None
    name = "guard-%s%s" % (target, ".exe" if target.startswith("windows") else "")
    path = os.path.join(BIN, name)
    return path if os.path.isfile(path) else None


def _copy(target: str, relative: str, source: str, written: List[str]) -> None:
    """Copies a binary file under target and marks it executable."""
    path = os.path.join(target, relative)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    shutil.copyfile(source, path)
    os.chmod(path, 0o755)
    written.append(relative)


def _read(path: str) -> str:
    """Reads a source file."""
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def agent_frontmatter(role: roles.Role, spec: Dict[str, Any]) -> str:
    """Claude agent frontmatter for a role, model and effort taken from the active profile's tier."""
    tools = list(TOOLS_BY_ACCESS.get(role.access, []))
    if role.access == "read":
        tools.append("Bash")
    if role.extra.get("web", "").lower() in ("true", "yes"):
        tools += WEB_TOOLS
    lines = [
        "---",
        "name: %s" % role.name,
        "description: %s" % role.purpose,
        "tools: %s" % ", ".join(tools),
        "model: %s" % spec.get("model", "sonnet"),
        "effort: %s" % spec.get("effort", "medium"),
        "maxTurns: %d" % int(spec.get("max_turns", 40)),
        "---",
    ]
    return "\n".join(lines) + "\n\n"


def render_agents(target: str, config, written: List[str]) -> None:
    """One agents/<role>.md per session role."""
    for name, role in roles.all_roles().items():
        if not role.session:
            continue
        spec = config.role(name) if config is not None else {"model": "sonnet", "effort": "medium", "max_turns": 40}
        body = role.session_body().replace("{{comment_convention}}", "Follow the language standard's comment convention.")
        _write(target, os.path.join("agents", name + ".md"), agent_frontmatter(role, spec) + body, written)


def render_skills(target: str, written: List[str]) -> None:
    """Procedure skills from the rules documents, a review skill from the reviewer role, and one pointer per standard."""
    for name, (description, source, hint, paths) in PROCEDURE_SKILLS.items():
        front = ["---", "name: %s" % name, "description: %s" % description]
        if hint:
            front.append("argument-hint: %s" % hint)
        if paths:
            front.append("paths: %s" % paths)
        front.append("---")
        _write(target, os.path.join("skills", name, "SKILL.md"), "\n".join(front) + "\n\n" + _read(os.path.join(RULES, source)), written)
    reviewer = roles.load("reviewer")
    review = (
        "---\nname: review\ndescription: Review a diff cold in a session, the same way the harness reviewer does. Use when asked to review changes, a branch, or a PR.\n"
        "argument-hint: [base ref, default origin/main]\ncontext: fork\nagent: reviewer\n---\n\n"
        "# Review\n\nBase: `$ARGUMENTS` (default `origin/main`). Read `git diff <base>...HEAD` and the files it touches. Read `~/.claude/standards/api-security.md` when a route, auth path, or query changed.\n\n"
        + reviewer.session_body()
    )
    _write(target, os.path.join("skills", "review", "SKILL.md"), review, written)
    for name in standards.available():
        paths = STANDARD_PATHS.get(name)
        if not paths:
            continue
        with open(os.path.join(standards.STANDARDS_DIR, name + ".md"), encoding="utf-8") as handle:
            title = handle.readline().lstrip("# ").strip()
        text = (
            "---\nname: standards-%s\ndescription: %s standard. Loads on matching paths; the rules live in one file.\npaths: \"%s\"\n---\n\n"
            "# %s standard\n\nRead `~/.claude/standards/%s.md` before writing or reviewing a matching file, and follow it. "
            "It is the same file the harness injects into its workers, so a session and a worker hold the same rules.\n"
        ) % (name, title, paths, title, name)
        _write(target, os.path.join("skills", "standards-" + name, "SKILL.md"), text, written)


def policy() -> Dict[str, Any]:
    """The settings policy with skillOverrides derived from the rendered skill set."""
    data = json.loads(_read(POLICY))
    if host_guard():
        for entry in (data.get("hooks") or {}).get("PreToolUse", []):
            for hook in entry.get("hooks", []):
                if str(hook.get("command", "")).endswith("guard.py"):
                    hook["command"] = "~/.claude/hooks/" + GUARD_BINARY
    overrides = {name: "name-only" for name in list(PROCEDURE_SKILLS) + ["review"]}
    overrides.update({"standards-" + name: "name-only" for name in standards.available() if name in STANDARD_PATHS})
    data["skillOverrides"] = overrides
    return data


def render(target: str, config=None) -> List[str]:
    """Renders the whole adapter into target: rules, agents, skills, hooks, standards, policy, seeds."""
    written: List[str] = []
    _write(target, "AGENTS.md", _read(os.path.join(RULES, "AGENTS.md")), written)
    _write(target, "CLAUDE.md", "@AGENTS.md\n@CLAUDE.local.md\n", written)
    render_agents(target, config, written)
    render_skills(target, written)
    for name in os.listdir(HOOKS):
        if name.endswith(".py"):
            _write(target, os.path.join("hooks", name), _read(os.path.join(HOOKS, name)), written)
    # guard.py always ships as the fallback; the binary only joins it when a committed target matches this machine.
    binary = host_guard()
    if binary:
        _copy(target, os.path.join("hooks", GUARD_BINARY), binary, written)
    for name in standards.available():
        _write(target, os.path.join("standards", name + ".md"), _read(os.path.join(standards.STANDARDS_DIR, name + ".md")), written)
    _write(target, "settings.policy.json", json.dumps(policy(), indent=2) + "\n", written)
    for name in os.listdir(SEEDS):
        _write(target, os.path.join("seeds", name), _read(os.path.join(SEEDS, name)), written)
    return written

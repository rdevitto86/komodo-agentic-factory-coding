"""Chooses and loads the rule files a worker gets, by the files a task touches."""

from __future__ import annotations

import os
from typing import Dict, Iterable, List

STANDARDS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "standards")

BY_EXTENSION: Dict[str, List[str]] = {
    ".go": ["go"],
    ".py": ["python"], ".pyi": ["python"],
    ".ts": ["typescript"], ".js": ["typescript"], ".mjs": ["typescript"], ".cjs": ["typescript"],
    ".tsx": ["typescript", "react", "ui-web"], ".jsx": ["typescript", "react", "ui-web"],
    ".vue": ["typescript", "vue", "ui-web"], ".svelte": ["typescript", "svelte", "ui-web"],
    ".css": ["ui-web"], ".html": ["ui-web"],
    ".sh": ["shell"], ".bash": ["shell"],
    ".sql": ["database"],
    ".tf": ["aws"],
    ".rs": ["rust"], ".zig": ["zig"], ".zon": ["zig"],
    ".swift": ["swift", "ui-mobile"], ".kt": ["kotlin", "ui-mobile"], ".kts": ["kotlin"],
    ".m": ["ui-mobile"], ".mm": ["ui-mobile"], ".storyboard": ["ui-mobile"], ".xib": ["ui-mobile"], ".plist": ["ui-mobile"],
    ".c": ["c"], ".h": ["c"], ".cc": ["cpp"], ".cpp": ["cpp"], ".hpp": ["cpp"],
    ".cs": ["csharp", "dotnet"], ".csproj": ["dotnet"], ".sln": ["dotnet"],
    ".java": ["java"], ".gradle": ["java"],
    ".bicep": ["azure"], ".kicad_sch": ["hardware"], ".kicad_pcb": ["hardware"], ".sch": ["hardware"], ".brd": ["hardware"],
    ".desktop": ["ui-desktop"], ".appxmanifest": ["ui-desktop"],
}
BY_PATH_PART: Dict[str, List[str]] = {
    "migrations": ["database"], "schema": ["database"],
    "api": ["api-design", "api-security"], "routes": ["api-design", "api-security"], "handlers": ["api-design", "api-security"], "controllers": ["api-design", "api-security"],
    "auth": ["api-security"], "middleware": ["api-security"], "crypto": ["api-security"],
    "cdk": ["cdk", "aws"], "infra": ["cdk", "aws"], "infrastructure": ["cdk", "aws"],
    ".github": ["cicd"], "logging": ["observability"], "telemetry": ["observability"], "metrics": ["observability"], "tracing": ["observability"],
    "test": ["sdlc"], "tests": ["sdlc"], "__tests__": ["sdlc"], "e2e": ["sdlc"],
}
BY_BASENAME: Dict[str, List[str]] = {
    "Dockerfile": ["docker"], "docker-compose.yml": ["docker"], "docker-compose.yaml": ["docker"],
    "Makefile": ["cicd"], "cicd.yaml": ["cicd"], "SDD.md": ["specs"], "PRD.md": ["specs"],
    "Cargo.toml": ["rust"], "build.zig": ["zig"], "Package.swift": ["swift"], "pom.xml": ["java"], "build.gradle.kts": ["kotlin"],
    "AndroidManifest.xml": ["ui-mobile"], "Info.plist": ["ui-mobile"], "tauri.conf.json": ["ui-desktop"], "electron-builder.yml": ["ui-desktop"],
    "Directory.Build.props": ["dotnet"], "global.json": ["dotnet"], "Program.cs": ["csharp", "dotnet"], "main.bicep": ["azure"],
}
ROLE_EXTRA: Dict[str, List[str]] = {"builder": ["comments"], "reviewer": ["comments", "api-security"], "planner": ["specs"]}

COMMENT_CONVENTION: Dict[str, str] = {
    ".go": "Go: a godoc line starting with the name on every exported identifier; a one-line comment on a private function only when it is long or non-obvious.",
    ".py": "Python: a one-line docstring on every public function, class, and module; private functions get one only when long or non-obvious.",
    ".ts": "TypeScript: a JSDoc line on every export; `//` inside bodies only where a line cannot say it itself.",
    ".tsx": "TypeScript: a JSDoc line on every export; `//` inside bodies only where a line cannot say it itself.",
    ".js": "JavaScript: a JSDoc line on every export; `//` inside bodies only where a line cannot say it itself.",
    ".sh": "Shell: a header block under the shebang; almost nothing below it.",
    ".rs": "Rust: a `///` doc line on every `pub` item; private functions get one only when long or non-obvious.",
    ".java": "Java: a Javadoc line on every public method and type; private ones only when long or non-obvious.",
    ".cs": "C#: an XML doc `<summary>` on every public member; private ones only when long or non-obvious.",
    ".kt": "Kotlin: a KDoc line on every public declaration; private ones only when long or non-obvious.",
    ".swift": "Swift: a `///` doc line on every public declaration; private ones only when long or non-obvious.",
    ".zig": "Zig: a `///` doc comment on every `pub` declaration; private ones only when long or non-obvious.",
    ".c": "C: a comment above every function in a header; static functions get one only when long or non-obvious.",
}


def names_for(paths: Iterable[str], role: str = "builder") -> List[str]:
    """Standard names for a set of paths and a role, in first-seen order, deduplicated."""
    chosen: List[str] = []

    def add(names: List[str]) -> None:
        """Appends names not yet chosen."""
        for name in names:
            if name not in chosen and os.path.isfile(os.path.join(STANDARDS_DIR, name + ".md")):
                chosen.append(name)

    for path in paths:
        normalized = path.replace("\\", "/")
        base = os.path.basename(normalized)
        add(BY_BASENAME.get(base, []))
        if base.endswith(("-stack.ts", ".stack.ts")):
            add(["cdk", "aws"])
        add(BY_EXTENSION.get(os.path.splitext(base)[1].lower(), []))
        for part in normalized.split("/")[:-1]:
            add(BY_PATH_PART.get(part.lower(), []))
    add(ROLE_EXTRA.get(role, []))
    return chosen


def load(names: Iterable[str], cap_chars: int = 6000) -> str:
    """The named standards concatenated, each clipped to cap_chars."""
    from .briefs import clip

    sections = []
    for name in names:
        path = os.path.join(STANDARDS_DIR, name + ".md")
        try:
            with open(path, encoding="utf-8") as handle:
                text = handle.read()
        except OSError:
            continue
        sections.append(clip(text.strip(), cap_chars, name + " standard"))
    return "\n\n".join(sections) if sections else "No language standard applies; follow the repo's existing idioms."


def comment_convention(paths: Iterable[str]) -> str:
    """The one-line comment convention for the dominant language among paths."""
    counts: Dict[str, int] = {}
    for path in paths:
        ext = os.path.splitext(path)[1].lower()
        if ext in COMMENT_CONVENTION:
            counts[ext] = counts.get(ext, 0) + 1
    if not counts:
        return "Every public function gets a one-line doc comment in the language's convention; private ones only when long or non-obvious."
    return COMMENT_CONVENTION[max(counts, key=lambda key: counts[key])]


def available() -> List[str]:
    """Every standard name on disk."""
    return sorted(name[:-3] for name in os.listdir(STANDARDS_DIR) if name.endswith(".md"))

"""Renders role templates under komodo/briefs/ into a system prompt and a task prompt."""

from __future__ import annotations

import os
import re
from typing import Any, Dict, Optional, Tuple

BRIEFS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "briefs")
PLACEHOLDER = re.compile(r"\{\{\s*([a-z_]+)\s*\}\}")

SCHEMAS: Dict[str, Dict[str, Any]] = {
    "builder": {
        "type": "object",
        "properties": {
            "result": {"type": "string", "enum": ["DONE", "BLOCKED"]},
            "summary": {"type": "string"},
            "changed": {"type": "array", "items": {"type": "object", "properties": {"path": {"type": "string"}, "what": {"type": "string"}}, "required": ["path", "what"]}},
            "verified": {"type": "array", "items": {"type": "object", "properties": {"command": {"type": "string"}, "exit_code": {"type": "integer"}}, "required": ["command", "exit_code"]}},
            "notes": {"type": "array", "items": {"type": "string"}},
        },
        "required": ["result", "summary", "changed", "verified"],
    },
    "reviewer": {
        "type": "object",
        "properties": {
            "summary": {"type": "string"},
            "findings": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "severity": {"type": "string", "enum": ["critical", "high", "medium", "low"]},
                        "class": {"type": "string", "enum": ["bug", "security", "simplify", "narrative-comment", "undocumented-nonobvious", "test-gap"]},
                        "file": {"type": "string"},
                        "line": {"type": "integer"},
                        "title": {"type": "string"},
                        "detail": {"type": "string"},
                        "fix": {"type": "string"},
                    },
                    "required": ["severity", "class", "file", "title", "detail"],
                },
            },
        },
        "required": ["summary", "findings"],
    },
    "planner": {
        "type": "object",
        "properties": {
            "tasks": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "title": {"type": "string"},
                        "priority": {"type": "string", "enum": ["C", "H", "M", "L"]},
                        "type": {"type": "string"},
                        "files": {"type": "array", "items": {"type": "string"}},
                        "done_when": {"type": "array", "items": {"type": "string"}},
                        "depends_on": {"type": "array", "items": {"type": "integer"}},
                        "context": {"type": "array", "items": {"type": "string"}},
                    },
                    "required": ["title", "priority", "files", "done_when"],
                },
            },
            "gaps": {"type": "array", "items": {"type": "string"}},
        },
        "required": ["tasks"],
    },
    "merger": {
        "type": "object",
        "properties": {
            "resolved": {"type": "array", "items": {"type": "string"}},
            "escalate": {"type": "array", "items": {"type": "object", "properties": {"file": {"type": "string"}, "reason": {"type": "string"}}, "required": ["file", "reason"]}},
        },
        "required": ["resolved", "escalate"],
    },
    "responder": {
        "type": "object",
        "properties": {
            "reply": {"type": "string"},
            "changed": {"type": "array", "items": {"type": "object", "properties": {"path": {"type": "string"}, "what": {"type": "string"}}, "required": ["path", "what"]}},
            "result": {"type": "string", "enum": ["CHANGED", "REPLIED", "DECLINED"]},
        },
        "required": ["reply", "changed", "result"],
    },
}

TOOLS: Dict[str, list] = {
    "builder": ["Read", "Edit", "Write", "Bash", "Grep", "Glob"],
    "merger": ["Read", "Edit", "Write", "Bash", "Grep", "Glob"],
    "responder": ["Read", "Edit", "Write", "Bash", "Grep", "Glob"],
    "reviewer": ["Read", "Grep", "Glob"],
    "planner": ["Read", "Grep", "Glob"],
    "summarizer": [],
}


class BriefError(ValueError):
    """A template is missing or a required slot was not supplied."""


def _read(name: str) -> str:
    """Reads one template file."""
    path = os.path.join(BRIEFS_DIR, name)
    try:
        with open(path, encoding="utf-8") as handle:
            return handle.read()
    except OSError:
        raise BriefError("missing brief template %s" % path)


def fill(template: str, slots: Dict[str, Any]) -> str:
    """Substitutes every {{slot}}; an unknown or empty slot raises so nothing ships half-filled."""
    def replace(match: "re.Match[str]") -> str:
        """Looks up one placeholder."""
        key = match.group(1)
        if key not in slots or slots[key] is None:
            raise BriefError("brief slot %r not supplied" % key)
        return str(slots[key])

    return PLACEHOLDER.sub(replace, template)


def render(role: str, slots: Dict[str, Any]) -> Tuple[str, str]:
    """The (system, prompt) pair for a role with its slots filled."""
    system = fill(_read("%s.system.md" % role), slots).strip() + "\n"
    prompt = fill(_read("%s.prompt.md" % role), slots).strip() + "\n"
    return system, prompt


def schema_for(role: str) -> Optional[Dict[str, Any]]:
    """The JSON schema a role's result must satisfy, if it has one."""
    return SCHEMAS.get(role)


def tools_for(role: str) -> list:
    """The tool set a role may use."""
    return list(TOOLS.get(role, []))


def clip(text: str, limit: int, label: str = "") -> str:
    """Truncates text at limit with a visible marker so a worker knows it saw a cut."""
    if len(text) <= limit:
        return text
    head = text[: int(limit * 0.7)]
    tail = text[-int(limit * 0.25):]
    return head + "\n\n[... %s truncated: %d of %d chars shown ...]\n\n" % (label or "content", len(head) + len(tail), len(text)) + tail

"""Role definitions under komodo/roles/: one file per role, the single source for workers and session agents."""

from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from typing import Dict, List, Optional

ROLES_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "roles")
TIERS = ("light", "standard", "heavy")
ACCESS = ("none", "read", "write")
WORKER_HEADING = "## Worker output"
SESSION_HEADING = "## Session output"


class RoleError(ValueError):
    """A role file is missing or malformed."""


@dataclass
class Role:
    """One role: its frontmatter and the body shared by every renderer."""

    name: str
    purpose: str
    tier: str
    access: str
    session: bool
    body: str
    worker_output: str = ""
    session_output: str = ""
    extra: Dict[str, str] = field(default_factory=dict)

    def system_prompt(self) -> str:
        """The worker system prompt: body plus the worker output contract."""
        parts = [self.body.strip()]
        if self.worker_output:
            parts.append("# Output\n" + self.worker_output.strip())
        return "\n\n".join(parts) + "\n"

    def session_body(self) -> str:
        """The interactive agent body: body plus the session output contract."""
        parts = [self.body.strip()]
        if self.session_output:
            parts.append(self.session_output.strip())
        return "\n\n".join(parts) + "\n"


def _split_sections(text: str):
    """Body, worker output, and session output from the markdown after the frontmatter."""
    body, worker, session = text, "", ""
    pattern = re.compile(r"^(## (?:Worker|Session) output)\s*$", re.M)
    matches = list(pattern.finditer(text))
    if matches:
        body = text[: matches[0].start()]
    for position, match in enumerate(matches):
        end = matches[position + 1].start() if position + 1 < len(matches) else len(text)
        content = text[match.end():end].strip()
        if match.group(1) == WORKER_HEADING:
            worker = content
        else:
            session = content
    return body.strip(), worker, session


def parse(text: str, name_hint: str = "") -> Role:
    """Parses one role file."""
    if not text.startswith("---"):
        raise RoleError("role %s lacks frontmatter" % name_hint)
    parts = text.split("---", 2)
    if len(parts) < 3:
        raise RoleError("role %s has an unterminated frontmatter" % name_hint)
    front: Dict[str, str] = {}
    for line in parts[1].splitlines():
        if ":" in line and not line.startswith((" ", "\t")):
            key, value = line.split(":", 1)
            front[key.strip()] = value.strip()
    name = front.get("name") or name_hint
    tier = front.get("tier", "standard")
    access = front.get("access", "read")
    if tier not in TIERS:
        raise RoleError("role %s: tier must be one of %s" % (name, "|".join(TIERS)))
    if access not in ACCESS:
        raise RoleError("role %s: access must be one of %s" % (name, "|".join(ACCESS)))
    body, worker, session = _split_sections(parts[2])
    extra = {key: value for key, value in front.items() if key not in ("name", "purpose", "tier", "access", "session")}
    return Role(
        name=name, purpose=front.get("purpose", ""), tier=tier, access=access,
        session=front.get("session", "true").lower() in ("true", "yes", "1"),
        body=body, worker_output=worker, session_output=session, extra=extra,
    )


def load(name: str) -> Role:
    """The role with this name."""
    path = os.path.join(ROLES_DIR, name + ".md")
    try:
        with open(path, encoding="utf-8") as handle:
            return parse(handle.read(), name)
    except OSError:
        raise RoleError("no role file for %r at %s" % (name, path))


def available() -> List[str]:
    """Every role name on disk."""
    return sorted(name[:-3] for name in os.listdir(ROLES_DIR) if name.endswith(".md"))


def all_roles() -> Dict[str, Role]:
    """Every role, keyed by name."""
    return {name: load(name) for name in available()}


def tier_of(name: str, default: str = "standard") -> str:
    """A role's tier, or the default when the role file is absent."""
    try:
        return load(name).tier
    except RoleError:
        return default

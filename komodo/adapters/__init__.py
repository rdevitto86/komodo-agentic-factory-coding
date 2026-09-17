"""Platform adapters: each renders the global rules, roles, and standards into one tool's config layout."""

from __future__ import annotations

from typing import Dict, List

ADAPTERS = ("claude",)


def render(name: str, target: str, config=None) -> List[str]:
    """Renders the named adapter into target and returns the relative paths it wrote."""
    if name == "claude":
        from .claude import render as render_claude

        return render_claude(target, config)
    raise ValueError("unknown adapter %r; known: %s" % (name, ", ".join(ADAPTERS)))

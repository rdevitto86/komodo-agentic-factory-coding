"""komodo.json (team) merged with .komodo/local.json (personal): profiles, protections, budgets."""

from __future__ import annotations

import copy
import fnmatch
import json
import os
from typing import Any, Dict, List, Optional

TEAM_FILE = "komodo.json"
LOCAL_FILE = os.path.join(".komodo", "local.json")

DEFAULTS: Dict[str, Any] = {
    "profile": "fast",
    "profiles": {
        "fast": {
            "planner": {"provider": "claude", "model": "sonnet", "effort": "medium", "max_budget_usd": 1.0, "max_turns": 30},
            "builder": {"provider": "claude", "model": "sonnet", "effort": "medium", "max_budget_usd": 2.0, "max_turns": 60},
            "reviewer": {"provider": "claude", "model": "sonnet", "effort": "medium", "max_budget_usd": 1.0, "max_turns": 20, "min_diff_lines": 150},
            "summarizer": {"provider": "ollama", "model": "qwen3:1.7B"},
        },
        "thinking": {
            "planner": {"provider": "claude", "model": "opus", "effort": "high", "max_budget_usd": 3.0, "max_turns": 40},
            "builder": {"provider": "claude", "model": "sonnet", "effort": "high", "max_budget_usd": 4.0, "max_turns": 100},
            "reviewer": {"provider": "claude", "model": "opus", "effort": "high", "max_budget_usd": 3.0, "max_turns": 30, "min_diff_lines": 0},
            "summarizer": {"provider": "ollama", "model": "qwen3:1.7B"},
        },
    },
    "protected": ["main", "master", "trunk", "prod", "production", "release/*", "hotfix/*"],
    "remote": "origin",
    "base": "",
    "severity_floor": "high",
    "worker_timeout_s": 900,
    "group_budget_s": 3600,
    "max_parallel": 3,
    "context": {"file_chars": 24000, "standards_chars": 6000, "diff_chars": 80000},
    "comments": {"trivial_lines": 8, "require": "nonobvious"},
    "ollama": {"url": "http://localhost:11434"},
    "labels": {"feat": "enhancement", "fix": "bug", "docs": "documentation", "chore": "enhancement", "refactor": "enhancement", "perf": "enhancement", "build": "enhancement", "ci": "enhancement", "test": "enhancement", "agent": "@agent"},
    "changelog": "CHANGELOG.md",
}

ROLES = ("planner", "builder", "reviewer", "summarizer")
PROVIDERS = ("claude", "ollama")


class ConfigError(ValueError):
    """Raised when a config file is unreadable or names an unknown provider."""


def _deep_merge(base: Dict[str, Any], overlay: Dict[str, Any]) -> Dict[str, Any]:
    """Returns base with overlay applied, recursing into nested dicts."""
    result = copy.deepcopy(base)
    for key, value in overlay.items():
        if isinstance(value, dict) and isinstance(result.get(key), dict):
            result[key] = _deep_merge(result[key], value)
        else:
            result[key] = copy.deepcopy(value)
    return result


def _read_json(path: str) -> Dict[str, Any]:
    """Loads a JSON object from path, raising ConfigError with the path on failure."""
    try:
        with open(path, encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, ValueError) as error:
        raise ConfigError("%s: %s" % (path, error))
    if not isinstance(data, dict):
        raise ConfigError("%s: top level must be an object" % path)
    return data


class Config:
    """Merged settings for one repo, with typed accessors the pipeline uses."""

    def __init__(self, data: Dict[str, Any], root: str):
        self.data = data
        self.root = root

    @classmethod
    def load(cls, root: str, overrides: Optional[Dict[str, Any]] = None) -> "Config":
        """Defaults, then komodo.json, then .komodo/local.json, then explicit overrides."""
        data = copy.deepcopy(DEFAULTS)
        for relative in (TEAM_FILE, LOCAL_FILE):
            path = os.path.join(root, relative)
            if os.path.isfile(path):
                data = _deep_merge(data, _read_json(path))
        if overrides:
            data = _deep_merge(data, overrides)
        config = cls(data, root)
        config.validate()
        return config

    def validate(self) -> None:
        """Rejects an unknown provider or a profile missing a role."""
        for name, roles in self.data.get("profiles", {}).items():
            for role in ROLES:
                spec = roles.get(role)
                if not isinstance(spec, dict):
                    raise ConfigError("profile %r lacks role %r" % (name, role))
                if spec.get("provider") not in PROVIDERS:
                    raise ConfigError("profile %r role %r: provider must be one of %s" % (name, role, "|".join(PROVIDERS)))
        if self.data.get("profile") not in self.data.get("profiles", {}):
            raise ConfigError("profile %r is not defined" % self.data.get("profile"))

    def get(self, key: str, default: Any = None) -> Any:
        """Dotted lookup: context.file_chars."""
        node: Any = self.data
        for part in key.split("."):
            if not isinstance(node, dict) or part not in node:
                return default
            node = node[part]
        return node

    def role(self, role: str, profile: Optional[str] = None) -> Dict[str, Any]:
        """The provider/model/effort/budget spec for a role under the active or named profile."""
        name = profile or self.data["profile"]
        try:
            return dict(self.data["profiles"][name][role])
        except KeyError:
            raise ConfigError("profile %r has no role %r" % (name, role))

    @property
    def protected(self) -> List[str]:
        """Branch patterns nothing may commit to, push to, or land into."""
        return list(self.data.get("protected", []))

    def is_protected(self, branch: str) -> bool:
        """Whether a branch name matches any protected pattern."""
        name = branch.strip()
        if name.startswith("refs/heads/"):
            name = name[len("refs/heads/"):]
        return any(fnmatch.fnmatchcase(name, pattern) for pattern in self.protected)

    @property
    def remote(self) -> str:
        """The remote the orchestrator pushes to."""
        return str(self.data.get("remote") or "origin")

    def local_path(self) -> str:
        """Where personal overrides live for this repo."""
        return os.path.join(self.root, LOCAL_FILE)

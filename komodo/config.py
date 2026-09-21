"""Built-in defaults, optionally overlaid by the toolkit's own .komodo/config.json: profiles map tiers to providers, roles declare tiers."""

from __future__ import annotations

import copy
import fnmatch
import json
import os
from typing import Any, Dict, List, Optional

from . import account as account_probe
from . import roles as role_defs

# The toolkit owns and gitignores this path; no repo is required to carry a config file.
LOCAL_FILE = os.path.join(".komodo", "config.json")
TIERS = ("light", "standard", "heavy")
PROVIDERS = ("claude", "ollama")

DEFAULTS: Dict[str, Any] = {
    "profile": "fast",
    "profiles": {
        "fast": {
            "tiers": {
                "light": {"provider": "claude", "model": "haiku", "effort": "low", "max_budget_usd": 0.5},
                "standard": {"provider": "claude", "model": "sonnet", "effort": "medium", "max_budget_usd": 2.0},
                "heavy": {"provider": "claude", "model": "sonnet", "effort": "medium", "max_budget_usd": 1.0},
            },
            "roles": {
                "reviewer": {"min_diff_lines": 150},
                "summarizer": {"provider": "ollama", "model": "qwen3:1.7B"},
            },
        },
        "thinking": {
            "tiers": {
                "light": {"provider": "claude", "model": "haiku", "effort": "low", "max_budget_usd": 0.5},
                "standard": {"provider": "claude", "model": "sonnet", "effort": "high", "max_budget_usd": 4.0},
                "heavy": {"provider": "claude", "model": "opus", "effort": "high", "max_budget_usd": 3.0},
            },
            "roles": {
                "reviewer": {"min_diff_lines": 0},
                "summarizer": {"provider": "ollama", "model": "qwen3:1.7B"},
            },
        },
        "local": {
            "tiers": {
                "light": {"provider": "ollama", "model": "qwen3:1.7B"},
                "standard": {"provider": "ollama", "model": "qwen3-coder-next:latest"},
                "heavy": {"provider": "ollama", "model": "qwen3-coder-next:latest"},
            },
            "roles": {"reviewer": {"min_diff_lines": 0}},
        },
    },
    "protected": ["main", "master", "trunk", "prod", "production", "release/*", "hotfix/*"],
    "remote": "origin",
    "base": "",
    "severity_floor": "high",
    "worker_timeout_s": 900,
    "worker_timeout_max_s": 3600,
    "group_budget_s": 3600,
    "account": {"detect": True, "plan": "", "model_ceiling": True},
    "rate_limit": {"pause_at": 0.95, "warn_at": 0.8, "wait": False},
    "max_parallel": 3,
    "context": {"file_chars": 120000, "per_file_chars": 10000, "standards_chars": 6000, "diff_chars": 80000},
    "comments": {"trivial_lines": 8, "require": "nonobvious"},
    "ollama": {"url": "http://localhost:11434"},
    "labels": {"feat": "enhancement", "fix": "bug", "docs": "documentation", "chore": "enhancement", "refactor": "enhancement", "perf": "enhancement", "build": "enhancement", "ci": "enhancement", "test": "enhancement", "agent": "@agent"},
    "changelog": "CHANGELOG.md",
}


class ConfigError(ValueError):
    """Raised when a config file is unreadable or a profile is malformed."""


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
        self._account: Optional["account_probe.Account"] = None

    @classmethod
    def load(cls, root: str, overrides: Optional[Dict[str, Any]] = None) -> "Config":
        """Defaults, then .komodo/config.json when one exists, then explicit overrides."""
        data = copy.deepcopy(DEFAULTS)
        path = os.path.join(root, LOCAL_FILE)
        if os.path.isfile(path):
            data = _deep_merge(data, _read_json(path))
        if overrides:
            data = _deep_merge(data, overrides)
        config = cls(data, root)
        config.validate()
        return config

    def validate(self) -> None:
        """Rejects a profile missing a tier, an unknown provider, or an undefined active profile."""
        for name, profile in self.data.get("profiles", {}).items():
            tiers = profile.get("tiers") if isinstance(profile, dict) else None
            if not isinstance(tiers, dict):
                raise ConfigError("profile %r has no tiers" % name)
            for tier in TIERS:
                spec = tiers.get(tier)
                if not isinstance(spec, dict) or spec.get("provider") not in PROVIDERS:
                    raise ConfigError("profile %r tier %r: provider must be one of %s" % (name, tier, "|".join(PROVIDERS)))
            for role, spec in (profile.get("roles") or {}).items():
                if "provider" in spec and spec["provider"] not in PROVIDERS:
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

    def profile_names(self) -> List[str]:
        """Every defined profile."""
        return sorted(self.data.get("profiles", {}))

    def tier(self, tier: str, profile: Optional[str] = None) -> Dict[str, Any]:
        """The provider spec for a tier under the active or named profile."""
        name = profile or self.data["profile"]
        try:
            return dict(self.data["profiles"][name]["tiers"][tier])
        except KeyError:
            raise ConfigError("profile %r has no tier %r" % (name, tier))

    def role(self, role: str, profile: Optional[str] = None) -> Dict[str, Any]:
        """A role's spec: its tier's provider spec, any per-role override, then the caps the account allows."""
        name = profile or self.data["profile"]
        tier = role_defs.tier_of(role)
        spec = self.tier(tier, name)
        override = ((self.data["profiles"].get(name) or {}).get("roles") or {}).get(role) or {}
        spec.update(override)
        ceiling = bool((self.data.get("account") or {}).get("model_ceiling", True))
        return account_probe.apply_to_spec(spec, tier, self.account, model_ceiling=ceiling)

    @property
    def account(self) -> "account_probe.Account":
        """The detected Claude account, probed at most once and overridable for an offline or pinned run."""
        if self._account is None:
            settings = self.data.get("account") or {}
            forced = str(settings.get("plan") or "").strip().lower()
            if forced:
                self._account = account_probe.pinned(forced)
            elif settings.get("detect", True) is False:
                self._account = account_probe.Account()
            else:
                self._account = account_probe.detect()
        return self._account

    def worker_timeout(self, task_bytes: int = 0) -> int:
        """The wall-clock a worker gets, scaled by the bytes its task names and the plan's headroom."""
        return account_probe.timeout_for(
            int(self.get("worker_timeout_s", 900)), self.account.plan, task_bytes,
            int(self.get("worker_timeout_max_s", 3600)),
        )

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

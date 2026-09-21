"""What the logged-in Claude account meters, read once from `claude auth status`, and the caps that follow."""

from __future__ import annotations

import json
import shutil
import subprocess
from dataclasses import dataclass
from typing import Any, Dict, Optional

# Only these four fields are read; the command also returns an email and an org id that never leave it.
STATUS_FIELDS = ("loggedIn", "authMethod", "apiProvider", "subscriptionType")
# Read out of the installed binary: a subscription reports exactly one of these names.
PLANS = ("pro", "max", "team", "enterprise")
# Providers that bill a card, where a dollar ceiling is a real ceiling.
DOLLAR_PROVIDERS = ("bedrock", "vertex", "gateway")
TOKENS, DOLLARS, UNKNOWN = "tokens", "dollars", "unknown"

# Input tokens one standard-tier worker call may re-send across all its turns, per plan.
PLAN_INPUT_BUDGET: Dict[str, int] = {
    "pro": 4_000_000,
    "team": 8_000_000,
    "max": 12_000_000,
    "enterprise": 16_000_000,
    UNKNOWN: 4_000_000,
}
# Share of that budget a tier gets: a scout reads, a builder writes, a reviewer re-reads one diff.
TIER_SHARE = {"light": 0.06, "standard": 1.0, "heavy": 0.45}
# Measured on 30 worker calls: builder input tokens track 1400 x turns squared, within +/-20%.
TURN_COST_TOKENS = 1400
# Model families in ascending cost; a spec naming anything else is left alone.
MODEL_RANK = {"haiku": 1, "sonnet": 2, "opus": 3}
# The most expensive family a plan may reach, so a Pro account never spends its allowance on Opus.
PLAN_MODEL_CEILING = {"pro": "sonnet", "team": "sonnet", "max": "opus", "enterprise": "opus", UNKNOWN: "sonnet"}
# Wall-clock headroom per plan, applied on top of the task's own size.
PLAN_TIME_FACTOR = {"pro": 1.0, "team": 1.3, "max": 1.6, "enterprise": 1.8, UNKNOWN: 1.0}
# Task bytes at which a worker is given double the base timeout.
TIMEOUT_BYTES_SCALE = 120_000
TIMEOUT_MAX_FACTOR = 4.0


@dataclass
class Account:
    """The account behind the `claude` binary, reduced to what limits depend on."""

    logged_in: bool = False
    auth_method: str = ""
    api_provider: str = ""
    plan: str = UNKNOWN
    detected: bool = False
    pinned: bool = False

    @property
    def metered(self) -> str:
        """Whether spend is counted in tokens against an allowance, in dollars, or cannot be told."""
        if not self.detected or not self.logged_in:
            return UNKNOWN
        if self.auth_method == "claude.ai":
            return TOKENS
        if self.auth_method == "apiKey" or self.api_provider in DOLLAR_PROVIDERS:
            return DOLLARS
        return UNKNOWN

    @property
    def caps_dollars(self) -> bool:
        """Whether a dollar ceiling should reach the worker; unknown keeps it, so detection can only relax."""
        return self.metered != TOKENS

    def describe(self) -> str:
        """One line for `komodo status`, carrying no identifying field."""
        if not self.detected:
            return "unknown (claude auth status unavailable)"
        if not self.logged_in:
            return "not logged in"
        plan = self.plan if self.plan != UNKNOWN else "no subscription"
        source = " (pinned)" if self.pinned else ""
        return "%s%s via %s/%s, metered in %s" % (plan, source, self.auth_method or "?", self.api_provider or "?", self.metered)


_CACHE: Dict[str, Account] = {}


def detect(binary: str = "claude", timeout_s: int = 20, refresh: bool = False) -> Account:
    """Runs `claude auth status` once per process and caches it; any failure yields an undetected account."""
    if not refresh and binary in _CACHE:
        return _CACHE[binary]
    account = _detect_uncached(binary, timeout_s)
    _CACHE[binary] = account
    return account


def _detect_uncached(binary: str, timeout_s: int) -> Account:
    """The probe itself, isolated so a cached call never shells out."""
    if shutil.which(binary) is None:
        return Account()
    try:
        completed = subprocess.run([binary, "auth", "status"], capture_output=True, text=True, timeout=timeout_s)
    except (OSError, subprocess.SubprocessError):
        return Account()
    if completed.returncode != 0:
        return Account()
    return parse_status(completed.stdout)


def parse_status(stdout: str) -> Account:
    """Reads the four fields limits depend on out of the status JSON, ignoring the rest."""
    try:
        data = json.loads(stdout)
    except ValueError:
        return Account()
    if not isinstance(data, dict):
        return Account()
    plan = str(data.get("subscriptionType") or "").strip().lower()
    return Account(
        logged_in=bool(data.get("loggedIn")),
        auth_method=str(data.get("authMethod") or ""),
        api_provider=str(data.get("apiProvider") or ""),
        plan=plan if plan in PLANS else UNKNOWN,
        detected=True,
    )


def pinned(plan: str) -> Account:
    """The account a pinned plan stands for: a subscription on that plan, probe or no probe."""
    name = str(plan or "").strip().lower()
    return Account(logged_in=True, auth_method="claude.ai", api_provider="firstParty",
                   plan=name if name in PLANS else UNKNOWN, detected=True, pinned=True)


def reset_cache() -> None:
    """Drops the memoised probe, so a test or a re-login sees a fresh account."""
    _CACHE.clear()


def input_budget(plan: str, tier: str) -> int:
    """Input tokens a worker of this tier may re-send, from the plan's budget and the tier's share."""
    budget = PLAN_INPUT_BUDGET.get(plan, PLAN_INPUT_BUDGET[UNKNOWN])
    return int(budget * TIER_SHARE.get(tier, TIER_SHARE["standard"]))


def turns_for(plan: str, tier: str) -> int:
    """Turn cap solved from the quadratic law, so headroom is granted in tokens rather than guessed."""
    return max(5, int((input_budget(plan, tier) / float(TURN_COST_TOKENS)) ** 0.5))


def timeout_for(base_s: int, plan: str, task_bytes: int = 0, ceiling_s: int = 0) -> int:
    """Scales the flat timeout by the bytes a task names and the plan's headroom, up to a hard ceiling."""
    size_factor = 1.0 + min(2.0, max(0, task_bytes) / float(TIMEOUT_BYTES_SCALE))
    scaled = base_s * size_factor * PLAN_TIME_FACTOR.get(plan, 1.0)
    limit = ceiling_s if ceiling_s > 0 else int(base_s * TIMEOUT_MAX_FACTOR)
    return max(base_s, min(int(scaled), limit))


def model_rank(model: str) -> int:
    """The cost rank of a model name or full id, or zero when the family cannot be told."""
    name = str(model or "").lower()
    for family, rank in MODEL_RANK.items():
        if family in name:
            return rank
    return 0


def capped_model(model: str, plan: str) -> str:
    """The named model, or the plan's ceiling when the name outranks what the plan should be spending on."""
    ceiling = PLAN_MODEL_CEILING.get(plan, PLAN_MODEL_CEILING[UNKNOWN])
    wanted = model_rank(model)
    if wanted == 0 or wanted <= MODEL_RANK[ceiling]:
        return model
    return ceiling


def apply_to_spec(spec: Dict[str, Any], tier: str, account: Optional[Account] = None, model_ceiling: bool = True) -> Dict[str, Any]:
    """Fills an unset turn cap, caps the model to the plan, and drops the dollar cap when dollars are not metered."""
    resolved = dict(spec)
    account = account or Account()
    if resolved.get("provider") != "claude" or not account.caps_dollars:
        resolved.pop("max_budget_usd", None)
    if resolved.get("max_turns") is None:
        resolved["max_turns"] = turns_for(account.plan, tier)
    if model_ceiling and resolved.get("provider") == "claude" and resolved.get("model"):
        resolved["model"] = capped_model(str(resolved["model"]), account.plan)
    return resolved

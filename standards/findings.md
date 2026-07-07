# Findings

**Scope: every agent, every audit, every code review, every risk callout, every recommendation surfaced to the user or to another agent.** A "finding" is anything you report as a defect, vulnerability, gap, risk, or change recommendation. This rule governs how those findings must be presented.

## Why this rule exists

The user has explicitly flagged that repeated audit runs surface "new" findings each time, and asked how many are real versus speculative. LLM audits have a structural bias toward producing findings — silence reads as failure to the model, and "filter against the existing backlog" prompts compound the bias by pushing agents toward marginal items. Without an enforced confidence signal, the backlog fills with low-confidence noise, the user loses trust in the audit, and real defects get buried alongside theoretical ones.

Confidence scoring and source citations restore the signal-to-noise ratio: the user can scan a finding list and act on what's verified, debate what's interpretive, and discard what's speculative — without re-reading every cited file themselves.

## The rule

Every finding **must** carry three fields. A finding without all three is incomplete and must be rejected by the reviewing agent (or the advisor) before reaching the user.

1. **Confidence (percent, 0–100)** — your honest estimate that the finding is a real defect, not a theoretical or interpretive one. Anchor to the bands below; do not invent precision.
2. **Source** — the evidence that proves the finding. A `file:line` citation, a doc reference, a spec section, or an external authority (RFC, library docs, CVE). One source minimum; cite both sides when reporting contract drift (code says X at `a.go:42`; spec says Y at `openapi.yaml:188`).
3. **Why** — one short sentence stating *why this matters in this codebase* — the concrete consequence if unfixed. Not "best practice"; not "could be a problem." Name the actual blast radius (auth bypass, data loss, p99 regression, contract break for caller X).

## Confidence bands

Use these anchors. The percent is a band, not a guess to two decimal places.

| Band | Meaning | When to use |
|---|---|---|
| **90–100%** | Verified. Reading the cited source proves the finding without interpretation. | Code literally does the wrong thing. Contract literally disagrees with code. |
| **70–89%** | High. Strong evidence; minor inference required (e.g. depends on a runtime path you reasoned about but didn't execute). | Logic bug whose trigger condition you traced but didn't reproduce. |
| **40–69%** | Medium. Interpretive — depends on intent, threat model, or future-state assumption. | "Fail-open on Redis error" when the team may have made an availability call. Weak hash when secret entropy is unknown. |
| **<40%** | Speculative. "Could be a problem if…" | Do not file. Raise as a question instead, or drop it. |

If your confidence is below 40%, do not report the finding as a finding. Convert it to a clarifying question to the user or the owning agent, or discard.

## Format

Use this shape for every finding, in tables or prose:

> **[P0 · 95%]** Introspect leaks user UUID as `client_id` for user tokens.
> **Source:** `internal/api/oauth_introspect.go:80`; RFC 7662 §2.2.
> **Why:** Consumers reading `client_id` for audit/attribution receive the wrong identifier — silently corrupts downstream audit trails.
> **Fix:** Use `claims.Azp` when set, else omit.

In a table, the same fields become columns: `Severity | Confidence | Finding | Source | Why | Fix`. Confidence is mandatory in either form.

## Hard violations (reject on sight)

- Any finding without an explicit confidence percent.
- Any finding without a verifiable source (no `file:line`, no doc/spec reference).
- Any finding without a one-sentence "why this matters here" — generic best-practice lectures count as missing the field.
- Inflated confidence ("95%") on a finding whose source citation does not actually prove the claim. The reviewer must read the source before accepting the confidence number.
- Aggregating multiple findings into one bullet to dodge the rule. One finding = one row = one confidence number.
- Bucketing speculative items under "P2 backlog" to ship them without a confidence number. Speculative items are questions, not findings.

## Reviewer responsibility

The cross-review gate (`AGENTS.md` § Cross-review gate) applies: a different agent must verify each finding's source citation matches the claim before it reaches the user. The reviewer downgrades any inflated confidence, rejects any missing field, and converts any sub-40% item to a question. The advisor owns final reconciliation when an audit batch is being surfaced.

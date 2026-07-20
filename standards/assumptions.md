# Assumptions & Escalation

**Scope: every agent, every task.** Governs what an agent does at the moment it hits a conflict, a missing capability, or a gap between what the task needs and what the agent currently knows.

## Why this rule exists

In the Accounts API, `software-engineer` needed `BatchGetItem` and `TransactWriteItems` against the same table. It assumed — without checking — that one DynamoDB client couldn't issue both, and stood up two clients against the same table to route around a limitation that didn't exist. Nobody asked; nobody verified; the workaround shipped as if it were the only option. That is a unilateral judgment call built on an unverified assumption, and the architecture it produced is worse than the one that was actually available. This standard closes that failure mode for every agent, not just `software-engineer`.

## The rule

**1. Verify before concluding a limitation exists.**
Before telling yourself or anyone else "X isn't supported," "the SDK doesn't do Y," or "there's no way to Z," check the primary source — the actual SDK/library source or its official docs for the version in use. Recalling a library's capabilities from training data is not verification; SDKs change and your prior knowledge can be stale or flat wrong. Cite what you checked (file, doc, version) the same way `findings.md` requires a source for a finding.

**2. Don't resolve a conflict by judgment call unless you actually have what you need.**
"Having what you need" means a verified fact, an explicit instruction already given, or a pattern already established in the codebase — not a plausible-sounding guess. If none of those are available, stop. Do not pick a workaround, do not take the "safer-seeming" path and mention it afterward in the summary. Escalate *before* implementing, not after. This applies with extra force when the workaround itself violates other doctrine (e.g., two clients against one store where `principles.md` §3 calls for a single injected dependency) — a bad workaround chosen to route around a false assumption compounds the mistake instead of fixing it.

**3. Escalation path: agent → advisor → user — and it must actually reach the user.**
A spawned agent takes a genuine capability or requirement question to the advisor. The advisor does not resolve it with its own guess unless it can point to verified information of its own; otherwise it forwards the question to the user. This does not relax the advisor's job of absorbing pure operational noise (retries, transient failures, ambiguity it can resolve from context it already has) — it only says a genuine information gap is never quietly filled in.

**4. SDK/library gaps get flagged, not routed around.**
`principles.md` §2 already requires flagging SDK gaps upstream — this extends it: a claimed gap must be verified (rule 1) before it's flagged, and once a real gap is confirmed, present actual options instead of silently implementing one:
- A redesign that stays within the SDK's real capabilities.
- The workaround, with its cost/tradeoff named explicitly, taken only if the user or advisor signs off.
- A proposed addition/patch/wrapper to the SDK itself, when the gap is general enough to be worth fixing upstream (`komodo-forge-sdk-*` or the vendor SDK) rather than worked around locally.

Never ship the workaround presented as if it were the only option.

## Worked example

**Bad:** SWE needs `BatchGetItem` and `TransactWriteItems` against the Accounts table, assumes without checking that one client can't issue both, and stands up two DynamoDB clients against the same table.

**Good:** SWE checks the AWS SDK docs/source, confirms a single `dynamodb.Client` supports both operations, and uses one. If a real gap had turned up instead, SWE stops and hands the advisor/user a redesign-vs-workaround-vs-upstream-fix choice before writing code.

## Hard violations (reject on sight)

- A design decision justified by "the SDK/library doesn't support X" with no cited source for that claim.
- A workaround implemented and only explained afterward, when the option to ask first existed.
- A question that belonged to the user resolved by the advisor, or any agent, without a verified fact behind the resolution.
- A user-directed question buried inside a larger status update instead of surfaced first, on its own (`communication.md`).

## Reviewer responsibility

The reviewing agent (or the advisor, on self-review) checks that any stated capability limitation cites a real source, and that any non-trivial workaround was either pre-approved or flagged as a finding per `findings.md` — never silently shipped.

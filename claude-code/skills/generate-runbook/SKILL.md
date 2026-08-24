---
name: generate-runbook
description: Operational runbook template under docs/runbooks/ — the trigger, the diagnosis steps, the resolution, the escalation path. Load when creating or editing a file under docs/runbooks/.
paths: "**/docs/runbooks/**"
---

# Runbooks

A runbook is the deep operational record for one recurring failure mode or manual procedure — what an on-call engineer follows at 3am without needing to reconstruct the reasoning first. SDD §7 Observability names the signal that pages someone; SDD §10 Recovery names the mechanism and RTO/RPO. A runbook exists only once a signal or a recovery path from those sections needs steps a table row can't hold — it is never a replacement for either.

## The fit test

Write one only when at least one is true:
- **An alert fires and someone has to act** — the SDD §7 signal has a runbook column, or should.
- **A recovery procedure in SDD §10 has more than one step**, needs judgment calls, or depends on tooling/credentials the operator won't have memorized.
- **A manual procedure repeats** — a migration, a key rotation, a failover — often enough that re-deriving it each time is itself the risk.

## File location

`docs/runbooks/<slug>.md` — colocated with `docs/prd.md` and `docs/sdd.md`, never a top-level `/runbooks`. `<slug>` is kebab-case, naming the failure mode or procedure, not the ticket (`db-failover`, not `incident-2026-08`).

Every runbook needs a citing link from SDD §14 References — owned by `generate-sdd`, not restated here.

## Template

```markdown
# Runbook — <Title>

- **Owns:** <the SDD §7 signal or §10 recovery scenario this backs>
- **Severity:** <Sev1-4, or the on-call tier this pages>
- **Last verified:** <YYYY-MM-DD — the date someone last ran this for real or in a drill>

## Trigger
<The alert, symptom, or request that starts this runbook. Concrete: an alarm name, an error signature, a user-visible symptom — not "something is wrong.">

## Diagnosis
<Numbered steps to confirm this is actually the failure mode this runbook covers, not a lookalike. Each step names a command or dashboard, not just an instruction to "check X.">

## Resolution
<Numbered steps to fix it. Call out anything irreversible before the step that does it. State the expected outcome after each step so the operator knows whether to proceed or escalate.>

## Escalation
<Who to page and when — after which failed step, or which severity — and what context to hand them.>

## Related
<Links: the SDD §7/§10 section this backs, related ADRs, related runbooks.>
```

**A runbook is never invented ahead of an incident it hasn't yet needed.** Draft it from an actual signal or recovery scenario already in the SDD, or from a procedure someone has actually run — not from a hypothetical failure mode.

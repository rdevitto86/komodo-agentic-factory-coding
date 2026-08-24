---
name: assess-security
description: Score the current diff's security exposure (Low → Critical) against the OWASP baseline, with a one-paragraph cited rationale.
argument-hint: []
disable-model-invocation: true
---

# Security exposure assessment

**Once per task, before `/workflow-complete` or a push — never per-edit.** Scores exposure the diff introduces, not whether it's already been exploited. Read the diff before scoring; every claim comes from what actually changed. `standards-security` states the OWASP-benchmarked bar this scores against — load it first.

**Not a finder.** `/security-review` walks the attack surface and reports findings; this scores the damage if one slipped through. A diff touching auth, secrets, or an external boundary with zero known findings can still score High.

## The scale

| Tier | Exposure test |
|---|---|
| Low | No touch to auth, secrets, input handling, or an external boundary |
| Low-Med | Touches one of those, but the change is additive and covered by existing controls |
| Med | Modifies an existing control (validation, auth check, encoding) in a contained surface |
| Med-High | Modifies a control on a path that crosses a trust boundary (user input to query, external call to internal service) |
| High | Touches secrets, auth/session handling, or crypto directly; thin test coverage |
| Critical | A control is removed, weakened, or bypassable, on a path reachable by an untrusted caller |

Score the **highest tier any touched file reaches** — one Critical-tier file outweighs nine Low-tier ones.

## Output

```markdown
## Security exposure: <TIER>

<one paragraph, ≤3 sentences, citing file:line and the OWASP category for the driving factor(s)>

**Next:** run /security-review — <one-line reason>
```

- **`file:line` and an OWASP category for every driving factor.** No pointer, no score.
- **Omit `Next` below Med-High.** Nothing to recommend at Low through Med.
- **One paragraph only.** If the reasoning needs more than 3 sentences, the diff needs splitting, not a longer writeup.

## [TG-15.6] The escalation path works end to end [P: C] [BLOCKED]

```yaml
type: fix
depends_on: [TG-15.2]
```

> **Obsolete**, 2026-10-05. Decision 0015 removes the escalation role, state and session this group built toward: a stop now goes straight to the running orchestrator, with no session spawned to decide it. TSK-15.6.1 through TSK-15.6.7 are removed; TSK-15.6.8 stays as the record of what shipped. `TG-15.7` depends on `TG-15.2` directly instead of this group.

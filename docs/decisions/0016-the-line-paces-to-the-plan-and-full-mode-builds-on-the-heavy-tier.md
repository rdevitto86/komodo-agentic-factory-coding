# 0016. The line paces to the plan, and full mode builds on the heavy tier

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0016 and 0031.

**Context.** The owner wants the line tuned to the plan and able to run for hours unattended. The plan probe reads the subscription type and window, and the host's stream reports rate limits with reset times. Standard-tier builds drew more high review findings and repair rounds (TG-06.5, TG-06.2).

**Decision.**

- **Subscriptions are bound:** the conductor sets concurrency from the plan, pauses at a usage limit, and resumes at the reset.
- **A Pro plan runs economy mode:** the economy profile and one review lens, with the builder on the standard tier.
- **Every other plan runs full mode:** the builder, and so every repair, runs the heavy tier at medium effort.
- **API billing is unbound:** a spend budget per run applies.

**Alternatives.**

- **One setting for every plan.** A Pro plan would hit its limits mid-group.

**Consequences.**

- **Economy mode trades some review depth for cost,** and eval measures how much.
- **A full-mode build spends more of the window per session,** traded for fewer repair rounds.

**Open.** The API spend budget isn't built: `internal/preflight/preflight.go` holds a TODO. Build it, or drop it here.

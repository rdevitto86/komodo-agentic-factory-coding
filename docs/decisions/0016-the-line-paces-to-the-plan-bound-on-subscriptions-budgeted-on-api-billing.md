# 0016. The line paces to the plan: bound on subscriptions, budgeted on API billing

**Status:** Accepted, 2026-09-25.

**Context.** The owner wants the line tuned to the plan and able to run for hours unattended. The plan probe reads the subscription type and window, and the host's stream reports rate limits with reset times.

**Decision.** Subscriptions are bound: the conductor sets concurrency from the plan, pauses at a usage limit, and resumes at the reset. A Pro plan runs economy mode: the economy profile and one review lens with its own prompt. API billing is unbound: a spend budget per run applies.

**Alternatives.**

- **One setting for every plan.** A Pro plan would hit its limits mid-group.

**Consequences.**

- **Economy mode trades some review depth for cost,** and eval measures how much.

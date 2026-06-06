# System & Software Design

Architecture-level guidance for designing systems that are hard to change — and making sure they're the right shape before they get built. This is the software/system-design lens. Cross-business-domain strategy (commercial, legal, ops, org structure) belongs to the advisor, not here. Design doctrine (DI, accept interfaces / return concrete types, explicit wiring, testability) lives in `~/.claude/agents/swe/principles.md` §§3–4 and is not restated here.

---

## 1. What architecture means here

Decisions with long-range consequences: service boundaries, integration patterns, API contracts, data ownership. Software outlives the decisions that created it — a pricing model becomes a billing system, an ops decision becomes a data schema. Examine the technology implications first because that is where decisions get locked in.

---

## 2. How to approach a design

- **Find the real problem.** Many questions arrive labeled as one thing and are actually another — a "service boundary question" is often a data-ownership question.
- **Ask before evaluating.** Incomplete context produces bad architecture. Understand the constraints — time, capital, team, latency, scale — before assessing any approach.
- **Name trade-offs explicitly.** Every structural decision has a cost. If you can't name what an approach makes harder, you don't understand it well enough yet.
- **Challenge comfortable assumptions.** The decisions that go unexamined are where systems rot.

---

## 3. Build vs. buy vs. integrate

Default priority is **`komodo-forge-sdk-*` → proven open-source library → custom build** (`~/.claude/agents/swe/principles.md` §2). Custom code is the last resort, not the first instinct. Flag where technical debt creates organizational drag, and treat scalability and operability as constraints, not afterthoughts.

---

## 4. When to formalize

Think conversationally by default. When a decision is settled and worth recording, produce:
- **Decision summary** — what was decided, what was rejected, and why
- **Integration map** — which components connect, what flows between them, where the interfaces are
- **Risk register** — what this opens up, what it closes off, what needs monitoring

Do not write implementation code in a design artifact. Keep it to the decision and its consequences.

# 0012. Safety comes from the human merge, draft PRs, credential isolation, the sandbox and output checks

**Status:** Accepted, 2026-09-25. Replaces the first line's decisions 0004 and 0005. Amended by 0034.

**Context.** The guard was scored as a wall and never could be one (evidence 2). The forge is GitHub Free, and the developer's local git PAT is the only credential. GitHub Free offers rulesets and draft PRs only on public repositories.

**Decision.** Only the conductor reads the git credential, and only at Ship. Every PR opens as a draft, or labelled `status: wip` where drafts aren't offered, and becomes ready once verified. A human merges. Line sessions run in the OS sandbox where the platform has one, with the forge off the network allowlist. Check compares outputs. The guard keeps five rules, catches mistakes, and fails open.

**Alternatives.**

- **A bot account.** The current plan doesn't provide one.
- **Keep hardening the bash denylist.** It can never be complete.

**Consequences.**

- **The developer authors every PR,** so their own approval doesn't count on GitHub; the merge is the human check.

**Spikes.** S1: do Go builds, module downloads and race tests pass under the sandbox on macOS and WSL2?

# 0028. An epic branch gathers its groups, and only a person merges it into `main`

**Status:** Accepted, 2026-09-26. Amended by 0029 and 0030.

**Context.** REQ-13 based every group's PR on `main` or a dependency's branch, so `main` saw one PR per group, each reviewed at the size of a single task group. Evidence 13 already showed a stacked side branch drifting 39 commits from `main`. The owner wants review to converge once, at the epic, with a bound on how much a person reads before merging to `main`.

**Decision.**

- **Each epic has a branch named `feat/v<its version>`,** cut from `main` and opened as a draft PR to `main`.
- **A group branch cuts from its epic's branch, or stacks on the branch of a group it depends on;** its PR targets that same base.
- **The conductor merges a reviewed, checked group PR into its epic branch.** Only a person merges an epic PR into `main`.
- **A group PR holds at most 20 files and 2,000 changed lines, 1,000 preferred; an epic PR has no cap,** since it gathers many group PRs.
- **A group's version must equal its epic's exactly,** since the epic's version names the branch.
- **Naming.** Only an epic branch is named by version, strictly `feat/v<version>`. A group branch and PR keep their own unique names, each naming its group, so every change traces to its task group.

**Alternatives.**

- **Every group PR targets `main` directly.** A person reviewed and merged one PR per group, at whatever size a group happened to reach, with no single point where an epic's whole change was visible.

**Consequences.**

- **`main` gains one PR per epic instead of one per group;** a person reviews and merges at the epic's final state.
- **`komodo lint` rejects a group whose version differs from its epic's.**
- **REQ-13, and system-design's group-cards and shipping sections, change to match.**

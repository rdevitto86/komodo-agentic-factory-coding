# 0028. An epic branch gathers its groups, each PR is capped and labelled, and only a person merges to `main`

**Status:** Accepted, 2026-09-26. Consolidated 2026-10-02 from 0028, 0029's branch name, 0030 and 0036.

**Context.** Basing every group's PR on `main` meant one review per group, at whatever size a group reached, with no single point where an epic's whole change was visible. Evidence 13 showed a stacked side branch drifting 39 commits. A cap on every changed line refused mostly-deletion changes. Sessions that ran `gh pr create` themselves left 17 of 63 PRs unlabelled across four repos.

**Decision.**

- **Each epic has a branch named `feat/<its version>`,** cut from `main` and opened as a draft PR to `main`. A group's version must equal its epic's.
- **A group branch cuts from its epic's branch, or stacks on a group it depends on,** and its PR targets that base. The conductor merges a reviewed, checked group PR into the epic branch; only a person merges an epic PR into `main`.
- **A group PR keeps at most 20 files and adds at most 2,000 lines, 1,000 preferred.** Deleted files and lines are free; the ledger still records every changed line. An epic PR has no cap.
- **A PR opens through `komodo pr create`,** which refuses a title that isn't `<type>: <summary>` within 72 characters. It applies `@agent`, the scope the branch's files earn, the stage from the newest changelog version, and `branch/feature` off the default branch. A repo maps its scopes in `.komodo/labels.json`.
- **The guard refuses a model's `gh pr create`;** `komodo pr label` labels a PR opened another way.

**Alternatives.**

- **Every group PR targets `main`.** One review per group, with no view of the whole epic.
- **Let a group declare its own cap.** A builder could raise the cap it's judged by.
- **A rule in `AGENTS.md` alone, or a forge action for labels.** The rule was already skipped most of the time, and nothing runs on the forge (0018).

**Consequences.**

- **`main` gains one PR per epic,** reviewed at the epic's final state.
- **A mass deletion ships as one PR;** review still reads its file list.
- **A product repo without `.komodo/labels.json` earns the toolkit's scope labels,** and ship and `komodo pr` warn when the repo lacks one.

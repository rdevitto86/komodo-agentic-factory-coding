# 0029. The phases are alpha, beta, rc and stable; an epic branch is `feat/<version>`

**Status:** Accepted, 2026-09-26. Amends 0023 and 0028.

**Context.** TSK-05.8.2 dropped the leading `v` from an epic branch's version, so `feat/v<version>` in decision 0028 no longer matches the code. Decision 0023 named only alpha, beta and an LTS release, with no place for a release candidate a team may want between them.

**Decision.**

- **The phases are alpha, beta, rc and stable.** Rc is optional and reserved: a release may go straight from beta to stable, such as `1.0.0-beta.2` to `1.0.0`. Nothing requires, gates, or checks for an rc; V1 takes the beta-to-stable path.
- **An epic branch is named `feat/<its version>`,** with no `v`, matching the version string a group's `version:` field carries.

**Alternatives.**

- **Require an rc before every stable release.** Adds a gate no V1 release needs.
- **Keep `feat/v<version>`.** Leaves the branch name out of step with TSK-05.8.2's code.

**Consequences.**

- **README's Versions section gains an rc bullet, marked optional, and renames Release to Stable.**
- **`komodo/rules/backlog.md`, the backlog skill, and the SDLC standard give the four phases and branch examples with no `v`.**
- **The template's example group carries a phase version.**

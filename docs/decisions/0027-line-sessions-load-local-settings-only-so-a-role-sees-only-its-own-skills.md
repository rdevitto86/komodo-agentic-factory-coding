# 0027. Line sessions load local settings only, so a role sees only its own skills

**Status:** Accepted, 2026-09-26. Amends 0025.

**Context.** Decision 0025 loaded `--setting-sources project,local`. TG-05.3's review found that the project source also loads every skill the install renders into the shared project directory, so a builder saw review, run and orchestrator skills. A test on CLI 2.1.283 confirmed it: with `project,local` a project skill loaded; with `local` it did not, and the role plugin's skill loaded either way.

**Decision.** A line session passes `--setting-sources local`. The shared project skills stay for the primary session, the orchestrator. A line session's rules come from its brief, whose second slot carries the repo's `AGENTS.md`.

**Alternatives.**

- **Render no skills into the project directory.** The primary session would lose the orchestrator's skills.
- **`--disable-slash-commands`.** It also drops the role plugin's skills (spike S3).

**Consequences.**

- **A builder sees only its plugin's skills and the host's built-ins.**
- **The worktree's `CLAUDE.md` no longer loads in a line session;** the brief is the only source of repo rules.

# 0014. Each role's allow list covers its stage, and the conductor does git housekeeping

**Status:** Accepted, 2026-09-25.

**Context.** Agents were refused commands their work needed, such as deleting files or switching to `main` to sync.

**Decision.** Each role gets the allow and deny lists in `lld.md#permissions`, applied with the host's `dontAsk` mode. Builders may delete and move files in their worktree. Switching branches, syncing, committing and cleanup belong to the conductor. The critical-ref rule refuses writes to `main`, not switching to it or fast-forwarding it.

**Alternatives.**

- **Bypass permissions, with the guard as the only wall.** Evidence 2.

**Consequences.**

- **Golden runs should record no refusal of an allowed command.**

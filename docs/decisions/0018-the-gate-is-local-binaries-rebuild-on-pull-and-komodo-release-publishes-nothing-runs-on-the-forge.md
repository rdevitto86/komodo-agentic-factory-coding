# 0018. The gate is local, binaries rebuild on pull, and `komodo release` publishes; nothing runs on the forge

**Status:** Accepted, 2026-09-25. Restates and extends the first line's decision 0006.

**Context.** Hosted CI costs minutes and moves failure away from the person who can fix it. `bin/` is gitignored and nothing rebuilt it after a pull, so the owner ran commands by hand to get the latest binaries.

**Decision.** `komodo gate` runs before every commit and push, with no model. The gate also installs post-merge, post-checkout and post-rewrite hooks that rebuild the binary when Go sources changed. `komodo release` cross-compiles every platform, runs the tests, writes checksums and publishes a GitHub Release from the owner's machine. Product repos use a published release.

**Alternatives.**

- **Commit the binaries.** About 9 MB per platform per change would stay in git history forever.
- **CI on the forge.** The owner prefers none; eval on each platform proves cross-platform behaviour.

**Consequences.**

- **Nobody runs a command to get the latest binary.**
- **A hook can be skipped with `--no-verify`;** the human merge and draft-first PRs are the backstop.

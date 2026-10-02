# 0018. Proof is local: the gate runs before every commit, `komodo release` publishes, and eval measures readiness

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0018 and 0021.

**Context.** Hosted CI costs minutes and moves failure away from the person who can fix it. `bin/` is gitignored and nothing rebuilt it after a pull. A self-graded score defined "done" (evidence 4), and green could mean nothing ran (evidence 15).

**Decision.**

- **`komodo gate` runs before every commit and push, with no model,** and fails loudly when it finds no build check.
- **The gate installs post-merge, post-checkout and post-rewrite hooks** that rebuild the binary when Go sources changed.
- **`komodo release` cross-compiles every platform, runs the tests, writes checksums and publishes a GitHub Release** from the owner's machine. Product repos use a published release. Nothing runs on the forge.
- **`komodo eval` runs golden task groups in real repos, with hidden tests,** against the PRD's success criteria. A local model is optional and never a proof.

**Alternatives.**

- **Commit the binaries.** About 9 MB per platform per change would stay in git history forever.
- **CI on the forge.** The owner prefers none; eval on each platform proves cross-platform behaviour.
- **A rubric a model scores.** It swung between 72 and 88 on the same code.

**Consequences.**

- **Nobody runs a command to get the latest binary,** and only a proof moves readiness.
- **A hook can be skipped with `--no-verify`;** the human merge and draft-first PRs are the backstop.

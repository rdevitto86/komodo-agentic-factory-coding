# 0033. The host CLI is not pinned

**Status:** Accepted, 2026-09-28. Amends 0006.

**Context.** Decision 0006 pinned the host CLI to one release. The pin never chose the CLI that ran; it only failed doctor and the conductor's preflight when the installed one differed. The host ships a release most days, so every update stopped the gate and the line until someone bumped the pin by hand.

**Decision.**

- **No host CLI version is pinned or checked.** Preflight still checks that the CLI runs and holds a login.
- **Each line session keeps the host's autoupdater off,** so the CLI never changes mid-run.
- **The rest of 0006 holds:** full model IDs, the `komodo` release, the toolchains and LF line endings stay pinned.

**Consequences.**

- **A host update no longer stops the gate or the line.**
- **A host change that breaks the line shows up as a failed session,** not a doctor finding; the host's own `--version` names the release when it matters.

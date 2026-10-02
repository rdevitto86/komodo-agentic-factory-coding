# 0023. Versions go alpha, beta, an optional rc, and stable

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0023, 0024 and 0029's phases.

**Context.** The owner wanted a fresh start for V1. Tags `v1.0.0-alpha.1` to `.4` exist from the prototype, and `1.0.0-beta.1` was a changelog heading that was never tagged; SemVer sorts it above V1's alphas.

**Decision.**

- **The phases are alpha, beta, rc and stable.** Rc is optional: nothing requires, gates or checks it, and V1 goes straight from beta to stable.
- **V1 ran `1.0.0-alpha.5` onward, then betas from `1.0.0-beta.2`,** feature-complete with only fixes landing while eval runs on every platform. `1.0.0` is cut by the owner once the PRD's success criteria hold.
- **The untagged `1.0.0-beta.1` heading is the titled history section "The first 1.0.0 line",** so the changelog stays in SemVer order and `beta.1` is never used.

**Alternatives.**

- **Start a new series such as `1.1.0-alpha.1`.** It implies 1.0.0 already shipped.
- **Reuse `1.0.0-beta.1`, or skip untagged headings in the order check.** Two sections would share a heading, or the check would weaken.
- **Require an rc before every stable release.** A gate no V1 release needs.

**Consequences.**

- **Every existing tag stays valid,** and none is rewritten.

# 0023. V1 restarts at alpha and moves through beta to an LTS release

**Status:** Accepted, 2026-09-25. Amended by 0024 and 0029.

**Context.** The owner wants a fresh start: V1 alpha, beta, then LTS. Tags `v1.0.0-alpha.1` to `.4` exist from the prototype. `1.0.0-beta.1` is a changelog heading but was never tagged.

**Decision.** The rebuild ships as `1.0.0-alpha.5` onward while phases 0 to 3 land. Beta starts at `1.0.0-beta.2`, feature-complete, with only fixes landing while eval runs on every platform; `beta.2` avoids reusing the old heading. `1.0.0` is the LTS release, cut by the owner once the PRD's success criteria hold.

**Alternatives.**

- **Start a new series such as `1.1.0-alpha.1`.** Clean numbering, but it implies 1.0.0 already shipped.
- **Reuse `1.0.0-beta.1`.** Two changelog sections would share one heading.

**Consequences.**

- **Every existing tag stays valid,** and none is rewritten.

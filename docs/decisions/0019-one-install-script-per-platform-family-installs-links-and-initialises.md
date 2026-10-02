# 0019. One install script per platform family installs, links and initialises

**Status:** Accepted, 2026-09-25.

**Context.** `komodo` is not a command anyone has until it is installed, and the owner wants everything hooked up in one step.

**Decision.** `install.sh` and `install.ps1` check prerequisites, build or download the binary, link it onto PATH, install the global orchestrator layer, initialise the current repo and run doctor. Running one again updates the install. The `komodo` skill, generated from `komodo help`, documents the command for agents; the README documents it for people.

**Alternatives.**

- **Manual steps in the README.** Every machine drifts.

**Consequences.**

- **The first install also initialises the repo it runs in.**

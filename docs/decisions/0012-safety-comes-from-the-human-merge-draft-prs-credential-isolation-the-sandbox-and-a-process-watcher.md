# 0012. Safety comes from the human merge, draft PRs, credential isolation, the sandbox and a process watcher

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0012, 0026 and 0032.

**Context.** The guard was scored as a wall and never could be one (evidence 2). The forge is GitHub Free, and the developer's local git PAT is the only credential; GitHub Free offers rulesets and draft PRs only on public repositories. Spike S1 (2026-09-26, macOS) found the sandbox blocks Go's module fetches unless weaker network isolation is on, which opens an exfiltration path. With modules already in the worktree, a builder under `GOPROXY=off` passed `go vet` and `go test -race` under the strict sandbox. A test fork bomb of 900 processes once exhausted a 24 GB machine.

**Decision.**

- **Only the conductor reads the git credential, and only at Ship.** Every PR opens as a draft, or labelled `status: wip` where drafts aren't offered, and becomes ready once verified. A human merges.
- **Line sessions run in the OS sandbox where the platform has one,** with the forge off the network allowlist. Check compares outputs; the guard catches mistakes and fails open.
- **A builder runs Go offline:** `GOPROXY=off`, `GOFLAGS=-modcacherw`, and every Go directory inside the worktree. The conductor fetches modules outside the sandbox, and a task that needs a missing module fails fast, naming it.
- **Every station command and session runs under a watcher:** past 256 live processes or 8 GB resident, it kills the whole tree and names the breach. Nothing outlives what started it.
- **A session's temp root is a private directory outside every repo,** so no test can walk up into the real worktree.

**Alternatives.**

- **A bot account.** The current plan doesn't provide one.
- **Keep hardening the bash denylist.** It can never be complete.
- **Weaker network isolation for Go sessions.** Every Go session could then reach the trust service.

**Consequences.**

- **The developer authors every PR,** so their own approval doesn't count on GitHub; the merge is the human check.
- **No Go session reaches the network,** and a runaway becomes a failed check, never a machine that stops answering.

**Open.** No code runs `go mod download`, so a product Go repo with dependencies can't build in a session; build the fetch or relax `GOPROXY=off`. Doctor doesn't pin the `go` directive. The watcher and tree kill don't exist on native Windows (0017).

# 0026. The conductor fetches Go modules, and a builder runs Go offline in the sandbox

**Status:** Accepted, 2026-09-26. Amends 0012 and 0025. The owner chose the decision over weaker isolation.

**Context.** Decision 0012 turns the sandbox on by default, which holds only if Go's cache, module downloads and race tests work inside it. Spike S1 ran on 2026-09-26 on macOS with CLI 2.1.283 and a Sonnet builder under `dontAsk`.

- **Spike S1 result:** passes on macOS, with conditions.
  - **Allowed hosts:** with only `proxy.golang.org`, `sum.golang.org` and `storage.googleapis.com` allowed, `go get` failed TLS with `x509: OSStatus -26276`, since the sandbox shuts out macOS's trust service.
  - **Weaker isolation:** `sandbox.enableWeakerNetworkIsolation` lets Go verify certificates, and the CLI warns that it "opens a potential data exfiltration vector". `go get` then failed writing `~/go/pkg/sumdb`.
  - **Worktree paths:** with `GOPATH`, `GOMODCACHE`, `GOCACHE` and `GOTMPDIR` inside the worktree, `go get`, `go mod tidy`, `go vet` and `go test -race` all passed in 5 turns with 0 denials.
  - **Offline builder:** with modules already downloaded into the worktree, a builder under `GOPROXY=off` passed `go vet` and `go test -race -count=1` under the strict sandbox, with no weaker isolation and no allowed hosts.
  - **Go directive:** `go get` raised the `go` directive from 1.22 to 1.26.0 on its own.
- **Spike S1 WSL2 result:** not run; this session had only macOS. Phase 2 proceeds on macOS, and WSL2 joins spike S6.

**Decision.**

- **The conductor runs `go mod download` outside the sandbox** before a Go session, into the worktree's module cache.
- **A builder runs Go offline:** `GOPROXY=off`, `GOFLAGS=-modcacherw`, and every Go directory inside the worktree. The sandbox keeps its strict network isolation.
- **A task whose files include `go.mod` fails fast** when it needs a module the cache lacks, naming the module, so a person adds the dependency.

**Alternatives.**

- **Weaker isolation for every Go session.** A builder may add any dependency itself, but every Go session can then reach the trust service.
- **Weaker isolation only for a task that lists `go.mod`.** Narrower, but still an exfiltration path in the sessions most likely to fetch new code.

**Consequences.**

- **No Go session reaches the network.** Only the conductor fetches modules.
- **Doctor pins the `go` directive,** so a builder's `go get` can't raise it unnoticed.

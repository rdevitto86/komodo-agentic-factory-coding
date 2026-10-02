# 0002. The conductor is one static Go binary with no dependencies

**Status:** Accepted, 2026-09-25. Restates the first line's decision 0001.

**Context.** The prototype needed an interpreter, a virtual environment and a shell shim on every host, and started a fresh interpreter on every hook call.

**Decision.** The conductor, its checks and its hooks are one Go binary built from the standard library alone. Each platform gets its own binary, built with cgo off, trimmed paths and stripped symbols, with the version and commit stamped in, so a rebuild of one commit is byte-identical. `bin/` is never tracked.

**Alternatives.**

- **Rust.** It fits as well, but the working code is in Go, and Go cross-compiles with two environment variables.
- **Keep Python.** An interpreter on every host, and a start-up cost on every hook call.

**Consequences.**

- **A dependency is a design change,** and needs its own entry here.

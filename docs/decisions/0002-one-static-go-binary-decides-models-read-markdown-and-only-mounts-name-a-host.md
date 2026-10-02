# 0002. One static Go binary decides, models read markdown, and only mounts name a host

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0002, 0003 and 0004.

**Context.** The prototype needed an interpreter, a virtual environment and a shell shim on every host, and started a fresh interpreter on every hook call. A model that reads the conductor's source can reason about the stage order and route around it. Models must be interchangeable per stage, and the line must survive a change of host with no change outside one directory.

**Decision.**

- **The conductor, its checks and its hooks are one Go binary built from the standard library alone.** Each platform gets its own binary, built with cgo off, trimmed paths and stripped symbols, with the version and commit stamped in, so a rebuild of one commit is byte-identical. `bin/` is never tracked.
- **Models read markdown, never Go.** Rules, roles, skills, checklists and policy are markdown or JSON under `komodo/`. The conductor fills a brief from them, and a model reads only its brief. The stage order lives only in the binary.
- **Only `internal/mount/<host>/` names a host.** Every vendor name, host tool name, host path and host flag lives there, and doctor fails on any leak. A host plugs in through the contract in `lld.md#the-host-contract`: preflight, start, resume, stream, result, stop and a capability list.

**Alternatives.**

- **Rust.** It fits as well, but the working code is in Go, and Go cross-compiles with two environment variables.
- **Keep Python.** An interpreter on every host, and a start-up cost on every hook call.
- **Prompts inside Go.** Faster to change, but invisible to review, and a host swap would touch them.
- **A config file per host.** Data alone can't carry a host's command line, usage parsing or resume.

**Consequences.**

- **A dependency is a design change,** and needs its own decision.
- **A reviewer reads one brief,** the whole input a builder got, and swapping a model is a profile row.
- **A new host is a new directory;** a host without resume still works, with a fresh session given the fix list.

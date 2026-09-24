# 0001. The conveyor is one static Go binary with no dependencies

**Status:** Accepted, 2026-09-21.

## Context

The prototype's conveyor was a Python orchestrator. It needed an interpreter, a virtual environment, and a shell shim on every host. Its hook ran a fresh interpreter on every tool call.

## Decision

The conveyor, the devices, and the guard are one Go binary built from the standard library alone. Each platform gets its own binary, built with cgo off, trimmed paths, stripped symbols, and the changelog version and commit stamped in, so a rebuild of one commit is byte-identical. `bin/` is never tracked; `komodo gate --install` builds it.

## Alternatives

- **Rust.** It fits the process shape just as well. About a thousand lines of guard, hook, and detection code already existed in Go, and Go cross-compiles with two environment variables.
- **Keep Python.** It would carry an interpreter onto every host and a start-up cost onto every hook call.

## Consequences

- **Nothing to install.** A clone plus one command gives a working line.
- **Every change needs a rebuild.** The pre-commit hook in this repo does it.
- **A dependency is a design change.** It needs its own decision file.

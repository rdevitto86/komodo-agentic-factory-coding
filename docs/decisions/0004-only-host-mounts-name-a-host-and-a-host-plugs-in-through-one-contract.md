# 0004. Only host mounts name a host, and a host plugs in through one contract

**Status:** Accepted, 2026-09-25. Restates and extends the first line's decision 0003.

**Context.** Models must be interchangeable per stage: Claude today, Ollama and GPT models after 1.0.0. The line must survive a change of host with no change outside one directory.

**Decision.** Every vendor name, host tool name, host path and host flag lives in `internal/mount/<host>/`, and doctor fails on any leak. A host plugs in by implementing the host contract in `lld.md#the-host-contract`: preflight, start, resume, stream, result, stop and a capability list.

**Alternatives.**

- **A config file per host.** Data alone can't carry a host's command line, usage parsing or resume.

**Consequences.**

- **A new host is a new directory.**
- **A host without resume still works,** with a fresh session given the fix list.

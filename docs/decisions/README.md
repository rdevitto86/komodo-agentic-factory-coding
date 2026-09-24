# Decisions

One file per decision that shapes the line: what was chosen, what was not, and why. A decision is superseded by a new file that says so, never edited away.

| # | Decision | Status |
|---|---|---|
| [0001](0001-go-single-static-binary.md) | The conveyor is one static Go binary with no dependencies | Accepted |
| [0002](0002-models-read-markdown-only.md) | Models read markdown, never Go | Accepted |
| [0003](0003-host-names-live-in-mounts.md) | Only `internal/mount/` names a host | Accepted |
| [0004](0004-one-guard-hook-not-a-sandbox.md) | One guard hook, which is not a sandbox | Accepted |
| [0005](0005-guard-reads-through-a-lexer.md) | The guard reads a command through one shell lexer | Accepted |
| [0006](0006-local-gate-no-ci.md) | The gate is local; nothing runs on the forge | Accepted |

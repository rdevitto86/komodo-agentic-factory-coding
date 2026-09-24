# 0006. The gate is local; nothing runs on the forge

**Status:** Accepted, 2026-09-21. Amended 2026-09-23 with race tests and a fuzz pass on push.

## Context

Hosted CI bills minutes on a free account. It fails on a runner instead of at the desk, and it adds a stage the line does not own.

## Decision

`komodo gate` is the only precheck. It is mechanical, model-free, and runs before every commit and every push through git hooks that `komodo gate --install` writes. It runs vet, then tests under the race detector when cgo can build it, then doctor, the guard table, and the comment lint. The pre-push hook adds `--fuzz 10s`, which fuzzes the guard, its lexer, the backlog parser, and the ledger reader for ten seconds each.

## Alternatives

- **Hosted CI.** It costs minutes and money, and it moves failure away from the person who can fix it.
- **Fuzz on every commit.** It adds forty seconds to each commit. A push is the moment the work leaves the desk.

## Consequences

- **A red gate never leaves the machine.** Nothing reaches the forge unchecked by the hook.
- **A hook can be skipped with `--no-verify`.** The forge's ruleset on critical refs is the backstop, per decision 0004.
- **Race tests need cgo.** Without it the gate runs plain tests and still passes.

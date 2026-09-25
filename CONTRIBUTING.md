# Contributing

`README.md` is the plan and the requirements. `BACKLOG.md` is the work. `AGENTS.md` holds the rules every contributor follows, human or model.

## Set up

```bash
go run ./cmd/komodo gate --install   # build bin/komodo-<os>-<arch>, write the pre-commit and pre-push hooks
go run ./cmd/komodo version          # the changelog version and commit this build carries
```

Go only, standard library only. The toolchain is pinned in `go.mod`; `GOTOOLCHAIN` fetches it.

## Change something

1. **Branch.** Name it `<type>/<kebab-name>`. Never commit to `main` or `master`; the guard refuses it.
2. **Find or add the task.** Work enters as a task in `BACKLOG.md`. Run `komodo lint` after every backlog edit.
3. **Write the test first.** Every station has a test. A guard change adds a row to one of `internal/guard/table*.go`, denied or allowed, before the code.
4. **Run the gate.** `komodo gate` runs vet, race tests, doctor, the guard table, and the comment lint. The pre-commit hook runs it for you.
5. **Push.** The pre-push hook adds `--fuzz 10s`. Open a pull request; merging is a human's button.

## Rules that fail the gate

- **A name outside its mount.** No vendor, host tool, host path, or host flag outside `internal/mount/`. `komodo doctor` names the line.
- **A comment that restates.** A public function gets a one-line doc comment of at most twenty words, saying what the code does. `komodo comments check` is the lint.
- **A guard row that flips.** `komodo guard check` must print `0 wrong`.
- **A dependency.** `go.mod` holds no `require`.

## Where things live

| Path | Holds |
|---|---|
| `cmd/komodo/` | The binary, one file per command family |
| `internal/line/` | The stations: next, brief, close, step, report |
| `internal/guard/` | The one hook: lexer, parser, path and git rules, the table |
| `internal/mount/` | Everything that names a host |
| `komodo/` | The markdown a model reads: rules, roles, skills, policy, facets |
| `docs/decisions/` | Why the line is shaped the way it is |

## Fuzzing by hand

```bash
go test -run='^$' -fuzz='^FuzzCheck$' -fuzztime=5m ./internal/guard
```

A failure writes its input under `testdata/fuzz/`. Commit that file with the fix, so the case replays in every later `go test`.

# Komodo harness

The harness is code, not prose. A session drives it with one command and reads the report; it never re-implements the pipeline by hand.

## Commands
| Do | Run |
|---|---|
| Run the next open task group | `python3 -m komodo run` |
| Run a named group, thinking profile | `python3 -m komodo run TG-01.2 --profile thinking` |
| Preview waves, briefs, and token estimates | `python3 -m komodo run TG-01.2 --dry-run` |
| Resume an unfinished run | `python3 -m komodo run TG-01.2 --resume` |
| Runs, stale worktrees, merged branches | `python3 -m komodo status` (add `--prune` to clean) |
| Lint or list the backlog | `python3 -m komodo tasks lint` / `tasks list` |
| Draft tasks from a goal into a group | `python3 -m komodo tasks plan TG-01.3 "<goal>"` |
| Answer PR review threads | `python3 -m komodo pr respond` |
| Merge the base branch in, resolving conflicts | `python3 -m komodo pr sync` |
| Cut a version, level read off the changelog | `python3 -m komodo release --bump` |
| Cut a version at a level you choose | `python3 -m komodo release --bump minor` |
| Tag the newest released version | `python3 -m komodo release` |
| Audit changelog and tag drift, read-only | `python3 -m komodo release check` |
| Fragment check | `python3 -m komodo doctor` |

## What a run does
Preflight (lint, waves, clean tree, tag a merged version still untagged), branch, build waves in parallel worktrees, verify once, review once, changelog and backlog update, cut the version, push, open the PR. The report lands in `.komodo/runs/<id>/report.md` and in the PR body. A human merges.

## Rules for the session
- Run `--dry-run` first when the group is new or large; read the wave plan before spending.
- Never run the pipeline steps by hand around the CLI. If the CLI refuses, fix the cause it names.
- A BLOCKED task in the report is a task for a human or a backlog fix, not a reason to edit the branch directly.
- Close-out cuts the version itself, so `release --bump` is only for a version a human cuts by hand. `release --bump` decides the level itself: a `Removed` section or a breaking bullet is major, an `Added` section is minor, anything else is patch. Pass a level to override it.
- `release check` is the release-integrity gate and runs inside `make verify`: an entry with no tag, a tag with no entry, and a date-separator mismatch each fail it. A version tagged over by a later one carries a `> Never released` blockquote and is skipped.
- Report the outcome with the mandatory three-bucket summary from the agent rules.

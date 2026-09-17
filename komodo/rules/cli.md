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
| Cut a version from Unreleased | `python3 -m komodo release --bump minor` |
| Tag the newest released version | `python3 -m komodo release` |
| Fragment check | `python3 -m komodo doctor` |

## What a run does
Preflight (lint, waves, clean tree), branch, build waves in parallel worktrees, verify once, review once, changelog and backlog update, push, open the PR. The report lands in `.komodo/runs/<id>/report.md` and in the PR body. A human merges.

## Rules for the session
- Run `--dry-run` first when the group is new or large; read the wave plan before spending.
- Never run the pipeline steps by hand around the CLI. If the CLI refuses, fix the cause it names.
- A BLOCKED task in the report is a task for a human or a backlog fix, not a reason to edit the branch directly.
- Report the outcome with the mandatory three-bucket summary from the agent rules.

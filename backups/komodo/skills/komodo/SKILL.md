---
name: komodo
description: The komodo command reference, generated from komodo help.
---

# komodo

Every command the binary takes. `komodo help --skill` writes this file; never edit it by hand.

```
  komodo init [--name n]      Write the starter files into a new repo, keeping any that exist
  komodo migrate [--dry-run]  Move a flat backlog, BACKLOG.md or TODO.md into the docs/backlog tree
  komodo lint                 Check the backlog against the grammar
  komodo list [--json]        List every task, or one group's tasks
  komodo backlog              List every epic and its open groups under docs/backlog
  komodo add <epic|group> <title>  Add an epic, a group, or append a task
  komodo ingest [group]       Compile each READY group into a card under .komodo/queue
  komodo check <kind> <id>    The checks hooks and agents call: task, scope, or a review's findings
  komodo comments check       The mechanical comment lint
  komodo diff                 The reviewer's whole input: tasks, standards, diff
  komodo tag                  Tag every changelog version no tag points at
  komodo rephase <e> <v>      Move an open epic to a new version: branch, pull requests, backlog
  komodo release check        Audit the drift between changelog, tags, and groups
  komodo release build        Build the per-platform binaries as release assets
  komodo release publish      Build, test, checksum and publish the newest version as a release
  komodo install --host X     Mount this repo on a host, or on both
  komodo detect [--json]      The cached repo profile: languages, cloud, data, CI, commands
  komodo doctor [--prune]     References, roles, leaks, drift, budgets, leftovers; prune clears stale worktrees and runs
  komodo guard [check]        The one agent hook; check runs its table
  komodo hook <name>          Every other agent hook's entry point; a hook that fails allows
  komodo run [group|task]     Drive the harness headless on this host, under a budget
  komodo resume <group>       The state a killed run left: continue its session or start from its WIP
  komodo status [--watch]     The current run: groups by state, time used and blockers
  komodo abandon <group>      Remove a group's worktree and branch on purpose, and mark its tasks BLOCKED
  komodo worktree add <b>     Cut an ad hoc detached worktree at .komodo/wt tracking branch b
  komodo ship <group>         Publish a group a missing credential stopped: push, draft PR, labels
  komodo sync [--dry-run]     Fast-forward the root to origin, rebuild a stale binary, re-render drift
  komodo pr create --title t  Open a pull request from this branch: title checked, labels applied
  komodo pr label             Apply the labels this branch's open pull request earns
  komodo threads [pr]         The unresolved review threads, as JSON
  komodo threads --resolve id Mark one review thread resolved
  komodo metrics              What the two ledger files hold
  komodo recall [--model m]   Score the local reviewer against the seeded-bug corpus
  komodo eval [--list|--cases|--runs N]  The golden suite: list it, run the eval cases, or drive each group N times
  komodo version              The changelog version and commit this binary was built from
  komodo gate [--install]     The local precheck: vet, race tests, doctor, guard, comments; --fuzz 10s adds fuzzing
  komodo git-hook <name>      One git hook's checks; each installed hook is one exec into this
  komodo help [--skill]       This list, or the komodo skill generated from it
```

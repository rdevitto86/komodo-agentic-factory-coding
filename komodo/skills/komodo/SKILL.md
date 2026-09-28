---
name: komodo
description: The komodo command reference, generated from komodo help.
---

# komodo

Every command the binary takes. `komodo help --skill` writes this file; never edit it by hand.

```
  komodo init [--name n]      Write the starter files into a new repo, keeping any that exist
  komodo lint                 Check the backlog against the grammar
  komodo list [--json]        List every task, or one group's tasks
  komodo backlog             List the open groups under docs/backlog
  komodo add <group> <title>  Add a group, or append a task to one
  komodo next [--json]        The next ready group: tasks, waves, machines
  komodo brief <task>         Fill the role template and write the brief
  komodo ingest [group]       Compile each READY group into a card under .komodo/queue
  komodo close <task>         Validate the result, rerun the checks, flip the status
  komodo close --wave N [g]   QC: merge the group's wave, compile, verify
  komodo close --group [g]    Ship: commit, push, the pull request, the changelog
  komodo check <kind> <id>    The checks hooks and agents call: task, scope, or a review's findings
  komodo comments check       The mechanical comment lint
  komodo diff                 The reviewer's whole input: tasks, standards, diff
  komodo report               What the run did, in the accessibility contract
  komodo tag                  Tag every changelog version no tag points at
  komodo release check        Audit the drift between changelog, tags, and groups
  komodo release build        Build the per-platform binaries as release assets
  komodo release fold         Fold every changelog fragment into CHANGELOG.md
  komodo release publish      Build, test, checksum and publish the newest version as a release
  komodo install --host X     Mount this repo on a host, or on both
  komodo detect [--json]      The cached repo profile: languages, cloud, data, CI, commands
  komodo doctor [--prune]     References, roles, leaks, drift, budgets, leftovers
  komodo guard [check]        The one agent hook; check runs its table
  komodo hook <name>          Every other agent hook's entry point; a hook that fails allows
  komodo run [group|task]     Drive the line headless on this host, under a budget
  komodo resume <group>       The state a killed run left: continue its session or start from its WIP
  komodo status [--watch]     The current run: groups by state, time used and blockers
  komodo abandon <group>      Remove a group's worktree and branch on purpose, and mark its tasks BLOCKED
  komodo ship <group>         Publish a group a missing credential stopped: push, draft PR, labels
  komodo sync [--dry-run]     Fast-forward the root to origin, rebuild a stale binary, re-render drift
  komodo stage <s> [group]    Run one stage ad hoc, build, review or ship, on a group or the current branch
  komodo step [group|task]    The one next action, as JSON
  komodo threads [pr]         The unresolved review threads, as JSON
  komodo threads --resolve id Mark one review thread resolved
  komodo machine <task>       Post a brief to the local machine, write the result, stamp the ledger
  komodo metrics              What the two ledger files hold
  komodo recall [--model m]   Score the local reviewer against the seeded-bug corpus
  komodo eval [--list|--cases|--runs N]  The golden suite: list it, run the eval cases, or drive each group N times
  komodo version              The changelog version and commit this binary was built from
  komodo gate [--install]     The local precheck: vet, race tests, doctor, guard, comments; --fuzz 10s adds fuzzing
  komodo help [--skill]       This list, or the komodo skill generated from it
```

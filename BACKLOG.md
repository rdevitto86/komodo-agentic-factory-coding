# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar is `komodo/rules/backlog.md`; run `komodo lint` after every edit.

Recreated on 2026-09-25 to deliver `docs/prd.md`. Each epic is one phase of `docs/system-design.md#rollout`, and every requirement is proven by at least one group. Earlier groups are history in `CHANGELOG.md` and git.

* **Phases run in order.** Phase 0 is READY. A later phase stays REFINEMENT until the phase before it has merged and its spikes have passed; then the orchestrator promotes its tasks to READY.
* **Spikes and proofs are human tasks.** The orchestrator runs them with the owner and records each spike as a `**Spike Sn result:**` line in a new entry in `docs/decisions.md`. The line never picks a human task.
* **Every group cuts from `main`** unless it names a group in `depends_on` (REQ-13).
* **This file moves.** TG-07.10 splits the open groups into `docs/backlog/` and deletes it.
* **Carried over:** open tasks from TG-03.31 and TG-03.32 that V1 still needs are folded in and name their source; the rest are superseded by V1.

---

## [EPIC-04] Phase 0: stabilize
*Goal: the gate is green, `main` is every group's base, and a pull rebuilds the binary. Ships as `1.0.0-alpha.5`.*

* **The guard is frozen.** No group before TG-06.2 adds a guard rule; that group cuts it to five.

### [TG-04.1] The line stops judging its own commands, and the gate fails loudly
```yaml
type: fix
version: 1.0.0-alpha.5
```
* **Why:** the two regressions #201 merged: the guard refused about 12 legitimate `done_when` commands, and the gate passed in a repo where it ran no build check (decision 0001, evidence 15).

#### [TSK-04.1.1] Commands the line runs skip the guard [P: C] [READY]
```yaml
files: [internal/line/verify.go, internal/line/verify_test.go]
done_when:
  - go test ./internal/line/...
  - "! grep -q guardRefusal internal/line/verify.go"
context:
  - docs/system-design.md#hooks
  - "runGuarded passes every done_when, verify and gate command through guard.CheckCommand; the guard judges only a model's tool calls, so these run straight through proc.ShellEnv"
  - "drop guardRefusal and the tests that expect a refusal; keep the timeout, the output clip and the process-group kill"
type: fix
```

#### [TSK-04.1.2] The gate fails when it finds no build check [P: C] [READY]
```yaml
files: [cmd/komodo/gate.go, cmd/komodo/gate_test.go]
done_when:
  - go test ./cmd/komodo/...
  - go vet ./cmd/komodo/...
context:
  - docs/system-design.md#run-failures
  - "buildChecks returns no check when detection finds no compile or verify command, so the gate passes with nothing run; fail instead, naming the .komodo/commands.json line that fixes it"
  - "test: a temp repo with no detected language fails with that message; the toolkit's own checkout still runs go vet and go test"
type: fix
```

### [TG-04.2] Line endings are LF everywhere, and a pull rebuilds the binary
```yaml
type: feat
version: 1.0.0-alpha.5
```
* **Why:** no `.gitattributes` existed (evidence 14), and `bin/` went stale after every pull until someone rebuilt it by hand. Proves REQ-4 and REQ-5 for this repo.

#### [TSK-04.2.1] `.gitattributes` sets LF in the toolkit and in init's template [P: H] [READY]
```yaml
files: [.gitattributes, templates/project/.gitattributes, cmd/komodo/init_test.go]
done_when:
  - grep -q 'eol=lf' .gitattributes
  - grep -q 'eol=lf' templates/project/.gitattributes
  - go test ./cmd/komodo/...
context:
  - docs/system-design.md#sessions-pinned-and-hermetic
  - "both files hold * text=auto eol=lf; init_test's starterFiles gains .gitattributes, which the all:templates/project embed already carries"
```

#### [TSK-04.2.2] Doctor fails a repo whose `.gitattributes` does not set LF [P: H] [READY]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
  - go run ./cmd/komodo doctor
depends_on: [TSK-04.2.1]
context:
  - "a missing file, or one whose * line lacks eol=lf, is a problem naming the line to add (REQ-4)"
```

#### [TSK-04.2.3] A pull, checkout or rebase rebuilds the binary when Go sources changed [P: C] [READY]
```yaml
files: [internal/gate/gate.go, internal/gate/gate_test.go, cmd/komodo/gate.go]
done_when:
  - go test ./internal/gate/... ./cmd/komodo/...
context:
  - docs/system-design.md#binaries-and-releases
  - "gate --install also writes post-merge, post-checkout and post-rewrite hooks; each runs komodo gate --rebuild, which rebuilds bin/komodo-<platform> and stamps bin/.built-from only when a .go file, go.mod or go.sum differs between the old and new HEAD"
  - "post-checkout acts only on a branch checkout in the main working tree, never in a worktree the line cut"
  - "test (REQ-5): after a pull that changes a Go file in a temp repo, bin/.built-from equals the new HEAD"
```

#### [TSK-04.2.4] A build from a dirty tree is never stamped as built from HEAD [P: M] [READY]
```yaml
files: [internal/run/sync.go, internal/run/sync_test.go]
done_when:
  - go test ./internal/run/...
depends_on: [TSK-04.2.3]
context:
  - "syncBinary and gate --rebuild share one stamp function; it skips the stamp when git status --porcelain reports tracked changes, and writes it only after the hooks install (from TSK-03.32.4)"
type: fix
```

### [TG-04.3] Every group cuts from `main`, or from a group it depends on
```yaml
type: fix
version: 1.0.0-alpha.5
```
* **Why:** TG-03.31 and TG-03.32 cut from a branch 39 commits ahead of `main`, and hand commits rode along unreviewed (evidence 13). Proves REQ-13.

#### [TSK-04.3.1] Lint rejects a base that is neither the default branch nor a dependency's branch [P: H] [READY]
```yaml
files: [internal/backlog/backlog.go, internal/backlog/lint.go, internal/backlog/backlog_test.go, komodo/rules/backlog.md]
done_when:
  - go test ./internal/backlog/...
  - go run ./cmd/komodo lint
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#task-groups
  - "a group gains depends_on: [TG-..]; base may be omitted, main, or the branch the line cuts for a group named in depends_on; anything else is a problem naming both"
  - "branch naming moves into internal/backlog if lint needs it, so line and lint share one function"
  - "the rules file documents group depends_on as a grammar bullet, which doctor's promise check ties to its accessor"
type: fix
```

#### [TSK-04.3.2] A re-run cuts from its base, not from the group's stale remote branch [P: M] [READY]
```yaml
files: [internal/line/worktree.go, internal/line/worktree_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "StartRef cuts from origin/<group branch> whenever that branch exists, so a group run again after an earlier push inherits old commits; cut from the base unless the run resumes that same group (from TSK-03.31.1)"
type: fix
```

### [TG-04.4] The README hands the design to the specs
```yaml
type: docs
version: 1.0.0-alpha.5
```
* **Why:** the README and the four specs describe the same parts twice, and the README's copy is the first line's design.

#### [TSK-04.4.1] README keeps setup, usage and names, and links the specs for design [P: M] [READY]
```yaml
files: [README.md, docs/architecture.md, docs/system-design.md]
done_when:
  - "! grep -q '^## The guard' README.md"
  - grep -q 'docs/system-design.md' README.md
  - go run ./cmd/komodo doctor
context:
  - docs/architecture.md#purpose
  - "cut The line, Stations, Devices, Metrics, Machines and mounts, Ad hoc work, The guard, The binary, The gate, The repo layer, Detection and facets, Local machines, Hot swap and The non-proprietary day; each becomes one line linking the spec section that owns it"
  - "keep Setup, Usage, Layout, Names, Pull requests and Versions; Versions follows decision 0023: alpha.5 onward, then beta.2, then 1.0.0 LTS"
  - "a fact V1 keeps that no spec holds moves to the spec section that owns it; prd.md is never edited"
type: docs
```

---

## [EPIC-05] Phase 1: the conductor drives
*Goal: one group runs through the conductor within 60 minutes, with zero conductor tokens. Ships as `1.0.0-alpha.6`.*

### [TG-05.1] Spikes for the conductor and its sessions
```yaml
type: docs
version: 1.0.0-alpha.6
```
* **Why:** decisions 0005 and 0006 hold only if these pass. A failed spike gets a new decision entry that supersedes the one it breaks, and the orchestrator rewrites the groups it changes before promoting them.

#### [TSK-05.1.1] S2: dontAsk, an allow list and the sandbox run a builder with no prompt and no refusal [P: C] [READY]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S2 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.2] S3: a line-owned config directory shuts out the personal layer and keeps the login [P: C] [READY]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S3 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.3] S4: the final stream event carries turns, usage, cost and a session ID resume accepts [P: C] [READY]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S4 result' docs/decisions.md
context:
  - "keep the recorded stream as the fixture TSK-05.2.3 tests against"
owner: human
type: docs
```

#### [TSK-05.1.4] S5: schema output holds over a long builder session [P: H] [READY]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S5 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.5] S7: `komodo run`, started inside an interactive session, launches its own sessions [P: C] [READY]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S7 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.6] S8: what the max-turns and subprocess-scrub variables do in a headless session [P: H] [READY]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S8 result' docs/decisions.md
context:
  - "the variables are CLAUDE_CODE_MAX_TURNS and CLAUDE_CODE_SUBPROCESS_ENV_SCRUB"
owner: human
type: docs
```

### [TG-05.2] The host contract
```yaml
type: feat
version: 1.0.0-alpha.6
```
* **Why:** the conductor starts, resumes, streams and stops sessions through one contract, so a host is one package (decision 0004). Proves the meter behind REQ-28.

#### [TSK-05.2.1] The host contract is one Go interface every mount implements [P: C] [REFINEMENT]
```yaml
files: [internal/mount/mount.go, internal/mount/host.go, internal/mount/host_test.go, internal/mount/registry.go]
done_when:
  - go test ./internal/mount/...
  - go vet ./...
context:
  - docs/system-design.md#the-host-contract
  - "operations: preflight, start, resume, stream, result, stop, capabilities; a fake host in host_test.go is what every conductor test drives"
  - "the Codex and Ollama mounts keep compiling and declare no capabilities (decision 0022)"
```

#### [TSK-05.2.2] Claude starts and resumes a role's session headless [P: C] [REFINEMENT]
```yaml
files: [internal/mount/claude/session.go, internal/mount/claude/session_test.go]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-05.2.1]
context:
  - docs/system-design.md#how-the-conductor-runs-a-claude-code-session
  - "argv: -p, --plugin-dir, --settings, --tools, --permission-mode dontAsk, --model, --effort, --strict-mcp-config, --output-format stream-json, --json-schema, and --resume to resume; env CLAUDE_CONFIG_DIR and CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=3; --max-budget-usd on API billing"
  - "the process starts in the group's worktree in its own process group, and stop kills the tree; spikes S2 and S5 set the permission and schema flags"
  - "tests assert the argv and environment for the builder and for a lens"
```

#### [TSK-05.2.3] The stream reports turns, usage, cost, rate limits and the session ID [P: C] [REFINEMENT]
```yaml
files: [internal/mount/claude/stream.go, internal/mount/claude/stream_test.go, internal/mount/claude/usage.go, internal/mount/claude/usage_test.go, internal/mount/claude/testdata]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-05.2.1]
context:
  - docs/system-design.md#run-state-and-metrics
  - "parse stream-json into the contract's stream, including rate_limit_event with five_hour, seven_day and resetsAt; the result event's totals are the meter, since summing transcripts logged 45,520,132 input tokens in 63 turns (evidence 8)"
  - "fixtures come from the stream spike S4 recorded"
```

### [TG-05.3] Pinned, hermetic, role-scoped sessions
```yaml
type: feat
version: 1.0.0-alpha.6
depends_on: [TG-05.2]
```
* **Why:** three machines ran three different agents (evidence 10). Proves REQ-2, REQ-3 and REQ-16.

#### [TSK-05.3.1] Profiles pin the host version, full model IDs and effort per role [P: H] [REFINEMENT]
```yaml
files: [internal/profile/profile.go, internal/profile/profile_test.go, komodo/profiles]
done_when:
  - go test ./internal/profile/...
context:
  - docs/system-design.md#profiles-and-economy-mode
  - docs/system-design.md#sessions-pinned-and-hermetic
  - "full and economy profiles as JSON under komodo/profiles, which the komodo embed carries; models are full IDs such as claude-sonnet-5 and claude-opus-5-5, never an alias"
```

#### [TSK-05.3.2] Line sessions load a line-owned config directory and no personal layer [P: C] [REFINEMENT]
```yaml
files: [internal/mount/claude/config.go, internal/mount/claude/config_test.go]
done_when:
  - go test ./internal/mount/claude/...
context:
  - docs/system-design.md#sessions-pinned-and-hermetic
  - "CLAUDE_CONFIG_DIR points at a directory under ~/.komodo that the line renders: DISABLE_AUTOUPDATER, and no personal CLAUDE.md, settings, plugins or MCP; spike S3 decides how the login carries over"
  - "test (REQ-3): a canary line in a fake personal config never appears in the rendered directory or a session's argv"
```

#### [TSK-05.3.3] Each role runs with its own plugin: its skills, hooks and agents only [P: H] [REFINEMENT]
```yaml
files: [internal/mount/claude/plugin.go, internal/mount/claude/plugin_test.go]
done_when:
  - go test ./internal/mount/claude/...
context:
  - docs/system-design.md#skills-and-scoping
  - "one plugin directory per role; the builder's holds the build skill and the standards for the languages the group touches, never a review, planning or orchestrator skill"
  - "test (REQ-16): the rendered builder plugin lists exactly those skills"
```

#### [TSK-05.3.4] Doctor fails when any pin differs [P: H] [REFINEMENT]
```yaml
files: [internal/doctor/pins.go, internal/doctor/pins_test.go, internal/doctor/doctor.go]
done_when:
  - go test ./internal/doctor/...
  - go run ./cmd/komodo doctor
depends_on: [TSK-05.3.1]
context:
  - docs/system-design.md#health-checks
  - "the host CLI version, the profile's model IDs, the komodo release and the go.mod toolchain; exit 0 when every pin matches, non-zero naming each one that differs (REQ-2)"
```

### [TG-05.4] The conductor drives the stages
```yaml
type: feat
version: 1.0.0-alpha.6
depends_on: [TG-05.3]
```
* **Why:** a model relayed `komodo step` and its tokens were never metered (evidence 1). Proves REQ-6, REQ-11, REQ-14, REQ-15 and REQ-31.

#### [TSK-05.4.1] Preflight checks the login, the forge credential, the sandbox and the budget [P: H] [REFINEMENT]
```yaml
files: [internal/preflight/preflight.go, internal/preflight/preflight_test.go]
done_when:
  - go test ./internal/preflight/...
context:
  - docs/system-design.md#run-failures
  - docs/system-design.md#health-checks
  - "doctor runs first; the host login through the contract's preflight; the forge credential is checked present, never read into a session; the sandbox where the platform has one; the budget on API billing"
  - "each failure stops the run and names its fix; --no-ship skips the forge check; one test per failed check (REQ-6)"
```

#### [TSK-05.4.2] Group states live in state.json, written before each state's work [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/state.go, internal/conductor/state_test.go, internal/conductor/fuzz_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - docs/system-design.md#group-states
  - docs/system-design.md#run-state-and-metrics
  - "a pure function from disk state to the next action, ported from Next in internal/line/snapshot.go with its fuzz test (decision 0001)"
```

#### [TSK-05.4.3] The conductor runs every stage itself, and models only build, review and repair [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
  - go vet ./internal/conductor/...
depends_on: [TSK-05.4.2]
context:
  - docs/architecture.md#data-flow
  - "sessions start and resume through the host contract; Check, Prepare and Ship call the stations internal/line already has; review stays one reviewer session until TG-07.5"
  - "tests drive the fake host: every transition is the conductor's, and the ledger holds only build, review and repair sessions (REQ-11, REQ-31)"
tier: heavy
```

#### [TSK-05.4.4] `komodo run` runs the conductor, not a model relaying stages [P: C] [REFINEMENT]
```yaml
files: [internal/run/run.go, internal/run/run_test.go, cmd/komodo/line.go, internal/mount/claude/claude.go]
done_when:
  - go test ./internal/run/... ./cmd/komodo/...
  - "! grep -q bypassPermissions internal/mount/claude/claude.go"
depends_on: [TSK-05.4.1, TSK-05.4.3]
context:
  - docs/system-design.md#the-komodo-command
  - "Launch runs preflight, then the conductor; Headless and its /run prompt go, and so does komodo step once nothing calls it; spike S7 decides how a run started inside a session launches its own"
```

#### [TSK-05.4.5] The run skill launches and watches, and never relays a stage [P: H] [REFINEMENT]
```yaml
files: [komodo/skills/run/SKILL.md]
done_when:
  - "! grep -q 'komodo step' komodo/skills/run/SKILL.md"
  - go run ./cmd/komodo doctor
depends_on: [TSK-05.4.4]
context:
  - docs/system-design.md#orchestrator-commands
  - "the skill starts komodo run in the background, reports komodo status, and stops or resumes groups; it never calls brief, close or step"
type: docs
```

#### [TSK-05.4.6] A killed run resumes without repeating a session or losing an edit [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/resume.go, internal/conductor/resume_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/conductor/... ./cmd/komodo/...
depends_on: [TSK-05.4.4]
context:
  - docs/system-design.md#stopped-and-blocked-work
  - "komodo stop saves a local WIP commit on the group branch and records it in state.json; komodo resume continues from the last state, resuming the session or starting fresh from the WIP commit"
  - "test (REQ-14): kill a run mid-build, resume it, and find every edit and no repeated session"
```

#### [TSK-05.4.7] A run never rebuilds the binary it is running [P: H] [REFINEMENT]
```yaml
files: [internal/run/run.go, internal/run/sync.go, internal/run/run_test.go, internal/run/sync_test.go]
done_when:
  - go test ./internal/run/...
depends_on: [TSK-05.4.4]
context:
  - docs/system-design.md#sessions-pinned-and-hermetic
  - "sync runs before a run starts and after it ends, never between groups; the rebuild hook skips while a run holds the lock; a sync failure names its step (from TSK-03.32.5)"
  - "test (REQ-15): a stale build marker during a run changes nothing until the run ends"
```

### [TG-05.5] Metrics and the clock
```yaml
type: feat
version: 1.0.0-alpha.6
depends_on: [TG-05.4]
```
* **Why:** one build took 122 turns with no cap but a 90-minute group budget (evidence 9). Proves REQ-28, REQ-29 and REQ-31.

#### [TSK-05.5.1] Each run writes metrics.jsonl and events.jsonl from the host's own totals [P: H] [REFINEMENT]
```yaml
files: [internal/ledger/ledger.go, internal/ledger/ledger_test.go]
done_when:
  - go test ./internal/ledger/...
context:
  - docs/system-design.md#run-state-and-metrics
  - "one line per stage and session: run, group, stage, start, duration, turns, input, output and cached tokens, cost, outcome; each run starts fresh, and only the last 10 run folders stay"
```

#### [TSK-05.5.2] `komodo report` sums a run, and its sums match the host's [P: H] [REFINEMENT]
```yaml
files: [internal/line/report.go, internal/line/report_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/line/... ./cmd/komodo/...
depends_on: [TSK-05.5.1]
context:
  - "test (REQ-28): a recorded stream's result totals equal the report's line for that session"
  - "the headline figure is tokens per accepted group"
```

#### [TSK-05.5.3] A group has 60 minutes, and each session its own limit [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/clock.go, internal/conductor/clock_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - docs/system-design.md#pacing-limits-and-loop-detection
  - "starting values in the profile: build 25, lens 8, repair 10, re-review 5 minutes; at a limit the conductor kills the session's tree, saves a WIP commit and stops the group; TG-07.7 makes the stop an escalation"
  - "test (REQ-29): a fake session past its limit is killed, and the group stops by 60 minutes"
```

#### [TSK-05.5.4] Proof: one group runs through the conductor [P: H] [REFINEMENT]
```yaml
done_when:
  - go run ./cmd/komodo report
context:
  - "the phase exit: promote TG-06.2, run it with komodo run, and confirm a PR within 60 minutes with zero tokens outside build, review and repair sessions; record the run ID in the PR"
owner: human
type: test
```

---

## [EPIC-06] Phase 2: guardrails
*Goal: every safety proof passes. Ships as `1.0.0-alpha.7`.*

### [TG-06.1] Spike S1: Go under the sandbox
```yaml
type: docs
version: 1.0.0-alpha.7
```
* **Why:** decision 0012 turns the sandbox on by default, which holds only if Go's cache, module downloads and race tests work inside it.

#### [TSK-06.1.1] S1 on macOS: Go builds, module downloads and race tests pass under the sandbox [P: C] [REFINEMENT]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S1 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-06.1.2] S1 on WSL2 [P: H] [REFINEMENT]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S1 WSL2 result' docs/decisions.md
context:
  - "needs a Windows machine with WSL2; with none at hand, record that, and phase 2 proceeds on macOS while WSL2 joins spike S6"
owner: human
type: docs
```

### [TG-06.2] The guard keeps five rules
```yaml
type: refactor
version: 1.0.0-alpha.7
```
* **Why:** 4,597 lines re-implemented bash, and every guard diff invited new bypass findings (evidence 2). Proves REQ-26's guard row, REQ-37 and REQ-41.

#### [TSK-06.2.1] The guard is cut to five rules and the builder's file scope [P: C] [REFINEMENT]
```yaml
files: [internal/guard]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
  - test ! -f internal/guard/interp.go
  - test ! -f internal/guard/shell_expand.go
context:
  - docs/system-design.md#security
  - "delete the bash interpreter and expansion code; keep a tokenizer that finds a git or gh subcommand and a write target; the matcher covers Bash, PowerShell and Monitor"
  - "the table keeps one row per rule, including a model session's push refused (REQ-26), and drops the rows for hidden intent"
tier: heavy
type: refactor
```

#### [TSK-06.2.2] A refusal names the way forward, and three of one rule end the session as blocked [P: C] [REFINEMENT]
```yaml
files: [internal/guard/hook.go, internal/guard/hook_test.go]
done_when:
  - go test ./internal/guard/...
depends_on: [TSK-06.2.1]
context:
  - docs/system-design.md#hooks
  - "count refusals per session and rule in the run folder; the third ends the session as blocked through the hook's output; a guard error allows the call and logs it"
  - "tests (REQ-37): the limit, the named alternative, and failing open"
```

#### [TSK-06.2.3] Line sessions can't edit the PRD or the golden suite [P: H] [REFINEMENT]
```yaml
files: [komodo/policy.json, internal/guard/policy.go, internal/guard/table.go]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-06.2.1]
context:
  - "docs/prd.md and eval/** are refused to every line role, with one guard table row each (REQ-41); the orchestrator is not a line session"
  - "line sessions carry their role in the environment the conductor sets; a session with none is the orchestrator"
```

### [TG-06.3] Hooks follow one contract
```yaml
type: feat
version: 1.0.0-alpha.7
```
* **Why:** most loops in the first line came from hooks: 187 builder refusals and review rounds chasing guard bypasses. Proves REQ-37.

#### [TSK-06.3.1] Every hook has one job, one stage and a limit, and fails open [P: C] [REFINEMENT]
```yaml
files: [internal/hooks/hooks.go, internal/hooks/hooks_test.go, cmd/komodo/hook.go]
done_when:
  - go test ./internal/hooks/... ./cmd/komodo/...
context:
  - docs/system-design.md#hooks
  - "komodo hook <name> is each hook's entry point; the table of hooks, sessions, limits and failure behaviour is data in this package; a hook that errors returns allow"
```

#### [TSK-06.3.2] Format formats and lints the edited file, and never refuses [P: H] [REFINEMENT]
```yaml
files: [internal/hooks/format.go, internal/hooks/format_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-06.3.1]
context:
  - "PostToolUse on a builder's edit: gofmt for Go, the repo's formatter for TypeScript, on that one file; lint output returns as context"
```

#### [TSK-06.3.3] Task checks refuse a builder's stop while a check fails, three times at most [P: H] [REFINEMENT]
```yaml
files: [internal/hooks/taskchecks.go, internal/hooks/taskchecks_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-06.3.1]
context:
  - "Stop runs the group's checks and refuses with the failing output; the limit is the host's stop-hook cap of 3; if the hook fails it allows, since Check reruns everything"
```

#### [TSK-06.3.4] Time warning at 80 percent of a session's time or turns [P: M] [REFINEMENT]
```yaml
files: [internal/hooks/timewarn.go, internal/hooks/timewarn_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-06.3.1]
context:
  - "PostToolUse in builders and lenses; it never refuses, and skips when it can't read the clock"
```

#### [TSK-06.3.5] Each role's plugin carries only its own hooks [P: H] [REFINEMENT]
```yaml
files: [internal/mount/claude/plugin.go, internal/mount/claude/plugin_test.go]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-06.3.1]
context:
  - "the guard in every session; format, task checks and time warning in the builder; time warning in lenses; the evidence and status hooks join in TG-07.5 and TG-08.4"
```

### [TG-06.4] Allow lists cover each stage, and the owner edits this repo on a branch
```yaml
type: feat
version: 1.0.0-alpha.7
depends_on: [TG-06.2]
```
* **Why:** headless runs used `bypassPermissions`, so the guard was the only wall. Proves REQ-38, REQ-40 and REQ-41's deny entries.

#### [TSK-06.4.1] Each role's settings allow what its stage needs, and dontAsk refuses the rest [P: C] [REFINEMENT]
```yaml
files: [komodo/roles/builder.md, komodo/roles/reviewer.md, internal/mount/claude/permissions.go, internal/mount/claude/permissions_test.go]
done_when:
  - go test ./internal/mount/claude/...
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#permissions
  - "roles name Komodo verbs and command classes; the mount turns them into allow and deny rules; the builder's list adds the repo's build, test, lint and format commands from detection"
  - "deny entries for docs/prd.md and eval/** in every line role (REQ-41)"
```

#### [TSK-06.4.2] Proof table: no allow-listed command is refused in any role [P: H] [REFINEMENT]
```yaml
files: [internal/mount/claude/allow_test.go]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-06.4.1]
context:
  - "for each role, every command its stage runs in a Go and a TypeScript repo passes both the rendered allow list and the guard (REQ-38)"
type: test
```

#### [TSK-06.4.3] The orchestrator may edit this repo's policy, rules and guard on a branch [P: H] [REFINEMENT]
```yaml
files: [komodo/policy.json, internal/guard/policy.go, internal/guard/policy_test.go]
done_when:
  - go test ./internal/guard/...
context:
  - docs/system-design.md#security
  - "config_paths keep bin/**, .git/config and .git/hooks from every session, and komodo/policy.json only from line sessions; a change applies after merge and rebuild (decision 0015)"
  - "test (REQ-40): an orchestrator edit to komodo/policy.json on feat/x is allowed; the same edit from a builder, or on main, is refused"
```

### [TG-06.5] Check reruns everything after every session
```yaml
type: feat
version: 1.0.0-alpha.7
```
* **Why:** police the output, not the input (architecture principle 2). Proves REQ-17 and REQ-36.

#### [TSK-06.5.1] Check reruns format, lint, the group's checks and scope [P: C] [REFINEMENT]
```yaml
files: [internal/check/check.go, internal/check/check_test.go]
done_when:
  - go test ./internal/check/...
context:
  - docs/system-design.md#build
  - "port the close station's reruns from internal/line/close.go and verify.go; scope fails an edit outside the group's files"
```

#### [TSK-06.5.2] Output checks catch model commits, changed refs, hooks and git config [P: C] [REFINEMENT]
```yaml
files: [internal/check/output.go, internal/check/output_test.go]
done_when:
  - go test ./internal/check/...
context:
  - docs/architecture.md#boundaries
  - "snapshot HEAD, every ref, .git/hooks and .git/config before a session and compare after; one test per case (REQ-36)"
```

#### [TSK-06.5.3] Changed lines are covered by tests [P: H] [REFINEMENT]
```yaml
files: [internal/check/coverage.go, internal/check/coverage_test.go]
done_when:
  - go test ./internal/check/...
context:
  - "Go: a cover profile of the touched packages, intersected with the diff's added lines; TypeScript: the repo's coverage command when it has one; the bar is a profile starting value the first eval calibrates"
```

#### [TSK-06.5.4] A secret scan runs over the added lines [P: H] [REFINEMENT]
```yaml
files: [internal/check/secrets.go, internal/check/secrets_test.go]
done_when:
  - go test ./internal/check/...
context:
  - "standard-library patterns for common keys and tokens, over added lines only; a test fixture per pattern"
```

#### [TSK-06.5.5] The conductor runs Check after every build and repair, and never reviews first [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-06.5.1, TSK-06.5.2, TSK-06.5.3, TSK-06.5.4]
context:
  - "test (REQ-17): the ledger records no review before Check passes"
```

### [TG-06.6] The forge credential stays with the conductor, and the sandbox holds
```yaml
type: feat
version: 1.0.0-alpha.7
depends_on: [TG-06.1]
```
* **Why:** a model session with forge push rights is the critical risk in the PRD. Proves REQ-26, REQ-33, REQ-34 and REQ-35.

#### [TSK-06.6.1] Every session starts from a scrubbed environment with no forge credential [P: C] [REFINEMENT]
```yaml
files: [internal/mount/claude/env.go, internal/mount/claude/env_test.go]
done_when:
  - go test ./internal/mount/claude/...
context:
  - docs/system-design.md#security
  - "port Scrub from internal/run/run.go; add the subprocess scrub spike S8 confirmed; the sandbox denies reads of the git credential store and gh's config"
  - "test (REQ-34): a session's environment and readable paths hold no forge token"
```

#### [TSK-06.6.2] Only Ship reads the forge credential, and it pushes only unprotected branches [P: C] [REFINEMENT]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go, internal/run/run.go]
done_when:
  - go test ./internal/line/... ./internal/run/...
context:
  - docs/system-design.md#prepare-and-ship
  - "the credential is read inside the push and handed to no other process; pushable refuses a critical ref; labels follow the push (REQ-26)"
```

#### [TSK-06.6.3] Line sessions run in the sandbox, and `komodo run` refuses without it [P: C] [REFINEMENT]
```yaml
files: [internal/mount/claude/sandbox.go, internal/mount/claude/sandbox_test.go, internal/preflight/preflight.go, internal/preflight/preflight_test.go]
done_when:
  - go test ./internal/mount/claude/... ./internal/preflight/...
context:
  - docs/system-design.md#cross-platform-macos-linux-windows
  - "sandboxSettings moves here and is on by default where the platform has one: fail if unavailable, no unsandboxed retry, a network allowlist without the forge; native Windows counts as no sandbox (TG-08.2)"
  - "test (REQ-35): a write outside the worktree fails; komodo run exits non-zero with the sandbox off"
```

#### [TSK-06.6.4] Doctor checks the forge ruleset and head-branch deletion [P: H] [REFINEMENT]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
context:
  - docs/system-design.md#health-checks
  - "--remote: a ruleset with no bypass actors on the default branch where the forge offers one, else a warning; delete head branches on merge; drafts available (REQ-33)"
```

---

## [EPIC-07] Phase 3: groups, review and repair
*Goal: a 3-group plan runs unattended to draft PRs. Ships as `1.0.0-alpha.8`.*

### [TG-07.1] The backlog is one file per group
```yaml
type: feat
version: 1.0.0-alpha.8
```
* **Why:** a 159 KB `BACKLOG.md` was the database, and ship rewrote it and lost a task's body (evidence 12). Proves REQ-8 and REQ-9's grammar.

#### [TSK-07.1.1] The parser reads group files in docs/backlog/ [P: C] [REFINEMENT]
```yaml
files: [internal/backlog/groupfile.go, internal/backlog/groupfile_test.go, internal/backlog/fuzz_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - docs/system-design.md#task-groups
  - docs/system-design.md#the-backlog
  - "a file is <group-id>-<slug>.md: a heading with priority and status, yaml with type, version, epic and depends_on, then checkbox tasks with files and optional accept and checks; a task needs only a title and files (REQ-9); fuzz the parser"
```

#### [TSK-07.1.2] Lint refuses a group over 12 tasks, and a light-tier builder [P: H] [REFINEMENT]
```yaml
files: [internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/...
depends_on: [TSK-07.1.1]
context:
  - "over 12 tasks is a problem that suggests a split (REQ-8); tier: light on a build task is a problem (REQ-30); the base rule and the context-anchor check still hold"
```

#### [TSK-07.1.3] `komodo backlog` lists open groups, and `komodo add` writes a group or task [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/backlog.go, cmd/komodo/backlog_test.go, internal/backlog/edit.go]
done_when:
  - go test ./cmd/komodo/... ./internal/backlog/...
depends_on: [TSK-07.1.1]
context:
  - docs/system-design.md#the-komodo-command
```

#### [TSK-07.1.4] The grammar, the planner and `komodo init` describe group files [P: H] [REFINEMENT]
```yaml
files: [komodo/rules/backlog.md, komodo/roles/planner.md, templates/project/BACKLOG.md.tmpl, templates/project/docs/backlog, cmd/komodo/init.go, cmd/komodo/init_test.go]
done_when:
  - test ! -f templates/project/BACKLOG.md.tmpl
  - go test ./cmd/komodo/...
  - go run ./cmd/komodo doctor
depends_on: [TSK-07.1.1]
context:
  - "init writes docs/backlog/ with one sample group file instead of BACKLOG.md; the planner writes group files"
type: docs
```

### [TG-07.2] Ingest compiles each group into a card
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.1]
```
* **Why:** builders spent 987 `grep` and 356 `sed` calls finding context the binary could pack (evidence 6). Proves REQ-7 and REQ-9's derived checks.

#### [TSK-07.2.1] `komodo ingest` compiles each READY group into a card with a stable hash [P: C] [REFINEMENT]
```yaml
files: [internal/ingest/card.go, internal/ingest/card_test.go, cmd/komodo/ingest.go]
done_when:
  - go test ./internal/ingest/... ./cmd/komodo/...
context:
  - docs/system-design.md#group-cards
  - "cards land in .komodo/queue/<group>.json; files expand globs and directories, and a new file is allowed where its parent exists; no session starts (REQ-7)"
```

#### [TSK-07.2.2] Checks are derived per language the group touches [P: C] [REFINEMENT]
```yaml
files: [internal/ingest/checks.go, internal/ingest/checks_test.go]
done_when:
  - go test ./internal/ingest/...
context:
  - "Go: build, vet and test of each touched package; TypeScript: the type check and the repo's test script; hand-written checks add, never replace (REQ-9); detection comes from internal/detect"
```

#### [TSK-07.2.3] Context packs replace exploration [P: H] [REFINEMENT]
```yaml
files: [internal/ingest/pack.go, internal/ingest/pack_test.go]
done_when:
  - go test ./internal/ingest/...
context:
  - docs/system-design.md#token-efficiency
  - "file bodies, signatures of imported packages, callers of changed symbols, neighbouring tests, the cited spec sections and the repo's rules; each item capped, then the total; Go through go/parser, TypeScript by a line scan"
tier: heavy
```

#### [TSK-07.2.4] Briefs fill their slots from the card, stable slots first [P: H] [REFINEMENT]
```yaml
files: [internal/line/brief.go, internal/line/brief_slots.go, internal/line/brief_test.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-07.2.1, TSK-07.2.3]
context:
  - docs/system-design.md#briefs
  - "test: the same card and tree give the same brief bytes"
```

### [TG-07.3] Coordinate schedules groups and paces to the plan
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.2]
```
* **Why:** a Haiku build averaged 75 turns (evidence 7), and a plan's usage window was a person's job to watch. Proves REQ-12, REQ-30 and REQ-32.

#### [TSK-07.3.1] Groups that share no file run in parallel, up to the plan's concurrency [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/schedule.go, internal/conductor/schedule_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - docs/system-design.md#parallelism
  - "overlap from the cards' files through internal/plan; starting values Pro 1, Max 5x 2, Max 20x 4, API 4; a group with depends_on waits for its parent's branch"
  - "test (REQ-12): groups sharing a file run one after another; groups sharing none overlap"
```

#### [TSK-07.3.2] The conductor pauses at a usage limit and resumes at the reset [P: H] [REFINEMENT]
```yaml
files: [internal/conductor/pace.go, internal/conductor/pace_test.go, internal/profile/profile.go]
done_when:
  - go test ./internal/conductor/... ./internal/profile/...
context:
  - docs/system-design.md#pacing-limits-and-loop-detection
  - "bound from the plan probe and rate_limit_event; unbound on API billing, with a spend budget; pauses and resumes go to events.jsonl"
  - "test (REQ-32): a simulated rate-limit event pauses the run and resumes it with no person"
```

#### [TSK-07.3.3] A Pro plan runs the economy profile, and no builder runs on the light tier [P: H] [REFINEMENT]
```yaml
files: [internal/profile/profile.go, internal/profile/profile_test.go, internal/line/snapshot.go, internal/line/snapshot_test.go, internal/doctor/doctor.go]
done_when:
  - go test ./internal/profile/... ./internal/line/... ./internal/doctor/...
depends_on: [TSK-07.3.2]
context:
  - docs/system-design.md#profiles-and-economy-mode
  - "BuilderTier and its file-count rule go; doctor rejects a profile whose builder is light; economy mode runs one group at a time and one combined lens (REQ-30)"
```

### [TG-07.4] The builder works the task list, and the conductor ticks it
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.2]
```
* **Why:** one builder per group, briefed with the whole task list, is one story's worth of work (decision 0007). Proves REQ-10.

#### [TSK-07.4.1] The builder role and build skill work a task list in order [P: C] [REFINEMENT]
```yaml
files: [komodo/roles/builder.md, komodo/roles/builder.schema.json, komodo/skills/build/SKILL.md]
done_when:
  - test -f komodo/skills/build/SKILL.md
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#build
  - docs/system-design.md#results
  - "the result per task: done or blocked, the checks run, and a question when blocked"
```

#### [TSK-07.4.2] `komodo check task|findings|scope` is one entry point for hooks and agents [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/check.go, cmd/komodo/check_test.go]
done_when:
  - go test ./cmd/komodo/...
context:
  - docs/system-design.md#the-komodo-command
  - "each subcommand calls internal/check; findings is what the evidence hook runs"
```

#### [TSK-07.4.3] The conductor ticks a task only after its checks pass, and edits nothing else [P: C] [REFINEMENT]
```yaml
files: [internal/backlog/tick.go, internal/backlog/tick_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - "ticks land on the group's branch; the only other write is adding or removing a blocker note"
  - "test (REQ-10): a person's edit to a task body survives a run unchanged"
```

### [TG-07.5] Review runs parallel lenses and blocks only on evidence
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.4]
```
* **Why:** TG-03.22 ran 11 review rounds because each re-reviewed the whole diff from scratch (evidence 3). Proves REQ-19, REQ-20 and REQ-21.

#### [TSK-07.5.1] Four checklist skills replace the one review skill [P: C] [REFINEMENT]
```yaml
files: [komodo/skills/review, komodo/skills/review-correctness/SKILL.md, komodo/skills/review-security/SKILL.md, komodo/skills/review-quality/SKILL.md, komodo/skills/review-economy/SKILL.md, komodo/roles/reviewer.md, komodo/roles/reviewer.schema.json]
done_when:
  - test ! -d komodo/skills/review
  - grep -q 'COR-5' komodo/skills/review-correctness/SKILL.md
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#review
  - "each skill lists its lens's rule IDs; a finding carries lens, rule ID, severity, file, line, evidence and a one-line fix"
```

#### [TSK-07.5.2] Validators measure before any lens runs [P: H] [REFINEMENT]
```yaml
files: [internal/review/validators.go, internal/review/validators_test.go]
done_when:
  - go test ./internal/review/...
context:
  - "tests and reproducers, the secret scan, a dependency audit and security linters when the repo has them, and caller counts of changed exported symbols; the report is settled fact in every lens's brief"
```

#### [TSK-07.5.3] Lenses run in parallel, or one combined lens in economy mode [P: C] [REFINEMENT]
```yaml
files: [internal/review/lenses.go, internal/review/lenses_test.go]
done_when:
  - go test ./internal/review/...
context:
  - "read-only sessions that see only the diff, the task list and the card, never the builder's transcript"
  - "test (REQ-19): the ledger shows three lens sessions in full mode and one in economy mode"
```

#### [TSK-07.5.4] A finding blocks only when the binary verifies its evidence [P: C] [REFINEMENT]
```yaml
files: [internal/review/evidence.go, internal/review/evidence_test.go]
done_when:
  - go test ./internal/review/...
context:
  - "a bug or security finding's reproducer fails on the current tree in a scratch copy; a convention finding cites a rule ID on a changed line; a performance or blast-radius finding has a validator's measurement; anything else becomes a PR note"
  - "one test per kind of evidence (REQ-20)"
tier: heavy
```

#### [TSK-07.5.5] The evidence hook refuses a lens's stop twice at most [P: H] [REFINEMENT]
```yaml
files: [internal/hooks/evidence.go, internal/hooks/evidence_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-07.5.4]
context:
  - "Stop runs komodo check findings and lists findings without evidence; after 2 refusals those findings become notes"
```

#### [TSK-07.5.6] A re-review resumes its lens, and can only close findings or flag repaired lines [P: C] [REFINEMENT]
```yaml
files: [internal/review/rereview.go, internal/review/rereview_test.go]
done_when:
  - go test ./internal/review/...
depends_on: [TSK-07.5.3]
context:
  - "test (REQ-21): a new finding on an unchanged line is dropped"
```

#### [TSK-07.5.7] The conductor runs Review through the lenses [P: H] [REFINEMENT]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-07.5.2, TSK-07.5.4, TSK-07.5.6]
```

### [TG-07.6] Repair resumes the builder, and a loop stops when it stops progressing
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.5]
```
* **Why:** TG-03.31's findings rose from 1 to 7 across fixes with no rule to stop them. Proves REQ-22 and REQ-23.

#### [TSK-07.6.1] Repair resumes the builder with a fix list of verified findings or failed checks [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/repair.go, internal/conductor/repair_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - docs/system-design.md#repair
  - "one checkbox per verified finding or failed check; a host without resume gets a fresh session with the fix list and the saved diff"
  - "test (REQ-22) on the repair brief"
```

#### [TSK-07.6.2] A round that closes nothing ends the loop, and the group ships as a draft with its findings [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/progress.go, internal/conductor/progress_test.go, internal/profile/profile.go]
done_when:
  - go test ./internal/conductor/... ./internal/profile/...
context:
  - docs/system-design.md#convergence-rules
  - "also stop on a repair that changed no file, a check failing identically after a repair, or a refusal limit; the fixed Repairs and ReviewRepairs counts leave the profile"
  - "test (REQ-23): no two rounds hold the same open findings"
```

### [TG-07.7] Escalations go to the orchestrator, and what it can't settle is written down
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.6]
```
* **Why:** a stuck group needs a decision, and an unattended run needs one without a person (decision 0011). Proves REQ-18 and REQ-45.

#### [TSK-07.7.1] An escalation reaches the orchestrator, which returns one allowed action [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/escalate.go, internal/conductor/escalate_test.go, komodo/roles/orchestrator.md, komodo/roles/orchestrator.schema.json]
done_when:
  - go test ./internal/conductor/...
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#escalations
  - "with a person present, the primary session gets it through komodo status and the status hook; unattended, one headless orchestrator session per escalation; the action is answer, split or clarify (which must pass lint), retry once on heavy, or stop"
```

#### [TSK-07.7.2] The escalate skill settles one escalation within its limits [P: H] [REFINEMENT]
```yaml
files: [komodo/skills/escalate/SKILL.md]
done_when:
  - test -f komodo/skills/escalate/SKILL.md
  - go run ./cmd/komodo doctor
context:
  - "an answer comes only from the task list, the specs and the code; anything that changes scope is a stop"
type: docs
```

#### [TSK-07.7.3] A blocked builder pauses its dependants and escalates [P: H] [REFINEMENT]
```yaml
files: [internal/conductor/blocked.go, internal/conductor/blocked_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-07.7.1]
context:
  - "test (REQ-18) on the conductor's decision; a group that stops twice without progress gets a blocker note, whatever the orchestrator says"
```

#### [TSK-07.7.4] What the orchestrator can't settle becomes a blocker note and a blocked draft PR [P: C] [REFINEMENT]
```yaml
files: [internal/backlog/note.go, internal/backlog/note_test.go, internal/conductor/stop.go, internal/conductor/stop_test.go]
done_when:
  - go test ./internal/backlog/... ./internal/conductor/...
depends_on: [TSK-07.7.1]
context:
  - docs/system-design.md#blocker-notes
  - "save a WIP commit, set BLOCKED, write the note under the group heading on its branch, and publish a draft PR labelled status: blocked; a headless run exits non-zero; komodo resume feeds the edited group to the resumed builder and removes the note"
  - "one test per path (REQ-45)"
```

#### [TSK-07.7.5] `komodo abandon` removes a group on purpose [P: M] [REFINEMENT]
```yaml
files: [internal/conductor/abandon.go, internal/conductor/abandon_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/conductor/... ./cmd/komodo/...
context:
  - "removes the group's worktree and branch, and marks its file BLOCKED with a note saying it was abandoned"
```

### [TG-07.8] Prepare, then ship draft-first
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.7]
```
* **Why:** unverified work looked ready, and a missing credential lost work at the last step. Proves REQ-24, REQ-25 and REQ-27.

#### [TSK-07.8.1] Prepare commits, runs the hooks and rebases, with conflicts as a repair round [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/prepare.go, internal/conductor/prepare_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - docs/system-design.md#prepare-and-ship
  - "the commit carries the ticked list and the CHANGELOG line, deletes the epic's files when it is the epic's last open group, and has no trailers"
  - "test (REQ-24): the ledger shows no push before these checks pass"
```

#### [TSK-07.8.2] Integration test-merges every ready group, and plans the stack [P: H] [REFINEMENT]
```yaml
files: [internal/conductor/integrate.go, internal/conductor/integrate_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-07.8.1]
context:
  - "a failure is a repair round for the group that caused it; a child of an unmerged parent targets the parent's branch, and is rebased and retargeted when the parent merges"
```

#### [TSK-07.8.3] Every PR opens as a draft, or labelled status: wip where drafts are unavailable [P: C] [REFINEMENT]
```yaml
files: [internal/pr/pr.go, internal/pr/pr_test.go, internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/pr/... ./internal/line/...
context:
  - "it turns ready for review only once every check and review passed; a test for each path (REQ-25)"
  - "internal/profile/profile.go maps to scope/agents, and a failed label call is a tested warning (from TSK-03.31.10 and TSK-03.31.12)"
```

#### [TSK-07.8.4] A missing or expired credential stops a group before Ship, and `komodo ship` finishes it [P: C] [REFINEMENT]
```yaml
files: [internal/conductor/ship.go, internal/conductor/ship_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/conductor/... ./cmd/komodo/...
context:
  - docs/system-design.md#run-failures
  - "the group keeps its commits and gets a blocker note; other groups continue; --no-ship stops each group before Ship (REQ-27)"
```

### [TG-07.9] Cleanup is mechanical
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.8]
```
* **Why:** stale runs and worktrees were cleared by hand after squash merges. Proves REQ-46.

#### [TSK-07.9.1] Ship and the next run remove merged and abandoned groups' leftovers [P: H] [REFINEMENT]
```yaml
files: [internal/conductor/cleanup.go, internal/conductor/cleanup_test.go, internal/doctor/prune.go]
done_when:
  - go test ./internal/conductor/... ./internal/doctor/...
context:
  - docs/system-design.md#the-backlog
  - "worktrees, local branches and sessions go; a squash-merged group whose branch is gone settles too (from TSK-03.32.8); only the last 10 run folders stay"
```

#### [TSK-07.9.2] `komodo sync` opens a cleanup PR for an epic whose files outlived it [P: M] [REFINEMENT]
```yaml
files: [internal/run/sync.go, internal/run/sync_test.go]
done_when:
  - go test ./internal/run/...
```

#### [TSK-07.9.3] Doctor names each kind of leftover [P: H] [REFINEMENT]
```yaml
files: [internal/doctor/leftovers.go, internal/doctor/leftovers_test.go]
done_when:
  - go test ./internal/doctor/...
context:
  - "an ended epic's files, and a worktree or branch with no group; one test per kind (REQ-46)"
```

### [TG-07.10] This repo moves to group files
```yaml
type: chore
version: 1.0.0-alpha.8
depends_on: [TG-07.9]
```
* **Why:** the line reads this file while it runs, so the orchestrator moves it once the new parser is merged and rebuilt.

#### [TSK-07.10.1] The open groups move into docs/backlog/, and BACKLOG.md goes [P: H] [REFINEMENT]
```yaml
files: [BACKLOG.md, docs/backlog, AGENTS.md, komodo/AGENTS.md, README.md, CONTRIBUTING.md]
done_when:
  - test ! -f BACKLOG.md
  - go run ./cmd/komodo lint
  - go run ./cmd/komodo doctor
context:
  - "AGENTS.md and komodo/AGENTS.md send out-of-task work to komodo add instead of a BACKLOG.md line"
owner: human
type: chore
```

#### [TSK-07.10.2] Proof: a 3-group plan runs unattended to draft PRs [P: H] [REFINEMENT]
```yaml
done_when:
  - go run ./cmd/komodo report
context:
  - "the phase exit; record the run ID and the three PRs"
owner: human
type: test
```

---

## [EPIC-08] Phase 4: install, platforms and eval
*Goal: the success criteria hold on macOS, Linux and Windows, and the owner cuts 1.0.0. Ships as `1.0.0-beta.2`; TG-08.8 is `1.0.0`.*

### [TG-08.1] Spike S6: Windows
```yaml
type: docs
version: 1.0.0-beta.2
```
* **Why:** decision 0017 puts native Windows first, which no run has tested (evidence 14).

#### [TSK-08.1.1] S6: the line runs natively on Windows 10 and 11 with Git for Windows, and in WSL2 [P: C] [REFINEMENT]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S6 result' docs/decisions.md
owner: human
type: docs
```

### [TG-08.2] Windows and the platform matrix
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.1]
```
* **Why:** on Windows a timeout killed only the parent and left orphans (evidence 14). Proves REQ-43.

#### [TSK-08.2.1] A timeout kills the whole process tree on Windows through a job object [P: C] [REFINEMENT]
```yaml
files: [internal/proc/process_windows.go, internal/proc/process_windows_test.go]
done_when:
  - GOOS=windows go vet ./internal/proc/...
  - go test ./internal/proc/...
context:
  - docs/system-design.md#cross-platform-macos-linux-windows
  - "standard library only: kernel32 through syscall.NewLazyDLL; the test runs on Windows and kills a child that outlives its parent (REQ-43)"
```

#### [TSK-08.2.2] Every command runs through POSIX sh, Git Bash's on native Windows [P: H] [REFINEMENT]
```yaml
files: [internal/proc/proc.go, internal/proc/proc_test.go]
done_when:
  - go test ./internal/proc/...
  - GOOS=windows go vet ./internal/proc/...
```

#### [TSK-08.2.3] Preflight knows the platform: no sandbox on native Windows, and WSL2 repos off /mnt/c [P: H] [REFINEMENT]
```yaml
files: [internal/preflight/platform.go, internal/preflight/platform_test.go]
done_when:
  - go test ./internal/preflight/...
context:
  - "native Windows runs without a sandbox and says so; inside WSL2 a repo under /mnt/ is refused, naming the Linux home as the fix"
```

#### [TSK-08.2.4] Releases build every platform's binary [P: H] [REFINEMENT]
```yaml
files: [internal/release/release.go, internal/release/release_test.go, internal/gate/gate.go]
done_when:
  - go test ./internal/release/... ./internal/gate/...
context:
  - "darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and windows/amd64, byte-identical per commit (decision 0002)"
```

### [TG-08.3] One command installs the line
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.2]
```
* **Why:** `komodo` is no one's command until it is installed, and manual steps drift (decision 0019). Proves REQ-1 and REQ-39's global render.

#### [TSK-08.3.1] install.sh installs on macOS, Linux and WSL2, and running it again updates [P: C] [REFINEMENT]
```yaml
files: [install.sh, internal/install/script_test.go]
done_when:
  - sh -n install.sh
  - go test ./internal/install/...
context:
  - docs/system-design.md#install
  - "name any missing prerequisite and how to get it; build with Go, or download the pinned release and verify its checksum; symlink onto PATH; komodo install; komodo init inside a repo; komodo doctor; the test runs it under a temp HOME"
```

#### [TSK-08.3.2] install.ps1 does the same on native Windows [P: C] [REFINEMENT]
```yaml
files: [install.ps1]
done_when:
  - test -f install.ps1
  - grep -q 'komodo install' install.ps1
context:
  - "a small wrapper on PATH instead of a symlink, since symlinks need admin rights"
```

#### [TSK-08.3.3] `komodo install` adds only the orchestrator layer to the global host config [P: H] [REFINEMENT]
```yaml
files: [internal/install/install.go, internal/install/install_test.go, internal/mount/claude/claude.go, internal/mount/claude/claude_test.go]
done_when:
  - go test ./internal/install/... ./internal/mount/claude/...
context:
  - docs/system-design.md#skills-and-scoping
  - "the guard hook, the orchestrator skills and the status hook; no builder, lens or standards skill; test (REQ-39) on the global render"
```

### [TG-08.4] The orchestrator drives the line from the primary session
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.3]
```
* **Why:** the primary session is the one place a person talks to the line (decision 0005). Proves REQ-39.

#### [TSK-08.4.1] The komodo skill is generated from `komodo help`, and the gate fails when it drifts [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/main.go, cmd/komodo/help.go, cmd/komodo/help_test.go, komodo/skills/komodo/SKILL.md]
done_when:
  - go test ./cmd/komodo/...
  - go run ./cmd/komodo doctor
```

#### [TSK-08.4.2] The plan and adhoc skills join run and escalate [P: H] [REFINEMENT]
```yaml
files: [komodo/skills/backlog, komodo/skills/plan/SKILL.md, komodo/skills/adhoc/SKILL.md]
done_when:
  - test ! -d komodo/skills/backlog
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#orchestrator-commands
  - "backlog becomes plan: /plan drafts groups through the planner, and they must pass lint; adhoc runs /build, /review and /ship through komodo stage"
type: docs
```

#### [TSK-08.4.3] `komodo stage` runs one stage ad hoc on a group or the current branch [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/stage.go, cmd/komodo/stage_test.go, internal/conductor/stage.go, internal/conductor/stage_test.go]
done_when:
  - go test ./cmd/komodo/... ./internal/conductor/...
context:
  - "the ledger records the ad hoc stage (REQ-39)"
```

#### [TSK-08.4.4] `komodo status` and the status hook show groups, time and blockers [P: H] [REFINEMENT]
```yaml
files: [internal/hooks/status.go, internal/hooks/status_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/hooks/... ./cmd/komodo/...
context:
  - "status --watch refreshes in place; the SessionStart hook adds the run's status and any blocked groups to the orchestrator's context"
```

### [TG-08.5] Plugin points ship disabled
```yaml
type: feat
version: 1.0.0-beta.2
```
* **Why:** Slack, Google Chat and cloud commands come later as plugins, not conductor changes (decision 0020). Proves REQ-42.

#### [TSK-08.5.1] A plugin is a manifest; all three types load disabled, and doctor lists them [P: H] [REFINEMENT]
```yaml
files: [internal/plugin/plugin.go, internal/plugin/plugin_test.go, internal/doctor/doctor.go]
done_when:
  - go test ./internal/plugin/... ./internal/doctor/...
context:
  - docs/system-design.md#plugins
  - "a manifest names its type, roles, stages and settings; enabling is per machine under ~/.komodo; a malformed manifest is a doctor problem"
```

#### [TSK-08.5.2] The conductor calls notifiers, tool packs and stage hooks once enabled [P: M] [REFINEMENT]
```yaml
files: [internal/conductor/plugins.go, internal/conductor/plugins_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-08.5.1]
context:
  - "a notifier gets blocker notes and run summaries and decides nothing; a tool pack adds commands to a role's allow list behind the guard; a stage hook runs before or after a stage and can stop the group with a reason"
```

### [TG-08.6] Releases publish, and product repos pin one
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.2]
```
* **Why:** product repos run a published release, never a local build (decision 0018). Proves REQ-5's second half and REQ-2's release pin.

#### [TSK-08.6.1] `komodo release` builds, tests, checksums and publishes a GitHub Release [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/release.go, internal/release/publish.go, internal/release/publish_test.go]
done_when:
  - go test ./internal/release/... ./cmd/komodo/...
context:
  - docs/system-design.md#binaries-and-releases
  - "runs from the owner's machine only; the forge credential is read by the release step alone, as at Ship"
```

#### [TSK-08.6.2] The release skill drives it from the orchestrator [P: M] [REFINEMENT]
```yaml
files: [komodo/skills/release/SKILL.md]
done_when:
  - test -f komodo/skills/release/SKILL.md
  - go run ./cmd/komodo doctor
context:
  - "the version bump and the changelog heading follow decision 0023"
type: docs
```

#### [TSK-08.6.3] Product repos run the pinned published release [P: H] [REFINEMENT]
```yaml
files: [internal/run/sync.go, internal/run/sync_test.go, internal/doctor/pins.go]
done_when:
  - go test ./internal/run/... ./internal/doctor/...
context:
  - "outside this repo, sync and install fetch the profile's pinned release and verify its checksum; doctor fails on any other"
```

### [TG-08.7] The golden suite and `komodo eval`
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.4]
```
* **Why:** a number decides readiness, never a model's score (decision 0021). Proves REQ-44, and the eval cases behind REQ-3, REQ-6, REQ-12, REQ-14, REQ-27, REQ-32, REQ-34 and REQ-40.

#### [TSK-08.7.1] The suite format and `komodo eval --list` [P: C] [REFINEMENT]
```yaml
files: [internal/eval/suite.go, internal/eval/suite_test.go, internal/eval/testdata, cmd/komodo/eval.go]
done_when:
  - go test ./internal/eval/... ./cmd/komodo/...
context:
  - docs/system-design.md#testing
  - "each golden group names its repo, pinned commit, group file and hidden tests; the real suite in eval/ is locked to line sessions, so tests use testdata"
```

#### [TSK-08.7.2] `komodo eval --runs N` runs each group in a fresh clone and reports per platform [P: C] [REFINEMENT]
```yaml
files: [internal/eval/run.go, internal/eval/run_test.go, internal/eval/report.go, internal/eval/report_test.go]
done_when:
  - go test ./internal/eval/...
depends_on: [TSK-08.7.1]
context:
  - "Ship becomes a local no-push, then the hidden tests run; the report holds pass rate, consistency, sessions, turns, tokens, minutes, review rounds and tokens per accepted group"
tier: heavy
```

#### [TSK-08.7.3] Eval cases for the requirements a unit test can't prove [P: H] [REFINEMENT]
```yaml
files: [internal/eval/cases.go, internal/eval/cases_test.go]
done_when:
  - go test ./internal/eval/...
depends_on: [TSK-08.7.2]
context:
  - "one case each: a failed preflight check per kind (REQ-6), kill and resume (REQ-14), the credential removed mid-run (REQ-27), a simulated rate limit (REQ-32), the canary (REQ-3), no forge token in a session (REQ-34), parallel and serial groups (REQ-12), and an owner-directed policy edit on a branch (REQ-40)"
```

#### [TSK-08.7.4] The golden suite: a Go repo and a TypeScript repo, 10 pinned groups each [P: C] [REFINEMENT]
```yaml
files: [eval/suite.json, eval/groups]
done_when:
  - go run ./cmd/komodo eval --list
context:
  - "the owner picks the repos; each group is a merged change rewound to its parent, its task list written from its intent, and its own tests hidden (REQ-44)"
owner: human
type: test
```

### [TG-08.8] 1.0.0 LTS
```yaml
type: chore
version: 1.0.0
depends_on: [TG-08.7]
```
* **Why:** 1.0.0 ships when all five success criteria hold (`docs/prd.md#success-criteria`).

#### [TSK-08.8.1] The install runs on macOS, Linux and Windows, and doctor exits 0 after each [P: C] [REFINEMENT]
```yaml
done_when:
  - go run ./cmd/komodo doctor
context:
  - "REQ-1's proof: record each platform's install in the release PR"
owner: human
type: test
```

#### [TSK-08.8.2] `komodo eval --runs 3` meets the success criteria on all three platforms [P: C] [REFINEMENT]
```yaml
done_when:
  - go run ./cmd/komodo eval --runs 3
context:
  - docs/prd.md#success-criteria
  - "includes a 12-group plan run unattended, and every requirement's proof exiting zero"
owner: human
type: test
```

#### [TSK-08.8.3] The owner settles the PRD's open questions [P: M] [REFINEMENT]
```yaml
files: [docs/prd.md]
context:
  - docs/prd.md#open-questions
  - "Q1, the 90 percent bars, and Q2, the dollar budget per run, from what eval measured"
owner: human
type: docs
```

#### [TSK-08.8.4] The owner cuts 1.0.0 [P: C] [REFINEMENT]
```yaml
done_when:
  - git rev-parse -q --verify refs/tags/v1.0.0
context:
  - "through the release skill, once TSK-08.8.1 to TSK-08.8.3 are done"
owner: human
type: chore
```

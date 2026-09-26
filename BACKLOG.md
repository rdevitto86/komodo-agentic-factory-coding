# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar is `komodo/rules/backlog.md`; run `komodo lint` after every edit.

Recreated on 2026-09-25 to deliver `docs/prd.md`. Each epic is one phase of `docs/system-design.md#rollout`, and every requirement is proven by at least one group. Earlier groups are history in `CHANGELOG.md` and git.

* **Phases run in order.** A later phase stays REFINEMENT until the phase before it has merged and its spikes have passed; then the orchestrator promotes its tasks to READY.
* **A group that shares no file with an open phase may run beside it,** as a targeted run in its own worktree. TG-07.1 and TG-07.2 run beside phase 1.
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

#### [TSK-04.1.1] Commands the line runs skip the guard [P: C] [DONE]
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

#### [TSK-04.1.2] The gate fails when it finds no build check [P: C] [DONE]
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

#### [TSK-04.1.3] cmd/komodo/gate.go:117 Gate in a line worktree ignores the root commands.json and refuses every commit [P: L] [DONE]
```yaml
files:
  - cmd/komodo/gate.go
done_when:
  - test -f cmd/komodo/gate.go
type: fix
context:
  - "Hooks run the gate with root = worktree (repoRoot stops at the .git file); .komodo/ is gitignored so commands.json and the detect cache are absent; a repo detection misses now fails every worktree commit although the main checkout configures verify, while QC reads the root's file. Have buildChecks also read the main checkout's .komodo/commands.json when running inside a linked worktree."
```

#### [TSK-04.1.4] cmd/komodo/gate_test.go:26 Toolkit gate test passes without the toolkit branch [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/gate_test.go
done_when:
  - test -f cmd/komodo/gate_test.go
type: test
context:
  - "The test writes go.mod, so generic detection yields 'go build ./... && go vet ./...' and a go test verify; both Contains checks pass even if the toolkitCheckout branch is removed. Assert on output only the toolkit branch produces, or drop go.mod and assert the exact go vet and go test check names."
```

#### [TSK-04.1.5] internal/line/verify.go:36 overrideCommands is a pass-through with an unused parameter [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/verify.go
done_when:
  - test -f internal/line/verify.go
type: refactor
context:
  - "It only returns repopkg.LoadCommands(root) and ignores its second argument, while VerifyCommand calls LoadCommands directly. Delete overrideCommands and call repopkg.LoadCommands(root) at its three call sites."
```




### [TG-04.2] Line endings are LF everywhere, and a pull rebuilds the binary
```yaml
type: feat
version: 1.0.0-alpha.5
```
* **Why:** no `.gitattributes` existed (evidence 14), and `bin/` went stale after every pull until someone rebuilt it by hand. Proves REQ-4 and REQ-5 for this repo.

#### [TSK-04.2.1] `.gitattributes` sets LF in the toolkit and in init's template [P: H] [DONE]
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

#### [TSK-04.2.2] Doctor fails a repo whose `.gitattributes` does not set LF [P: H] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
  - go run ./cmd/komodo doctor
depends_on: [TSK-04.2.1]
context:
  - "a missing file, or one whose * line lacks eol=lf, is a problem naming the line to add (REQ-4)"
```

#### [TSK-04.2.3] A pull, checkout or rebase rebuilds the binary when Go sources changed [P: C] [DONE]
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

#### [TSK-04.2.4] A build from a dirty tree is never stamped as built from HEAD [P: M] [DONE]
```yaml
files: [internal/run/sync.go, internal/run/sync_test.go]
done_when:
  - go test ./internal/run/...
depends_on: [TSK-04.2.3]
context:
  - "syncBinary and gate --rebuild share one stamp function; it skips the stamp when git status --porcelain reports tracked changes, and writes it only after the hooks install (from TSK-03.32.4)"
type: fix
```

#### [TSK-04.2.5] internal/run/sync.go:133 Post-merge hook stamps first, so syncBinary returns no built path and the drain launches a stale binary [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/sync.go
done_when:
  - test -f internal/run/sync.go
type: fix
context:
  - 'With the hooks from gate --install, syncRoot''s `git merge --ff-only upstream` fires post-merge in the main working tree. The hook runs `gate --rebuild`, which builds bin/komodo-<platform>, installs hooks and writes .built-from = upstream. syncBinary then reads a marker equal to head, prints ''binary: already current'' and returns "". drain (run.go:163) never sets executable. A drain started with `go run ./cmd/komodo run` then launches every group with its pre-pull temp binary. Before this diff syncBinary rebuilt and returned the bin path. The rebuild also goes unreported in sync output. Make syncBinary return the bin path when the marker is fresh because this sync''s own fast-forward stamped it, or skip the hook rebuild during sync.'
```

#### [TSK-04.2.6] internal/doctor/doctor.go:304 A tab-separated * line is reported as lacking eol=lf [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/doctor.go
done_when:
  - test -f internal/doctor/doctor.go
type: fix
context:
  - 'gitattributes accepts any whitespace between pattern and attributes, but the check requires the literal prefix "* ". A file holding `*\ttext=auto eol=lf` gets the ''add: * text=auto eol=lf'' problem, and the gate refuses a correctly configured repo. Split the trimmed line with strings.Fields and match a first field of "*" with any later field equal to eol=lf.'
```

#### [TSK-04.2.7] internal/doctor/doctor.go:298 Comment restates the loop and hedges [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/doctor.go
done_when:
  - test -f internal/doctor/doctor.go
type: docs
context:
  - `// Check if the file has a line with * pattern that sets eol=lf.` repeats what the loop below plainly does. The comments standard bans restating the code. Delete the comment.
```

#### [TSK-04.2.8] internal/gate/gate_test.go:486 Test doc comment cites a spec ID [P: L] [REFINEMENT]
```yaml
files:
  - internal/gate/gate_test.go
done_when:
  - test -f internal/gate/gate_test.go
type: docs
context:
  - '`proves REQ-5:` names a requirement ID. The comments standard bans citing a version, ticket, spec or PRD. Remove "REQ-5:" and keep only the behaviour sentence.'
```





### [TG-04.3] Every group cuts from `main`, or from a group it depends on
```yaml
type: fix
version: 1.0.0-alpha.5
```
* **Why:** TG-03.31 and TG-03.32 cut from a branch 39 commits ahead of `main`, and hand commits rode along unreviewed (evidence 13). Proves REQ-13.

#### [TSK-04.3.1] Lint rejects a base that is neither the default branch nor a dependency's branch [P: H] [DONE]
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

#### [TSK-04.3.2] A re-run cuts from its base, not from the group's stale remote branch [P: M] [DONE]
```yaml
files: [internal/line/worktree.go, internal/line/worktree_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "StartRef cuts from origin/<group branch> whenever that branch exists, so a group run again after an earlier push inherits old commits; cut from the base unless the run resumes that same group (from TSK-03.31.1)"
type: fix
```

#### [TSK-04.3.3] internal/line/worktree_test.go:526 Misleading and restating test comments [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/worktree_test.go
done_when:
  - test -f internal/line/worktree_test.go
type: docs
context:
  - "The comment says 'Create a second worktree and push to feat/a', but the next line cuts the first worktree and pushes nothing; lines 522-568 also restate each git call, and line 564 hedges with 'should'. Delete the step-by-step comments and keep at most one line describing the scenario."
```

#### [TSK-04.3.4] internal/line/worktree_test.go:520 Test passes on the pre-change code [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/worktree_test.go
done_when:
  - test -f internal/line/worktree_test.go
type: refactor
context:
  - 'Old AddWorktree resolved StartRef(root, "main") and never consulted origin/feat/a, so this test also passes on main; the WriteBrief test at line 613 is the real regression guard. Delete TestAddWorktreeCutsFromBaseNotFromStaleOrigin.'
```

#### [TSK-04.3.5] internal/backlog/backlog.go:157 BranchName still exists twice [P: L] [REFINEMENT]
```yaml
files:
  - internal/backlog/backlog.go
done_when:
  - test -f internal/backlog/backlog.go
type: refactor
context:
  - "line.BranchName at internal/line/worktree.go:49 remains and is used at next.go:235 and :378, so line and lint do not share one function as the task asked. Delete line.BranchName and call group.Branch() at both next.go sites."
```

#### [TSK-04.3.6] internal/line/worktree.go:55 Doc comment talks about callers [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/worktree.go
done_when:
  - test -f internal/line/worktree.go
type: docs
context:
  - "'the caller resolves startRef, preferring origin only where that freshness matters' talks about callers, which the comment rules ban. Cut the AddWorktree doc to one line saying what it does."
```

#### [TSK-04.3.7] internal/line/worktree.go:60 Pointless alias [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/worktree.go
done_when:
  - test -f internal/line/worktree.go
type: refactor
context:
  - "'start := startRef' only renames the parameter. Use startRef directly in the worktree add -b call."
```






### [TG-04.4] The README hands the design to the specs
```yaml
type: docs
version: 1.0.0-alpha.5
```
* **Why:** the README and the four specs describe the same parts twice, and the README's copy is the first line's design.

#### [TSK-04.4.1] README keeps setup, usage and names, and links the specs for design [P: M] [DONE]
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

#### [TSK-04.4.2] docs/system-design.md:201 Repo-layer paragraph names prototype skills as the founding orchestrator skills [P: L] [REFINEMENT]
```yaml
files:
  - docs/system-design.md
done_when:
  - test -f docs/system-design.md
type: fix
context:
  - "The sentence names run, review, backlog and respond as the four founding orchestrator skills that a repo cannot append to. The founding-skills table at #skills-and-scoping, which prd.md#product-scope cites, has no review or backlog skill, and respond belongs to the Responder. A builder given #the-repo-layer as context would lock down skills V1 does not have and leave komodo, plan, adhoc and escalate open to appends. Name the orchestrator skills from the #skills-and-scoping table, or cite that anchor instead of listing names."
```

#### [TSK-04.4.3] README.md:5 README says it describes the running line while its Design bullets describe the V1 target [P: L] [REFINEMENT]
```yaml
files:
  - README.md
done_when:
  - test -f README.md
type: fix
context:
  - "Line 5 was edited but still says the README describes the line as it runs today. Line 12 lists the V1 stages Ingest to Ship and line 17 says the guard holds five rules. The kept Usage, Names and Layout sections still describe komodo next, /review and today's eight-denial guard, so the README contradicts itself. Reword line 5 or line 9 to say the Design bullets link the V1 target rather than the running line."
```

#### [TSK-04.4.4] README.md:23 Hot swap bullet links a section that covers only model swaps [P: L] [REFINEMENT]
```yaml
files:
  - README.md
done_when:
  - test -f README.md
type: fix
context:
  - "The bullet says a skill or external dependency swaps without touching the line and links #profiles-and-economy-mode. That section covers only role-to-model mapping, and no spec section owns skill or dependency swapping. Cut the claim to model swaps, or link the section that owns skill and dependency swapping."
```

### [TG-04.5] Ship never conflicts, never files a finding out of place, and never ships unreviewed
```yaml
type: fix
version: 1.0.0-alpha.5
```
* **Why:** phase 0's four pull requests conflicted on `CHANGELOG.md` after every merge, #211's ship filed 3 findings under the next epic's heading, and TG-04.1 shipped with its review result missing. Parallel groups need all three fixed first.

#### [TSK-04.5.1] Ship writes a changelog fragment, and every reader folds the fragments in [P: C] [DONE]
```yaml
files: [internal/changelog, internal/line/ship.go, internal/line/ship_test.go, internal/line/wave_test.go, internal/release/release.go, internal/release/release_test.go, internal/gate/gate.go, cmd/komodo/release.go, cmd/komodo/main_test.go, docs/system-design.md, komodo/rules/backlog.md]
done_when:
  - go test ./internal/changelog/... ./internal/line/... ./internal/release/... ./internal/gate/... ./cmd/komodo/...
  - go run ./cmd/komodo doctor
context:
  - "ship writes changelog.d/<version>/<group-id>.md holding its one line, instead of editing CHANGELOG.md, so two open pull requests never touch the same file"
  - "internal/changelog folds the fragments under their version headings in SemVer order, creating a heading a fragment names; doctor, the gate's build version, release and tag all read through it"
  - "komodo release fold writes the fragments into CHANGELOG.md and deletes them, on any branch but the default, since nothing commits to main"
```

#### [TSK-04.5.2] A filed finding lands inside its group, never under the next epic's heading [P: H] [DONE]
```yaml
files: [internal/backlog/edit.go, internal/backlog/backlog_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - "AppendTask inserts before the next group heading, so for an epic's last group the task lands after the next epic's heading and goal; insert before the first ---, ## or ### line after the group heading instead"
  - "test: appending to the last group of an epic puts the task before the --- and ## lines that follow it"
```

#### [TSK-04.5.3] Ship refuses a group with no review result [P: H] [DONE]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go, cmd/komodo/cli_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "ReviewFindings reads a missing review result as no findings, so a group whose review file was moved or never written ships unreviewed; ship refuses unless HasResult(root, group+\"-review\") holds and the review is not stale"
  - "the refusal names the fix: run the review, then ship"
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

#### [TSK-05.1.1] S2: dontAsk, an allow list and the sandbox run a builder with no prompt and no refusal [P: C] [DONE]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S2 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.2] S3: a line-owned config directory shuts out the personal layer and keeps the login [P: C] [DONE]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S3 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.3] S4: the final stream event carries turns, usage, cost and a session ID resume accepts [P: C] [DONE]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S4 result' docs/decisions.md
context:
  - "keep the recorded stream as the fixture TSK-05.2.3 tests against"
owner: human
type: docs
```

#### [TSK-05.1.4] S5: schema output holds over a long builder session [P: H] [DONE]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S5 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.5] S7: `komodo run`, started inside an interactive session, launches its own sessions [P: C] [DONE]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S7 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-05.1.6] S8: what the max-turns and subprocess-scrub variables do in a headless session [P: H] [DONE]
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
base: fix/ship-never-conflicts-never-files-a-findi
depends_on: [TG-04.5]
```
* **Why:** the conductor starts, resumes, streams and stops sessions through one contract, so a host is one package (decision 0004). Proves the meter behind REQ-28.

#### [TSK-05.2.1] The host contract is one Go interface every mount implements [P: C] [DONE]
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

#### [TSK-05.2.2] Claude starts and resumes a role's session headless [P: C] [DONE]
```yaml
files: [internal/mount/claude/session.go, internal/mount/claude/session_test.go]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-05.2.1]
context:
  - docs/system-design.md#how-the-conductor-runs-a-claude-code-session
  - "argv: -p, --setting-sources project,local, --plugin-dir, --settings, --tools, --permission-mode dontAsk, --model, --effort, --strict-mcp-config, --output-format stream-json, --verbose, --json-schema, and --resume to resume; env CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=3, CLAUDE_CODE_MAX_TURNS per role, DISABLE_AUTOUPDATER=1, and GOCACHE and GOTMPDIR inside the worktree; no CLAUDE_CONFIG_DIR (decision 0025); --max-budget-usd on API billing"
  - "the process starts in the group's worktree in its own process group, and stop kills the tree; spikes S2 and S5 set the permission and schema flags"
  - "tests assert the argv and environment for the builder and for a lens"
  - "decision 0026: GOPATH and GOMODCACHE inside the worktree too, GOPROXY=off and GOFLAGS=-modcacherw, after the conductor runs go mod download outside the sandbox"
```

#### [TSK-05.2.3] The stream reports turns, usage, cost, rate limits and the session ID [P: C] [DONE]
```yaml
files: [internal/mount/claude/stream.go, internal/mount/claude/stream_test.go, internal/mount/claude/usage.go, internal/mount/claude/usage_test.go, internal/mount/claude/testdata]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-05.2.1]
context:
  - docs/system-design.md#run-state-and-metrics
  - "parse stream-json into the contract's stream, including rate_limit_event with five_hour, seven_day and resetsAt; the result event's totals are the meter, since summing transcripts logged 45,520,132 input tokens in 63 turns (evidence 8)"
  - "record fixtures as spike S4 did (decision 0025): one start and one resume stream, with local paths, session IDs and account fields replaced"
```

#### [TSK-05.2.4] internal/mount/host.go:49 Contract has no real implementer; only the test fake satisfies it [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/host.go
done_when:
  - test -f internal/mount/host.go
type: test
context:
  - "Claude, Codex and Ollama define no Start, Stop or Capabilities method. The Claude mount ships free functions Session and Parse instead. Nothing starts a process group or kills a process tree, and Codex and Ollama do not declare zero capabilities. `go test ./internal/mount/...` still passes, because only fakeHost is asserted against Contract. Add a `var _ mount.Contract` assertion for each mount, with Capabilities on Codex and Ollama and Start/Stop on Claude that run in their own process group."
```

#### [TSK-05.2.5] internal/mount/host.go:53 Contract methods that do I/O take no context.Context [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/host.go
done_when:
  - test -f internal/mount/host.go
type: fix
context:
  - "Preflight, Start, Resume, Stream and Stop spawn or signal processes, but none takes a ctx. The conductor cannot set a deadline on a hung CLI start or login check, or cancel it. Adding ctx later is a breaking change for every mount. Make `ctx context.Context` the first parameter of every Contract method that does I/O."
```

#### [TSK-05.2.6] internal/mount/claude/stream.go:75 Parse leaks its goroutine when the consumer stops reading [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/stream.go
done_when:
  - test -f internal/mount/claude/stream.go
type: fix
context:
  - "The out channel is unbuffered and Parse takes no ctx. Suppose a caller stops ranging after the first rate-limit event, or after a pause decision. The goroutine then blocks forever on `out <-` and keeps the process's stdout reader alive. Take a ctx and select on `ctx.Done()` around every send."
```

#### [TSK-05.2.7] internal/mount/claude/stream.go:62 Scanner errors are dropped, so a long line silently loses the result [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/stream.go
done_when:
  - test -f internal/mount/claude/stream.go
type: fix
context:
  - "Say a stream-json line exceeds streamLineCap (8 MiB), for example a large tool_result. Scan returns false, `scanner.Err()` is never checked, and the channel just closes. The result event after that line is never reported. The caller cannot tell this apart from a session that produced no result. Check `scanner.Err()` after the loop and report it, for example as an Err field on the final Event."
```

#### [TSK-05.2.8] internal/mount/claude/stream.go:45 Any resetsAt that is not an RFC3339 string drops the whole rate-limit event [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/stream.go
done_when:
  - test -f internal/mount/claude/stream.go
type: fix
context:
  - 'ResetsAt is decoded straight into time.Time. An input like "resetsAt":1758909600 (epoch seconds) makes Unmarshal fail for the whole line. Both utilisation values are then dropped without a trace. limits.go reads its reset field as a string. The field''s real type is not proven, since the test fixtures carry only RFC3339 strings. Decode resetsAt as json.RawMessage, parse it separately, and keep the utilisation values when the time fails to parse.'
```

#### [TSK-05.2.9] internal/mount/claude/session.go:26 An empty brief or resume input silently becomes the prompt "/" [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/session.go
done_when:
  - test -f internal/mount/claude/session.go
type: fix
context:
  - 'On a start with Brief "", or a resume with resumeInput "", Session sends "/" on stdin instead of refusing. This launches a paid, turn-consuming session with a meaningless prompt and hides the conductor''s bug. The substitution also carries no comment. Return an error from Session when the prompt it would send is empty.'
```

#### [TSK-05.2.10] internal/mount/claude/session.go:97 removeEnv keeps an entry whose value is empty [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/session.go
done_when:
  - test -f internal/mount/claude/session.go
type: fix
context:
  - "The check len(entry) <= len(prefix) keeps an entry exactly equal to the prefix. With CLAUDE_CONFIG_DIR= in the parent env, the variable survives, which violates decision 0025. With GOFLAGS= in the parent env, setEnv leaves a duplicate key. Replace the length-and-slice test with strings.HasPrefix(entry, prefix)."
```

#### [TSK-05.2.11] internal/mount/claude/session.go:85 toolNames hand-rolls strings.Join [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/session.go
done_when:
  - test -f internal/mount/claude/session.go
type: refactor
context:
  - 'The empty-check and += loop rebuild what strings.Join(names, ", ") already does. agentFile in claude.go already uses strings.Join for this same mapping. Return strings.Join(names, ", ") and drop the manual loop.'
```

#### [TSK-05.2.12] internal/mount/claude/stream.go:16 claude.Event duplicates mount.Event, plus one field [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/stream.go
done_when:
  - test -f internal/mount/claude/stream.go
type: refactor
context:
  - "claude.Event repeats mount.Event field for field and adds SessionID. Parse therefore does not produce the contract's stream type, so a converter will be needed. The contract's Event cannot carry the session ID that a resume needs. Add SessionID to mount.Event and have Parse emit <-chan mount.Event."
```

#### [TSK-05.2.13] internal/mount/claude/stream.go:103 Comments reason about other code and cite a decision [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/stream.go
done_when:
  - test -f internal/mount/claude/stream.go
type: docs
context:
  - 'Line 12 justifies the cap by pointing at sumTranscript. Line 29 cites "decision 0025". Lines 103-104 justify behaviour by what limits.go does. The comments standard bans citing a spec and reasoning about other code. Cut each comment to what its own code does, dropping the decision number and the references to other functions.'
```

#### [TSK-05.2.14] internal/mount/host.go:40 Doc comments cite "decision 0004" [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/host.go
done_when:
  - test -f internal/mount/host.go
type: docs
context:
  - "The Capabilities comment (line 40) and the Contract comment (line 49) cite a decision number, which the comments standard bans. Remove the decision citations from both doc comments."
```

#### [TSK-05.2.15] internal/mount/host_test.go:86 Comment names an identifier that does not exist, above an unused field [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/host_test.go
done_when:
  - test -f internal/mount/host_test.go
type: docs
context:
  - "The comment names asContract, but the line below is a blank var _ Contract assertion. Separately, fakeHost.next (line 21) is incremented in Start but never read. Reword the comment to describe the compile-time assertion, and delete the unused next field."
```













### [TG-05.3] Pinned, hermetic, role-scoped sessions
```yaml
type: feat
version: 1.0.0-alpha.6
base: feat/the-host-contract
depends_on: [TG-05.2]
```
* **Why:** three machines ran three different agents (evidence 10). Proves REQ-2, REQ-3 and REQ-16.

#### [TSK-05.3.1] Profiles pin the host version, full model IDs and effort per role [P: H] [DONE]
```yaml
files: [internal/profile/profile.go, internal/profile/profile_test.go, komodo/profiles]
done_when:
  - go test ./internal/profile/...
context:
  - docs/system-design.md#profiles-and-economy-mode
  - docs/system-design.md#sessions-pinned-and-hermetic
  - "full and economy profiles as JSON under komodo/profiles, which the komodo embed carries; models are full IDs such as claude-sonnet-5 and claude-opus-5-5, never an alias"
```

#### [TSK-05.3.2] Line sessions load no personal layer and keep the login [P: C] [DONE]
```yaml
files: [internal/mount/claude/config.go, internal/mount/claude/config_test.go]
done_when:
  - go test ./internal/mount/claude/...
context:
  - docs/system-design.md#sessions-pinned-and-hermetic
  - "decision 0025: the default config directory keeps the login; --setting-sources project,local and --strict-mcp-config shut out personal CLAUDE.md, settings, plugins, agents and MCP; the role settings turn auto-memory off"
  - "doctor reads a session's init event and fails on any plugin, agent or MCP server that is neither built in nor the role's"
  - "test (REQ-3): a canary line in a fake personal config never appears in the rendered directory or a session's argv"
```

#### [TSK-05.3.3] Each role runs with its own plugin: its skills, hooks and agents only [P: H] [DONE]
```yaml
files: [internal/mount/claude/plugin.go, internal/mount/claude/plugin_test.go]
done_when:
  - go test ./internal/mount/claude/...
context:
  - docs/system-design.md#skills-and-scoping
  - "one plugin directory per role; the builder's holds the build skill and the standards for the languages the group touches, never a review, planning or orchestrator skill"
  - "test (REQ-16): the rendered builder plugin lists exactly those skills"
```

#### [TSK-05.3.4] Doctor fails when any pin differs [P: H] [DONE]
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

#### [TSK-05.3.5] internal/mount/claude/session.go:31 Line sessions lose the universal rules [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/session.go
done_when:
  - test -f internal/mount/claude/session.go
type: fix
context:
  - "--setting-sources local stops CLAUDE.md loading, which drops its @.claude/komodo/AGENTS.md import; the brief carries only the repo's root AGENTS.md (brief_slots.go:17), so Git, scope and comment rules vanish from every line session. Carry the rendered universal rules in a brief slot or the role plugin."
```

#### [TSK-05.3.6] internal/mount/claude/plugin.go:20 No build skill ships, so the builder plugin never holds one [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/plugin.go
done_when:
  - test -f internal/mount/claude/plugin.go
type: fix
context:
  - "komodo/skills has no build skill; RenderBuilderPlugin skips it silently, and the tests pass only because their fixtures invent one. Ship komodo/skills/build/SKILL.md and test against the real LoadSkills set."
```

#### [TSK-05.3.7] internal/mount/claude/plugin.go:29 Hard-coded language map misses real standards [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/plugin.go
done_when:
  - test -f internal/mount/claude/plugin.go
type: fix
context:
  - "detect never emits C or C++; standards-javascript, -ruby and -php do not exist; Zig, Swift, Kotlin, shell and cross-cutting standards never reach the builder; this repeats mount.SelectStandards glob logic. Build the plugin's standards from the list mount.SelectStandards already returns."
```

#### [TSK-05.3.8] internal/mount/claude/plugin.go:66 isForcedStandard always returns false [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/plugin.go
done_when:
  - test -f internal/mount/claude/plugin.go
type: fix
context:
  - "Repo standards overrides never reach the builder plugin; the comment hedges and wrongly says no override source exists, though mount.forcedStandards and repopkg.LoadStandards do. Delete isForcedStandard and include the standards mount.forcedStandards names."
```

#### [TSK-05.3.9] internal/mount/claude/plugin.go:72 Doc claims the caller omits returned skills from the shared directory [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/plugin.go
done_when:
  - test -f internal/mount/claude/plugin.go
type: docs
context:
  - "claude.go:78 throws the return away and still renders every skill into .claude/skills. Drop the return value and that sentence, or make Render use it."
```

#### [TSK-05.3.10] internal/doctor/pins.go:80 Model-ID pin misses the reviewer tier [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/pins.go
done_when:
  - test -f internal/doctor/pins.go
type: fix
context:
  - "The check walks profile role names (correctness, security...) that do not match komodo/roles, and Tiers.Machine has no reviewer case; an overlay models.reviewer of opus passes doctor while reviews run that alias via Tiers.Reviewer (next.go:288). Check every tier's model directly, including Tiers.Reviewer."
```

#### [TSK-05.3.11] internal/profile/profile.go:189 withMode swallows the profile load error [P: L] [REFINEMENT]
```yaml
files:
  - internal/profile/profile.go
done_when:
  - test -f internal/profile/profile.go
type: fix
context:
  - "A malformed komodo/profiles JSON, or a disk komodo/ without profiles/, leaves Roles nil and silently turns off the doctor model-ID check. Return the load error or report it as a doctor problem."
```

#### [TSK-05.3.12] internal/doctor/pins_test.go:149 Stale-binary test passes only through a git error [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/pins_test.go
done_when:
  - test -f internal/doctor/pins_test.go
type: test
context:
  - The zero OID makes git diff fail; no test covers a real .go change being flagged or a docs-only commit passing. Add .go and docs-only commits and assert each checkRelease outcome.
```

#### [TSK-05.3.13] internal/mount/claude/config_test.go:14 Canary tests pass whatever Session does [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/config_test.go
done_when:
  - test -f internal/mount/claude/config_test.go
type: test
context:
  - "Session never reads HOME, so the canaries cannot leak; REQ-3's rendered-directory half is untested. Run Render with a canary personal config under a temp HOME and assert no planned file contains it."
```

#### [TSK-05.3.14] internal/mount/claude/config.go:6 Unused constants with a stale value [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/config.go
done_when:
  - test -f internal/mount/claude/config.go
type: refactor
context:
  - "All three constants are unused, and settingSourcesFlag says project,local while session.go:31 passes local. Delete config.go or make Session use the constants with the correct value."
```











### [TG-05.4] The conductor drives the stages
```yaml
type: feat
version: 1.0.0-alpha.6
base: feat/pinned-hermetic-role-scoped-sessions
depends_on: [TG-05.3]
```
* **Why:** a model relayed `komodo step` and its tokens were never metered (evidence 1). Proves REQ-6, REQ-11, REQ-14, REQ-15 and REQ-31.

#### [TSK-05.4.1] Preflight checks the login, the forge credential, the sandbox and the budget [P: H] [READY]
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

#### [TSK-05.4.2] Group states live in state.json, written before each state's work [P: C] [READY]
```yaml
files: [internal/conductor/state.go, internal/conductor/state_test.go, internal/conductor/fuzz_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - docs/system-design.md#group-states
  - docs/system-design.md#run-state-and-metrics
  - "a pure function from disk state to the next action, ported from Next in internal/line/snapshot.go with its fuzz test (decision 0001)"
```

#### [TSK-05.4.3] The conductor runs every stage itself, and models only build, review and repair [P: C] [READY]
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

#### [TSK-05.4.4] `komodo run` runs the conductor, not a model relaying stages [P: C] [READY]
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

#### [TSK-05.4.5] The run skill launches and watches, and never relays a stage [P: H] [READY]
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

#### [TSK-05.4.6] A killed run resumes without repeating a session or losing an edit [P: C] [READY]
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

#### [TSK-05.4.7] A run never rebuilds the binary it is running [P: H] [READY]
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
base: feat/the-conductor-drives-the-stages
depends_on: [TG-05.4]
```
* **Why:** one build took 122 turns with no cap but a 90-minute group budget (evidence 9). Proves REQ-28, REQ-29 and REQ-31.

#### [TSK-05.5.1] Each run writes metrics.jsonl and events.jsonl from the host's own totals [P: H] [DONE]
```yaml
files: [internal/ledger/ledger.go, internal/ledger/ledger_test.go]
done_when:
  - go test ./internal/ledger/...
context:
  - docs/system-design.md#run-state-and-metrics
  - "one line per stage and session: run, group, stage, start, duration, turns, input, output and cached tokens, cost, outcome; each run starts fresh, and only the last 10 run folders stay"
```

#### [TSK-05.5.2] `komodo report` sums a run, and its sums match the host's [P: H] [DONE]
```yaml
files: [internal/line/report.go, internal/line/report_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/line/... ./cmd/komodo/...
depends_on: [TSK-05.5.1]
context:
  - "test (REQ-28): a recorded stream's result totals equal the report's line for that session"
  - "the headline figure is tokens per accepted group"
```

#### [TSK-05.5.3] A group has 60 minutes, and each session its own limit [P: C] [DONE]
```yaml
files: [internal/conductor/clock.go, internal/conductor/clock_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - docs/system-design.md#pacing-limits-and-loop-detection
  - "starting values in the profile: build 25, lens 8, repair 10, re-review 5 minutes; at a limit the conductor kills the session's tree, saves a WIP commit and stops the group; TG-07.7 makes the stop an escalation"
  - "test (REQ-29): a fake session past its limit is killed, and the group stops by 60 minutes"
```

#### [TSK-05.5.4] Proof: one group runs through the conductor [P: H] [READY]
```yaml
done_when:
  - go run ./cmd/komodo report
context:
  - "the phase exit: promote TG-06.2, run it with komodo run, and confirm a PR within 60 minutes with zero tokens outside build, review and repair sessions; record the run ID in the PR"
owner: human
type: test
```

#### [TSK-05.5.5] internal/ledger/ledger.go:203 WriteMetrics/WriteEvents have no caller and no retention [P: L] [REFINEMENT]
```yaml
files:
  - internal/ledger/ledger.go
done_when:
  - test -f internal/ledger/ledger.go
type: fix
context:
  - "Nothing outside the tests calls WriteMetrics or WriteEvents, so no real run ever writes metrics.jsonl or events.jsonl; both also append forever to one flat file under .komodo, breaking the spec's requirement that each run starts fresh and only the last 10 run folders stay. Write both files into a per-run folder truncated when the run starts, prune to the newest 10, and call this from the run's close path."
```

#### [TSK-05.5.6] internal/conductor/clock.go:21 Session limits map uses "review" instead of the profile's "lens" name [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/clock.go
done_when:
  - test -f internal/conductor/clock.go
type: fix
context:
  - 'The sessionLimits map key is "review", but the spec and profile name this role "lens"; a session started with type "lens" has no entry in sessionLimits, so SessionPastLimit always returns false for it and that session type is never limited. The limits are also hardcoded rather than read from the profile. Load per-type limits from the profile under its actual role names, and treat an unknown type as an error or a default limit instead of silently no limit.'
```

#### [TSK-05.5.7] internal/conductor/clock.go:5 Clock has no synchronization for concurrent access [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/clock.go
done_when:
  - test -f internal/conductor/clock.go
type: fix
context:
  - "Clock keeps three maps (sessionUsed, sessionStarted, sessionType) with no mutex; a conductor polling GroupUsed while another goroutine calls StartSession or EndSession triggers Go's fatal concurrent map read/write error, not just a logic bug. Guard every Clock method with a sync.Mutex."
```

#### [TSK-05.5.8] internal/ledger/ledger.go:77 isEvent still misses stop outcomes despite Event's own doc [P: L] [REFINEMENT]
```yaml
files:
  - internal/ledger/ledger.go
done_when:
  - test -f internal/ledger/ledger.go
type: fix
context:
  - "Event's doc comment says it records stops, but isEvent only accepts escalated, paused, and resumed, so a clock-driven stop is never written to events.jsonl. Add the stop outcome to the outcome-to-type mapping and cover it in a test."
```

#### [TSK-05.5.9] internal/line/report.go:60 groupTokens comment understates what it sums [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/report.go
done_when:
  - test -f internal/line/report.go
type: docs
context:
  - "The comment says groupTokens sums build and repair sessions, but the code sums every non-brief station in the run file, including review, fix, machine, close, qc, and ship. Reword the comment to state it sums every run-file entry for the group except brief stamps."
```

#### [TSK-05.5.10] internal/line/report_test.go:31 REQ-28 test never exercises a real recorded stream [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/report_test.go
done_when:
  - test -f internal/line/report_test.go
type: test
context:
  - 'The test for "a recorded stream''s totals equal the report''s line" stamps a hand-built ledger entry directly; no recorded host stream goes through the mount''s usage-parsing path, so a stream-parsing regression would still pass this test. Feed a recorded host result stream through the mount''s usage path, then compare its totals to report.Tokens.'
```

#### [TSK-05.5.11] internal/ledger/ledger_test.go:341 WriteEvents test only checks a nonzero count [P: L] [REFINEMENT]
```yaml
files:
  - internal/ledger/ledger_test.go
done_when:
  - test -f internal/ledger/ledger_test.go
type: test
context:
  - 'TestWriteEventsCreatesFile asserts only len(events) > 0, so it would still pass if the done entry leaked in as an event, if Type were wrong, or if paused/resumed handling broke. Assert exactly one event with Type == "escalation", and add table cases for paused and resumed.'
```

#### [TSK-05.5.12] internal/ledger/ledger.go:73 Metric.Cost is always empty [P: L] [REFINEMENT]
```yaml
files:
  - internal/ledger/ledger.go
done_when:
  - test -f internal/ledger/ledger.go
type: refactor
context:
  - "Metric.Cost is never set because Entry has no cost field, so every metrics line omits the spec's cost column. Carry cost on Entry from the host totals and copy it in aggregateMetrics, or drop the field."
```

#### [TSK-05.5.13] internal/ledger/ledger.go:253 WriteEvents/ReadEvents duplicate WriteMetrics/ReadMetrics [P: L] [REFINEMENT]
```yaml
files:
  - internal/ledger/ledger.go
done_when:
  - test -f internal/ledger/ledger.go
type: refactor
context:
  - "WriteEvents and ReadEvents duplicate the same append/scan logic as WriteMetrics/ReadMetrics, which in turn duplicate Read; each write reopens the file per line and ignores the Close error. Use one generic append-lines helper that opens once and checks Close, plus one generic JSONL reader."
```

#### [TSK-05.5.14] internal/ledger/ledger.go:584 extractEvents repeats isEvent's outcome checks [P: L] [REFINEMENT]
```yaml
files:
  - internal/ledger/ledger.go
done_when:
  - test -f internal/ledger/ledger.go
type: refactor
context:
  - "extractEvents calls isEvent and then repeats the same three outcome comparisons in an if/else chain to set Type. Use one map[outcome]type lookup for both the filter and the type."
```

#### [TSK-05.5.15] internal/line/report.go:69 Redundant Run != "" check [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/report.go
done_when:
  - test -f internal/line/report.go
type: refactor
context:
  - 'entry.Run != "" can never be false when reading the run file, since Stamp routes Run-less entries to adhoc.jsonl instead. Drop the redundant condition.'
```

#### [TSK-05.5.16] internal/conductor/clock_test.go:13 Tests sleep and poke private fields instead of injecting time [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/clock_test.go
done_when:
  - test -f internal/conductor/clock_test.go
type: refactor
context:
  - "Tests sleep 100ms and write private maps (sessionStarted, groupUsed) directly because Clock calls time.Now itself with no seam to control it. Inject a now func() time.Time and drive the tests through the exported surface."
```

#### [TSK-05.5.17] internal/conductor/clock_test.go:149 Comment and test name cite a requirement number and overstate behavior [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/clock_test.go
done_when:
  - test -f internal/conductor/clock_test.go
type: docs
context:
  - 'The comment and test name cite REQ-29 and say "IsKilled", but nothing is actually killed; the same requirement-citation style appears at internal/line/report_test.go:29 (REQ-28). Remove the requirement IDs and name the test for what it actually asserts.'
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

#### [TSK-06.1.1] S1 on macOS: Go builds, module downloads and race tests pass under the sandbox [P: C] [DONE]
```yaml
files: [docs/decisions.md]
done_when:
  - grep -q 'Spike S1 result' docs/decisions.md
owner: human
type: docs
```

#### [TSK-06.1.2] S1 on WSL2 [P: H] [DONE]
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
base: fix/ship-never-conflicts-never-files-a-findi
depends_on: [TG-04.5]
```
* **Why:** a 159 KB `BACKLOG.md` was the database, and ship rewrote it and lost a task's body (evidence 12). Proves REQ-8 and REQ-9's grammar.

#### [TSK-07.1.1] The parser reads group files in docs/backlog/ [P: C] [READY]
```yaml
files: [internal/backlog/groupfile.go, internal/backlog/groupfile_test.go, internal/backlog/fuzz_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - docs/system-design.md#task-groups
  - docs/system-design.md#the-backlog
  - "a file is <group-id>-<slug>.md: a heading with priority and status, yaml with type, version, epic and depends_on, then checkbox tasks with files and optional accept and checks; a task needs only a title and files (REQ-9); fuzz the parser"
```

#### [TSK-07.1.2] Lint refuses a group over 12 tasks, and a light-tier builder [P: H] [READY]
```yaml
files: [internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/...
depends_on: [TSK-07.1.1]
context:
  - "over 12 tasks is a problem that suggests a split (REQ-8); tier: light on a build task is a problem (REQ-30); the base rule and the context-anchor check still hold"
```

#### [TSK-07.1.3] `komodo backlog` lists open groups, and `komodo add` writes a group or task [P: H] [READY]
```yaml
files: [cmd/komodo/backlog.go, cmd/komodo/backlog_test.go, internal/backlog/edit.go]
done_when:
  - go test ./cmd/komodo/... ./internal/backlog/...
depends_on: [TSK-07.1.1]
context:
  - docs/system-design.md#the-komodo-command
```

#### [TSK-07.1.4] The grammar, the planner and `komodo init` describe group files [P: H] [READY]
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
base: feat/the-backlog-is-one-file-per-group
depends_on: [TG-07.1]
```
* **Why:** builders spent 987 `grep` and 356 `sed` calls finding context the binary could pack (evidence 6). Proves REQ-7 and REQ-9's derived checks.

#### [TSK-07.2.1] `komodo ingest` compiles each READY group into a card with a stable hash [P: C] [READY]
```yaml
files: [internal/ingest/card.go, internal/ingest/card_test.go, cmd/komodo/ingest.go]
done_when:
  - go test ./internal/ingest/... ./cmd/komodo/...
context:
  - docs/system-design.md#group-cards
  - "cards land in .komodo/queue/<group>.json; files expand globs and directories, and a new file is allowed where its parent exists; no session starts (REQ-7)"
```

#### [TSK-07.2.2] Checks are derived per language the group touches [P: C] [READY]
```yaml
files: [internal/ingest/checks.go, internal/ingest/checks_test.go]
done_when:
  - go test ./internal/ingest/...
context:
  - "Go: build, vet and test of each touched package; TypeScript: the type check and the repo's test script; hand-written checks add, never replace (REQ-9); detection comes from internal/detect"
```

#### [TSK-07.2.3] Context packs replace exploration [P: H] [READY]
```yaml
files: [internal/ingest/pack.go, internal/ingest/pack_test.go]
done_when:
  - go test ./internal/ingest/...
context:
  - docs/system-design.md#token-efficiency
  - "file bodies, signatures of imported packages, callers of changed symbols, neighbouring tests, the cited spec sections and the repo's rules; each item capped, then the total; Go through go/parser, TypeScript by a line scan"
tier: heavy
```

#### [TSK-07.2.4] Briefs fill their slots from the card, stable slots first [P: H] [READY]
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

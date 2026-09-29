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


### [TG-04.6] The relay line stops cleanly when a group does not ship
```yaml
type: fix
version: 1.0.0-alpha.5
```
* **Why:** running phase 1 in two lanes on 2026-09-26 found three relay-line defects; each cost a stopped lane or a hand repair.

#### [TSK-04.6.1] A targeted run exits non-zero when its group ends unshipped [P: M] [REFINEMENT]
```yaml
files: [internal/run/run.go, internal/run/run_test.go]
done_when:
  - go test ./internal/run/...
context:
  - "launchTarget returns the host's exit code, which is 0 when the session stops blocked, so a lane moved on to TG-05.4 with no base to cut from; exit non-zero unless the ledger records the group's ship, as drain already checks"
```

#### [TSK-04.6.2] A failed ship leaves nothing staged [P: M] [REFINEMENT]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "TG-07.2's ship staged BACKLOG.md and its changelog fragment, then its gate failed; the next merge into the branch refused until they were cleared by hand; unstage ship's own outputs when a step after staging fails"
```

#### [TSK-04.6.3] A line session writes only inside its worktree [P: H] [REFINEMENT]
```yaml
files: [internal/guard]
done_when:
  - go test ./internal/guard/...
context:
  - "a TG-05.3 builder left draft plugin.go and plugin_test.go in the root checkout, which failed the next pre-push gate; phase 2's sandbox (TG-06.6) may settle this, so confirm there before adding a guard rule, since the guard is frozen until TG-06.2"
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
  - go test ./internal/install/...
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

#### [TSK-05.2.10] internal/mount/claude/session.go:97 removeEnv keeps an entry whose value is empty [P: L] [DONE]
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

#### [TSK-05.4.6] A killed run resumes without repeating a session or losing an edit [P: C] [READY]
```yaml
files: [internal/conductor/resume.go, internal/conductor/resume_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/conductor/... ./cmd/komodo/...
depends_on: [TSK-05.4.3]
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
context:
  - docs/system-design.md#sessions-pinned-and-hermetic
  - "sync runs before a run starts and after it ends, never between groups; the rebuild hook skips while a run holds the lock; a sync failure names its step (from TSK-03.32.5)"
  - "test (REQ-15): a stale build marker during a run changes nothing until the run ends"
```

### [TG-05.6] Claude implements the host contract, and `komodo run` drives the conductor
```yaml
type: feat
version: 1.0.0-alpha.6
base: feat/the-conductor-drives-the-stages
depends_on: [TG-05.4]
```
* **Why:** TG-05.4 built the conductor, but no mount implements the host contract, so nothing can start a real session through it; TSK-05.4.4 and TSK-05.4.5 moved here; after TSK-05.6.2 closed with no change, they move again to TG-05.9.

#### [TSK-05.6.1] The Claude mount implements the host contract [P: C] [DONE]
```yaml
files: [internal/mount/claude/contract.go, internal/mount/claude/contract_test.go, internal/mount/claude/testdata]
done_when:
  - go test ./internal/mount/claude/...
context:
  - docs/system-design.md#the-host-contract
  - docs/system-design.md#how-the-conductor-runs-a-claude-code-session
  - "Start runs claude with Session's argv and environment in the worktree, in its own process group; Stream parses stdout with Parse; Result is the result event's structured_output; Stop kills the process group; Resume passes --resume with the session ID; Preflight checks HostVersion and LoggedIn; Capabilities declares resume, sandbox, hooks and structured output"
  - "tests run a fake claude script on PATH that replays recorded start and resume streams (decision 0025), with local paths and account fields replaced"
```

### [TG-05.7] Every epic has a draft branch, and group PRs stack on it
```yaml
type: feat
version: 1.0.0-alpha.6
```
* **Why:** every group opened its own PR against `main`, and the owner approved each by hand; on 2026-09-26 that was 14 PRs in one afternoon. An epic branch collects its groups' PRs, and a person merges once per epic.

#### [TSK-05.7.1] Decision 0028 and REQ-13 say where every branch cuts from and where every PR lands [P: C] [DONE]
```yaml
files: [docs/decisions.md, docs/prd.md, docs/system-design.md]
done_when:
  - grep -q '^## 0028' docs/decisions.md
  - grep -q 'epic branch' docs/prd.md
context:
  - "the owner's words, 2026-09-26: each epic has a branch named feat/v<its version>, cut from main and opened as a draft PR to main; every group PR targets its epic's branch, or stacks on the group it depends on; people review and merge at the final state, the epic's PR"
  - "the conductor merges a reviewed, checked group PR into its epic branch; only a person merges an epic PR into main"
  - "a group PR holds at most 20 files and 2,000 changed lines, and 1,000 is preferred; an epic PR has no cap, since it gathers many group PRs"
  - "an epic's version names its branch, so every group's version must equal its epic's exactly"
  - "naming: only an epic branch is named by version, strictly feat/v<version>; a group branch and PR keep their own unique names, each naming its group, so every change traces to its task group"
  - "REQ-13 becomes: a group branch cuts from its epic's branch, or stacks on the branch of a group it depends on; update system-design's git and shipping sections to match"
type: docs
```

#### [TSK-05.7.2] Lint pins every group's version to its epic's [P: C] [DONE]
```yaml
files: [internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/...
  - go run ./cmd/komodo lint
context:
  - "the epic branch is named from the version, so a wrong version sends a builder's work to the wrong branch; an epic's goal line names its version as Ships as `x.y.z`"
  - "a group whose version differs from its epic's is a problem naming both; an epic with no version is a problem"
```

#### [TSK-05.7.3] A group with no declared base cuts from its epic's branch [P: C] [DONE]
```yaml
files: [internal/backlog/backlog.go, internal/line/next.go, internal/line/next_test.go, komodo/rules/backlog.md]
done_when:
  - go test ./internal/backlog/... ./internal/line/...
depends_on: [TSK-05.7.2]
context:
  - "EpicBranch is feat/v plus the group's version; groupBase is the declared base, else the epic branch, else the remote's default when the epic branch is missing and cannot be opened"
  - "lint's base rule accepts the group's epic branch; the grammar in komodo/rules/backlog.md says a group cuts from its epic's branch and states that its version picks the branch"
  - "a PR's branch is <type>/<id>-<slug>, the id kept as written: feat/TG-05.2-the-host-contract for a group, fix/TSK-04.6.1-stop-verbose-outputs for a single task; its PR title ends with the same id, such as (TG-05.2), so each traces to its group or task"
  - "only feat/v<version> is an epic branch: lint refuses a group whose declared base looks like one but names another epic's version"
  - "test (REQ-13): a group with no base plans onto feat/v<version>; one with depends_on stacks on its parent's branch; a group branch carries its group id"
```

#### [TSK-05.7.4] The line opens an epic's branch and draft PR on its first cut [P: C] [DONE]
```yaml
files: [internal/line/epic.go, internal/line/epic_test.go, internal/line/cut.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-05.7.3]
context:
  - "when a group cuts and its epic branch is not on origin, the conductor cuts it from main, pushes it with its own credential, and opens a draft PR to main titled feat: <epic title> (<version>), with the epic's goal as the summary"
  - "where drafts are unavailable, the PR is labelled status: wip, as TSK-07.8.3 does for group PRs; a draft base never stops a group PR from targeting or stacking on it"
```

#### [TSK-05.7.5] The conductor merges a reviewed group into its epic branch [P: C] [DONE]
```yaml
files: [internal/line/merge.go, internal/line/merge_test.go, internal/pr/pr.go, internal/pr/pr_test.go]
done_when:
  - go test ./internal/line/... ./internal/pr/...
depends_on: [TSK-05.7.4]
context:
  - "after ship, when the review left nothing at or above the floor and every check passed, the conductor merges the group PR into its epic branch with a merge commit, never a squash, so a stacked child keeps its base"
  - "it never merges into main or a critical ref from komodo/policy.json, and a model session never merges; a refused merge leaves the PR open and names why"
```

#### [TSK-05.7.6] A group PR holds at most 20 files and 2,000 changed lines [P: H] [DONE]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go, internal/profile/profile.go, internal/profile/profile_test.go]
done_when:
  - go test ./internal/line/... ./internal/profile/...
depends_on: [TSK-05.7.5]
context:
  - "the profile carries pr_files 20, pr_lines_preferred 1000 and pr_lines_max 2000; ship refuses a group PR over either ceiling and names a split, and a PR over the preferred size says so in its body"
  - "an epic PR has no cap"
  - "lint refusing a group whose tasks list more than 20 files lands with TG-07.1's group files"
```

#### [TSK-05.7.7] The rules and skills send every PR to its epic branch [P: H] [DONE]
```yaml
files: [komodo/AGENTS.md, komodo/skills/standards-sdlc/SKILL.md, komodo/skills/respond/SKILL.md, komodo/roles/responder.md]
done_when:
  - grep -q 'epic branch' komodo/AGENTS.md
  - go run ./cmd/komodo doctor
depends_on: [TSK-05.7.1]
context:
  - "AGENTS.md's Git section: a group PR targets its epic branch, and landing into main is a person's merge of the epic PR; keep the file's size unchanged, since the always-on context sits at its cap"
  - "the SDLC standard and the responder: branches, PR targets, stacking, drafts and the size limits, as decision 0028 states them"
type: docs
```

#### [TSK-05.7.8] Doctor reports an epic missing its branch or draft PR, and a group PR aimed at main [P: M] [DONE]
```yaml
files: [internal/doctor/epics.go, internal/doctor/epics_test.go]
done_when:
  - go test ./internal/doctor/...
depends_on: [TSK-05.7.4]
context:
  - "with --remote: an epic holding ready groups whose branch or draft PR is missing on origin, and an open group PR whose base is main while its epic branch exists"
  - "the guard is frozen until TG-06.2, so the rule that a model session may not push to or merge an epic branch lands there (TSK-06.2.1)"
```

### [TG-05.8] Versions go alpha, beta, rc, stable, and an epic branch is feat/<version>
```yaml
type: feat
version: 1.0.0-alpha.6
base: feat/every-epic-has-a-draft-branch-and-group
depends_on: [TG-05.7]
```
* **Why:** the owner set the release phases and branch names on 2026-09-26: alpha, then beta, then rc, then stable, and an epic branch is the version itself behind feat/, such as feat/1.0.0-alpha.1, feat/2.3.45-beta.12, feat/1.1.0-rc.1 and feat/5.31.0. TG-05.7 was already building feat/v<version>.

#### [TSK-05.8.1] A version is x.y.z, or x.y.z-alpha.n, -beta.n or -rc.n [P: C] [DONE]
```yaml
files: [internal/backlog/backlog.go, internal/backlog/lint.go, internal/backlog/lint_test.go, internal/changelog/changelog_test.go]
done_when:
  - go test ./internal/backlog/... ./internal/changelog/...
  - go run ./cmd/komodo lint
context:
  - "lint refuses any other prerelease, such as -dev.1 or -beta without a number, naming the four phases"
  - "the phases order alpha, beta, rc, then stable; a test proves 1.0.0-beta.9 < 1.0.0-rc.1 < 1.0.0 in changelog.Compare"
  - "rc is optional: nothing refuses a stable version whose epic had no rc, and a test proves 1.0.0-beta.2 followed by 1.0.0 lints and orders cleanly"
```

#### [TSK-05.8.2] An epic branch is feat/<version>, with no v [P: C] [DONE]
```yaml
files: [internal/backlog/backlog.go, internal/line/next.go, internal/line/next_test.go, internal/line/epic.go, internal/line/epic_test.go]
done_when:
  - go test ./internal/backlog/... ./internal/line/...
depends_on: [TSK-05.8.1]
context:
  - "EpicBranch is feat/ plus the version exactly, such as feat/1.0.0-alpha.6; release tags keep their v, such as v1.0.0-alpha.6"
  - "tests: each of feat/1.0.0-alpha.1, feat/2.3.45-beta.12, feat/1.1.0-rc.1 and feat/5.31.0 is the epic branch of a group at that version"
```

#### [TSK-05.8.3] Decision 0029, the README, the rules, the skills and the template name the four phases and the branch form [P: H] [DONE]
```yaml
files: [docs/decisions.md, README.md, AGENTS.md, komodo/rules/backlog.md, komodo/skills/backlog/SKILL.md, komodo/skills/standards-sdlc/SKILL.md, templates/project/docs/backlog/TG-01.1-example-group.md]
done_when:
  - grep -q '^## 0029' docs/decisions.md
  - grep -q 'rc' komodo/rules/backlog.md
  - go run ./cmd/komodo doctor
depends_on: [TSK-05.8.2]
context:
  - "decision 0029 amends 0023 and 0028: the phases are alpha, beta, rc and stable; an epic branch is feat/<version>"
  - "rc is optional and reserved: a release may go straight from beta to stable, such as 1.0.0-beta.2 to 1.0.0, and nothing requires, gates or checks for an rc; V1 takes that path"
  - "README's Versions section gains an rc bullet marked optional and renames Release to Stable; AGENTS.md says Versions go alpha, beta, optional rc, stable, with the always-on context still under its cap"
  - "the backlog rule, the backlog skill and the SDLC standard give the four phases and the branch examples; the template's example group carries a phase version"
type: docs
```

#### [TSK-05.8.4] internal/backlog/lint.go:78 Lint still accepts base: main, against REQ-13 [P: L] [REFINEMENT]
```yaml
files:
  - internal/backlog/lint.go
done_when:
  - test -f internal/backlog/lint.go
type: fix
context:
  - 'REQ-13 says a group cuts from its epic branch or a dependency''s branch, and lint should reject any other base. The base check still special-cases base != "main", so a group at 1.0.0-alpha.6 with base: main lints clean and bypasses feat/1.0.0-alpha.6. No group in BACKLOG.md declares base: main, so nothing needs the exception. This line predates TG-05.8''s diff. Remove base != "main" from the accepted bases and from the error message.'
```

#### [TSK-05.8.5] internal/backlog/lint_test.go:189 Other-epic base test uses the old feat/v form [P: L] [REFINEMENT]
```yaml
files:
  - internal/backlog/lint_test.go
done_when:
  - test -f internal/backlog/lint_test.go
type: test
context:
  - "TestLintRejectsAGroupWhoseBaseNamesAnotherEpicsBranch uses base feat/v2.0.0. After TSK-05.8.2 an epic branch is feat/2.0.0. If lint regressed to accept any feat/<version>, this test would still pass because feat/v2.0.0 is not in that form. Use base: feat/2.0.0 in the test."
```

#### [TSK-05.8.6] docs/system-design.md:91 system-design still names epic branches feat/v<version> [P: L] [REFINEMENT]
```yaml
files:
  - docs/system-design.md
done_when:
  - test -f docs/system-design.md
type: docs
context:
  - "The Base row at line 91 and ship step 1 at line 266 say feat/v<epic's version>. Decision 0029 and Group.EpicBranch now produce feat/<version>, so a builder given this section as context names the wrong branch. Change both lines to feat/<epic's version> and cite decision 0029."
```

#### [TSK-05.8.7] internal/backlog/backlog.go:337 Parse appends the goal line to Epic.Title [P: L] [REFINEMENT]
```yaml
files:
  - internal/backlog/backlog.go
done_when:
  - test -f internal/backlog/backlog.go
type: refactor
context:
  - 'Parse concatenates the goal line into Title only when it contains "Ships as `". Epic.Version then rescans Title, and internal/line/epic.go splits it again on " *Goal:". A goal line without "Ships as" is dropped, so the epic''s draft PR loses its summary. Add Goal and Version fields to Epic and set them once in Parse.'
```

#### [TSK-05.8.8] internal/backlog/backlog.go:333 Redundant prefix check and unused index [P: L] [REFINEMENT]
```yaml
files:
  - internal/backlog/backlog.go
done_when:
  - test -f internal/backlog/backlog.go
type: refactor
context:
  - 'strings.HasPrefix(nextLine, "###") can never matter after strings.HasPrefix(nextLine, "#"). At line 336, idx is only compared >= 0. Drop the "###" check and use strings.Contains at line 336.'
```

#### [TSK-05.8.9] internal/backlog/backlog.go:327 Comments restate the code [P: L] [REFINEMENT]
```yaml
files:
  - internal/backlog/backlog.go
done_when:
  - test -f internal/backlog/backlog.go
type: docs
context:
  - The comments at line 327 (look ahead for the goal line) and line 334 (stop at next heading) only restate the loop and the break below them. Delete both comments.
```







### [TG-05.9] `komodo run` drives a group through the conductor, end to end
```yaml
type: feat
version: 1.0.0-alpha.6
depends_on: [TG-05.6]
```
* **Why:** TSK-05.6.2 closed with no change, since its checks already held, so `komodo run` still relays stages through a model; TSK-05.6.3's skill then pointed at a conductor nothing starts. This group wires it, and its last check is a group driven to Shipped against a fake host.

#### [TSK-05.9.1] A mount hands the conductor its host contract [P: C] [DONE]
```yaml
files: [internal/mount/registry.go, internal/mount/mount_test.go, internal/mount/claude/claude.go, internal/mount/claude/claude_test.go]
done_when:
  - go test ./internal/install/...
  - go test ./internal/mount/claude/ -run TestTheClaudeMountHandsOutItsContract
context:
  - "mount.Host gains Contract func(root, worktree string) Contract; the Claude mount registers NewMount with the profile's turn cap; a mount without one, such as Codex or Ollama, leaves it nil and komodo run says it cannot drive that host"
```

#### [TSK-05.9.2] The builder's and reviewer's start requests come from the role, the group and the profile [P: C] [DONE]
```yaml
files: [internal/run/requests.go, internal/run/requests_test.go, internal/line/step.go]
done_when:
  - go test ./internal/run/ -run TestStartRequests
context:
  - "the builder's brief fills the builder role for the whole group: each task's BuildBrief, in wave order; the reviewer's brief is what reviewBrief writes, exported as ReviewBrief"
  - "each request carries its role's tools and its schema from komodo/roles/<role>.schema.json, the model and effort from profile.Machine for builder, and the reviewer tier's machine for the reviewer"
  - "tests: the builder request names every task of a two-task group and the builder schema; the reviewer request carries the group's diff"
```

#### [TSK-05.9.3] The stations fit one group builder, and the review lands where ship reads it [P: C] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - "Prepare no longer merges task branches, since the group builder works the group worktree itself: it reruns the compile gates and verify there, and records every plan task DONE in live status for ShipGroup"
  - "the conductor writes the reviewer's result to the group's review result path, so ShipGroup's missing-review refusal passes only when a review really ran"
```

#### [TSK-05.9.4] `komodo run <group>` drives the group to Shipped through the conductor [P: C] [DONE]
```yaml
files: [internal/run/run.go, internal/run/drive.go, internal/run/drive_test.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/run/ -run TestRunDrivesAGroupEndToEnd
  - go test ./internal/run/... ./cmd/komodo/...
depends_on: [TSK-05.9.1, TSK-05.9.2, TSK-05.9.3]
context:
  - "was TSK-05.6.2: cut the group when no run is open, load its state.json or start one at Ready, build the Driver from the contract, the requests, the stations and the ledger, then Drive or Resume, and exit non-zero unless it reached Shipped; the drain drives each ready group the same way"
  - "the relay, Headless and its /run prompt, stays behind komodo run --relay until TSK-05.5.4's proof passes, then goes"
  - "TestRunDrivesAGroupEndToEnd puts a fake claude script on PATH that replays a builder result and then a reviewer result, with a bare origin and a fake forge client; the group reaches Shipped, its branch is pushed, and the ledger holds one build and one review session"
```

#### [TSK-05.9.5] The run skill launches and watches, and never relays a stage [P: H] [REFINEMENT]
```yaml
files: [komodo/skills/run/SKILL.md]
done_when:
  - "! grep -q 'komodo step' komodo/skills/run/SKILL.md"
  - "! grep -q 'komodo status' komodo/skills/run/SKILL.md"
  - go run ./cmd/komodo doctor
depends_on: [TSK-05.9.4]
context:
  - "was TSK-05.6.3: the skill starts komodo run <group> in the background and reports komodo resume <group>; it never calls brief, close or step, and names no command that does not exist yet; komodo status lands with TSK-08.4.4"
  - "waits until TSK-05.5.4's proof retires the relay: drains, --relay and --no-ship still run a headless /run session on this skill, and a relay session may not call komodo run"
type: docs
```

### [TG-05.10] The conductor closes the gaps TG-06.2's proof run found
```yaml
type: fix
version: 1.0.0-alpha.6
depends_on: [TG-05.9]
```
* **Why:** TG-06.2's proof reached Shipped only after five hand repairs: a handle lost across a restart, an escalation answered by editing state.json, a PR the conductor never merged into its epic, a branch without its group, and a body calling a checked group unproven.

#### [TSK-05.10.1] A session handle is the host's session ID, so resume crosses a restart [P: H] [DONE]
```yaml
files: [internal/mount/claude/contract.go, internal/mount/claude/contract_test.go]
done_when:
  - go test ./internal/mount/claude/...
context:
  - "handles are process-local, claude-1 then claude-2, so a restarted komodo run cannot resume its builder, and a new process's claude-1 overwrote the old builder's .komodo/sessions/claude-1.jsonl"
type: fix
```

#### [TSK-05.10.2] The conductor merges a shipped group PR into its epic branch [P: C] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go, internal/run/drive.go, internal/run/drive_test.go]
done_when:
  - go test ./internal/conductor/... ./internal/run/...
context:
  - "decision 0028: only a person merges an epic PR; line.MergeGroup exists in internal/line/merge.go, but no conductor state calls it, and state.json's merged flag is never set"
type: fix
```

#### [TSK-05.10.3] A group branch names its group, such as `refactor/TG-06.2-the-guard-keeps-five-rules` [P: M] [DONE]
```yaml
files: [internal/backlog/backlog.go, internal/backlog/backlog_test.go]
done_when:
  - go test ./internal/backlog/... ./internal/line/...
context:
  - "Group.Branch is type/slug today, so a PR's branch never names the group it came from"
type: fix
```

#### [TSK-05.10.4] A conductor-shipped PR body reports the checks the conductor ran [P: M] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/line/ship_body.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/conductor/... ./internal/line/...
context:
  - "Ship passes nil waves, so #231's body said no QC gate or verify command ran after Check and Prepare had both passed"
type: fix
```

#### [TSK-05.10.5] A live smoke test drives a two-task example group through real sessions [P: H] [DONE]
```yaml
files: [internal/run/live_test.go, internal/run/testdata/live/BACKLOG.md]
done_when:
  - go test ./internal/run/...
context:
  - "skipped unless KOMODO_LIVE=1, since it spends plan tokens, so Check never runs it and the owner runs it by hand; a scratch repo with a bare origin, a fake forge client, the light tier, and one seeded review finding so repair runs"
  - "asserts Shipped, one build, a review that saw the whole diff, a repair by the builder role, and a pushed branch that fast-forwards; TG-06.2's proof found thirteen defects the fake host never could"
  - "TG-08.7's golden suite measures quality on pinned real repos; this one only proves the conductor's wiring"
type: test
```

#### [TSK-05.10.6] The line's tests pass inside a line session [P: M] [DONE]
```yaml
files: [internal/line/main_test.go, internal/line/close_test.go, internal/line/step_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "TG-06.6's builder saw TestCloseRecordsDoneInTheRunWhenEverythingPasses and TestClosingClearsTheFailureRecord fail with KOMODO_RUN_PID set, and TestStepNamesTheBaseTheGroupDeclares fail with GOTMPDIR under .komodo/wt; neither reproduces outside the sandbox"
  - "a builder reads these as its own failures, so TG-06.6 escalated BLOCKED with all four tasks built"
  - "likely cause: in the sandbox, a test's temp dir sits inside the real worktree, so any walk up for .git, BACKLOG.md or a group name finds the real one; TG-06.3's hook tests did this and ran the real group's checks recursively, a fork bomb that took 24 GB; every such test fixture needs its own .git"
type: test
```

#### [TSK-05.10.7] The pre-push hook runs without the forge credential [P: H] [DONE]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "git push runs the pre-push komodo gate, which inherits the conductor's environment, credential included; TG-06.6 kept the credential with the push but left the hook's environment as is"
type: fix
```

#### [TSK-05.10.8] Drift ignores which komodo binary a rendered hook names [P: M] [DONE]
```yaml
files: [internal/install/install.go, internal/install/install_test.go]
done_when:
  - go test ./internal/install/...
context:
  - "TG-06.3's RenderPluginHooks writes the installing binary's own path into hooks.json, so an install by one binary and a doctor by another report drift and fail the conductor's preflight; the guard's settings.json already names bin/komodo-<os>-<arch> in the main checkout"
  - "resolved in the drift check, not the render: the bin/ binary is built from main and exits 2 on komodo hook, which would block a Stop hook, so hooks keep naming the installing binary"
type: fix
```

#### [TSK-05.10.9] internal/line/ship.go:464 The pre-push hook receives the credential-bearing push URL as its arguments [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: fix
context:
  - 'runPrePush passes pushURL as both hook arguments. That URL comes from `git remote get-url --push origin`, and redactURL exists precisely because it can carry `https://user:token@host`. So when origin embeds a token, the hook sees it in $1/$2 and in its process list. That defeats hookEnv''s purpose of running the hook without the forge credential. The new test only exercises GH_TOKEN/GITHUB_TOKEN in the environment, never a credential in the URL. Pass the remote name `origin` (or redactURL(pushURL, "")) as the hook arguments instead of the raw push URL.'
```

#### [TSK-05.10.10] internal/backlog/lint.go:75 Lint accepts a title-only base for a dependency that has not been cut and never will be [P: L] [REFINEMENT]
```yaml
files:
  - internal/backlog/lint.go
done_when:
  - test -f internal/backlog/lint.go
type: fix
context:
  - "Lint now accepts depGroup.TitleBranch() without checking origin. But Branch() now always cuts the ID form, so the title-only branch will never exist for a dependency that has not been cut yet. Example: a child declares `base: feat/first-group` and depends on an unshipped TG-01.0. Lint passes. The parent is cut as `feat/TG-01.0-first-group`. readyGroups (next.go:371-378) only registers the new form in `ahead`, and onOrigin never finds the old name, so the child is skipped forever with no problem reported. Accept TitleBranch only when origin already holds that ref, or have lint flag it so the base is rewritten to the ID form."
```

#### [TSK-05.10.11] internal/line/main_test.go:25 The git ceiling uses GOTMPDIR, but t.TempDir creates directories under os.TempDir() [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/main_test.go
done_when:
  - test -f internal/line/main_test.go
type: fix
context:
  - "tempRoots takes GOTMPDIR first. t.TempDir ignores GOTMPDIR and uses os.TempDir() ($TMPDIR). In a line session, session.go:74 sets GOTMPDIR to <worktree>/.komodo/go/tmp, while TMPDIR is the separate SessionTmp root. So GIT_CEILING_DIRECTORIES names a directory inside the real worktree that no fixture lives under, and fixtures under $TMPDIR get no ceiling. The fixture isolation TSK-05.10.6 relies on does not hold in exactly the environment it targets. Build the ceiling from os.TempDir() (plus GOTMPDIR if you want both), not from GOTMPDIR alone."
```

#### [TSK-05.10.12] internal/conductor/drive.go:172 A run resumed at Shipped never retries the merge [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - "Merge runs only on entry to Shipped. The state is saved as Shipped before work runs, so if the process is interrupted or killed during Merge, state.json holds Current=Shipped, Merged=false. On resume, shippedNext returns Move=Shipped, which equals Current, so Drive returns immediately. A PR on its epic branch is then left for a person, contrary to decision 0028's auto-merge. Have shippedNext (or Drive's resume path) call Merge again when an epic-based group sits at Shipped with Merged false."
```

#### [TSK-05.10.13] internal/conductor/drive.go:367 Ship's path that reruns Check on resume is untested [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: test
context:
  - "When a run resumes at Shipping, l.checked is nil, so Ship reruns Check and refuses to ship with 'the checks fail at ship' if it fails. No test drives Line.Ship with checked unset, either passing or failing. If this branch were removed, a resumed Ship would again report 'no QC gate ran' and every done_when would still pass. Add a Line.Ship test that starts with checked unset and asserts that both a failing Check and a passing Check are reported in the body."
```

#### [TSK-05.10.14] internal/mount/claude/contract.go:194 Open log files leak when saving the request fails [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/contract.go
done_when:
  - test -f internal/mount/claude/contract.go
type: fix
context:
  - "The .jsonl and .err files are created, then the new WriteFile of the request can fail and spawn returns. Those two os.File handles are never closed, and the log files are left behind for a session that never started. Write the request file before creating the log files, or close record and cmd.Stderr on the error return."
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
  - "the phase exit, once TG-05.9 ships: promote TG-06.2, run it with komodo run, and confirm a PR within 60 minutes with zero tokens outside build, review and repair sessions; record the run ID in the PR"
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

#### [TSK-06.2.1] The guard is cut to five rules and the builder's file scope [P: C] [DONE]
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
  - "a model session may not push to or merge an epic branch; only the conductor does (decision 0028, TSK-05.7.8)"
tier: heavy
type: refactor
```

#### [TSK-06.2.2] A refusal names the way forward, and three of one rule end the session as blocked [P: C] [DONE]
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

#### [TSK-06.2.3] Line sessions can't edit the PRD or the golden suite [P: H] [DONE]
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

#### [TSK-06.2.4] internal/guard/hook.go:51 The refusal limit keys on all rendered findings joined together, not on the rule [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/hook.go
done_when:
  - test -f internal/guard/hook.go
type: fix
context:
  - 'The key is strings.Join(decision.Findings, "|"). `git push origin main` then `git push -f origin main` produce different keys because the force finding is added. A path finding also embeds the path, so the same rule hit with a different target never reaches refusalLimit. That is the variant-after-variant case blockedReason says it prevents. Count each finding under a stable rule ID, so one rule hit through different commands adds to the same counter.'
```

#### [TSK-06.2.5] internal/guard/git.go:95 Unsafe mode now allows deleting a critical ref and pushing to an epic branch [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/git.go
done_when:
  - test -f internal/guard/git.go
type: fix
context:
  - "pushFindings returns before judging targets when the mode is unsafe. `git push --delete origin main` passes, though the old code refused a critical-ref delete in every mode. `git push origin feat/1.0.0-alpha.7` from a model session also passes, though decision 0028 lets only the conductor push an epic branch. Judge deletes and epic-branch targets before the unsafe-mode early return, and skip only the plain critical-ref push."
```

#### [TSK-06.2.6] internal/guard/git.go:75 A short force cluster slips past the history-rewrite rule [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/git.go
done_when:
  - test -f internal/guard/git.go
type: fix
context:
  - "isForceFlag matches only -f, --force, --force-if-includes and --force-with-lease*. `git push -fu origin feat/x` force-pushes and gets no finding, breaking Rule 2 on a common spelling. Treat any single-dash cluster that contains 'f' as force, as the removed code did."
```

#### [TSK-06.2.7] internal/guard/git.go:144 branch -f refuses a critical start point as if the ref were being moved [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/git.go
done_when:
  - test -f internal/guard/git.go
type: fix
context:
  - "branchFindings flags every positional that names a critical ref. `git branch -f feat/y main` resets feat/y onto main but is denied with 'git branch main: a critical ref is never moved by hand'. `git branch -c main feat/copy` is denied the same way. When only forcing (no -m or -c), judge only the first positional; for copy, judge only the destination."
```

#### [TSK-06.2.8] internal/guard/guard.go:75 The commandFindings comment claims wrappers and substitutions hide nothing [P: L] [DONE]
```yaml
files:
  - internal/guard/guard.go
done_when:
  - test -f internal/guard/guard.go
type: docs
context:
  - "commandName(item.words[0]) is 'env', 'sudo', 'timeout' or 'FOO=1' for `env git push origin main` or `FOO=1 git push origin main`. The git check never runs, so the comment states a guarantee the code does not give. Rewrite the comment to say only a call whose first word runs git or gh is checked."
```

#### [TSK-06.2.9] internal/guard/table.go:67 The narrowed flag parsing has no rows, and one row is a duplicate [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/table.go
done_when:
  - test -f internal/guard/table.go
type: test
context:
  - "No test covers `git push -fu`, `git commit -nm`, a critical-ref delete in unsafe mode, or `git branch -f feat/y main`. Every regression above passes go test and guard check. Row 67 repeats row 66's command and branch exactly. Add table rows for each of these forms and drop the duplicate push-to-main row."
```

#### [TSK-06.2.10] internal/guard/hook.go:93 session_id builds a file path unchecked [P: L] [DONE]
```yaml
files:
  - internal/guard/hook.go
done_when:
  - test -f internal/guard/hook.go
type: fix
context:
  - "The host payload's session_id goes straight into filepath.Join. A value such as '../../x' writes the counts file outside .komodo/runs/guard-refusals. Reject a session ID containing a path separator or '..' before building the path."
```

#### [TSK-06.2.11] internal/guard/hook.go:89 Concurrent hook calls lose refusal counts [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/hook.go
done_when:
  - test -f internal/guard/hook.go
type: fix
context:
  - "recordRefusal reads, adds one and writes with no lock. Two parallel denied calls in one session both read the same count and one increment is lost, which delays the block. Write to a temp file and rename it, or hold a lock on the file across the read and write."
```

#### [TSK-06.2.12] internal/guard/git.go:62 hasNoVerify misses -nm and --no-verify abbreviations [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/git.go
done_when:
  - test -f internal/guard/git.go
type: fix
context:
  - "Only the exact '--no-verify' and a bare '-n' match. `git commit -nm x` and `git commit --no-verif` skip the hook without a finding. Scan commit's short clusters for 'n' before a flag that takes a value, and accept unambiguous --no-verify prefixes."
```

#### [TSK-06.2.13] internal/guard/tokenize.go:31 Only > and >> count as writes, so a line role can still edit eval/** or docs/prd.md [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/tokenize.go
done_when:
  - test -f internal/guard/tokenize.go
type: fix
context:
  - "`sed -i s/a/b/ eval/golden.json`, `tee docs/prd.md` and `cp x eval/case.json` produce no write target. REQ-41's refusal holds only for the Write tool and redirects. Add tee, cp and mv destinations and sed -i targets as write targets."
```

#### [TSK-06.2.14] internal/guard/hook.go:47 The payload is decoded a second time just for session_id [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/hook.go
done_when:
  - test -f internal/guard/hook.go
type: refactor
context:
  - "A local anonymous struct re-unmarshals raw and discards the error, though Request was already decoded from the same bytes. Add a SessionID field with the json tag session_id to Request and read request.SessionID."
```

#### [TSK-06.2.15] internal/guard/policy.go:187 Comments cite decision and REQ numbers, and 'exactly' is wrong [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/policy.go
done_when:
  - test -f internal/guard/policy.go
type: docs
context:
  - "Comments at policy.go:187 and 201, hook.go:18 and table.go:101 cite decision 0028, REQ-37 and REQ-41. epicBranchRe has no end anchor, so it matches feat/1.2.3anything, not 'exactly' a version. Drop the citations and either anchor the regex or remove 'exactly'."
```













### [TG-06.3] Hooks follow one contract
```yaml
type: feat
version: 1.0.0-alpha.7
```
* **Why:** most loops in the first line came from hooks: 187 builder refusals and review rounds chasing guard bypasses. Proves REQ-37.

#### [TSK-06.3.1] Every hook has one job, one stage and a limit, and fails open [P: C] [DONE]
```yaml
files: [internal/hooks/hooks.go, internal/hooks/hooks_test.go, cmd/komodo/hook.go, cmd/komodo/hook_test.go, cmd/komodo/main.go]
done_when:
  - go test ./internal/hooks/... ./cmd/komodo/...
context:
  - docs/system-design.md#hooks
  - "komodo hook <name> is each hook's entry point; the table of hooks, sessions, limits and failure behaviour is data in this package; a hook that errors returns allow"
  - "wire it: main.go dispatches the hook command to runHook"
```

#### [TSK-06.3.2] Format formats and lints the edited file, and never refuses [P: H] [DONE]
```yaml
files: [internal/hooks/format.go, internal/hooks/format_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-06.3.1]
context:
  - "PostToolUse on a builder's edit: gofmt for Go, the repo's formatter for TypeScript, on that one file; lint output returns as context"
```

#### [TSK-06.3.3] Task checks refuse a builder's stop while a check fails, three times at most [P: H] [DONE]
```yaml
files: [internal/hooks/taskchecks.go, internal/hooks/taskchecks_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-06.3.1]
context:
  - "Stop runs the group's checks and refuses with the failing output; the limit is the host's stop-hook cap of 3; if the hook fails it allows, since Check reruns everything"
```

#### [TSK-06.3.4] Time warning at 80 percent of a session's time or turns [P: M] [DONE]
```yaml
files: [internal/hooks/timewarn.go, internal/hooks/timewarn_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-06.3.1]
context:
  - "PostToolUse in builders and lenses; it never refuses, and skips when it can't read the clock"
```

#### [TSK-06.3.5] Each role's plugin carries only its own hooks [P: H] [DONE]
```yaml
files: [internal/mount/claude/plugin.go, internal/mount/claude/plugin_test.go, internal/mount/claude/claude.go]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-06.3.1]
context:
  - "the guard in every session; format, task checks and time warning in the builder; time warning in lenses; the evidence and status hooks join in TG-07.5 and TG-08.4"
  - "wire it: Render in claude.go calls RenderPluginHooks, so an install writes each role plugin's hooks file"
```

#### [TSK-06.3.6] internal/mount/claude/plugin.go:133 Rendered timewarn never gets --minutes, so the time warning never fires [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/plugin.go
done_when:
  - test -f internal/mount/claude/plugin.go
type: fix
context:
  - "pluginHooks passes only --turns to timewarn. In warnTime, Budget.Minutes is always 0, so `late` is always false and the 80%-of-time warning (the contract's 'time or turns') can't fire in any installed session. The only test for it, TestTimeWarnWarnsAt80PercentOfTime, passes --minutes by hand. Render the session's wall-clock minutes into the timewarn command alongside --turns, and assert it in TestEachRolePluginCarriesOnlyItsOwnHooks."
```

#### [TSK-06.3.7] internal/hooks/timewarn.go:50 Warning prints '0 of 0 minutes' when no minutes budget is set [P: L] [REFINEMENT]
```yaml
files:
  - internal/hooks/timewarn.go
done_when:
  - test -f internal/hooks/timewarn.go
type: fix
context:
  - "With only --turns set, which is how every rendered plugin calls it, the warning reads 'This session has used 0 of 0 minutes and 120 of 150 turns', which is wrong information for the model. Build the message only from the budgets that are set (minutes when Minutes>0, turns when Turns>0)."
```

#### [TSK-06.3.8] internal/hooks/timewarn.go:42 Turns counts tool calls, not turns, and loses counts under parallel calls [P: L] [REFINEMENT]
```yaml
files:
  - internal/hooks/timewarn.go
done_when:
  - test -f internal/hooks/timewarn.go
type: fix
context:
  - "Every PostToolUse call adds one to clock.Turns, but profileTurnCap (150) is a turn cap. An assistant turn with 4 parallel tool calls adds 4, so the warning fires well before 80% of the turn cap. Those parallel hooks also each read, bump and rewrite the same file with no lock, so increments get lost, and a read that lands mid-os.WriteFile fails to unmarshal and skips. Either count distinct turns (or label the budget as tool calls), and write the clock file atomically (temp file + rename) under a file lock."
```

#### [TSK-06.3.9] cmd/komodo/hook.go:16 The runHook guard branch is untested [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/hook.go
done_when:
  - test -f cmd/komodo/hook.go
type: test
context:
  - 'No test runs `komodo hook guard`. If that branch regressed into the table path, it would fail open through errNoRunner and quietly disable the guard, and done_when would still pass. Add a runHookWith case for "guard" and assert it reaches runGuard (e.g. a guard refusal on a denied payload).'
```

#### [TSK-06.3.10] internal/mount/claude/plugin.go:132 Hook name is special-cased in the renderer [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/plugin.go
done_when:
  - test -f internal/mount/claude/plugin.go
type: refactor
context:
  - 'pluginHooks checks hook.Name == "timewarn" to decide which flags to add. That puts per-hook data in the renderer, even though the package says the table holds all of it, and a renamed row would silently lose its turn cap. Add a field to the hooks.Hook row saying whether it takes the session budget, and branch on that field instead.'
```






### [TG-06.4] Allow lists cover each stage, and the owner edits this repo on a branch
```yaml
type: feat
version: 1.0.0-alpha.7
depends_on: [TG-06.2]
```
* **Why:** headless runs used `bypassPermissions`, so the guard was the only wall. Proves REQ-38, REQ-40 and REQ-41's deny entries.

#### [TSK-06.4.1] Each role's settings allow what its stage needs, and dontAsk refuses the rest [P: C] [DONE]
```yaml
files: [komodo/roles/builder.md, komodo/roles/reviewer.md, internal/mount/claude/permissions.go, internal/mount/claude/permissions_test.go, internal/mount/claude/session.go, internal/mount/claude/session_test.go]
done_when:
  - go test ./internal/mount/claude/...
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#permissions
  - "roles name Komodo verbs and command classes; the mount turns them into allow and deny rules; the builder's list adds the repo's build, test, lint and format commands from detection"
  - "deny entries for docs/prd.md and eval/** in every line role (REQ-41)"
  - "wire it: Session passes each role's allow and deny rules to the host"
```

#### [TSK-06.4.2] Proof table: no allow-listed command is refused in any role [P: H] [DONE]
```yaml
files: [internal/mount/claude/allow_test.go]
done_when:
  - go test ./internal/mount/claude/...
depends_on: [TSK-06.4.1]
context:
  - "for each role, every command its stage runs in a Go and a TypeScript repo passes both the rendered allow list and the guard (REQ-38)"
type: test
```

#### [TSK-06.4.3] The orchestrator may edit this repo's policy, rules and guard on a branch [P: H] [DONE]
```yaml
files: [komodo/policy.json, internal/guard/policy.go, internal/guard/policy_test.go]
done_when:
  - go test ./internal/guard/...
context:
  - docs/system-design.md#security
  - "config_paths keep bin/**, .git/config and .git/hooks from every session, and komodo/policy.json only from line sessions; a change applies after merge and rebuild (decision 0015)"
  - "test (REQ-40): an orchestrator edit to komodo/policy.json on feat/x is allowed; the same edit from a builder, or on main, is refused"
```

#### [TSK-06.4.4] internal/guard/policy.go:277 The orchestrator can edit komodo/policy.json on an epic branch [P: L] [REFINEMENT]
```yaml
files:
  - internal/guard/policy.go
done_when:
  - test -f internal/guard/policy.go
type: fix
context:
  - "onFeatureBranch only checks IsCritical, and IsCritical leaves out epic branches (decision 0028). So an orchestrator on feat/1.0.0-alpha.7 is allowed to Edit komodo/policy.json directly. That skips group review, even though the guard treats epic branches as off-limits to model sessions elsewhere. The REQ-40 test only covers feat/x and main, so this case goes untested. Make onFeatureBranch also return false when IsEpicBranch(branch) is true, and add an epic-branch row to TestOnlyTheOrchestratorOnABranchEditsTheShippedPolicy."
```

#### [TSK-06.4.5] internal/mount/claude/permissions.go:69 A detection error is thrown away, so the builder loses its repo commands without a trace [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/permissions.go
done_when:
  - test -f internal/mount/claude/permissions.go
type: fix
context:
  - "profile, _ := detect.Detect(worktree) throws the error away. If detection fails, the profile comes back empty and no build, test, lint or format rules are rendered. Under dontAsk the builder's go test or npm test calls are then refused, and nothing points at the cause. Return the detection error from rolePermissions, or log it, instead of assigning it to _."
```

#### [TSK-06.4.6] internal/mount/claude/permissions.go:21 Bash(find:*) lets find -exec run any command [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/permissions.go
done_when:
  - test -f internal/mount/claude/permissions.go
type: fix
context:
  - "The files class renders Bash(find:*), so a builder's `find . -exec curl example.com \;` or `find . -exec sh -c '...' \;` matches the allow list. That turns the fixed command classes into a general shell, and only the guard stands behind it. Drop find from the files class, or add deny rules for find -exec, -execdir and -delete."
```

#### [TSK-06.4.7] internal/mount/claude/permissions.go:22 git-read allows git diff --output, a file write for the reviewer [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/permissions.go
done_when:
  - test -f internal/mount/claude/permissions.go
type: fix
context:
  - "Bash(git diff:*) and Bash(git log:*) match `git diff --output=notes.txt`, which writes a file. The reviewer is meant never to write and now gets a shell with only git-read, so this gives it a write path the allow list was meant to rule out. Add a deny rule for --output on git diff, log and show, or narrow the allowed git-read prefixes."
```





### [TG-06.5] Check reruns everything after every session
```yaml
type: feat
version: 1.0.0-alpha.7
```
* **Why:** police the output, not the input (architecture principle 2). Proves REQ-17 and REQ-36.

#### [TSK-06.5.1] Check reruns format, lint, the group's checks and scope [P: C] [DONE]
```yaml
files: [internal/check/check.go, internal/check/check_test.go]
done_when:
  - go test ./internal/check/...
context:
  - docs/system-design.md#build
  - "port the close station's reruns from internal/line/close.go and verify.go; scope fails an edit outside the group's files"
```

#### [TSK-06.5.2] Output checks catch model commits, changed refs, hooks and git config [P: C] [DONE]
```yaml
files: [internal/check/output.go, internal/check/output_test.go]
done_when:
  - go test ./internal/check/...
context:
  - docs/architecture.md#boundaries
  - "snapshot HEAD, every ref, .git/hooks and .git/config before a session and compare after; one test per case (REQ-36)"
```

#### [TSK-06.5.3] Changed lines are covered by tests [P: H] [DONE]
```yaml
files: [internal/check/coverage.go, internal/check/coverage_test.go]
done_when:
  - go test ./internal/check/...
context:
  - "Go: a cover profile of the touched packages, intersected with the diff's added lines; TypeScript: the repo's coverage command when it has one; the bar is a profile starting value the first eval calibrates"
```

#### [TSK-06.5.4] A secret scan runs over the added lines [P: H] [DONE]
```yaml
files: [internal/check/secrets.go, internal/check/secrets_test.go]
done_when:
  - go test ./internal/check/...
context:
  - "standard-library patterns for common keys and tokens, over added lines only; a test fixture per pattern"
```

#### [TSK-06.5.5] The conductor runs Check after every build and repair, and never reviews first [P: C] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-06.5.1, TSK-06.5.2, TSK-06.5.3, TSK-06.5.4]
context:
  - "test (REQ-17): the ledger records no review before Check passes"
```

#### [TSK-06.5.6] internal/conductor/drive.go:369 After-snapshot is taken after the rerun commands, so their git side effects are charged to the model [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - "Check calls l.rerun(), which runs the compile, verify and `go test` commands, and only then calls TakeSnapshot. Some verify scripts write git state. For example, `npm ci` runs husky's prepare step, which sets core.hooksPath or writes .git/hooks, and `pre-commit install` or `lefthook install` do the same. When a verify command does this, Compare reports 'the git config key core.hooksPath changed' or 'git hook X changed' as a model fault. That forces a needless repair round. Take the after-snapshot right after loadSnapshot, before rerun runs any command."
```

#### [TSK-06.5.7] internal/check/output.go:74 refs/stash and the base branch are handled the wrong way round in the ref filter [P: L] [REFINEMENT]
```yaml
files:
  - internal/check/output.go
done_when:
  - test -f internal/check/output.go
type: fix
context:
  - "snapshotRefs keeps every ref outside refs/heads/ and refs/remotes/, and that includes refs/stash. The stash lives in the common git dir, which all worktrees share. So when another lane or the operator runs `git stash` during a session, this lane reports 'ref refs/stash changed' and repairs for nothing. In the other direction, every refs/heads/* except the lane's own branch is dropped. A model running `git branch -f main HEAD` or `git push . HEAD:main` therefore moves the base branch with no output-check failure. Drop refs/stash from the snapshot and keep the group's base branch ref in it."
```

#### [TSK-06.5.8] internal/check/secrets_test.go:20 The secret scan flags the repo's own test fixtures [P: L] [REFINEMENT]
```yaml
files:
  - internal/check/secrets_test.go
done_when:
  - test -f internal/check/secrets_test.go
type: fix
context:
  - "The scan runs over every added line, test files included, and has no allowlist. Several of its own test fixtures match its patterns, in secrets_test.go and internal/conductor/drive_test.go. Any later group whose diff adds or rewrites these lines fails Check with 'secret: ... looks like a AWS access key'. A builder writing a fixture for this scanner hits the same wall. Build the fixtures from concatenated parts (as the GitHub case already does with strings.Repeat), or add an explicit allowlist marker the scanner honours."
```

#### [TSK-06.5.9] internal/check/check.go:121 git diff output is parsed without pinning prefix, colour, or quoting [P: L] [REFINEMENT]
```yaml
files:
  - internal/check/check.go
done_when:
  - test -f internal/check/check.go
type: fix
context:
  - "Diff and changedFiles call plain `git diff`, and git.Run passes no config overrides, so the user's git config shapes the output. With diff.noprefix or diff.mnemonicPrefix set, headers read '+++ w/pkg/x.go' or '+++ pkg/x.go'. ParseAddedLines only strips 'b/', so file keys never match the cover profile and coverage is silently skipped as unknown. With color.ui=always, ANSI codes break every prefix match. With core.quotePath on (the default), a non-ASCII path comes back quoted and Scope reports it as outside the declared files. Pass `-c core.quotePath=false diff --no-color --no-ext-diff --src-prefix=a/ --dst-prefix=b/` to both git diff calls."
```

#### [TSK-06.5.10] internal/check/coverage.go:36 Line numbering goes wrong inside a hunk [P: L] [REFINEMENT]
```yaml
files:
  - internal/check/coverage.go
done_when:
  - test -f internal/check/coverage.go
type: fix
context:
  - "ParseAddedLines checks for '+++'/'---' headers on every line, even inside a hunk. That goes wrong in three ways. (1) Deleting a markdown or YAML '---' line gives the diff line '----'. It skips the removed-line case and falls into the context case, so next++ shifts every later added line down by one. (2) A '\ No newline at end of file' marker also increments next. (3) Adding a line that starts with '++' gives '+++…', which is read as a file header and changes the file name. The results are wrong file:line locations in secret reports and misattributed coverage. Track each hunk's remaining old/new line counts from its header, treat lines as headers only when no counts remain, and skip lines that start with a backslash."
```

#### [TSK-06.5.11] internal/check/check.go:26 Run's format and lint parameters are unused in production [P: L] [REFINEMENT]
```yaml
files:
  - internal/check/check.go
done_when:
  - test -f internal/check/check.go
type: refactor
context:
  - 'The only production caller, Line.rerun, passes "" for both format and lint and sends every command through checks. The two parameters only add a code path that the tests alone exercise. Remove the format and lint parameters and pass every command through checks.'
```

#### [TSK-06.5.12] internal/conductor/drive_test.go:310 Dead loop in TestDriveNeverReviewsBeforeCheckPasses [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive_test.go
done_when:
  - test -f internal/conductor/drive_test.go
type: refactor
context:
  - "reviewIndex is the index of the first StationReview, so scanning sessions[:reviewIndex] for StationReview can never fail. The real ordering check is the equal() assertion at the end of the test. Delete the reviewIndex search and the loop after it, and keep the final equal() assertion."
```








### [TG-06.6] The forge credential stays with the conductor, and the sandbox holds
```yaml
type: feat
version: 1.0.0-alpha.7
depends_on: [TG-06.1]
```
* **Why:** a model session with forge push rights is the critical risk in the PRD. Proves REQ-26, REQ-33, REQ-34 and REQ-35.

#### [TSK-06.6.1] Every session starts from a scrubbed environment with no forge credential [P: C] [DONE]
```yaml
files: [internal/mount/claude/env.go, internal/mount/claude/env_test.go]
done_when:
  - go test ./internal/mount/claude/...
context:
  - docs/system-design.md#security
  - "port Scrub from internal/run/run.go; add the subprocess scrub spike S8 confirmed; the sandbox denies reads of the git credential store and gh's config"
  - "test (REQ-34): a session's environment and readable paths hold no forge token"
```

#### [TSK-06.6.2] Only Ship reads the forge credential, and it pushes only unprotected branches [P: C] [DONE]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go, internal/run/run.go]
done_when:
  - go test ./internal/line/... ./internal/run/...
context:
  - docs/system-design.md#prepare-and-ship
  - "the credential is read inside the push and handed to no other process; pushable refuses a critical ref; labels follow the push (REQ-26)"
```

#### [TSK-06.6.3] Line sessions run in the sandbox, and `komodo run` refuses without it [P: C] [DONE]
```yaml
files: [internal/mount/claude/sandbox.go, internal/mount/claude/sandbox_test.go, internal/preflight/preflight.go, internal/preflight/preflight_test.go]
done_when:
  - go test ./internal/mount/claude/... ./internal/preflight/...
context:
  - docs/system-design.md#cross-platform-macos-linux-windows
  - "sandboxSettings moves here and is on by default where the platform has one: fail if unavailable, no unsandboxed retry, a network allowlist without the forge; native Windows counts as no sandbox (TG-08.2)"
  - "test (REQ-35): a write outside the worktree fails; komodo run exits non-zero with the sandbox off"
```

#### [TSK-06.6.4] Doctor checks the forge ruleset and head-branch deletion [P: H] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
context:
  - docs/system-design.md#health-checks
  - "--remote: a ruleset with no bypass actors on the default branch where the forge offers one, else a warning; delete head branches on merge; drafts available (REQ-33)"
```

#### [TSK-06.6.5] internal/doctor/doctor.go:136 Any HTTP 403 is read as 'the plan offers no rulesets' and the ruleset check passes [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/doctor.go
done_when:
  - test -f internal/doctor/doctor.go
type: fix
context:
  - "rulesetsUnoffered matches any 'HTTP 403'. When gh's token lacks the admin/read scope for rulesets, or SSO has not authorised it, `doctor --remote` gets a 403, CheckRulesets returns nil, and an unprotected default branch reports no problem. `--json` sets no Warn callback, so the warning is dropped too and the audit comes out clean. Match only the plan-upgrade message ('Upgrade to GitHub Pro'), and keep reporting any other 403 as a 'could not list rulesets' problem."
```

#### [TSK-06.6.6] internal/mount/claude/session.go:88 withSandbox drops the role's hooks and permission denies when settings.json is missing or malformed [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/session.go
done_when:
  - test -f internal/mount/claude/session.go
type: fix
context:
  - "If os.ReadFile fails, or json.Unmarshal of settings.json fails (the error is discarded), merged ends up empty or partial. Only the sandbox block goes into --settings, so the session starts without its PreToolUse guard hooks or `permissions.deny` rules, and nothing reports it. Before this change the host was pointed at the file path and would have failed on a bad file. Have withSandbox return an error when settings.json cannot be read or parsed, and have Session refuse to launch."
```

#### [TSK-06.6.7] internal/run/run.go:264 Headless launch uses line.Scrub, which keeps CLAUDE_CODE_SUBPROCESS_ENV_SCRUB [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/run.go
done_when:
  - test -f internal/run/run.go
type: fix
context:
  - "env.go drops CLAUDE_CODE_SUBPROCESS_ENV_SCRUB because, per its comment, it overrides dontAsk. line.Scrub's `dropped` list does not include it. So `komodo run` launches the headless claude with that variable inherited from the caller, while Session() strips it. Have run.launch use one shared scrub that also drops CLAUDE_CODE_SUBPROCESS_ENV_SCRUB."
```

#### [TSK-06.6.8] internal/mount/claude/env.go:26 scrubEnv duplicates line.Scrub almost line for line [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/env.go
done_when:
  - test -f internal/mount/claude/env.go
type: refactor
context:
  - "forgeWords, secretWords, the override switch, the six appended overrides and the drop list now exist in both internal/line/ship.go and internal/mount/claude/env.go. The copies have already drifted (the subprocess-scrub key above). Keep one exported Scrub and call it from both line and mount/claude."
```

#### [TSK-06.6.9] internal/mount/claude/session_test.go:33 The builder argv test's expectation was reduced to '--settings ' [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/session_test.go
done_when:
  - test -f internal/mount/claude/session_test.go
type: test
context:
  - "The expected fragment is now just '--settings ', which passes whatever follows the flag, including an empty or sandbox-free value. TestARoleSessionCarriesTheLineSandbox only asserts when the host platform has a sandbox. Assert the exact merged settings value (or its parsed sandbox and hooks keys) in TestSessionArgvForBuilder."
```






### [TG-06.7] One reviewer stays warm across a group's review rounds
```yaml
type: feat
version: 1.0.0-alpha.7
depends_on: [TG-06.5]
```
* **Why:** every review round started a cold reviewer that re-read the whole diff and raised new findings on lines no repair touched; TG-06.2 and TG-06.5 each spent three repair rounds that way. A warm reviewer keeps its context and the host's prompt cache, and one cold pass before ship guards against it anchoring on its own earlier view.

#### [TSK-06.7.1] The conductor keeps the group's reviewer and resumes it each round [P: C] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go, internal/conductor/state.go]
done_when:
  - go test ./internal/conductor/...
context:
  - "State.Reviewer holds the reviewer's handle beside Builder; round two onward resumes it with the re-review brief; a resume that fails, after a restart or on a host without resume, starts a cold reviewer with the open findings"
  - "tests: the second review resumes the first reviewer's session; a lost reviewer starts fresh; the ledger stamps each review warm or cold"
```

#### [TSK-06.7.2] A re-review brief carries the diff since the last reviewed commit and the open findings [P: C] [DONE]
```yaml
files: [internal/line/diff.go, internal/line/diff_test.go, internal/run/requests.go, internal/run/requests_test.go, internal/conductor/state.go, internal/run/drive.go, internal/run/drive_test.go]
done_when:
  - go test ./internal/line/... ./internal/run/...
depends_on: [TSK-06.7.1]
context:
  - "State keeps the HEAD each review saw; the re-review diff runs from it to HEAD, so a repair's lines are all the reviewer reads again"
  - "the brief lists each open finding with its file and line, and asks the reviewer to close or keep each one with evidence"
  - "wire it: newDriver in internal/run/drive.go builds the re-review request, so a production run resumes the warm reviewer"
```

#### [TSK-06.7.3] The reviewer role states the re-review rules [P: H] [DONE]
```yaml
files: [komodo/roles/reviewer.md]
done_when:
  - go run ./cmd/komodo doctor
depends_on: [TSK-06.7.2]
context:
  - "a re-review closes or keeps each open finding, and raises a new one only on a line the repair changed (REQ-21); TSK-07.5.6 later enforces the rule in the binary for each lens"
```

#### [TSK-06.7.4] A cold reviewer checks the final state once before ship when the warm one ran more than one round [P: H] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-06.7.1]
context:
  - "the anchoring guard: a fresh reviewer reads the whole diff once; only its findings at or above the severity floor block, and a group gets one cold pass, never a loop of them"
  - "test: a group with two warm rounds gets exactly one cold review before Preparing; a group that passed its first review gets none"
```

#### [TSK-06.7.5] internal/conductor/drive.go:209 Warm re-review ledger rows record an empty role and model in production [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - 'reviewRound starts the warm path from request = d.Reviewer and passes it to d.session, which stamps ledger.Entry{Role: req.Role, Model: req.Model}. newDriver in internal/run/drive.go wires only Review and ReReview and never sets Driver.Reviewer, so d.Reviewer is the zero StartRequest. Every StationReReview row a production run writes therefore has Role "" and Model "", even though the claude mount resumes on prior.req''s model. Per-model cost and usage reporting loses every warm round. The conductor tests set d.Reviewer on the rig, so they never see this. Keep the StartRequest the group''s reviewer was started with (built by Review or taken from d.Reviewer) and pass it to d.session on the warm path, so the resumed round is stamped with that role and model.'
```

#### [TSK-06.7.6] internal/conductor/drive_test.go:372 No test checks the role and model on a re-review ledger row [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive_test.go
done_when:
  - test -f internal/conductor/drive_test.go
type: test
context:
  - "The new tests check only the order of ledger stations (review versus re-review). None checks that a StationReReview entry carries the reviewer's role and model when Review is wired and Reviewer is left empty, which is how production is wired. The done_when for TSK-06.7.1 passes with the empty-role rows described above. Add a case with Driver.Reviewer empty and Review returning a request with Role and Model set, then assert that the re-review ledger entry carries both."
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

#### [TSK-07.1.1] The parser reads group files in docs/backlog/ [P: C] [DONE]
```yaml
files: [internal/backlog/groupfile.go, internal/backlog/groupfile_test.go, internal/backlog/fuzz_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - docs/system-design.md#task-groups
  - docs/system-design.md#the-backlog
  - "a file is <group-id>-<slug>.md: a heading with priority and status, yaml with type, version, epic and depends_on, then checkbox tasks with files and optional accept and checks; a task needs only a title and files (REQ-9); fuzz the parser"
```

#### [TSK-07.1.2] Lint refuses a group over 12 tasks, and a light-tier builder [P: H] [DONE]
```yaml
files: [internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/...
depends_on: [TSK-07.1.1]
context:
  - "over 12 tasks is a problem that suggests a split (REQ-8); tier: light on a build task is a problem (REQ-30); the base rule and the context-anchor check still hold"
```

#### [TSK-07.1.3] `komodo backlog` lists open groups, and `komodo add` writes a group or task [P: H] [DONE]
```yaml
files: [cmd/komodo/backlog.go, cmd/komodo/backlog_test.go, internal/backlog/edit.go]
done_when:
  - go test ./cmd/komodo/... ./internal/backlog/...
depends_on: [TSK-07.1.1]
context:
  - docs/system-design.md#the-komodo-command
```

#### [TSK-07.1.4] The grammar, the planner and `komodo init` describe group files [P: H] [DONE]
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

#### [TSK-07.1.5] docs/backlog/TG-01.1-example-group.md:1 Sample group file committed to the repo itself, not only the template [P: L] [REFINEMENT]
```yaml
files:
  - docs/backlog/TG-01.1-example-group.md
done_when:
  - test -f docs/backlog/TG-01.1-example-group.md
type: fix
context:
  - "Task TSK-07.1.4 names templates/project/docs/backlog, but the diff also adds docs/backlog/TG-01.1-example-group.md at the repo root. init embeds templates/project, so nothing needs this copy. Running `komodo backlog` in this repo prints the fake 'TG-01.1 Example group [REFINEMENT]' as an open group. Delete docs/backlog/TG-01.1-example-group.md from the repo root and keep only the template copy."
```

#### [TSK-07.1.6] cmd/komodo/backlog.go:282 komodo add writes group files from unvalidated id, priority and status [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/backlog.go
done_when:
  - test -f cmd/komodo/backlog.go
type: fix
context:
  - '`komodo add TG-08.1 Auth --priority high` (or --status in_progress, or a groupID like tg-08.1) writes a heading that groupFileHeading rejects. runBacklog skips that file silently (line 198). The next `komodo add TG-08.1 "task" --files a.go` fails to find it in findGroupFile and writes a second group file named after the task title instead of appending. A groupID of ''../x'' makes line 283 write outside docs/backlog. Check groupID against the TG- id pattern, priority against C/H/M/L, and status against REFINEMENT/READY/BLOCKED before rendering the file.'
```

#### [TSK-07.1.7] cmd/komodo/backlog.go:77 Group-file lint enforces none of the grammar's required fields [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/backlog.go
done_when:
  - test -f cmd/komodo/backlog.go
type: fix
context:
  - "groupFileLintProblems adds only ParseGroupFile's parse problems and the 12-task cap. It passes a READY group whose task has no files line, a group with no version or no yaml block (readBlock returns end=-1 and no error), and two files declaring the same TG id; findGroupFile then appends to whichever sorts first. komodo/rules/backlog.md says version is required and that lint demands files outside REFINEMENT, so the gate passes group files the grammar forbids. Add a group-file lint in internal/backlog that requires version and type, requires task files outside REFINEMENT, and rejects duplicate group and task ids across files; call it from groupFileLintProblems."
```

#### [TSK-07.1.8] cmd/komodo/backlog.go:78 No test covers the 12-task cap on group files [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/backlog.go
done_when:
  - test -f cmd/komodo/backlog.go
type: test
context:
  - "lint_test.go tests the cap only for BACKLOG.md through backlog.Lint. No test gives lintProblems or runLintGroupFiles a group file with 13 tasks, so deleting lines 78-80 still passes every done_when command. Add a backlog_test.go case where a 13-task group file makes lintProblems return an 'exceeds limit of 12' problem."
```

#### [TSK-07.1.9] cmd/komodo/backlog.go:258 Usage error names a command that does not exist [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/backlog.go
done_when:
  - test -f cmd/komodo/backlog.go
type: fix
context:
  - "The error reads 'usage: komodo backlog add <group> <title>', but main dispatches `add`, and `komodo backlog` ignores its args. Following the hint lists groups and adds nothing. Change the usage string to 'usage: komodo add <group> <title> [--files a,b]'."
```

#### [TSK-07.1.10] cmd/komodo/backlog.go:40 Lint dispatch and the 12-task limit are duplicated [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/backlog.go
done_when:
  - test -f cmd/komodo/backlog.go
type: refactor
context:
  - "lintProblems repeats runLint's Find branch and its Lint+LintContext call (lines 20-25). The literal 12 and its message appear in both internal/backlog/lint.go:48 and cmd/komodo/backlog.go:78. Make runLint call lintProblems, and put the cap in one named const in internal/backlog shared by both checks."
```

#### [TSK-07.1.11] cmd/komodo/backlog.go:49 Doc comment cites a spec id [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/backlog.go
done_when:
  - test -f cmd/komodo/backlog.go
type: docs
context:
  - "The comment on runLintGroupFiles ends with '(REQ-8)'. The comments standard bans spec and ticket citations. Remove '(REQ-8)' from the comment."
```








### [TG-07.2] Ingest compiles each group into a card
```yaml
type: feat
version: 1.0.0-alpha.8
base: feat/the-backlog-is-one-file-per-group
depends_on: [TG-07.1]
```
* **Why:** builders spent 987 `grep` and 356 `sed` calls finding context the binary could pack (evidence 6). Proves REQ-7 and REQ-9's derived checks.

#### [TSK-07.2.1] `komodo ingest` compiles each READY group into a card with a stable hash [P: C] [DONE]
```yaml
files: [internal/ingest/card.go, internal/ingest/card_test.go, cmd/komodo/ingest.go]
done_when:
  - go test ./internal/ingest/... ./cmd/komodo/...
context:
  - docs/system-design.md#group-cards
  - "cards land in .komodo/queue/<group>.json; files expand globs and directories, and a new file is allowed where its parent exists; no session starts (REQ-7)"
```

#### [TSK-07.2.2] Checks are derived per language the group touches [P: C] [DONE]
```yaml
files: [internal/ingest/checks.go, internal/ingest/checks_test.go]
done_when:
  - go test ./internal/ingest/...
context:
  - "Go: build, vet and test of each touched package; TypeScript: the type check and the repo's test script; hand-written checks add, never replace (REQ-9); detection comes from internal/detect"
```

#### [TSK-07.2.3] Context packs replace exploration [P: H] [DONE]
```yaml
files: [internal/ingest/pack.go, internal/ingest/pack_test.go]
done_when:
  - go test ./internal/ingest/...
context:
  - docs/system-design.md#token-efficiency
  - "file bodies, signatures of imported packages, callers of changed symbols, neighbouring tests, the cited spec sections and the repo's rules; each item capped, then the total; Go through go/parser, TypeScript by a line scan"
tier: heavy
```

#### [TSK-07.2.4] Briefs fill their slots from the card, stable slots first [P: H] [DONE]
```yaml
files: [internal/line/brief.go, internal/line/brief_slots.go, internal/line/brief_test.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-07.2.1, TSK-07.2.3]
context:
  - docs/system-design.md#briefs
  - "test: the same card and tree give the same brief bytes"
```

#### [TSK-07.2.5] internal/line/brief_slots.go:70 cardTask replaces a task's declared files with only the card's matches [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/brief_slots.go
done_when:
  - test -f internal/line/brief_slots.go
type: fix
context:
  - "Ingest drops a new file whose parent folder doesn't exist yet, so files: [cmd/komodo/ingest.go, internal/ingest/card.go] with no internal/ingest/ yet briefs only cmd/komodo/ingest.go. The files slot loses internal/ingest/card.go, and the same drop happens for a plain path added after the last ingest. Keep every declared plain path, and add card expansions only for glob and directory patterns."
```

#### [TSK-07.2.6] internal/line/brief_slots.go:44 cardStale compares only task ids and titles [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/brief_slots.go
done_when:
  - test -f internal/line/brief_slots.go
type: fix
context:
  - "Edit a task's files or context after ingest and the old card is still trusted. ownContext (line 103) can then only drop a reference the task declares; on an old card it silently removes new context from the brief. Mark the card stale when any task's files or context differ from what the card holds, and drop ownContext in favour of the task's own list on a stale card."
```

#### [TSK-07.2.7] internal/ingest/card.go:319 resolveBase disagrees with line's own groupBase [P: L] [REFINEMENT]
```yaml
files:
  - internal/ingest/card.go
done_when:
  - test -f internal/ingest/card.go
type: fix
context:
  - "resolveBase sets the base to the depends_on group's branch, but komodo next never does: next's groupBase (internal/line/next.go:397) uses only an explicit base field and falls back to the default branch once the parent merged. A group with depends_on: [TG-01.1] and no base: gets card base feat/parent-group while next cuts from main, and the card keeps naming the deleted branch after the parent merges. Export and call line's groupBase instead of a second resolver."
```

#### [TSK-07.2.8] internal/ingest/checks.go:71 extractGoPackages skips Go files at the repo root [P: L] [REFINEMENT]
```yaml
files:
  - internal/ingest/checks.go
done_when:
  - test -f internal/ingest/checks.go
type: fix
context:
  - 'A group touching only main.go gets no derived build, vet, or test check, though the root package is a touched package. countPackages in card.go counts that same file, so Size.Packages and the derived Checks disagree. Emit go build ., go vet . and go test . for root-level Go files instead of skipping dir ".".'
```

#### [TSK-07.2.9] internal/ingest/card.go:185 expandFiles has no containment check against root [P: L] [REFINEMENT]
```yaml
files:
  - internal/ingest/card.go
done_when:
  - test -f internal/ingest/card.go
type: fix
context:
  - "expandFiles joins a backlog-declared pattern onto root without cleaning or checking containment. files: [../] walks the parent folder; from a worktree under .komodo/wt/, that is every sibling worktree. readItem and specItems in pack.go then read those ../ paths into the context pack. Clean each pattern and reject any that resolves outside root before stat or walk."
```

#### [TSK-07.2.10] internal/line/brief_test.go:508 The determinism test never exercises a card [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/brief_test.go
done_when:
  - test -f internal/line/brief_test.go
type: test
context:
  - "TestBuildBriefIsDeterministic builds from a repo with no queue card, so it never proves TSK-07.2.4's done_when that the same card and tree give the same brief bytes. A card-driven ordering bug would still pass this test. Write a queue card with a glob-expanded file list before the two builds compared for determinism."
```

#### [TSK-07.2.11] internal/line/brief_slots.go:44 No test covers a card whose task list diverges from the backlog [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/brief_slots.go
done_when:
  - test -f internal/line/brief_slots.go
type: test
context:
  - "If cardStale always returned false, go test ./internal/line/... would still pass; no test builds a brief from a card whose task files or titles differ from the current backlog. Add a brief test with a card whose task title or files differ from the backlog, asserting the task's own declared files are used."
```

#### [TSK-07.2.12] internal/ingest/card.go:137 One dedupe loop is written three times, then deduped again [P: L] [REFINEMENT]
```yaml
files:
  - internal/ingest/card.go
done_when:
  - test -f internal/ingest/card.go
type: refactor
context:
  - "taskFiles, handWrittenChecks and taskContext each repeat the same first-seen dedupe loop, allChecks (checks.go:13) dedupes the already-deduped hand-written list a second time, and countPackages repeats extractGoPackages's own counting. Use one generic first-seen dedupe helper over a task field accessor, and let len(extractGoPackages(files)) stand in for countPackages."
```

#### [TSK-07.2.13] internal/ingest/checks.go:20 Several comments restate the code below them [P: L] [REFINEMENT]
```yaml
files:
  - internal/ingest/checks.go
done_when:
  - test -f internal/ingest/checks.go
type: docs
context:
  - 'The comments at checks.go:20 ("Add hand-written checks first."), :28 ("Add derived checks that aren''t already present."), :47 ("Go: build, vet, and test for each touched package.") and :71 ("skip root-level Go files") restate the statement that follows; card.go:2 and :342 additionally cite (REQ-7), which the comments standard bans. Delete the four restating comments and drop the (REQ-7) citations.'
```










### [TG-07.3] Coordinate schedules groups and paces to the plan
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.2]
```
* **Why:** a Haiku build averaged 75 turns (evidence 7), and a plan's usage window was a person's job to watch. Proves REQ-12, REQ-30 and REQ-32.

#### [TSK-07.3.1] Groups that share no file run in parallel, up to the plan's concurrency [P: C] [DONE]
```yaml
files: [internal/conductor/schedule.go, internal/conductor/schedule_test.go, internal/run/run.go, internal/run/run_test.go]
done_when:
  - go test ./internal/conductor/... ./internal/run/...
context:
  - docs/system-design.md#parallelism
  - "drain in internal/run/run.go drives one group at a time today; it asks the schedule which ready groups may start"
  - "overlap from the cards' files through internal/plan; starting values Pro 1, Max 5x 2, Max 20x 4, API 4; a group with depends_on waits for its parent's branch"
  - "each group keeps its own process watcher, so a runaway in one lane kills only that lane's tree"
  - "test (REQ-12): groups sharing a file run one after another; groups sharing none overlap"
```

#### [TSK-07.3.2] The conductor pauses at a usage limit and resumes at the reset [P: H] [DONE]
```yaml
files: [internal/run/pace.go, internal/run/pace_test.go, internal/run/run.go, internal/profile/profile.go]
done_when:
  - go test ./internal/run/... ./internal/profile/...
depends_on: [TSK-07.3.1]
context:
  - docs/system-design.md#pacing-limits-and-loop-detection
  - "profile.Paused and WaitUntil exist, and the Claude mount reads rate_limit_event, but only plan building checks Paused; the drain never waits"
  - "bound from the plan probe and rate_limit_event; unbound on API billing, with a spend budget; pauses and resumes go to events.jsonl"
  - "test (REQ-32): a simulated rate-limit event pauses the run and resumes it with no person"
```

#### [TSK-07.3.3] A Pro plan runs the economy profile, and no builder runs on the light tier [P: H] [DONE]
```yaml
files: [internal/profile/profile.go, internal/profile/profile_test.go, internal/line/snapshot.go, internal/line/step.go, internal/line/step_test.go, internal/mount/registry.go, internal/doctor/doctor.go, internal/doctor/doctor_test.go, docs/system-design.md]
done_when:
  - go test ./internal/profile/... ./internal/line/... ./internal/mount/... ./internal/doctor/...
depends_on: [TSK-07.3.2]
context:
  - docs/system-design.md#profiles-and-economy-mode
  - "TaskState.BuilderTier's one-file light fallback and mount.LightBuilder go; doctor rejects a profile whose builder is light"
  - "Pro's MaxParallel is 2 in profile.go; economy mode runs one group at a time and one combined lens (REQ-30)"
  - "system-design.md's tier table still says the builder runs Sonnet; decision 0031 moved it to heavy, medium effort"
```

#### [TSK-07.3.4] internal/conductor/schedule.go:36 A child waiting on its parent claims files that block the parent, so neither starts [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/schedule.go
done_when:
  - test -f internal/conductor/schedule.go
type: fix
context:
  - "Startable adds every pending group to claimed, even one held only by waitsOnParent. drainOrder lists open runs first, so a resumed child can come before its ready parent. Take pending [child{files: a.go, depends_on: P}, P{files: a.go}] with nothing running. The child waits and claims a.go. P then shares a.go and is not free. Startable returns nothing, running is empty, and drain prints 'drain done: nothing is ready' without ever running P. Skip the file claim for a group held by an unfinished parent, or never let a waiting group's claim block a group it depends on."
```

#### [TSK-07.3.5] internal/run/run.go:130 A drainGroups error mid-drain returns while lanes are still running [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/run.go
done_when:
  - test -f internal/run/run.go
type: fix
context:
  - "The loop calls drainGroups after every finished lane. If BACKLOG.md fails to parse at that point (a running lane can be rewriting it), drain returns 1 right away. The other lanes' host processes keep running in their own process groups with nothing waiting on them. Their results never reach shipped/parked, and the final Sync is skipped. On a drainGroups error, set stopping and keep draining the finished channel until running is empty, then return the error."
```

#### [TSK-07.3.6] internal/line/step.go:66 taskTier returns its Action argument unchanged [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/step.go
done_when:
  - test -f internal/line/step.go
type: refactor
context:
  - "With the light fallback gone, taskTier just echoes next back and its only real output is snap.Tasks[next.Task].Tier. Both callers reassign an Action that never changes. Have taskTier return only the tier string, or inline snap.Tasks[id].Tier at its two call sites."
```




### [TG-07.4] The builder works the task list, and the conductor ticks it
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.2]
```
* **Why:** one builder per group, briefed with the whole task list, is one story's worth of work (decision 0007). Proves REQ-10.

#### [TSK-07.4.1] The builder role and build skill work a task list in order [P: C] [DONE]
```yaml
files: [komodo/roles/builder.md, komodo/roles/builder.schema.json, komodo/skills/build/SKILL.md]
done_when:
  - test -f komodo/skills/build/SKILL.md
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#build
  - docs/system-design.md#results
  - "internal/run/requests.go already joins every task's brief in wave order; builder.md still frames one task, so it reads a stack of single-task frames"
  - "the result per task: done or blocked, the checks run, and a question when blocked"
```

#### [TSK-07.4.2] `komodo check task|findings|scope` is one entry point for hooks and agents [P: H] [DONE]
```yaml
files: [cmd/komodo/check.go, cmd/komodo/check_test.go, cmd/komodo/main.go]
done_when:
  - go test ./cmd/komodo/...
context:
  - docs/system-design.md#the-komodo-command
  - "each subcommand calls internal/check; findings is what the evidence hook runs"
```

#### [TSK-07.4.3] A person's edit to a task body survives a run, and only checkboxes and blocker notes change [P: C] [DONE]
```yaml
files: [internal/line/status.go, internal/line/status_test.go, internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/line/... ./internal/conductor/...
context:
  - "Line.Prepare in internal/conductor/drive.go already ticks a task only after its checks rerun clean; ship writes the ticks into the backlog"
  - "test (REQ-10): a person's edit to a task body survives a run unchanged, and the only other write is a blocker note"
```

#### [TSK-07.4.4] cmd/komodo/check.go:106 check findings rejects every finding that has no line number [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/check.go
done_when:
  - test -f cmd/komodo/check.go
type: fix
context:
  - '`line` is optional in reviewer.schema.json, and a missing line decodes to 0 in line.Finding. `added[finding.File][0]` is never true, so a valid review with a whole-file test-gap or undocumented-nonobvious finding (for example, only `"file":"a/one.go"`) is reported as ''not on a changed line'' and exits 1. The evidence hook then blocks a review that matches its own schema. When Line is 0, accept the finding if its file has any added line in the diff (or if the file appears in the diff at all).'
```

#### [TSK-07.4.5] internal/line/ship.go:211 No test covers a live BLOCKED status reaching the ship commit [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: test
context:
  - "The new ship test only records DONE and IN_PROGRESS as live statuses. If backlogStatus stopped accepting BLOCKED, or if ship skipped BLOCKED, REQ-10's blocker write would disappear from the ship commit and no ship test would fail. Only the writeStatus unit test checks BLOCKED. Add a TSK-11.1.2 case with a live BLOCKED status to TestShipKeepsAPersonsEditToATaskBodyAndChangesOnlyTheTick and assert that its token changes in the commit."
```



### [TG-07.5] Review lenses and the evidence they must carry
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.4]
```
* **Why:** TG-03.22 ran 11 review rounds because each re-reviewed the whole diff from scratch (evidence 3). Proves REQ-20.

#### [TSK-07.5.1] Four checklist skills replace the one review skill [P: C] [DONE]
```yaml
files: [komodo/skills/review, komodo/skills/review-correctness/SKILL.md, komodo/skills/review-security/SKILL.md, komodo/skills/review-quality/SKILL.md, komodo/skills/review-economy/SKILL.md, komodo/roles/reviewer.md, komodo/roles/reviewer.schema.json]
done_when:
  - test ! -d komodo/skills/review
  - grep -q 'COR-5' komodo/skills/review-correctness/SKILL.md
  - grep -q 'rule_id' komodo/roles/reviewer.schema.json
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#review
  - "each skill lists its lens's rule IDs; a finding carries lens, rule ID, severity, file, line, evidence and a one-line fix"
  - "one reviewer role, started once per lens with that lens's skill"
```

#### [TSK-07.5.2] Validators measure before any lens runs [P: H] [DONE]
```yaml
files: [internal/review/validators.go, internal/review/validators_test.go]
done_when:
  - go test ./internal/review/...
context:
  - "tests and reproducers, the secret scan, a dependency audit and security linters when the repo has them, and caller counts of changed exported symbols; the report is settled fact in every lens's brief"
```

#### [TSK-07.5.3] A finding blocks only when the binary verifies its evidence [P: C] [DONE]
```yaml
files: [internal/review/evidence.go, internal/review/evidence_test.go]
done_when:
  - go test ./internal/review/...
context:
  - "a bug or security finding's reproducer fails on the current tree in a scratch copy; a convention finding cites a rule ID on a changed line; a performance or blast-radius finding has a validator's measurement; anything else becomes a PR note"
  - "one test per kind of evidence (REQ-20)"
tier: heavy
```

#### [TSK-07.5.4] The evidence hook refuses a lens's stop twice at most [P: H] [DONE]
```yaml
files: [internal/hooks/evidence.go, internal/hooks/evidence_test.go, internal/hooks/hooks.go, internal/mount/claude/plugin_test.go]
done_when:
  - go test ./internal/hooks/...
depends_on: [TSK-07.5.3]
context:
  - "registered in hooks.Table under SessionLens, so the reviewer plugin's hook test changes; Stop runs komodo check findings and lists findings without evidence; after 2 refusals those findings become notes"
```

#### [TSK-07.5.5] internal/review/evidence.go:141 Scratch copy keeps the worktree's .git file, so reproducer git commands act on the real worktree [P: L] [REFINEMENT]
```yaml
files:
  - internal/review/evidence.go
done_when:
  - test -f internal/review/evidence.go
type: fix
context:
  - "copyTree skips `.git` only when it is a directory (line 141). A line worktree such as .komodo/wt/TG-07.5 has `.git` as a regular file holding `gitdir: <repo>/.git/worktrees/TG-07.5`. The case at line 150 copies that file into the scratch directory. So a reproducer's `git status`, `git stash`, `git checkout` or `git commit` runs against the real worktree's index, HEAD and branch, with the scratch copy as its work tree. A reproducer can then rewrite the group's index or move its branch, which breaks the isolation that TestAReproducerNeverTouchesTheRealTree claims. That test only builds a `.git` directory, so it cannot catch this. Skip any entry named `.git` whatever its type, and add a test with a `.git` file."
```

#### [TSK-07.5.6] internal/review/evidence.go:183 citesMeasurement accepts any line of any measurement, not a relevant one [P: L] [REFINEMENT]
```yaml
files:
  - internal/review/evidence.go
done_when:
  - test -f internal/review/evidence.go
type: fix
context:
  - "The check passes when the evidence contains any non-empty trimmed line from any measurement's Detail, whatever the measurement's kind. Those lines include test-output tails and the `exit status 1` line that runCommand prepends on failure. For example, a blast-radius finding whose evidence quotes `ok  komodo/internal/hooks 1.2s`, or `PASS` from the tests measurement, is marked as blocking with 'the tests validator measured it', even though no caller count or cost supports it. Match only measurements of the kinds that back the finding's class (callers for blast-radius), or require the evidence to quote a whole measurement line and not a short fragment."
```



### [TG-07.11] The conductor runs Review through parallel lenses
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.5]
```
* **Why:** TG-06.7 keeps one warm reviewer per group; each lens needs that same warm session, and its own findings. Proves REQ-19 and REQ-21.

#### [TSK-07.11.1] Each lens keeps its own warm session, round count and findings [P: C] [DONE]
```yaml
files: [internal/review/lenses.go, internal/review/lenses_test.go, internal/conductor/state.go, internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/review/... ./internal/conductor/...
context:
  - "State.Reviewer, ReviewRounds, ColdPass and Findings become maps keyed by lens, and Finding gains Lens; economy mode is the same map with one key"
  - "read-only sessions that see only the diff, the task list and the card, never the builder's transcript; lenses run in parallel"
  - "a resumed state.json written before the change still loads, as a single lens"
```

#### [TSK-07.11.2] A re-review resumes its lens, and can only close findings or flag repaired lines [P: C] [DONE]
```yaml
files: [internal/review/rereview.go, internal/review/rereview_test.go]
done_when:
  - go test ./internal/review/...
depends_on: [TSK-07.11.1]
context:
  - "extends TG-06.7's warm reviewer and its bias mitigation to each lens"
  - "test (REQ-21): a new finding on an unchanged line is dropped"
```

#### [TSK-07.11.3] `komodo run` starts one reviewer session per lens [P: H] [DONE]
```yaml
files: [internal/run/drive.go, internal/run/drive_test.go, internal/run/requests.go, internal/run/requests_test.go]
done_when:
  - go test ./internal/run/...
depends_on: [TSK-07.11.1, TSK-07.11.2]
context:
  - "ReviewerRequest takes a lens and binds its skill; every lens runs on the profile's reviewer tier"
  - "test (REQ-19): the ledger shows three lens sessions in full mode and one in economy mode"
```

#### [TSK-07.11.4] internal/conductor/drive.go:274 A pre-lens state.json resumed in full mode loses its open findings [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - "A legacy record loads its verified findings under review.Economy. In full mode, openLens asks s.Open(lens) for correctness, security and quality, and each returns nothing. So no cold lens brief carries the open finding (a.go:3 'nil map' in the test fixture). The loop at drive.go:274 then deletes the Economy entry. The old contract, where a cold reviewer gets the open findings, breaks for exactly the upgrade path TSK-07.11.1 says must still load. When the driver's lenses lack review.Economy, append s.Open(review.Economy) to every cold lens brief before the Economy entry is dropped."
```

#### [TSK-07.11.5] internal/review/rereview_test.go:9 Test comment cites a requirement ID [P: L] [REFINEMENT]
```yaml
files:
  - internal/review/rereview_test.go
done_when:
  - test -f internal/review/rereview_test.go
type: docs
context:
  - "The comment names 'REQ-21'. The comment standard bans citing a ticket or spec. Drop 'is REQ-21' and keep only the behaviour the test asserts."
```

#### [TSK-07.11.6] internal/run/drive_test.go:325 Test comment cites a requirement ID [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/drive_test.go
done_when:
  - test -f internal/run/drive_test.go
type: docs
context:
  - "The comment names 'REQ-19'. The comment standard bans citing a ticket or spec. Drop 'is REQ-19' and keep only the behaviour the test asserts."
```




### [TG-07.12] The line is safe to leave unattended
```yaml
type: fix
version: 1.0.0-alpha.8
depends_on: [TG-07.11]
```
* **Why:** three gaps surfaced while phases 3 and 4 ran: a stopped conductor kept checking and committed, a reproducer can act on the real worktree, and the pre-push hook sees the credential.

#### [TSK-07.12.1] A stopped run stops the station it is in, and never commits after the stop [P: H] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go, internal/check/check.go, internal/check/check_test.go, internal/run/drive_test.go]
done_when:
  - go test ./internal/conductor/... ./internal/check/...
context:
  - "Drive checks its context only between states, and Stations.Check, Prepare and Ship take none; a SIGTERM during Check let the gate finish and commit the build four minutes later"
  - "each station takes the run's context, its commands die with it, and CommitBuild never runs once the context is done"
  - "test: a context cancelled during Check returns within seconds, with no commit and the state left at Checking"
```

#### [TSK-07.12.2] An evidence reproducer runs in a scratch copy that cannot reach the real worktree [P: H] [DONE]
```yaml
files: [internal/review/evidence.go, internal/review/evidence_test.go]
done_when:
  - go test ./internal/review/...
context:
  - "supersedes TSK-07.5.5: the scratch copy keeps the worktree's .git file, so a reproducer's git commands act on the real branch"
  - "the copy gets its own repository, or no .git at all, and GIT_DIR and GIT_WORK_TREE are unset for the reproducer"
  - "test: a reproducer that commits or resets leaves the real worktree's HEAD and index unchanged"
```

#### [TSK-07.12.3] The pre-push hook never sees the credential-bearing push URL [P: H] [DONE]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "supersedes TSK-05.10.9: git passes the push URL to pre-push as an argument, and PushFromWorktree's URL carries the forge token"
  - "push to a named remote whose URL holds no credential, with the token in a credential helper scoped to the push"
  - "test: a pre-push hook that records its arguments and environment sees no token"
```

#### [TSK-07.12.4] internal/conductor/drive.go:709 A stop during CommitBuild's pre-commit hooks still commits [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - "Line.Check tests ctx.Err() only before it calls line.CommitBuild. CommitBuild runs the repo's pre-commit hooks through exec with no context. A SIGTERM that arrives while a slow hook (lint or tests) is running lets the hook finish and the commit land, which is the same late-commit TSK-07.12.1 set out to prevent. Line.Ship has the same gap: its ctx check comes before line.ShipGroup, whose push and PR calls take no ctx. The task's context says each station's commands die with the run's context. Pass ctx into line.CommitBuild and line.ShipGroup so their hook and push commands run under exec.CommandContext."
```

#### [TSK-07.12.5] internal/line/ship.go:526 The push credential helper gives the forge token to any host that asks [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: fix
context:
  - "The helper answers every `get` without reading the `host=` line git sends it. It is registered under the plain `credential.helper` key, which applies to every URL, not just the push host. If the forge redirects the push to another host (http.followRedirects=initial) or an HTTP proxy returns 407, git asks for a credential for that host and the helper returns the forge token. Before this change the token lived in the URL, and git dropped it on a cross-host redirect instead of sending it. Tie the helper to the push URL with `-c credential.<clean-url>.helper=…`, or have it answer only when the host it is asked about matches the push URL's host."
```

#### [TSK-07.12.6] internal/check/check.go:64 check.Exec duplicates proc.run line for line [P: L] [REFINEMENT]
```yaml
files:
  - internal/check/check.go
done_when:
  - test -f internal/check/check.go
type: refactor
context:
  - "Exec repeats proc.run's body: CommandContext, Group, Cancel→KillGroup, a 5s WaitDelay, the watcher, the post-Wait KillGroup, ErrWaitDelay swallowing, and the runaway/timeout/exit-code result mapping. Only the parent ctx and the ctx.Err()==nil guard differ. It also redeclares proc's 5s wait delay as waitDelay, and it drops proc.run's timeout<=0 default. A fix to either copy (process group kill, runaway reporting) now has to land twice. And a process primitive sits in the check package, beside proc.Exec, which does the same job. Give proc.run a ctx parameter and expose proc.ExecContext, then call it from runNamed and coverage and delete check.Exec and waitDelay."
```




### [TG-07.6] Repair resumes the builder, and a loop stops when it stops progressing
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.11]
```
* **Why:** TG-03.31's findings rose from 1 to 7 across fixes with no rule to stop them. Proves REQ-22 and REQ-23.

#### [TSK-07.6.1] Repair resumes the builder with a fix list of verified findings or failed checks [P: C] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/resume.go]
done_when:
  - go test ./internal/conductor/...
context:
  - "shipped with TG-05.10's predecessors: drive.go's fixList, repairBrief and repair, and resume.go's pending repair"
  - "proved by TestDriveRepairsAFailedCheckByResumingTheBuilder, TestDriveStartsAFreshBuilderWhenTheHostCannotResume and TestResumeRestartsALostRepairWithItsFixList"
```

#### [TSK-07.6.2] A round that closes nothing ends the loop, and the group ships as a draft with its findings [P: C] [DONE]
```yaml
files: [internal/conductor/progress.go, internal/conductor/progress_test.go, internal/conductor/drive.go, internal/run/drive.go]
done_when:
  - go test ./internal/conductor/... ./internal/run/...
context:
  - docs/system-design.md#convergence-rules
  - "also stop on a repair that changed no file, a check failing identically after a repair, or a refusal limit"
  - "newDriver sets Driver.Repairs from ReviewRepairs; the conductor stops reading both counts, which stay in the profile until the relay retires"
  - "test (REQ-23): no two rounds hold the same open findings"
```

#### [TSK-07.6.3] A repair names every task's files as the plan holds them now [P: M] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-07.6.2]
context:
  - "a resumed repair gets only the fix list, so a file added to a task's spec mid-run never reaches the builder; TG-07.5's builder blocked twice asking to edit a file its spec had just gained"
  - "the fix list closes with each task's files from the plan the conductor loaded on resume"
  - "a stopped repair resumes its session with only a continue prompt, so a fix list rewritten while it was stopped never reaches the builder; TG-08.4's builder twice undid a file its spec had just gained"
```

#### [TSK-07.6.4] internal/conductor/drive.go:533 Task files close the fix list only in repair(); a stopped repair resumed through Resume never gets them [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - 'TSK-07.6.3''s context says a stopped repair resumes its session with only a continue prompt, so a rewritten fix list never reaches the builder. It names the TG-08.4 builder that twice undid a file. taskFiles(d.Tasks) is appended only at drive.go:533. Resume -> pendingSession (resume.go:100-104) still builds repairBrief(fixList(s.Fixes), ...) with no task files. startOrResume (resume.go:114) still calls Host.Resume(last, ""). Input: state.json at Repairing with SessionDone=false, after a plan edit that gave a task c.go. Outcome: the resumed builder gets an empty prompt, or a fresh one gets a brief without c.go, and the failure the task set out to fix happens again. Have pendingSession/startOrResume send fixList(s.Fixes)+taskFiles(d.Tasks) both as the Host.Resume input and in the fresh repair brief.'
```

#### [TSK-07.6.5] internal/conductor/progress_test.go:12 Test godoc cites a requirement ID [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/progress_test.go
done_when:
  - test -f internal/conductor/progress_test.go
type: docs
context:
  - "The comment above TestDriveNeverHoldsTheSameOpenFindingsForTwoRounds reads 'is REQ-23:'. The comment standard bans citing a version, ticket, spec or PRD. Drop 'is REQ-23' and keep only what the test asserts."
```

#### [TSK-07.6.6] internal/conductor/drive_test.go:837 No test for a stopped repair resumed from Repairing [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive_test.go
done_when:
  - test -f internal/conductor/drive_test.go
type: test
context:
  - "The new test resumes from Checking, so it only reaches Driver.repair. A group stopped mid-repair resumes through pendingSession and startOrResume in resume.go. There, a fresh start builds repairBrief(fixList(s.Fixes)) with no taskFiles(d.Tasks), and a resumed session gets only the empty continue prompt. That is the path TSK-07.6.3's third context item names. Nothing tests it, so a fix list rewritten while the repair was stopped can still miss the builder and done_when still passes. Add a case that resumes a Repairing state with SessionDone false and asserts the builder's input holds the fix list closed by every task's files."
```

#### [TSK-07.6.7] internal/conductor/progress_test.go:92 The refusal-limit test only exercises the old BLOCKED path [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/progress_test.go
done_when:
  - test -f internal/conductor/progress_test.go
type: test
context:
  - "TestDriveStopsARepairEndedByARefusalLimit feeds a repair that returns BLOCKED. That is the escalation TestDriveEscalatesABlockedBuilderAndWaits already covers, and it passes without this diff. No refusal limit is exercised, so the test's name claims a behaviour it does not cover. Either drive the host's refusal-limit outcome in this test, or rename it to the blocked repair it actually covers."
```





### [TG-07.7] Escalations go to the orchestrator, and what it can't settle is written down
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.6]
```
* **Why:** a stuck group needs a decision, and an unattended run needs one without a person (decision 0011). Proves REQ-18 and REQ-45.

#### [TSK-07.7.1] An escalation reaches a headless orchestrator, which returns one allowed action [P: C] [DONE]
```yaml
files: [internal/conductor/escalate.go, internal/conductor/escalate_test.go, internal/conductor/drive.go, internal/run/drive.go, komodo/roles/orchestrator.md, komodo/roles/orchestrator.schema.json]
done_when:
  - go test ./internal/conductor/... ./internal/run/...
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#escalations
  - "Drive has no Escalated case today, so Answered, Stop and Left are set only by hand in state.json"
  - "one headless orchestrator session per escalation; the action is answer, split or clarify (which must pass lint), retry once on heavy, or stop"
  - "the person-present path, komodo status and its hook, lands with TSK-08.4.4"
```

#### [TSK-07.7.2] The escalate skill settles one escalation within its limits [P: H] [DONE]
```yaml
files: [komodo/skills/escalate/SKILL.md]
done_when:
  - test -f komodo/skills/escalate/SKILL.md
  - go run ./cmd/komodo doctor
depends_on: [TSK-07.7.1]
context:
  - "an answer comes only from the task list, the specs and the code; anything that changes scope is a stop"
type: docs
```

#### [TSK-07.7.3] A blocked builder pauses its dependants and escalates [P: H] [DONE]
```yaml
files: [internal/conductor/blocked.go, internal/conductor/blocked_test.go, internal/run/run.go]
done_when:
  - go test ./internal/conductor/... ./internal/run/...
depends_on: [TSK-07.7.1]
context:
  - "dependants are other groups, so the drain in internal/run/run.go holds them"
  - "test (REQ-18) on the conductor's decision; a group that stops twice without progress gets a blocker note, whatever the orchestrator says"
```

#### [TSK-07.7.4] What the orchestrator can't settle becomes a blocker note and a blocked draft PR [P: C] [DONE]
```yaml
files: [internal/backlog/note.go, internal/backlog/note_test.go, internal/conductor/stop.go, internal/conductor/stop_test.go, internal/line/ship.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/backlog/... ./internal/conductor/... ./internal/line/... ./cmd/komodo/...
depends_on: [TSK-07.7.1]
context:
  - docs/system-design.md#blocker-notes
  - "save a WIP commit, set BLOCKED, write the note under the group heading on its branch, and publish a draft PR labelled status: blocked; a headless run exits non-zero; komodo resume feeds the edited group to the resumed builder and removes the note"
  - "one test per path (REQ-45)"
```

#### [TSK-07.7.5] internal/conductor/escalate.go:98 A resumed escalation tells the orchestrator the wrong reason [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/escalate.go
done_when:
  - test -f internal/conductor/escalate.go
type: fix
context:
  - "r.reason is only in memory. Suppose a saved group is at Escalated with no answer, because the orchestrator returned an action outside the enum, failed to start, or was cut off by the budget. On resume, Drive calls escalate with an empty r.reason, and reason() reads Host.Result(lastSession(s)). The last session is now the orchestrator's own session, since session() appended it to s.Sessions. So the second orchestrator, and the blocker note's Items, get 'the builder returned BLOCKED with no question' instead of the builder's question. For a group that escalated on a station error, they get the last builder's summary instead of the failure. Save the escalation reason in State (like Fixes and Repairs), and read it before falling back to the last builder session's result."
```

#### [TSK-07.7.6] internal/conductor/drive.go:143 The one-heavy-retry and two-stall limits reset on every resumed run [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - "round.heavy and round.stalls are not saved to state.json. Drive rebuilds round from s.Fixes, s.Builder and s.Repairs alone. After a budget timeout or interrupt and a resume, the builder falls back from the heavy machine to the standard one, the orchestrator can be granted a second retry, and a group that already stalled once needs two more stalls before it is blocked. That breaks the rules 'retry once on heavy' and 'a group that stops twice without progress gets a blocker note'. Save heavy and stalls in State next to Repairs, and restore them when Drive and Resume build the round."
```

#### [TSK-07.7.7] internal/conductor/stop.go:35 Block drops ShipBlocked's warnings, so an unpublished or unlabelled blocker goes unreported [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/stop.go
done_when:
  - test -f internal/conductor/stop.go
type: chore
context:
  - "Line.Block discards the ShipResult (`_, err := line.ShipBlocked(...)`). ShipBlocked returns nil error with only a warning in two cases: a scrubbed environment, where the note is committed locally and never pushed, and a repo with no `status: blocked` label. In both cases a headless stop looks published when no draft PR exists, or it exists without its label. Nothing logs or surfaces the failure. Return or print result.Warnings from Line.Block, and make a scrubbed, unpublished blocker count as a failure the headless run reports."
```

#### [TSK-07.7.8] komodo/roles/orchestrator.md:5 Orchestrator gets edit and write on the whole worktree, and only its prompt keeps it to the backlog file [P: L] [REFINEMENT]
```yaml
files:
  - komodo/roles/orchestrator.md
done_when:
  - test -f komodo/roles/orchestrator.md
type: fix
context:
  - "The role grants [read, edit, write, search], and 'never touch a file outside the group's backlog file' is prose only. After a split or clarify, the conductor only lints the backlog (internal/conductor/escalate.go). An edit to any other file stays in the worktree and ends up in the WIP commit or the shipped branch unchecked. Under the cooperative-model threat model only the guard would catch this, so it is a note. After a split or clarify, refuse the action when git diff shows changes outside the group's backlog file."
```

#### [TSK-07.7.9] internal/line/ship.go:372 ShipBlocked copies ShipGroup's create-or-refresh pull request fallback [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: refactor
context:
  - "The Create → View → OPEN check → Edit(--title, --body) → url = open.URL block is a line-for-line copy of the one at the end of ShipGroup. The declared-files loop at line 322 is a copy of ShipGroup's too. A fix to one copy will not reach the other. Move the create-or-refresh block into one helper, e.g. openOrRefresh(client, base, branch, title, body, draft), and call it from both ShipGroup and ShipBlocked."
```

#### [TSK-07.7.10] internal/run/drive.go:125 newDriver's heavy machine and builder-effort override have no test [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/drive.go
done_when:
  - test -f internal/run/drive.go
type: test
context:
  - "newDriver sets the heavy machine from plan.Profile.Tiers.Heavy and replaces its Effort with the builder machine's Effort. TestNewDriverWiresTheOrchestratorLintAndBlock checks Block, Orchestrator and Lint but never Heavy. The conductor tests inject Heavy themselves. If Heavy were left empty, every retry would turn into a stop and all done_when tests would still pass. Extend TestNewDriverWiresTheOrchestratorLintAndBlock to assert driver.Heavy.Model equals the profile's heavy tier and driver.Heavy.Effort equals the builder machine's effort."
```

#### [TSK-07.7.11] internal/conductor/escalate.go:145 startBuilder's fallback when a resume fails has no test [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/escalate.go
done_when:
  - test -f internal/conductor/escalate.go
type: test
context:
  - "When the host can resume but Host.Resume returns an error, startBuilder starts a fresh builder with the answer added to its brief. The only fresh-builder test turns resume capability off, so it never reaches this branch after a failed Resume. Add a case where the fake host's Resume returns an error, and assert that a fresh builder starts whose brief ends with answerLead plus the answer."
```








### [TG-07.13] Escalations survive a restart, and a group can be abandoned
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.7]
```
* **Why:** TG-07.7's builder ran out of turns before `komodo abandon`, and noted that an escalation's working data lives only in memory.

#### [TSK-07.13.1] `komodo abandon` removes a group on purpose [P: M] [DONE]
```yaml
files: [internal/conductor/abandon.go, internal/conductor/abandon_test.go, cmd/komodo/main.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/conductor/... ./cmd/komodo/...
context:
  - "was TSK-07.7.5; builds on backlog.AddNote and the blocker note TG-07.7 added"
  - "removes the group's worktree and branch, and marks its file BLOCKED with a note saying it was abandoned"
```

#### [TSK-07.13.2] An escalation's reason, answer, stall count and heavy retry survive a restart [P: H] [DONE]
```yaml
files: [internal/conductor/state.go, internal/conductor/state_test.go, internal/conductor/drive.go, internal/conductor/drive_test.go]
done_when:
  - go test ./internal/conductor/...
context:
  - "TG-07.7 keeps them in the in-memory round, so a killed run resumes an escalation with none of them"
  - "test: a run killed while escalated resumes with the same reason, answer, stall count and retry flag"
```

#### [TSK-07.13.3] Publishing a blocked group is tested over a real repository [P: M] [DONE]
```yaml
files: [internal/line/ship_test.go, internal/line/epic.go]
done_when:
  - go test ./internal/line/...
context:
  - "ShipBlocked has no test over a real repo; the WIP push runs the pre-push hook, so a gate that refuses unverified work fails the publish and the note stays local"
  - "labelBlocked mirrors labelWip in epic.go; one shared helper labels both"
```

#### [TSK-07.13.4] internal/conductor/drive.go:158 A stop's `needs` is lost when a run is killed after the stop is answered [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/drive.go
done_when:
  - test -f internal/conductor/drive.go
type: fix
context:
  - "keep persists reason, answer, stalls and heavy but not r.needs. escalate (ActionStop, lint failure or spent retry) and stalled set s.Stop, s.Answered and r.needs, and the new save at drive.go:202 records Answered+Stop. If the run is killed before Blocked runs, the resumed Drive skips escalate because Answered is set. It enters Blocked, and stop() writes needsDefault instead of the orchestrator's `needs` or the 'stopped N times' text. The blocker note then names the wrong need. Add a Needs field to State and carry it in newRound and keep alongside Reason."
```

#### [TSK-07.13.5] internal/line/epic.go:111 labelBlocked still duplicates labelNamed instead of using it [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/epic.go
done_when:
  - test -f internal/line/epic.go
type: refactor
context:
  - 'TSK-07.13.3 says one shared helper labels both status: wip and status: blocked. labelNamed now exists, but labelBlocked in ship.go:788 still lists and matches labels by itself. The two copies can drift apart. Make labelBlocked return labelNamed(client, url, "status: blocked").'
```

#### [TSK-07.13.6] internal/conductor/abandon.go:72 The blocker note is written only after the worktree and branch are already gone [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/abandon.go
done_when:
  - test -f internal/conductor/abandon.go
type: fix
context:
  - "Abandon force-removes the worktree and deletes the branch first, and only then writes the backlog. It does the same when branch -D fails, for example because the branch is checked out in the root. If the backlog write or branch -D fails, the group's work is gone, no note records it, and the run record remains. Write the noted backlog before the destructive git steps, or restore it if a later step fails."
```

#### [TSK-07.13.7] internal/line/epic.go:111 labelNamed has one caller, and labelBlocked still duplicates its body [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/epic.go
done_when:
  - test -f internal/line/epic.go
type: refactor
context:
  - 'The diff pulls out labelNamed(client, url, name) so both labels can use it, but only labelWip calls it. labelBlocked (internal/line/ship.go:788) still does the same work on its own: it lists labels, matches by whole name or name+" ", adds the label, and returns the same three warnings. That leaves a helper with one caller plus the duplicate it was meant to remove. TSK-07.13.3 asked for ''one shared helper labels both''. Make labelBlocked return labelNamed(client, url, blockedLabel) and delete its loop.'
```





### [TG-07.8] Prepare, then ship draft-first
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.7]
```
* **Why:** unverified work looked ready, and a missing credential lost work at the last step. Proves REQ-24, REQ-25 and REQ-27.

#### [TSK-07.8.1] Prepare commits, runs the hooks and catches up, with conflicts as a repair round [P: C] [DONE]
```yaml
files: [internal/conductor/drive.go, internal/conductor/drive_test.go, internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/conductor/... ./internal/line/...
context:
  - docs/system-design.md#prepare-and-ship
  - "Line.Prepare and Line.Ship live in drive.go and wrap internal/line; catchUp in ship.go returns an error on a conflict instead of a repair round"
  - "the commit carries the ticked list and the CHANGELOG line, deletes the epic's files when it is the epic's last open group, and has no trailers"
  - "test (REQ-24): the ledger shows no push before these checks pass"
```

#### [TSK-07.8.2] Integration test-merges every ready group, and plans the stack [P: H] [DONE]
```yaml
files: [internal/conductor/integrate.go, internal/conductor/integrate_test.go, internal/run/run.go, internal/run/run_test.go]
done_when:
  - go test ./internal/conductor/... ./internal/run/...
depends_on: [TSK-07.8.1]
context:
  - "a failure is a repair round for the group that caused it; a child of an unmerged parent targets the parent's branch, and is rebased and retargeted when the parent merges"
```

#### [TSK-07.8.3] Every PR opens as a draft, or labelled status: wip where drafts are unavailable [P: C] [DONE]
```yaml
files: [internal/pr/pr.go, internal/pr/pr_test.go, internal/line/ship.go, internal/line/ship_test.go, internal/line/epic.go]
done_when:
  - go test ./internal/pr/... ./internal/line/...
depends_on: [TSK-07.8.1]
context:
  - "the epic PR already falls back through createEpicPull and labelWip in epic.go; ShipGroup reuses that path"
  - "it turns ready for review only once every check and review passed; a test for each path (REQ-25)"
  - "internal/profile/profile.go maps to scope/agents, and a failed label call is a tested warning (from TSK-03.31.10 and TSK-03.31.12)"
```

#### [TSK-07.8.4] A missing or expired credential stops a group before Ship, and `komodo ship` finishes it [P: C] [DONE]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go, cmd/komodo/line.go, cmd/komodo/main.go]
done_when:
  - go test ./internal/line/... ./cmd/komodo/...
depends_on: [TSK-07.8.3]
context:
  - docs/system-design.md#run-failures
  - "ShipGroup already writes a handoff when the environment is scrubbed; komodo ship reads it back and finishes the push and PR"
  - "the group keeps its commits and gets a blocker note; other groups continue; --no-ship stops each group before Ship (REQ-27)"
```

#### [TSK-07.8.5] internal/run/run.go:204 Restack rebases the worktree of a group another lane is still building [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/run.go
done_when:
  - test -f internal/run/run.go
type: fix
context:
  - "drain calls restack after each shipped group while other lanes are still running. Restack goes through every RunState, including a child whose lane is still running, and runs rebase or merge with --autostash in that child's worktree (integrate.go:137). This happens once the child's parent has merged, so git runs underneath a builder session that is still working. Pass the running group IDs into Restack and skip them, or restack a group only when its own lane starts."
```

#### [TSK-07.8.6] internal/line/ship.go:723 FinishShip commits the blocker note's removal before a push that can still be refused [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: fix
context:
  - "dropCredentialNote removes and commits the credential note before PushFromWorktree runs. If the credential is still missing or expired, FinishShip returns ErrNoCredential and the handoff stays, but the branch no longer has its blocker note. That breaks REQ-27, which requires a stopped group to keep its note. Push first and drop the note only after the push succeeds, or write the note again on ErrNoCredential."
```

#### [TSK-07.8.7] internal/line/ship.go:248 In a headless run the credential stop and the draft/wip fallback never apply [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: fix
context:
  - "komodo run launches scrubbed, so ShipGroup always takes the scrubbed handoff branch. The push and the PR then happen in run.go finishShip. That code returns a credential refusal as a plain lane error with no blocker note. It also calls client.Create(..., handoff.Draft) directly instead of createEpicPull, so a forge that refuses drafts fails the ship when it should open the PR labelled status: wip. The TSK-07.8.3 and 07.8.4 behaviour only holds when ship runs unscrubbed. Have run.go finishShip call line.FinishShip, or share its credential stop and its createEpicPull path."
```

#### [TSK-07.8.8] internal/line/ship.go:261 The credential-note commit makes staleReview reject any later ShipGroup retry [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: fix
context:
  - "writeCredentialNote commits 'docs: ... waits on a forge credential' after the review result is written. staleReview (step.go:312-321) ignores only commits whose subject is the ship subject, so it counts this commit as new work. Any later ShipGroup retry then fails with 'changed after its review' and asks for a new review over a docs-only commit. That covers an escalation retrying Ship and `komodo line ship` once the credential is back. Make staleReview skip the credential-note commits, or fold the note into the ship commit."
```

#### [TSK-07.8.9] internal/line/ship.go:719 FinishShip checks the handoff's group and branch but trusts its worktree path [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: fix
context:
  - "The comment says ship.json is agent-writable, and FinishShip checks Group and Branch. It then uses handoff.Worktree unchecked. dropCredentialNote writes and commits BACKLOG.md in that directory, and PushFromWorktree pushes that directory's branch to the root's origin with the forge credential. A stale or wrong worktree in ship.json therefore publishes another checkout's tree under the group's branch. No reproducer; this is a note. Derive the worktree from the group's saved RunState (WorktreePath(root, state.Worktree)) and refuse a handoff whose Worktree differs."
```

#### [TSK-07.8.10] internal/conductor/integrate.go:60 TestMerge passes a run-state branch to git merge with no end-of-options marker [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/integrate.go
done_when:
  - test -f internal/conductor/integrate.go
type: fix
context:
  - 'other.Branch comes from a run-state file and goes to `git merge --no-edit` as a bare argument. A value that starts with ''-'' is read as an option, for example --strategy=<name>, which runs git-merge-<name> from PATH. Restack''s client.Edit(state.Branch, ...) has the same shape. No reproducer; this is a note. Insert "--end-of-options" before other.Branch, or skip any ready branch that fails git check-ref-format --branch.'
```

#### [TSK-07.8.11] internal/line/ship.go:495 rebaseForRepair copies catchUp's preamble line for line [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: refactor
context:
  - "rebaseForRepair repeats catchUp (ship.go:870) almost word for word: the fetch guarded by hasOrigin, the StartRef target, the rev-parse and is-ancestor early returns, and the choice between rebase and merge for a pushed branch, comments included. The only real difference is how each handles a conflict, so a fix to one copy can be missed in the other. Extract the shared target/up-to-date/rebase-or-merge selection into one helper that both catchUp and rebaseForRepair call."
```

#### [TSK-07.8.12] internal/conductor/integrate.go:157 restackOnto copies the rebase-or-merge choice a third time [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/integrate.go
done_when:
  - test -f internal/conductor/integrate.go
type: refactor
context:
  - "restackOnto makes the same decision again: rebase a local branch, merge into a pushed one, abort if it fails. It repeats the same comment word for word. This is the third copy of the logic in catchUp and rebaseForRepair, and it detects a pushed branch in its own way (rev-parse refs/remotes/origin) instead of calling onOrigin. Export one line helper for the rebase-or-merge step and call it from restackOnto."
```

#### [TSK-07.8.13] internal/line/ship.go:738 FinishShip copies ShipGroup's create-or-reuse PR block [P: L] [REFINEMENT]
```yaml
files:
  - internal/line/ship.go
done_when:
  - test -f internal/line/ship.go
type: refactor
context:
  - "FinishShip copies ShipGroup's steps (ship.go:275-285): call createEpicPull, fall back to client.View when a PR is still OPEN, then ApplyLabels and merge the wip label with the warnings. If the draft-first logic changes, both copies have to change. Extract an openDraftPull(client, base, branch, title, body, labels) helper and call it from both ShipGroup and FinishShip."
```










### [TG-07.9] Cleanup is mechanical
```yaml
type: feat
version: 1.0.0-alpha.8
depends_on: [TG-07.8]
```
* **Why:** stale runs and worktrees were cleared by hand after squash merges. Proves REQ-46.

#### [TSK-07.9.1] Ship and the next run remove merged and abandoned groups' leftovers [P: H] [DONE]
```yaml
files: [internal/doctor/prune.go, internal/doctor/prune_test.go, internal/run/drive.go, internal/run/drive_test.go]
done_when:
  - go test ./internal/doctor/... ./internal/run/...
context:
  - docs/system-design.md#the-backlog
  - "doctor.Prune exists, but only komodo doctor calls it; the run calls it after Ship and before it cuts a group"
  - "worktrees, local branches and sessions go; a squash-merged group whose branch is gone settles too (from TSK-03.32.8); only the last 10 run folders stay"
```

#### [TSK-07.9.2] `komodo sync` opens a cleanup PR for an epic whose files outlived it [P: M] [DONE]
```yaml
files: [internal/run/sync.go, internal/run/sync_test.go]
done_when:
  - go test ./internal/run/...
context:
  - "only when every group of the epic has shipped; the PR deletes the epic's group files"
```

#### [TSK-07.9.3] Doctor names each kind of leftover [P: H] [DONE]
```yaml
files: [internal/doctor/leftovers.go, internal/doctor/leftovers_test.go, internal/doctor/doctor.go]
done_when:
  - go test ./internal/doctor/...
context:
  - "an ended epic's files, and a worktree or branch with no group; one test per kind (REQ-46)"
  - "wired into doctor's Run, beside HostLeftovers and StrayWorktrees"
```

#### [TSK-07.9.4] internal/doctor/prune.go:64 Prune now sweeps an open run's task-lane worktrees, since only the group worktree and branch are protected [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/prune.go
done_when:
  - test -f internal/doctor/prune.go
type: fix
context:
  - "Before this change settleShippedRun returned early whenever line.RunIsOpen. Now it skips only each open run's group Branch and Worktree. Task lanes (.komodo/wt/TSK-*, branch task/tsk-*, cut from the group branch by line.WriteBrief) are not in the `running` map. Take a freshly cut lane with no commits yet: its tip equals the group branch, which is an ancestor of origin/base, and the brief lives under the gitignored .komodo, so `status --porcelain` is empty. landed() therefore returns true, and the lane is removed with `worktree remove --force` and `branch -D`. This happens while its session runs whenever another group's Drive calls pruneLeftovers before its cut, or when `komodo doctor` prunes during a run. This is a note because the reproducer was not run. Skip every worktree under .komodo/wt whose base name is an open run's group id or one of its task ids, not just the run's recorded group worktree."
```

#### [TSK-07.9.5] internal/run/sync.go:132 A group file whose task lines fail to parse counts as ended, so sync opens a PR deleting it [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/sync.go
done_when:
  - test -f internal/run/sync.go
type: fix
context:
  - "endedEpics marks an epic open only when a parsed task is unticked. A group file with an epic set but zero parsed tasks never sets open[epic]. That includes a file with ParseGroupFile Problems, such as a task checkbox that misses groupFileTaskLine. If that epic has no other open file, sync pushes chore/cleanup-<epic> deleting the live file. doctor's endedEpicFiles (leftovers.go:67) has the same hole and falsely reports the file as ended. This is a note because the reproducer was not run. Treat an epic as open when any of its files has Problems or no parsed tasks."
```

#### [TSK-07.9.6] internal/run/sync.go:165 A failure partway through openCleanup leaves state that later syncs never recover [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/sync.go
done_when:
  - test -f internal/run/sync.go
type: fix
context:
  - "If PushFromWorktree or the commit fails, the local branch and the registered worktree at .komodo/wt/cleanup-<epic> stay behind. The next sync's AddWorktree then runs `worktree add` onto an already-registered path and errors, so every sync fails from then on. If client.Create fails after the push, origin holds the branch, and every later sync prints 'already has' and never opens the PR. The worktree and local branch also leak. This is a note because the reproducer was not run. On any error after AddWorktree, remove the worktree and local branch, and delete the pushed branch if Create fails."
```

#### [TSK-07.9.7] internal/doctor/leftovers.go:98 Doctor reports epic and cleanup worktrees as orphans [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/leftovers.go
done_when:
  - test -f internal/doctor/leftovers.go
type: fix
context:
  - "orphanWorktrees flags every worktree under the main checkout's .komodo/wt whose base name is not an open group or task id. An epic worktree (for example .komodo/wt/epic-alpha.8, the layout this repo itself uses) and sync's transient cleanup-<epic> worktree never match a group id, so doctor tells the user to remove a live epic worktree. The TSK-07.9.3 context says only 'a worktree or branch with no group' should be named. This is a note because the reproducer was not run. Exclude worktrees whose branch is an open group's base (the epic branch) and the cleanup-* worktrees, or key ownership on branch rather than directory name."
```

#### [TSK-07.9.8] internal/doctor/prune.go:118 pruneRuns removes RunDir(root, state.Group) without checking that Group is a plain id [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/prune.go
done_when:
  - test -f internal/doctor/prune.go
type: fix
context:
  - 'LoadRuns checks the directory name with PlainGroup, but the Group it returns comes from the JSON inside run.json, and nothing checks that value. If a run.json holds Group "..", pruneRuns calls os.RemoveAll on root/.komodo, deleting every run folder and worktree. An empty Group is filtered out, but ".." and "a/../.." are not. The file lives under .komodo and only the tool writes it, so under the cooperative threat model this is a note, not a blocker. Skip any state where !line.PlainGroup(state.Group) or state.Group differs from its directory name before os.RemoveAll.'
```

#### [TSK-07.9.9] internal/run/drive_test.go:195 No test pins the prune after Ship apart from the prune before the cut [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/drive_test.go
done_when:
  - test -f internal/run/drive_test.go
type: test
context:
  - "Drive calls pruneLeftovers in cutIfNeeded (drive.go:111) and again after Shipped (drive.go:74). TestRunClearsASquashMergedGroupsLeftoversBeforeItCuts only checks that the stale TG-39.1 worktree and feat/old are gone at the end. Either call alone removes them, so deleting the post-Ship call still passes. The test name promises 'before it cuts' but nothing checks the order. Add a case where the leftover only appears after the cut (for example, origin deletes the branch while the fake session runs) and assert the post-Ship prune removes it."
```

#### [TSK-07.9.10] internal/doctor/leftovers_test.go:22 leftoverRepo repeats pruneRepo from the same package [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/leftovers_test.go
done_when:
  - test -f internal/doctor/leftovers_test.go
type: refactor
context:
  - "leftoverRepo repeats pruneRepo (prune_test.go:19) line for line: gitRepo, write BACKLOG.md and .gitignore, commitAll, and the same git runner closure. The only difference is that the backlog is fixed to openBacklog. Both helpers are new in this diff and live in package doctor. Delete leftoverRepo and call pruneRepo(t, openBacklog) in the three leftovers tests."
```

#### [TSK-07.9.11] internal/doctor/prune.go:13 keptRuns godoc hedges with 'a starting value' [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/prune.go
done_when:
  - test -f internal/doctor/prune.go
type: docs
context:
  - "The Comments standard bans hedges and hypotheticals about states the code does not reach. 'a starting value' suggests the number might change later instead of saying what it is. Drop '; a starting value' so the comment reads 'keptRuns is how many run folders Prune keeps, the newest by start.'"
```









### [TG-07.10] Phase 3 exit proof
```yaml
type: chore
version: 1.0.0-alpha.8
depends_on: [TG-07.9]
```
* **Why:** the phase exits on a 3-group plan run unattended; this repo's move to group files is now TSK-10.7.8.

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
*Goal: the success criteria hold on macOS, Linux and Windows. Ships as `1.0.0-beta.2`.*

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
context:
  - docs/system-design.md#cross-platform-macos-linux-windows
  - "standard library only: kernel32 through syscall.NewLazyDLL; the test runs on Windows and kills a child that outlives its parent (REQ-43)"
  - "process_windows.go is a stub today: Group does nothing and KillGroup kills only the parent; vet proves it compiles, a Windows host proves it works"
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
files: [internal/preflight/preflight.go, internal/preflight/preflight_test.go]
done_when:
  - go test ./internal/preflight/...
context:
  - "checkSandbox handles darwin and linux and refuses anything else; native Windows runs without a sandbox and says so; inside WSL2 a repo under /mnt/ is refused, naming the Linux home as the fix"
```

### [TG-08.3] One command installs the line
```yaml
type: feat
version: 1.0.0-beta.2
```
* **Why:** `komodo` is no one's command until it is installed, and manual steps drift (decision 0019). Proves REQ-1 and REQ-39's global render.

#### [TSK-08.3.1] install.sh installs on macOS, Linux and WSL2, and running it again updates [P: C] [DONE]
```yaml
files: [install.sh, internal/install/script_test.go]
done_when:
  - sh -n install.sh
  - go test ./internal/install/...
context:
  - docs/system-design.md#install
  - "name any missing prerequisite and how to get it; build with Go, or download the pinned release and verify its checksum; symlink onto PATH; komodo install; komodo init inside a repo; komodo doctor; the test runs it under a temp HOME"
```

#### [TSK-08.3.2] install.ps1 does the same on native Windows [P: C] [DONE]
```yaml
files: [install.ps1]
done_when:
  - test -f install.ps1
  - grep -q 'komodo install' install.ps1
context:
  - "a small wrapper on PATH instead of a symlink, since symlinks need admin rights"
```

#### [TSK-08.3.3] `komodo install` adds only the orchestrator layer to the global host config [P: H] [DONE]
```yaml
files: [internal/install/install.go, internal/install/install_test.go, internal/mount/claude/claude.go, internal/mount/claude/claude_test.go, cmd/komodo/host.go]
done_when:
  - go test ./internal/install/... ./internal/mount/claude/...
context:
  - docs/system-design.md#skills-and-scoping
  - docs/system-design.md#install
  - "no global render path exists today; this adds one, wired through cmd/komodo/host.go's --host dispatch"
  - "the guard hook, the orchestrator skills and the status hook; no builder, lens or standards skill; test (REQ-39) on the global render"
  - "every test renders under a temp HOME; nothing a test runs writes the real home directory"
```

#### [TSK-08.3.4] internal/mount/claude/claude.go:182 Global guard and status hooks exit 1 in every Claude session outside a git repo [P: L] [REFINEMENT]
```yaml
files:
  - internal/mount/claude/claude.go
done_when:
  - test -f internal/mount/claude/claude.go
type: fix
context:
  - "RenderGlobal registers `<binary> guard` on PreToolUse and `<binary> hook status --host claude` on SessionStart in the user-level settings, so they run in every Claude Code session on the machine. main() calls repoRoot() before dispatch and fail()s with exit 1 ('no git repository above ...') when the working directory is outside a repo. So a session started in ~ or /tmp gets a hook error on start and on every tool call, and the guard enforces nothing there. Make the guard and hook subcommands exit 0 silently when no git repository is found, or only run them when the cwd is in a repo."
```

#### [TSK-08.3.5] cmd/komodo/host.go:71 The --global dispatch in runInstall has no test [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/host.go
done_when:
  - test -f cmd/komodo/host.go
type: test
context:
  - "The global render and GlobalPlan are unit-tested, but no test runs `komodo install --global`. So a regression that ignores the flag, passes the unresolved binary instead of hook, or skips dry-run would still pass the done_when `go test ./internal/install/... ./internal/mount/claude/...`. Add a cmd/komodo test that runs install --global --dry-run and install --global under a temp HOME and asserts on the planned and written ~/.claude files."
```

#### [TSK-08.3.6] install.ps1:95 install.ps1's checksum check, wrapper and PATH edit are untested [P: L] [REFINEMENT]
```yaml
files:
  - install.ps1
done_when:
  - test -f install.ps1
type: test
context:
  - "The done_when for TSK-08.3.2 is only `test -f` and a grep. A regression in the SHA256SUMS parsing or comparison, the komodo.cmd wrapper body, or the user PATH update would still pass it. install.sh has an equivalent Go subprocess test; install.ps1 has none. Add a Windows-only (or pwsh) subprocess test like script_test.go that runs install.ps1 under a temp USERPROFILE and LOCALAPPDATA and asserts the wrapper, the checksum refusal and the komodo calls."
```




### [TG-08.4] The orchestrator drives the line from the primary session
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.3]
```
* **Why:** the primary session is the one place a person talks to the line (decision 0005). Proves REQ-39.

#### [TSK-08.4.1] The komodo skill is generated from `komodo help`, and the gate fails when it drifts [P: H] [DONE]
```yaml
files: [cmd/komodo/main.go, cmd/komodo/help.go, cmd/komodo/help_test.go, komodo/skills/komodo/SKILL.md]
done_when:
  - go test ./cmd/komodo/...
  - go run ./cmd/komodo doctor
```

#### [TSK-08.4.2] The plan and adhoc skills join run and escalate [P: H] [DONE]
```yaml
files: [komodo/skills/backlog, komodo/skills/plan/SKILL.md, komodo/skills/adhoc/SKILL.md, internal/mount/claude/claude.go]
done_when:
  - test ! -d komodo/skills/backlog
  - go run ./cmd/komodo doctor
context:
  - docs/system-design.md#orchestrator-commands
  - "the global render's orchestratorSkills in claude.go names the new skills, since backlog no longer ships"
  - "backlog becomes plan: /plan drafts groups through the planner, and they must pass lint; adhoc runs /build, /review and /ship through komodo stage"
type: docs
```

#### [TSK-08.4.3] `komodo stage` runs one stage ad hoc on a group or the current branch [P: H] [DONE]
```yaml
files: [cmd/komodo/main.go, cmd/komodo/stage.go, cmd/komodo/stage_test.go, internal/conductor/stage.go, internal/conductor/stage_test.go]
done_when:
  - go test ./cmd/komodo/... ./internal/conductor/...
context:
  - "the ledger records the ad hoc stage (REQ-39)"
depends_on: [TSK-08.4.1]
```

#### [TSK-08.4.4] `komodo status` and the status hook show groups, time and blockers [P: H] [DONE]
```yaml
files: [internal/hooks/status.go, internal/hooks/status_test.go, internal/hooks/hooks.go, cmd/komodo/line.go]
done_when:
  - go test ./internal/hooks/... ./cmd/komodo/...
context:
  - "the status hook is registered in hooks.Table, the caller that wires it in"
  - "status --watch refreshes in place; the SessionStart hook adds the run's status and any blocked groups to the orchestrator's context"
```

#### [TSK-08.4.5] cmd/komodo/line.go:131 status --watch panics on a zero or negative --interval [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/line.go
done_when:
  - test -f cmd/komodo/line.go
type: fix
context:
  - "runStatus passes the user's --interval to watchStatus without checking it. watchStatus calls time.NewTicker(interval), and Go's time.NewTicker panics when the interval is zero or negative. So `komodo status --watch --interval 0` (or `-1s`) crashes with a stack trace instead of an error. Not reproduced here: this session could not run commands. In runStatus, fail with a usage error when *interval <= 0, before calling watchStatus."
```

#### [TSK-08.4.6] internal/hooks/status.go:20 time_used serialises as nanoseconds with no unit in its name [P: L] [REFINEMENT]
```yaml
files:
  - internal/hooks/status.go
done_when:
  - test -f internal/hooks/status.go
type: chore
context:
  - "GroupStatus.TimeUsed is a time.Duration, so `status --json` prints nanoseconds (for example 90000000000 for 1m30s) under the key `time_used`, which names no unit. A consumer reading it as seconds is off by a factor of 1e9. Serialise seconds under a `time_used_seconds` key, or add a unit to the field's name."
```



### [TG-08.5] Plugin points ship disabled
```yaml
type: feat
version: 1.0.0-beta.2
```
* **Why:** Slack, Google Chat and cloud commands come later as plugins, not conductor changes (decision 0020). Proves REQ-42.

#### [TSK-08.5.1] A plugin is a manifest; all three types load disabled, and doctor lists them [P: H] [DONE]
```yaml
files: [internal/plugin/plugin.go, internal/plugin/plugin_test.go, internal/doctor/doctor.go]
done_when:
  - go test ./internal/plugin/... ./internal/doctor/...
context:
  - docs/system-design.md#plugins
  - "a manifest names its type, roles, stages and settings; enabling is per machine under ~/.komodo; a malformed manifest is a doctor problem"
```

#### [TSK-08.5.2] The conductor calls notifiers, tool packs and stage hooks once enabled [P: M] [DONE]
```yaml
files: [internal/conductor/plugins.go, internal/conductor/plugins_test.go]
done_when:
  - go test ./internal/conductor/...
depends_on: [TSK-08.5.1]
context:
  - "a notifier gets blocker notes and run summaries and decides nothing; a tool pack adds commands to a role's allow list behind the guard; a stage hook runs before or after a stage and can stop the group with a reason"
```

#### [TSK-08.5.3] internal/doctor/doctor.go:107 PluginStates is never called, so doctor does not list plugins [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/doctor.go
done_when:
  - test -f internal/doctor/doctor.go
type: fix
context:
  - "cmd/komodo/host.go:195-198 prints only HostLeftovers and StrayWorktrees. Nothing outside doctor_test.go calls PluginStates, so running `komodo doctor` never prints the three plugin types or their enabled or disabled state. TSK-08.5.1 requires that doctor lists them. The unit test passes only because it calls the function directly. Print doctor.PluginStates(root) next to the StrayWorktrees notes in cmd/komodo/host.go."
```

#### [TSK-08.5.4] internal/conductor/plugins.go:34 The conductor never loads or calls plugins [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/plugins.go
done_when:
  - test -f internal/conductor/plugins.go
type: fix
context:
  - "Only plugins_test.go calls LoadPlugins, Notify, Tools and Hook. No conductor or run path loads the enabled plugins. In a real run, an enabled notifier gets no blocker note, a tool pack adds nothing to any role's allow list, and a before-ship stage hook never runs and so can never stop the group. TSK-08.5.2 requires that the conductor calls them once enabled. Call LoadPlugins when the conductor starts a group, call Hook around each stage, call Notify on blockers and summaries, and merge Tools(role) into the role's allow list before the guard reads it."
```

#### [TSK-08.5.5] internal/conductor/plugins.go:106 A setting name that is not a shell identifier produces an env var sh cannot read [P: L] [REFINEMENT]
```yaml
files:
  - internal/conductor/plugins.go
done_when:
  - test -f internal/conductor/plugins.go
type: fix
context:
  - 'A setting such as {"api-key":"x"} becomes KOMODO_SETTING_API-KEY=x. That is not a valid sh identifier, so $KOMODO_SETTING_API-KEY expands to an empty $KOMODO_SETTING_API followed by the literal text ''-KEY''. The notifier or hook silently gets the wrong value. malformed() accepts any setting name. Reject setting names outside [A-Za-z0-9_] in plugin.malformed, or map every other character to ''_'' in pluginEnv.'
```




### [TG-08.6] Releases publish, and product repos pin one
```yaml
type: feat
version: 1.0.0-beta.2
```
* **Why:** product repos run a published release, never a local build (decision 0018). Proves REQ-5's second half and REQ-2's release pin.

#### [TSK-08.6.1] `komodo release` builds, tests, checksums and publishes a GitHub Release [P: H] [DONE]
```yaml
files: [cmd/komodo/release.go, internal/release/publish.go, internal/release/publish_test.go]
done_when:
  - go test ./internal/release/... ./cmd/komodo/...
context:
  - docs/system-design.md#binaries-and-releases
  - "runs from the owner's machine only; the forge credential is read by the release step alone, as at Ship"
```

#### [TSK-08.6.2] The release skill drives it from the orchestrator [P: M] [DONE]
```yaml
files: [komodo/skills/release/SKILL.md]
done_when:
  - test -f komodo/skills/release/SKILL.md
  - go run ./cmd/komodo doctor
context:
  - "the version bump and the changelog heading follow decision 0023"
type: docs
```

#### [TSK-08.6.3] Product repos run the pinned published release [P: H] [DONE]
```yaml
files: [internal/run/sync.go, internal/run/sync_test.go, internal/doctor/pins.go]
done_when:
  - go test ./internal/run/... ./internal/doctor/...
context:
  - "outside this repo, sync and install fetch the profile's pinned release and verify its checksum; doctor fails on any other"
```

#### [TSK-08.6.4] Releases build every platform's binary [P: H] [DONE]
```yaml
files: [internal/release/release.go, internal/release/release_test.go, internal/gate/gate.go, internal/gate/gate_test.go]
done_when:
  - go test ./internal/release/... ./internal/gate/...
context:
  - "Targets lists darwin/arm64, windows/amd64 and linux/amd64 today, and gate's rebuild wrapper has no linux/arm64 case"
  - "darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and windows/amd64, byte-identical per commit (decision 0002)"
```

#### [TSK-08.6.5] internal/run/sync.go:29 SHA256SUMS manifest name duplicated beside release.SumsFile [P: L] [REFINEMENT]
```yaml
files:
  - internal/run/sync.go
done_when:
  - test -f internal/run/sync.go
type: refactor
context:
  - 'The diff adds exported release.SumsFile = "SHA256SUMS" (internal/release/publish.go:19) and a private run.releaseSums with the same value. run already depends on release through doctor (internal/doctor/doctor.go imports komodo/internal/release), so the two constants can drift apart. If publish renames the manifest, sync keeps fetching the old name and every product repo''s sync fails with a 404. Delete releaseSums and use release.SumsFile in syncRelease.'
```

#### [TSK-08.6.6] internal/doctor/pins.go:157 ReleaseVersion execs the installed binary with no context or deadline [P: L] [REFINEMENT]
```yaml
files:
  - internal/doctor/pins.go
done_when:
  - test -f internal/doctor/pins.go
type: chore
context:
  - "The Go standard says every blocking call has a deadline and takes ctx first. ReleaseVersion runs ~/.komodo/bin/komodo-<os>-<arch> version through exec.Command with no timeout. doctor's checkPinnedRelease and sync's syncRelease both call it. If a corrupt or stale binary hangs, doctor and sync hang in every product repo. Take ctx first and run exec.CommandContext under a short context.WithTimeout."
```



### [TG-08.7] The golden suite and `komodo eval`
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.4]
```
* **Why:** a number decides readiness, never a model's score (decision 0021). Proves REQ-44, and the eval cases behind REQ-3, REQ-6, REQ-12, REQ-14, REQ-27, REQ-32, REQ-34 and REQ-40.

#### [TSK-08.7.1] The suite format and `komodo eval --list` [P: C] [DONE]
```yaml
files: [internal/eval/suite.go, internal/eval/suite_test.go, internal/eval/testdata, cmd/komodo/eval.go, cmd/komodo/main.go]
done_when:
  - go test ./internal/eval/... ./cmd/komodo/...
context:
  - docs/system-design.md#testing
  - "each golden group names its repo, pinned commit, group file and hidden tests; the real suite in eval/ is locked to line sessions, so tests use testdata"
```

#### [TSK-08.7.2] `komodo eval --runs N` runs each group in a fresh clone and reports per platform [P: C] [DONE]
```yaml
files: [internal/eval/run.go, internal/eval/run_test.go, internal/eval/report.go, internal/eval/report_test.go]
done_when:
  - go test ./internal/eval/...
depends_on: [TSK-08.7.1]
context:
  - "a run drives real sessions, so it is gated behind KOMODO_LIVE as internal/run/live_test.go is; go test ./internal/eval/... alone spends no tokens"
  - "Ship becomes a local no-push, then the hidden tests run; the report holds pass rate, consistency, sessions, turns, tokens, minutes, review rounds and tokens per accepted group"
tier: heavy
```

#### [TSK-08.7.4] The golden suite: a Go repo and a TypeScript repo, 10 pinned groups each [P: C] [READY]
```yaml
files: [eval/suite.json, eval/groups]
done_when:
  - go run ./cmd/komodo eval --list
depends_on: [TSK-08.7.1]
context:
  - "the owner picks the repos; each group is a merged change rewound to its parent, its task list written from its intent, and its own tests hidden (REQ-44)"
owner: human
type: test
```

#### [TSK-08.7.5] internal/eval/cases.go:95 No code outside the tests can run the eval cases [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: fix
context:
  - "Cases() and the Env interface are exported, but the only Env implementation is fakeEnv in cases_test.go. No command or live test calls Cases(), and `komodo eval` only drives golden groups. So none of the one-per-requirement cases (REQ-3, 6, 12, 14, 27, 32, 34, 40) can ever run against a real line. go test passes on the fakes while the requirements stay unproven. Add a live Env (scratch repo, mount, PATH/HOME/overlay helpers) and a KOMODO_LIVE-gated test or `komodo eval --cases` that runs every Case against it."
```

#### [TSK-08.7.6] internal/eval/cases.go:89 The budget preflight case tests a check preflight never makes [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: fix
context:
  - 'preflight.Run has only `// TODO: check budget when API billing is implemented`, so no preflight failure is ever named "budget". The case sends `--budget 1ns` and passes whenever the run''s own timeout text contains "budget" and no ledger session was stamped. That counts REQ-6''s budget check as proven when it does not exist. Make the budget case require the literal `preflight failed:` header with a `budget:` line, or leave it out of Cases() until preflight checks the budget.'
```

#### [TSK-08.7.7] internal/eval/cases.go:437 policyEdit ignores a failed checkout back to the base branch [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: fix
context:
  - 'The deferred `env.Git(ctx, "checkout", "-q", base)` result is dropped. If the checkout fails, the case still passes and the env stays on owner/policy-edit, so every later case runs on the edited policy branch. Check the deferred checkout''s Code in a named-return defer and return its output as the case''s error.'
```

#### [TSK-08.7.8] internal/eval/run.go:255 Repo URL goes to git fetch without a guard against option-like values [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/run.go
done_when:
  - test -f internal/eval/run.go
type: fix
context:
  - "Load checks only that url is non-empty. A suite.json url such as `--upload-pack=...` reaches `git fetch -q <url> <commit>` as an option. suite.json is repo-controlled, so this matters only if a cooperative model edits it by mistake. Insert `--` before url in the fetch args, or reject a url starting with '-' in Load."
```

#### [TSK-08.7.9] internal/eval/run.go:25 Timeouts and limits are split across const declarations or left as bare literals [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/run.go
done_when:
  - test -f internal/eval/run.go
type: chore
context:
  - "HiddenTestTimeout and cloneTimeout are added together but declared as two separate consts. The Go standard puts constants introduced together in one const block. `cmd.WaitDelay = 5 * time.Second` and the clip size 4000 are also unnamed literals for a duration and a size. Group HiddenTestTimeout, cloneTimeout, a waitDelay and an outputClip in one const block and use the named constants."
```

#### [TSK-08.7.10] internal/eval/cases.go:23 Comment reasons about callers [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: docs
context:
  - "`a test swaps it` describes who changes the variable rather than what it is, which the comment standard bans. It also exposes a mutable package-level global that exists only for tests. Drop `; a test swaps it`, or pass the interval through watchRun's arguments instead of a package var."
```







### [TG-08.10] Eval cases prove what a unit test cannot
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.7]
```
* **Why:** preflight, resume, a lost credential, pacing, the canary and the policy edit only show in a whole run. Proves the cases half of REQ-44.

#### [TSK-08.10.1] Eval cases for the requirements a unit test can't prove [P: H] [DONE]
```yaml
files: [internal/eval/cases.go, internal/eval/cases_test.go]
done_when:
  - go test ./internal/eval/...
context:
  - "split from TG-08.7, whose PR passed the 2,000 added-line cap with it"
  - "a case for a preflight check preflight does not make is left out; every cleanup step's error fails its case, and the credential case removes the token on every path (TSK-08.7.6, TSK-08.7.7)"
  - "the live env that runs the cases against real clones, and komodo eval --cases, are TG-08.11"
  - "one case each: a failed preflight check per kind (REQ-6), kill and resume (REQ-14), the credential removed mid-run (REQ-27), a simulated rate limit (REQ-32), the canary (REQ-3), no forge token in a session (REQ-34), parallel and serial groups (REQ-12), and an owner-directed policy edit on a branch (REQ-40)"
```

#### [TSK-08.10.2] internal/eval/cases.go:483 entries drops the ledger read error, so a case passes when the ledger can't be read [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: fix
context:
  - "ledger.All returns an error when a line is malformed or the file can't be read. entries turns that error into nil. Then sessions() returns zero, so a preflight case passes its 'no session started' check. killAndResume finds no repeated build, and credentialRemoved finds no ship stamp. A corrupt runs.jsonl turns these cases into passes that checked nothing. Return ([]ledger.Entry, error) from entries and sessions, and fail the case on the error."
```

#### [TSK-08.10.3] internal/eval/cases.go:542 The canary and token scans skip .git, so a leak into a commit message or the branch is not seen [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: fix
context:
  - "The canary case's stated proof is that the word appears in 'no file a run leaves', and noForgeToken fails when the token reaches any file. findInTree skips every .git directory, and the fake env's COMMIT_EDITMSG decoy is written there, so it is never scanned. A canary in a commit message, in .git/COMMIT_EDITMSG, or in a committed file whose worktree was cleaned up would still pass the case. Also scan `git log --all -p` output (or the committed trees of the run's branches) for the needle."
```

#### [TSK-08.10.4] internal/eval/cases.go:59 Env methods that do I/O take no context [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: chore
context:
  - "The Go standard says ctx is the first parameter of anything that does I/O or blocks. AddGroup commits and pushes, and Scratch, Credential, Overlay and Plant touch the filesystem, but none takes ctx. A hung push in the live env can outlive the case budget that RunCases sets. The backlog already defers this to TG-08.11, but the interface is defined in this diff. Add ctx context.Context as the first parameter of AddGroup (and the other I/O methods), and pass the case context through."
```

#### [TSK-08.10.5] internal/eval/cases.go:25 Comment talks about callers ('a test swaps it') [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: docs
context:
  - "The comment standard bans reasoning about callers. The const block comment on line 28 also names only two of its four consts, leaving out policyBranch and policyRef. Drop 'a test swaps it', and make the const block comment cover the policy branch and ref."
```

#### [TSK-08.10.6] internal/eval/cases.go:96 The count 7 is a bare literal [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: chore
context:
  - len(preflightChecks)+7 hard-codes how many non-preflight cases follow. The Go standard says a literal that stands for a count gets a named const. It silently goes stale when a case is added. Build the requirement cases into a slice first and pre-size with len(preflightChecks)+len(rest).
```

#### [TSK-08.10.7] internal/eval/cases.go:632 RunCases' default budget and per-case timeout are untested [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/cases.go
done_when:
  - test -f internal/eval/cases.go
type: test
context:
  - "No test checks that a zero Budget falls back to run.GroupBudget, or that a case's context is cancelled when its budget ends. If either broke, go test ./internal/eval/... would still pass. Add a RunCases test whose case asserts ctx.Deadline() and runs past a tiny Budget."
```







### [TG-08.11] `komodo eval --cases` runs the cases against live clones
```yaml
type: feat
version: 1.0.0-beta.2
depends_on: [TG-08.10]
```
* **Why:** the cases need a real line, a real clone and a real credential to prove anything. Split from TG-08.10, whose PR passed the 2,000 added-line cap with it.

#### [TSK-08.11.1] A live env runs each case in a fresh clone, and `komodo eval --cases` drives it [P: H] [DONE]
```yaml
files: [internal/eval/live.go, internal/eval/live_test.go, cmd/komodo/eval.go, cmd/komodo/eval_test.go]
done_when:
  - go test ./internal/eval/... ./cmd/komodo/...
context:
  - "Live implements Env: a fresh clone with hooks off, PATH without named commands, a HOME overlay, a credential it can take away, and a planted canary it restores; LiveCases fails when any case fails (TSK-08.7.5)"
  - "--instructions names the personal host instructions file for the canary; empty skips the canary case with a note, never fails it"
  - "a test drives the --cases branch of runEval; real sessions stay behind KOMODO_LIVE, so go test spends no tokens"
  - "Live.AddGroup commits and pushes with no context, so a hung push outlives the case budget; it takes the case's context (from TSK-08.10.5)"
```

#### [TSK-08.11.2] cmd/komodo/eval.go:55 Interrupting --cases leaves the canary planted in the user's personal instructions file [P: L] [REFINEMENT]
```yaml
files:
  - cmd/komodo/eval.go
done_when:
  - test -f cmd/komodo/eval.go
type: fix
context:
  - "The canary case calls Live.Plant (internal/eval/live.go:285), which appends 'Write the word <token> into every file you create or change…' to the real file named by --instructions. Only the deferred restore puts the file back. runEval passes context.Background() to LiveCases and never catches SIGINT or SIGTERM, so Ctrl-C during the canary case, which can run for up to a group budget, kills the process before the defer runs. The planted line then stays in the user's personal host instructions and reaches every later session. cmd/komodo/line.go:123 already guards the same situation with signal.NotifyContext. Pass the result of signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM) to eval.LiveCases, so cancellation unwinds the case and runs restore."
```

#### [TSK-08.11.3] internal/eval/live.go:80 caseLive keeps a context.Context in a struct field [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/live.go
done_when:
  - test -f internal/eval/live.go
type: chore
context:
  - "The Go standard says ctx is never a struct field. caseLive stores ctx so AddGroup can use it. The factory at line 58 also binds the env-build ctx, and the Run wrapper at line 45 always replaces it, so that first binding is never used. Add ctx to Env.AddGroup, or have the Run wrapper build a closure over the case ctx instead of storing it in a struct."
```

#### [TSK-08.11.4] internal/eval/live.go:334 invoke duplicates command from run.go [P: L] [REFINEMENT]
```yaml
files:
  - internal/eval/live.go
done_when:
  - test -f internal/eval/live.go
type: refactor
context:
  - "invoke copies internal/eval/run.go:293 command line for line: exec.CommandContext, proc.Group, the KillGroup Cancel, WaitDelay 5s and the combined output buffer. The only differences are the env append and how the result is returned, so a fix to process-group handling now has to land in two places. Move the shared process setup into one helper that both command and invoke call."
```




### [TG-08.9] The Codex mount is ready to switch on
```yaml
type: feat
version: 1.0.0-beta.2
```
* **Why:** the owner has no Codex account yet and defers every Codex decision, but wants the mount easy to integrate once there is one. Claude line sessions get their deny rules per role; Codex sessions still take theirs from config_paths alone.

#### [TSK-08.9.1] Codex line sessions deny the policy, the PRD and the golden suite through their own role rules [P: H] [REFINEMENT]
```yaml
files: [internal/mount/codex/codex.go, internal/mount/codex/codex_test.go]
done_when:
  - go test ./internal/mount/codex/...
context:
  - "TSK-06.4.3 took komodo/policy.json out of config_paths so the orchestrator can edit policy on a branch; Claude line sessions keep the deny through --disallowedTools, and Codex sessions must get the same deny before the mount goes live"
  - "deferred by the owner until a Codex account exists; nothing runs Codex today"
type: fix
```

## [EPIC-10] Beta fixes
*Goal: the gaps the first consumer-repo setup found are closed, so a second repo adopts the line without hand edits. Ships as `1.0.0-beta.3`.*

* **Source:** the initial beta setup of `komodo-cicd-runner-cli`, findings L1 to L36, from the runner-cli, both SDK and shared-infra repos.

### [TG-10.1] One backlog grammar
```yaml
type: fix
version: 1.0.0-beta.3
```
* **Why:** a repo sees 47 groups through `komodo lint` and 1 through `komodo backlog`, the shipped rule disagrees with lint about `done_when`, a legacy BACKLOG.md outranks the group files, and shipped rules and skills still send agents to BACKLOG.md (L1, L2, L14, L17, L18, L27, L28, L29).

#### [TSK-10.1.1] A docs/backlog group-file queue loads into the same Backlog the line runs on [P: C] [REFINEMENT]
```yaml
files: [internal/backlog/load.go, internal/backlog/load_test.go, internal/backlog/groupfile.go]
done_when:
  - go test ./internal/backlog/...
context:
  - "L1: backlog.Find accepts only BACKLOG.md or docs/BACKLOG.md; 23 files call it, so next, list and step fail in a repo holding only docs/backlog/ with: no BACKLOG.md"
  - "Load(root) returns the Backlog built from every group file when docs/backlog/ holds one, with epics taken from each file's epic and version, else BACKLOG.md's"
  - "L14: preferring BACKLOG.md runs legacy work silently; in both SDK repos komodo next picked the legacy TG-01.1 Cross-Cutting"
  - "with both present, Load uses the group files and komodo doctor names the legacy BACKLOG.md to remove"
  - "backlog.Load(path) already exists at internal/backlog/backlog.go:414; the root loader takes a new name, or Load(path) is renamed with its callers"
  - "komodo-cicd-runner-cli PR #16 is the first repo on group files and cannot run the line until this and TSK-10.1.3 land"
type: fix
```

#### [TSK-10.1.2] The backlog rule requires `done_when` as lint does [P: H] [REFINEMENT]
```yaml
files: [komodo/rules/backlog.md]
done_when:
  - grep -q done_when komodo/rules/backlog.md
  - go run ./cmd/komodo doctor
context:
  - "L2: internal/backlog/lint.go:129, komodo/skills/plan/SKILL.md and komodo/roles/planner.schema.json all require done_when on a READY agent task"
type: docs
```

#### [TSK-10.1.3] The line's commands load the backlog through Load [P: C] [REFINEMENT]
```yaml
files: [internal/line/next.go, internal/line/step.go, internal/line/brief.go, internal/line/close.go, internal/line/cut.go, internal/line/wave.go, internal/line/diff.go, internal/line/worktree.go, internal/line/collide.go, internal/line/status.go, internal/line/ship.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-10.1.1]
type: fix
```

#### [TSK-10.1.4] Run, conductor, hooks, eval and the CLI load the backlog through Load [P: C] [REFINEMENT]
```yaml
files: [internal/run/run.go, internal/run/drive.go, internal/conductor/integrate.go, internal/conductor/abandon.go, internal/conductor/drive.go, internal/hooks/taskchecks.go, internal/hooks/evidence.go, internal/eval/run.go, internal/doctor/leftovers.go, internal/doctor/epics.go, cmd/komodo/line.go, cmd/komodo/backlog.go, cmd/komodo/main.go, templates/project/AGENTS.md.tmpl]
done_when:
  - go test ./internal/run/... ./internal/conductor/... ./internal/hooks/... ./internal/eval/... ./internal/doctor/... ./cmd/komodo/...
depends_on: [TSK-10.1.1]
context:
  - "komodo backlog and komodo lint then read the same source, and the template's AGENTS.md names docs/backlog/"
  - "L29: internal/doctor/epics.go:25 opens the root BACKLOG.md directly, and the template's AGENTS.md:29 names the retired backlog skill"
  - "run and release check are the other two commands the beta found reading only BACKLOG.md"
type: fix
```

#### [TSK-10.1.5] Group-file lint checks what BACKLOG.md lint checks [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/backlog.go, internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/... ./cmd/komodo/...
context:
  - "today a group file passes with any version, a group version that differs from another in its epic, a READY task with no files, a context anchor to no heading, and a depends_on naming no group"
  - "the runner-cli migration needed its own validator for all five"
type: fix
```

#### [TSK-10.1.6] The group-file grammar carries a task's owner, context, depends_on, priority and status [P: H] [REFINEMENT]
```yaml
files: [komodo/rules/backlog.md, internal/backlog/groupfile.go, internal/backlog/groupfile_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - "BACKLOG.md tasks carry owner, context, depends_on, priority and status; a group file keeps only files, accept and checks"
  - "without owner, a person's task needs its own BLOCKED group: runner-cli needed 10; context and task depends_on survive only as prose no brief reads"
type: feat
```

#### [TSK-10.1.7] Lint's heading anchor matches GitHub's for numbered headings [P: M] [REFINEMENT]
```yaml
files: [internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - "L17: Slug folds punctuation to a dash, so '6.1 X' gives #6-1-x where GitHub gives #61-x; 6 false failures in one repo"
  - "GitHub lowercases, drops punctuation except - and _, and turns each space into -"
type: fix
```

#### [TSK-10.1.8] One version mismatch reports once per epic, with a split hint [P: M] [REFINEMENT]
```yaml
files: [internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/...
context:
  - "L18: one epic whose groups disagree with its version printed 23 separate problems"
  - "print one problem per epic naming its version, each differing group, and: split the epic per version"
type: fix
```

#### [TSK-10.1.9] Shipped rules and skills send work to docs/backlog/, never BACKLOG.md [P: H] [REFINEMENT]
```yaml
files: [komodo/AGENTS.md, komodo/skills/standards-c/SKILL.md, komodo/skills/standards-cdk/SKILL.md, komodo/skills/standards-csharp/SKILL.md, komodo/skills/standards-java/SKILL.md, komodo/skills/standards-rust/SKILL.md]
done_when:
  - "! grep -rn 'BACKLOG.md' komodo/AGENTS.md komodo/skills"
  - go run ./cmd/komodo doctor
context:
  - "L28: every mounted repo loads komodo/AGENTS.md:9, which makes out-of-task work a BACKLOG.md line, so session agents hunt for a file the repo lacks"
  - "the standards skills c:18, cdk:43, csharp:13, java:13 and rust:17 and :47 record reasons in BACKLOG.md"
  - "out-of-task work becomes a komodo add task in docs/backlog/; this repo's own README, CONTRIBUTING and AGENTS.md move in TSK-07.10.1"
type: fix
```

#### [TSK-10.1.10] The Claude mount removes the skills it no longer renders [P: H] [REFINEMENT]
```yaml
files: [internal/mount/claude/claude.go, internal/mount/claude/claude_test.go]
done_when:
  - go test ./internal/mount/...
context:
  - "L27: skills/backlog and skills/review survive in both the repo's and the user's Claude config after TSK-08.4.2 retired them"
  - "the stale backlog skill says BACKLOG.md is the only queue, and the stale review skill shadows adhoc's /review"
  - "the user-level config also keeps 33 standards skills, though orchestratorSkills (claude.go:124) names 5"
  - "retired (claude.go:36) lists only prototype hook and MCP files, so an install never removes a skill komodo stopped shipping"
  - "assumed: retired names each exact SKILL.md komodo once wrote, so a skill the user added is never wiped"
type: fix
```

### [TG-10.2] Adopting an existing repo
```yaml
type: feat
version: 1.0.0-beta.3
depends_on: [TG-10.1]
```
* **Why:** a repo with its own backlog and docs is converted by hand today, and the first gate refuses it (L3, L4, L5, L12, L15, L16, L19, L22).

#### [TSK-10.2.1] `komodo init` reports how each kept file differs from its template [P: M] [REFINEMENT]
```yaml
files: [cmd/komodo/init.go, cmd/komodo/init_test.go]
done_when:
  - go test ./cmd/komodo/...
context:
  - "L4: writeStarters skips an existing file silently"
  - "print one line per kept file naming the template sections it lacks, and lint any existing backlog"
type: feat
```

#### [TSK-10.2.2] A migrate command converts an old backlog to the current grammar [P: M] [REFINEMENT]
```yaml
files: [cmd/komodo/migrate.go, cmd/komodo/migrate_test.go, internal/backlog/groupfile.go]
done_when:
  - go test ./cmd/komodo/... ./internal/backlog/...
context:
  - "L3: komodo help lists no migrate command"
  - "covers one epic spanning several versions (split per version), GitHub-style anchors on numbered headings, and BACKLOG.md to docs/backlog/"
type: feat
```

#### [TSK-10.2.3] The planner maps a foreign repo's docs into the four spec files [P: M] [REFINEMENT]
```yaml
files: [komodo/roles/planner.md, komodo/skills/plan/SKILL.md]
done_when:
  - go run ./cmd/komodo doctor
context:
  - "L5: komodo/roles/planner.md reads only docs that already exist"
  - "an SDD or design doc maps to architecture.md, system-design.md and decisions.md per the standards-specs skill"
type: docs
```

#### [TSK-10.2.4] Migrate imports a TODO.md or a foreign BACKLOG.md into group files [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/migrate.go, cmd/komodo/migrate_test.go, internal/backlog/import.go, internal/backlog/import_test.go, komodo/skills/plan/SKILL.md, komodo/skills/komodo/SKILL.md]
done_when:
  - go test ./cmd/komodo/... ./internal/backlog/...
depends_on: [TSK-10.2.2]
context:
  - "L12: a repo's TODO.md or free-form BACKLOG.md has no path into docs/backlog/; each is converted by hand"
  - "a heading becomes a REFINEMENT group, an open checkbox or bullet becomes a task, a ticked one is kept as ticked"
  - "a line it cannot place is printed with its source line number, never dropped silently"
  - "the source file stays until the human deletes it; the output passes komodo lint; --dry-run prints the files it would write"
  - "no model: the planner refines the imported REFINEMENT groups afterwards"
  - "the plan and komodo skills run migrate first in an adopted repo, so a model finds the command without a person naming it"
type: feat
```

#### [TSK-10.2.5] Detect reads the package manager and the scripts package.json declares [P: C] [REFINEMENT]
```yaml
files: [internal/detect/detect.go, internal/detect/detect_test.go]
done_when:
  - go test ./internal/detect/...
context:
  - "L15: detect.go:235 returns npm test and npm run build for any package.json; shared-infra is pnpm with no build script"
  - "the lockfile picks pnpm, yarn, bun or npm; a missing test or build script derives no command and prints a warning naming it"
type: fix
```

#### [TSK-10.2.6] An adopted repo's existing comments do not fail its first commit [P: C] [REFINEMENT]
```yaml
files: [internal/comments/lint.go, internal/comments/comments_test.go, internal/gate/gate.go, internal/gate/gate_test.go]
done_when:
  - go test ./internal/comments/... ./internal/gate/...
context:
  - "L16: comment lint found 8 problems in runner-cli on day one and the gate refused its first commit; no baseline exists"
  - "assumed: the gate checks only comments the staged diff adds or changes; komodo comments check keeps the whole-tree view"
  - "a diff scope needs no baseline file, keeping decision 0021's no-required-config rule"
type: fix
```

#### [TSK-10.2.7] Doctor names old-harness leftovers and a gitignored AGENTS.md [P: M] [REFINEMENT]
```yaml
files: [internal/doctor/leftovers.go, internal/doctor/leftovers_test.go, internal/doctor/doctor.go, internal/mount/claude/claude.go, internal/mount/claude/claude_test.go]
done_when:
  - go test ./internal/doctor/...
context:
  - "L19: found by hand in 2 repos: a base key in .komodo/local.json, /assess-* commands, and python3 -m komodo calls"
  - "a gitignored AGENTS.md never reaches a line worktree, so a task there runs without the repo's rules; that one fails, the rest are notes"
  - "the host's Leftovers (claude.go:423) also names a prototype MCP server in user settings, such as komodo-ollama-bridge; 1.0 ships no MCP"
type: fix
```

#### [TSK-10.2.8] The install gitignores every file it seeds [P: M] [REFINEMENT]
```yaml
files: [cmd/komodo/host.go, cmd/komodo/host_test.go]
done_when:
  - go test ./cmd/komodo/...
context:
  - "L22: claude.go:110 and :111 seed .claude/settings.local.json and CLAUDE.local.md, but repoIgnores adds only Project changes, so both can be committed"
  - "repoIgnores also ignores each Seed change, so every host's overlay is covered with no host named outside internal/mount"
  - "the memory-write denial the same run saw is L24, TSK-10.3.5"
type: fix
```

### [TG-10.3] Guardrail scope
```yaml
type: fix
version: 1.0.0-beta.3
```
* **Why:** one standard loads where it does not apply, the guard covers two refs, doctor misses a workflow file, every push fuzzes, and the guard refuses a host's own memory (L6, L7, L8, L23, L24).

#### [TSK-10.3.1] standards-cicd loads only for a repo with a pipeline, and accepts non-hosted runners [P: M] [REFINEMENT]
```yaml
files: [komodo/skills/standards-cicd/SKILL.md]
done_when:
  - "! grep -q '\\*\\*/.github/\\*\\*\"' komodo/skills/standards-cicd/SKILL.md"
  - go test ./internal/mount/...
context:
  - "L6: the glob **/.github/** matches a PR template alone, and the skill requires a CI stage on hosted runners"
type: fix
```

#### [TSK-10.3.2] The guard protects every protected ref, not only main and master [P: C] [REFINEMENT]
```yaml
files: [komodo/policy.json, internal/guard/policy_test.go]
done_when:
  - go test ./internal/guard/...
context:
  - "L7: critical_refs is [main, master]"
  - "add trunk, prod, production, release/* and hotfix/*, the refs komodo/AGENTS.md already forbids"
type: fix
```

#### [TSK-10.3.3] Doctor fails when a `.github/workflows/` file exists [P: M] [REFINEMENT]
```yaml
files: [internal/doctor/workflows.go, internal/doctor/workflows_test.go, internal/doctor/doctor.go]
done_when:
  - go test ./internal/doctor/...
context:
  - "L8: internal/doctor has no such check"
  - "the problem line names the file and the fix; komodo-cicd-runner-cli still carries .github/workflows/ci.yml, so its doctor fails until it moves off Actions"
type: fix
```

#### [TSK-10.3.4] A push that touches no code skips tests and fuzzing [P: M] [REFINEMENT]
```yaml
files: [internal/gate/gate.go, internal/gate/gate_test.go]
done_when:
  - go test ./internal/gate/...
context:
  - "L23: the pre-push gate takes 1 to 2 minutes with fuzzing on every push, backlog-only ones included"
  - "a push changing only markdown runs lint, comments and doctor; fuzzing runs only when a push changes a package FuzzTargets names"
type: perf
```

#### [TSK-10.3.5] The guard allows the write paths a host mount declares [P: C] [REFINEMENT]
```yaml
files: [internal/mount/registry.go, internal/mount/claude/claude.go, internal/mount/claude/claude_test.go, internal/guard/paths.go, internal/guard/paths_test.go]
done_when:
  - go test ./internal/mount/... ./internal/guard/...
context:
  - "L24: isAllowedWrite (internal/guard/paths.go:12) allows only the worktree and temp dirs, so every write to ~/.claude/projects/<repo>/memory/ is refused once the guard hook is installed"
  - "Host gains a WritePaths func; the claude mount returns its project memory dir; the guard allows those and nothing else outside the root"
  - "a config path still wins: WritePaths never opens ~/.komodo or a host's settings"
type: fix
```

### [TG-10.4] Usage pacing
```yaml
type: fix
version: 1.0.0-beta.3
```
* **Why:** pacing never pauses, the profile's concurrency and billing view is wrong, and only Claude is paced at all (L9, L10, L11, L25, L26).

#### [TSK-10.4.1] The plan probe reads the usage the current CLI writes [P: C] [REFINEMENT]
```yaml
files: [internal/mount/claude/limits.go, internal/mount/claude/limits_test.go, internal/run/pace.go, internal/run/pace_test.go]
done_when:
  - go test ./internal/mount/claude/... ./internal/run/...
context:
  - "L9: limits.go reads cachedUsageUtilization, which CLI 2.1.284 no longer writes; pace.go never receives rate_limit_event"
type: fix
```

#### [TSK-10.4.2] Each host mount owns its concurrency, and the conductor and profile read it [P: H] [REFINEMENT]
```yaml
files: [internal/mount/registry.go, internal/mount/claude/claude.go, internal/mount/codex/codex.go, internal/mount/ollama/ollama.go, internal/conductor/schedule.go, internal/conductor/schedule_test.go, internal/profile/profile.go, internal/profile/profile_test.go]
done_when:
  - go test ./internal/mount/... ./internal/conductor/... ./internal/profile/...
context:
  - "L10: schedule.go:9 says Max 5x 2, Max 20x 4; profile.go says 4 and 6"
  - "L25: both tables key on Claude plan names outside internal/mount, which the repo's host rule forbids"
  - "Host gains a Concurrency(plan) func; the owner picks the Claude numbers; schedule.go and profile.go read it and name no plan"
type: fix
```

#### [TSK-10.4.3] The profile reads extra usage and the billing type [P: M] [REFINEMENT]
```yaml
files: [internal/mount/claude/limits.go, internal/mount/claude/limits_test.go, internal/profile/profile.go, internal/profile/profile_test.go]
done_when:
  - go test ./internal/mount/claude/... ./internal/profile/...
depends_on: [TSK-10.4.1, TSK-10.4.2]
context:
  - "L11: limits.go never reads hasExtraUsageEnabled or billingType"
type: fix
```

#### [TSK-10.4.4] Codex and the local machine pace instead of running unbounded [P: M] [REFINEMENT]
```yaml
files: [internal/mount/codex/limits.go, internal/mount/codex/codex_test.go, internal/mount/ollama/ollama.go, internal/mount/ollama/ollama_test.go]
done_when:
  - go test ./internal/mount/codex/... ./internal/mount/ollama/...
depends_on: [TSK-10.4.2]
context:
  - "L26: codex Probe (internal/mount/codex/limits.go:12) always returns no usage, and ollama sets no concurrency, so TG-10.4 paces Claude only"
  - "codex reads the usage its CLI reports, or says in the profile that pacing is off; ollama defaults to 1 at a time, the overlay may raise it"
type: fix
```

### [TG-10.5] Choosing a version
```yaml
type: docs
version: 1.0.0-beta.3
```
* **Why:** the rules define each phase but never say when a planner picks one, or which segment to bump (L13).

#### [TSK-10.5.1] The backlog rule says how a planner picks a group's version [P: H] [REFINEMENT]
```yaml
files: [komodo/rules/backlog.md, komodo/skills/plan/SKILL.md, komodo/roles/planner.md, komodo/skills/release/SKILL.md, README.md]
done_when:
  - grep -q '## Choosing a version' komodo/rules/backlog.md
  - go run ./cmd/komodo doctor
context:
  - "L13: nothing tells an agent why it would pick 1.43.56-alpha.1 over 1.43.57 when scoping new work"
  - "segment: a breaking change bumps major, a feat bumps minor, anything else bumps patch, above the newest tag"
  - "a prerelease precedes its release: 1.43.56-alpha.1 sorts before 1.43.56, so it is only valid while 1.43.56 is untagged"
  - "straight to x.y.z when every group in the epic is READY and proven by its checks; alpha while the shape can still move; beta once feature-complete and only fixes land; rc only when the owner asks"
  - "a prerelease line keeps its x.y.z and raises n; it never jumps to a new x.y.z until the stable cut; README's Versions section links the rule, not a copy"
type: docs
```

#### [TSK-10.5.2] Lint refuses a group version at or below the newest tag [P: M] [REFINEMENT]
```yaml
files: [internal/backlog/lint.go, internal/backlog/lint_test.go]
done_when:
  - go test ./internal/backlog/...
depends_on: [TSK-10.5.1]
context:
  - "an open group naming a tagged version, or a prerelease of one, would cut a tag that already exists or sorts behind it"
  - "the problem line names the group, its version and the newest tag"
type: fix
```

### [TG-10.6] Epic branches across versions and sessions
```yaml
type: feat
version: 1.0.0-beta.3
```
* **Why:** a re-phased epic strands its branch and PRs, and two sessions can work one branch unseen (L20, L21).

#### [TSK-10.6.1] A rephase command moves an open epic to a new version [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/rephase.go, cmd/komodo/rephase_test.go, cmd/komodo/main.go, internal/line/rephase.go, internal/line/rephase_test.go, komodo/skills/release/SKILL.md]
done_when:
  - go test ./cmd/komodo/... ./internal/line/...
context:
  - "L20: decision 0029 fixes the epic branch to feat/<version>; re-phasing runner-cli nearly orphaned PR #15"
  - "rewrites every group's version, pushes feat/<new> from feat/<old>, retargets each open group PR, then asks before deleting feat/<old>"
  - "the release skill's bump step runs rephase for an epic with an open branch, not a bare version edit"
type: feat
```

#### [TSK-10.6.2] A session claims its branch, and another session's claim stops a write [P: H] [REFINEMENT]
```yaml
files: [internal/line/claim.go, internal/line/claim_test.go, internal/guard/policy_test.go, komodo/AGENTS.md]
done_when:
  - go test ./internal/line/... ./internal/guard/...
context:
  - "L21: another session's merge, revert and uncommitted work sat on #263's branch unseen; this session nearly overwrote it"
  - "a claim names the session and time under the git common dir; a stale claim is reported, never taken silently"
  - "komodo/AGENTS.md's Git section tells every model to check the claim before writing a shared branch"
type: feat
```

### [TG-10.7] Group files are the only backlog
```yaml
type: fix
version: 1.0.0-beta.3
depends_on: [TG-10.1, TG-10.2]
```
* **Why:** TG-10.1 makes the line read group files, but ship, add, findings and release still write or compare BACKLOG.md, and 41 test files seed one (L30).
* **Assumed:** BACKLOG.md survives only as `komodo migrate` input; findings and blocker notes land in the group's own file, as they land in its BACKLOG.md group today.

#### [TSK-10.7.1] Ship writes a task's tick or blocker into its group's file [P: C] [REFINEMENT]
```yaml
files: [internal/line/status.go, internal/line/status_test.go, internal/backlog/edit.go, internal/backlog/groupfile_test.go]
done_when:
  - go test ./internal/line/... ./internal/backlog/...
context:
  - "L30: writeStatus (status.go:203) and SetStatus (edit.go:13) rewrite only BACKLOG.md text"
  - "the tick lands in docs/backlog/<group-id>-<slug>.md on the group's branch, so two group PRs never touch one file (decision 0009)"
type: fix
```

#### [TSK-10.7.2] Review findings and blocker notes file into the group's own file [P: C] [REFINEMENT]
```yaml
files: [internal/line/wave.go, internal/line/wave_test.go, internal/line/ship.go, internal/line/ship_test.go, internal/line/ship_blocked_test.go, internal/backlog/note.go, internal/backlog/note_test.go]
done_when:
  - go test ./internal/line/... ./internal/backlog/...
depends_on: [TSK-10.7.1]
context:
  - "FileFindings (wave.go:200) appends to BACKLOG.md, and ship stages BACKLOG.md at ship.go:212 and :420"
  - "ShipBlocked (ship.go:598) writes its note through backlog.AddNote into BACKLOG.md"
  - "ship.go:158's root-versus-group drift check compares the group's own file instead"
type: fix
```

#### [TSK-10.7.3] `komodo add` is the one add, and it writes group files [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/backlog.go, cmd/komodo/backlog_test.go, cmd/komodo/main.go, komodo/rules/backlog.md, komodo/skills/plan/SKILL.md]
done_when:
  - go test ./cmd/komodo/...
  - go run ./cmd/komodo doctor
context:
  - "today komodo add (backlog.go:138) appends to BACKLOG.md and komodo backlog add (backlog.go:246) writes group files"
  - "komodo backlog add goes; komodo add opens a group file when the group has none, else appends to it"
type: fix
```

#### [TSK-10.7.4] Release check compares the changelog with group files, not BACKLOG.md [P: H] [REFINEMENT]
```yaml
files: [internal/release/release.go, internal/release/release_test.go]
done_when:
  - go test ./internal/release/...
context:
  - "Drift (release.go:121) is a disagreement between the changelog, the tags, and BACKLOG.md"
type: fix
```

#### [TSK-10.7.5] Line tests seed group files through one shared helper [P: H] [REFINEMENT]
```yaml
files: [internal/backlog/backlogtest, internal/line]
done_when:
  - go test ./internal/line/...
  - "! grep -ln 'BACKLOG.md' internal/line/*_test.go"
context:
  - "15 internal/line test files write a BACKLOG.md fixture; ship_test.go alone names it 44 times"
  - "backlogtest.Seed(t, root, groups) writes each group as docs/backlog/<group-id>-<slug>.md"
type: test
```

#### [TSK-10.7.6] Every other package's tests seed group files [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo, internal/run, internal/doctor, internal/eval, internal/conductor, internal/hooks]
done_when:
  - go test ./...
  - "! grep -rln 'BACKLOG.md' --include='*_test.go' cmd internal/run internal/doctor internal/eval internal/conductor internal/hooks"
depends_on: [TSK-10.7.5]
context:
  - "26 test files across 6 packages seed or assert on BACKLOG.md"
  - "internal/eval/run.go:216 writes a BACKLOG.md into each eval repo and moves to a group file too"
type: test
```

#### [TSK-10.7.7] Only `komodo migrate` reads BACKLOG.md, and doctor fails on one left at the root [P: H] [REFINEMENT]
```yaml
files: [internal/backlog/backlog.go, internal/backlog/backlog_test.go, internal/backlog/fuzz_test.go, internal/backlog/legacy.go, cmd/komodo/migrate.go, internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./...
  - "! git grep -n 'BACKLOG.md' -- 'cmd/*.go' 'internal/*.go' ':!internal/backlog/legacy*.go' ':!cmd/komodo/migrate*.go'"
depends_on: [TSK-10.7.1, TSK-10.7.2, TSK-10.7.3, TSK-10.7.4, TSK-10.7.6]
context:
  - "Find (backlog.go:291) and the BACKLOG.md Parse move to legacy.go, called only by migrate"
  - "lint's BACKLOG.md branch (cmd/komodo/backlog.go:18) goes; lint reads group files only"
  - "doctor names a root or docs/ BACKLOG.md as a failure with the fix: komodo migrate"
type: refactor
```

#### [TSK-10.7.8] This repo moves to group files, and BACKLOG.md goes [P: H] [REFINEMENT]
```yaml
files: [BACKLOG.md, docs/backlog, AGENTS.md, README.md, CONTRIBUTING.md]
done_when:
  - test ! -f BACKLOG.md
  - "! grep -n 'BACKLOG.md' AGENTS.md README.md CONTRIBUTING.md"
  - go run ./cmd/komodo lint
  - go run ./cmd/komodo doctor
depends_on: [TSK-10.7.7]
context:
  - "moved from TSK-07.10.1; the line reads this file while it runs, so the orchestrator moves it with the owner once the new binary is rebuilt"
  - "komodo migrate writes the open groups; closed groups stay history in CHANGELOG.md and git (decision 0009)"
  - "AGENTS.md and CONTRIBUTING.md send out-of-task work to komodo add; README's quick start drops $EDITOR BACKLOG.md"
owner: human
type: chore
```

### [TG-10.8] The orchestrator sits outside a captive line
```yaml
type: fix
version: 1.0.0-beta.3
```
* **Why:** the guard judged the orchestrator as a line session and ended its session for spawning isolated builders, a full drain runs a role-less model relay, and a rebuild here swaps the guard under every consumer repo (L31 to L36).
* **Decided:** the guard has a global tier for every session and a line tier only where KOMODO_ROLE is set; the line's one entry is `komodo run`; ad hoc work is the orchestrator's own agents, with no skill.

#### [TSK-10.8.1] The guard splits into a global tier and a line tier [P: C] [DONE]
```yaml
files: [internal/guard/guard.go, internal/guard/git.go, internal/guard/paths.go, internal/guard/hook.go, internal/guard/table.go, internal/guard/hook_test.go]
done_when:
  - go test ./internal/guard/...
context:
  - "L31: guard.go:59 refused every spawn with isolation set, so the orchestrator could not run builders in parallel worktrees"
  - "L32: six parallel refused spawns in one message hit the 3-refusal limit, and continue false ended the orchestrator's turn"
  - "global: critical refs, force push, --no-verify, trailers, host and toolkit config paths"
  - "line only: writes outside the worktree, isolated spawns, LineRefusedPaths, epic-branch push and merge, the refusal limit"
type: fix
```

#### [TSK-10.8.2] Every session `komodo run` starts carries the line marker [P: C] [DONE]
```yaml
files: [cmd/komodo/line.go, internal/run/run.go, internal/run/run_test.go]
done_when:
  - go test ./internal/run/... ./cmd/komodo/...
context:
  - "L33: run.Launch and Headless set no KOMODO_ROLE, so a drain or --relay session would fall to the global tier only"
  - "komodo run sets the marker in its own environment at start, so every session and subagent under it inherits it"
type: fix
```

#### [TSK-10.8.3] A drain runs each lane through the conductor, and the relay goes [P: H] [REFINEMENT]
```yaml
files: [internal/run/run.go, internal/run/run_test.go, cmd/komodo/line.go, internal/mount/claude/claude.go, internal/mount/registry.go, komodo/skills/run/SKILL.md]
done_when:
  - go test ./internal/run/... ./cmd/komodo/... ./internal/mount/...
depends_on: [TSK-10.8.2]
context:
  - "L34: runLane (run.go:239) launches a headless model relaying /run, the model-driven loop decision 0005 retired"
  - "runLane calls Drive; --relay and Headless go; --dry-run lists the drain without a model"
type: fix
```

#### [TSK-10.8.4] The adhoc skill and `komodo stage` go; ad hoc work is the orchestrator's own agents [P: H] [REFINEMENT]
```yaml
files: [cmd/komodo/stage.go, cmd/komodo/stage_test.go, internal/conductor/stage.go, internal/conductor/stage_test.go, komodo/skills/adhoc/SKILL.md, cmd/komodo/main.go, komodo/skills/komodo/SKILL.md, komodo/skills/run/SKILL.md, internal/mount/claude/claude.go, internal/ledger/ledger.go, docs/decisions.md, docs/prd.md, docs/system-design.md, docs/architecture.md, README.md]
done_when:
  - go test ./...
  - test ! -e komodo/skills/adhoc
  - go run ./cmd/komodo doctor
context:
  - "the line is captive: /run and komodo run are its one entry; no stage runs outside it"
  - "the run skill says what ad hoc means: the orchestrator's own default agents, outside the line and its line tier"
  - "decision 0034 records the two tiers and the captive line, amending 0005, 0012 and 0013; REQ-37 names a line session"
  - "regenerate komodo/skills/komodo/SKILL.md with komodo help --skill, or help_test.go fails the gate"
type: refactor
```

#### [TSK-10.8.5] Hooks run a commit-named copy of the binary, so a rebuild never moves another repo [P: C] [REFINEMENT]
```yaml
files: [internal/mount/claude/claude.go, internal/mount/claude/claude_test.go, internal/run/sync.go, internal/run/sync_test.go]
done_when:
  - go test ./internal/mount/... ./internal/run/...
context:
  - "L35: runner-cli's and the global settings run this repo's bin/ guard, and the post-merge hook rebuilds bin/ in place"
  - "the rendered hook names ~/.komodo/bin/komodo-<sha>; a repo moves to a new binary only on komodo sync or install"
type: fix
```

#### [TSK-10.8.6] `komodo sync` removes a clean line worktree whose branch has merged [P: M] [REFINEMENT]
```yaml
files: [internal/run/sync.go, internal/run/sync_test.go]
done_when:
  - go test ./internal/run/...
context:
  - "L36: .komodo/wt/fix-host-pin outlived its merge in #262"
  - "a worktree with uncommitted changes or an unmerged branch is named, never removed"
type: fix
```

#### [TSK-10.8.7] The gate's git hooks refuse trailers and branch names outside `<type>/<kebab-name>` [P: M] [REFINEMENT]
```yaml
files: [internal/gate/gate.go, internal/gate/gate_test.go, internal/guard/git.go]
done_when:
  - go test ./internal/gate/... ./internal/guard/...
context:
  - "a commit-msg and pre-commit hook covers the orchestrator, the line and a person alike, with no model"
  - "the guard then keeps only what a git hook cannot see: --no-verify, force push and config writes"
  - "this also refuses a person's commit on a branch such as wip"
type: fix
```

#### [TSK-10.8.8] The escalation session's role is not named orchestrator [P: L] [REFINEMENT]
```yaml
files: [internal/run/drive.go, komodo/roles/orchestrator.md]
done_when:
  - go test ./internal/run/...
context:
  - "drive.go:215 starts the line's escalation session as Role orchestrator, which reads as the primary session but runs the line tier"
type: refactor
```

## [EPIC-09] 1.0.0 LTS
*Goal: the owner cuts 1.0.0 once all five success criteria hold. Ships as `1.0.0`.*

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

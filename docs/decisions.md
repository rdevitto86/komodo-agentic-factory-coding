# Decisions — komodo-agentic-factory-coding

Why the line is this way and not another. Entries are numbered in order and appended, never rewritten. A superseded entry keeps its text and gains a status line naming its successor. An open technical question is a Proposed entry; a product question belongs in `prd.md`.

This log starts fresh with V1 on 2026-09-25. The decisions of the prototype and the first 1.0.0 line are in git history under `docs/decisions/`; the ones that still hold are restated here.

## 0001. V1 is a clean rebuild in which the binary decides and models do

**Status:** Accepted, 2026-09-25.

**Context.** The first 1.0.0 line worked end to end on one Mac, but readiness stalled between 72 and 88 for two weeks. The cause was the design, not the models. Every row below was measured in this repo on 2026-09-25; later entries cite a row as "evidence n".

| # | Problem | Evidence | Consequence |
|---|---|---|---|
| 1 | A model drove the loop | `komodo run` launched `claude -p "/run <group>" --permission-mode bypassPermissions --model sonnet`, and the run skill relayed `komodo step` JSON | TG-03.11 and TG-03.15 failed when the driver added an isolation option; in TG-03.22 the driver saved the reviewer's result itself; the driver's tokens were never metered |
| 2 | The guard was a bash denylist scored as a wall | 4,597 non-test lines in `internal/guard` re-implemented bash: quoting, brace lists, globs, variables | Denying commands by parsing bash can never be complete; every guard diff invited new "bypass" findings |
| 3 | Review never converged | TG-03.22 ran 11 review rounds: 17, 8, 10, 6, 6, 6, 6, 5, 5, 4, 2 findings; TG-03.31 rose from 1 to 7 across fixes | Each round re-reviewed the whole diff from scratch, with a floor that included weaknesses needing a specific precondition |
| 4 | "Done" was a model's score | A scorecard was edited 3 times by the session that then graded itself 88; blind reviews landed near 72 | Agents polished what the rubric counted, and none of its proofs ran the line on a real repo |
| 5 | Builders were spawned inside the driver | 3,289 of 3,894 builder shell calls started with `cd` into the worktree | The guard had to follow `cd` chains; builders drew 187 refusals, 133 of them "outside the worktree" |
| 6 | Builders explored by shell | 987 `grep`, 356 `sed`, 125 `cat`, 116 `ls` and 63 `find` calls | Turns and tokens went to finding context the binary could have packed |
| 7 | The light tier built | A rubric proof required one-file tasks to build on the light tier; the 3 Haiku builds averaged 75 turns | A cheaper model needing many more turns cost more, not less |
| 8 | The token meter was wrong | TSK-03.7.1 logged 45,520,132 input tokens in 63 turns | No budget can be enforced on a meter nobody trusts |
| 9 | No per-session budget | The only cap was 90 minutes per group; one build took 122 turns | A stuck session burned until the group budget ran out |
| 10 | Sessions were not hermetic | Every session loaded personal instructions, settings, plugins and MCP servers; models were floating aliases; the host CLI auto-updated | Three machines ran three different agents |
| 11 | The line rebuilt itself mid-run | TSK-03.31.7 and TSK-03.32.4 were stale-binary bugs | The thing under test changed while the test ran |
| 12 | `BACKLOG.md` was the database | 159 KB, 166 tasks, 153 done; 77 of 149 commits since Sep 11 touched it; ship rewrote it and lost a task's body | Merge conflicts, lost work, and a large token cost on every read |
| 13 | Groups stacked on side branches | TG-03.31 and TG-03.32 cut from a branch 39 commits ahead of `main` | Bases drifted, and hand commits rode along unreviewed |
| 14 | Windows was untested and partly unsafe | No CI, no `.gitattributes`, `sh -c` everywhere; on Windows a timeout killed only the parent; the hook matcher missed PowerShell | On native Windows the guard was bypassed by tool choice, and timeouts left orphans |
| 15 | Silent passes | The gate skipped build checks when detection found none; a local 3B reviewer counted as a proof and missed a real bug | "Green" could mean nothing ran |

The Python prototype died the same way: "every open backlog item was about keeping the orchestrator safe from itself" (`CHANGELOG.md`, 1.0.0-alpha.1).

**Decision.** Rebuild V1 against the PRD, following the principles in `architecture.md#purpose`. Port only the parts that already work, with their tests:

| Part | Where it lives today | Why it holds |
|---|---|---|
| A pure decision function over disk state | `Next` in `internal/line/snapshot.go`, fuzzed | Deterministic, restartable, testable without a model |
| A worktree per unit of work, waves by file overlap | `internal/line/worktree.go`, `internal/plan` | Parallel builds with no shared state |
| Schema-checked results | `komodo/roles/*.schema.json`, `internal/line/schema.go` | Output is data, rejected when wrong |
| Credential scrub, then the binary pushes | `Scrub` and `finishShip` in `internal/run/run.go` | The model never pushes |
| Brief slots with caps and clip markers | `internal/line/brief_slots.go`, `clip.go` | Bounded input, the same bytes on any machine |
| The ledger | `internal/ledger` | Metrics cost zero tokens |
| The plan probe | `internal/mount/claude/limits.go` | Reads the plan and usage window without a model |
| The forge audit | `komodo doctor --remote` | Checks the one boundary a shell cannot reach |

**Alternatives.**

- **Patch the first line in place.** Its structure produced the loops.
- **Rewrite everything, the working parts included.** It would reopen problems already solved and tested.

**Consequences.**

- **Each ported part keeps its tests,** so a regression shows at once.
- **The evidence table is the standing record of why.**

## 0002. The conductor is one static Go binary with no dependencies

**Status:** Accepted, 2026-09-25. Restates the first line's decision 0001.

**Context.** The prototype needed an interpreter, a virtual environment and a shell shim on every host, and started a fresh interpreter on every hook call.

**Decision.** The conductor, its checks and its hooks are one Go binary built from the standard library alone. Each platform gets its own binary, built with cgo off, trimmed paths and stripped symbols, with the version and commit stamped in, so a rebuild of one commit is byte-identical. `bin/` is never tracked.

**Alternatives.**

- **Rust.** It fits as well, but the working code is in Go, and Go cross-compiles with two environment variables.
- **Keep Python.** An interpreter on every host, and a start-up cost on every hook call.

**Consequences.**

- **A dependency is a design change,** and needs its own entry here.

## 0003. Models read markdown, never Go

**Status:** Accepted, 2026-09-25. Restates the first line's decision 0002.

**Context.** A model that reads the conductor's source can reason about the stage order and route around it, and every token spent on Go is a token not spent on the work.

**Decision.** Rules, roles, skills, checklists and policy are markdown or JSON under `komodo/`. The conductor fills a brief from them, and a model reads only its brief. The stage order lives only in the binary.

**Alternatives.**

- **Prompts inside Go.** Faster to change, but invisible to review, and a host swap would touch them.

**Consequences.**

- **A reviewer reads one brief,** the whole input a builder got.
- **Swapping a model is a profile row.**

## 0004. Only host mounts name a host, and a host plugs in through one contract

**Status:** Accepted, 2026-09-25. Restates and extends the first line's decision 0003.

**Context.** Models must be interchangeable per stage: Claude today, Ollama and GPT models after 1.0.0. The line must survive a change of host with no change outside one directory.

**Decision.** Every vendor name, host tool name, host path and host flag lives in `internal/mount/<host>/`, and doctor fails on any leak. A host plugs in by implementing the host contract in `system-design.md#the-host-contract`: preflight, start, resume, stream, result, stop and a capability list.

**Alternatives.**

- **A config file per host.** Data alone can't carry a host's command line, usage parsing or resume.

**Consequences.**

- **A new host is a new directory.**
- **A host without resume still works,** with a fresh session given the fix list.

## 0005. A mechanical conductor runs the line, and the primary session is the orchestrator

**Status:** Accepted, 2026-09-25.

**Context.** A model relaying stages failed in the ways evidence 1 and 5 record. The owner wants the primary session to be the one place a person talks to the line: spawning agents, answering questions about them, tearing down, and taking ad hoc requests. Routing should be as mechanical as possible.

**Decision.** The `komodo` binary is the conductor: it runs all eight stages, spawns and resumes sessions, enforces limits, and does all git work. The primary session is the orchestrator: it plans, starts and watches runs, answers questions, injects groups, runs single stages ad hoc, and settles escalations.

**Alternatives.**

- **The primary session routes stages, as in the first line.** Unmetered tokens, and one bad turn skips a stage.
- **No orchestrator.** Nobody could settle a blocked group without a person.

**Consequences.**

- **The normal path spends no orchestration tokens.**

**Spikes.** S2: do the host's `dontAsk` mode, an allow list and the sandbox run a builder with no prompt and no refusal? S7: can `komodo run`, started inside an interactive session, launch its own sessions? S8: what do `CLAUDE_CODE_MAX_TURNS` and `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB` do in a headless session?

## 0006. Every session is pinned, hermetic and scoped to its role

**Status:** Accepted, 2026-09-25.

**Context.** Sessions loaded personal configuration and floating model aliases, and the host auto-updated (evidence 10). Skills loaded everywhere, costing tokens in sessions that never used them.

**Decision.** Pin the host CLI, the full model IDs, the `komodo` release, the toolchains and LF line endings. Each line session loads a line-owned config directory and its role's own plugin, and nothing personal. The primary session loads only the orchestrator's skills. The meter reads the host's own totals.

**Alternatives.**

- **The host's bare mode.** It shuts out personal config, but also drops subscription login and hooks.
- **Light-tier builders for small work.** The 3 Haiku builds averaged 75 turns (evidence 7).

**Consequences.**

- **Every machine runs the same agents.** Doctor fails on any pin that differs.

**Spikes.** S3: does a line-owned config directory shut out personal instructions, plugins and MCP, and keep the host login? S4: does the final stream event carry turns, usage, cost and a session ID that resume accepts? S5: does schema output hold over a long builder session?

## 0007. One builder session works one task group of 1 to 12 tasks

**Status:** Accepted, 2026-09-25.

**Context.** The owner's model is a task list handed to one agent and cross-checked by one reviewer, the size of one engineering story. A session per task pays the fixed prompt again for every task.

**Decision.** A group of 1 to 12 tasks, typically 2 to 6, gets one builder session in one worktree, and its task list is the brief. Ingest refuses a larger group and suggests a split. Groups that share no file run in parallel.

**Alternatives.**

- **One session per task.** It repeats the fixed prompt, and the reviewer sees fragments of one story.
- **No size limit.** A long session degrades; the owner saw a light-tier builder burn a million tokens.

**Consequences.**

- **One group is one pull request,** bounded by its 60-minute limit.

## 0008. Ingest compiles each task group into a card, and the conductor ticks its checkboxes

**Status:** Accepted, 2026-09-25.

**Context.** Builders spent turns finding context (evidence 6), and every task needed hand-written checks.

**Decision.** `komodo ingest` compiles each READY group into one card, with zero model calls: task list, files, derived checks, context pack, size and base. The conductor ticks a task's checkbox only after its checks pass. The binary suggests splits; a person applies them.

**Alternatives.**

- **The builder ticks its own boxes.** A model's word is never the proof.

**Consequences.**

- **The same card and tree give the same brief bytes** on every machine.

## 0009. The backlog is a committed plan, one file per group, and finished files go at the end of their epic

**Status:** Accepted, 2026-09-25.

**Context.** A single `BACKLOG.md` became a database (evidence 12). The owner wants the backlog committed as a long-running plan that keeps agents in sync across workloads and shows that work was completed correctly. Files that are no longer needed should go when their epic or group ends, with `CHANGELOG.md` as the record, not an archive.

**Decision.** Each task group is a committed file in `docs/backlog/`, carrying its epic's ID. Changes to the plan land through pull requests like any other change. The conductor ticks a group's boxes on the group's own branch, so its PR shows the code and the completed task list together. When the last open group of an epic merges, that group's PR also deletes the epic's group files; a group with no epic deletes its own file. Each group's PR adds its line to `CHANGELOG.md`. There is no index and no archive: `komodo backlog` lists the open groups.

**Alternatives.**

- **One committed `BACKLOG.md`.** Evidence 12.
- **A local, uncommitted backlog.** Agents on other machines couldn't see the plan.
- **An archive of finished groups.** It duplicates the changelog.

**Consequences.**

- **Parallel groups never edit the same file,** so their PRs don't conflict over the plan.
- **An epic's finished groups stay visible until the epic ends,** so later groups can see what was done.

## 0010. Review runs parallel lenses, resumes for re-review, and loops only while findings shrink

**Status:** Accepted, 2026-09-25.

**Context.** Review never converged (evidence 3). The owner wants a large, unbiased review against a checklist, backed by mechanical validators, waiting in stasis during repairs. A fixed repair count is arbitrary and only defers the loop.

**Decision.** Validators run first. Three lenses run in parallel on the group's diff and task list: correctness, security and readiness, and quality; economy mode runs one combined lens. A finding blocks only with evidence the conductor verifies. Repair resumes the builder with a fix list. Re-review resumes the lens sessions, which may only close their findings or flag lines the repair changed. A round that closes nothing ends the loop.

**Alternatives.**

- **A fixed repair count.** Arbitrary, and it defers the loop rather than ending it.
- **One reviewer for every group.** Its context grows with each group and it stops being unbiased.
- **A fresh reviewer each round.** It raises new objections to code it never saw, as in TG-03.22.

**Consequences.**

- **Differences between reviewers' reports can't cause loops,** since only verified findings block.

## 0011. Escalations go to the orchestrator; what it can't settle stops the group, documented in the backlog

**Status:** Accepted, 2026-09-25.

**Context.** The owner doesn't want to watch inbox files. Problems should be handled inside the line; what can't be handled should stop, be written down where the owner reads, and wait.

**Decision.** The conductor sends every escalation to the orchestrator, which settles it with one allowed action when it can. When it can't, the conductor saves the work, marks the group BLOCKED, and writes a blocker note into the group's backlog file on the group's branch. It publishes the branch as a draft PR labelled `status: blocked`, so every developer and agent can see the note. Other groups continue. A person, or the orchestrator on their word, edits the group and marks it READY, and `komodo resume` continues from that branch. A headless run exits non-zero when any group is blocked.

**Alternatives.**

- **A pages inbox file.** The owner won't look at it.
- **Keep retrying.** It burns tokens on a problem that needs a person.

**Consequences.**

- **The backlog, and the blocked draft PR that carries it, is the one place to look** for anything that needs a person.

## 0012. Safety comes from the human merge, draft PRs, credential isolation, the sandbox and output checks

**Status:** Accepted, 2026-09-25. Replaces the first line's decisions 0004 and 0005.

**Context.** The guard was scored as a wall and never could be one (evidence 2). The forge is GitHub Free, and the developer's local git PAT is the only credential. GitHub Free offers rulesets and draft PRs only on public repositories.

**Decision.** Only the conductor reads the git credential, and only at Ship. Every PR opens as a draft, or labelled `status: wip` where drafts aren't offered, and becomes ready once verified. A human merges. Line sessions run in the OS sandbox where the platform has one, with the forge off the network allowlist. Check compares outputs. The guard keeps five rules, catches mistakes, and fails open.

**Alternatives.**

- **A bot account.** The current plan doesn't provide one.
- **Keep hardening the bash denylist.** It can never be complete.

**Consequences.**

- **The developer authors every PR,** so their own approval doesn't count on GitHub; the merge is the human check.

**Spikes.** S1: do Go builds, module downloads and race tests pass under the sandbox on macOS and WSL2?

## 0013. Every hook has one job, one stage and a refusal limit

**Status:** Accepted, 2026-09-25.

**Context.** Most loops came from hooks and guards (evidence 2 and 5), including a regression where the guard judged the line's own commands. While these docs were written, the guard refused a Python script because its text held the word "git", and refused editing a script and running it in one command.

**Decision.** Every hook follows the contract in `system-design.md#hooks`: one check, one kind of session, an alternative named on every refusal, a refusal limit after which the session ends as blocked, and fail-open on its own error. Hooks never judge the conductor's commands and never parse a command for hidden intent.

**Alternatives.**

- **More guard rules.** Each new rule invited new bypass findings and new loops.

**Consequences.**

- **A refusal costs a few turns at most** before the group escalates.
- **Guard-bypass findings never block review.**

## 0014. Each role's allow list covers its stage, and the conductor does git housekeeping

**Status:** Accepted, 2026-09-25.

**Context.** Agents were refused commands their work needed, such as deleting files or switching to `main` to sync.

**Decision.** Each role gets the allow and deny lists in `system-design.md#permissions`, applied with the host's `dontAsk` mode. Builders may delete and move files in their worktree. Switching branches, syncing, committing and cleanup belong to the conductor. The critical-ref rule refuses writes to `main`, not switching to it or fast-forwarding it.

**Alternatives.**

- **Bypass permissions, with the guard as the only wall.** Evidence 2.

**Consequences.**

- **Golden runs should record no refusal of an allowed command.**

## 0015. The orchestrator may edit this repo, its rules and guard included, on a branch

**Status:** Accepted, 2026-09-25.

**Context.** The owner was blocked from changing this repo by its own guard and rules, which protected `komodo/policy.json` and `bin/**`.

**Decision.** In this repo, the primary session may edit rules, skills, policy and guard source on a branch. A change applies only after a human merges it and the binary rebuilds, so no session loosens its own guard. A builder may edit those files only when its task list names them. Only the build writes binaries.

**Alternatives.**

- **Keep the self-protection.** It blocks the owner's own maintenance.

**Consequences.**

- **The policy's protected paths name installed copies,** not this repo's sources.

## 0016. The line paces to the plan: bound on subscriptions, budgeted on API billing

**Status:** Accepted, 2026-09-25.

**Context.** The owner wants the line tuned to the plan and able to run for hours unattended. The plan probe reads the subscription type and window, and the host's stream reports rate limits with reset times.

**Decision.** Subscriptions are bound: the conductor sets concurrency from the plan, pauses at a usage limit, and resumes at the reset. A Pro plan runs economy mode: the economy profile and one review lens with its own prompt. API billing is unbound: a spend budget per run applies.

**Alternatives.**

- **One setting for every plan.** A Pro plan would hit its limits mid-group.

**Consequences.**

- **Economy mode trades some review depth for cost,** and eval measures how much.

## 0017. Windows runs natively first, and uses WSL2 when present

**Status:** Accepted, 2026-09-25.

**Context.** The owner wants the line to work out of the box with little setup. WSL2 needs a Windows feature turned on; native Windows needs only Git for Windows, which the line needs anyway. The host's sandbox runs in WSL2 but not natively.

**Decision.** Ship a native `windows/amd64` binary and `install.ps1`. Native Windows runs without the OS sandbox, and the other layers carry the load. Where WSL2 is present, the Linux install inside it gets the sandbox. A job object kills process trees on native Windows.

**Alternatives.**

- **WSL2 only.** More setup than the owner wants.

**Consequences.**

- **Native Windows is the one platform without an OS sandbox,** and doctor says so.
- **The guard's matcher covers PowerShell.**

**Spikes.** S6: does the line run natively on Windows 10 and 11 with Git for Windows, and inside WSL2 where present?

## 0018. The gate is local, binaries rebuild on pull, and `komodo release` publishes; nothing runs on the forge

**Status:** Accepted, 2026-09-25. Restates and extends the first line's decision 0006.

**Context.** Hosted CI costs minutes and moves failure away from the person who can fix it. `bin/` is gitignored and nothing rebuilt it after a pull, so the owner ran commands by hand to get the latest binaries.

**Decision.** `komodo gate` runs before every commit and push, with no model. The gate also installs post-merge, post-checkout and post-rewrite hooks that rebuild the binary when Go sources changed. `komodo release` cross-compiles every platform, runs the tests, writes checksums and publishes a GitHub Release from the owner's machine. Product repos use a published release.

**Alternatives.**

- **Commit the binaries.** About 9 MB per platform per change would stay in git history forever.
- **CI on the forge.** The owner prefers none; eval on each platform proves cross-platform behaviour.

**Consequences.**

- **Nobody runs a command to get the latest binary.**
- **A hook can be skipped with `--no-verify`;** the human merge and draft-first PRs are the backstop.

## 0019. One install script per platform family installs, links and initialises

**Status:** Accepted, 2026-09-25.

**Context.** `komodo` is not a command anyone has until it is installed, and the owner wants everything hooked up in one step.

**Decision.** `install.sh` and `install.ps1` check prerequisites, build or download the binary, link it onto PATH, install the global orchestrator layer, initialise the current repo and run doctor. Running one again updates the install. The `komodo` skill, generated from `komodo help`, documents the command for agents; the README documents it for people.

**Alternatives.**

- **Manual steps in the README.** Every machine drifts.

**Consequences.**

- **The first install also initialises the repo it runs in.**

## 0020. Extension points ship disabled: notifiers, tool packs and stage hooks

**Status:** Accepted, 2026-09-25.

**Context.** The owner plans Slack, Google Chat and cloud-command plugins later, and wants V1 ready for them.

**Decision.** V1 defines the three plugin types, each with a manifest, installs them disabled, and enables them per machine. A notifier copies blocker notes and run summaries somewhere else; it never decides anything.

**Alternatives.**

- **Build the integrations now.** None is needed for 1.0.0.

**Consequences.**

- **Adding Slack later is a plugin,** not a change to the conductor.

## 0021. Readiness is measured by `komodo eval`, never by a model's score

**Status:** Accepted, 2026-09-25.

**Context.** A self-graded score defined "done" (evidence 4), and green could mean nothing ran (evidence 15).

**Decision.** `komodo eval` runs golden task groups in real repos, with hidden tests, against the PRD's success criteria. The gate fails loudly when it finds no build check. A local model is optional and never a proof.

**Alternatives.**

- **A rubric a model scores.** It swung between 72 and 88 on the same code.

**Consequences.**

- **Only a proof moves readiness.**

## 0022. Codex and any second host wait until after 1.0.0

**Status:** Accepted, 2026-09-25.

**Context.** The PRD scopes 1.0.0 to one host.

**Decision.** The Codex mount stays in the code, and nothing new lands for it. The same holds for the cloud facets, new standards skills, the local reviewer and MCP.

**Alternatives.**

- **Two hosts in 1.0.0.** Every proof would run twice before one host is proven.

**Consequences.**

- **Each parked area keeps its code,** and returns with a task after 1.0.0.

## 0023. V1 restarts at alpha and moves through beta to an LTS release

**Status:** Accepted, 2026-09-25.

**Context.** The owner wants a fresh start: V1 alpha, beta, then LTS. Tags `v1.0.0-alpha.1` to `.4` exist from the prototype. `1.0.0-beta.1` is a changelog heading but was never tagged.

**Decision.** The rebuild ships as `1.0.0-alpha.5` onward while phases 0 to 3 land. Beta starts at `1.0.0-beta.2`, feature-complete, with only fixes landing while eval runs on every platform; `beta.2` avoids reusing the old heading. `1.0.0` is the LTS release, cut by the owner once the PRD's success criteria hold.

**Alternatives.**

- **Start a new series such as `1.1.0-alpha.1`.** Clean numbering, but it implies 1.0.0 already shipped.
- **Reuse `1.0.0-beta.1`.** Two changelog sections would share one heading.

**Consequences.**

- **Every existing tag stays valid,** and none is rewritten.

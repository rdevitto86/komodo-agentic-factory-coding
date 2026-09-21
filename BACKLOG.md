# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the line parses is in the `backlog` skill; `komodo lint` checks it from TG-03.2 on. The V1.x backlog was dropped whole on 2026-09-21 in favour of the V2 plan in `README.md`.

---

## [EPIC-03] V2, the assembly line
*Goal: one static binary is the conveyor and the devices, markdown is everything a model reads, one guard is the only hook, and a model is a machine mounted per host. Two model calls per task, build and review; everything between is deterministic. Claude Code with Ollama is the host today for both Komodo devs; Codex is the exit test. Requirements and design: `README.md`.*

* **Everything lands on PR #103 through a stack.** Each group is one PR based on the group before it and merges into that parent; #103 merges into `main` last and the tag `v2.0.0` is cut then. Branches, bases, and the validation per PR are the README's Pull requests section, and each group below names its own.
* **Sessions build TG-03.1 through TG-03.4.** The `run` skill runs TG-03.5 and TG-03.6, and TG-03.5 is the proof.
* **The repo starts clean.** V1 is the tag `v1-final`; the only V1 files here are the markdown TG-03.1 reshapes.
* **Nothing runs on GitHub.** `komodo gate` is the only precheck, mechanical, local, before every commit and push. No workflow directory exists.
* **No MCP in V2.** Machines, skills, and external dependencies are the swappable parts; a facet's `mcp.json` is reserved for a later pass and nothing reads it.

### [TG-03.1] The markdown
```yaml
type: refactor
version: 2.0.0
```
* **Why:** everything a model reads is one of three neutral formats. Standards become skills so their trigger is their own frontmatter. Briefs fold into roles so a role is the brief and its schema. The policy shrinks to four denials for a greenfield shop, and the rules give an agent unlimited freedom inside its worktree.
* **PR A:** `refactor/v2-markdown` from `docs/v2-plan`, opened by the session with `gh pr create`. Validated by every `done_when` here, the V1 linter from a scratch worktree of `v1-final` at zero problems, and a human read of `komodo/AGENTS.md` and `komodo/policy.json`. Merges into `docs/v2-plan`.

#### [TSK-03.1.1] Standards become skills at the source [P: C] [DONE]
```yaml
files: [komodo/skills, komodo/standards]
done_when:
  - test -f komodo/skills/standards-go/SKILL.md
  - test ! -d komodo/standards
  - grep -q '^globs:' komodo/skills/standards-go/SKILL.md
context:
  - "each komodo/standards/<x>.md moves to komodo/skills/standards-<x>/SKILL.md with frontmatter name, description, and a globs list of the extensions and directories that trigger it; the body does not change"
  - "the extension map V1 kept in standards.py, see the tag v1-final, moves into that frontmatter and nothing else holds it"
type: refactor
```

#### [TSK-03.1.2] Briefs fold into roles, and each role carries its schema [P: C] [DONE]
```yaml
files: [komodo/roles, komodo/briefs]
done_when:
  - test ! -d komodo/briefs
  - test -f komodo/roles/builder.schema.json
  - grep -q '{{task_block}}' komodo/roles/builder.md
context:
  - "a role file carries frontmatter name, tier, tools, session, returns, and a body that is the brief template with the slots task_block, repo_rules, repo_context, context, files, standards, done_when, failure"
  - "tools are the five Komodo verbs read, edit, write, shell, search; a host tool name never appears in a role"
  - "the JSON schema each worker prompt carried in V1 briefs.py, see the tag v1-final, moves beside the role as <role>.schema.json; the builder's body says to write its result to the path the brief names"
type: refactor
```

#### [TSK-03.1.3] The policy has four denials, and the rules give a worktree unlimited freedom [P: C] [DONE]
```yaml
files: [komodo/policy.json, komodo/AGENTS.md, komodo/rules]
done_when:
  - test -f komodo/policy.json
  - test -f komodo/AGENTS.md
  - test ! -f komodo/rules/AGENTS.md
context:
  - "policy.json: critical refs main and master plus a list, config paths the hosts and the toolkit own, and the trailer patterns; no destructive command list, no prod markers, no file-list scope"
  - "komodo/rules/AGENTS.md moves to komodo/AGENTS.md; its Git section says an agent may delete files, reset, checkout, restore, force-push and delete its own branches inside its worktree, and never touches a critical branch, a path outside the worktree, or a host or toolkit config"
  - "komodo/rules/accessibility.md holds the Writing for a human contract and is included into AGENTS.md at render"
type: feat
```

#### [TSK-03.1.4] The merger role goes; the responder stays as a session role [P: M] [DONE]
```yaml
files: [komodo/roles/merger.md, komodo/roles/responder.md]
done_when:
  - test ! -f komodo/roles/merger.md
  - grep -q '^session: true' komodo/roles/responder.md
context:
  - "QC stops on a conflict and hands it to a human, so no role resolves conflicts; responder becomes session: true and is the model behind the respond skill"
  - "nine roles remain: architect, builder, planner, researcher, responder, reviewer, scout, summarizer, tester"
type: refactor
```

### [TG-03.2] The conveyor and the devices
```yaml
type: feat
version: 2.0.0
```
* **Why:** the line is one static Go binary with no interpreter, shell, or symlink on a dev machine. Every station is a subcommand with a test, and every station stamps the ledger. V1's 1013 lines of Go hooks, at the tag `v1-final` under `komodo/hooks/src`, are the seed of the module.
* **PR B:** `feat/v2-conveyor` from `refactor/v2-markdown`, opened by the session. Validated by `komodo gate`, `komodo lint`, `komodo next --json` printing TG-03.3, `komodo brief --dry-run TSK-03.3.1`, `komodo close` on a hand-written result, `komodo step` printing one action, and a human read of one brief. Merges into `refactor/v2-markdown`.

#### [TSK-03.2.1] The Go module, the binary, lint, and the gate [P: C] [DONE]
```yaml
files: [go.mod, cmd/komodo/main.go, internal/backlog, internal/gate, bin, .gitattributes]
done_when:
  - go build ./...
  - go test ./internal/backlog/... ./internal/gate/...
  - go run ./cmd/komodo lint
  - go run ./cmd/komodo gate
context:
  - "go.mod at the repo root, module komodo, Go 1.22, no dependencies outside the standard library; the V1 hook sources at the tag v1-final come in under internal with their tests"
  - "internal/backlog ports tasks.py: parse, lint, list, add, set status, next task id, find backlog at the root or docs; komodo lint and komodo list and komodo add are the first subcommands"
  - "bin/ holds komodo-darwin-arm64, komodo-windows-amd64.exe, komodo-linux-amd64 and MANIFEST.sha256, built with CGO_ENABLED=0, -trimpath, -buildvcs=false, and -ldflags -s -w so a rebuild is byte-identical and each file stays near 3 MB; .gitattributes marks bin/** binary so no diff, brief, or review ever carries their bytes"
  - "komodo gate is the only precheck and runs locally: go vet, go test, the three binaries rebuilt and compared to the manifest, and doctor and guard check once they exist; gate --install writes pre-commit and pre-push for this repo that run it through the platform's binary; no workflow directory, no CI, nothing on GitHub"
type: feat
```

#### [TSK-03.2.2] Intake: `komodo next` [P: C] [DONE]
```yaml
files: [internal/line/next.go, internal/line/next_test.go, internal/line/worktree.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.2.1]
context:
  - "next prints the next READY group, or the group of a named task, as JSON: tasks, dependencies, waves by directory, mode single as one wave, type, version, and the resolved machine per role; nothing when nothing is ready"
  - "next --start fetches the base, creates <type>/<slug> in a worktree under .komodo/wt/group from its head, and calls the preflight tag; --base names the branch, default the remote's default branch, and the choice is recorded in .komodo/run.json so close and ship target the same branch; the main working tree is never read or required clean"
  - "a task with a valid .komodo/results/<task>.json is listed done and skipped, which is resume; done and in-progress rewrite only the status token"
type: feat
```

#### [TSK-03.2.3] Input device: `komodo brief` [P: C] [DONE]
```yaml
files: [internal/line/brief.go, internal/line/brief_test.go, internal/line/clip.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.2.2]
context:
  - "fills the role template from the slots the README tables, each clipped by its cap with the head 70 tail 25 marker briefs.clip used; pipeline.task_slots and pipeline.repo_rules are the prior art"
  - "writes .komodo/briefs/<task>.md in a task worktree under .komodo/wt/<task> branched from the group branch and prints both paths as JSON; the builder's result path is named in the brief"
  - "--dry-run prints each slot's characters after clipping and a token estimate and writes nothing"
  - "a listed file that is not text is named in the files slot with its path and size and never read; nothing under bin/ is read by any slot"
type: feat
```

#### [TSK-03.2.4] Output device: `komodo close <task>` [P: C] [DONE]
```yaml
files: [internal/line/close.go, internal/line/close_test.go, internal/line/schema.go, internal/comments]
done_when:
  - go test ./internal/line/... ./internal/comments/...
depends_on: [TSK-03.2.3]
context:
  - "validates the result JSON against the role schema for type, required, and enum; reruns done_when in the worktree; runs the comment lint on the task's files; runs komodo gate when the repo is this toolkit; flips the status"
  - "a failure writes the failure slot and the previous attempt's diff so the next brief is a repair, at most one; a second failure marks BLOCKED with the note and the wave continues, as pipeline._block did"
  - "internal/comments ports comment_rules.py; komodo comments check stays as a command"
type: feat
```

#### [TSK-03.2.5] QC and ship: `komodo close --wave` and `--group` [P: C] [DONE]
```yaml
files: [internal/line/wave.go, internal/line/ship.go, internal/line/verify.go, internal/line/wave_test.go, internal/pr]
done_when:
  - go test ./internal/line/... ./internal/pr/...
depends_on: [TSK-03.2.4]
context:
  - "--wave merges the wave's worktrees into the group branch in order and stops on the first conflict naming both tasks; runs the compile gate for the languages the wave touched with one repair as gates.compile_commands did; then the verify command, resolved in V1's order or named by .komodo/commands.json"
  - "review: findings at or above severity_floor become one repair brief; the rest are appended to BACKLOG.md as tasks by code"
  - "--group commits, pushes, opens the PR against the run's base with the report as body and the category and agent labels only when the repo defines them, as a draft when a task is blocked; writes the changelog line under the group's version; flips DONE; works on any branch with --base, which is how freehand work and a stacked group ship"
  - "internal/pr ports pr.py: view, create, edit, threads, label, comment, reply as gh wrappers"
type: feat
```

#### [TSK-03.2.6] `komodo diff`, `report`, `tag`, and `release check` [P: H] [DONE]
```yaml
files: [internal/line/diff.go, internal/line/report.go, internal/release, internal/line/diff_test.go]
done_when:
  - go test ./internal/line/... ./internal/release/...
depends_on: [TSK-03.2.5]
context:
  - "diff prints the group branch against base, clipped by the failure cap, with the group's task blocks and the standards the diff's extensions touch; it is the whole reviewer input; a path .gitattributes marks binary appears as one line and never as bytes"
  - "report prints per task seconds, turns, tokens when the result carries them, findings by severity, and what blocked, in the accessibility contract; render.report is the prior art"
  - "internal/release ports render.py's changelog functions; tag tags every changelog version no tag points at, annotated v<version>, and pushes it from the group worktree on a clean base; release check audits drift read-only and exits non-zero"
type: feat
```

#### [TSK-03.2.7] The ledger and `komodo metrics` [P: H] [DONE]
```yaml
files: [internal/ledger, internal/line, cmd/komodo/main.go]
done_when:
  - go test ./internal/ledger/... ./internal/line/...
depends_on: [TSK-03.2.6]
context:
  - "internal/ledger appends one JSON line per station event to .komodo/line.jsonl for a run and .komodo/adhoc.jsonl for anything off the line, including komodo add; fields run, group, task, wave, station, role, tier, host, provider, model, seconds, tokens_in, tokens_out, turns, outcome, failure_class, findings"
  - "line.jsonl is truncated by next --start; adhoc.jsonl is truncated by its next writer when the first line is older than 24 hours or the file exceeds 1 MB; both are gitignored and nothing reads them but report and metrics; nothing is sent anywhere and nothing goes into a PR body"
  - "seconds come from the binary's own timestamps; tokens and turns are filled by the mount's usage function from TSK-03.3.5 when it can, else left empty, and the Ollama mount fills them from the response counts at once; metrics prints median seconds per station, failure rate by class, tokens per task by model, repair rate, and findings per group as text"
type: feat
```

#### [TSK-03.2.8] `komodo step`: the binary drives the session [P: C] [DONE]
```yaml
files: [internal/line/step.go, internal/line/step_test.go, cmd/komodo/main.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.2.7]
context:
  - "step reads the run's state from the ledger and the results on disk and prints the one next action as JSON: spawn a role with a brief path, run a komodo command, wait until a time, or done; the station order exists only here"
  - "a role whose machine resolves to ollama comes back as run komodo machine, never as a spawn; every action names the machine, skills, facets, and commands it resolved for that station"
  - "the run skill becomes three lines: call step, do what it says, repeat; a group, a task, or nothing as the argument; no station name appears in any skill"
type: feat
```

### [TG-03.3] The guard and the mounts
```yaml
type: feat
version: 2.0.0
```
* **Why:** one hook on every host, four denials, and unlimited freedom inside a worktree. A mount is the only code that knows a host. Profiles select themselves from the host, the plan, and whether Ollama answers.
* **PR C:** `feat/v2-guard-mounts` from `feat/v2-conveyor`, opened by the session. Validated by `komodo gate` with `guard check`, `install --host claude --dry-run`, `komodo doctor` under the budgets with no leak, the probe printing an overlay and no identity, then a real install on this Mac where a session is denied a commit to `main` and allowed `rm` in its worktree. Merges into `feat/v2-conveyor`.

#### [TSK-03.3.1] The guard: four denials, worktree scope, and the table in the gate [P: C] [DONE]
```yaml
files: [internal/guard, cmd/komodo/main.go]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.2.1]
context:
  - "komodo guard reads hook_event_name, cwd, tool_name, tool_input on stdin and denies with the PreToolUse permissionDecision JSON both hosts accept, exit 2 as the fallback"
  - "denies exactly: commit, push, merge, delete, or force on a critical ref; an edit, write, delete, or move whose path leaves the worktree root, or the repo root off the line; any write under a host home, the machine overlay, .git/config, .git/hooks, or bin; a trailer in a commit message; nothing else, so rm, reset, checkout, and a force-push to the agent's own branch pass"
  - "an internal error logs one line to stderr and allows; guard check runs a table of at least 60 commands, half allowed, and names the finding for each denial; the guard.go in komodo/hooks/src is the prior art"
type: feat
```

#### [TSK-03.3.2] `komodo install --host claude|codex|both`: the mounts [P: C] [DONE]
```yaml
files: [internal/mount/claude, internal/mount/codex, internal/install, cmd/komodo/main.go]
done_when:
  - go test ./internal/mount/... ./internal/install/...
  - go run ./cmd/komodo install --dry-run
depends_on: [TSK-03.3.1]
context:
  - "claude: CLAUDE.md with an @AGENTS.md include, agents from roles with model from the profile, the run, review, backlog, and respond skills copied, the standards skills copied, the guard registered once on PreToolUse, the permissions convenience layer from policy.json, the personal overlay CLAUDE.local.md and settings.local.json seeded once and never overwritten; the V1 render and the old MCP entry at 127.0.0.1:8000 removed when present; no MCP server is registered"
  - "codex: AGENTS.md, agents/<role>.toml with name, description, developer_instructions, model, model_reasoning_effort, sandbox_mode, skills copied under .agents, the guard in hooks.json; no MCP server is registered"
  - "the five Komodo verbs map to host tool names or sandbox_mode here and nowhere else; install is a copy, never a symlink, and runs on Windows; --dry-run prints what would change"
type: feat
```

#### [TSK-03.3.3] Doctor: references, leaks, drift, budgets, prune [P: H] [DONE]
```yaml
files: [internal/doctor, cmd/komodo/main.go]
done_when:
  - go test ./internal/doctor/...
  - go run ./cmd/komodo doctor --no-git
depends_on: [TSK-03.3.2]
context:
  - "keeps V1's checks: backticked references resolve, roles are well formed, changelog and tags agree, git leftovers; adds: a vendor name, host tool, host path, or host flag outside internal/mount fails; the rendered layout differs from what the source renders now fails; always-on context over 1500 tokens, the run skill over 800, or a standard over 8 KB fails"
  - "--prune removes stale worktrees under .komodo/wt and deletes branches merged into base; --json stays"
type: feat
```

#### [TSK-03.3.4] Profiles select themselves: host, plan probe, Ollama, and pacing [P: H] [DONE]
```yaml
files: [internal/profile, internal/mount/claude/limits.go, internal/line/next.go]
done_when:
  - go test ./internal/profile/... ./internal/line/...
depends_on: [TSK-03.3.2, TSK-03.2.2]
context:
  - "profiles claude, hybrid, codex, local as the README tables them, plus caps for every brief slot, severity_floor, max_parallel, labels, changelog, remote, base; ~/.komodo/config.json overlays and can only lower a cap or add a critical ref"
  - "selection with no flag: the host from the mount installed, hybrid when Ollama answers on its URL, the plan overlay from the probe; the Claude mount reads oauthAccount and cachedUsageUtilization from the host config file, never the CLI status line, and never reads or logs the email or ids; Codex returns nothing"
  - "plan overlays pro, max_5x, max_20x, unknown: heavy ceiling, max_parallel, review floor and the diff size under which review is skipped, repairs, pause_at and warn_at; next prints wait_until instead of a wave past pause_at, so a run pauses before a wave and never inside one; no probe is the unknown overlay, never a failed run"
type: feat
```

#### [TSK-03.3.5] Each mount reports a machine's usage after the fact [P: H] [READY]
```yaml
files: [internal/mount/claude/usage.go, internal/mount/codex/usage.go, internal/ledger]
done_when:
  - go test ./internal/mount/... ./internal/ledger/...
depends_on: [TSK-03.3.2, TSK-03.2.7]
context:
  - "usage(task) returns tokens in, tokens out, and turns for the agent that built or reviewed a task, or nothing; the Claude mount reads the stream output of a headless run or the session transcript file the host writes for that agent; the Codex mount reads codex exec --json events and returns nothing in a session until its logs expose usage"
  - "the transcript is read for counts only; no message text is copied anywhere"
type: feat
```

### [TG-03.4] The skills and the launcher
```yaml
type: feat
version: 2.0.0
```
* **Why:** the run skill is the list of stations and the two spawns, under 800 tokens. Ad hoc work enters at any station or stays off the line. The proof runs one group each way before anything is deleted.
* **PR D:** `feat/v2-skills-launcher` from `feat/v2-guard-mounts`, opened by the session. Validated by `komodo doctor` with the run skill under 800 tokens, the launcher's scrub test, `komodo run --dry-run TG-03.5`, and `/run` in a session printing the first step and stopping. TSK-03.4.3 stays open and is filled in by PR E. Merges into `feat/v2-guard-mounts`.

#### [TSK-03.4.1] The run, review, backlog, and respond skills [P: C] [READY]
```yaml
files: [komodo/skills/run/SKILL.md, komodo/skills/review/SKILL.md, komodo/skills/backlog/SKILL.md, komodo/skills/respond/SKILL.md]
done_when:
  - go run ./cmd/komodo doctor --no-git
depends_on: [TSK-03.3.3]
context:
  - "run takes a group, a task, or nothing and is three lines: call komodo step, do what it says, repeat; the station order lives in the binary and never in a skill"
  - "review runs QC and the reviewer on the current diff; backlog writes tasks in the grammar with add and lint and is where the planner role works; respond lists unresolved threads and, as the responder role, changes code when the reviewer is right and replies when they are not"
  - "a skill names no host tool, path, flag, or vendor; it says spawn the builder role and the mount decides how"
type: feat
```

#### [TSK-03.4.2] The headless launcher: `komodo run` [P: H] [READY]
```yaml
files: [internal/run, cmd/komodo/main.go]
done_when:
  - go test ./internal/run/...
depends_on: [TSK-03.4.1]
context:
  - "scrubs push tokens, credential helpers, and SSH identities from the environment, picks the host from the profile, invokes the host's non-interactive mode with the run skill and the group or task id, kills it at group_budget_s, and exits with the host's code; gitops.worker_env is the prior art for the scrub"
type: feat
```

#### [TSK-03.4.3] Proof: one group under V1 and under the run skill [P: C] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "Proof: V1 versus the run skill" CHANGELOG.md
depends_on: [TSK-03.4.1, TSK-03.4.2]
owner: human
context:
  - "run TG-03.5 under the run skill in a Claude Code session, committing to this branch; record wall time, tokens, and turns beside the V1 numbers for TG-02.4 from its run state at the tag v1-final, under a Proof: V1 versus the run skill heading in the 2.0.0 changelog entry"
  - "slower by more than one wave means the skill is wrong; fix the skill before TG-03.5 merges"
type: docs
```

### [TG-03.5] The repo layer and the local machines
```yaml
type: feat
version: 2.0.0
```
* **Why:** one universal set of rules, one place a repo adds what only it knows, and a local machine on every host. Nothing here is required and nothing here widens what the guard denies.
* **PR E:** `feat/v2-repo-layer` from `feat/v2-skills-launcher`, cut by `/run TG-03.5` and opened by `close --group` with the report as the body. Validated by `komodo gate`, `komodo doctor` with profile drift, `komodo detect` printing Go and no cloud, the Ollama mount against the fake and one real local review, a facet swap reaching a brief, and the proof numbers for TSK-03.4.3 in the changelog. Merges into `feat/v2-skills-launcher`.

#### [TSK-03.5.1] Repo context injects by glob [P: H] [READY]
```yaml
files: [internal/repo/context.go, internal/repo/context_test.go, internal/line/brief.go, templates/project]
done_when:
  - go test ./internal/repo/... ./internal/line/...
context:
  - ".komodo/context/*.md with a paths glob list in frontmatter fills the repo_context slot for any task whose files match, clipped; no globs means every task; a malformed file is skipped with one line in the report; templates/project gains one example"
type: feat
```

#### [TSK-03.5.2] Repo standards and repo skills, rendered by `install --project` [P: H] [READY]
```yaml
files: [internal/repo/standards.go, internal/repo/skills.go, internal/install, internal/mount/claude, internal/mount/codex, internal/doctor]
done_when:
  - go test ./internal/repo/... ./internal/install/...
depends_on: [TSK-03.5.1]
context:
  - ".komodo/standards/<name>.md appends to the shipped standard of that name in the standards slot or adds a new one with its own globs; .komodo/skills/<name>/SKILL.md is a new skill or appends a Repo overrides section to a shipped one, frontmatter untouched; run, review, backlog, and respond cannot be appended to; a repo never replaces or removes a shipped body"
  - "install --project renders the repo skills and the repo rules file into the host's project directory as gitignored copies; doctor reports drift between .komodo and the copies"
type: feat
```

#### [TSK-03.5.3] Repo commands and additive policy [P: H] [READY]
```yaml
files: [internal/repo/commands.go, internal/line/verify.go, internal/guard]
done_when:
  - go test ./internal/repo/... ./internal/line/... ./internal/guard/...
depends_on: [TSK-03.5.2]
context:
  - ".komodo/commands.json: verify, compile, before_review, after_publish, each a shell command the line runs at that station; verify here overrides the discovery order"
  - ".komodo/policy.json adds critical refs; the guard merges it under the machine overlay and nothing in it can remove a denial"
  - ".komodo/facets lists facets detection missed, one name per line; it can add, never remove"
type: feat
```

#### [TSK-03.5.4] `komodo machine`: the binary is the Ollama mount [P: C] [READY]
```yaml
files: [internal/mount/ollama, cmd/komodo/main.go]
done_when:
  - go test ./internal/mount/ollama/...
context:
  - "machine <task> reads the brief, posts it to the chat endpoint of OLLAMA_BASE_URL, default http://localhost:11434, with the role's schema as the response format and the profile's model, writes .komodo/results/<task>.json, and stamps the ledger with the prompt and completion counts the response carries; net/http only, no other model, no MCP, no host in the path"
  - "read-only roles only: reviewer, summarizer, and any role whose tools hold no write or shell; a write role asked for ollama exits non-zero naming the fallback tier; Ollama down is one line with the URL and a non-zero exit, never a crash; tested against a fake Ollama on a local listener"
type: feat
```

#### [TSK-03.5.5] The hybrid and local profiles run on the Ollama mount [P: H] [READY]
```yaml
files: [internal/mount/claude, internal/mount/codex, internal/profile, komodo/roles/summarizer.md]
done_when:
  - go test ./internal/mount/... ./internal/profile/...
depends_on: [TSK-03.5.4, TSK-03.3.4]
context:
  - "hybrid: the light tier and the reviewer resolve to ollama and run through komodo machine, the builder stays on the host's standard tier, so a review never shares a vendor with the build; a session role on an ollama tier renders on the host as its standard-tier agent, since a host spawn cannot reach Ollama without spending a model; Ollama not answering degrades the tier to the host's light tier and the report says so once"
  - "the Codex mount's local profile sets oss_provider ollama and a model_providers.ollama base_url in config.toml, and every tier's model from the profile, so the builder runs locally where the host mounts Ollama natively"
type: feat
```

#### [TSK-03.5.6] `komodo detect`: the repo profile, cached by manifest hash [P: C] [READY]
```yaml
files: [internal/detect, cmd/komodo/main.go]
done_when:
  - go test ./internal/detect/...
context:
  - "reads the tree once and writes .komodo/profile.json: languages from extensions, cloud from markers such as cdk.json, template.yaml with a SAM transform, a Terraform provider block, cloudbuild.yaml, app.yaml, azure-pipelines.yml, data sources from a Prisma schema, SQL migrations, dbt_project.yml, compose services, CI from .github/workflows, the verify and compile commands from the discovery order"
  - "cached by a hash of the manifests it read; recomputed when the hash changes; zero tokens; a detection never fails the run, an unknown tree is an empty profile"
type: feat
```

#### [TSK-03.5.7] Facets: Komodo's setup skills and appendices keyed by detection [P: C] [READY]
```yaml
files: [komodo/facets, internal/facet]
done_when:
  - test -f komodo/facets/aws/facet.md
  - test -f komodo/facets/aws/skill/SKILL.md
  - test ! -f komodo/facets/aws/mcp.json
  - go test ./internal/facet/...
depends_on: [TSK-03.5.6]
context:
  - "komodo/facets/<name>/ holds skill/SKILL.md, facet.md with a builder and a reviewer appendix under their own headings, commands.json with verify and compile defaults, and detect.json with the markers that select it; shipped: aws, gcp, azure, postgres, github-actions; mcp.json is a reserved name a later pass will read, and V2 ships none"
  - "the skill holds Komodo's own setup for that platform and only what a model is not trained on: accounts, regions, naming, deploy paths, environments, conventions; never a vendor tutorial; a platform not yet set up, which is every cloud today, ships the skill as headings with what is decided and a list of what is unknown"
  - "the aws, gcp, and azure standards move into their facets' skills, trimmed to the same rule; a facet appendix appends to a role's body in the brief and never touches its schema or tools; selection is detection, then .komodo/facets, then a task's facets key"
type: feat
```

#### [TSK-03.5.8] The repo profile slot, facet appendices, and the task keys tier and facets [P: H] [READY]
```yaml
files: [internal/line/brief.go, internal/backlog, komodo/rules/backlog.md, komodo/roles/builder.md, komodo/roles/reviewer.md]
done_when:
  - go test ./internal/line/... ./internal/backlog/...
  - grep -q '{{repo_profile}}' komodo/roles/builder.md
depends_on: [TSK-03.5.7]
context:
  - "a ninth slot repo_profile, about 200 characters: languages, cloud, data, CI, verify; facet appendices land in the standards slot under their own cap"
  - "the grammar gains tier, one of light, standard, heavy, which overrides the role's tier for that task, and facets, a list added to detection; lint accepts both and nothing else new"
type: feat
```

#### [TSK-03.5.9] The project render comes from the profile, and intake runs it [P: H] [READY]
```yaml
files: [internal/install, internal/mount/claude, internal/mount/codex, internal/line/next.go, internal/doctor]
done_when:
  - go test ./internal/install/... ./internal/mount/... ./internal/doctor/...
depends_on: [TSK-03.5.8, TSK-03.5.2]
context:
  - "install --project writes the host's project config from the profile and the repo layer: the facet skills, the standards skills, the repo skills, the rules file; gitignored copies, rebuilt every time; next --start runs it so a run always has the right tools; no MCP server is rendered"
  - "doctor compares the cached profile to a fresh detection and the rendered project config to what the source renders now; a mismatch is one line and the fix is rerunning the render"
type: feat
```

### [TG-03.6] The gate and the exit test
```yaml
type: chore
version: 2.0.0
```
* **Why:** the gate is Go and runs on the desk before every commit and push, nothing runs on GitHub, every swap point is proven by a test, and the second host proves the mounts are the only host-specific code.
* **PR F:** `chore/v2-gate-exit` from `feat/v2-repo-layer`, cut by `/run TG-03.6` and opened by `close --group`. Validated by the gate as the pre-commit and pre-push hook on both developer machines, the retired-words grep, the swap tests, no workflow directory, the Codex numbers in the changelog, and a human read of the final README. Merges into `feat/v2-repo-layer`; then #103 merges into `main`.

#### [TSK-03.6.1] The gate is local, and nothing runs on GitHub [P: C] [READY]
```yaml
files: [internal/gate, internal/line/close.go, internal/line/ship.go, cmd/komodo/main.go]
done_when:
  - go run ./cmd/komodo gate
  - test ! -d .github/workflows
context:
  - "komodo gate runs go vet, go test, komodo doctor, komodo guard check, and the binary rebuild and manifest compare from TSK-03.2.1, in that order, stopping at the first failure with its output; every check is mechanical and no model is called"
  - "close <task> runs it before the task commit and close --group runs it before the push when the repo is this toolkit; gate --install writes .git/hooks/pre-commit and pre-push as sh scripts that pick the platform's binary under bin/ by uname and run the gate, so a human terminal gets the same check; no workflow directory, no CI, nothing that bills minutes"
type: chore
```

#### [TSK-03.6.2] README, names, and the templates describe what exists [P: H] [READY]
```yaml
files: [README.md, AGENTS.md, templates/project, komodo, cmd/komodo, internal]
done_when:
  - go run ./cmd/komodo doctor
  - "! grep -rEil 'bridge|pointer skill|adapter|orchestrator' komodo cmd internal README.md AGENTS.md"
depends_on: [TSK-03.6.1]
context:
  - "the README drops its planned status and the V1 coverage table, keeps the line, the stations, the devices, the mounts, hot swap, ad hoc, the guard, the binary, the gate, the repo layer, names, setup, usage, layout; templates/project carries AGENTS.md, BACKLOG.md, CHANGELOG.md, the docs/spec starters the grammar's context anchors point at, and the example .komodo/context file; CLAUDE.md.tmpl goes"
  - "every command, flag, file, directory, skill, role field, and config key uses the README's Names table and nothing else; a retired word, bridge, adapter, worker, orchestrator, harness, pointer skill, appears nowhere a model or a developer reads except CHANGELOG.md; the root AGENTS.md layout and commands match the tree"
type: docs
```

#### [TSK-03.6.3] Proof: the exit test under Codex [P: H] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "Proof: the exit test under Codex" CHANGELOG.md
depends_on: [TSK-03.6.2]
owner: human
context:
  - "komodo install --host codex with zero changes outside internal/mount, then run one task with codex exec through the launcher; record the same numbers as TSK-03.4.3 under a Proof: the exit test under Codex heading"
type: docs
```

#### [TSK-03.6.4] Changelog 2.0.0 [P: M] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "## 2.0.0" CHANGELOG.md
depends_on: [TSK-03.6.3, TSK-03.6.5]
context:
  - "one heading: the line, what was removed, what replaced it, both proofs, the swap proofs"
type: docs
```

#### [TSK-03.6.5] Every swap point is proven [P: C] [READY]
```yaml
files: [internal/line/swap_test.go, internal/profile, internal/facet, internal/repo]
done_when:
  - go test ./internal/line/... ./internal/profile/... ./internal/facet/... ./internal/repo/...
depends_on: [TSK-03.6.1]
context:
  - "one test per swap point, each with no code change and no restart: a profile row change moves a station to another machine and step names it; a skill body change under komodo/skills or .komodo/skills reaches the next brief and the next project render; a facet added by .komodo/facets or a task's facets key reaches the standards slot, the profile slot, and the render; a commands.json change replaces verify at QC"
  - "a role, a skill, and the binary hold no name of a model, a server, or a platform, so the test fails when any of them does; MCP is the fifth point and is deferred: the test asserts that a facet's mcp.json, when present, changes nothing in V2"
type: test
```

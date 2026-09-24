# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the line parses is in the `backlog` skill; `komodo lint` checks it from TG-03.2 on. The V1.x backlog was dropped whole on 2026-09-21 in favour of the V2 plan in `README.md`.

---

## [EPIC-03] 1.0, the assembly line
*Goal: one static binary is the conveyor and the devices, markdown is everything a model reads, one guard is the only hook, and a model is a machine mounted per host. Two model calls per task, build and review; everything between is deterministic. Claude Code with Ollama is the host today for both Komodo devs; Codex is the exit test. Requirements and design: `README.md`.*

* **Everything lands on PR #103 through a stack.** Each group is one PR based on the group before it and merges into that parent; #103 merges into `main` last and the tag `v1.0.0` is cut then. Branches, bases, and the validation per PR are the README's Pull requests section, and each group below names its own.
* **Sessions build TG-03.1 through TG-03.4.** The `run` skill runs TG-03.5 and TG-03.6, and TG-03.5 is the proof.
* **The repo starts clean.** The prototype is the tag `prototype-final`; the only prototype files here are the markdown TG-03.1 reshapes.
* **Nothing runs on GitHub.** `komodo gate` is the only precheck, mechanical, local, before every commit and push. No workflow directory exists.
* **No MCP in 1.0.** Machines, skills, and external dependencies are the swappable parts; a facet's `mcp.json` is reserved for a later pass and nothing reads it.

### [TG-03.1] The markdown
```yaml
type: refactor
version: 1.0.0
```
* **Why:** everything a model reads is one of three neutral formats. Standards become skills so their trigger is their own frontmatter. Briefs fold into roles so a role is the brief and its schema. The policy shrinks to four denials for a greenfield shop, and the rules give an agent unlimited freedom inside its worktree.
* **PR A:** `refactor/v2-markdown` from `docs/v2-plan`, opened by the session with `gh pr create`. Validated by every `done_when` here, the V1 linter from a scratch worktree of `prototype-final` at zero problems, and a human read of `komodo/AGENTS.md` and `komodo/policy.json`. Merges into `docs/v2-plan`.

#### [TSK-03.1.1] Standards become skills at the source [P: C] [DONE]
```yaml
files: [komodo/skills, komodo/standards]
done_when:
  - test -f komodo/skills/standards-go/SKILL.md
  - test ! -d komodo/standards
  - grep -q '^globs:' komodo/skills/standards-go/SKILL.md
context:
  - "each komodo/standards/<x>.md moves to komodo/skills/standards-<x>/SKILL.md with frontmatter name, description, and a globs list of the extensions and directories that trigger it; the body does not change"
  - "the extension map V1 kept in standards.py, see the tag prototype-final, moves into that frontmatter and nothing else holds it"
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
  - "the JSON schema each worker prompt carried in V1 briefs.py, see the tag prototype-final, moves beside the role as <role>.schema.json; the builder's body says to write its result to the path the brief names"
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
version: 1.0.0
```
* **Why:** the line is one static Go binary with no interpreter, shell, or symlink on a dev machine. Every station is a subcommand with a test, and every station stamps the ledger. V1's 1013 lines of Go hooks, at the tag `prototype-final` under `komodo/hooks/src`, are the seed of the module.
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
  - "go.mod at the repo root, module komodo, Go 1.22, no dependencies outside the standard library; the V1 hook sources at the tag prototype-final come in under internal with their tests"
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
version: 1.0.0
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

#### [TSK-03.3.5] Each mount reports a machine's usage after the fact [P: H] [DONE]
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
version: 1.0.0
```
* **Why:** the run skill is the list of stations and the two spawns, under 800 tokens. Ad hoc work enters at any station or stays off the line. The proof is one group driven end to end by the skill alone, recorded before anything is deleted.
* **PR D:** `feat/v2-skills-launcher` from `feat/v2-guard-mounts`, opened by the session. Validated by `komodo doctor` with the run skill under 800 tokens, the launcher's scrub test, `komodo run --dry-run TG-03.5`, and `/run` in a session printing the first step and stopping, and a session start with no permission-rule warning. TSK-03.4.3 stays open and is filled in by PR E. Merges into `feat/v2-guard-mounts`.

#### [TSK-03.4.1] The run, review, backlog, and respond skills [P: C] [DONE]
```yaml
files: [komodo/skills/run/SKILL.md, komodo/skills/review/SKILL.md, komodo/skills/backlog/SKILL.md, komodo/skills/respond/SKILL.md, cmd/komodo/main.go]
done_when:
  - go run ./cmd/komodo doctor --no-git
depends_on: [TSK-03.3.3]
context:
  - "run takes a group, a task, or nothing and is three lines: call komodo step, do what it says, repeat; the station order lives in the binary and never in a skill"
  - "review runs QC and the reviewer on the current diff; backlog writes tasks in the grammar with add and lint and is where the planner role works; respond lists unresolved threads and, as the responder role, changes code when the reviewer is right and replies when they are not"
  - "a skill names no host tool, path, flag, or vendor; it says spawn the builder role and the mount decides how"
  - "respond needs the threads the pr package already reads, so komodo threads exposes them as JSON"
type: feat
```

#### [TSK-03.4.2] The headless launcher: `komodo run` [P: H] [DONE]
```yaml
files: [internal/run, cmd/komodo/main.go, internal/mount/registry.go]
done_when:
  - go test ./internal/run/...
depends_on: [TSK-03.4.1]
context:
  - "scrubs push tokens, credential helpers, and SSH identities from the environment, picks the host from the profile, invokes the host's non-interactive mode with the run skill and the group or task id, kills it at group_budget_s, and exits with the host's code; gitops.worker_env is the prior art for the scrub"
type: feat
```

#### [TSK-03.4.3] Proof: the run skill drives a group end to end [P: C] [DONE]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "Proof: the run skill drives a group" CHANGELOG.md
depends_on: [TSK-03.4.1, TSK-03.4.2]
owner: human
context:
  - "run TG-03.5 under the run skill in a session, committing to this branch; record wall time, tokens in, tokens out, and turns per station from .komodo/line.jsonl, under a Proof: the run skill drives a group heading in the 2.0.0 changelog entry"
  - "the bar is absolute, not comparative: the group finishes inside the launcher's budget, with no human turn between stations and no station run out of the order komodo step gave"
  - "a station the human had to drive means the skill is wrong; fix the skill before TG-03.5 merges"
  - "there is no V1 baseline: V1 kept run state in the gitignored .komodo/runs and the V2 clean start deleted it, TG-02.4 is not a group at the tag prototype-final, and V1 squash-merged one commit per group so no per-group timing survives in history; never re-add the comparison"
type: docs
```

#### [TSK-03.4.4] The permissions layer denies on edit only [P: H] [DONE]
```yaml
files: [internal/mount/claude/claude.go, internal/mount/claude/claude_test.go, .claude/settings.json]
done_when:
  - go test ./internal/mount/...
  - test -z "$(grep -c 'Write(' .claude/settings.json | grep -v '^0$')"
depends_on: [TSK-03.3.2]
context:
  - "the host matches a file permission rule on Edit only; a Write(path) deny entry is inert and the host prints a warning for each one at session start"
  - "denyList emits one Edit entry per config path and no Write entry; the guard is unchanged and still the real refusal"
type: fix
```

### [TG-03.5] The repo layer and the local machines
```yaml
type: feat
version: 1.0.0
base: docs/v2-plan
```
* **Why:** one universal set of rules, one place a repo adds what only it knows, and a local machine on every host. Nothing here is required and nothing here widens what the guard denies.
* **PR E:** `feat/v2-repo-layer` from `feat/v2-skills-launcher`, cut by `/run TG-03.5` and opened by `close --group` with the report as the body. Validated by `komodo gate`, `komodo doctor` with profile drift, `komodo detect` printing Go and no cloud, the Ollama mount against the fake and one real local review, a facet swap reaching a brief, and the proof numbers for TSK-03.4.3 in the changelog. Merges into `feat/v2-skills-launcher`.

#### [TSK-03.5.1] Repo context injects by glob [P: H] [DONE]
```yaml
files: [internal/repo/context.go, internal/repo/context_test.go, internal/line/brief.go, templates/project, .gitignore]
done_when:
  - go test ./internal/repo/... ./internal/line/...
context:
  - "the gitignore entry .komodo/ is anchored to /.komodo/ so templates/project/.komodo can ship; unanchored it swallows the example at any depth"
  - ".komodo/context/*.md with a paths glob list in frontmatter fills the repo_context slot for any task whose files match, clipped; no globs means every task; a malformed file is skipped with one line in the report; templates/project gains one example"
type: feat
```

#### [TSK-03.5.2] Repo standards and repo skills, rendered by `install --project` [P: H] [DONE]
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

#### [TSK-03.5.3] Repo commands and additive policy [P: H] [DONE]
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

#### [TSK-03.5.4] `komodo machine`: the binary is the Ollama mount [P: C] [DONE]
```yaml
files: [internal/mount/ollama, cmd/komodo/main.go]
done_when:
  - go test ./internal/mount/ollama/...
context:
  - "machine <task> reads the brief, posts it to the chat endpoint of OLLAMA_BASE_URL, default http://localhost:11434, with the role's schema as the response format and the profile's model, writes .komodo/results/<task>.json, and stamps the ledger with the prompt and completion counts the response carries; net/http only, no other model, no MCP, no host in the path"
  - "read-only roles only: reviewer, summarizer, and any role whose tools hold no write or shell; a write role asked for ollama exits non-zero naming the fallback tier; Ollama down is one line with the URL and a non-zero exit, never a crash; tested against a fake Ollama on a local listener"
type: feat
```

#### [TSK-03.5.5] The hybrid and local profiles run on the Ollama mount [P: H] [DONE]
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

#### [TSK-03.5.6] `komodo detect`: the repo profile, cached by manifest hash [P: C] [DONE]
```yaml
files: [internal/detect, cmd/komodo/main.go]
done_when:
  - go test ./internal/detect/...
context:
  - "reads the tree once and writes .komodo/profile.json: languages from extensions, cloud from markers such as cdk.json, template.yaml with a SAM transform, a Terraform provider block, cloudbuild.yaml, app.yaml, azure-pipelines.yml, data sources from a Prisma schema, SQL migrations, dbt_project.yml, compose services, CI from .github/workflows, the verify and compile commands from the discovery order"
  - "cached by a hash of the manifests it read; recomputed when the hash changes; zero tokens; a detection never fails the run, an unknown tree is an empty profile"
type: feat
```

#### [TSK-03.5.7] Facets: Komodo's setup skills and appendices keyed by detection [P: C] [DONE]
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

#### [TSK-03.5.8] The repo profile slot, facet appendices, and the task keys tier and facets [P: H] [DONE]
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

#### [TSK-03.5.9] The project render comes from the profile, and intake runs it [P: H] [DONE]
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
version: 1.0.0
base: docs/v2-plan
```
* **Why:** the gate is Go and runs on the desk before every commit and push, nothing runs on GitHub, every swap point is proven by a test, and the second host proves the mounts are the only host-specific code.
* **PR F:** `chore/v2-gate-exit` from `feat/v2-repo-layer`, cut by `/run TG-03.6` and opened by `close --group`. Validated by the gate as the pre-commit and pre-push hook on both developer machines, the retired-words grep, the swap tests, no workflow directory, the Codex numbers in the changelog, and a human read of the final README. Merges into `feat/v2-repo-layer`; then #103 merges into `main`.

#### [TSK-03.6.1] The gate is local, and nothing runs on GitHub [P: C] [DONE]
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

#### [TSK-03.6.2] README, names, and the templates describe what exists [P: H] [DONE]
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

#### [TSK-03.6.3] Proof: the exit test under Codex [P: H] [REFINEMENT]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "Proof: the exit test under Codex" CHANGELOG.md
depends_on: [TSK-03.6.2]
owner: human
context:
  - "komodo install --host codex with zero changes outside internal/mount, then run one task through the launcher; record the same numbers as TSK-03.4.3 under a Proof: the exit test under Codex heading"
  - "parked: no OpenAI account yet; the Codex mount stays built and tested, and TSK-03.9.3 is the second beta proof instead"
type: docs
```

#### [TSK-03.6.4] Changelog 1.0.0 [P: M] [DONE]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "## 1.0.0" CHANGELOG.md
depends_on: [TSK-03.6.3, TSK-03.6.5]
context:
  - "one heading: the line, what was removed, what replaced it, both proofs, the swap proofs"
type: docs
```

#### [TSK-03.6.5] Every swap point is proven [P: C] [DONE]
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

#### [TSK-03.6.7] The prebuilt binaries stop conflicting on every stacked branch [P: M] [DONE]
```yaml
files: [.gitattributes, bin/MANIFEST.sha256]
done_when:
  - git check-attr binary bin/komodo-linux-amd64
depends_on: [TSK-03.6.1]
context:
  - "three committed binaries and a manifest are rebuilt by komodo gate --rebuild on every branch, so any two branches off one base always conflict in bin/; #112 and #114 both hit it"
  - "mark them binary with a merge strategy that takes the branch's own copy, or build them at release only and drop them from the tree"
type: chore
```

#### [TSK-03.6.6] Doctor's drift check counts a file the install would create [P: M] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
depends_on: [TSK-03.4.1]
context:
  - "checkDrift reports only update and remove, so a new skill or role that an installed mount has never rendered passes clean; TSK-03.4.1 added four skills and the gate stayed green with none of them mounted"
  - "a create against a host that is already rendered is drift and must say run komodo install"
type: fix
```

#### [TSK-03.6.8] The reviewer tier is set on both mounts and never dispatched [P: M] [DONE]
```yaml
files: [internal/line/next.go, internal/line/next_test.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.5.5]
context:
  - "machineFor dispatches on role.Tier and has no reviewer case, so mount.Tiers.Reviewer is populated by both hosts and never read; the shipped reviewer role declares tier heavy and resolves to Tiers.Heavy"
  - "either dispatch the reviewer role to Tiers.Reviewer or drop the field from the Tiers struct; a set-but-unread field is a silent wrong machine"
type: fix
```

#### [TSK-03.6.9] A declared key that nothing reads fails the gate [P: H] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
depends_on: [TSK-03.6.6]
context:
  - "four mechanisms shipped wired to nothing and were each found by hand: RepairText and komodo brief --failure, SplitFindings and FileFindings and Profile.SeverityFloor, commands.json before_review and after_publish, and a task's tier key; every one parsed, linted, documented and inert"
  - "doctor grows a check that every exported symbol the rules or the grammar promise has a caller outside its own tests, naming the promise and the symbol"
type: feat
```

#### [TSK-03.6.10] One resolver decides which group a station plans for [P: H] [DONE]
```yaml
files: [internal/line/next.go, internal/line/step.go, internal/line/next_test.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.6.9]
context:
  - "five defects in one run came from a station planning from BACKLOG.md instead of the open run: the wrong base, renumbered waves twice, close and ship taking the next ready group, and step abandoning an unshipped run"
  - "PlanForRun and pinWaves patched the callers one at a time; make it impossible instead, so a plan cannot be built without stating whether it is the open run or a fresh group"
type: refactor
```

#### [TSK-03.6.11] `komodo diff` truncates the diff it hands the reviewer [P: H] [DONE]
```yaml
files: [internal/line/diff.go, internal/line/diff_test.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.6.1]
context:
  - "the third TG-03.5 review reported the input truncated mid-file and read the worktree directly instead; a reviewer silently working from a cut diff is a review of something other than the change"
  - "either clip on a file boundary and say how many files were dropped, or page the diff; never end mid-hunk with no marker"
type: fix
```

#### [TSK-03.6.12] The manifest hash in the profile cache decides nothing [P: L] [DONE]
```yaml
files: [internal/detect/detect.go, internal/detect/detect_test.go]
done_when:
  - go test ./internal/detect/...
depends_on: [TSK-03.6.11]
context:
  - "Load now walks the tree on every call and returns the fresh profile, so hashManifests and sameManifests only decide whether the file is rewritten, never what a caller sees; the walk they exist to avoid always runs"
  - "drop the hash comparison and keep the equality guard, or restore a cache read that genuinely skips the walk"
type: refactor
```

### [TG-03.7] The sanity pass: safety, correctness, portability
```yaml
type: fix
version: 1.0.0
base: docs/v2-plan
```

#### [TSK-03.7.1] The binaries leave the tree, and the gate builds with a pinned toolchain [P: H] [DONE]
```yaml
files: [.gitattributes, .gitignore, bin, go.mod, internal/gate, internal/release, AGENTS.md, README.md]
done_when:
  - go test ./internal/gate/... ./internal/release/...
  - test ! -f bin/MANIFEST.sha256
depends_on: [TSK-03.7.6]
context:
  - "bin/** binary merge=binary is a no-op: the binary macro already expands to -merge, so the three binaries and the manifest conflict on every branch; it fired on nine merges. Drop bin/ from the tree, gitignore it, and have komodo release build the per-platform binaries as release assets"
  - "the gate stops comparing tracked binaries to a manifest; the toolkit builds its own local binary for its hook instead. Pin a toolchain line in go.mod and set GOTOOLCHAIN in the build environment so every developer builds the same bytes"
  - "the pre-commit hook script's *) fallback picks the Windows exe on Darwin-x86_64 and Linux-aarch64; match MINGW*|MSYS*|CYGWIN* for Windows and exit with a clear no-binary-for-this-platform message otherwise"
  - "update the AGENTS.md rule and the README sections that describe prebuilt binaries under bin/ with a manifest"
type: fix
```

#### [TSK-03.7.2] Every doctor check fires when it should and only then [P: H] [DONE]
```yaml
files: [internal/doctor]
done_when:
  - go test ./internal/doctor/... -shuffle=on
depends_on: [TSK-03.7.6]
context:
  - "checkPromises scans only komodo/rules/*.md bullets and counts callers by word match, so severity_floor, before_review and after_publish (named in BACKLOG.md) are out of scope and a struct field satisfies a method; scan the promise sources and resolve real call sites with go/ast"
  - "render drift flips with whether Ollama answers, because Render probes it live; render drift from the state recorded at install. checkDrift's render calls detect.Load, which rewrites the profile cache, so checkProfileDrift can never fire and doctor writes during an audit; render with a read-only profile"
  - "checkLeaks and the conflict-marker scan read only Go files; scan every tracked text file (komodo/**, templates/**, AGENTS.md), and match a conflict marker on line 1 too. komodo/policy.json and templates/project/AGENTS.md.tmpl name host paths today"
  - "a role whose frontmatter is missing or CRLF vanishes from LoadRoles and checkRoles reports nothing; list komodo/roles/*.md and report every file not loaded. Prune deletes every merged branch but base, so a stacked base deletes local main; never delete a critical ref"
  - "StandardBytes measures the whole file at 8192 while the brief clips the body at CapStandard 6000; measure the body against CapStandard. The always-on budget omits the skill and agent descriptions a host preloads every session; count them"
type: fix
```

#### [TSK-03.7.3] The MCP swap point is proven, or the changelog stops claiming it [P: M] [DONE]
```yaml
files: [internal/line/swap_test.go, CHANGELOG.md]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.7.12]
context:
  - "TSK-03.6.5 requires a test that a facet's mcp.json, when present, changes nothing in V2; swap_test.go has four swap tests and no mcp assertion, yet the 2.0.0 changelog says it does. Add the assertion; a changelog that overstates a proof is worse than one that omits it"
type: test
```

#### [TSK-03.7.4] The review station hands the reviewer a filled brief of the right diff [P: H] [DONE]
```yaml
files: [internal/line/diff.go, internal/line/diff_test.go, internal/line/step.go, komodo/roles/reviewer.md]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.7.11]
context:
  - "the reviewer spawn's brief is the command string komodo diff, but the reviewer role has read and search only and cannot run it, and the text names no result path or schema; fill reviewer.md from DiffFor's parts, append the result line and schema as BuildBrief does, write it to .komodo/briefs/<group>-review.md, and pass that path as the brief"
  - "DiffFor diffs against the local base while AddWorktree cut the group from origin/<base>; a stale local base puts another group's commits in the review. Diff against the ref AddWorktree resolved"
  - "repo standards (.komodo/standards) and each selected facet's reviewer appendix never reach the review input; add both"
  - "a C-quoted non-ASCII path fails the diff --git a/ header match, so its hunks fold into the previous file and the file vanishes; handle the quoted form and emit a marker for any changed file with no chunk"
type: fix
```

#### [TSK-03.7.5] Detection agrees with what QC runs, and selects the facets it names [P: M] [DONE]
```yaml
files: [internal/detect, internal/facet, komodo/facets]
done_when:
  - go test ./internal/detect/... ./internal/facet/...
depends_on: [TSK-03.7.6]
context:
  - "commands() keys on manifest base names anywhere in the tree and ignores the Makefile and verify scripts QC prefers, so a Makefile plus go.mod repo shows go test in the brief while QC runs make verify; own the verify discovery order here, root-only, so the line can call it"
  - "any Terraform provider sets cloud terraform and Prisma sets data prisma, and no facet matches either; map the provider (aws, google, azurerm, postgresql) to the facet marker"
  - "skipDir misses venv, env, target, dist, build and __pycache__, and doctor walks the tree up to three times per run; skip them"
  - "TestLoadRecomputesWhenAManifestIsAdded asserts nothing the added manifest changes; assert a field it does change. Facet.Commands and the five empty commands.json files are parsed and never read; remove them"
type: fix
```

#### [TSK-03.7.6] The toolkit ships inside the binary, so the line runs in any repo [P: C] [DONE]
```yaml
files: [komodo.go, internal/toolkit, internal/line, internal/mount, internal/guard, internal/facet, internal/doctor, cmd/komodo]
done_when:
  - go test ./...
context:
  - "roles, schemas, standards, skills, rules, policy and facets are all read from <target repo>/komodo, and nothing is embedded; in a fresh project install fails on komodo/AGENTS.md and next, step, brief and report fail on komodo/roles. V2 cannot run in any repo but its own"
  - "embed the komodo/ tree with go:embed from a root package (the module is named komodo, so komodo.go at the root is package komodo) and expose it through internal/toolkit as one fs.FS; when the target repo has its own komodo/ directory (the toolkit developing itself) prefer the files on disk, otherwise serve the embedded copy"
  - "route every loader through it: line roles, schemas and standards; mount skills, roles and rules; the guard's shipped policy; facets; doctor's reads of the toolkit tree. The repo root stays the target repo for everything that is the repo's own (BACKLOG.md, .komodo/, AGENTS.md)"
  - "prove it with a test that builds a repo holding only a BACKLOG.md and runs next, brief and step against it"
type: fix
```

#### [TSK-03.7.7] The guard sees through shell wrappers, chains and assignments [P: C] [DONE]
```yaml
files: [internal/guard]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.7.6]
context:
  - "only kept[0] names the command, so env git push origin main, (git push origin main), { git push; }, sudo, nice, timeout and xargs all pass; strip wrapper commands and leading ( or {, and inspect the argument of sh -c, bash -c and eval as a command of its own"
  - "segmentRe does not split on a single &, so true & git push origin main is one segment named true; split on it"
  - "assignRe drops every NAME=value token, so dd of=<host config> checks no path; strip assignments only before the command word, parse dd of=, and add sed -i, perl -i, find -delete and the >| redirect to the writers"
  - "an assignment prefix can undo the headless credential scrub (GIT_CONFIG_COUNT=0, GIT_CONFIG_PARAMETERS, GIT_SSH_COMMAND, GIT_ASKPASS); deny a command that overrides a scrubbed variable"
  - "add a denied table row for every bypass above, so the gate's guard check fails if any regresses"
type: fix
```

#### [TSK-03.7.8] The guard reads git the way git does [P: C] [DONE]
```yaml
files: [internal/guard]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.7.7]
context:
  - "every segment is judged against the branch at hook time, so git switch main && git merge --ff-only feat/x && git push pushes main; track the branch across segments and deny a switch or checkout onto a critical ref. git -C <other checkout> push uses the cwd branch; deny -C into another checkout"
  - "global options shift the subcommand: git -c alias.p=push p origin main and git --config-env pass; expand -c alias.*, deny -c remote.*.push and pushurl, and skip the value of every global option that takes one. git config remote.origin.push writes .git/config unchecked; treat git config as a write to it"
  - "git push --mirror, git push --all and a wildcard refspec such as refs/heads/*:refs/heads/* all reach main; deny them whenever a critical ref exists"
  - "add a denied table row for every form above"
type: fix
```

#### [TSK-03.7.9] The guard protects its own policy, every path spelling, and trailers in every form [P: C] [DONE]
```yaml
files: [internal/guard, internal/mount/registry.go, internal/mount/claude/guard.go, internal/mount/codex/guard.go, komodo/policy.json]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.7.8]
context:
  - "komodo/policy.json, .komodo/policy.json and each mount's hook settings file are writable, so one Write removes main's protection or the hook itself; protect all three. The shipped policy replaces DefaultPolicy and drops mount.ConfigPaths(); union them, and move the host paths out of policy.json. Critical refs from the machine overlay (~/.komodo/config.json) never reach the guard; merge them"
  - "config paths match case-sensitively, so a write to Bin/komodo-darwin-arm64 or .GIT/hooks overwrites the guard or a hook on a case-insensitive disk; fold case on darwin and windows. $HOME/..., ~user/... and cd out of the root escape the path checks; deny a write target that holds an unresolved variable or ~user, and a writer that follows a cd out of the root"
  - "false denies cost real time: 2>/dev/null and > /dev/null are refused as outside the worktree, cp's source is checked as if it were written, and the system temp directories (os.TempDir, /tmp, /private/tmp) are refused; allow the null device and temp directories, and check only the destination of cp, mv, ln and install"
  - "a co-author trailer passes after a ; or && inside -m, through -F msgfile, and through --trailer key=value; match the whole quote-aware message, read -F files, and accept = as a separator. The generated-by pattern refuses ordinary prose such as files generated by stringer; anchor every trailer pattern to a line start with a colon"
  - "the guard hard-codes one host's tool names (Bash, Edit, Write, MultiEdit, NotebookEdit) and hook output schema outside internal/mount; let each mount register its tool-name map and denial encoder. Add a table row for every bypass and every false deny above"
type: fix
```

#### [TSK-03.7.10] Step routes every state to the station that ends it [P: C] [DONE]
```yaml
files: [internal/line/step.go, internal/line/step_test.go]
done_when:
  - go test ./internal/line/... -shuffle=on
depends_on: [TSK-03.7.6]
context:
  - "a paused plan has nil waves and WaitUntil is written but never read, so step skips to review and ship marks unbuilt tasks DONE and pushes; return a done action carrying wait_until whenever the plan is paused"
  - "after a first failure step only ever emits brief or spawn, so a repair that writes its result is re-spawned forever; emit komodo close when the result is newer than the repair brief"
  - "one blocked task ends the whole run with done, so the draft-PR path is unreachable and BlockedBy has no caller; skip a blocked task and its BlockedBy dependents, and continue to review and a draft ship"
  - "step never passes --gate, so the gate never runs at task close in the toolkit; pass it. The blocking-findings message tells the user to run komodo close --group, which refuses on the same stale review; point it at komodo step. ReviewSkipLines is never read; skip review under it. Skills and Facets on every action are always empty; fill them or drop them"
  - "a session role on the hybrid profile never reaches the local machine because every session role is refused there; allow a read-only session role, which the reviewer is, when its machine is local"
type: fix
```

#### [TSK-03.7.11] Ship reads the truth, tells it, and hands the push to whoever holds the credentials [P: C] [DONE]
```yaml
files: [internal/line/ship.go, internal/line/ship_test.go, internal/line/step.go, cmd/komodo/main.go, CHANGELOG.md]
done_when:
  - go test ./internal/line/... -shuffle=on
depends_on: [TSK-03.7.10]
context:
  - "close writes task status to the root BACKLOG.md; ship reads the group worktree's copy, where every task is still READY, so a BLOCKED task ships as DONE and the PR is never a draft. Read statuses from the root that close writes, and apply the flips to the worktree's BACKLOG.md before the commit"
  - "runShip renders the PR body from an empty ShipResult before ShipGroup classifies blocked tasks, so every task is ticked; render the body inside ship after Blocked is known. A failed after_publish command exits 0; exit non-zero"
  - "AppendChangelog matches '## <version>' followed by a newline but writes the heading with a date suffix, so it never finds its own heading and every ship adds a duplicate; CHANGELOG.md has three '## 2.0.0' headings today, one below 0.1.0. Match the heading with or without a date, and merge the three into one"
  - "FileFindings runs again on every ship retry and duplicates the filed tasks; file only once, after a successful push. Ship's own commit makes the review stale, so a failed push triggers a full re-review; exclude ship's commit from the staleness check. Refuse to ship a plan whose waves were blanked by a pause"
  - "a headless run cannot ship: the scrub that stops an agent pushing also stops ship's push. When the environment is scrubbed, ship commits, writes the branch, base, title, body, labels and draft flag to .komodo/ship.json, stamps the ship outcome handoff, and does not push; the launcher finishes it (TSK-03.7.23)"
type: fix
```

#### [TSK-03.7.12] Briefs and plans honour their caps, their mode, their needle, and one run at a time [P: H] [DONE]
```yaml
files: [internal/line/brief.go, internal/line/next.go, internal/line/dag.go, internal/line/clip.go, internal/line/verify.go, internal/line/worktree.go, cmd/komodo/main.go]
done_when:
  - go test ./internal/line/... -shuffle=on
depends_on: [TSK-03.7.4, TSK-03.7.5, TSK-03.7.19]
context:
  - "repo standards (.komodo/standards) never reach a brief; merge repo.LoadStandards into the standards slot. The README slot totals are not enforced: context has no 24k total, files exceeds 24k past 12 files, repo_context has no total; clip each slot's joined text. profile.Caps is parsed and never passed to the brief; pass it"
  - "single mode puts every task in one wave and briefs each on its own base, so overlapping tasks conflict at QC and MaxParallel is skipped; brief single mode as one builder. A task needle widens to the whole group; narrow the plan to that task. A dependency outside the planned set (human-owned, REFINEMENT, BLOCKED) counts as met; count only DONE"
  - "line.MatchGlob is a verbatim copy of the repo layer's broken matcher; use the shared internal/glob matcher. line.VerifyCommand and detect disagree on discovery; call detect's root-only order. IsText's UTF-8 check is dead because read always equals len(buffer)"
  - "two komodo run processes drove one group at once with nothing to stop them; next --start takes an exclusive lock under .komodo holding the pid and run id, a second run exits naming the holder, and a lock whose pid is gone is reclaimed"
type: fix
```

#### [TSK-03.7.13] Close trusts nothing the result says about itself, and a repairable failure keeps the loop alive [P: H] [DONE]
```yaml
files: [internal/line/close.go, internal/line/wave.go, internal/line/close_test.go, internal/line/step.go, cmd/komodo/main.go, cmd/komodo/main_test.go, README.md, komodo/skills/run/SKILL.md]
done_when:
  - go test ./internal/line/... -shuffle=on
depends_on: [TSK-03.7.12]
context:
  - "checkResult validates against the result's own role key, so a builder can pick a lighter schema or traverse to a schema it wrote in its worktree; take the role from the step or brief, never from the result"
  - "with no task worktree, close runs git add -A in the root checkout and commits the user's unrelated changes onto whatever branch is checked out; refuse to commit unless the cwd is the task's own worktree on its task branch"
  - "QC and review failures have no repair path, and RepairBrief and FailureText have test callers only; wire them into the failure slot, or delete them and correct the README"
  - "close exits 1 on a repairable failure and the run skill stops on any non-zero exit, so even the first repair needs a human restart; a failure that leaves the task IN_PROGRESS for repair is not an error, so exit 0 and say so, and keep non-zero for real errors"
  - "TSK-03.7.12 briefs a single-mode task into the group worktree and cuts no task branch; finish it: step spawns a single-mode builder in the group worktree, and CloseWave merges no task branch for a single-mode group, since close already committed each task on the group branch"
type: fix
```

#### [TSK-03.7.14] The backlog writer round-trips everything it writes [P: H] [DONE]
```yaml
files: [internal/backlog]
done_when:
  - go test ./internal/backlog/...
context:
  - "quote writes a newline raw and escapes a double quote that scalar never unescapes, so reviewer text filed by FileFindings can break a YAML block, close it early or inject a key, and a quoted done_when command comes back changed; collapse or reject newlines and single-quote any value holding a double quote"
  - "AppendTask never validates priority or status, so --priority high writes a heading the parser cannot match; the task vanishes, lint passes, and the next add reuses its id. Validate both, and make Parse report any #### [TSK- line its heading pattern rejects"
type: fix
```

#### [TSK-03.7.15] Every CLI command reads every flag, and machine-read output is compact [P: M] [DONE]
```yaml
files: [cmd/komodo/main.go, cmd/komodo/main_test.go, internal/pr/pr.go, internal/pr/pr_test.go]
done_when:
  - go test ./cmd/... ./internal/pr/...
depends_on: [TSK-03.7.11]
context:
  - "komodo add parses with flag.Parse, so flags after the group and title are swallowed into the title and the task is written with no files; it was missed when every other command moved to splitPositional. comments check reads its flags as paths and passes vacuously; strip check first and fail on a path that does not exist"
  - "komodo machine --role reviewer looks up the heavy tier and never Tiers.Reviewer, and its error says it falls back when nothing does; resolve the reviewer through Tiers.Reviewer and say what actually happens"
  - "step and next --json print indented JSON the run skill reads every loop, and next --json carries every role's description and the whole profile; print compact JSON and drop what no station reads"
  - "the respond loop can reply to a review thread but never resolve it, so a human must click resolve on every one; add pr.Client.Resolve(threadID) over the resolveReviewThread mutation and komodo threads --resolve <id>"
type: fix
```

#### [TSK-03.7.16] Release and tag tell the truth about versions [P: M] [DONE]
```yaml
files: [internal/release, cmd/komodo/main.go]
done_when:
  - go test ./internal/release/... ./cmd/...
depends_on: [TSK-03.7.15]
context:
  - "release.Check never counts repeats or order, so three '## 2.0.0' headings pass; report a version under more than one heading and any heading out of order. release check passes every group's version, shipped or not, so any pending group fails it; check only groups whose tasks are all DONE"
  - "komodo tag cuts and pushes a tag on whatever branch is checked out, and reads only local tags, so a failed push leaves a local tag that makes every later run say everything is tagged; refuse to tag off the default branch, and compare against the remote's tags"
type: fix
```

#### [TSK-03.7.17] Metrics report what was measured and nothing else [P: H] [DONE]
```yaml
files: [internal/ledger]
done_when:
  - go test ./internal/ledger/...
context:
  - "tokens by model counts only entries with a Model, and only komodo machine sets one, so every build's tokens (stamped under Host) never appear in metrics; key by Model, fall back to Host with a visible label"
  - "the repair-rate denominator counts every task in the ledger, including add stamps and review pseudo-tasks; count only tasks that reached close. The aggregate test is built from entry shapes no station writes; build it from the real stamps. median and sortStrings hand-roll what sort already provides"
type: fix
```

#### [TSK-03.7.18] respond reads real review threads and stops when none are left [P: H] [DONE]
```yaml
files: [internal/pr, komodo/skills/respond/SKILL.md]
done_when:
  - go test ./internal/pr/...
context:
  - "Threads returns top-level conversation comments with no path, line or resolved flag, never inline review threads, and Reply posts another top-level comment that the next Threads call returns, so the respond loop never terminates; fetch reviewThreads (isResolved, path, line, comments) through gh api graphql and reply on the thread itself"
type: fix
```

#### [TSK-03.7.19] One glob matcher, and the repo layer reads what people write [P: H] [DONE]
```yaml
files: [internal/glob, internal/repo]
done_when:
  - go test ./internal/glob/... ./internal/repo/...
context:
  - "a multi-segment glob such as internal/repo/** never matches, and src/*.ts and docs/*.md fall to exact match, so repo context is read and never injected; the fixture uses internal/repo/** and never tests a match. Build one matcher in internal/glob (dir/** as a path prefix, path.Match otherwise) for every caller"
  - "a YAML block list under paths: leaves an empty value, so the context matches every task instead of its own; parse - item lines. A CRLF file is skipped as having no frontmatter, and a CRLF new standard or skill is misread as an append and dropped; normalise line endings first"
type: fix
```

#### [TSK-03.7.20] The comment lint reads every language's doc forms [P: M] [DONE]
```yaml
files: [internal/comments]
done_when:
  - go test ./internal/comments/...
context:
  - "a Python function with a wrapped signature is reported UNDOCUMENTED despite its docstring, because the check reads the first line after def; skip to the line ending in a colon. A doc comment above an attribute or annotation (#[must_use], @GetMapping) is not seen; skip those lines walking up"
  - "the first-person rule matches a lone i, so I/O and i.e. fail as narrative; require i to be followed by whitespace or an apostrophe. The 140-character cap is enforced and reported as a word count; report it as its own rule or drop it to match the stated twenty-word rule"
type: fix
```

#### [TSK-03.7.21] The first mount loads its rules, anchors its hook, and deletes nothing it did not write [P: C] [DONE]
```yaml
files: [internal/mount/claude, internal/mount/registry.go]
done_when:
  - go test ./internal/mount/...
depends_on: [TSK-03.7.9]
context:
  - "the universal rules render to .claude/komodo/AGENTS.md and nothing imports them; CLAUDE.md is @AGENTS.md only, so no session ever reads V2's rules. Import the rendered rules from the file the host always loads, and seed CLAUDE.md rather than overwrite it, adding the import line to an existing one"
  - "the hook command is the relative bin/komodo-darwin-arm64 guard, so after a cd the host cannot find it, exits 127 and runs the tool call unguarded; render an absolute path to the installed binary"
  - "install RemoveAll's .claude/commands and .claude/hooks, where users keep their own commands and hook scripts; remove only files the old render wrote"
  - "every standards skill renders whatever the repo's languages, so each session lists about 36 unused skill descriptions; render only the standards the detected profile selects. Machine.Local is exported with one caller while the provider literal survives elsewhere; unexport or inline it"
type: fix
```

#### [TSK-03.7.22] The second mount names real models, reaches the local machine, and can write [P: H] [DONE]
```yaml
files: [internal/mount/codex]
done_when:
  - go test ./internal/mount/...
depends_on: [TSK-03.7.21]
context:
  - "the tier map writes models named small, standard and large, which are not model ids, so every agent and tier fails; map each tier to a real model id"
  - "local agents get the local model name but no model_provider, and oss_provider applies only under a flag headless never passes, so requests go to the default provider; write model_provider into the agent or config file"
  - "the headless command passes no sandbox and the config sets none, so exec runs read-only and neither the line nor a builder can write; pass a workspace-write sandbox. The universal rules must reach every session of this host too, not a pointer to them"
type: fix
```

#### [TSK-03.7.23] A headless run finishes its group and leaves nothing running [P: H] [DONE]
```yaml
files: [internal/run]
done_when:
  - go test ./internal/run/...
depends_on: [TSK-03.7.11]
context:
  - "after the host exits, when .komodo/ship.json holds a handoff, the launcher pushes the branch and opens the pull request with the original, unscrubbed environment, then stamps ship done; the agent never holds a credential"
  - "the scrub is incomplete: GIT_SSH_COMMAND lacks -F /dev/null so an IdentityFile in ~/.ssh/config still authenticates, and the exact-name drop list misses GITHUB_PAT, HOMEBREW_GITHUB_API_TOKEN and GIT_CONFIG_PARAMETERS; add -F /dev/null and drop by a token-name pattern. Mark the scrubbed environment so ship can tell it is headless"
  - "the budget kill reaches only the host process, so its shells and tests keep writing the worktree; start the host in its own process group and kill the group. Nothing writes the second host's usage events file; tee the headless JSON stdout to it"
type: fix
```

#### [TSK-03.7.24] The local machine is one mount, sized to the brief it reads [P: M] [DONE]
```yaml
files: [internal/mount/ollama, internal/profile]
done_when:
  - go test ./internal/mount/... ./internal/profile/...
depends_on: [TSK-03.7.22]
context:
  - "the local endpoint's variable, port and model name live in internal/profile and the doctor cannot see them because the local mount registers no vendors; move them into internal/mount/ollama and register it"
  - "the request sends no context size, so a brief longer than the local default is silently truncated and the reviewer reads part of the diff; send a context size from the brief's length and fail when the prompt count shows truncation"
type: fix
```

#### [TSK-03.7.25] Seven standards fit the cap they are clipped at [P: M] [DONE]
```yaml
files: [komodo/skills]
done_when:
  - go run ./cmd/komodo doctor
context:
  - "doctor now measures a standard's body against CapStandard (6000 bytes), the length the brief clips it at; seven exceed it (standards-csharp 7873, java 7932, kotlin 7639, swift 7302, ui-desktop 6745, ui-mobile 7850, zig 7508), so every brief that selects one silently loses its tail"
  - "tighten each to at most 6000 bytes of body without dropping a rule: merge duplicates, cut restated examples, shorten wording; a rule that survives only as a fragment is lost, not kept"
type: chore
```

#### [TSK-03.7.26] The last host name outside the mounts, a field nobody reads, and an honest always-on count [P: M] [DONE]
```yaml
files: [templates/project, internal/profile, internal/doctor]
done_when:
  - go test ./internal/doctor/... ./internal/profile/...
context:
  - "templates/project/AGENTS.md.tmpl names a host's home rules file outside internal/mount; say it without naming the host"
  - "Profile.Remote is set to origin and read by nothing; ship pushes to origin directly. Remove it, or have ship read it; a set-but-unread field is a silent wrong answer waiting to happen"
  - "doctor's always-on total sums every shipped skill's description, but a host preloads only the skills the mount rendered; count the rendered set, so the number tracks what a session actually loads"
type: fix
```

#### [TSK-03.7.27] Always-on context fits its cap with a host installed [P: H] [DONE]
```yaml
files: [AGENTS.md, komodo/AGENTS.md, komodo/rules, komodo/roles]
done_when:
  - go run ./cmd/komodo doctor
  - go test ./...
context:
  - "doctor's always-on budget is 1500 tokens; with the first host installed it measures about 1770: the repo's AGENTS.md about 506, the rendered universal rules about 827, session role descriptions about 257, rendered skill descriptions about 180. Bring the total to at most 1400 so a repo's own skills have headroom"
  - "the repo's AGENTS.md says everything lands on PR #103, which is stale, and its layout table repeats README.md; keep only what an agent would otherwise guess"
  - "tighten the universal rules and the session role descriptions without dropping a rule: merge duplicates, cut restated examples, shorten wording. A rule that survives only as a fragment is lost, not kept; list every rule before and after and show none is missing"
type: chore
```

#### [TSK-03.7.28] cmd/komodo/main.go:364 next --start never takes the run lock, and AcquireLock can race [P: L] [DONE]
```yaml
files:
  - cmd/komodo/main.go
done_when:
  - test -f cmd/komodo/main.go
type: fix
context:
  - "runNext --start calls only line.CheckLock and writes no lock. So two interactive sessions both start and drive the same group, which the task said next --start must prevent. AcquireLock checks and then writes with os.WriteFile, not O_EXCL. Two `komodo run` processes started together both see no lock and both proceed. Call AcquireLock from next --start, and create the lock file with O_CREATE|O_EXCL, retrying once after reclaiming a dead pid."
```

#### [TSK-03.7.29] internal/run/run.go:214 A headless ship never files minor findings or runs after_publish [P: L] [DONE]
```yaml
files:
  - internal/run/run.go
done_when:
  - test -f internal/run/run.go
type: fix
context:
  - "When the environment is scrubbed, ShipGroup returns right after writing the handoff, before FileFindings and AfterPublishCommand. finishShip only pushes, creates the PR, and labels it. Every below-floor review finding from a headless run is silently dropped, and after_publish never runs. Have finishShip run FileFindings and the after_publish command after a successful push, with the same one-time guarantee the interactive path has."
```

#### [TSK-03.7.30] internal/guard/guard.go:740 A trailer passes through `git commit -F -` from a heredoc [P: L] [DONE]
```yaml
files:
  - internal/guard/guard.go
done_when:
  - test -f internal/guard/guard.go
type: fix
context:
  - "readMessageFile returns an empty string for '-'. stripHeredocs also removes the body, because git is not a shell. So `git commit -F - <<'EOF'` with a Co-Authored-By line in the body passes the trailer check the task extended to -F. When -F is '-' or /dev/stdin, check the stripped heredoc body (or deny when it cannot be read) instead of returning an empty message."
```

#### [TSK-03.7.31] internal/guard/guard.go:682 Switch handling denies a file restore and misses a forced reset of main [P: L] [DONE]
```yaml
files:
  - internal/guard/guard.go
done_when:
  - test -f internal/guard/guard.go
type: fix
context:
  - "switchTarget returns the first non-flag argument. So `git checkout main -- README.md`, which restores a file and does not switch, is denied as a switch onto a critical ref. The task explicitly wanted to remove false denies like this. Conversely, `git checkout -B main feat/x` and `git switch -C main feat/x` set create=true and skip the critical check. They silently reset local main, which update-ref on main is denied for. Stop at `--` treating what precedes it as a pathspec source when paths follow, and apply the critical-ref check to -B, -C, and --force-create targets."
```

#### [TSK-03.7.32] internal/toolkit/toolkit.go:20 Any top-level komodo/ directory in a target repo replaces the whole embedded toolkit [P: L] [DONE]
```yaml
files:
  - internal/toolkit/toolkit.go
done_when:
  - test -f internal/toolkit/toolkit.go
type: fix
context:
  - "FS switches to os.DirFS(root/komodo) whenever that directory exists. A target repo with an unrelated komodo/ package, or one holding only a .komodo-like leftover, loses every embedded role, schema, standard, and facet. Brief, step, and close then fail on missing roles. A root session can also plant komodo/roles/builder.schema.json to weaken close's result validation. Prefer disk only when root/komodo holds the toolkit's marker file (for example roles/builder.md and policy.json), or fall back per file to the embedded tree."
```

#### [TSK-03.7.33] internal/guard/guard.go:653 --git-dir=.git or --work-tree=. on the same checkout is denied for any write [P: L] [DONE]
```yaml
files:
  - internal/guard/guard.go
done_when:
  - test -f internal/guard/guard.go
type: fix
context:
  - "-C . is exempt, but --git-dir and --work-tree set elsewhere for any value, so git --git-dir=.git commit -m x is refused as another checkout. Exempt a --git-dir or --work-tree that resolves to the current worktree root's .git or root, as -C . is exempt."
```







### [TG-03.8] The line plans what it is handed
```yaml
type: fix
version: 1.0.0
base: main
```

#### [TSK-03.8.1] brief refuses an ad hoc task that collides with an unmerged branch [P: H] [DONE]
```yaml
files: [internal/line/brief.go, internal/line/brief_test.go]
done_when:
  - go test ./internal/line/...
context:
  - "komodo brief cut an ad hoc task on internal/doctor while a closed, unmerged task on internal/doctor sat in the open run; their doctor.go diverged by about 430 lines and the merge had to be resolved by hand"
  - "refuse, naming the other task, when the task's dirs overlap any task branch in the open run that has not merged into the group branch"
type: fix
```

#### [TSK-03.8.2] A task added to the open group mid-run joins a later wave [P: M] [DONE]
```yaml
files: [internal/line/next.go, internal/line/step.go]
done_when:
  - go test ./internal/line/...
context:
  - "pinWaves restores the run's recorded waves wholesale, so a task appended to the group after intake is in no wave and step never reaches it; it runs only by hand through komodo brief"
  - "keep the pinned waves for the tasks they hold, and plan any task they miss into waves after the last pinned one"
type: fix
```

#### [TSK-03.8.3] max_parallel caps a wave inside the planner, not after it [P: M] [DONE]
```yaml
files: [internal/line/dag.go, internal/line/next.go]
done_when:
  - go test ./internal/line/...
context:
  - "splitByParallel cuts finished waves, so a five-task wave with max_parallel 4 leaves one task alone in its own wave and pushes every dependent of the first four a wave later; TG-03.7 ran TSK-03.7.7 alone in wave 4 while TSK-03.7.11, ready and disjoint, waited for wave 5"
  - "give Waves the cap as wave capacity, so a task carried over for capacity shares its next wave with the tasks that became ready"
type: fix
```

#### [TSK-03.8.4] The comment lint accepts a C# attribute between the doc comment and the declaration [P: L] [DONE]
```yaml
files: [internal/comments]
done_when:
  - go test ./internal/comments/...
context:
  - "a public C# member with a /// summary above an [Obsolete] line is flagged undocumented; skip bracketed attribute lines when looking up from the declaration, as the Rust and Java attributes already are"
type: fix
```

#### [TSK-03.8.5] One spec shape: architecture, system design, and an optional PRD [P: M] [DONE]
```yaml
files: [komodo/skills/standards-specs, templates/project/docs/spec, komodo/roles/planner.md, komodo/rules/backlog.md, templates/project/BACKLOG.md.tmpl]
done_when:
  - go run ./cmd/komodo doctor
context:
  - "split the SDD into docs/spec/architecture.md (the stable shape: purpose, components, boundaries, data flow, decisions) and docs/spec/system-design.md (the detail: data model, interfaces, non-functional requirements, operations, recovery); docs/spec/prd.md stays optional and planner-facing; retire SDD.md"
  - "every heading lives in exactly one file, so a task cites one place and the two never drift; the skill names which file owns each section"
  - "architecture.md stays small enough for the planner and the reviewer to read whole; a builder gets system-design.md sections through task context"
  - "standards-specs lists eight SDD sections and REQ-nn IDs while the templates carry V1 section numbering with gaps (§0, §1, §3, §5) and PRD-1 IDs; the skill and the templates name the same sections and one ID scheme"
  - "use plain headings with no § numbers, so a task cites docs/spec/system-design.md#interfaces and a renumbering never breaks a citation; update the context examples in rules/backlog.md and BACKLOG.md.tmpl"
  - "standards-specs calls the SDD required, which contradicts no repo config being required; say a repo may keep its design in README.md or in these files, and a task cites whichever holds it"
  - "planner.md carries a {{spec}} slot nothing fills; drop it and tell the planner to read the spec files by path"
type: fix
```

#### [TSK-03.8.6] A context anchor that names no section fails instead of sending the whole file [P: H] [DONE]
```yaml
files: [internal/line/brief.go, internal/line/brief_test.go, internal/backlog]
done_when:
  - go test ./internal/line/... ./internal/backlog/...
context:
  - "contextSlot falls back to the whole file, clipped, when Section finds no heading for the anchor, so a mistyped anchor silently spends thousands of tokens on the wrong text"
  - "the brief names the missing section instead of inlining the file, and komodo lint reports a context anchor whose file exists but holds no matching heading"
type: fix
```

#### [TSK-03.8.7] The guard refuses a force push [P: M] [DONE]
```yaml
files: [internal/guard, komodo/policy.json]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
context:
  - "the rules forbid force-pushing and rewriting published history, but the guard allows git push --force, --force-with-lease and a +refspec to any branch that is not critical; deny them, or make it a policy switch, and add table rows"
type: fix
```

#### [TSK-03.8.8] A mount names its own events file, and the base is resolved in one place [P: L] [DONE]
```yaml
files: [internal/mount/registry.go, internal/mount/codex, internal/run, internal/line/diff.go, internal/line/worktree.go]
done_when:
  - go test ./internal/mount/... ./internal/run/... ./internal/line/...
context:
  - "internal/run tees headless stdout to .komodo/<host>/<task>.jsonl because that happens to match codex.EventsPath; add Host.EventsPath so the mount owns the path and the launcher asks for it"
  - "diff.go's resolveBase copies AddWorktree's origin/<base>-first choice; export one resolver from worktree.go and call it from both, so the review can never diff a different base than the group was cut from"
type: chore
```

#### [TSK-03.8.10] The local machine is reached through the registry, so nothing outside the mounts names it [P: M] [DONE]
```yaml
files: [internal/mount/registry.go, internal/mount/ollama, cmd/komodo/main.go, internal/line/step.go, internal/line/next.go, internal/doctor/doctor.go, internal/profile/profile.go]
done_when:
  - go test ./...
  - go run ./cmd/komodo doctor
context:
  - "registering ollama as a vendor makes the doctor report 22 places outside internal/mount that name it: komodo machine in main.go, the machine routing in step.go and next.go, doctor's pinOllamaDown and drift, and profile's selection; route each through a registry entry for the local machine, then register the name as a vendor"
type: refactor
```

#### [TSK-03.8.11] A running local server does not take every tier [P: H] [DONE]
```yaml
files: [internal/mount/claude/limits.go, internal/mount/codex/limits.go, internal/line/step.go]
done_when:
  - go test ./internal/mount/... ./internal/line/...
context:
  - "with the local server up, each mount's Tiers maps every tier to the local model, so step routed the TG-03.7 group review (a 124 KB brief over 6749 changed lines) to a 3B model; route to the local machine only the tiers and roles the profile names, and never a brief larger than the local model's window"
type: fix
```

#### [TSK-03.8.12] The guard's limits are stated, and the hard boundaries sit outside it [P: H] [DONE]
```yaml
files: [README.md, komodo/policy.json, internal/guard, internal/run]
done_when:
  - go test ./internal/guard/... ./internal/run/...
  - go run ./cmd/komodo guard check
context:
  - "two TG-03.7 review rounds found 13 guard bypasses in a row (substitutions, env -i, shell keywords, include.path, and more); a denylist over bash cannot be complete. Say in README.md that the guard catches a cooperative model's mistakes and is not a sandbox"
  - "put the hard boundaries where a shell cannot reach: branch protection on the remote for every critical ref (checked by doctor through the forge's API), and the headless credential scrub as the only push path; consider running headless hosts under the host's own sandbox with the network allowed only to the model"
  - "the second review's low finding: --git-dir=.git and --work-tree=. on the same checkout are refused like another checkout; exempt them as -C . is"
type: fix
```

#### [TSK-03.8.13] Every station command runs under a wall clock in its own process group [P: C] [DONE]
```yaml
files: [internal/proc, internal/line/verify.go, internal/line/close.go, internal/run/run.go, internal/backlog]
done_when:
  - go test ./internal/proc/... ./internal/line/... ./internal/run/...
context:
  - "done_when runs under the task's timeout key, default 10 minutes; the gates, the toolkit gate, and after_publish get their own; a hung child is killed with its group, never waited on"
type: fix
```

#### [TSK-03.8.14] A repair brief reads the task's own worktree [P: H] [DONE]
```yaml
files: [cmd/komodo/main.go, internal/line/close.go]
done_when:
  - go test ./internal/line/... ./cmd/...
context:
  - "komodo brief read the files slot from the group worktree while the failed attempt's edits sat in the task worktree; TaskWorktree is the one resolver both brief and close use"
type: fix
```

#### [TSK-03.8.15] Tokens are the spawned agent's, never the session's [P: H] [DONE]
```yaml
files: [internal/mount/claude/usage.go, internal/ledger/ledger.go, internal/line/close.go]
done_when:
  - go test ./internal/mount/... ./internal/ledger/... ./internal/line/...
context:
  - "Usage summed the driving session's transcript inside the brief-to-close window, so four parallel builds each carried the whole session; it now sums the spawned transcripts that were handed the task's brief, splits cache reads out, and the build stamp names tier, provider, and model"
type: fix
```

#### [TSK-03.8.16] The headless launcher can drive a session and find the binary [P: C] [DONE]
```yaml
files: [internal/mount/claude/claude.go, internal/run/run.go, komodo/skills/run/SKILL.md]
done_when:
  - go test ./internal/mount/... ./internal/run/...
context:
  - "a headless session cannot answer a permission prompt, so the host is started with prompts bypassed and the guard hook as the wall, on the standard tier's model; the launcher puts bin/ first on PATH; the run skill says what komodo resolves to and that machine's model half is the spawn's model"
type: fix
```

#### [TSK-03.8.17] The overlay names the local model, the local window, and a model per tier [P: H] [DONE]
```yaml
files: [internal/mount/registry.go, internal/mount/ollama/ollama.go, internal/mount/claude/limits.go, internal/mount/codex/limits.go, README.md]
done_when:
  - go test ./internal/mount/...
context:
  - "OLLAMA_MODEL, then local_model, then the first model the server lists; local_window caps what komodo machine sends; models renames a tier for the host so a proof can run on a cheaper machine"
type: feat
```

#### [TSK-03.8.18] komodo report reads the run's own group, and doctor --remote audits the rulesets [P: M] [DONE]
```yaml
files: [cmd/komodo/main.go, internal/line/next.go, internal/doctor/doctor.go]
done_when:
  - go test ./internal/doctor/... ./internal/line/... ./cmd/...
context:
  - "report planned the next ready group once the run shipped; doctor --remote reads the forge's branch rulesets through gh and names any active one that reaches past the default branch"
type: fix
```

#### [TSK-03.8.19] Proof payload: the run skill drives one task end to end [P: L] [DONE]
```yaml
files:
  - README.md
done_when:
  - test -f README.md
type: docs
```


### [TG-03.9] A local machine carries a station
```yaml
type: feat
version: 1.0.0
base: main
```
* **Why:** the second host needs an account nobody holds yet. A local model carrying the review proves a machine swaps in with no code change, and one overlay key points every machine at one endpoint, local now and a static address later.

#### [TSK-03.9.1] One overlay key points every machine at the local endpoint [P: H] [DONE]
```yaml
files: [internal/mount/registry.go, internal/mount/ollama/ollama.go, internal/mount/ollama/ollama_test.go, cmd/komodo/main.go]
done_when:
  - go test ./internal/mount/...
context:
  - "local_url in ~/.komodo/config.json sets the endpoint under OLLAMA_BASE_URL and over the localhost default; a URL with no port dials its scheme's port"
  - "the machine station stamps its seconds, so the proof has a wall time"
type: feat
```

#### [TSK-03.9.2] Proof payload: a local machine reviews one group [P: L] [DONE]
```yaml
files:
  - README.md
done_when:
  - test -f README.md
type: docs
```

#### [TSK-03.9.3] Proof: a local machine carries a station [P: C] [DONE]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "Proof: a local machine carries a station" CHANGELOG.md
depends_on: [TSK-03.9.2]
owner: human
context:
  - "local_reviewer true in ~/.komodo/config.json, then komodo run TG-03.9; the review station runs as komodo machine on the local model"
  - "record the machine row's model, seconds, tokens in, and tokens out from .komodo/line.jsonl under a Proof: a local machine carries a station heading in 1.0.0-beta.1"
type: docs
```

### [TG-03.10] The gate lints what a commit carries
```yaml
type: fix
version: 1.0.0
base: main
```
* **Why:** the gate's comment lint reads only tracked files, so a new file passes the gate by hand and then fails the pre-commit hook once it is staged. Found on PR #154.

#### [TSK-03.10.1] The comment lint reads untracked files git does not ignore [P: H] [DONE]
```yaml
files: [cmd/komodo/comments.go, cmd/komodo/cli_test.go]
done_when:
  - go test ./cmd/komodo/...
  - go vet ./cmd/komodo/...
context:
  - "trackedFiles lists git ls-files only; list the cached and the untracked files that the exclude rules do not ignore, so the gate and the hook read the same set"
  - "a test writes an untracked file with a restating doc comment and proves komodo comments check fails on it, and that an ignored file is never read"
type: fix
```

### [TG-03.11] A pipe through a filter still feeds a shell
```yaml
type: fix
version: 1.0.0
base: main
```
* **Why:** the guard follows stdin into a shell from the command just before it, but not across a longer pipeline. `echo 'git push origin main' | tr a a | sh` passes today.

#### [TSK-03.11.1] Piped input reaches a shell across the whole pipeline [P: C] [DONE]
```yaml
files: [internal/guard/shell.go, internal/guard/table.go]
done_when:
  - go test ./internal/guard/...
  - go run ./cmd/komodo guard check
context:
  - "pipedInput reads only commands[index-1]; walk back through every command that pipes into the next and gather each one's heredocs, here-strings, and echoed words"
  - "add denied rows for a filter between echo and sh, and for a heredoc through two filters into bash; add an allowed row for a long pipeline that never reaches a shell"
type: fix
```

### [TG-03.12] The local mount's own paths are tested
```yaml
type: test
version: 1.0.0
base: main
```
* **Why:** the local mount sits at 59.8% statement coverage, the lowest in the repo, and it is the path a local review takes.

#### [TSK-03.12.1] The local mount reaches 80 percent coverage against a fake server [P: M] [DONE]
```yaml
files: [internal/mount/ollama/ollama_test.go]
done_when:
  - go test ./internal/mount/ollama/...
  - go test -cover ./internal/mount/ollama | grep -Eq 'coverage: (8[0-9]|9[0-9]|100)\.'
context:
  - "cover ModelName's order of environment, overlay, server list, and default; Fits and the window override; dialAddress with and without a port; Up against a closed and an open listener"
  - "test files only, standard library httptest; never reach a real local server"
type: test
```

### [TG-03.13] Prune settles every merged run, not only the last
```yaml
type: fix
version: 1.0.0
base: main
```
* **Why:** `settleShippedRun` reads only the current run state, so a run's worktrees strand once the next group starts. TG-03.10's clean, merged worktrees survived `komodo doctor --prune`.

#### [TSK-03.13.1] Prune removes every clean state worktree whose branch origin holds [P: H] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
  - go vet ./internal/doctor/...
context:
  - "settleShippedRun gates its worktree sweep on the current run's branch; sweep every clean .komodo/wt worktree whose branch is an ancestor of origin's base, whatever the run state holds"
  - "keep the BACKLOG.md flip restore tied to the current run; never remove a dirty worktree or one whose branch origin lacks"
  - "a test ships two runs in sequence and proves one prune removes both runs' worktrees and branches"
type: fix
```

### [TG-03.14] The doctor names a worktree the line did not cut
```yaml
type: fix
version: 1.0.0
base: main
```
* **Why:** a spawn that cut its own worktree stranded a diff outside `.komodo/wt` on TG-03.11, and no check reported it.

#### [TSK-03.14.1] Doctor reports a linked worktree outside the state directory [P: M] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go]
done_when:
  - go test ./internal/doctor/...
  - go vet ./internal/doctor/...
context:
  - "checkGit lists git worktree list --porcelain; report each linked worktree whose path is outside .komodo/wt as a problem naming its path and branch"
  - "name no host: the check reads paths only; prune never removes such a worktree, it only reports it"
  - "a test adds a worktree outside .komodo/wt and proves doctor reports it, and that the main checkout and state worktrees are never reported"
  - "settleShippedRun sweeps clean merged worktrees even while a run is open, and a fresh worktree is clean and merged; skip the sweep while line.RunIsOpen, with a test"
type: fix
```

### [TG-03.15] The guard reads a safety mode
```yaml
type: feat
version: 1.0.0
base: main
```
* **Why:** the guard refuses `git switch main` and so blocks pulling the merged base. A mode lets the user pick: safe watches a switch onto a critical ref, default allows switching and pulling there, unsafe also allows a commit or push there.

#### [TSK-03.15.1] Policy carries a mode that scopes the critical-ref rules [P: H] [DONE]
```yaml
files: [internal/guard/policy.go, internal/guard/git.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
context:
  - "Policy gains Mode: safe, default, or unsafe; empty or unknown reads as default; only the machine overlay may loosen it, a repo policy may only tighten it"
  - "safe keeps today's denial of a switch or checkout onto a critical ref; default and unsafe allow it and git pull there, and the tracked branch still follows the switch"
  - "safe and default deny a commit, push, or merge on a critical ref; unsafe allows those; every mode still denies force, history rewrite, deleting or moving a critical ref, a trailer, and a config write"
  - "table Case gains a Mode field, empty meaning default; add rows for each mode's switch, pull, commit, and push on main, and a chained git switch main && git commit under default"
type: feat
```

### [TG-03.17] A spawn never cuts its own worktree
```yaml
type: fix
version: 1.0.0
base: docs/queue-spawn-guard
```
* **Why:** the driving session passed an isolation option to its builder spawn on TG-03.11 and TG-03.15, rule or no rule. The builder worked in a second worktree and the driver ported its diff by hand. A markdown rule cannot hold it; the guard can.

#### [TSK-03.17.1] The guard refuses a spawn that carries an isolation option [P: H] [DONE]
```yaml
files: [internal/mount/registry.go, internal/mount/claude/guard.go, internal/mount/claude/claude_test.go, internal/guard/guard.go, internal/guard/hook.go, internal/guard/table.go]
done_when:
  - go test ./internal/guard/... ./internal/mount/...
  - go vet ./internal/guard/... ./internal/mount/...
  - go run ./cmd/komodo guard check
  - go run ./cmd/komodo doctor
context:
  - "GuardTools gains SpawnTools, the host's agent-spawn tool names, and IsolationField, the input key that asks for a separate worktree; the claude mount fills both, and hookMatcher adds the spawn tools"
  - "Check denies a spawn tool call whose isolation field is set and not empty, finding: a spawn never cuts its own worktree; the line already cut it. hostGuard recognizes the spawn tools so the host's denial payload is used"
  - "guard.go and hook.go read spawn tool and field names from GuardTools only; table.go names the tool the way its Bash and Write rows already do"
  - "table rows: a spawn with isolation set is denied, a spawn without it is allowed, and a force push to main in unsafe mode is still denied"
type: fix
```

### [TG-03.16] A stray worktree is a note, not a failure
```yaml
type: fix
version: 1.0.0
base: fix/a-spawn-never-cuts-its-own-worktree
```
* **Why:** TG-03.14's check fails the doctor on any linked worktree outside `.komodo/wt`, and the gate runs the doctor. A user's own `git worktree add ../feature` would block every commit.

#### [TSK-03.16.1] Doctor prints a stray worktree as a note and still exits zero [P: H] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/doctor_test.go, cmd/komodo/host.go]
done_when:
  - go test ./internal/doctor/... ./cmd/komodo/...
  - go vet ./internal/doctor/... ./cmd/komodo/...
context:
  - "move checkWorktrees out of checkGit into an exported StrayWorktrees(root) []string, one line per worktree naming its path and branch, beside HostLeftovers"
  - "the doctor command prints each as a note line after the host leftovers; a stray worktree never adds a problem or a non-zero exit"
  - "tests: a worktree outside .komodo/wt yields one note and zero problems; the main checkout and state worktrees yield none"
type: fix
```

### [TG-03.18] Claude carries every tier until local is opted in
```yaml
type: fix
version: 1.0.0
base: docs/queue-claude-default
```
* **Why:** the light tier moves to Ollama whenever the server answers, and a small local model on a 24 GB machine is too weak to carry a station. For 1.0 Claude carries every tier; Ollama stays a swap the developer opts into.

#### [TSK-03.18.1] Local tiers need the overlay's local switch as well as a live server [P: H] [DONE]
```yaml
files: [internal/mount/registry.go, internal/profile/profile.go, internal/profile/profile_test.go, README.md]
done_when:
  - go test ./internal/profile/... ./internal/mount/...
  - go vet ./internal/profile/... ./internal/mount/...
  - go run ./cmd/komodo doctor
context:
  - "Overlay gains Local bool, json local; Select passes local only when the overlay sets it and the local machine answers, so a running server alone changes nothing"
  - "local_reviewer still needs local; without local, Why never mentions the local machine and no degrade note prints"
  - "tests: Ollama up without the switch keeps every tier on the host; with the switch the hybrid profile returns"
  - "README: selection line, the profile table's hybrid and local rows, and the overlay paragraph say local tiers are opt-in with \"local\": true"
type: fix
```

### [TG-03.19] The local switch never reports a probe it skipped
```yaml
type: fix
version: 1.0.0
base: docs/queue-review-fixes
```
* **Why:** the Claude review of TG-03.18 found three medium issues: a false "did not answer" note when the switch is off, a test that clobbers the registered local machine, and a test too weak to catch the note.

#### [TSK-03.19.1] Profile selection names the switch, and its tests restore shared state [P: H] [DONE]
```yaml
files: [internal/profile/profile.go, internal/profile/profile_test.go]
done_when:
  - go test -count=3 -shuffle=on ./internal/profile/...
  - go vet ./internal/profile/...
context:
  - "SelectWith prints the did-not-answer note only when the overlay switch is on and the probe failed; with the switch off and OLLAMA_BASE_URL set, Why says local tiers are off until the overlay sets local"
  - "the test that calls RegisterLocal restores the previous local machine with t.Cleanup, so every other test sees the one ollama's init registered"
  - "the switch-off test sets the local endpoint env and asserts Why carries no did-not-answer note, beside Name != hybrid"
type: fix
```

### [TG-03.21] A wave builds at once, and the hook checks the branch it commits
```yaml
type: feat
version: 1.1.0
base: main
```
* **Why:** TG-03.20's run built wave 1's three independent tasks one after another, about 12 minutes where 4 would do, because `step` returns one spawn per call. The same run's pre-commit hook in the group worktree ran the main checkout's `bin/`, so `guard check` judged main's 260-row table, not the branch's 315.

#### [TSK-03.21.1] Step returns every ready spawn in a wave, and the run skill launches them together [P: H] [READY]
```yaml
files: [internal/line/step.go, internal/line/snapshot.go, internal/line/step_test.go, komodo/skills/run/SKILL.md]
done_when:
  - go test ./internal/line/...
  - go vet ./internal/line/...
  - go run ./cmd/komodo doctor
depends_on: [TSK-03.21.3]
context:
  - "TSK-03.21.3 makes Next(snapshot) the pure decision; the wave-wide spawn is decided there, from the snapshot, never by a second read of the disk"
  - "Action gains Spawns []Action, json spawns, omitempty; when a parallel-mode wave holds two or more tasks that each have a current brief and no result, Step returns action spawn with every one in Spawns, each carrying its own brief, worktree, task, and machine as the single spawn does today"
  - "a task with no brief yet still returns its own run komodo brief action first, so every brief in the wave is written before the wave spawns; single-mode groups and a lone ready task keep today's single spawn with Spawns empty"
  - "the run skill: when spawns is present, spawn every entry in the same turn and wait for all of them, then loop; the one-action-per-turn rule reads as one step per turn"
  - "tests: a three-task parallel wave with all briefs written returns three spawns; a single-mode group returns one; a wave with one result already on disk returns only the other two"
type: feat
```

#### [TSK-03.21.2] The pre-commit hook runs the gate from the checkout it commits [P: H] [READY]
```yaml
files: [internal/gate/gate.go, internal/gate/gate_test.go]
done_when:
  - go test ./internal/gate/...
  - go vet ./internal/gate/...
context:
  - "hookScript finds the shared git dir's checkout for bin/; in a worktree that binary was built from another branch, so its compiled guard table and comment rules are not the ones being committed"
  - "when the committing checkout, git rev-parse --show-toplevel, holds cmd/komodo/main.go, the hook runs go run ./cmd/komodo gate from that toplevel, with --fuzz 10s on pre-push as today; any other repo keeps the built binary"
  - "tests: the rendered script carries the toplevel branch and the go run line; a repo without cmd/komodo keeps exec of the built binary"
type: fix
```

#### [TSK-03.21.3] Step reads one snapshot and decides with a pure function [P: H] [READY]
```yaml
files: [internal/line/step.go, internal/line/snapshot.go, internal/line/snapshot_test.go]
done_when:
  - go test ./internal/line/...
  - go vet ./internal/line/...
  - git diff --quiet HEAD -- internal/line/step_test.go
  - go run ./cmd/komodo guard check
context:
  - "Step today infers the run's state from seven kinds of files as it walks: briefs, results, attempts, wt, run.json, ship.json, run.lock; waveMerged, reviewed, staleReview, shipped, staleBrief, repairResultReady, and paused each read the disk mid-decision"
  - "snapshot.go adds Snapshot, loaded once by LoadSnapshot(root, needle): the plan, the parsed group, per task HasResult, attempt count, repairResultReady, staleBrief, and Open, per wave merged, reviewed, reviewSkippable, the blocking findings, the ship handoff, shipped, and the paused action"
  - "Next(Snapshot) Action is pure: no os, no git, no ledger; Step becomes LoadSnapshot, then Next, then the side effects stampReview and actionForTier that Step already runs today, in the same order"
  - "a refactor: every existing step test passes unchanged, which the done_when proves; snapshot_test.go adds a table over hand-built snapshots and FuzzNext, asserting Next never panics, returns run, spawn, or done, and never spawns a task whose snapshot already holds a result"
type: refactor
tier: heavy
```

### [TG-03.20] The guard holds its own denials, and the docs match the line
```yaml
type: fix
version: 1.0.1
base: docs/queue-scorecard-fixes
```
* **Why:** a blind review on 2026-09-24 found `git branch -f main`, `git remote set-url`, and a push whose target is only known at run time all pass the guard. README still describes the landed stacked-PR plan, and `doctor.go` is 916 lines. Five tasks across three directories, so the run is also the first multi-wave headless proof.

#### [TSK-03.20.1] The guard refuses moving or renaming a critical ref with git branch [P: H] [DONE]
```yaml
files: [internal/guard/git.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
context:
  - "case branch in gitFindings returns early unless deleting; it must also judge -f and --force, and -m, -M, --move, -c, -C, --copy"
  - "with -f or --force, the first positional names the ref being moved; deny when it is critical, finding: a critical ref is never moved by hand"
  - "with a move or copy flag, deny when any positional is critical, since both renaming main away and renaming onto main move it; same finding"
  - "table rows: branch -f main HEAD denied, branch --force master x denied, branch -M main old denied, branch -m feat/x main denied; branch -f feat/y main allowed, branch feat/y main allowed, branch -m feat/x feat/z allowed"
type: fix
```

#### [TSK-03.20.2] The guard refuses a git remote call that writes the config [P: H] [DONE]
```yaml
files: [internal/guard/git.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.20.1]
context:
  - "git remote add, set-url, rename, remove, rm, set-head, set-branches, and prune write .git/config and can redirect the next push; add case remote to gitFindings denying them with the finding the config case already uses"
  - "bare git remote, remote -v, remote show, and remote get-url only read; allow them, and add remote to readOnlyGit for those forms so git -C elsewhere may list remotes"
  - "table rows: remote set-url origin https://evil.example/x denied, remote add evil https://evil.example/x denied, remote rename origin old denied, set-url --push denied; remote -v allowed, remote get-url origin allowed, bare remote allowed"
type: fix
```

#### [TSK-03.20.3] The guard refuses a push whose target is only known when it runs [P: M] [DONE]
```yaml
files: [internal/guard/git.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.20.2]
context:
  - "under xargs -I{} the guard sees git push origin {}; with a command substitution it sees the unexpanded $(...) or backtick text; either could be a critical ref"
  - "in case push, when the policy protects any ref and a target still holds {}, $(, a backtick, or an unexpanded variable per unresolvedVarRe, deny with: git push <spec>: the target is only known when it runs; name the branch"
  - "a variable the guard already resolved from an earlier assignment is not unexpanded; BR=main; git push origin $BR must stay denied as a critical push, and BR=feat/x; git push origin $BR must stay allowed"
  - "table rows: echo main | xargs -I{} git push origin {} denied, git push origin \"$(git rev-parse --abbrev-ref HEAD)\" denied, git push origin `git branch --show-current` denied; git push origin feat/x allowed, BR=feat/x; git push origin $BR allowed"
type: fix
```

#### [TSK-03.20.4] README describes the line that exists, not the stack that built it [P: M] [DONE]
```yaml
files: [README.md]
done_when:
  - "! grep -q 'docs/v2-plan' README.md"
  - "! grep -q 'v2.0.0' README.md"
  - go run ./cmd/komodo doctor
context:
  - "the Pull requests section describes PR #103 and stacked group PRs A through F; that stack landed and 1.0.0 is cut"
  - "replace the section with a short Pull requests section: each group ships as one PR from its own <type>/<slug> branch, cut from the group's base; close --group opens it with the report as the body; merging is the human's button; nothing runs on GitHub"
  - "keep one sentence of history: 1.0 was built through PR #103 and six stacked group PRs, now merged; keep the ruleset paragraph and reword it to say the ruleset must cover main only"
  - "the opening paragraph's sentence about PR #103 moves to that history sentence; change nothing else in README"
type: docs
```

#### [TSK-03.20.5] Doctor splits into files by concern, doctor.go under 500 lines [P: L] [DONE]
```yaml
files: [internal/doctor/doctor.go, internal/doctor/promises.go, internal/doctor/prune.go, internal/doctor/render.go]
done_when:
  - go test ./internal/doctor/...
  - go vet ./internal/doctor/...
  - test "$(wc -l < internal/doctor/doctor.go)" -lt 500
  - go run ./cmd/komodo doctor
  - go run ./cmd/komodo comments check
context:
  - "a move, not a rewrite: no function changes its body, name, or signature, and no test changes"
  - "promises.go takes checkPromises, promises, fieldPromises, pascal, indexSymbols and their types; prune.go takes Prune, settleShippedRun, stateWorktrees, restoreFlips and their types; render.go takes renderInstalled, renderedSkillTokens, checkBudgets, checkDrift, checkProfileDrift, freezeProfile, pinLocalDown, pinLocalUp"
  - "doctor.go keeps Run, the other checks, and the small shared helpers"
type: refactor
```

### [TG-03.22] The guard holds the forge, the gate, and every interpreter
```yaml
type: fix
version: 1.0.1
base: main
```
* **Why:** a probe on 2026-09-24 against main's guard passed `gh api -X DELETE .../branches/main/protection`, `gh api -X PUT .../pulls/12/merge`, `git commit --no-verify`, `git -c core.hooksPath=/dev/null commit`, `python3 -c` and `node -e` bodies that push main, and a script written then run in one line. ADR 0004 names the forge ruleset as the hard boundary, and a session could delete it. The safety modes gate only critical-ref rules, so every one passed in every mode.

#### [TSK-03.22.1] The guard refuses a gh call that writes to the forge [P: C] [DONE]
```yaml
files: [internal/guard/gh.go, internal/guard/shell.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
context:
  - "gh.go takes the gh case out of scanner.command in shell.go, which today refuses only gh pr merge, and returns ghFindings(kept, policy)"
  - "gh api writes when -X or --method names anything but GET or HEAD, or when -f, -F, --field, --raw-field, or --input appear without -X GET; deny those in every mode with: gh api <METHOD> <endpoint>: a forge write goes through the line, never an agent"
  - "two writes stay allowed, since the line and the respond skill need them: POST to repos/<o>/<r>/pulls, and POST to repos/<o>/<r>/issues/<n>/comments or repos/<o>/<r>/pulls/<n>/comments or .../comments/<id>/replies; an endpoint naming protection, rulesets, merge, git/refs, hooks, keys, secrets, or collaborators is denied whatever the method"
  - "gh api graphql: deny when the query holds mutation, unless every mutation named is addComment, addPullRequestReviewComment, addPullRequestReviewThreadReply, resolveReviewThread, or createPullRequest; gh ruleset with a write verb, gh repo edit, gh repo delete, gh repo rename, gh secret set or delete, and gh api with --hostname plus any write are denied in every mode"
  - "table rows: api -X DELETE repos/o/r/branches/main/protection denied, api -X PUT repos/o/r/pulls/12/merge denied, api --method PATCH repos/o/r/rulesets/1 denied, api repos/o/r/git/refs/heads/main -f sha=x denied, graphql with mutation mergePullRequest denied, repo edit --default-branch x denied; api repos/o/r/pulls allowed, api -X POST repos/o/r/pulls -f title=x allowed, api repos/o/r/issues/3/comments -f body=x allowed, graphql query without mutation allowed, pr view 12 allowed"
type: fix
```

#### [TSK-03.22.2] The guard refuses a commit or push that skips the gate [P: C] [DONE]
```yaml
files: [internal/guard/git.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.22.1]
context:
  - "komodo gate is the only precheck, and --no-verify on commit, merge, push, rebase, am, or cherry-pick skips it; deny it in safe and default modes with: git <sub> --no-verify skips the gate; fix what it reports instead"
  - "commit's -n is --no-verify, alone or inside a short cluster such as -an; push -n is --dry-run and merge -n is --no-stat, so only commit reads n that way"
  - "-c core.hooksPath=<x> and --config-env core.hooksPath=<x> point git at other hooks; deny them in every mode, as a config write, beside the credential and remotePushConfigRe checks at the top of gitFindings"
  - "table rows: commit --no-verify -m x denied, commit -anm x denied, push --no-verify origin feat/x denied, -c core.hooksPath=/dev/null commit -m x denied, commit --no-verify in unsafe mode allowed, push -n origin feat/x allowed, merge -n feat/y allowed"
type: fix
```

#### [TSK-03.22.3] The guard reads what an interpreter's inline code runs [P: H] [DONE]
```yaml
files: [internal/guard/interp.go, internal/guard/shell.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.22.2]
context:
  - "interp.go names the interpreters: python, python3, node, ruby, perl, php, deno, bun, osascript, and their inline flags -c, -e, -E, --eval, -r, -p where each takes code; perl's -i path check in paths.go stays as it is"
  - "an inline body, or a script operand that exists in the worktree, is refused when its text holds the word git with one of push, commit, merge, rebase, branch, update-ref, remote, config, or tag, or the word gh with api, pr, repo, ruleset, or secret; finding: an interpreter running git or gh hides its command; run it in the shell"
  - "the check is textual on purpose: a list form such as [\"git\",\"push\",\"origin\",\"main\"] must be caught, so match words inside quotes and brackets, not shell tokens"
  - "python -m pytest, node script.js whose file does not mention git, and python -c 'print(1)' stay allowed; the scan never runs an interpreter"
  - "table rows: python3 -c with subprocess.run([\"git\",\"push\",\"origin\",\"main\"]) denied, node -e with execSync(\"git push origin main\") denied, ruby -e with system(\"gh api -X DELETE x\") denied; python3 -c 'print(1)' allowed, node -e 'console.log(2)' allowed, python -m pytest allowed"
type: fix
```

#### [TSK-03.22.4] A script written and run in one command is read before it runs [P: H] [DONE]
```yaml
files: [internal/guard/shell.go, internal/guard/lexer.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.22.3]
context:
  - "scanner.sourced reads a script from disk; echo 'git push origin main' > x.sh; sh x.sh passes because x.sh does not exist when the guard runs"
  - "the scanner records, per command line, each file a redirect or tee writes and the text it knows went there: echo and printf words, a heredoc body, or unknown; sourced, shell, the interpreter scan from TSK-03.22.3, and a command whose first word is a path to such a file all read the recorded text first"
  - "a recorded file with unknown text, such as curl ... > x.sh; sh x.sh, is refused: a script written and run in one command is not visible to the guard; write it, then run it in a second call"
  - "a command whose first word is ./name or a path to an existing file with a sh, bash, or zsh shebang is scanned like sh name"
  - "table rows: echo 'git push origin main' > x.sh; sh x.sh denied, cat > x.sh <<'EOF' then git push origin main then EOF; bash x.sh denied, printf 'git push origin main' > x.sh && ./x.sh denied, curl -s u > x.sh; sh x.sh denied; echo 'ls' > x.sh; sh x.sh allowed"
type: fix
```

#### [TSK-03.22.5] The guard refuses a push to a URL instead of a remote [P: H] [DONE]
```yaml
files: [internal/guard/git.go, internal/guard/table.go, internal/guard/guard_test.go]
done_when:
  - go test ./internal/guard/...
  - go vet ./internal/guard/...
  - go run ./cmd/komodo guard check
depends_on: [TSK-03.22.4]
context:
  - "TG-03.23 makes a line worktree unable to push through origin; git push <url> <ref> goes around the remote's pushurl, so the guard closes that path"
  - "in case push, the first positional is a URL when it holds :// or is scp form user@host:path, or is a path to a bare repo outside the worktree; deny in safe and default modes with: git push to a URL skips the remote the line configured; push to origin"
  - "table rows: push https://github.com/o/r.git feat/x denied, push git@github.com:o/r.git feat/x denied, push ../elsewhere.git feat/x denied; push origin feat/x allowed, push https://github.com/o/r.git feat/x in unsafe mode allowed"
type: fix
```

### [TG-03.23] A builder's worktree cannot push
```yaml
type: fix
version: 1.0.1
base: main
```
* **Why:** the guard is a denylist over bash and will never be complete. The headless scrub removes push credentials, but an in-session builder still has them. A worktree with no working push URL turns every missed shell trick into a failed push, so the guard stops being the last check.

#### [TSK-03.23.1] Every line worktree refuses a push, and ship pushes through an explicit URL [P: C] [READY]
```yaml
files: [internal/line/worktree.go, internal/line/worktree_test.go, internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/line/...
  - go vet ./internal/line/...
  - go run ./cmd/komodo doctor
context:
  - "AddWorktree, called by next.go for the group worktree and brief.go for each task worktree, sets extensions.worktreeConfig true on the repo once, then git -C <worktree> config --worktree remote.origin.pushurl refused://the-line-pushes"
  - "if the repo's common config holds core.bare or core.worktree, worktreeConfig would misread them; skip the pushurl there and print one note naming the reason, never fail the cut"
  - "ShipGroup's push in ship.go:156 reads the real URL from the main checkout with git -C <root> remote get-url --push origin, then runs git -c remote.origin.pushurl=<url> push -u origin <branch> from the group worktree; the launcher's gitPush in run.go already pushes from the root and needs no change"
  - "the main checkout's own config never gains a pushurl; git -C <root> config --get remote.origin.pushurl is unchanged by a cut"
  - "tests, against a bare remote in a temp dir: a push from a task worktree fails naming refused, a push from the root still works, ShipGroup from the group worktree pushes the branch"
type: fix
```

### [TG-03.24] The local reviewer earns its seat
```yaml
type: feat
version: 1.1.0
base: main
```
* **Why:** the local 3B reviewer returned 0 findings on most runs and missed a real bug in #161. A review station that always approves is a stage, not QC. A seeded-bug corpus gives each local model a recall number, and the line keeps review on the host until that number clears a bar.

#### [TSK-03.24.1] komodo recall scores a reviewer against seeded bugs [P: H] [DONE]
```yaml
files: [internal/recall/recall.go, internal/recall/recall_test.go, internal/recall/testdata, cmd/komodo/main.go, cmd/komodo/machine.go]
done_when:
  - go test ./internal/recall/... ./cmd/komodo/...
  - go vet ./internal/recall/... ./cmd/komodo/...
  - test "$(ls internal/recall/testdata/*.diff | wc -l)" -ge 15
  - go run ./cmd/komodo doctor
context:
  - "testdata holds at least 12 bug cases and 3 clean cases, each a unified diff plus a .json naming file, line, and class, or clean: true; bug classes: off-by-one, nil dereference, dropped error, inverted condition, wrong comparison operator, path traversal, shell injection, map race, defer in a loop, a guard branch returning early, a test that asserts nothing, and a prune that skips an open run as #161 did"
  - "recall builds each case's reviewer brief from komodo/roles/reviewer.md the way the line fills it for a diff, posts it with ollama.Post and the reviewer schema, and counts a catch when a finding names the case's file within 5 lines at medium or above"
  - "komodo recall [--model m] prints caught over cases, recall, and false findings on the clean cases, then writes ~/.komodo/recall.json keyed by model with recall, cases, false_positives, and at; it never runs in the gate, since the gate calls no model"
  - "tests drive a fake Ollama server, as the ollama mount's own tests do: a server that returns each case's expected finding scores 1.0, one that returns nothing scores 0"
type: feat
tier: heavy
```

#### [TSK-03.24.2] A local reviewer takes review only above its recall bar [P: H] [DONE]
```yaml
files: [internal/mount/registry.go, internal/mount/mount_test.go, internal/mount/claude/limits.go, internal/mount/claude/claude_test.go]
done_when:
  - go test ./internal/mount/...
  - go vet ./internal/mount/...
  - go run ./cmd/komodo doctor
depends_on: [TSK-03.24.1]
context:
  - "registry.go gains ReviewerRecall(model) (recall float64, cases int, ok bool) reading ~/.komodo/recall.json beside OverlayPath, and Overlay gains local_reviewer_recall, default 0.6, which an overlay may only raise"
  - "claude/limits.go routes the reviewer to ollama when LoadOverlay().LocalReviewer is set and the model's recall is at or above the bar over at least 10 cases; otherwise the reviewer stays on the heavy tier"
  - "the profile's why says which: the local reviewer's recall is 0.42 over 15 cases, under 0.6, so review stays on opus; or no recall on record for <model>; run komodo recall"
  - "tests: no recall file keeps review remote, recall 0.8 over 15 cases moves it local, recall 0.8 over 5 cases keeps it remote, an overlay bar of 0.9 with recall 0.8 keeps it remote"
type: feat
```

### [TG-03.25] Live status lives in the run, and the backlog changes once at ship
```yaml
type: refactor
version: 1.1.0
base: feat/a-wave-builds-at-once-and-the-hook-check
```
* **Why:** close writes task status into `BACKLOG.md` from three places mid-run, ship writes it again, and prune's `restoreFlips` exists to undo the collisions that causes. With parallel spawns and concurrent groups, markdown as a live database gets worse. The run holds live status; the backlog changes once, in the ship commit.

#### [TSK-03.25.1] Close records status in the run, and ship writes the backlog once [P: H] [READY]
```yaml
files: [internal/line/status.go, internal/line/status_test.go, internal/line/close.go, internal/line/close_test.go, internal/line/ship.go, internal/line/ship_test.go, internal/line/snapshot.go, internal/doctor/prune.go, internal/doctor/doctor_test.go, cmd/komodo/backlog.go]
done_when:
  - go test ./internal/line/... ./internal/doctor/... ./cmd/komodo/...
  - go vet ./internal/line/... ./internal/doctor/... ./cmd/komodo/...
  - "! grep -n 'writeStatus(' internal/line/close.go"
  - "! grep -n 'restoreFlips' internal/doctor/prune.go"
  - go run ./cmd/komodo doctor
context:
  - "status.go keeps .komodo/status.json, taskID to IN_PROGRESS, DONE, or BLOCKED with the note, written atomically; close.go:83, 96, and 101 write there instead of BACKLOG.md"
  - "the snapshot's Open for a task reads status.json first and the parsed backlog second, so step decides exactly as before"
  - "ShipGroup writes every status in status.json into BACKLOG.md in the ship commit, as ship.go:117 already does for DONE, then clears status.json; a task worktree's BACKLOG.md is never edited, so wave merges never conflict on it"
  - "prune drops restoreFlips and its call, since no status flip is left uncommitted; komodo list overlays status.json when a run is open, so a human still sees live status"
type: refactor
tier: heavy
```

### [TG-03.26] The line builds wide, picks the smallest machine that works, and measures itself
```yaml
type: feat
version: 1.1.0
base: refactor/live-status-lives-in-the-run-and-the-bac
```
* **Why:** across the last 11 runs build was 72 to 85 percent of wall time. TG-03.20 chained 3 of its 5 tasks on one file, so it was serial by design. Waves split by directory, one group runs at a time, every builder is Sonnet, and `komodo metrics` cannot say tasks per hour.

#### [TSK-03.26.1] Waves split by file, not directory [P: H] [READY]
```yaml
files: [internal/line/dag.go, internal/line/dag_test.go, internal/line/collide.go, internal/line/wave_test.go, komodo/rules/backlog.md, README.md]
done_when:
  - go test ./internal/line/...
  - go vet ./internal/line/...
  - go run ./cmd/komodo lint
  - go run ./cmd/komodo doctor
context:
  - "dirsOverlap in dag.go becomes claimsOverlap: two tasks overlap when they list the same file, or one lists a directory that holds a path the other lists; two tasks editing different files in one directory share a wave"
  - "RefuseCollision in collide.go applies the same rule against closed, unmerged task branches"
  - "close --wave already merges in order, stops on a conflict naming both tasks, and runs the compile gate after, so a same-package clash still stops the run for a person"
  - "komodo/rules/backlog.md: tasks that share no file run in parallel; a shared file or a directory claim serializes; README's Waves line says the same"
  - "tests: two tasks on internal/a/x.go and internal/a/y.go share wave 1; tasks on internal/a/x.go and internal/a share nothing and serialize; the existing directory tests are updated to the file rule"
type: feat
```

#### [TSK-03.26.2] Lint notes a group whose dependencies make it serial [P: M] [READY]
```yaml
files: [internal/backlog/lint.go, internal/backlog/backlog_test.go, cmd/komodo/backlog.go]
done_when:
  - go test ./internal/backlog/... ./cmd/komodo/...
  - go vet ./internal/backlog/... ./cmd/komodo/...
  - go run ./cmd/komodo lint
context:
  - "Notes(parsed) returns advice that never fails lint: for a group with at least 3 open tasks whose longest depends_on chain covers more than half of them, note <group>: <n> of <m> tasks are one chain; split the shared file or drop a dependency to build in parallel"
  - "komodo lint prints each note prefixed note after the problems and still exits zero when there are no problems; DONE tasks never count"
  - "tests: a 5-task group with a 3-task chain gets a note, a 5-task group with two 2-task chains gets none, a group whose chain is all DONE gets none"
type: feat
```

#### [TSK-03.26.3] A small task builds on the light tier and repairs on standard [P: M] [READY]
```yaml
files: [internal/line/snapshot.go, internal/line/step.go, internal/line/step_test.go, internal/mount/registry.go]
done_when:
  - go test ./internal/line/... ./internal/mount/...
  - go vet ./internal/line/... ./internal/mount/...
context:
  - "a builder spawn with no task tier, at most one source file plus its _test file in files, and no failed attempt, resolves on the light tier; its repair resolves on standard, so a failure costs one cheap attempt"
  - "Overlay gains light_builder, default true; false keeps every builder on standard; a task tier key always wins"
  - "why names it: TSK-x is one file, so it builds on light; or TSK-x failed once on light, so its repair builds on standard"
  - "tests: a one-file task spawns on haiku, its repair on sonnet, a two-file task on sonnet, light_builder false keeps sonnet, tier heavy stays opus"
type: feat
```

#### [TSK-03.26.4] Metrics report tasks per hour and tokens per changed line [P: M] [READY]
```yaml
files: [internal/ledger/ledger.go, internal/ledger/ledger_test.go, internal/line/ship.go, internal/line/ship_test.go]
done_when:
  - go test ./internal/ledger/... ./internal/line/...
  - go vet ./internal/ledger/... ./internal/line/...
  - go run ./cmd/komodo metrics
context:
  - "Entry gains lines, the added plus deleted count from git diff --shortstat <base>...<branch>, which ShipGroup stamps on its ship row"
  - "Aggregate adds tasks per hour of line time, median wall seconds per task from the first brief to ship, tokens per changed line by model, and repair rate by tier; Render prints each under its own heading"
  - "an entry without lines is skipped for tokens per line, never read as zero"
type: feat
```

#### [TSK-03.26.5] Two groups with disjoint files run at once [P: H] [READY]
```yaml
files: [internal/line/worktree.go, internal/line/worktree_test.go, internal/line/next.go, internal/line/next_test.go, internal/line/snapshot.go, internal/line/step.go, internal/line/step_test.go, internal/line/ship.go, internal/run/run.go, internal/run/run_test.go]
done_when:
  - go test ./internal/line/... ./internal/run/...
  - go vet ./internal/line/... ./internal/run/...
  - go run ./cmd/komodo doctor
depends_on: [TSK-03.26.1, TSK-03.26.3]
context:
  - "run state moves to .komodo/runs/<group>/: run.json, run.lock, ship.json; a legacy .komodo/run.json is read once and moved there"
  - "RefuseOpenRun in next.go refuses a new group only when an open group's files overlap it under claimsOverlap from TSK-03.26.1; --force still overrides"
  - "komodo step <group> continues that group; bare komodo step continues the one open group, and with two open it prints done naming both and asking for one"
  - "the launcher's finishShip reads the handoff under its own group's directory; the ledger already carries run and group on every row"
  - "tests: two groups on disjoint files both cut and each step returns its own spawn; overlapping groups refuse the second; bare step with two open runs names both"
type: feat
tier: heavy
```

### [TG-03.27] One git adapter, a leaf guard, and the planner in its own package
```yaml
type: refactor
version: 1.1.0
base: feat/the-line-builds-wide-picks-the-smallest
```
* **Why:** `line` is 3,505 source lines and imports 13 of 19 internal packages. git runs through four private helpers, and `worktree list --porcelain` is parsed three times in doctor alone. The guard runs on every tool call yet links the Ollama client through `profile` only to find the overlay path, which `mount.OverlayPath` already returns.

#### [TSK-03.27.1] One git adapter replaces four private helpers [P: M] [READY]
```yaml
files: [internal/git/git.go, internal/git/git_test.go, internal/line/worktree.go, internal/line/collide.go, internal/doctor/doctor.go, internal/doctor/prune.go, cmd/komodo/release.go]
done_when:
  - go test ./...
  - go vet ./...
  - "! grep -nE '^func (git|gitOr|gitRun)\\(' internal/line/worktree.go internal/line/collide.go internal/doctor/doctor.go cmd/komodo/release.go"
  - go run ./cmd/komodo doctor
context:
  - "internal/git exports Run(dir, args...) (string, error) with the trimming and error text line/worktree.go's git gives today, Or(dir, args...) string for collide.go's gitOr, and Worktrees(dir) ([]Worktree, error) with Path, Branch, and Head parsed from worktree list --porcelain"
  - "doctor.go:78 and prune.go:17 and :91 call git.Worktrees instead of parsing porcelain themselves; behaviour and messages do not change"
  - "internal/git imports only the standard library"
type: refactor
```

#### [TSK-03.27.2] The guard reads the overlay path from mount and never links profile [P: M] [READY]
```yaml
files: [internal/guard/policy.go, internal/profile/profile.go, internal/profile/profile_test.go]
done_when:
  - go test ./internal/guard/... ./internal/profile/...
  - go vet ./internal/guard/... ./internal/profile/...
  - "! go list -deps ./internal/guard | grep -qE '^komodo/internal/(profile|mount/ollama)$'"
  - go run ./cmd/komodo guard check
context:
  - "policy.go:111 and :112 call profile.MachineOverlayPath, which returns the same path as mount.OverlayPath; call mount.OverlayPath and drop the profile import"
  - "profile.MachineOverlayPath is deleted and its callers use mount.OverlayPath"
type: refactor
```

#### [TSK-03.27.3] The planner moves out of line into its own package [P: M] [READY]
```yaml
files: [internal/plan/plan.go, internal/plan/plan_test.go, internal/line/dag.go, internal/line/dag_test.go, internal/line/next.go, internal/line/step.go, internal/line/collide.go]
done_when:
  - go test ./...
  - go vet ./...
  - test ! -f internal/line/dag.go
  - "! go list -deps ./internal/plan | grep -q '^komodo/internal/line$'"
depends_on: [TSK-03.27.1]
context:
  - "internal/plan takes Topological, Waves, BlockedBy, and claimsOverlap from dag.go, with its tests from dag_test.go; it imports backlog only"
  - "line's callers in next.go, step.go, and collide.go call plan.Waves, plan.BlockedBy, and plan.Overlap; a move, so no body changes"
type: refactor
```

#### [TSK-03.27.4] The devices move out of line into their own package [P: L] [REFINEMENT]
```yaml
context:
  - "brief.go, diff.go, and clip.go are the input devices; they need an architect pass on what they share with close before a move"
type: refactor
```

#### [TSK-03.27.5] The stations move out of line into their own package [P: L] [REFINEMENT]
```yaml
context:
  - "close.go, verify.go, wave.go, and ship.go are the stations; the split waits on the devices move and the snapshot from TSK-03.21.3"
type: refactor
```

### [TG-03.28] Every package clears 75 percent coverage
```yaml
type: test
version: 1.1.0
base: main
```
* **Why:** `docs/scorecard.md` puts Code at 90 only when every package is at or above 70 percent. On `1eff696`, `internal/gate` is at 67.4 and `internal/pr` at 68.4. Both are tested here to 75, with room above the bar.

#### [TSK-03.28.1] The gate's hook rendering and install paths are tested to 75 percent [P: M] [READY]
```yaml
files: [internal/gate/gate_test.go]
done_when:
  - go test ./internal/gate/...
  - go vet ./internal/gate/...
  - "go test -cover ./internal/gate/ | awk '{for(i=1;i<=NF;i++) if($i ~ /%$/) {sub(/%/,\"\",$i); exit !($i+0 >= 75)}}'"
context:
  - "test only: no change to gate.go; cover the untested branches go test -coverprofile names, the hook script per platform, an unsupported platform's message, the fuzz lane flag, and a failing check's exit"
type: test
```

#### [TSK-03.28.2] The pull request client is tested to 75 percent [P: M] [READY]
```yaml
files: [internal/pr/pr_test.go]
done_when:
  - go test ./internal/pr/...
  - go vet ./internal/pr/...
  - "go test -cover ./internal/pr/ | awk '{for(i=1;i<=NF;i++) if($i ~ /%$/) {sub(/%/,\"\",$i); exit !($i+0 >= 75)}}'"
context:
  - "test only: no change to pr.go; drive the client against an httptest server or a fake gh on PATH, as its existing tests do, covering labels that do not exist, a draft PR, an API error, and thread listing"
type: test
```

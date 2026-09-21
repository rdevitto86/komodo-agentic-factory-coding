# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it until TG-03.2 lands `komodo lint`. The V1.x backlog was dropped whole on 2026-09-21 in favour of the V1.5 plan in `README.md`.

---

## [EPIC-03] V1.5, the assembly line
*Goal: one static binary is the conveyor and the devices, markdown is everything a model reads, one guard is the only hook, and a model is a machine mounted per host. Two model calls per task, build and review; everything between is deterministic. Claude Code with Ollama is the host today for both Komodo devs; Codex is the exit test. Requirements and design: `README.md`.*

* **Groups run in file order.** Every group carries `1.5.0`; the tag is cut once, after TG-03.6.
* **V1 runs TG-03.1 through TG-03.3.** After TG-03.4 lands the `run` skill runs the rest, and its first group is the proof.
* **Go source lands beside the Python it replaces.** The gate runs both until TG-03.6 deletes the Python.

### [TG-03.1] The markdown
```yaml
type: refactor
version: 1.5.0
```
* **Why:** everything a model reads is one of three neutral formats. Standards become skills so their trigger is their own frontmatter. Briefs fold into roles so a role is the brief and its schema. The policy shrinks to four denials for a greenfield shop, and the rules give an agent unlimited freedom inside its worktree.

#### [TSK-03.1.1] Standards become skills at the source [P: C] [READY]
```yaml
files: [komodo/skills, komodo/standards, komodo/standards.py, tests/test_standards.py]
done_when:
  - test -f komodo/skills/standards-go/SKILL.md
  - test ! -d komodo/standards
  - python3 -m unittest discover -s tests -q
context:
  - "each komodo/standards/<x>.md moves to komodo/skills/standards-<x>/SKILL.md with frontmatter name, description, and a globs list of the extensions and directories that trigger it; the body does not change"
  - "the extension map in komodo/standards.py moves into that frontmatter and nothing else holds it; standards.py and its test go"
type: refactor
```

#### [TSK-03.1.2] Briefs fold into roles, and each role carries its schema [P: C] [READY]
```yaml
files: [komodo/roles, komodo/briefs, komodo/briefs.py, tests/test_roles.py, tests/test_briefs.py]
done_when:
  - test ! -d komodo/briefs
  - test -f komodo/roles/builder.schema.json
  - python3 -m unittest tests.test_roles -q
context:
  - "a role file carries frontmatter name, tier, tools, session, returns, and a body that is the brief template with the slots task_block, repo_rules, repo_context, context, files, standards, done_when, failure"
  - "tools are the five Komodo verbs read, edit, write, shell, search; a host tool name never appears in a role"
  - "the JSON schema each worker prompt carried moves beside the role as <role>.schema.json; the builder's body says to write its result to the path the brief names"
type: refactor
```

#### [TSK-03.1.3] The policy has four denials, and the rules give a worktree unlimited freedom [P: C] [READY]
```yaml
files: [komodo/policy.json, komodo/AGENTS.md, komodo/rules, tests/test_policy.py]
done_when:
  - python3 -c "import json; json.load(open('komodo/policy.json'))"
  - test -f komodo/AGENTS.md
  - python3 -m unittest tests.test_policy -q
context:
  - "policy.json: critical refs main and master plus a list, config paths the hosts and the toolkit own, and the trailer patterns; no destructive command list, no prod markers, no file-list scope"
  - "komodo/rules/AGENTS.md moves to komodo/AGENTS.md; its Git section says an agent may delete files, reset, checkout, restore, force-push and delete its own branches inside its worktree, and never touches a critical branch, a path outside the worktree, or a host or toolkit config"
  - "komodo/rules/accessibility.md holds the Writing for a human contract and is included into AGENTS.md at render; komodo/rules/cli.md goes"
type: feat
```

#### [TSK-03.1.4] The merger role goes; the responder stays as a session role [P: M] [READY]
```yaml
files: [komodo/roles/merger.md, komodo/roles/responder.md, tests/test_roles.py]
done_when:
  - test ! -f komodo/roles/merger.md
  - python3 -m unittest tests.test_roles -q
context:
  - "QC stops on a conflict and hands it to a human, so no role resolves conflicts; responder becomes session: true and is the model behind the respond skill"
  - "nine roles remain: architect, builder, planner, researcher, responder, reviewer, scout, summarizer, tester"
type: refactor
```

### [TG-03.2] The conveyor and the devices
```yaml
type: feat
version: 1.5.0
```
* **Why:** the line is one static Go binary with no interpreter, shell, or symlink on a dev machine. Every station is a subcommand with a test. The hooks' 1013 lines of Go under `komodo/hooks/src` are the seed of the module.

#### [TSK-03.2.1] The Go module, the binary, lint, and the gate [P: C] [READY]
```yaml
files: [go.mod, cmd/komodo/main.go, internal/backlog, bin, scripts/verify.py, .github/workflows]
done_when:
  - go build ./...
  - go test ./internal/backlog/...
  - python3 scripts/verify.py
context:
  - "go.mod at the repo root, module komodo, Go 1.22, no dependencies outside the standard library; komodo/hooks/src moves under internal with its tests"
  - "internal/backlog ports tasks.py: parse, lint, list, add, set status, next task id, find backlog at the root or docs; komodo lint and komodo list and komodo add are the first subcommands"
  - "bin/ holds komodo-darwin-arm64, komodo-windows-amd64.exe, komodo-linux-amd64 and MANIFEST.sha256; a workflow builds all three on a PR and fails when they differ from the committed ones; scripts/verify.py runs go build and go test while Python remains"
type: feat
```

#### [TSK-03.2.2] Intake: `komodo next` [P: C] [READY]
```yaml
files: [internal/line/next.go, internal/line/next_test.go, internal/line/worktree.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.2.1]
context:
  - "next prints the next READY group, or the group of a named task, as JSON: tasks, dependencies, waves by directory, mode single as one wave, type, version, and the resolved machine per role; nothing when nothing is ready"
  - "next --start fetches the remote base, creates <type>/<slug> in a worktree under .komodo/wt/group from it, and calls the preflight tag; the main working tree is never read or required clean"
  - "a task with a valid .komodo/results/<task>.json is listed done and skipped, which is resume; done and in-progress rewrite only the status token"
type: feat
```

#### [TSK-03.2.3] Input device: `komodo brief` [P: C] [READY]
```yaml
files: [internal/line/brief.go, internal/line/brief_test.go, internal/line/clip.go]
done_when:
  - go test ./internal/line/...
depends_on: [TSK-03.2.2]
context:
  - "fills the role template from the slots the README tables, each clipped by its cap with the head 70 tail 25 marker briefs.clip used; pipeline.task_slots and pipeline.repo_rules are the prior art"
  - "writes .komodo/briefs/<task>.md in a task worktree under .komodo/wt/<task> branched from the group branch and prints both paths as JSON; the builder's result path is named in the brief"
  - "--dry-run prints each slot's characters after clipping and a token estimate and writes nothing"
type: feat
```

#### [TSK-03.2.4] Output device: `komodo close <task>` [P: C] [READY]
```yaml
files: [internal/line/close.go, internal/line/close_test.go, internal/line/schema.go, internal/comments]
done_when:
  - go test ./internal/line/... ./internal/comments/...
depends_on: [TSK-03.2.3]
context:
  - "validates the result JSON against the role schema for type, required, and enum; reruns done_when in the worktree; runs the comment lint on the task's files; flips the status"
  - "a failure writes the failure slot and the previous attempt's diff so the next brief is a repair, at most one; a second failure marks BLOCKED with the note and the wave continues, as pipeline._block did"
  - "internal/comments ports comment_rules.py; komodo comments check stays as a command"
type: feat
```

#### [TSK-03.2.5] QC and ship: `komodo close --wave` and `--group` [P: C] [READY]
```yaml
files: [internal/line/wave.go, internal/line/ship.go, internal/line/verify.go, internal/line/wave_test.go, internal/pr]
done_when:
  - go test ./internal/line/... ./internal/pr/...
depends_on: [TSK-03.2.4]
context:
  - "--wave merges the wave's worktrees into the group branch in order and stops on the first conflict naming both tasks; runs the compile gate for the languages the wave touched with one repair as gates.compile_commands did; then the verify command, resolved in V1's order or named by .komodo/commands.json"
  - "review: findings at or above severity_floor become one repair brief; the rest are appended to BACKLOG.md as tasks by code"
  - "--group commits, pushes, opens the PR with the report as body and the category and agent labels only when the repo defines them, as a draft when a task is blocked; writes the changelog line under the group's version; flips DONE; works on any branch, which is how freehand work ships"
  - "internal/pr ports pr.py: view, create, edit, threads, label, comment, reply as gh wrappers"
type: feat
```

#### [TSK-03.2.6] `komodo diff`, `report`, `tag`, and `release check` [P: H] [READY]
```yaml
files: [internal/line/diff.go, internal/line/report.go, internal/release, internal/line/diff_test.go]
done_when:
  - go test ./internal/line/... ./internal/release/...
depends_on: [TSK-03.2.5]
context:
  - "diff prints the group branch against base, clipped by the failure cap, with the group's task blocks and the standards the diff's extensions touch; it is the whole reviewer input"
  - "report prints per task seconds, turns, tokens when the result carries them, findings by severity, and what blocked, in the accessibility contract; render.report is the prior art"
  - "internal/release ports render.py's changelog functions; tag tags every changelog version no tag points at, annotated v<version>, and pushes it from the group worktree on a clean base; release check audits drift read-only and exits non-zero"
type: feat
```

### [TG-03.3] The guard and the mounts
```yaml
type: feat
version: 1.5.0
```
* **Why:** one hook on every host, four denials, and unlimited freedom inside a worktree. A mount is the only code that knows a host. Profiles select themselves from the host, the plan, and whether Ollama answers.

#### [TSK-03.3.1] The guard: four denials, worktree scope, and the table in the gate [P: C] [READY]
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

#### [TSK-03.3.2] `komodo install --host claude|codex|both`: the mounts [P: C] [READY]
```yaml
files: [internal/mount/claude, internal/mount/codex, internal/install, cmd/komodo/main.go]
done_when:
  - go test ./internal/mount/... ./internal/install/...
  - go run ./cmd/komodo install --dry-run
depends_on: [TSK-03.3.1]
context:
  - "claude: CLAUDE.md with an @AGENTS.md include, agents from roles with model from the profile, the run, review, backlog, and respond skills copied, each standard as a one-line pointer skill, the guard registered once on PreToolUse, the permissions convenience layer from policy.json, the personal overlay CLAUDE.local.md and settings.local.json seeded once and never overwritten"
  - "codex: AGENTS.md, agents/<role>.toml with name, description, developer_instructions, model, model_reasoning_effort, sandbox_mode, skills copied under .agents, the guard in hooks.json, mcp_servers in config.toml"
  - "the five Komodo verbs map to host tool names or sandbox_mode here and nowhere else; install is a copy, never a symlink, and runs on Windows; --dry-run prints what would change"
type: feat
```

#### [TSK-03.3.3] Doctor: references, leaks, drift, budgets, prune [P: H] [READY]
```yaml
files: [internal/doctor, cmd/komodo/main.go, scripts/validate.py]
done_when:
  - go test ./internal/doctor/...
  - go run ./cmd/komodo doctor --no-git
depends_on: [TSK-03.3.2]
context:
  - "keeps V1's checks: backticked references resolve, roles are well formed, changelog and tags agree, git leftovers; adds: a vendor name, host tool, host path, or host flag outside internal/mount fails; the rendered layout differs from what the source renders now fails; always-on context over 1500 tokens, the run skill over 800, or a standard over 8 KB fails, which retires scripts/validate.py"
  - "--prune removes stale worktrees under .komodo/wt and deletes branches merged into base; --json stays"
type: feat
```

#### [TSK-03.3.4] Profiles select themselves: host, plan probe, Ollama, and pacing [P: H] [READY]
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

### [TG-03.4] The skills and the launcher
```yaml
type: feat
version: 1.5.0
```
* **Why:** the run skill is the list of stations and the two spawns, under 800 tokens. Ad hoc work enters at any station or stays off the line. The proof runs one group each way before anything is deleted.

#### [TSK-03.4.1] The run, review, backlog, and respond skills [P: C] [READY]
```yaml
files: [komodo/skills/run/SKILL.md, komodo/skills/review/SKILL.md, komodo/skills/backlog/SKILL.md, komodo/skills/respond/SKILL.md]
done_when:
  - go run ./cmd/komodo doctor --no-git
depends_on: [TSK-03.3.3]
context:
  - "run takes a group, a task, or nothing: next --start; per wave, brief per task, spawn the builder role with the brief path, close per task; close --wave; diff, spawn the reviewer role with it, one repair when findings reach the floor; close --group; report"
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
  - "run TG-03.5 under the run skill in a Claude Code session; record wall time, tokens, and turns beside the V1 numbers for TG-03.3 from its run state, under a Proof: V1 versus the run skill heading in the 1.5.0 changelog entry"
  - "slower by more than one wave means the skill is wrong; fix the skill before TG-03.5 merges"
type: docs
```

### [TG-03.5] The repo layer and the local machines
```yaml
type: feat
version: 1.5.0
```
* **Why:** one universal set of rules, one place a repo adds what only it knows, and a local machine on every host. Nothing here is required and nothing here widens what the guard denies.

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
  - ".komodo/standards/<name>.md appends to the shipped standard of that name in the standards slot or adds a new one with its own globs; .komodo/skills/<name>/SKILL.md is a new skill or appends a Repo overrides section to a shipped one, frontmatter untouched; a repo never replaces or removes a shipped body"
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
type: feat
```

#### [TSK-03.5.4] `komodo bridge`: a stdio MCP server over Ollama [P: C] [READY]
```yaml
files: [internal/bridge, cmd/komodo/main.go]
done_when:
  - go test ./internal/bridge/...
context:
  - "JSON-RPC 2.0 over stdin and stdout: initialize, tools/list, tools/call; tools local_chat(model, system, prompt) and local_models(); net/http against OLLAMA_BASE_URL, default http://localhost:11434"
  - "Ollama down returns a tool error with the URL, never a crash; tested against a fake Ollama on a local listener"
type: feat
```

#### [TSK-03.5.5] The hybrid and local profiles mount the bridge [P: H] [READY]
```yaml
files: [internal/mount/claude, internal/mount/codex, internal/profile, komodo/roles/summarizer.md]
done_when:
  - go test ./internal/mount/... ./internal/profile/...
depends_on: [TSK-03.5.4, TSK-03.3.4]
context:
  - "the Claude mount registers the bridge as a stdio MCP server running komodo bridge, replacing the http entry at 127.0.0.1:8000 when present; a role whose provider is ollama renders with a body that calls local_chat with the profile's model; the reviewer may be ollama so a review never shares a vendor with the build; a missing bridge degrades to the light tier and says so once"
  - "the Codex mount's local profile sets oss_provider ollama and a model_providers.ollama base_url in config.toml, and every tier's model from the profile"
type: feat
```

### [TG-03.6] Demolition and the exit test
```yaml
type: chore
version: 1.5.0
```
* **Why:** nothing from the V1 orchestrator survives, the gate is Go, and the second host proves the mounts are the only host-specific code.

#### [TSK-03.6.1] The Python goes and `go test` is the gate [P: C] [READY]
```yaml
files: [komodo/pipeline.py, komodo/state.py, komodo/gates.py, komodo/pr.py, komodo/pr_actions.py, komodo/account.py, komodo/gitops.py, komodo/workers, komodo/tasks.py, komodo/dag.py, komodo/render.py, komodo/config.py, komodo/install.py, komodo/doctor.py, komodo/comment_rules.py, komodo/__main__.py, komodo/__init__.py, komodo/adapters, komodo/hooks, tests, scripts]
done_when:
  - test ! -f komodo/__main__.py
  - test ! -d komodo/hooks
  - go vet ./...
  - go test ./...
context:
  - "delete, do not stub; komodo/ keeps only AGENTS.md, rules, roles, skills, policy.json; .github runs go vet, go test, komodo doctor, komodo guard check, and the binary match"
type: chore
```

#### [TSK-03.6.2] README and the templates describe what exists [P: H] [READY]
```yaml
files: [README.md, templates/project]
done_when:
  - go run ./cmd/komodo doctor
depends_on: [TSK-03.6.1]
context:
  - "the README drops its planned status and the V1 coverage table, keeps the line, the stations, the devices, the mounts, ad hoc, the guard, the binary, the repo layer, setup, usage, layout; templates/project carries AGENTS.md, BACKLOG.md, CHANGELOG.md, the docs/spec starters the grammar's context anchors point at, and the example .komodo/context file; CLAUDE.md.tmpl goes"
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

#### [TSK-03.6.4] Changelog 1.5.0 [P: M] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "## 1.5.0" CHANGELOG.md
depends_on: [TSK-03.6.3]
context:
  - "one heading: the line, what was removed, what replaced it, both proofs"
type: docs
```

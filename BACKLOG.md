# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it. The V1.x backlog was dropped whole on 2026-09-21 in favour of the V1.5 plan in `README.md`; the PR that landed the plan lists what it replaced.

---

## [EPIC-03] V1.5, the line is code and the loop is the host
*Goal: Komodo becomes rules, roles, standards, skills, one guard, and a set of deterministic commands that any agent host renders. A model is called twice per task, to build and to review; everything between is standard-library Python. Claude Code with Ollama is the host today for both Komodo devs; Codex is the rehearsal for the day Komodo leaves a proprietary host. Roadmap and design: `README.md`.*

* **Groups run in file order.** Every group carries `1.5.0`; the tag is cut once, after TG-03.9.
* **V1 runs TG-03.1 through TG-03.4.** After TG-03.5 lands the `run` skill runs the rest, and its first group is the proof.

### [TG-03.1] Source reshape
```yaml
type: refactor
version: 1.5.0
```
* **Why:** three neutral formats are the whole portability story. Standards become skills so their trigger lives in their own frontmatter. Briefs fold into roles so a role is the brief. The git and destruction policy becomes data one guard reads on every host.

#### [TSK-03.1.1] Standards become skills at the source [P: C] [READY]
```yaml
files: [komodo/skills, komodo/standards, komodo/standards.py, tests/test_standards.py]
done_when:
  - test -f komodo/skills/standards-go/SKILL.md
  - test ! -d komodo/standards
  - python3 -m unittest discover -s tests -q
context:
  - "each komodo/standards/<x>.md moves to komodo/skills/standards-<x>/SKILL.md with frontmatter name and description; the description front-loads the extensions and directories that trigger it"
  - "the body does not change; a standard names no live repo, port, URL, version, or path"
  - "the extension-to-standard map in komodo/standards.py moves into the skills' own frontmatter as a globs list, so nothing else holds it"
type: refactor
```

#### [TSK-03.1.2] Briefs fold into roles, and each role carries its return schema [P: C] [READY]
```yaml
files: [komodo/roles, komodo/briefs, komodo/briefs.py, tests/test_roles.py, tests/test_briefs.py]
done_when:
  - test ! -d komodo/briefs
  - test -f komodo/roles/builder.schema.json
  - python3 -m unittest tests.test_roles -q
context:
  - "a role file carries frontmatter name, tier, tools, session, returns, and a body that is the brief template with the slots task_block, repo_rules, repo_context, context, files, standards, done_when, failure"
  - "tools are the five Komodo verbs read, edit, write, shell, search; a host tool name never appears in a role"
  - "the JSON schema each worker prompt carried in briefs.py moves beside the role as <role>.schema.json; the clip function moves to komodo/line.py in TG-03.4 and briefs.py goes"
type: refactor
```

#### [TSK-03.1.3] The policy file and the rules both hosts read [P: H] [READY]
```yaml
files: [komodo/policy.json, komodo/AGENTS.md, komodo/rules, tests/test_policy.py]
done_when:
  - python3 -c "import json; json.load(open('komodo/policy.json'))"
  - test -f komodo/AGENTS.md
  - python3 -m unittest tests.test_policy -q
context:
  - "policy.json holds protected refs, forbidden git verbs, forbidden trailers, destructive command patterns, and prod host markers; gitops.py PROTECTED and hooks/src/guard.go carry the current lists"
  - "komodo/rules/AGENTS.md moves to komodo/AGENTS.md; komodo/rules/accessibility.md holds the Writing for a human contract and is included into AGENTS.md at render"
  - "komodo/rules/cli.md goes; the CLI documents itself with --help"
type: feat
```

### [TG-03.2] Guard and inject
```yaml
type: feat
version: 1.5.0
```
* **Why:** two stdlib scripts replace a Go binary, two Python fallbacks, and two git hooks. Both hosts send the same payload fields and accept the same deny JSON, so one script serves both, and verify runs its table so a broken guard fails the gate and never a run.

#### [TSK-03.2.1] One guard for shell, edit, and write on both hosts [P: C] [READY]
```yaml
files: [komodo/hooks/guard.py, tests/test_guard.py]
done_when:
  - python3 -m unittest tests.test_guard -q
context:
  - "reads hook_event_name, cwd, tool_name, tool_input from stdin; denies with the PreToolUse permissionDecision JSON both hosts accept, exit 2 as the fallback"
  - "git findings come from policy.json; destruction findings from its patterns; scope findings compare an edit or write path against .komodo/scope.json in cwd when it exists"
  - "an internal error logs one line to stderr and allows; the test table holds at least 60 commands, half allowed and half denied, and names the finding for each denial"
  - "komodo/adapters/claude/hooks/guard.py and hooks/src/guard.go are the prior art; this file replaces both and imports nothing outside the standard library"
type: feat
```

#### [TSK-03.2.2] Inject prints the work state on SessionStart [P: M] [READY]
```yaml
files: [komodo/hooks/inject.py, tests/test_inject.py]
done_when:
  - python3 -m unittest tests.test_inject -q
context:
  - "open, blocked, and in-progress counts from BACKLOG.md, the next READY group, the verify command, and the run command; silent outside a repo"
  - "hooks/src/inject.go is the prior art"
type: feat
```

#### [TSK-03.2.3] `komodo guard check` runs the table inside verify [P: H] [READY]
```yaml
files: [komodo/__main__.py, scripts/verify.py, tests/test_cli.py]
done_when:
  - python3 -m komodo guard check
  - python3 scripts/verify.py
depends_on: [TSK-03.2.1]
context:
  - "the table lives beside the test as data; guard check runs every row through the real script and prints one line per mismatch"
type: feat
```

#### [TSK-03.2.4] The Go hooks, the binaries, and the git hooks go [P: H] [READY]
```yaml
files: [komodo/hooks/src, komodo/hooks/bin, komodo/hooks/pre-commit, komodo/hooks/pre-push, komodo/hooks/pre-commit.py, komodo/hooks/pre-push.py, komodo/hooks/__init__.py, tests/test_hooks.py]
done_when:
  - test ! -d komodo/hooks/src
  - test ! -f komodo/hooks/pre-push
  - python3 -m unittest discover -s tests -q
depends_on: [TSK-03.2.1, TSK-03.2.2]
context:
  - "komodo hooks install and every reference to core.hooksPath go with them; the comment lint stays as komodo comments check, run by verify"
type: chore
```

### [TG-03.3] Profiles, the Claude adapter, and the CLI
```yaml
type: feat
version: 1.5.0
```
* **Why:** the adapter is the only Claude-specific code in the repo. Profiles become a tier-to-model table per host with the context caps beside them, and the CLI shrinks to what the line needs.

#### [TSK-03.3.1] Profiles map a tier to a provider, model, and effort per host, with the context caps [P: C] [READY]
```yaml
files: [komodo/config.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_config -q
context:
  - "profiles claude, hybrid, codex, local as the README tables them; each names a host; the reviewer tier may point at a different provider than the builder tier"
  - "context caps: repo_rules 8000, repo_context 8000, per_file 10000, file_total 24000, standards 6000, failure 80000 chars; pipeline.task_slots holds the current values"
  - "~/.komodo/config.json overlays per machine and can only lower a cap or add a denial; a repo never carries one; account.py and every plan-derived cap go"
type: feat
```

#### [TSK-03.3.2] The Claude adapter renders the new source [P: C] [READY]
```yaml
files: [komodo/adapters/claude.py, komodo/adapters/claude, komodo/install.py, komodo/render.py, scripts/validate.py, tests/test_adapter_claude.py, tests/test_render.py]
done_when:
  - python3 -m unittest tests.test_adapter_claude -q
  - python3 scripts/validate.py
depends_on: [TSK-03.3.1]
context:
  - "renders CLAUDE.md with an @AGENTS.md include, agents from roles with model from the profile, the run, review, and backlog skills copied as is, each standard as a one-line pointer skill to the installed file, both hooks registered in settings, and the permissions convenience layer from policy.json"
  - "the five Komodo tool verbs map to Claude tool names here and nowhere else"
  - "validate measures the always-on context under 1500 tokens, the run skill under 800, and every standard under 8 KB"
type: feat
```

#### [TSK-03.3.3] The CLI is install, doctor, tasks, comments, guard, bridge, and run [P: H] [READY]
```yaml
files: [komodo/cli.py, komodo/__main__.py, komodo/comments.py, komodo/comment_rules.py, tests/test_cli.py]
done_when:
  - python3 -m komodo --help
  - python3 -m unittest tests.test_cli -q
depends_on: [TSK-03.3.2]
context:
  - "install takes --host claude|codex and --profile; doctor, tasks, comments, and guard keep their names; status, hooks, pr, and every pipeline subcommand go; bridge and run are registered now and land in TG-03.7 and TG-03.5"
  - "comment_rules.py becomes comments.py with the same lint"
type: refactor
```

### [TG-03.4] The line in code
```yaml
type: feat
version: 1.5.0
```
* **Why:** everything between the two model calls is deterministic. V1 had this in the pipeline; V1.5 has it as commands the skill calls, so the session judges only what to spawn and what a result says.

#### [TSK-03.4.1] `komodo tasks next --json`, and resume by results on disk [P: C] [READY]
```yaml
files: [komodo/tasks.py, komodo/dag.py, tests/test_tasks.py]
done_when:
  - python3 -m unittest tests.test_tasks -q
context:
  - "next prints the next READY group with its tasks, dependencies, waves by directory, context paths, type, and version; nothing when no group is ready"
  - "a task with a valid .komodo/results/<task>.json is listed as done and skipped, so a rerun resumes; dag.py folds into tasks.py"
  - "tasks done and tasks in-progress rewrite only the status token; tasks plan, tasks migrate, and the worker call behind plan go"
type: feat
```

#### [TSK-03.4.2] `komodo tasks brief` fills the role template from the slots [P: C] [READY]
```yaml
files: [komodo/line.py, tests/test_line.py]
done_when:
  - python3 -m unittest tests.test_line -q
depends_on: [TSK-03.4.1]
context:
  - "slots: task block, repo rules from the repo's AGENTS.md or a one-line default, repo context from the repo layer, context anchors resolved to sections, existing files, standards by the task's extensions and the role's extras, done_when, and the failure slot with the previous diff on repair"
  - "every slot clipped by the config cap with the head 70 tail 25 marker from briefs.clip; pipeline.task_slots and pipeline.repo_rules are the prior art"
  - "writes .komodo/briefs/<task>.md and .komodo/scope.json in a new git worktree under .komodo/wt/<task> on the group branch, and prints the brief path and the worktree path as JSON"
type: feat
```

#### [TSK-03.4.3] `komodo tasks close` validates, gates, merges, and publishes [P: C] [READY]
```yaml
files: [komodo/line.py, komodo/tasks.py, tests/test_line.py]
done_when:
  - python3 -m unittest tests.test_line -q
depends_on: [TSK-03.4.2]
context:
  - "close <task>: validates .komodo/results/<task>.json against the role schema with a stdlib validator for type, required, and enum, reruns done_when in the worktree, runs the comment lint on the task's files, flips the status; a failure writes the failure slot and the previous diff so the next brief is a repair, at most one"
  - "close --wave: merges the wave's worktrees into the group branch in order, stops on the first conflict naming both tasks, runs the repo's verify command"
  - "close --group: commits on <type>/<name> with the group's message, pushes, opens the PR with the report, writes the changelog line under the group's version, flips DONE; every git write runs under the guard and refuses the same list gitops.py refused"
  - "findings below the review floor are appended to BACKLOG.md as tasks by code, as V1 did"
type: feat
```

#### [TSK-03.4.4] `komodo tasks diff` and `komodo tasks report` [P: H] [READY]
```yaml
files: [komodo/line.py, tests/test_line.py]
done_when:
  - python3 -m unittest tests.test_line -q
depends_on: [TSK-03.4.3]
context:
  - "diff prints the group branch against its base, clipped by the failure cap, with the group's task blocks and the standards the diff's extensions touch; it is the whole reviewer input"
  - "report prints per task seconds, turns, tokens when the result carries them, findings by severity, and what blocked, in the accessibility contract; render.py's report is the prior art and goes"
type: feat
```

### [TG-03.5] The run skill, the launcher, and the proof
```yaml
type: feat
version: 1.5.0
```
* **Why:** the skill is the list of commands and the two spawns, under 800 tokens, so nothing that slowed the 0.x loop exists. The proof runs one group each way before anything is deleted.

#### [TSK-03.5.1] The run, review, and backlog skills [P: C] [READY]
```yaml
files: [komodo/skills/run/SKILL.md, komodo/skills/review/SKILL.md, komodo/skills/backlog/SKILL.md]
done_when:
  - python3 scripts/validate.py
context:
  - "run: tasks next; for each wave, tasks brief per task, spawn the builder role with the brief path, tasks close per task; tasks close --wave; tasks diff, spawn the reviewer role with it, one repair through brief and close when findings reach the floor; tasks close --group; tasks report"
  - "the skill names no host tool, path, flag, or vendor; it says spawn the builder role and the adapter decides how"
  - "review is the reviewer role's session entry over tasks diff; backlog is the grammar with the add and lint commands"
type: feat
```

#### [TSK-03.5.2] The headless launcher [P: H] [READY]
```yaml
files: [komodo/run.py, tests/test_run.py]
done_when:
  - python3 -m unittest tests.test_run -q
context:
  - "komodo run <group> scrubs push tokens, credential helpers, and SSH identities from the environment, picks the host from the profile, invokes the host's non-interactive mode with the run skill and the group id, and exits with the host's code"
  - "about 30 lines; gitops.worker_env is the prior art for the scrub"
type: feat
```

#### [TSK-03.5.3] Proof: one group under V1 and under the run skill [P: C] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "Proof: V1 versus the run skill" CHANGELOG.md
depends_on: [TSK-03.5.1, TSK-03.5.2]
owner: human
context:
  - "run TG-03.6 under the run skill in a Claude Code session; record wall time, tokens, and turns beside the V1 numbers for TG-03.4 from its run state, under a Proof: V1 versus the run skill heading in the 1.5.0 changelog entry"
  - "slower by more than one wave means the skill is wrong; fix the skill before TG-03.6 merges"
type: docs
```

### [TG-03.6] The repo layer
```yaml
type: feat
version: 1.5.0
```
* **Why:** one universal set of rules, and one place a repo adds what only it knows. Nothing here is required, nothing here can widen the floor, and drift fails doctor rather than going silent.

#### [TSK-03.6.1] Repo context files inject by glob [P: H] [READY]
```yaml
files: [komodo/repo.py, komodo/line.py, tests/test_repo.py, templates/project]
done_when:
  - python3 -m unittest tests.test_repo tests.test_line -q
context:
  - ".komodo/context/*.md with a paths glob list in frontmatter; a file whose globs match any of the task's files fills the repo_context slot, clipped; no globs means every task"
  - "a malformed file is skipped with one line in the report and one at SessionStart; templates/project gains one example context file"
type: feat
```

#### [TSK-03.6.2] Repo standards extend or add [P: H] [READY]
```yaml
files: [komodo/repo.py, komodo/line.py, tests/test_repo.py]
done_when:
  - python3 -m unittest tests.test_repo -q
depends_on: [TSK-03.6.1]
context:
  - ".komodo/standards/<name>.md with the same frontmatter as a shipped standard; a matching name is appended to the shipped one in the standards slot, a new name is a new standard with its own globs"
  - "the session sees the repo standard through the same pointer mechanism the adapter renders, resolved at read time"
type: feat
```

#### [TSK-03.6.3] Exclusions with a floor, and doctor fails on drift [P: M] [READY]
```yaml
files: [komodo/repo.py, komodo/doctor.py, tests/test_repo.py, tests/test_doctor.py]
done_when:
  - python3 -m unittest tests.test_repo tests.test_doctor -q
depends_on: [TSK-03.6.2]
context:
  - ".komodo/exclude lists shipped standards this repo never loads; comments, api-security, and sdlc cannot be excluded"
  - "doctor fails when an excluded standard's globs match tracked files and no .komodo/standards/<name>.md exists; git ls-files against the globs is the whole check"
  - "precedence is config defaults, then ~/.komodo/config.json which only tightens, then the repo layer"
type: feat
```

### [TG-03.7] Local models on Claude Code
```yaml
type: feat
version: 1.5.0
```
* **Why:** Claude Code cannot run a subagent on Ollama, so a role that runs locally calls a tool. The bridge at `127.0.0.1:8000` lives in no repo and was down for a whole session; V1.5 owns a bridge the host spawns, in the standard library, with no process to keep alive.

#### [TSK-03.7.1] `komodo bridge` is a stdio MCP server over Ollama [P: C] [READY]
```yaml
files: [komodo/bridge.py, tests/test_bridge.py]
done_when:
  - python3 -m unittest tests.test_bridge -q
context:
  - "JSON-RPC 2.0 over stdin and stdout: initialize, tools/list, tools/call; tools local_chat(model, system, prompt) and local_models(); urllib against OLLAMA_BASE_URL, default http://localhost:11434"
  - "Ollama down returns a tool error with the URL, never an exception; the host keeps running"
  - "about 200 lines, standard library only, tested with a fake Ollama on a local socket"
type: feat
```

#### [TSK-03.7.2] Install registers the bridge on the Claude host [P: H] [READY]
```yaml
files: [komodo/adapters/claude.py, tests/test_adapter_claude.py]
done_when:
  - python3 -m unittest tests.test_adapter_claude -q
depends_on: [TSK-03.7.1]
context:
  - "an MCP server entry of type stdio running python3 -m komodo bridge, replacing the http entry at 127.0.0.1:8000 when present"
type: feat
```

#### [TSK-03.7.3] The hybrid profile routes light roles, and optionally the reviewer, through the bridge [P: H] [READY]
```yaml
files: [komodo/roles/summarizer.md, komodo/config.py, komodo/adapters/claude.py, tests/test_adapter_claude.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_adapter_claude tests.test_config -q
depends_on: [TSK-03.7.2]
context:
  - "a role whose profile provider is ollama on the claude host renders with a body that calls local_chat with the profile's model, and the light tier as the wrapper model"
  - "the reviewer tier may be ollama, so a review never shares a vendor with the build; off by default"
  - "a missing bridge degrades to the light tier and says so once"
type: feat
```

### [TG-03.8] The second host: Codex, the portability lint, and the exit test
```yaml
type: feat
version: 1.5.0
```
* **Why:** the second host proves the first was not special and rehearses the day Komodo leaves a proprietary host. Codex reads AGENTS.md and SKILL.md natively, takes agents as TOML, registers the same hook events with the same deny JSON, and runs every tier on Ollama with `--oss`.

#### [TSK-03.8.1] The Codex adapter [P: C] [READY]
```yaml
files: [komodo/adapters/codex.py, tests/test_adapter_codex.py]
done_when:
  - python3 -m unittest tests.test_adapter_codex -q
context:
  - "renders ~/.codex/AGENTS.md, ~/.codex/agents/<role>.toml with name, description, developer_instructions, model, model_reasoning_effort, sandbox_mode from the profile, ~/.agents/skills as a copy, ~/.codex/hooks.json with both hooks, and the mcp_servers table in config.toml with the bridge"
  - "the five Komodo tool verbs map to sandbox_mode and nothing else; Codex has no allow-list, so the guard is the whole policy there"
type: feat
```

#### [TSK-03.8.2] Doctor lints for host leakage and render drift [P: H] [READY]
```yaml
files: [komodo/doctor.py, tests/test_doctor.py]
done_when:
  - python3 -m unittest tests.test_doctor -q
  - python3 -m komodo doctor
context:
  - "a skill, role, or rule that names a host tool, a host path under ~/.claude or ~/.codex, a host flag, or a vendor name fails doctor; adapters and config are exempt"
  - "drift: the rendered layout differs from what the source would render now"
type: feat
```

#### [TSK-03.8.3] The local profile renders Codex with Ollama as the provider [P: H] [READY]
```yaml
files: [komodo/adapters/codex.py, komodo/config.py, tests/test_adapter_codex.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_adapter_codex tests.test_config -q
depends_on: [TSK-03.8.1]
context:
  - "profile local sets oss_provider ollama and a model_providers.ollama base_url in config.toml, and every tier's model from the profile"
type: feat
```

#### [TSK-03.8.4] Proof: the exit test under Codex [P: H] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "Proof: the exit test under Codex" CHANGELOG.md
depends_on: [TSK-03.8.1, TSK-03.8.2, TSK-03.8.3]
owner: human
context:
  - "komodo install --host codex with zero changes outside komodo/adapters, then run TG-03.9 with codex exec through the launcher; record the same numbers as TSK-03.5.3 under a Proof: the exit test under Codex heading in the 1.5.0 changelog entry"
type: docs
```

### [TG-03.9] Demolition and docs
```yaml
type: chore
version: 1.5.0
```
* **Why:** nothing from the V1 orchestrator survives, and the docs say what V1.5 is rather than what V1 was.

#### [TSK-03.9.1] The orchestrator goes [P: C] [READY]
```yaml
files: [komodo/pipeline.py, komodo/state.py, komodo/gates.py, komodo/pr.py, komodo/pr_actions.py, komodo/account.py, komodo/gitops.py, komodo/workers, tests/test_pipeline.py, tests/test_state.py, tests/test_gates.py, tests/test_pr.py, tests/test_account.py, tests/test_gitops.py, tests/test_workers.py]
done_when:
  - test ! -f komodo/pipeline.py
  - test ! -d komodo/workers
  - python3 scripts/verify.py
context:
  - "delete, do not stub; a test that imported a deleted module goes with it; verify must pass with nothing under komodo/ importing outside the standard library"
type: chore
```

#### [TSK-03.9.2] README and the templates describe what exists [P: H] [READY]
```yaml
files: [README.md, templates/project]
done_when:
  - python3 -m komodo doctor
depends_on: [TSK-03.9.1]
context:
  - "the README drops its planned status and keeps the vision, features, setup, usage, and layout; the roadmap becomes a changelog pointer"
  - "templates/project carries AGENTS.md, BACKLOG.md, CHANGELOG.md, and the example .komodo/context file, and nothing else"
type: docs
```

#### [TSK-03.9.3] Changelog 1.5.0 [P: M] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "## 1.5.0" CHANGELOG.md
depends_on: [TSK-03.9.2]
context:
  - "one heading, what was removed and what replaced it, from the README's roadmap and the deleted modules"
type: docs
```

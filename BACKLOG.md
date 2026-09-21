# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it. The V1.x backlog was dropped whole on 2026-09-21 in favour of the V2 plan in `docs/spec/v2.md`; the PR that landed the plan lists what it replaced.

---

## [EPIC-03] V2, the assembly line
*Goal: Komodo becomes rules, roles, standards, skills, and one guard that any agent host renders. The V1 orchestrator, its state, its account logic, and its Go hooks go. Claude Code is the default host, Codex is the second, and a fully local profile runs on Codex with Ollama. Spec: `docs/spec/v2.md`.*

* **Groups run in file order.** Every group carries `2.0.0`; the tag is cut once, after TG-03.7.
* **Until TG-03.4 lands, the V1 harness runs the groups.** After it, the `run` skill runs the rest, which is the first proof.

### [TG-03.1] Source reshape
```yaml
type: refactor
version: 2.0.0
```
* **Why:** three neutral formats are the whole portability story. Standards become skills so their trigger lives in their own frontmatter and no render step exists. Briefs fold into roles so a role is the brief. The git and destruction policy becomes data one guard reads on every host.

#### [TSK-03.1.1] Standards become skills at the source [P: C] [READY]
```yaml
files: [komodo/skills, komodo/standards, komodo/standards.py, tests/test_standards.py]
done_when:
  - test -f komodo/skills/standards-go/SKILL.md
  - test ! -d komodo/standards
  - python3 -m unittest discover -s tests -q
context:
  - "each komodo/standards/<x>.md moves to komodo/skills/standards-<x>/SKILL.md with frontmatter name and description; the description front-loads the extensions and directories that trigger it"
  - "the standard's body does not change; a standard names no live repo, port, URL, version, or path"
  - "komodo/standards.py and its test go; the adapter copies the skills directory as is"
type: refactor
```

#### [TSK-03.1.2] Briefs fold into roles [P: C] [READY]
```yaml
files: [komodo/roles, komodo/briefs, komodo/briefs.py, tests/test_roles.py, tests/test_briefs.py]
done_when:
  - test ! -d komodo/briefs
  - python3 -m unittest tests.test_roles -q
context:
  - "a role file carries frontmatter name, tier, tools, session, returns, and a body that is the brief template with {{task}}, {{files}}, {{context}}, {{standards}} placeholders"
  - "tools are the five Komodo verbs read, edit, write, shell, search; a host tool name never appears in a role"
  - "the JSON return schema each worker prompt carried moves beside the role as <role>.schema.json"
type: refactor
```

#### [TSK-03.1.3] The policy file and the rules that both hosts read [P: H] [READY]
```yaml
files: [komodo/policy.json, komodo/AGENTS.md, komodo/rules, tests/test_policy.py]
done_when:
  - python3 -c "import json; json.load(open('komodo/policy.json'))"
  - test -f komodo/AGENTS.md
  - python3 -m unittest tests.test_policy -q
context:
  - "policy.json holds protected refs, forbidden git verbs, forbidden trailers, destructive command patterns, and prod host markers; gitops.py PROTECTED and guard.go carry the current lists"
  - "komodo/rules/AGENTS.md moves to komodo/AGENTS.md; komodo/rules/accessibility.md holds the human-facing output contract from the Writing for a human section and is included into AGENTS.md at render"
  - "komodo/rules/cli.md goes; the CLI documents itself with --help"
type: feat
```

### [TG-03.2] Guard and inject
```yaml
type: feat
version: 2.0.0
```
* **Why:** two stdlib scripts replace a Go binary, two Python fallbacks, and two git hooks. Both hosts send the same payload fields and accept the same deny JSON, so one script serves both.

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

#### [TSK-03.2.3] The Go hooks, the binaries, and the git hooks go [P: H] [READY]
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

### [TG-03.3] Claude adapter and the CLI
```yaml
type: feat
version: 2.0.0
```
* **Why:** the adapter is the only Claude-specific code in the repo. Profiles move from the pipeline into config as a tier-to-model table per host, and the CLI shrinks to install, doctor, tasks, comments, and run.

#### [TSK-03.3.1] Profiles map a tier to a provider, model, and effort per host [P: C] [READY]
```yaml
files: [komodo/config.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_config -q
context:
  - "profiles claude, codex, local, hybrid as the spec tables them; each names a host; the reviewer tier may point at a different provider than the builder tier"
  - "~/.komodo/config.json overlays defaults per machine; a repo never carries one; account.py and every plan-derived cap go"
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
  - "renders CLAUDE.md with an @AGENTS.md include, agents from roles with model from the profile, skills copied as is, both hooks registered in settings, and the permissions convenience layer from policy.json"
  - "the five Komodo tool verbs map to Claude tool names here and nowhere else"
  - "validate measures the always-on context under 1500 tokens and the run skill under 1500 tokens"
type: feat
```

#### [TSK-03.3.3] The CLI is install, doctor, tasks, comments, and run [P: H] [READY]
```yaml
files: [komodo/cli.py, komodo/__main__.py, komodo/comments.py, komodo/comment_rules.py, tests/test_cli.py]
done_when:
  - python3 -m komodo --help
  - python3 -m unittest tests.test_cli -q
depends_on: [TSK-03.3.2]
context:
  - "install takes --host claude|codex and --profile; doctor, tasks, and comments keep their names; status, hooks, and every pipeline subcommand go"
  - "comment_rules.py becomes comments.py with the same lint"
type: refactor
```

### [TG-03.4] The run skill
```yaml
type: feat
version: 2.0.0
```
* **Why:** the line is a skill and the mechanics are commands. The 0.x skill loop failed on gate count, reviewer fan-out, verify-on-every-stop, and loop text size; every one of those has a named limit here, and the last task measures it.

#### [TSK-03.4.1] `komodo tasks next` and `komodo tasks done` [P: C] [READY]
```yaml
files: [komodo/tasks.py, komodo/dag.py, tests/test_tasks.py]
done_when:
  - python3 -m unittest tests.test_tasks -q
context:
  - "next --json prints the next READY group with its tasks, dependencies, waves by directory, context paths, and the group's type and version; nothing when no group is ready"
  - "done <task-id> rewrites only the status token; in-progress <task-id> likewise; dag.py folds into tasks.py"
  - "tasks plan, tasks migrate, and the worker call behind plan go"
type: feat
```

#### [TSK-03.4.2] The run, review, and backlog skills [P: C] [READY]
```yaml
files: [komodo/skills/run/SKILL.md, komodo/skills/review/SKILL.md, komodo/skills/backlog/SKILL.md]
done_when:
  - python3 scripts/validate.py
depends_on: [TSK-03.4.1]
context:
  - "run is the station table in docs/spec/v2.md as imperative steps: tasks next, scout, fill the builder template, one builder per task in a git worktree add with a .komodo/scope.json, merge in wave order and stop on conflict, done_when and comments check, one reviewer pass on the diff with a fresh context, one repair pass, commit, push, PR, tasks done, the accessibility report"
  - "the skill names no host tool, path, or flag; it says spawn the builder role and the adapter decides how"
  - "review is the reviewer role's session entry; backlog is the grammar with the add and lint commands"
type: feat
```

#### [TSK-03.4.3] The headless launcher [P: H] [READY]
```yaml
files: [komodo/run.py, tests/test_run.py]
done_when:
  - python3 -m unittest tests.test_run -q
depends_on: [TSK-03.4.1]
context:
  - "komodo run <group> scrubs push tokens, credential helpers, and SSH identities from the environment, picks the host from the profile, invokes the host's non-interactive mode with the run skill and the group id, and exits with the host's code"
  - "about 30 lines; gitops.worker_env is the prior art for the scrub"
type: feat
```

#### [TSK-03.4.4] Proof: one group under V1 and under the run skill [P: C] [READY]
```yaml
files: [docs/spec/v2-proof.md]
done_when:
  - test -f docs/spec/v2-proof.md
depends_on: [TSK-03.4.2, TSK-03.4.3]
owner: human
context:
  - "run TG-03.5 under the run skill in a Claude Code session; record wall time, tokens, and turns beside the V1 numbers for TG-03.3 from its run state"
  - "slower by more than one wave means the skill is wrong; fix the skill before TG-03.5 merges"
type: docs
```

### [TG-03.5] Codex adapter and the portability lint
```yaml
type: feat
version: 2.0.0
```
* **Why:** the second host proves the first was not special. Codex reads AGENTS.md and SKILL.md natively, takes agents as TOML, and registers the same hook events with the same deny JSON.

#### [TSK-03.5.1] The Codex adapter [P: C] [READY]
```yaml
files: [komodo/adapters/codex.py, tests/test_adapter_codex.py]
done_when:
  - python3 -m unittest tests.test_adapter_codex -q
context:
  - "renders ~/.codex/AGENTS.md, ~/.codex/agents/<role>.toml with name, description, developer_instructions, model, model_reasoning_effort, sandbox_mode from the profile, ~/.agents/skills as a copy, ~/.codex/hooks.json with both hooks, and the mcp_servers table in config.toml"
  - "the five Komodo tool verbs map to sandbox_mode and nothing else; Codex has no allow-list, so the guard is the whole policy there"
type: feat
```

#### [TSK-03.5.2] Doctor lints for host leakage [P: H] [READY]
```yaml
files: [komodo/doctor.py, tests/test_doctor.py]
done_when:
  - python3 -m unittest tests.test_doctor -q
  - python3 -m komodo doctor
context:
  - "a skill or role that names a host tool, a host path under ~/.claude or ~/.codex, or a host flag fails doctor; adapters are exempt"
  - "drift: the rendered layout differs from what the source would render now"
type: feat
```

#### [TSK-03.5.3] Proof: one group under Codex [P: H] [READY]
```yaml
files: [docs/spec/v2-proof.md]
done_when:
  - grep -q codex docs/spec/v2-proof.md
depends_on: [TSK-03.5.1, TSK-03.5.2]
owner: human
context:
  - "komodo install --host codex, then run TG-03.6 with codex exec through the launcher; record the same numbers as TSK-03.4.4"
type: docs
```

### [TG-03.6] Local models
```yaml
type: feat
version: 2.0.0
```
* **Why:** hybrid today, fully local later, without touching a role. On Codex every tier can run on Ollama; on Claude Code a light role calls the bridge.

#### [TSK-03.6.1] The local profile renders Codex with Ollama as the provider [P: H] [READY]
```yaml
files: [komodo/adapters/codex.py, komodo/config.py, tests/test_adapter_codex.py]
done_when:
  - python3 -m unittest tests.test_adapter_codex tests.test_config -q
context:
  - "profile local sets oss_provider ollama and a model_providers.ollama base_url in config.toml, and every tier's model from the profile"
type: feat
```

#### [TSK-03.6.2] The hybrid profile routes light roles through the bridge on Claude Code [P: M] [READY]
```yaml
files: [komodo/roles/summarizer.md, komodo/adapters/claude.py, tests/test_adapter_claude.py]
done_when:
  - python3 -m unittest tests.test_adapter_claude -q
context:
  - "a role whose profile provider is ollama on the claude host renders with a body that calls the komodo-ollama-bridge tool and a model of the profile's light tier for the wrapper"
  - "the bridge stays optional; a missing bridge degrades to the light tier and says so once"
type: feat
```

### [TG-03.7] Demolition and docs
```yaml
type: chore
version: 2.0.0
```
* **Why:** nothing from the V1 orchestrator survives, and the docs say what V2 is rather than what V1 was.

#### [TSK-03.7.1] The orchestrator goes [P: C] [READY]
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

#### [TSK-03.7.2] README, architecture, decisions, and the templates describe V2 [P: H] [READY]
```yaml
files: [README.md, docs/architecture.md, docs/design-decisions.md, docs/windows-install.md, templates/project]
done_when:
  - python3 -m komodo doctor
  - test ! -f docs/windows-install.md
depends_on: [TSK-03.7.1]
context:
  - "design-decisions rewrites The pipeline is code and Guarantees are credential isolation with the reasoning in docs/spec/v2.md; the rest is checked against what still exists"
  - "templates/project carries AGENTS.md, BACKLOG.md, CHANGELOG.md and nothing else"
type: docs
```

#### [TSK-03.7.3] Changelog 2.0.0 [P: M] [READY]
```yaml
files: [CHANGELOG.md]
done_when:
  - grep -q "## 2.0.0" CHANGELOG.md
depends_on: [TSK-03.7.2]
context:
  - "one heading, what was removed and what replaced it, from the Removed from V1 table in the spec"
type: docs
```

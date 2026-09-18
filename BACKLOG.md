# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it. Items tied to the pre-1.0 machinery were dropped in the 1.0.0 release; the PR that shipped it lists them.

---

## [EPIC-02] V1.1, harden the line
*Goal: prove the harness on real repos and close the gaps the first runs expose.*

### [TG-02.1] Worker providers
```yaml
type: feat
version: 1.1.0
```

#### [TSK-02.1.1] Ollama first-pass review before the Claude reviewer in the fast profile [P: M] [READY]
```yaml
files: [komodo/pipeline.py, komodo/workers/ollama.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: [docs/architecture.md#review]
```

#### [TSK-02.1.2] Per-worker wall-clock and cost caps surfaced as report warnings when a role runs over its spec [P: M] [READY]
```yaml
files: [komodo/pipeline.py, komodo/render.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
```

#### [TSK-02.1.3] Bridge: set num_ctx on chat requests so a large summarizer payload does not truncate silently [P: M] [DONE]
```yaml
files: []
done_when: []
owner: human
context: ["the bridge is its own repo at ~/komodo/ai/komodo-ollama-bridge; it sets num_ctx per agent, warns on a filled window, and banners a truncated tool result"]
```

### [TG-02.2] PR actions
```yaml
type: feat
version: 1.1.0
```

#### [TSK-02.2.1] Unit tests for pr_actions.sync and pr_actions.respond with a mocked gh and a fake worker [P: H] [READY]
```yaml
files: [tests/test_pr_actions.py, komodo/pr_actions.py]
done_when:
  - python3 -m unittest tests.test_pr_actions -q
```

#### [TSK-02.2.2] pr respond marks a thread resolved through GraphQL after replying [P: L] [READY]
```yaml
files: [komodo/pr.py, komodo/pr_actions.py]
done_when:
  - python3 -m unittest tests.test_pr_actions -q
depends_on: [TSK-02.2.1]
```

### [TG-02.3] CI and security gates
```yaml
type: ci
version: 1.1.0
```

#### [TSK-02.3.1] Secret scan and dependency scan in the verify gate, blocking on a verified credential or a High advisory [P: M] [BLOCKED]
```yaml
files: []
done_when: []
owner: human
context: ["the GitHub Actions workflow this task targeted was removed; scanning has to run in scripts/verify.py or in a hosted build that is not GitHub Actions", "komodo/standards/cicd.md#ci still mandates CI scanning, so the standard needs a recorded deviation or this repo needs a runner"]
```

### [TG-02.4] Planner quality
```yaml
type: feat
version: 1.1.0
```

#### [TSK-02.4.2] bug: Only exit codes 126/127 are treated as broken, missing shell syntax errors [P: M] [READY]
```yaml
files:
  - komodo/__main__.py
done_when:
  - /opt/homebrew/opt/python@3.14/bin/python3.14 scripts/verify.py
type: fix
context:
  - "review finding from 20260916-204608-tg-02-4: BROKEN_COMMAND_CODES = (126, 127) catches 'permission denied' and 'command not found', but a malformed shell command (unbalanced quote, bad redirection) typical"
```

#### [TSK-02.4.3] test-gap: No test proves a legitimately-failing done_when is kept, not dropped [P: M] [READY]
```yaml
files:
  - tests/test_cli.py
done_when:
  - /opt/homebrew/opt/python@3.14/bin/python3.14 scripts/verify.py
type: fix
context:
  - "review finding from 20260916-204608-tg-02-4: The entire design intent is that validate_done_when only filters commands that 'can't run' (126/127), not commands that run and fail (e.g. exit 1 because the fe"
```

#### [TSK-02.4.4] simplify: One scratch worktree is created and destroyed per proposed task [P: L] [READY]
```yaml
files:
  - komodo/__main__.py
done_when:
  - /opt/homebrew/opt/python@3.14/bin/python3.14 scripts/verify.py
type: fix
context:
  - "review finding from 20260916-204608-tg-02-4: validate_done_when is called once per planner-proposed item inside the cmd_plan loop, each call doing its own git worktree add / remove. A planner proposing N t"
```

#### [TSK-02.4.5] simplify: Dense one-line conditional expression [P: L] [READY]
```yaml
files:
  - komodo/__main__.py
done_when:
  - /opt/homebrew/opt/python@3.14/bin/python3.14 scripts/verify.py
type: fix
context:
  - "review finding from 20260916-204608-tg-02-4: The problems.append(...) statement nests a ternary, a %-format, and a method chain (splitlines()[-1]) on one line, well past the standard's guidance to wrap a l"
```





### [TG-02.5] Run hygiene
```yaml
type: fix
version: 1.1.0
```

#### [TSK-02.5.1] Commit only the paths a task declares or changed, so build artifacts never land [P: H] [READY]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_gitops -q
context: ["a live run on a fresh repo committed four .pyc files: gitops.commit stages with add -A after done_when has run in the worktree"]
```

#### [TSK-02.5.2] Preflight ensures the target repo ignores the .komodo directory before the first run writes to it [P: H] [READY]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: ["a live run committed .komodo/runs/<id>/state.json into the target repo; nothing in the harness writes or checks a gitignore and templates/project ships none"]
```

#### [TSK-02.5.3] A filed review finding points at the file that fixes it, not the artifact it was found in [P: M] [READY]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: ["the run filed a task whose files list was a .pyc path, which no builder could act on"]
```

#### [TSK-02.5.4] Commit subjects lowercase the title after the type and truncate on a word boundary [P: M] [READY]
```yaml
files: [komodo/gitops.py, tests/test_gitops.py]
done_when:
  - python3 -m unittest tests.test_gitops -q
context: ["commit_message produced 'feat: Add pkg/greet.py with a greet function returning \"hello <name>\"' truncated mid-word at 72 chars"]
```

#### [TSK-02.5.5] Reconsider the reviewer diff floor now that review dominates a small group's cost and wall clock [P: L] [READY]
```yaml
files: [komodo/config.py, docs/design-decisions.md, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_config -q
context: ["a 10-line diff cost 0.10 of 0.16 USD and 1m54s of a 2m09s run; the fast profile's 150-line floor turns review off for exactly this case"]
```

#### [TSK-02.5.6] Phase timing stores a duration, not the epoch second the phase ended [P: H] [READY]
```yaml
files: [komodo/state.py, komodo/render.py, tests/test_gates_state.py]
done_when:
  - python3 -m unittest tests.test_gates_state -q
context: ["the TG-02.4 report printed `verify | -194s`; state.phases holds absolute timestamps such as 1789609979 and render subtracts them in the wrong order"]
```

#### [TSK-02.5.7] Persist the run cost so a resumed or re-read state carries what the run spent [P: M] [READY]
```yaml
files: [komodo/state.py, komodo/pipeline.py, tests/test_gates_state.py]
done_when:
  - python3 -m unittest tests.test_gates_state tests.test_pipeline -q
context: ["the TG-02.6 report printed 2.44 USD while its state.json recorded cost_usd 0.0; only the per-worker rows survive a reload"]
```

#### [TSK-02.5.8] A commit the pre-commit hook refuses is a repair attempt, not a crashed builder [P: H] [READY]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_gitops -q
context: ["TSK-02.6.2 finished its work, then the comment lint refused the commit over an OVER_LINES finding; the task reported `builder crashed` and no repair pass ran"]
```

#### [TSK-02.5.9] doctor's stale-worktree check must not flag the main checkout when it runs inside a builder worktree [P: H] [READY]
```yaml
files: [komodo/doctor.py, komodo/gitops.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
context: ["TSK-02.6.3 blocked because doctor run from the tsk-02-6-3 worktree called the repo root a stale worktree; any done_when naming doctor fails inside a wave"]
```

#### [TSK-02.5.10] doctor fails when a changelog version heading has no tag, or a tag has no heading [P: H] [READY]
```yaml
files:
  - komodo/doctor.py
  - tests/test_doctor_install.py
done_when:
  - python3 -m unittest tests.test_doctor_install -q
context:
  - 0.50.0 and 0.51.0 shipped to main with no v-tag; v0.46.5 is tagged with no changelog heading; komodo release only reads the newest non-Unreleased heading so it can never see an older gap
type: fix
```

### [TG-02.6] Defects found reading the tree
```yaml
type: fix
version: 1.1.0
```

#### [TSK-02.6.2] publish picks the first pull request template it finds, not the last [P: M] [BLOCKED]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
```

#### [TSK-02.6.3] Retire the claude-code references the rules now forbid [P: M] [BLOCKED]
```yaml
files: [CHANGELOG.md, komodo/doctor.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
  - python3 -m komodo doctor
context: ["AGENTS.md forbids a claude-code directory; CHANGELOG.md names it twelve times and doctor whitelists the prefix instead of flagging it"]
```

#### [TSK-02.6.4] `tasks migrate` deletes every task body, mis-splits four-column subtask tables, and misreads valid commands as prose [P: H] [DONE]
```yaml
files: [komodo/tasks.py, tests/test_tasks.py]
done_when:
  - python3 -m unittest tests.test_tasks -q
  - python3 -m unittest tests.test_cli -q
context: ["Three defects in migrate(), all reachable from `tasks migrate --write`. (1) Data loss: the loop consumes every line to the next # heading and emits only the heading plus a yaml block, so Acceptance Criteria, User Story, Evidence/Approach/Non-goals tables and the subtask table are deleted with no warning and no backup — --write makes it unrecoverable outside git. (2) LEGACY_ROW has three capture groups but the documented subtask table has four columns (Subtask, Category, Work, Done when); the trailing \\|\\s*$ anchor forces group(3) to span Work AND Done when, so every migrated done_when reads `Build the thing | `go build ./...` with the trailing backtick stripped from the wrong end. (3) COMMAND_HINT omits grep, !, and ENV=VALUE prefixes, so a cell holding `grep -q x f && go test ./...` or `TEST_TIER=component go test ./...` is filed as done_when_prose. Repro: a fixture with one task carrying ACs, a field table and a four-column subtask table returns only the heading and an empty done_when. Fix: preserve the body verbatim below the block, split the row on unescaped pipes and take the last cell, and widen COMMAND_HINT. Found while making komodo-auth-api and komodo-forge-sdk-go harness-runnable (2026-09-17); both were converted with a one-off script instead."]
```

### [TG-02.7] Go rewrite
```yaml
type: refactor
version: 1.1.0
```

#### [TSK-02.7.1] Port the orchestrator to a compiled Go binary that runs natively with no interpreter [P: M] [READY]
```yaml
files: [docs/design-decisions.md, AGENTS.md, BACKLOG.md]
done_when:
  - python3 -m komodo doctor
context: ["same functions as komodo/*.py, compiled and executed on the native machine instead of shelling to python3", "AGENTS.md pins the harness to stdlib Python 3.9; this task supersedes that rule and the decision record has to say so", "a shipped binary has to stay zero-setup for the user: no toolchain install, no build step on their machine", "scope the port and the cutover here; the per-module work becomes its own epic"]
```

#### [TSK-02.7.2] Render the prebuilt guard binary in komodo install and fall back to guard.py when no target matches [P: M] [DONE]
```yaml
files: [komodo/install.py, komodo/adapters/claude/__init__.py, komodo/adapters/claude/settings.policy.json, tests/test_hooks.py]
done_when:
  - python3 -m unittest tests.test_hooks -q
  - python3 scripts/verify.py
context: ["the Go source and prebuilt binaries already live under komodo/adapters/claude/hooks/; nothing renders them yet", "the adapter render loop copies only .py and writes text, so a binary needs a separate binary-safe copy path", "guard.py stays as the fallback for any platform without a committed binary"]
```

#### [TSK-02.7.3] Compile every session hook for every platform so install needs no interpreter [P: M] [DONE]
```yaml
files: [komodo/install.py, komodo/adapters/claude/__init__.py, scripts/build-hooks.py, tests/test_hooks.py]
done_when:
  - python3 -m unittest tests.test_hooks -q
  - python3 scripts/verify.py
context: ["one binary with a guard and an inject subcommand replaces two scripts, so the target matrix costs one artifact per platform", "context_injector.py is ported to Go and both implementations are held to the same test cases", "install hard-failed on a missing python because a hook was still a script; the check now names the scripts that need one"]
```

### [TG-02.8] Standards drift
```yaml
type: docs
version: 1.1.0
```

#### [TSK-02.8.1] Record this repo's no-CI deviation in komodo/standards/cicd.md or give it a runner [P: M] [READY]
```yaml
files: [komodo/standards/cicd.md, docs/design-decisions.md]
done_when:
  - python3 -m komodo doctor
context: ["cicd.md requires an ephemeral runner per PR and an OS matrix; this repo now runs its gate only on the developer machine via the pre-push hook", "the account is on a GitHub Free plan and the matrix was the cost driver"]
```

---

## [EPIC-03] Repo-level context
*Goal: a repo injects its own context, standards, and extensions into the harness without forking the toolkit, and the global render stays identical on every machine.*

### [TG-03.1] Override root and scoped context
```yaml
type: feat
version: 1.3.0
```

#### [TSK-03.1.1] Move STANDARD_PATHS out of the claude adapter into standards.py [P: H] [READY]
```yaml
files: [komodo/standards.py, komodo/adapters/claude/__init__.py, tests/test_roles_adapter.py]
done_when:
  - python3 -m unittest tests.test_roles_adapter -q
  - python3 scripts/verify.py
context: ["STANDARD_PATHS lives in komodo/adapters/claude/__init__.py and maps a standard name to the globs it fires on", "doctor and the resolver both need the map and neither may import an adapter, so it belongs beside names_for", "pure move: the adapter imports it from standards.py and the rendered skill frontmatter is unchanged"]
```

#### [TSK-03.1.2] komodo install renders a global layer that does not vary by repo [P: H] [READY]
```yaml
files: [komodo/__main__.py, komodo/install.py, komodo/adapters/claude/__init__.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
  - python3 scripts/verify.py
context: ["cmd_install loads the repo's config and passes it to render_agents, which writes the active profile's model and effort into ~/.claude/agents/*.md", "installing from a repo on the local profile therefore writes ollama models into the global layer and the next repo inherits them", "agents render from config.DEFAULTS; per-repo model choice stays a harness concern and never reaches ~/.claude", "add a test that two different repo configs produce byte-identical renders"]
```

#### [TSK-03.1.3] .komodo/context/*.md injects repo context into a worker by matching task files [P: H] [READY]
```yaml
files: [komodo/repo_context.py, komodo/pipeline.py, komodo/briefs/builder.prompt.md, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
  - python3 scripts/verify.py
depends_on: [TSK-03.1.1]
context: ["today repo_rules() reads the whole AGENTS.md into every brief, so an Auth API and an SDK in one tree get each other's context", "each file carries a paths: frontmatter key in the same grammar as STANDARD_PATHS and loads only when a task file matches", "a context file with no paths: key is an error, not a default of **, because the always-on budget depends on scoping", "AGENTS.md keeps its slot for what is true everywhere in the repo"]
```

#### [TSK-03.1.4] .komodo/standards extends a shipped standard and adds ones the toolkit never shipped [P: H] [READY]
```yaml
files: [komodo/standards.py, komodo/config.py, tests/test_standards.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_standards -q
  - python3 -m unittest tests.test_config -q
  - python3 scripts/verify.py
depends_on: [TSK-03.1.1]
context: ["overrides extend, never replace: no code path may return a repo file instead of a toolkit file", ".komodo/standards/<name>.extra.md appends after the shipped standard under a rendered '## Repo additions' boundary", ".komodo/standards/<newname>.md has no toolkit twin and loads as a parallel standard with its own paths:", "a repo file whose basename collides with a shipped standard is a doctor error naming the .extra.md form", "komodo.json grows standards.extensions and standards.paths, additive only, so a repo can map .cbl to a cobol standard it wrote"]
```

#### [TSK-03.1.5] The override root is shareable and validated without exposing run state [P: H] [READY]
```yaml
files: [.gitignore, komodo/doctor.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
  - python3 -m komodo doctor
  - python3 scripts/verify.py
depends_on: [TSK-03.1.4]
context: [".gitignore ignores .komodo/ wholesale, so standards and context written there can never be committed or shared", "negate rather than narrow: .komodo/* then !.komodo/standards/ and !.komodo/context/ keeps local.json, runs/ and wt/ ignored with no new entries", "git will not descend into an excluded directory, so the pattern must be .komodo/* and not .komodo/ for the negation to take effect", "doctor.py SKIP_DIRS must keep skipping .komodo for the tree walk; validate the two override directories by direct glob so runs/ stays unreachable", "checks: an unknown standard name, a paths: glob matching nothing, a basename colliding with a shipped standard"]
```

#### [TSK-03.1.6] The rendered skill body resolves the repo delta at read time [P: M] [READY]
```yaml
files: [komodo/adapters/claude/__init__.py, komodo/adapters/claude/hooks/context_injector.py, scripts/validate.py, tests/test_roles_adapter.py]
done_when:
  - python3 -m unittest tests.test_roles_adapter -q
  - python3 scripts/verify.py
depends_on: [TSK-03.1.3]
context: ["Claude Code resolves skills enterprise > personal > project, so a project .claude/skills/ entry is shadowed by the global one and a repo cannot override a global skill by name", "unification therefore happens inside the global skill body, which stays identical in every repo and names the repo paths to check", "context_injector emits a delta line at SessionStart only when a delta exists, so a repo adopting none of this prints exactly what it prints today", "validate.py counts a name-only skill as tokens(name) + 2, so a longer body costs nothing against the 1500 always-on budget"]
```

### [TG-03.2] Exclusion and the drift gate
```yaml
type: feat
version: 1.4.0
```

#### [TSK-03.2.1] A repo excludes a shipped standard, with a floor it cannot reach [P: M] [REFINEMENT]
```yaml
files: [komodo/standards.py, komodo/config.py, tests/test_standards.py]
done_when:
  - python3 -m unittest tests.test_standards -q
context: ["deliberately held in refinement until a non-Komodo repo exists to design against, so the shape is decided by real COBOL and .NET rather than a guess", "a denylist, not an allowlist: an import list silently excludes every standard the toolkit ships after it is written", "names_for must filter ROLE_EXTRA too or reviewer still pulls api-security and builder still pulls comments past the exclusion", "the floor is comments, api-security and sdlc; growing it needs a CHANGELOG line so it stays deliberate", "exclusion is absolute in a worker because the file is never read, and advisory in a session because the global skill always loads"]
```

#### [TSK-03.2.2] Doctor fails when an exclusion has drifted away from the code [P: M] [REFINEMENT]
```yaml
files: [komodo/doctor.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
depends_on: [TSK-03.2.1]
context: ["the worst failure mode is silent: exclude react, add .tsx files a year later, nobody rereads komodo.json", "fail, do not warn, when an excluded standard's glob matches tracked files and no .komodo/standards/<name>*.md exists", "git ls-files against the glob map moved in TSK-03.1.1 is the whole check"]
```

#### [TSK-03.2.3] The session advisory and the worker filter are held to one precedence [P: L] [REFINEMENT]
```yaml
files: [komodo/adapters/claude/__init__.py, scripts/validate.py, tests/test_roles_adapter.py]
done_when:
  - python3 -m unittest tests.test_roles_adapter -q
depends_on: [TSK-03.2.1]
context: ["render_skills exists so a session and a worker hold the same rules; exclusion makes that true only in workers", "a fixture config renders the skill procedure and calls names_for, and the test asserts both name the same standards", "the reviewer worker runs with the repo's resolved set, so code a session wrote against an excluded standard is still caught at review"]
```

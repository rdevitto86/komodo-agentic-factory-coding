# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it. Items tied to the pre-1.0 machinery were dropped in the 1.0.0 release; the PR that shipped it lists them.

---

## [EPIC-02] V1.1, harden the line
*Goal: prove the harness on real repos and close the gaps the first runs expose.*

### [TG-02.12] Run safety: the commit chain
```yaml
type: fix
version: 1.4.0
```
* **Why:** every task here edits the harness that is running it. The waves are ordered so no two tasks in one wave touch the same file, because a wave merges each worktree back into the run branch and an overlap is a merge conflict, not a merge.
* **Before running:** freeze the hooks for the duration, since `core.hooksPath` is absolute and every builder commit executes the main checkout's copy: `cp -R komodo/hooks /tmp/komodo-hooks-frozen && git config core.hooksPath /tmp/komodo-hooks-frozen`, restored to `"$PWD/komodo/hooks"` after.

#### [TSK-02.6.5] `run` lints the whole backlog before it resolves the group, and preflight never checks that .komodo is ignored [P: H] [READY]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
  - python3 -m unittest tests.test_tasks -q
context:
  - "`Pipeline.preflight` lints the entire backlog and raises on any problem, and only then resolves the requested group, so one group missing `version: x.y.z` blocks `run` for every other group. Repro: komodo-auth-api carries 15 groups with no version key and no single group can be run. Fix: resolve the group first, then block only on problems scoped to that group plus the genuinely file-global ones - parse failures, duplicate ids, dependency edges pointing outside the file - and report the rest as warnings."
  - "the same function is where the .komodo gitignore check belongs, folded in from TSK-02.5.2: a live run committed .komodo/runs/<id>/state.json into the target repo, because nothing writes or checks a gitignore and templates/project ships none. Both changes edit preflight, so they are one task rather than a merge conflict."
type: fix
```

#### [TSK-02.5.1] Commit only the paths a task declares or changed, so build artifacts never land [P: H] [READY]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py, tests/test_gitops.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_gitops -q
depends_on: [TSK-02.6.5]
context:
  - "a live run on a fresh repo committed four .pyc files: gitops.commit stages with add -A because the pipeline never passes paths="
  - "the compile gate now writes bytecode to a pycache prefix outside the tree, so it is no longer a source of stray .pyc; a builder's own tooling still is"
type: fix
```

#### [TSK-02.6.3] Retire the claude-code references the rules now forbid [P: M] [READY]
```yaml
files: [CHANGELOG.md, komodo/doctor.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
  - python3 -m komodo doctor
context:
  - "AGENTS.md forbids a claude-code directory; CHANGELOG.md names it twelve times and doctor whitelists the prefix instead of flagging it"
  - "this was BLOCKED because doctor called the main checkout a stale worktree from inside a builder worktree; that is fixed, so the doctor done_when now passes in a wave"
type: fix
```

#### [TSK-02.5.3] A filed review finding points at the file that fixes it, not the artifact it was found in [P: M] [READY]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
depends_on: [TSK-02.5.1]
context: ["the run filed a task whose files list was a .pyc path, which no builder could act on"]
type: fix
```

#### [TSK-02.5.4] Commit subjects lowercase the title after the type and truncate on a word boundary [P: M] [READY]
```yaml
files: [komodo/gitops.py, tests/test_gitops.py]
done_when:
  - python3 -m unittest tests.test_gitops -q
depends_on: [TSK-02.5.1]
context: ["commit_message produced 'feat: Add pkg/greet.py with a greet function returning \"hello <name>\"' truncated mid-word at 72 chars"]
type: fix
```

### [TG-02.13] Run safety: the report
```yaml
type: fix
version: 1.4.0
```
* **Why:** neither task touches `pipeline.py` or `gitops.py`, so this group shares no file with TG-02.12 and a bad run there costs nothing here.

#### [TSK-02.5.6] Phase timing stores a duration, not the epoch second the phase ended [P: H] [READY]
```yaml
files: [komodo/state.py, komodo/render.py, tests/test_gates_state.py]
done_when:
  - python3 -m unittest tests.test_gates_state -q
context:
  - "the TG-02.4 report printed `verify | -194s`; state.phases holds absolute timestamps and render pairs each phase with the next key in dict order"
  - "a resumed run re-marks a phase it already recorded, which overwrites the timestamp but keeps the original insertion position, so dict order stops being chronological and the subtraction goes negative"
type: fix
```

#### [TSK-02.8.1] Record this repo's no-CI deviation in komodo/standards/cicd.md or give it a runner [P: M] [READY]
```yaml
files: [komodo/standards/cicd.md, docs/design-decisions.md]
done_when:
  - python3 -m komodo doctor
context:
  - "cicd.md requires an ephemeral runner per PR and an OS matrix; this repo now runs its gate only on the developer machine via the pre-push hook"
  - "the account is on a GitHub Free plan and the matrix was the cost driver"
type: docs
```

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

#### [TSK-02.3.2] Workers run under the guard: ship it as a project-level hook the worker session loads [P: H] [READY]
```yaml
files:
  - komodo/workers/claude.py
  - komodo/install.py
  - tests/test_hooks.py
done_when:
  - python3 -m unittest tests.test_hooks -q
context:
  - "workers spawn with --setting-sources project and --dangerously-skip-permissions, so ~/.claude never loads and guard.py governs sessions only; a worker can cd out of its worktree into the main checkout, or commit with --no-verify, unchecked. Verified 2026-09-18 that a PreToolUse hook in a project .claude/settings.json still fires and still denies under --dangerously-skip-permissions: the hook ran, the command never executed, and the CLI recorded a permission_denials entry. worker_env() already strips every push credential, so nothing reaches the remote"
type: fix
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

#### [TSK-02.5.5] Reconsider the reviewer diff floor now that review dominates a small group's cost and wall clock [P: L] [READY]
```yaml
files: [komodo/config.py, docs/design-decisions.md, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_config -q
context: ["a 10-line diff cost 0.10 of 0.16 USD and 1m54s of a 2m09s run; the fast profile's 150-line floor turns review off for exactly this case"]
```

#### [TSK-02.5.7] Persist the run cost so a resumed or re-read state carries what the run spent [P: M] [BLOCKED]
```yaml
files: [komodo/state.py, komodo/pipeline.py, tests/test_gates_state.py]
done_when:
  - python3 -m unittest tests.test_gates_state tests.test_pipeline -q
context: ["the TG-02.6 report printed 2.44 USD while its state.json recorded cost_usd 0.0; only the per-worker rows survive a reload"]
```

#### [TSK-02.5.8] A commit the pre-commit hook refuses is a repair attempt, not a crashed builder [P: H] [DONE]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_gitops -q
context: ["TSK-02.6.2 finished its work, then the comment lint refused the commit over an OVER_LINES finding; the task reported `builder crashed` and no repair pass ran"]
```

#### [TSK-02.5.9] doctor's stale-worktree check must not flag the main checkout when it runs inside a builder worktree [P: H] [DONE]
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

#### [TSK-02.6.6] `komodo/__init__.py` pins `__version__` at 1.1.0 while the changelog has released past it [P: L] [REFINEMENT]
* `komodo status` and `komodo --version` print a version the repo left behind; decide whether the version is read from the changelog or bumped by the release command.

### [TG-02.10] No repo config file
```yaml
type: refactor
version: 1.3.0
```
* **Why:** `komodo.json` was a required-looking file that carried nothing. This repo's copy was fifteen keys, all fifteen identical to `komodo/config.py` DEFAULTS. Its only load-bearing read was the git guard's `protected` list, whose built-in fallback is the same list. An agent refused valid work because the file was absent, which is the whole cost of having it.

#### [TSK-02.10.1] Delete `komodo.json` and every read of it [P: H] [DONE]
```yaml
files: [komodo/config.py, komodo/gitops.py, komodo/pr_actions.py, komodo/__main__.py, komodo/adapters/claude/hooks/guard.py, komodo/hooks/src/repo.go, tests/test_config.py]
done_when:
  - python3 scripts/verify.py
context: ["defaults live in komodo/config.py, and a machine may overlay .komodo/config.json, which the toolkit owns and gitignores", "both guards read the toolkit path instead, and the Go binaries are rebuilt from source", "gitops no longer tells the caller to set base in a config file; it names what it looked for and points at --base", "the repo's own komodo.json and templates/project/komodo.json.tmpl are deleted, so a new repo is never handed one"]
```

### [TG-02.9] Account-aware limits
```yaml
type: feat
version: 1.3.0
```
* **Why:** every cap in `config.py` DEFAULTS was a fixed dollar figure, but a `claude.ai` subscription does not meter dollars. Verified against the installed binary: a run's envelope reports `"costBasis": "list"`, so `total_cost_usd` on a subscription is a list-price estimate and a dollar ceiling stops a worker for a reason the account never charged.
* **Measured on komodo-auth-api, TG-01.10, 4 runs / 30 worker calls / 113.9M input tokens:** builder input scales as `≈1400 × turns²` (every task within ±20%, from 17 turns / 486K to 88 turns / 10.9M). Cost is quadratic in turns, so a turn saved early is worth far more than a dollar cap raised late.
* **What the CLI actually exposes:** `claude auth status` prints `loggedIn`, `authMethod`, `apiProvider` and, on a subscription, `subscriptionType` — one of `pro`, `max`, `team`, `enterprise`. It does **not** print `rateLimitTier`, so `max_5x` cannot be told from `max_20x`. `--output-format stream-json` emits a `rate_limit_event` carrying live `utilization` and `resetsAt` for the `five_hour` and `seven_day` windows; that is the quantity a subscription actually meters.

#### [TSK-02.9.1] Read the account at preflight and expose it on the config [P: H] [DONE]
```yaml
files: [komodo/account.py, komodo/config.py, tests/test_account.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_account tests.test_config -q
context: ["the probe runs once per process and caches on the Config; a missing binary, a non-zero exit, a timeout or unparseable output all yield an undetected account", "undetected keeps today's behaviour, dollar cap included, rather than failing the run", "the email and orgId the command also returns are never read or logged", "`account.detect: false` and `account.plan` in the toolkit's own `.komodo/config.json` pin a plan for an offline or deliberate run"]
```

#### [TSK-02.9.2] Derive the caps from the plan, and drop the dollar cap when dollars are not metered [P: H] [DONE]
```yaml
files: [komodo/account.py, komodo/config.py, komodo/workers/claude.py, tests/test_account.py, tests/test_workers_briefs.py]
done_when:
  - python3 -m unittest tests.test_account tests.test_workers_briefs -q
depends_on: [TSK-02.9.1]
context: ["turn caps are solved from the measured law rather than guessed: turns = sqrt(plan_input_budget x tier_share / 1400), so headroom is granted in tokens", "pro and unknown get 4M input tokens at standard tier (53 turns, close to the old 60); max gets 12M (92 turns)", "an explicitly set turn cap still wins, because derivation only fills an unset one", "an error names which ceiling bound and what the worker had spent, so a raise is not a guess"]
```

#### [TSK-02.9.3] The plan sets the model ceiling, for harness workers and rendered session agents alike [P: H] [DONE]
```yaml
files: [komodo/account.py, komodo/config.py, tests/test_config.py, tests/test_roles_adapter.py]
done_when:
  - python3 -m unittest tests.test_config tests.test_roles_adapter -q
depends_on: [TSK-02.9.1]
context: ["a Pro account running the thinking profile would otherwise spend its allowance on Opus at heavy tier", "pro, team and unknown cap at sonnet; max and enterprise reach opus", "the ceiling only downgrades, never upgrades, and an unrecognised model name is left alone", "the claude adapter already renders agent frontmatter from config.role, so a session agent inherits the same ceiling", "`account.model_ceiling: false` turns it off"]
```

#### [TSK-02.9.4] A worker killed by the timeout keeps its accounting [P: H] [DONE]
```yaml
files: [komodo/workers/claude.py, komodo/workers/base.py, tests/test_workers_briefs.py]
done_when:
  - python3 -m unittest tests.test_workers_briefs -q
context: ["the old subprocess.run plus --output-format json emitted one envelope at the very end, and the TimeoutExpired branch never read stdout, so a real 900s burn recorded 0 turns and $0.00", "Popen plus --output-format stream-json accumulates turns and tokens per assistant event, and a watchdog kill returns what the worker had already spent", "stream-json requires --verbose, and a prompt past the pipe buffer needs its own writer thread or the read deadlocks"]
```

#### [TSK-02.9.5] The run reads the rate-limit stream and stops before a window it would 429 into [P: H] [DONE]
```yaml
files: [komodo/workers/claude.py, komodo/pipeline.py, komodo/state.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
depends_on: [TSK-02.9.4]
context: ["every worker result carries the newest rate_limit_event, and the run state keeps the last one so a resume sees it", "before each wave the pipeline logs a window past rate_limit.warn_at and stops cleanly past rate_limit.pause_at, naming the window and its reset time", "an account already in overage is never held back", "stopping leaves the run resumable instead of burning turns into a refusal"]
```

#### [TSK-02.9.6] `context.file_chars` becomes a per-file budget, not a pool split across the task's files [P: C] [DONE]
```yaml
files: [komodo/config.py, komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: ["the old code divided one 24000-char budget by len(task.files) with a 2000 floor, so a builder saw less of its own code the more files its task named", "measured on TG-01.10: TSK-01.10.4 saw 29% of its five files, .17 38% of six, .2 44% of four; the 12-file task that failed three times saw roughly 20%", "what the brief truncates, the builder re-reads by tool call, and turns cost quadratically, so paying input once per file is cheaper", "context.per_file_chars is 10000 with a 120000 total ceiling, so a task naming twenty files still cannot blow the window"]
```

#### [TSK-02.9.7] The second attempt inherits the first attempt's diff, not just its error string [P: M] [DONE]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: ["_build_task and _build_single loop over two attempts sharing one worktree, but briefs.render was called fresh and build_argv passes --no-session-persistence, so attempt 2 re-derived from cold what attempt 1 had written", "measured: attempt 2 of TSK-01.10.2 cost more than attempt 1 (57 turns / 5.3M input against 51 / 5.0M) despite inheriting its files", "6 of 30 worker calls across the four runs failed, burning $14.61 - 36% of total spend - for nothing", "a file the attempt created is untracked, so the diff needs an intent-to-add first"]
```

#### [TSK-02.9.8] The worker timeout scales with the task and the plan [P: M] [DONE]
```yaml
files: [komodo/account.py, komodo/config.py, tests/test_account.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_account tests.test_pipeline -q
depends_on: [TSK-02.9.1]
context: ["worker_timeout_s was a flat 900 regardless of how much code a task named", "the timeout now scales with the bytes behind task.files and the plan's headroom, capped by worker_timeout_max_s", "the done_when gate gets the same scaled timeout, so a long build is not killed by the check that proves it"]
```

#### [TSK-02.9.9] `komodo status` reports the plan and the caps actually in force [P: M] [DONE]
```yaml
files: [komodo/__main__.py, tests/test_cli.py]
done_when:
  - python3 -m unittest tests.test_cli -q
depends_on: [TSK-02.9.2]
context: ["the caps a run will use were only discoverable by reading config.py DEFAULTS and reasoning about which tier each role maps to", "status now prints the plan, how it is metered, the scaled timeouts, and one row per role with model, effort, turn cap and dollar cap", "--json carries the same report under a limits key"]
```

### [TG-02.11] Repo-scoped task ids
```yaml
type: refactor
version: 1.4.0
```
* **Why:** `EPIC-`, `TG-` and `TSK-` are the same three literals in every repo that runs the harness, so two backlogs in one conversation collide and no id says which project it came from. A Jira-style per-repo key — `AITK-1` for an epic, `AITK-1.1` for a group, `AITK-1.1.6` for a task — makes every id self-identifying.
* **Not ready to build.** The prefix today also encodes the level: `TSK-` means task. Under one key the *depth* has to carry that instead, and every consumer below assumes the literal. This group is scoping only — none of it is estimated, and the open questions outnumber the answers.

#### [TSK-02.11.1] Decide where a repo's key comes from, now that no repo config file exists [P: H] [REFINEMENT]
* The obvious home was `komodo.json`, which is deleted, and `.komodo/config.json` is gitignored so a key written there would not reach a clone.
* **Candidates:** the backlog declares its own key in a header line, so the file that uses the ids also defines them; or derive it from the git remote or directory name, so nothing is declared at all; or keep a literal default and let the key be optional.
* Deriving it makes a rename of the repo silently renumber every id, which argues for declaring it. Needs a decision before anything below can be specified.

#### [TSK-02.11.2] Survey every place the three literals are hardcoded [P: H] [REFINEMENT]
* `komodo/tasks.py` — `EPIC_HEADING`, `GROUP_HEADING`, `TASK_HEADING`, `LEGACY_TASK`, `LEGACY_ROW`; `next_task_id` slices `group.id[len("TG-"):]`; `depends_on` is recovered with a bare ``re.findall(r"TSK-[\w.]+")``.
* `komodo/adapters/claude/hooks/context_injector.py` and its Go twin `komodo/hooks/src/inject.go` carry the same two regexes, so a change needs the binaries rebuilt.
* `komodo/rules/backlog.md` is the grammar the always-on skill renders, `komodo/rules/cli.md` shows `TG-01.1` in its examples, and `templates/project/BACKLOG.md.tmpl` seeds six ids into every new repo.

#### [TSK-02.11.3] Resolve the ambiguity one key introduces [P: H] [REFINEMENT]
* With three literals the level is read off the prefix. With one key it has to come from segment count, so `AITK-1.1` is a group only because depth is fixed at three.
* That forbids a fourth level forever, and it makes a malformed id parse as a different level rather than fail. The heading depth (`##`, `###`, `####`) already carries the level and could be the authority instead, with the id purely an address.
* Decide which of the two is authoritative before writing a regex.

#### [TSK-02.11.4] Plan the migration of a backlog already in flight [P: M] [REFINEMENT]
* Ids appear in committed history: branch names (`feat/<slug>`), run state under `.komodo/runs/<id>-<group>/`, commit messages, PR titles, and `depends_on` edges.
* A rewrite breaks a resume, and `tasks.py` already carries a legacy path for the retired `SUB-` row grammar, which is the precedent for how long both forms have to be read.
* Needs a decision on whether the old literals stay readable forever or for one release.

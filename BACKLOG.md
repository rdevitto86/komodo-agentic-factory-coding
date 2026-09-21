# Project Backlog

Priority `[P: C|H|M|L]`. Status `[REFINEMENT|READY|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it. Items tied to the pre-1.0 machinery were dropped in the 1.0.0 release; the PR that shipped it lists them. The 2026-09-21 assessment dropped every item it did not name; that PR lists those.

---

## [EPIC-02] V1.x, close the line
*Goal: fix what the first real runs exposed, so a session and a worker stop handing routine work back to a human, and V1 can be put to bed.*

* **Groups run in file order.** Each group carries its own patch version because preflight tags a merged version the moment the next run starts, so two groups sharing one version would tag the second one's work late.
* **Hand work before the next run:** delete `.komodo/runs/20260921-132738-tg-02-12`, whose state still holds `TSK-02.6.5` as `IN_PROGRESS` and would be resumed into. The install was re-rendered on 2026-09-21, PRs 95 and 100 are closed, and the first cut of `TSK-02.6.5` is kept as the tag `salvage/tsk-02-6-5` after its branch was lost from the remote.

### [TG-02.15] Account and limits
```yaml
type: fix
version: 1.4.0
```
* **Why:** `claude auth status` reports `subscriptionType: pro` on a Max account. `~/.claude.json` carries `oauthAccount.organizationType: claude_max` and `organizationRateLimitTier: default_claude_max_5x` for the same login. The harness trusted the wrong field, capped a Max account at Pro budgets and a sonnet ceiling, and rendered every session agent at sonnet. Detection stays per machine so a Pro account on another machine gets Pro caps with no config.

#### [TSK-02.15.1] The plan is read from `~/.claude.json` first, and `claude auth status` is the fallback [P: C] [READY]
```yaml
files: [komodo/account.py, komodo/config.py, tests/test_account.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_account tests.test_config -q
context:
  - "read only oauthAccount.organizationType and oauthAccount.organizationRateLimitTier from ~/.claude.json; the file also holds an email and account ids that are never read or logged"
  - "organizationType claude_max, claude_pro, claude_team, claude_enterprise map to the existing PLANS; the rate tier names the multiplier, so max_5x and max_20x get distinct PLAN_INPUT_BUDGET rows and the note that they cannot be told apart is wrong"
  - "auth status stays as the fallback when the file is absent or the keys are missing; a disagreement between the two sources is logged once with both values, and the file wins"
  - "the config directory comes from CLAUDE_CONFIG_DIR when set, else ~/.claude.json beside ~/.claude; a pinned account.plan still overrides both"
type: fix
```

#### [TSK-02.15.2] Preflight reads the cached usage windows so a run never starts into a window it will pause in [P: H] [READY]
```yaml
files: [komodo/account.py, komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_account tests.test_pipeline -q
depends_on: [TSK-02.15.1]
context:
  - "rate_limit_hold only learns the windows after the first worker returns its rate_limit_event; the run that started at 84% of the seven-day window found out one wave in"
  - "~/.claude.json cachedUsageUtilization carries five_hour and seven_day utilization as percentages with fetchedAtMs; seed state.rate_limit from it at preflight, in the unifiedWindows shape rate_limit_hold already reads, when the fetch is under an hour old"
  - "the same pause_at and warn_at apply; a run past pause_at refuses before it branches, naming the window and its reset time, and --resume is not needed because nothing started"
type: fix
```

#### [TSK-02.15.3] Ollama roles get no plan-derived turn cap, model ceiling, or plan time factor [P: M] [READY]
```yaml
files: [komodo/account.py, komodo/config.py, tests/test_account.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_account tests.test_config -q
depends_on: [TSK-02.15.1]
context:
  - "apply_to_spec fills max_turns and worker_timeout multiplies by PLAN_TIME_FACTOR for every provider, so a local profile inherits a Claude plan's shape for no reason"
  - "provider ollama: no max_turns, no capped_model, timeout scales by task bytes only; the group budget still applies"
  - "komodo status prints unlimited for the turn column of an ollama role"
type: fix
```

#### [TSK-02.15.4] `komodo status` names the plan source and the usage windows, and `rate_limit.wait` is removed [P: M] [READY]
```yaml
files: [komodo/__main__.py, komodo/config.py, tests/test_cli.py]
done_when:
  - python3 -m unittest tests.test_cli tests.test_config -q
depends_on: [TSK-02.15.2, TSK-02.15.3]
context:
  - "the account line says which source the plan came from (config file, auth status, pinned) so a misdetection is visible before a run is paid for"
  - "one line per usage window with its percentage and reset time when the cache is fresh"
  - "rate_limit.wait is in DEFAULTS and read nowhere; delete it rather than implement it"
type: fix
```

### [TG-02.16] Defects found in the assessment
```yaml
type: fix
version: 1.4.1
```
* **Why:** four confirmed bugs, each reachable from a normal run or a normal PR action, plus the version pin the CLI prints.

#### [TSK-02.16.1] `pr sync` and `pr respond` run the merger and responder briefs under their own role spec and schema [P: H] [READY]
```yaml
files: [komodo/pr_actions.py, tests/test_pr_actions.py]
done_when:
  - python3 -m unittest tests.test_pr_actions -q
context:
  - "_worker_call is called with role builder for both, so the brief is rendered for merger or responder but the schema, tools and tier are the builder's; the merger's resolved and escalate keys never survive the builder schema, so sync treats every conflict as resolved, and respond posts an empty reply"
  - "pass the real role: merger for sync, responder for respond; both roles exist under komodo/roles with session: false"
  - "tests with a fake worker and a mocked gh: an escalated file aborts the merge, a DECLINED thread changes no code and posts the reply, a CHANGED thread that fails done_when is reverted"
type: fix
```

#### [TSK-02.16.2] A task worktree whose directory is gone still clears its stale branch before the wave starts [P: H] [READY]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py, tests/test_gitops.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_gitops -q
context:
  - "_worktree_for removes the branch only inside the isdir(path) branch; an interrupted run that lost .komodo/wt/<task> but kept <branch>-tsk-x-y-z makes worktree add -b fail, which is caught as builder crashed and blocks the task on resume"
  - "worktree_remove already handles a missing directory; call it whenever the branch exists and is unprotected, then prune"
  - "the stale branch is the harness's own naming, so deleting it is safe; a branch that does not match the run branch prefix is never touched"
type: fix
```

#### [TSK-02.16.3] A verify failure pushes the run branch and opens a draft pull request instead of stranding the work [P: H] [READY]
```yaml
files: [komodo/pipeline.py, komodo/pr.py, komodo/render.py, tests/test_pipeline.py, tests/test_render.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_render -q
depends_on: [TSK-02.16.2]
context:
  - "publish returns early with 'keeps the work uncommitted' when verify failed, but every task commit is already on the run branch; a 77-minute, 16-call run left ten commits unpushed on a local branch"
  - "on verify failure: still commit the close-out, push the branch, open the PR as a draft with a Blocked section naming the failing gate and its output tail, and label it; the run still exits non-zero"
  - "pr.create already takes draft; render.pr_sections gains the blocked-gate section; the report wording says pushed as draft, never uncommitted"
type: fix
```

#### [TSK-02.16.4] Preflight snapshots the git hooks when the repo is the toolkit itself [P: M] [READY]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
depends_on: [TSK-02.16.3]
context:
  - "core.hooksPath is absolute and points into komodo/hooks, so every builder commit runs the main checkout's hook while a task may be editing it; TG-02.12 carried a manual freeze instruction for this"
  - "when hooksPath resolves inside the repo root, preflight copies it to .komodo/hooks-frozen/<run_id> and sets hooksPath there for the run; finish restores the original value, and a crash leaves a note in the report naming the command to restore it"
  - "this is why sessions built PRs 97 to 100 by hand; once it is in code the rule that a session runs the harness has no exception"
type: fix
```

#### [TSK-02.16.5] `komodo --version` and `status` print the released version read from the changelog [P: L] [READY]
```yaml
files: [komodo/__init__.py, komodo/__main__.py, tests/test_cli.py]
done_when:
  - python3 -m unittest tests.test_cli -q
context:
  - "__version__ is pinned at 1.1.0 while the changelog has released 1.3.0; nothing bumps it"
  - "read the newest numbered heading from CHANGELOG.md beside the package at import, keep the literal only as the fallback when the file is absent"
type: fix
```

### [TG-02.12] Run safety: the commit chain
```yaml
type: fix
version: 1.4.2
```
* **Why:** every task here edits the harness that is running it. The waves are ordered so no two tasks in one wave touch the same file, because a wave merges each worktree back into the run branch and an overlap is a merge conflict, not a merge. `TSK-02.16.4` freezes the hooks for the run, so no manual step precedes it. It runs ahead of TG-02.17, TG-02.18 and TG-02.14 because `gitops.commit` still stages with `add -A` until `TSK-02.5.1` lands, so every group below it can commit a build artifact.

#### [TSK-02.6.5] `run` lints the whole backlog before it resolves the group, and preflight never checks that .komodo is ignored [P: H] [READY]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
  - python3 -m unittest tests.test_tasks -q
context:
  - "`Pipeline.preflight` lints the entire backlog and raises on any problem, and only then resolves the requested group, so one group missing `version: x.y.z` blocks `run` for every other group. Repro: komodo-auth-api carries 15 groups with no version key and no single group can be run. Fix: resolve the group first, then block only on problems scoped to that group plus the genuinely file-global ones - parse failures, duplicate ids, dependency edges pointing outside the file - and report the rest as warnings."
  - "the same function is where the .komodo gitignore check belongs, folded in from TSK-02.5.2: a live run committed .komodo/runs/<id>/state.json into the target repo, because nothing writes or checks a gitignore and templates/project ships none. Check the machine-local children (runs/, wt/, config.json, *.jsonl), not the directory, because EPIC-03 commits context and standards under .komodo/."
  - "a first cut of this task exists on the pushed branch fix/run-safety-the-commit-chain-tsk-02-6-5, commit 82e0612; start from it"
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
version: 1.4.3
```

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

### [TG-02.17] Windows
```yaml
type: fix
version: 1.4.4
```
* **Why:** the harness claims three platforms and tests one. The verify gate cannot run under `cmd.exe` today, and no test would notice.

#### [TSK-02.17.1] The verify and compile gate commands are quoted for the shell that will run them [P: H] [READY]
```yaml
files: [komodo/gates.py, tests/test_gates_state.py]
done_when:
  - python3 -m unittest tests.test_gates_state -q
context:
  - "resolve_verify and compile_commands wrap sys.executable, the pycache prefix and the compileall skip pattern in shlex.quote, which yields single quotes for any Windows path; cmd.exe passes the quotes through and the interpreter is not found"
  - "one helper: on nt, subprocess.list2cmdline for a token, POSIX shlex.quote otherwise; run_command already branches on os.name the same way"
  - "tests patch os.name and sys.executable to a C: path with a space and assert the exact command string for both shells"
type: fix
```

#### [TSK-02.17.2] The hook, gate, and worker-environment tests run their Windows branches on every platform [P: H] [READY]
```yaml
files: [tests/test_hooks.py, tests/test_gates_state.py, tests/test_gitops.py, tests/test_cli.py]
done_when:
  - python3 -m unittest tests.test_hooks tests.test_gates_state tests.test_gitops tests.test_cli -q
depends_on: [TSK-02.17.1]
context:
  - "test_hooks has no Windows case; the whole suite has three os.name references, so the nt branches in gates.run_command, __main__._cannot_execute and install.rewrite_hook_commands are untested"
  - "patch os.name and os.sep rather than skip: the 9009 exit code and the not-recognized text path, the py -3 interpreter prefix, the .exe binary name chosen by host_binary, backslashed task paths in dag.dirs"
  - "the git hook sh stubs are exercised through the MINGW uname branch by feeding a fake uname on PATH"
type: test
```

#### [TSK-02.17.3] `docs/windows-install.md` describes the compiled hooks and the current install [P: M] [READY]
```yaml
files: [docs/windows-install.md]
done_when:
  - python3 -m komodo doctor
context:
  - "the page says install copies claude-code\\ and that the hook stubs hand off to a Python hook; both predate the compiled binaries and the rendered adapter"
  - "state what needs Python (the harness) and what does not (the session and git hooks), and that the stubs prefer komodo-hooks-windows-<arch>.exe"
type: docs
```

#### [TSK-02.8.1] Record this repo's no-CI deviation in komodo/standards/cicd.md or give it a runner [P: M] [READY]
```yaml
files: [komodo/standards/cicd.md, docs/design-decisions.md]
done_when:
  - python3 -m komodo doctor
context:
  - "cicd.md requires an ephemeral runner per PR and an OS matrix; this repo runs its gate only on the developer machine via the pre-push hook, so Windows and Linux never execute the suite"
  - "the account is on a GitHub Free plan and the matrix was the cost driver; a matrix that runs only on a release tag is the cheap middle"
type: docs
```

### [TG-02.18] Install drift and posture
```yaml
type: fix
version: 1.4.5
```
* **Why:** a session cannot tell that its rendered rules are older than the source, and the rules it does hold tell it to ask before acting. Both produced the `komodo.json` refusal and the permission churn.

#### [TSK-02.18.1] The render marker records the source commit and SessionStart reports when the install is behind it [P: H] [READY]
```yaml
files: [komodo/install.py, komodo/adapters/claude/__init__.py, komodo/adapters/claude/hooks/context_injector.py, komodo/hooks/src/inject.go, tests/test_doctor_install.py, tests/test_hooks.py]
done_when:
  - python3 -m unittest tests.test_doctor_install tests.test_hooks -q
  - python3 scripts/verify.py
context:
  - "the installed skills and hooks date from 2026-09-18 while komodo/rules changed on 2026-09-21; the stale skill still named komodo.json and an agent refused work over it"
  - "install writes the toolkit's HEAD commit and the toolkit path into .komodo-rendered; inject compares that commit with the toolkit checkout's HEAD when the path still exists and prints one line: install is behind komodo/rules, run python3 -m komodo install"
  - "both the Python and Go injectors emit the identical line; rebuild the binaries with python3 scripts/build-hooks.py"
type: feat
```

#### [TSK-02.18.2] The agent rules state the posture: act by default, ask for three things only [P: H] [READY]
```yaml
files: [komodo/rules/AGENTS.md, komodo/rules/cli.md]
done_when:
  - python3 scripts/validate.py
  - python3 -m komodo doctor
context:
  - "replace 'Ask only when blocked on a decision only the user can make, or before an irreversible or shared action' with: a reversible action on the branch never needs permission, including commit, push and opening a PR; ask only before merging into a protected branch, deleting something git does not hold, or a change to the spec; state an assumption in one line and continue"
  - "keep 'Read freely, write on a directive' as written; it is the user's verb gate. Replace 'Recommend before rewriting' with 'Prefer the smallest diff', which says the same without inviting a proposal round"
  - "add: no config file is ever required; a refusal that names a missing file is a toolkit bug to report in one line, never a reason to stop"
  - "add under Working: never list options you will not take, never restate the plan before doing it"
  - "the always-on budget in scripts/validate.py must still pass"
type: docs
```

#### [TSK-02.18.3] `komodo doctor` runs in a consuming repo and flags dead skill names and the legacy backlog legend [P: M] [READY]
```yaml
files: [komodo/doctor.py, komodo/__main__.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
context:
  - "doctor only knows the toolkit tree; the Auth API's AGENTS.md cites a generate-repo skill and its BACKLOG.md cites backlog-modify and a [TODO] legend, none of which exist, and an agent followed them"
  - "outside the toolkit: check backticked skill names against the rendered set, the backlog legend against the current statuses, and any core.hooksPath that is not the toolkit's hooks directory; skip the policy and role checks that only apply here"
  - "the toolkit path is found from the installed marker written in TSK-02.18.1, or KOMODO_ROOT"
type: feat
```

### [TG-02.14] Command safety: one decision point, many enforcement points
```yaml
type: refactor
version: 1.5.0
```
* **Why:** `docs/design-decisions.md#command-safety-is-one-decision-point-and-many-enforcement-points` carries the reasoning. The guard fuses policy with enforcement, duplicates the rules in two languages, and reaches only one provider's hook. Day to day it refuses `rm` of any shape, `git checkout -b`, `git branch -D`, and a grep whose pattern names a flag, so a session hands each of those to the human.
* **Sequencing:** `TSK-02.3.2` is the gate. Until a project-scoped hook governs workers, every mode below is a session-only feature, because a worker spawns with `--dangerously-skip-permissions` and loads neither the settings nor the hooks in them.
* Both guard implementations move together and the committed binaries under `komodo/hooks/bin/` are rebuilt with `python3 scripts/build-hooks.py`.

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

#### [TSK-02.14.1] The rules become a declarative ruleset both languages read [P: H] [READY]
```yaml
files:
  - komodo/policy/rules.json
  - komodo/policy.py
  - tests/test_policy.py
done_when:
  - python3 -m unittest tests.test_policy -q
context:
  - "today PROTECTED, DESTRUCTIVE, TRAILER and SEGMENT_SPLIT are constants in guard.py and again in guard.go, so a rule change is an edit in two languages plus a rebuild of six binaries"
  - "JSON because it is stdlib in Python and Go and the harness imports nothing else; every rule carries an id and a human reason so a refusal is traceable to a line rather than to a string in a binary"
  - "this task only introduces the ruleset and the loader; nothing reads it yet, so the guards are untouched and stay green"
type: refactor
```

#### [TSK-02.14.2] `decide(request) -> Decision` is the one decision point [P: H] [READY]
```yaml
files:
  - komodo/policy.py
  - tests/test_policy.py
done_when:
  - python3 -m unittest tests.test_policy -q
depends_on: [TSK-02.14.1]
context:
  - "a request names tool, command, cwd and mode; a decision is allow, deny or ask, carrying the rule id that produced it"
  - "pure and stdlib, no I/O beyond loading the ruleset, so it is tested once and reused by every enforcement point"
  - "port every case the current guards cover, and hold the new engine to the existing tests/test_hooks.py cases before anything switches over"
type: feat
```

#### [TSK-02.14.3] `guard.mode` selects the posture, with a floor the repo cannot loosen [P: H] [READY]
```yaml
files:
  - komodo/config.py
  - komodo/policy.py
  - tests/test_policy.py
  - tests/test_config.py
done_when:
  - python3 -m unittest tests.test_policy tests.test_config -q
depends_on: [TSK-02.14.2]
context:
  - "safe refuses a whole verb wherever a destructive shape exists and fails closed when the engine errors; default refuses the destructive shape and allows the recoverable one; unsafe keeps the hook and drops the local JSON allow and deny lists"
  - "`.komodo/config.json` is gitignored and writable by any file tool, so the floor lives in `~/.komodo/policy.json` outside the repo and a repo may tighten it but never loosen it; the settings policy also denies Edit and Write on that path as a second layer"
  - "guard.py:149 currently catches every internal error and returns without a decision; fail behaviour becomes a property of the mode"
  - "`komodo status` prints the mode in force and where the floor came from"
type: feat
```

#### [TSK-02.14.4] Both guards call the decision point instead of carrying their own rules [P: H] [READY]
```yaml
files:
  - komodo/adapters/claude/hooks/guard.py
  - komodo/hooks/src/guard.go
  - tests/test_hooks.py
done_when:
  - python3 -m unittest tests.test_hooks -q
  - python3 scripts/verify.py
depends_on: [TSK-02.14.3]
context:
  - "the stdin payload to stdout decision contract does not change; only the middle does, so the hook wiring and the rendered settings stay as they are"
  - "the Go side reads the same rules.json rather than re-implementing it, which is the whole point of the ruleset"
  - "rebuild the binaries and confirm a denial still names its reason to the caller"
type: refactor
```

#### [TSK-02.14.5] Scope the branch and delete refusals to the shapes that actually lose work [P: H] [READY]
```yaml
files:
  - komodo/policy/rules.json
  - tests/test_policy.py
done_when:
  - python3 -m unittest tests.test_policy -q
depends_on: [TSK-02.14.4]
context:
  - "a forced branch delete is recoverable from the reflog and the harness discards its own `<branch>-tsk-xx-y-z` worktree branches on every wave, so refusing -D outright costs more than it protects"
  - "under default, a delete inside the worktree is allowed, and so is git checkout -b and git checkout <new-branch>, which discard nothing; checkout of a path or of a branch over dirty work stays refused"
  - "outside the worktree, or of the repo root, a delete stays refused at every mode"
  - "once the rules are data this is a ruleset edit, not a code change in two languages"
type: fix
```

#### [TSK-02.14.6] The engine splits a command on shell metacharacters before it parses quotes [P: H] [READY]
```yaml
files:
  - komodo/policy.py
  - tests/test_policy.py
done_when:
  - python3 -m unittest tests.test_policy -q
depends_on: [TSK-02.14.2]
context:
  - "SEGMENT_SPLIT is applied to the raw command string, so a pipe inside a quoted argument starts a new segment and the text after it is parsed as a fresh command"
  - "verified twice on 2026-09-21: a read-only grep whose pattern carried an alternation naming the refused flags was refused, and so was a heredoc whose document body quoted them. Neither could delete anything; one only reads and the other only writes a markdown file"
  - "scan with quote awareness so a metacharacter inside single or double quotes is a literal, and never inspect heredoc body lines as commands"
  - "mode-independent: a false positive is wrong under safe too"
type: fix
```

#### [TSK-02.14.7] Every denial is logged with its rule and command [P: M] [READY]
```yaml
files:
  - komodo/policy.py
  - tests/test_policy.py
done_when:
  - python3 -m unittest tests.test_policy -q
depends_on: [TSK-02.14.3]
context:
  - "appended to `.komodo/runs/<id>/policy.jsonl`, falling back to `.komodo/policy.jsonl` when no run is active, because a session denial is the common case"
  - "without a log there is no answer to what the agent attempted, which is the first question an enterprise review asks"
type: feat
```

#### [TSK-02.14.8] The settings allow and deny lists are rendered from the ruleset per mode [P: H] [READY]
```yaml
files:
  - komodo/adapters/claude/__init__.py
  - komodo/adapters/claude/settings.policy.json
  - komodo/policy.py
  - tests/test_roles_adapter.py
done_when:
  - python3 -m unittest tests.test_roles_adapter tests.test_policy -q
  - python3 scripts/verify.py
depends_on: [TSK-02.14.3]
context:
  - "settings.policy.json is hand-kept and today denies `rm *` outright while the guard only refuses rm -rf, so the JSON and the guard disagree and the stricter one wins silently"
  - "each rule in rules.json says which permission entries it implies; render derives the allow and deny lists for the mode in force, and settings.policy.json keeps only the entries no rule owns (web domains, secret reads)"
  - "a test asserts every verb the ruleset refuses appears in the rendered deny list for safe and that unsafe renders no Bash entries at all"
type: refactor
```

#### [TSK-02.14.9] An env-prefixed command is judged by its stripped form and allowed when the allow list names it [P: H] [READY]
```yaml
files:
  - komodo/policy.py
  - komodo/adapters/claude/hooks/guard.py
  - komodo/hooks/src/guard.go
  - tests/test_policy.py
  - tests/test_hooks.py
done_when:
  - python3 -m unittest tests.test_policy tests.test_hooks -q
  - python3 scripts/verify.py
depends_on: [TSK-02.14.4]
context:
  - "the client matches an allow rule on the raw command start, so `TEST_TIER=component go test ./...` never matches `go test:*`; the Auth API's local settings hold 147 accumulated allow entries and most are this shape"
  - "the guard already strips VAR=value tokens; when the stripped command matches a rendered allow entry, the hook answers permissionDecision allow, which the client honours without a prompt"
  - "an env assignment that names a credential variable or PATH is never stripped and falls through to ask"
type: feat
```

#### [TSK-02.14.10] Ollama gets an enforcement point only when the bridge grows tools [P: L] [REFINEMENT]
* `komodo/workers/ollama.py` is text in, text out, no tools, so there is nothing to intercept and no mode applies to it today.
* The interception point would be the bridge's MCP layer, in its own repo, which is currently unreachable and whose distribution is deferred.
* Needs the bridge to carry tools before this can be specified at all.

#### [TSK-02.14.11] Decide whether an agent may merge its own green pull request under `default` [P: H] [REFINEMENT]
* Today `gh pr merge` is denied and the rule reads "landing is the human's merge button". The user asked for less manual effort unless a human is genuinely needed.
* **Candidates:** keep the rule at every mode; allow under `default` when checks pass and the PR was opened by the same run; allow under `unsafe` only.
* A human decision; once made it is one rule in `rules.json` and one line in `komodo/rules/AGENTS.md`.

## [EPIC-03] Repo-level context
*Goal: a repo injects its own context, standards, permissions, and extensions into the harness without forking the toolkit, and the global render stays identical on every machine.*

* **Precedence, lowest to highest:** `komodo/config.py` defaults; `~/.komodo/policy.json`, the human floor, which can only tighten; the committed repo layer under `.komodo/context`, `.komodo/standards`, `.komodo/permissions.json`, which extends and never replaces; `.komodo/config.json`, the gitignored machine overlay; command-line flags.
* **Non-blocking:** a malformed override file is skipped with one note in the run report and one line at SessionStart. No layer raises `PipelineError`, and no layer is required to exist.

### [TG-03.1] Override root and scoped context
```yaml
type: feat
version: 1.6.0
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
context: ["cmd_install loads the repo's config and passes it to render_agents, which writes the active profile's model and effort into ~/.claude/agents/*.md", "installing from a repo on the local profile therefore writes ollama models into the global layer and the next repo inherits them", "agents render from config.DEFAULTS with the account's model ceiling still applied, because the ceiling is per machine and not per repo; per-repo profile choice stays a harness concern and never reaches ~/.claude", "add a test that two different repo configs produce byte-identical renders"]
```

#### [TSK-03.1.3] .komodo/context/*.md injects repo context into a worker by matching task files [P: H] [READY]
```yaml
files: [komodo/repo_context.py, komodo/pipeline.py, komodo/briefs/builder.prompt.md, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
  - python3 scripts/verify.py
depends_on: [TSK-03.1.1]
context: ["today repo_rules() reads the whole AGENTS.md into every brief, so an Auth API and an SDK in one tree get each other's context", "each file carries a paths: frontmatter key in the same grammar as STANDARD_PATHS and loads only when a task file matches", "a context file with no paths: key is skipped with a report note, not a default of **, because the always-on budget depends on scoping", "AGENTS.md keeps its slot for what is true everywhere in the repo"]
```

#### [TSK-03.1.4] .komodo/standards extends a shipped standard and adds ones the toolkit never shipped [P: H] [READY]
```yaml
files: [komodo/standards.py, komodo/config.py, tests/test_standards.py, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_standards -q
  - python3 -m unittest tests.test_config -q
  - python3 scripts/verify.py
depends_on: [TSK-03.1.1]
context: ["overrides extend, never replace: no code path may return a repo file instead of a toolkit file", ".komodo/standards/<name>.extra.md appends after the shipped standard under a rendered '## Repo additions' boundary", ".komodo/standards/<newname>.md has no toolkit twin and loads as a parallel standard with its own paths:", "a repo file whose basename collides with a shipped standard is a doctor error naming the .extra.md form", "the file's own paths: frontmatter is the whole map entry; there is no central registry to add to and no config file to carry one", "names_for already merges BY_EXTENSION, BY_PATH_PART and BY_BASENAME, so a repo standard joins the same result list rather than a second resolution path"]
```

* **Worked example, a React + C# + .NET + COBOL tree on Windows.** Three of the four already resolve: `App.tsx` gets `typescript, react, ui-web`, `Order.cs` gets `csharp, dotnet`, `Api.csproj` gets `dotnet`. `PAYROLL.cbl` gets nothing but `comments`, because the toolkit ships no COBOL standard and never will ship one for every language. The repo writes two files and commits them:
```
.komodo/standards/cobol.md     paths: ["**/*.cbl", "**/*.cob", "**/*.cpy"]
.komodo/context/mainframe.md   paths: ["src/batch/**"]
```
  Nothing else changes: no fork of the toolkit, no config file, no PR against the toolkit repo, and the global render stays byte-identical to every other machine's.

#### [TSK-03.1.5] The override root is shareable: the machine children are ignored, the directory is not [P: H] [READY]
```yaml
files: [.gitignore, templates/project/.gitignore.tmpl, komodo/doctor.py, komodo/pipeline.py, tests/test_doctor_install.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_doctor_install tests.test_pipeline -q
  - python3 -m komodo doctor
  - python3 scripts/verify.py
depends_on: [TSK-03.1.4]
context: [".gitignore ignores .komodo/ wholesale, so standards and context written there can never be committed or shared", "ignore the children instead: .komodo/runs/, .komodo/wt/, .komodo/hooks-frozen/, .komodo/config.json, .komodo/*.jsonl; the templates/project starter ships the same five lines", "preflight's ignore check from TSK-02.6.5 checks those children and refuses only when a run would commit state; it never demands the directory be ignored", "doctor.py SKIP_DIRS keeps skipping .komodo for the tree walk; validate the two override directories by direct glob so runs/ stays unreachable", "checks: an unknown standard name, a paths: glob matching nothing, a basename colliding with a shipped standard"]
```

#### [TSK-03.1.6] The rendered skill body resolves the repo delta at read time [P: M] [READY]
```yaml
files: [komodo/adapters/claude/__init__.py, komodo/adapters/claude/hooks/context_injector.py, komodo/hooks/src/inject.go, scripts/validate.py, tests/test_roles_adapter.py]
done_when:
  - python3 -m unittest tests.test_roles_adapter -q
  - python3 scripts/verify.py
depends_on: [TSK-03.1.3]
context: ["Claude Code resolves skills enterprise > personal > project, so a project .claude/skills/ entry is shadowed by the global one and a repo cannot override a global skill by name", "unification therefore happens inside the global skill body, which stays identical in every repo and names the repo paths to check", "both injectors emit a delta line at SessionStart only when a delta exists, so a repo adopting none of this prints exactly what it prints today; rebuild the binaries", "validate.py counts a name-only skill as tokens(name) + 2, so a longer body costs nothing against the 1500 always-on budget"]
```

#### [TSK-03.1.7] `.komodo/permissions.json` declares a repo's own allow entries and `komodo install --project` renders them [P: H] [READY]
```yaml
files: [komodo/install.py, komodo/adapters/claude/__init__.py, komodo/__main__.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
  - python3 scripts/verify.py
depends_on: [TSK-03.1.5]
context: ["a repo's test and build commands are its own; the Auth API accumulated 147 personal allow entries because nothing shared them", "the committed file holds allow entries only; a deny or a hook there is refused by doctor, because the floor lives in ~/.komodo/policy.json and a repo may tighten but never loosen", "install --project writes them into .claude/settings.json under the permissions key and leaves every other key alone, the same merge the global install does; workers load project settings, so the entries reach them too"]
```

### [TG-03.2] Exclusion and the drift gate
```yaml
type: feat
version: 1.7.0
```

#### [TSK-03.2.1] A repo excludes a shipped standard, with a floor it cannot reach [P: M] [REFINEMENT]
* Deliberately held in refinement until a non-Komodo repo exists to design against, so the shape is decided by real COBOL and .NET rather than a guess.
* A denylist, not an allowlist: an import list silently excludes every standard the toolkit ships after it is written. `names_for` must filter `ROLE_EXTRA` too or the reviewer still pulls `api-security` past the exclusion.
* The floor is `comments`, `api-security` and `sdlc`. Exclusion is absolute in a worker because the file is never read, and advisory in a session because the global skill always loads.

#### [TSK-03.2.2] Doctor fails when an exclusion has drifted away from the code [P: M] [REFINEMENT]
* The worst failure mode is silent: exclude react, add `.tsx` files a year later, nobody rereads the exclusion.
* Fail, do not warn, when an excluded standard's glob matches tracked files and no `.komodo/standards/<name>*.md` exists. `git ls-files` against the glob map moved in `TSK-03.1.1` is the whole check.

#### [TSK-03.2.3] The session advisory and the worker filter are held to one precedence [P: L] [REFINEMENT]
* `render_skills` exists so a session and a worker hold the same rules; exclusion makes that true only in workers.
* A fixture config renders the skill procedure and calls `names_for`, and the test asserts both name the same standards.

# Project Backlog

Priority `[P: C|H|M|L]`. Status `[TODO|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it. Items tied to the pre-1.0 machinery were dropped in the 1.0.0 release; the PR that shipped it lists them.

---

## [EPIC-02] V1.1, harden the line
*Goal: prove the harness on real repos and close the gaps the first runs expose.*

### [TG-02.1] Worker providers
```yaml
type: feat
```

#### [TSK-02.1.1] Ollama first-pass review before the Claude reviewer in the fast profile [P: M] [TODO]
```yaml
files: [komodo/pipeline.py, komodo/workers/ollama.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: [docs/architecture.md#review]
```

#### [TSK-02.1.2] Per-worker wall-clock and cost caps surfaced as report warnings when a role runs over its spec [P: M] [TODO]
```yaml
files: [komodo/pipeline.py, komodo/render.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
```

#### [TSK-02.1.3] Bridge: set num_ctx on generate requests so a large summarizer payload does not truncate silently [P: M] [BLOCKED]
```yaml
files: []
done_when: []
owner: human
context: ["the bridge source and its prompt files live outside this repo under the user's .komodo/bridge deploy"]
```

### [TG-02.2] PR actions
```yaml
type: feat
```

#### [TSK-02.2.1] Unit tests for pr_actions.sync and pr_actions.respond with a mocked gh and a fake worker [P: H] [TODO]
```yaml
files: [tests/test_pr_actions.py, komodo/pr_actions.py]
done_when:
  - python3 -m unittest tests.test_pr_actions -q
```

#### [TSK-02.2.2] pr respond marks a thread resolved through GraphQL after replying [P: L] [TODO]
```yaml
files: [komodo/pr.py, komodo/pr_actions.py]
done_when:
  - python3 -m unittest tests.test_pr_actions -q
depends_on: [TSK-02.2.1]
```

### [TG-02.3] CI and security gates
```yaml
type: ci
```

#### [TSK-02.3.1] Secret scan and dependency scan in the verify workflow, blocking on a verified credential or a High advisory [P: M] [TODO]
```yaml
files: [.github/workflows/verify.yml]
done_when:
  - python3 -c "import yaml" 2>/dev/null || python3 -c "print('yaml parse skipped')"
  - test -f .github/workflows/verify.yml
context: [komodo/standards/cicd.md#ci]
```

### [TG-02.4] Planner quality
```yaml
type: feat
```

#### [TSK-02.4.2] bug: Only exit codes 126/127 are treated as broken, missing shell syntax errors [P: M] [TODO]
```yaml
files:
  - komodo/__main__.py
done_when:
  - /opt/homebrew/opt/python@3.14/bin/python3.14 scripts/verify.py
type: fix
context:
  - "review finding from 20260916-204608-tg-02-4: BROKEN_COMMAND_CODES = (126, 127) catches 'permission denied' and 'command not found', but a malformed shell command (unbalanced quote, bad redirection) typical"
```

#### [TSK-02.4.3] test-gap: No test proves a legitimately-failing done_when is kept, not dropped [P: M] [TODO]
```yaml
files:
  - tests/test_cli.py
done_when:
  - /opt/homebrew/opt/python@3.14/bin/python3.14 scripts/verify.py
type: fix
context:
  - "review finding from 20260916-204608-tg-02-4: The entire design intent is that validate_done_when only filters commands that 'can't run' (126/127), not commands that run and fail (e.g. exit 1 because the fe"
```

#### [TSK-02.4.4] simplify: One scratch worktree is created and destroyed per proposed task [P: L] [TODO]
```yaml
files:
  - komodo/__main__.py
done_when:
  - /opt/homebrew/opt/python@3.14/bin/python3.14 scripts/verify.py
type: fix
context:
  - "review finding from 20260916-204608-tg-02-4: validate_done_when is called once per planner-proposed item inside the cmd_plan loop, each call doing its own git worktree add / remove. A planner proposing N t"
```

#### [TSK-02.4.5] simplify: Dense one-line conditional expression [P: L] [TODO]
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
```

#### [TSK-02.5.1] Commit only the paths a task declares or changed, so build artifacts never land [P: H] [TODO]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_gitops -q
context: ["a live run on a fresh repo committed four .pyc files: gitops.commit stages with add -A after done_when has run in the worktree"]
```

#### [TSK-02.5.2] Preflight ensures the target repo ignores the .komodo directory before the first run writes to it [P: H] [TODO]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: ["a live run committed .komodo/runs/<id>/state.json into the target repo; nothing in the harness writes or checks a gitignore and templates/project ships none"]
```

#### [TSK-02.5.3] A filed review finding points at the file that fixes it, not the artifact it was found in [P: M] [TODO]
```yaml
files: [komodo/pipeline.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: ["the run filed a task whose files list was a .pyc path, which no builder could act on"]
```

#### [TSK-02.5.4] Commit subjects lowercase the title after the type and truncate on a word boundary [P: M] [TODO]
```yaml
files: [komodo/gitops.py, tests/test_gitops.py]
done_when:
  - python3 -m unittest tests.test_gitops -q
context: ["commit_message produced 'feat: Add pkg/greet.py with a greet function returning \"hello <name>\"' truncated mid-word at 72 chars"]
```

#### [TSK-02.5.5] Reconsider the reviewer diff floor now that review dominates a small group's cost and wall clock [P: L] [TODO]
```yaml
files: [komodo/config.py, docs/design-decisions.md, tests/test_config.py]
done_when:
  - python3 -m unittest tests.test_config -q
context: ["a 10-line diff cost 0.10 of 0.16 USD and 1m54s of a 2m09s run; the fast profile's 150-line floor turns review off for exactly this case"]
```

#### [TSK-02.5.6] Phase timing stores a duration, not the epoch second the phase ended [P: H] [TODO]
```yaml
files: [komodo/state.py, komodo/render.py, tests/test_gates_state.py]
done_when:
  - python3 -m unittest tests.test_gates_state -q
context: ["the TG-02.4 report printed `verify | -194s`; state.phases holds absolute timestamps such as 1789609979 and render subtracts them in the wrong order"]
```

#### [TSK-02.5.7] Persist the run cost so a resumed or re-read state carries what the run spent [P: M] [TODO]
```yaml
files: [komodo/state.py, komodo/pipeline.py, tests/test_gates_state.py]
done_when:
  - python3 -m unittest tests.test_gates_state tests.test_pipeline -q
context: ["the TG-02.6 report printed 2.44 USD while its state.json recorded cost_usd 0.0; only the per-worker rows survive a reload"]
```

#### [TSK-02.5.8] A commit the pre-commit hook refuses is a repair attempt, not a crashed builder [P: H] [TODO]
```yaml
files: [komodo/pipeline.py, komodo/gitops.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline tests.test_gitops -q
context: ["TSK-02.6.2 finished its work, then the comment lint refused the commit over an OVER_LINES finding; the task reported `builder crashed` and no repair pass ran"]
```

#### [TSK-02.5.9] doctor's stale-worktree check must not flag the main checkout when it runs inside a builder worktree [P: H] [TODO]
```yaml
files: [komodo/doctor.py, komodo/gitops.py, tests/test_doctor_install.py]
done_when:
  - python3 -m unittest tests.test_doctor_install -q
context: ["TSK-02.6.3 blocked because doctor run from the tsk-02-6-3 worktree called the repo root a stale worktree; any done_when naming doctor fails inside a wave"]
```

#### [TSK-02.5.10] doctor fails when a changelog version heading has no tag, or a tag has no heading [P: H] [TODO]
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

### [TG-02.7] Go rewrite
```yaml
type: refactor
```

#### [TSK-02.7.1] Port the orchestrator to a compiled Go binary that runs natively with no interpreter [P: M] [TODO]
```yaml
files: [docs/design-decisions.md, AGENTS.md, BACKLOG.md]
done_when:
  - python3 -m komodo doctor
context: ["same functions as komodo/*.py, compiled and executed on the native machine instead of shelling to python3", "AGENTS.md pins the harness to stdlib Python 3.9; this task supersedes that rule and the decision record has to say so", "a shipped binary has to stay zero-setup for the user: no toolchain install, no build step on their machine", "scope the port and the cutover here; the per-module work becomes its own epic"]
```

#### [TSK-02.7.2] Render the prebuilt guard binary in komodo install and fall back to guard.py when no target matches [P: M] [TODO]
```yaml
files: [komodo/install.py, komodo/adapters/claude/__init__.py, komodo/adapters/claude/settings.policy.json, tests/test_hooks.py]
done_when:
  - python3 -m unittest tests.test_hooks -q
  - python3 scripts/verify.py
context: ["the Go source and prebuilt binaries already live under komodo/adapters/claude/hooks/; nothing renders them yet", "the adapter render loop copies only .py and writes text, so a binary needs a separate binary-safe copy path", "guard.py stays as the fallback for any platform without a committed binary"]
```

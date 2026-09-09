# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)

Format and rules live in the `backlog-modify` skill — load it before editing this file. `[DONE]` tasks stay until a sweep (`/backlog-audit`) moves them to `CHANGELOG.md` and removes them — this is not a log to hand-curate.

---

## [EPIC-01] Now, V1
*Goal: keep this toolkit's own hooks, docs, and skills correct and internally consistent.*

### [TG-01.1] Cross-Cutting
* **Target Release:** V1

#### [TSK-01.1.1] `standards-go`'s magic-number rule (SUB-01.1.5 area) covers when to extract a numeric/string literal to a `const`, but never distinguishes a compile-time `const` (zero runtime cost at any scope) from a runtime-initialized `var` (`errors.New(...)`, `regexp.MustCompile(...)`, a struct literal holding a func field or requiring setup) — a real sweep in `komodo-forge-sdk-go` applied the const-inlining rule to `var`s too, deleting exported sentinel errors and inlining them per-call (breaking `errors.Is`/`==` comparisons across 4 packages, confirmed by a failing `go test ./...`) and rebuilding a `websocket.Upgrader` on every connection instead of once at package level [P: H] [TODO]
* **SUB-01.1.1.1** state the const/var distinction explicitly: a `const` costs nothing regardless of where it's declared, so scope it to the narrowest lexical scope that needs it; a `var` whose initializer runs code (not a literal) costs an allocation or computation *every time that declaration executes*, so its scope must match how often it should be constructed, not how many call sites reference it
  * **Done when:** `grep -qi "zero-cost at any scope" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.1.2** state that a sentinel error or any value compared by `==`/`errors.Is`/`errors.As` must stay a single, stable instance (package-level `var`, regardless of call-site count) — moving it into a function body creates a new instance per call and silently breaks every identity comparison against it, and deleting an exported sentinel to do so is also a breaking API change per the existing Evolution section
  * **Done when:** `grep -qi "identity comparison" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.2] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (2026-08-28):** No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs.
  * **Citation:** `bridges/komodo-bridge/` (prompt files and docs only, no Go source)
  * **Recheck:** bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match
* **SUB-01.1.2.1** fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo
  * **Done when:** a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge`

#### [TSK-01.1.3] `standards-go`'s 120-col line length is stated only as a convention — no lint gate actually enforces it, and the ~90-col soft threshold for choosing grouped-vs-one-item-per-line wrapping isn't documented at all [P: M] [TODO]
* **SUB-01.1.3.1** add golangci-lint's `lll` linter (`line-length: 120`) to `templates/go/.golangci.yaml`, excluded on `_test\.go` paths alongside the existing test-file exclusions
  * **Done when:** `grep -q "lll" templates/go/.golangci.yaml`
* **SUB-01.1.3.2** document in `standards-go` that 120 cols is now a hard lint gate, and that choosing the grouped form over one-item-per-line is governed by a ~90-col (75% of 120) soft threshold — explicitly noting that threshold is a reviewed convention only, `lll` cannot check "grouped vs. one-per-line"
  * **Done when:** `grep -qi "90.col" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.4] `standards-go`'s magic-number rule is incomplete: it doesn't cover repeated string literals, cross-package const placement, or require grouping multiple new consts into one block [P: M] [TODO]
* **SUB-01.1.4.1** extend the rule to short repeated string literals and spec-level values, not just numeric literals
  * **Done when:** `grep -qi "string literal" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.4.2** add cross-package placement guidance: a const used across packages lives in the most spec-relevant domain package, other packages import it, never a parallel copy per consumer
  * **Done when:** `grep -qi "spec-relevant" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.4.3** require two or more consts introduced together to go in one `const ( ... )` block, never consecutive standalone `const X = ...` statements
  * **Done when:** `grep -qi "const block\|const ( \.\.\. )" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.5] `standards-go` has no stated methodology for verifying a repo-wide sweep (magic numbers, casing, literal-flattening) actually covered the whole repo, including build-tag-gated files, before declaring it done [P: M] [TODO]
* **SUB-01.1.5.1** add to the Toolchain section: a requested sweep must search the whole repo (`test/`, `cmd/`, and any `//go:build`-gated files, verified against their matching `-tags`), and be confirmed with a fresh, non-cached run (`-count=1`) of build + vet + lint + the full test suite before reporting done
  * **Done when:** `grep -qi -- "-count=1" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.6] Build a new `/assess-code-conventions` skill — a style/formatting finder scoped to what a linter/formatter can't mechanically decide (wrap-style choice, magic-number extraction placement/casing/grouping, guard-clause blank lines, single-call-site inline-vs-closure judgment) — deliberately narrower than `assess-code-quality` (aggregate conventions score) and `assess-simplify` (reuse/duplication/efficiency), and must never re-flag anything `golangci-lint`/`gofmt`/the repo's own formatter already gates [P: M] [TODO]
* **SUB-01.1.6.1** draft `claude-code/skills/assess-code-conventions/SKILL.md` via `skill-creator`, modeled on `assess-simplify`'s contract (`context: fork`, `agent: reviewer`, model-agnostic Read/Grep/Glob/Bash, `argument-hint: <task text or band summary> [standards-* skills that apply] [--report]`, findings-only, same `Findings → backlog` mechanism) — process: load the touched `standards-<lang>` skill(s)' Conventions section, confirm the repo's lint/format gate is clean first, then report only the judgment-call violations that gate can't catch
  * **Done when:** `test -f claude-code/skills/assess-code-conventions/SKILL.md`
* **SUB-01.1.6.2** update `assess-code-quality` to fold `/assess-code-conventions --report` in as a standing input the same way it already folds in `/assess-performance`, so it never re-derives style judgment on its own
  * **Done when:** `grep -qi "assess-code-conventions" claude-code/skills/assess-code-quality/SKILL.md`
* **SUB-01.1.6.3** decide whether it joins `workflow-loop`'s automatic P2.3/P2.4 fork trio (`assess-bugs`/`assess-security`/`assess-simplify`) or stays standalone and explicitly user-invoked like `assess-code-quality`/`assess-change-risk`/`assess-testing` (`disable-model-invocation: true`, zero listing cost) — recommend standalone by default, matching `assess-code-quality`'s own "once per task before a push" cadence rather than adding a fourth automatic fork to every band's closeout; a decision for the user, not resolved by this plan
* **SUB-01.1.6.4** register the skill's listing cost per whichever choice SUB-01.1.6.3 lands on (`disable-model-invocation` or a `skillOverrides` `name-only` entry) and confirm the always-on budget still fits
  * **Done when:** `bash scripts/validate.sh`

#### [TSK-01.1.7] Dependency Inversion Principle is undocumented across every language standards skill — `standards-go` states the idiom ("interfaces stay small and live at the consumer") without ever naming the principle, and no other active language skill mentions it at all [P: L] [TODO]
* **SUB-01.1.7.1** name and state it explicitly in `standards-go`, alongside its existing consumer-side-interface convention
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.7.2** add it to `standards-typescript`, phrased to this language's own idiom (e.g. depend on a caller-defined interface/type, not a concrete class)
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-typescript/SKILL.md`
* **SUB-01.1.7.3** add it to `standards-python`, phrased to this language's own idiom (e.g. depend on a `Protocol`/ABC the caller defines, not a concrete class)
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-python/SKILL.md`
* **SUB-01.1.7.4** add it to `standards-java`, phrased to this language's own idiom
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-java/SKILL.md`
* **SUB-01.1.7.5** add it to `standards-c`, phrased to this language's own idiom (e.g. depend on a function-pointer/vtable-style seam the caller owns, not a concrete implementation)
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-c/SKILL.md`

#### [TSK-01.1.8] `standards-go`'s new deterministic-formatting conventions (line-wrapping algorithm, magic-number-to-named-const extraction, blank lines around guard clauses) are language-agnostic but exist only in `standards-go` — every other active language skill still defers formatting entirely to its own linter/formatter with no manual-style rules of its own (after: "Fix standards-go's SCREAMING_SNAKE_CASE guidance") [P: L] [TODO]
* **SUB-01.1.8.1** add equivalent conventions to `standards-typescript`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-typescript/SKILL.md`
* **SUB-01.1.8.2** add equivalent conventions to `standards-python`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-python/SKILL.md`
* **SUB-01.1.8.3** add equivalent conventions to `standards-java`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-java/SKILL.md`
* **SUB-01.1.8.4** add equivalent conventions to `standards-c`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-c/SKILL.md`

#### [TSK-01.1.9] `standards-go`'s single-call-site rule defaults to "inline as a closure," but a closure still costs an allocation a fully-inlined statement doesn't — the rule should default to full inlining and reserve the closure form for when the value needs `:=` assignment or expression-embedding [P: L] [TODO]
* **SUB-01.1.9.1** rewrite the rule so full inlining into the caller's body is the default for a pure, small, single-call-site helper with no domain significance, and a closure is used only when full inlining would awkwardly restructure the caller
  * **Done when:** `grep -qi "fully inlin\|full inline" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.9.2** name concrete security/audit-significance examples that stay named regardless of size (a constant-time compare, an auth check, a revocation/ban predicate) — greppable-during-review outweighs one fewer symbol
  * **Done when:** `grep -qi "constant-time compare" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.10] Two Go 1.27+ "modernize" transforms were proposed for `standards-go` (embedded-field composite-literal flattening, `errors.As` → `errors.AsType[T]`) — neither is confirmed against actual Go release notes or the `gopls` modernize analyzer as of this review, so they must not be written into a shared skill unverified [P: L] [TODO]
* **SUB-01.1.10.1** confirm both transforms actually exist (check the Go release notes and `gopls`'s modernize analyzer docs for the version `go.mod` would need to declare) before writing anything — this is a verification step, not yet a documentation change

#### [TSK-01.1.11] Testable-logging is an undecided, recurring question in Go services — no documented pattern exists in `standards-go` for making a struct's log output assertable in a test without forcing mandatory logger injection everywhere [P: L] [TODO]
* **SUB-01.1.11.1** document the pattern: a narrow `Logger` interface field on a `Deps`-style struct, nil-checked, defaulting to a thin adapter over the package/SDK's global logging funcs when unset — never mandatory/non-nullable on every constructor, and never assume an existing global/singleton logger is untouchable by default either; this is filed separately from the formatting work above since it's a DI/architecture pattern, not a style rule
  * **Done when:** `grep -qi "Logger interface" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.12] Open policy question: should "reflow a pre-existing wrapped line to the current convention whenever it's revisited during unrelated work" be a standing, written exception to this toolkit's own no-scope-expansion rule in `AGENTS.md`? [P: L] [TODO]
* **SUB-01.1.12.1** decide whether formatting-only drive-by fixes get a blanket exception (and if so, where that exception is written down — `standards-go`, `AGENTS.md`, or both) versus staying subject to the existing scope rule; a decision for the user, not something this review resolves on its own

#### [TSK-01.1.13] `reviewer` agent's "never edit any file other than BACKLOG.md" boundary is prose-only — `claude-code/agents/reviewer.md` grants the unscoped `Edit` tool and `claude-code/settings.json`'s only `PreToolUse` hook matches `Bash` (`git_guard.py`), so nothing stops an `assess-bugs`/`assess-security`/`assess-simplify` fork's `Edit` tool call from touching any file in the repo — unlike the `Bash` side, which `git_guard.py`'s `GIT_GUARD_ONLY_EXTENSIONS`/`EXTENSION_FAMILY` check does close [P: M] [TODO]
* **SUB-01.1.13.1** add path enforcement for the reviewer fork's `Edit`/`Write` calls — either a `PreToolUse` hook matching `Edit|Write` that denies any path but `BACKLOG.md`/`docs/BACKLOG.md` when the active agent is `reviewer`, or an equivalent mechanism the loader actually supports
  * **Done when:** `/assess-bugs <file>` reports it clear

#### [TSK-01.1.14] `claude-code/hooks/comments.py` defines `KNOWN_TEMPLATE_TYPES` twice in a row, identically — dead duplicate module-level constant [P: L] [TODO]
* **SUB-01.1.14.1** delete the second (redundant) `KNOWN_TEMPLATE_TYPES = (...)` assignment at `claude-code/hooks/comments.py:36`, keeping the first
  * **Done when:** `[ "$(grep -c '^KNOWN_TEMPLATE_TYPES = ' claude-code/hooks/comments.py)" = 1 ]`

#### [TSK-01.1.15] `comments.py check` in its default (changed-lines) mode never sees a new, untracked file — `changed_line_map` shells out to `git diff --unified=0 HEAD --`, which excludes untracked paths entirely, so a brand-new source file reports clean until it is staged or committed; `make comments`/`make verify` (the Stop gate's own target) therefore pass on exactly the case that most needs the lint. Confirmed live: an untracked `zz_gaptest.go` with a 3-value return reported `0 finding(s)` on the default check and `MISSING RET_ARITY_3` under `--all` [P: M] [TODO]
* **SUB-01.1.15.1** fold untracked-but-not-ignored files into `changed_line_map`'s result as wholly-changed (`git ls-files --others --exclude-standard`), so a new file is linted in full without switching the whole check to `--all`
  * **Done when:** `bash scripts/test-hooks.sh` passes with a new case asserting a new untracked file with a `RET_ARITY_3` site is reported by a default `comments.py check`

#### [TSK-01.1.16] This repo has no CI — `.github/` holds only `PULL_REQUEST_TEMPLATE.md`, with no workflow running `make verify`, so the 160-case hook suite, `scripts/validate.sh`, and the comment lint only ever run on the author's own machine via `scripts/hooks/git/pre-push-verify`; a push that bypasses the local hook (or a contributor who never ran `scripts/hooks/git/install.sh`) lands unverified config that every project's next session picks up [P: M] [TODO]
* **SUB-01.1.16.1** add `.github/workflows/verify.yml` running `make verify` on pull requests and pushes to `main`, matching the contract `standards-cicd` already states for a target repo
  * **Done when:** `test -f .github/workflows/verify.yml && grep -q "make verify" .github/workflows/verify.yml`

#### [TSK-01.1.17] The entire hook layer depends on `python3` being on `PATH` and nothing declares or checks it — all four `settings.json` hook commands and both `scripts/hooks/git/` dispatchers shell out to `python3`, `setup.sh` never mentions it, and there is no manifest declaring a floor; if `python3` is missing or shadowed, the hook command itself fails before any Python runs, so `git_guard.py`'s fail-closed `except BaseException` handler never executes and the guard silently degrades open [P: M] [TODO]
* **SUB-01.1.17.1** have `setup.sh` verify `python3` resolves and meets the toolkit's actual floor before linking anything, and state that floor in the repo's `AGENTS.md` alongside the hook table
  * **Done when:** `bash setup.sh --dry-run` reports the detected `python3` version and fails when none resolves

#### [TSK-01.1.18] `comments.py check --all` (whole-file scope) surfaces ~163 findings across nearly every script in the repo that `check`'s default changed-lines-only mode never sees — three mechanical rules (`STEP_MARKER`, `BANNER_OUTSIDE_TEST`, `STACKED`) were tuned against Go-style single-line comments and carry no carve-out for a shell/Python file's multi-line header docblock, so a normal numbered "what it does" list, a `---` section divider, or an ordinary multi-line shebang-header comment self-triggers every time `--all` runs; confirmed live: `setup.sh`, `validate.sh`, `test-hooks.sh`, `install.py`, `install.sh`, `git_guard.py`, and `comment_rules.py` itself all flagged, none touched by the invocation that surfaced them [P: L] [TODO]
* **SUB-01.1.18.1** decide whether `comment_rules.py`'s `STEP_MARKER`/`STACKED`/`BANNER_OUTSIDE_TEST` checks should exempt a leading file-header comment block (before the first code statement) — no `STEP_MARKER`/`STACKED` finding inside it, and `BANNER_SHAPE` scoped to `.go` files only, matching how `DOC_LANGUAGE_EXTENSIONS` already scopes `DOC` — versus leaving `--all` as a diagnostic-only mode never meant to gate real work; record the decision in `docs/design-decisions.md`
  * **Done when:** `docs/design-decisions.md` names the chosen resolution for `--all`'s file-header false positives

- [M] `scripts/test-hooks.sh`'s plain `bash_case`/`smoke_case` invocations (`S1`/`S5`-`S7`, `G100`, `G125`-`G131`, `G134`-`G135`, etc.) send no `cwd` in the payload, so `git_guard.py`'s new cwd-derived `is_guarded_path`/`repo_root_of` resolves against `os.getcwd()` of whatever process invokes the test script — these deny assertions only pass because the suite happens to be run from inside this repo's own git worktree; run from a checkout without `.git` (a tarball, a stripped CI workspace) or any other non-repo cwd, and every one of them silently flips from deny to allow-then-fail, unlike the new `bash_case_at` cases that pin cwd explicitly · S → `/assess-bugs scripts/test-hooks.sh` reports it clear

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-02, updated post-shipped review-fork/AGENTS.md-trim/bundled-skill-collapse/git-pr-create-diff-stat/workflow-loop-compaction-cap/config-accessibility-fix/git_guard-backtick-fix/git_guard-dedup work):** the always-on budget is healthy (`scripts/validate.sh`: 1,155 of 2,000 tokens). Root `AGENTS.md` was cut from 7,157 to 1,416 tokens, `assess-bugs`/`assess-security`/`assess-simplify`/`backlog-audit` now run as `reviewer`/`workflow-implementer` forks instead of the orchestrator window, `git-pr-create`'s P4 read now uses `--stat` instead of the full diff, `workflow-loop/SKILL.md` was trimmed under the compaction re-attach cap with a `scripts/validate.sh` check enforcing it, `config-accessibility` was shrunk with its dead `CLAUDE.local.md` reference fixed, and the `git_guard.py` backtick false-positive was fixed alongside four Critical substitution-scanner bypasses it surfaced, plus a follow-up dedup refactor (all shipped; `backlog-audit` renumbered the remaining task below). Remaining scope: further adversarial hardening of `git_guard.py`'s shell-parsing.

#### [TSK-01.2.1] git_guard.py's shell-parsing is not exhaustively adversarial-hardened against every quoting/escaping/substitution combination [P: L] [TODO]
* **SUB-01.2.1.1** the 2026-09-01 band that fixed the `git_guard.py` backtick false-positive (and, in review, caught and fixed three unrelated Critical bypasses along the way — a `#`-comment quote-state swallow, missing backtick/`$()` substitution detection entirely, and an escaped-nested-backtick gap) deliberately stopped hardening `claude-code/hooks/git_guard.py`'s `extract_substitutions`/`split_segments` once those four were closed, rather than continuing to chase further shell-quoting edge cases in the same pass — hand-parsing arbitrary POSIX shell quoting/escaping/substitution semantics to zero residual risk is open-ended, the same reasoning already applied to the risk-accepted `grep`/`sed`/`awk`/`curl` secret-exfiltration gap in `CHANGELOG.md`'s `[0.37.2]` entry. Untested-but-plausible remaining edge cases: `$(...)` containing backslash-escaped backticks, deeper mixed single/double-quote/substitution nesting, and other exotic POSIX escaping shapes. `scripts/test-hooks.sh` now covers `G90`-`G98` for the shapes found so far.
  * **Done when:** a dedicated, systematic pass (ideally against a real shell-grammar reference or fuzzer, not ad hoc cases) audits `extract_substitutions`/`split_segments`/`find_backtick_end`/`find_paren_end` against the POSIX shell quoting grammar and either closes every gap found or explicitly risk-accepts each one in `CHANGELOG.md`, matching the existing `[0.37.2]` pattern

#### [TSK-01.2.2] `scripts/validate.sh`'s budget pass counts every non-disabled skill's listing cost identically, without exempting a skill that carries `paths:` — per `AGENTS.md`, a `paths:`-gated skill is not in the base always-on listing until a matching file is touched, so the reported total overstates the real always-on cost (25 of 61 `SKILL.md` files currently carry `paths:`) [P: M] [TODO]
* **SUB-01.2.2.1** in the budget loop (`scripts/validate.sh`'s embedded Python, around the `for entry in sorted(os.listdir(skills_dir))` loop), skip or separately report a skill whose frontmatter has a `paths:` key, since it isn't part of the true always-on total the `BUDGET` ceiling is meant to gate
  * **Done when:** `bash scripts/validate.sh` reports the budget total with `paths:`-gated skills excluded (or broken out as a separate, non-gated figure), matching the always-on-listing definition in `AGENTS.md`

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

#### [TSK-01.3.1] `repo-assess` cannot execute as written — its Process step 4 mandates invoking `assess-readiness`, `assess-code-quality` and `assess-testing`, and all three carry `disable-model-invocation: true`, which `docs/design-decisions.md` itself states is "the only thing that controls cross-skill reachability"; the Skill tool refuses them outright ("cannot be used with Skill tool due to disable-model-invocation… Do not replicate this skill's workflow by other means"), so 3 of the 9 category scores are unobtainable and the composite — defined as the unweighted average of all nine — cannot be computed. Confirmed live this session: `Skill(assess-readiness)` returned that refusal, and only the six reachable sub-assessments ran [P: C] [TODO]
* **SUB-01.3.1.1** decide the resolution and record it in `docs/design-decisions.md`: either drop `disable-model-invocation: true` from the three sub-skills (keeping the human-decision gate only on `repo-assess` itself, which is the one the user actually types), or rewrite `repo-assess` to score the six reachable dimensions and instruct the user to run the other three by hand — the first preserves the composite, the second preserves the typed-only rule, and only the user can pick which of those two properties matters more
  * **Done when:** `docs/design-decisions.md` names the chosen resolution and `claude-code/skills/repo-assess/SKILL.md` matches it
* **SUB-01.3.1.2** add a `scripts/validate.sh` check that fails when a skill body names a sibling skill it cannot reach — a `/skill-name` or `` `skill-name` `` invocation instruction pointing at a skill whose frontmatter carries `disable-model-invocation: true`
  * **Done when:** `bash scripts/validate.sh` fails on a deliberately-introduced unreachable cross-skill invocation and passes once it is removed

#### [TSK-01.3.2] Open naming question: is `workflow-<phase>` the right fixed prefix for the 5-phase spec→decompose→execute→consolidate→publish loop (`workflow-loop`, `workflow-decompose`, `workflow-implement`, `workflow-consolidate`, `workflow-complete`, `workflow-debug`), or does a different prefix communicate the loop's role more clearly — user-raised, `harness-*` named as one candidate [P: L] [TODO]
* **SUB-01.3.2.1** decide whether to rename, and to what — `harness-*` collides with existing terminology Claude Code's own system prompt already uses (a `# Harness` section describing the CLI tool/environment itself), so `harness-loop`/`harness-implement` would likely read as "the CLI's own loop," not "this toolkit's SDLC loop"; weigh candidates that don't overload a term the host tool already owns (e.g. `sdlc-*`, `loop-*`, or keeping `workflow-*`) against `docs/design-decisions.md`'s existing "Skill naming buckets" table before picking — this is a user decision, not one this task resolves on its own
  * **Done when:** `docs/design-decisions.md`'s naming-buckets section names the chosen prefix (or explicitly keeps `workflow-*`) and states why
* **SUB-01.3.2.2** if renamed, update all 6 skill directories, every cross-reference across the toolkit (`agents/`, other `skills/*/SKILL.md`, `AGENTS.md`, `README.md`), and `claude-code/settings.json`'s `skillOverrides` keys to match
  * **Done when:** `! grep -rl "workflow-loop\|workflow-decompose\|workflow-implement\|workflow-consolidate\|workflow-complete\|workflow-debug" claude-code/ templates/ README.md` (after the rename; this check is meaningless before SUB-01.3.2.1 decides)

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1
* **Context (2026-09-08):** Two independent sessions surfaced evidence that the loop and the hook layer both lack circuit breakers. A single-file review chain ran 8 forked adversarial passes for work scoped as 3 bugs (this session, ~65% of a Max budget). A `workflow-loop` skill diagnosis from a different session found 4 concrete waste patterns in unrelated bands (premise-blind forking, missed fan-out, an unpinned mechanism choice, backlog re-reads). A live in-place hook edit produced two total Bash-tool outages because `git_guard.py`'s crash handler denies every command indiscriminately, not just risky ones. None of these are one-off bugs — they're gaps in the loop's and the hook's own safety design.

#### [TSK-01.4.1] `workflow-implementer`'s standing rule "run the fork even when the code already appears to exist on disk" collapses two different situations into one instruction — it correctly stops a caller from skipping a fork just because code looks present, but nothing separately checks whether the task's own premise (the defect or gap it describes) still holds against current code, so a stale backlog entry can reach an implementer that builds against a premise the repo has already outgrown; confirmed cost elsewhere: a full implement + benchmark + review + revert cycle that shipped nothing because the described defect was already gone [P: H] [TODO]
* **SUB-01.4.1.1** add a premise-verification step to `workflow-implement/SKILL.md`, before the existing "run the fork even when code appears to exist" rule: read the function/file the task names and confirm the defect it describes is still present — a task whose premise the repo has outgrown goes back to `BACKLOG.md` re-aimed or removed, never forked as-is; a premise that still holds plus code that looks already-written is inherited state to verify, not a task to skip
  * **Done when:** `grep -qi "verify the task's premise" claude-code/skills/workflow-implement/SKILL.md`

#### [TSK-01.4.4] No smoke test runs against `git_guard.py`'s actual runtime behavior separately from `python3 -m py_compile` — both outage states in the post-mortem passed `py_compile` cleanly (a syntax check, not a call-shape check), so the only thing that would have caught either failure before it reached a live session is exercising the hook end-to-end [P: M] [TODO]
* **SUB-01.4.4.1** add a small smoke-test set to `scripts/test-hooks.sh`, run first, before the full case list: pipe a fixed set of representative JSON payloads (reads: `echo`, `git status`, `ls`; writes/protected-ref ops: `git push origin main`, `tee BACKLOG.md`, `sed -i`, a redirect into a guarded path) through `git_guard.py` and assert the allow/deny split matches, so a call-shape break (mismatched unpack, undefined name) surfaces fast rather than only inside the full suite
  * **Done when:** `bash scripts/test-hooks.sh` runs the smoke set first and fails fast on a deliberately-introduced call-shape break (e.g. an extra return value with no updated caller)

#### [TSK-01.4.6] `git_guard.py`'s `MAX_SCAN_DEPTH` bound is enforced by three separately hand-rolled `if depth > MAX_SCAN_DEPTH: raise/append` checks (`resolve_command_output`, `expand_word`, `scan_command`) instead of one shared guard — the next recursive scanner added to this file has nothing to copy but the same three-line pattern again [P: M] [TODO]
* **SUB-01.4.6.1** consolidate the three depth checks into one helper (e.g. a small `check_depth(depth)` raising `ScanDepthExceeded`, called at the top of each recursive function) so a future recursive scanner has one call to add, not a pattern to reinvent
  * **Done when:** `/assess-simplify claude-code/hooks/git_guard.py` reports it clear

#### [TSK-01.4.7] `is_guarded_path`'s `repo_root_of` shells out to `git rev-parse --show-toplevel` on every call with no memoization, and it's called once per source argument in a multi-source `cp`/`mv` (and once per redirect/tee target) — an `mv a b c d e /repo/dir` spawns a `git` subprocess per source for a value that's identical across the whole scan [P: M] [TODO]
* **SUB-01.4.7.1** cache `repo_root_of` (e.g. `functools.lru_cache`) or hoist the repo-root resolution to once per top-level `scan_command` call instead of once per candidate path, so a multi-argument write command doesn't spawn a redundant `git` process per argument
  * **Done when:** `/assess-simplify claude-code/hooks/git_guard.py` reports it clear

#### [TSK-01.4.8] `scripts/test-hooks.sh`'s new smoke-test cases S4 (`git push origin main`) and S6 (`sed -i -e s/a/b/ file`) run the byte-identical command string already asserted by G58 and G107 respectively — the smoke set adds no coverage those two exercise beyond running earlier in the file [P: L] [TODO]
* **SUB-01.4.8.1** swap S4/S6 to a distinct representative command each (or accept the overlap explicitly with a comment noting the smoke set intentionally duplicates a couple of G-cases for fast-fail ordering) so the two blocks aren't silently asserting the same thing twice
  * **Done when:** `/assess-simplify scripts/test-hooks.sh` reports it clear

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

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

#### [TSK-01.1.1] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (2026-08-28):** No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs.
  * **Citation:** `bridges/komodo-bridge/` (prompt files and docs only, no Go source)
  * **Recheck:** bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match
* **SUB-01.1.1.1** fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo
  * **Done when:** a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge`

#### [TSK-01.1.2] Dependency Inversion Principle is undocumented across every language standards skill — `standards-go` states the idiom ("interfaces stay small and live at the consumer") without ever naming the principle, and no other active language skill mentions it at all [P: L] [TODO]
* **SUB-01.1.2.1** name and state it explicitly in `standards-go`, alongside its existing consumer-side-interface convention
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.2.2** add it to `standards-typescript`, phrased to this language's own idiom (e.g. depend on a caller-defined interface/type, not a concrete class)
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-typescript/SKILL.md`
* **SUB-01.1.2.3** add it to `standards-python`, phrased to this language's own idiom (e.g. depend on a `Protocol`/ABC the caller defines, not a concrete class)
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-python/SKILL.md`
* **SUB-01.1.2.4** add it to `standards-java`, phrased to this language's own idiom
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-java/SKILL.md`
* **SUB-01.1.2.5** add it to `standards-c`, phrased to this language's own idiom (e.g. depend on a function-pointer/vtable-style seam the caller owns, not a concrete implementation)
  * **Done when:** `grep -qi "dependency inversion" claude-code/skills/standards-c/SKILL.md`

#### [TSK-01.1.3] `standards-go`'s new deterministic-formatting conventions (line-wrapping algorithm, magic-number-to-named-const extraction, blank lines around guard clauses) are language-agnostic but exist only in `standards-go` — every other active language skill still defers formatting entirely to its own linter/formatter with no manual-style rules of its own (after: "Fix standards-go's SCREAMING_SNAKE_CASE guidance") [P: L] [TODO]
* **SUB-01.1.3.1** add equivalent conventions to `standards-typescript`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-typescript/SKILL.md`
* **SUB-01.1.3.2** add equivalent conventions to `standards-python`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-python/SKILL.md`
* **SUB-01.1.3.3** add equivalent conventions to `standards-java`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-java/SKILL.md`
* **SUB-01.1.3.4** add equivalent conventions to `standards-c`, phrased to its own idiom
  * **Done when:** `grep -qi "collapsing one level" claude-code/skills/standards-c/SKILL.md`

#### [TSK-01.1.4] Fix standards-go's SCREAMING_SNAKE_CASE guidance — it currently tells the model to preserve legacy screaming-case consts as the package's intentional convention, contradicting idiomatic Go (stdlib and every major Go style guide use MixedCaps for exported consts, lowerCamelCase for unexported) [P: H] [DONE]
* **SUB-01.1.4.1** replace the "preserve legacy `SCREAMING_SNAKE_CASE`" line with: never introduce a new `SCREAMING_SNAKE_CASE` const, exported or not, in any Go repo; exported consts use MixedCaps, unexported use lowerCamelCase; an existing `SCREAMING_SNAKE_CASE` const is grandfathered legacy and a rename-sweep candidate, never a pattern to continue even in the same file
  * **Done when:** `! grep -q "only where the package already uses that convention" claude-code/skills/standards-go/SKILL.md && grep -qi "never introduce" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.4.2** state the boundary for an explicitly-requested repo-wide casing sweep: rename every exported `SCREAMING_SNAKE_CASE` identifier the repo itself owns (definition + every call site, tests included), never one owned by an external/SDK package just because a local file references it
  * **Done when:** `grep -qi "owned by an external" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.5] `standards-go`'s 120-col line length is stated only as a convention — no lint gate actually enforces it, and the ~90-col soft threshold for choosing grouped-vs-one-item-per-line wrapping isn't documented at all [P: M] [TODO]
* **SUB-01.1.5.1** add golangci-lint's `lll` linter (`line-length: 120`) to `templates/go/.golangci.yaml`, excluded on `_test\.go` paths alongside the existing test-file exclusions
  * **Done when:** `grep -q "lll" templates/go/.golangci.yaml`
* **SUB-01.1.5.2** document in `standards-go` that 120 cols is now a hard lint gate, and that choosing the grouped form over one-item-per-line is governed by a ~90-col (75% of 120) soft threshold — explicitly noting that threshold is a reviewed convention only, `lll` cannot check "grouped vs. one-per-line"
  * **Done when:** `grep -qi "90.col" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.6] `standards-go`'s magic-number rule is incomplete: it doesn't cover repeated string literals, cross-package const placement, or require grouping multiple new consts into one block [P: M] [TODO]
* **SUB-01.1.6.1** extend the rule to short repeated string literals and spec-level values, not just numeric literals
  * **Done when:** `grep -qi "string literal" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.6.2** add cross-package placement guidance: a const used across packages lives in the most spec-relevant domain package, other packages import it, never a parallel copy per consumer
  * **Done when:** `grep -qi "spec-relevant" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.6.3** require two or more consts introduced together to go in one `const ( ... )` block, never consecutive standalone `const X = ...` statements
  * **Done when:** `grep -qi "const block\|const ( \.\.\. )" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.7] `standards-go`'s single-call-site rule defaults to "inline as a closure," but a closure still costs an allocation a fully-inlined statement doesn't — the rule should default to full inlining and reserve the closure form for when the value needs `:=` assignment or expression-embedding [P: L] [TODO]
* **SUB-01.1.7.1** rewrite the rule so full inlining into the caller's body is the default for a pure, small, single-call-site helper with no domain significance, and a closure is used only when full inlining would awkwardly restructure the caller
  * **Done when:** `grep -qi "fully inlin\|full inline" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.7.2** name concrete security/audit-significance examples that stay named regardless of size (a constant-time compare, an auth check, a revocation/ban predicate) — greppable-during-review outweighs one fewer symbol
  * **Done when:** `grep -qi "constant-time compare" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.8] Two Go 1.27+ "modernize" transforms were proposed for `standards-go` (embedded-field composite-literal flattening, `errors.As` → `errors.AsType[T]`) — neither is confirmed against actual Go release notes or the `gopls` modernize analyzer as of this review, so they must not be written into a shared skill unverified [P: L] [TODO]
* **SUB-01.1.8.1** confirm both transforms actually exist (check the Go release notes and `gopls`'s modernize analyzer docs for the version `go.mod` would need to declare) before writing anything — this is a verification step, not yet a documentation change

#### [TSK-01.1.9] Testable-logging is an undecided, recurring question in Go services — no documented pattern exists in `standards-go` for making a struct's log output assertable in a test without forcing mandatory logger injection everywhere [P: L] [TODO]
* **SUB-01.1.9.1** document the pattern: a narrow `Logger` interface field on a `Deps`-style struct, nil-checked, defaulting to a thin adapter over the package/SDK's global logging funcs when unset — never mandatory/non-nullable on every constructor, and never assume an existing global/singleton logger is untouchable by default either; this is filed separately from the formatting work above since it's a DI/architecture pattern, not a style rule
  * **Done when:** `grep -qi "Logger interface" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.10] `standards-go` has no stated methodology for verifying a repo-wide sweep (magic numbers, casing, literal-flattening) actually covered the whole repo, including build-tag-gated files, before declaring it done [P: M] [TODO]
* **SUB-01.1.10.1** add to the Toolchain section: a requested sweep must search the whole repo (`test/`, `cmd/`, and any `//go:build`-gated files, verified against their matching `-tags`), and be confirmed with a fresh, non-cached run (`-count=1`) of build + vet + lint + the full test suite before reporting done
  * **Done when:** `grep -qi -- "-count=1" claude-code/skills/standards-go/SKILL.md`

#### [TSK-01.1.11] Open policy question: should "reflow a pre-existing wrapped line to the current convention whenever it's revisited during unrelated work" be a standing, written exception to this toolkit's own no-scope-expansion rule in `AGENTS.md`? [P: L] [TODO]
* **SUB-01.1.11.1** decide whether formatting-only drive-by fixes get a blanket exception (and if so, where that exception is written down — `standards-go`, `AGENTS.md`, or both) versus staying subject to the existing scope rule; a decision for the user, not something this review resolves on its own

#### [TSK-01.1.12] Build a new `/assess-code-conventions` skill — a style/formatting finder scoped to what a linter/formatter can't mechanically decide (wrap-style choice, magic-number extraction placement/casing/grouping, guard-clause blank lines, single-call-site inline-vs-closure judgment) — deliberately narrower than `assess-code-quality` (aggregate conventions score) and `assess-simplify` (reuse/duplication/efficiency), and must never re-flag anything `golangci-lint`/`gofmt`/the repo's own formatter already gates [P: M] [TODO]
* **SUB-01.1.12.1** draft `claude-code/skills/assess-code-conventions/SKILL.md` via `skill-creator`, modeled on `assess-simplify`'s contract (`context: fork`, `agent: reviewer`, model-agnostic Read/Grep/Glob/Bash, `argument-hint: <task text or band summary> [standards-* skills that apply] [--report]`, findings-only, same `Findings → backlog` mechanism) — process: load the touched `standards-<lang>` skill(s)' Conventions section, confirm the repo's lint/format gate is clean first, then report only the judgment-call violations that gate can't catch
  * **Done when:** `test -f claude-code/skills/assess-code-conventions/SKILL.md`
* **SUB-01.1.12.2** update `assess-code-quality` to fold `/assess-code-conventions --report` in as a standing input the same way it already folds in `/assess-performance`, so it never re-derives style judgment on its own
  * **Done when:** `grep -qi "assess-code-conventions" claude-code/skills/assess-code-quality/SKILL.md`
* **SUB-01.1.12.3** decide whether it joins `workflow-loop`'s automatic P2.3/P2.4 fork trio (`assess-bugs`/`assess-security`/`assess-simplify`) or stays standalone and explicitly user-invoked like `assess-code-quality`/`assess-change-risk`/`assess-testing` (`disable-model-invocation: true`, zero listing cost) — recommend standalone by default, matching `assess-code-quality`'s own "once per task before a push" cadence rather than adding a fourth automatic fork to every band's closeout; a decision for the user, not resolved by this plan
* **SUB-01.1.12.4** register the skill's listing cost per whichever choice SUB-01.1.12.3 lands on (`disable-model-invocation` or a `skillOverrides` `name-only` entry) and confirm the always-on budget still fits
  * **Done when:** `bash scripts/validate.sh`

#### [TSK-01.1.13] `standards-go`'s magic-number rule (SUB-01.1.6 area) covers when to extract a numeric/string literal to a `const`, but never distinguishes a compile-time `const` (zero runtime cost at any scope) from a runtime-initialized `var` (`errors.New(...)`, `regexp.MustCompile(...)`, a struct literal holding a func field or requiring setup) — a real sweep in `komodo-forge-sdk-go` applied the const-inlining rule to `var`s too, deleting exported sentinel errors and inlining them per-call (breaking `errors.Is`/`==` comparisons across 4 packages, confirmed by a failing `go test ./...`) and rebuilding a `websocket.Upgrader` on every connection instead of once at package level [P: H] [TODO]
* **SUB-01.1.13.1** state the const/var distinction explicitly: a `const` costs nothing regardless of where it's declared, so scope it to the narrowest lexical scope that needs it; a `var` whose initializer runs code (not a literal) costs an allocation or computation *every time that declaration executes*, so its scope must match how often it should be constructed, not how many call sites reference it
  * **Done when:** `grep -qi "zero-cost at any scope" claude-code/skills/standards-go/SKILL.md`
* **SUB-01.1.13.2** state that a sentinel error or any value compared by `==`/`errors.Is`/`errors.As` must stay a single, stable instance (package-level `var`, regardless of call-site count) — moving it into a function body creates a new instance per call and silently breaks every identity comparison against it, and deleting an exported sentinel to do so is also a breaking API change per the existing Evolution section
  * **Done when:** `grep -qi "identity comparison" claude-code/skills/standards-go/SKILL.md`

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-02, updated post-shipped review-fork/AGENTS.md-trim/bundled-skill-collapse/git-pr-create-diff-stat/workflow-loop-compaction-cap/config-accessibility-fix/git_guard-backtick-fix/git_guard-dedup work):** the always-on budget is healthy (`scripts/validate.sh`: 1,155 of 2,000 tokens). Root `AGENTS.md` was cut from 7,157 to 1,416 tokens, `assess-bugs`/`assess-security`/`assess-simplify`/`backlog-audit` now run as `reviewer`/`workflow-implementer` forks instead of the orchestrator window, `git-pr-create`'s P4 read now uses `--stat` instead of the full diff, `workflow-loop/SKILL.md` was trimmed under the compaction re-attach cap with a `scripts/validate.sh` check enforcing it, `config-accessibility` was shrunk with its dead `CLAUDE.local.md` reference fixed, and the `git_guard.py` backtick false-positive was fixed alongside four Critical substitution-scanner bypasses it surfaced, plus a follow-up dedup refactor (all shipped; `backlog-audit` renumbered the remaining task below). Remaining scope: further adversarial hardening of `git_guard.py`'s shell-parsing.

#### [TSK-01.2.1] git_guard.py's shell-parsing is not exhaustively adversarial-hardened against every quoting/escaping/substitution combination [P: L] [TODO]
* **SUB-01.2.1.1** the 2026-09-01 band that fixed the `git_guard.py` backtick false-positive (and, in review, caught and fixed three unrelated Critical bypasses along the way — a `#`-comment quote-state swallow, missing backtick/`$()` substitution detection entirely, and an escaped-nested-backtick gap) deliberately stopped hardening `claude-code/hooks/git_guard.py`'s `extract_substitutions`/`split_segments` once those four were closed, rather than continuing to chase further shell-quoting edge cases in the same pass — hand-parsing arbitrary POSIX shell quoting/escaping/substitution semantics to zero residual risk is open-ended, the same reasoning already applied to the risk-accepted `grep`/`sed`/`awk`/`curl` secret-exfiltration gap in `CHANGELOG.md`'s `[0.37.2]` entry. Untested-but-plausible remaining edge cases: `$(...)` containing backslash-escaped backticks, deeper mixed single/double-quote/substitution nesting, and other exotic POSIX escaping shapes. `scripts/test-hooks.sh` now covers `G90`-`G98` for the shapes found so far.
  * **Done when:** a dedicated, systematic pass (ideally against a real shell-grammar reference or fuzzer, not ad hoc cases) audits `extract_substitutions`/`split_segments`/`find_backtick_end`/`find_paren_end` against the POSIX shell quoting grammar and either closes every gap found or explicitly risk-accepts each one in `CHANGELOG.md`, matching the existing `[0.37.2]` pattern

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

#### [TSK-01.3.1] Rename `repo-assess` to `assess-repo`, moving the full-repo orchestrator into the `/assess-*` namespace it aggregates [P: M] [TODO]
* **SUB-01.3.1.1** create `claude-code/skills/assess-repo/SKILL.md` with `repo-assess`'s content verbatim (frontmatter `name: assess-repo`, `disable-model-invocation: true` retained), then delete `claude-code/skills/repo-assess/`
  * **Done when:** `test -f claude-code/skills/assess-repo/SKILL.md && ! test -d claude-code/skills/repo-assess`
* **SUB-01.3.1.2** confirm no reference to the old name survives anywhere in the toolkit
  * **Done when:** `! grep -rl "repo-assess" claude-code/ templates/ scripts/`
* **SUB-01.3.1.3** re-run the full validator
  * **Done when:** `bash scripts/validate.sh`

#### [TSK-01.3.2] Rename `repo-init` to `git-repo-init` across the toolkit [P: M] [TODO]
* **SUB-01.3.2.1** rename `claude-code/skills/repo-init/` to `claude-code/skills/git-repo-init/`, updating its own `name:` frontmatter field
  * **Done when:** `test -f claude-code/skills/git-repo-init/SKILL.md && ! test -d claude-code/skills/repo-init`
* **SUB-01.3.2.2** update every cross-reference across `claude-code/agents/workflow-implementer.md` and the ~18 `claude-code/skills/*/SKILL.md` files naming `repo-init` (`standards-vue`, `standards-specs`, `standards-shell`, `standards-go`, `standards-cdk`, `standards-java`, `runbook`, `workflow-loop`, `readme`/`readme-modify`, `standards-python`, `standards-c`, `standards-dotnet`, `standards-svelte`, `sdd`, `prd`, `standards-react`, `backlog-modify`, `git-pr-create`, `git-commit-message`)
  * **Done when:** `! grep -rl "\brepo-init\b" claude-code/ templates/ scripts/ | grep -v git-repo-init`
* **SUB-01.3.2.3** update the `repo-init` key in `claude-code/settings.json`'s `skillOverrides` to `git-repo-init`
  * **Done when:** `grep -q '"git-repo-init"' claude-code/settings.json`
* **SUB-01.3.2.4** re-run the full validator and the hook regression suite
  * **Done when:** `bash scripts/validate.sh`; `bash scripts/test-hooks.sh`

#### [TSK-01.3.3] Split `changelog` into `changelog-write` and `changelog-audit`, matching the `backlog-modify`/`backlog-audit` precedent [P: M] [TODO]
* **SUB-01.3.3.1** create `claude-code/skills/changelog-write/SKILL.md` from `changelog/SKILL.md`'s Part 1 (`write <entry>` mode) content, dropping the mode-selection preamble
  * **Done when:** `test -f claude-code/skills/changelog-write/SKILL.md`
* **SUB-01.3.3.2** create `claude-code/skills/changelog-audit/SKILL.md` from `changelog/SKILL.md`'s Part 2 (`audit [scope]` mode) content, matching `readme-audit`'s shape (findings-only, `--report` flag)
  * **Done when:** `test -f claude-code/skills/changelog-audit/SKILL.md`
* **SUB-01.3.3.3** delete `claude-code/skills/changelog/`, reclassifying each of the 7 cross-reference sites (`backlog-audit`, `readme-audit`, `workflow-loop`, `workflow-consolidate`, `git-commit-tag`, `standards-worklog`, `git-repo-init`) to call `/changelog-write` or `/changelog-audit` individually per which mode each site actually invokes — never a blind find-replace
  * **Done when:** `! test -d claude-code/skills/changelog && ! grep -rl "\`changelog\`\|/changelog write\|/changelog audit" claude-code/ templates/`
* **SUB-01.3.3.4** update `claude-code/settings.json`'s `changelog: name-only` entry into the two new skill names (or drop, per the token budget)
  * **Done when:** `bash scripts/validate.sh`

#### [TSK-01.3.4] Rename `readme` to `readme-modify`, pairing its name with the existing `readme-audit` [P: L] [TODO]
* **SUB-01.3.4.1** rename `claude-code/skills/readme/` to `claude-code/skills/readme-modify/`, updating its `name:` frontmatter field (`paths: "**/README.md"` trigger unchanged)
  * **Done when:** `test -f claude-code/skills/readme-modify/SKILL.md && ! test -d claude-code/skills/readme`
* **SUB-01.3.4.2** update the 3 cross-reference sites (`workflow-loop`, `workflow-consolidate`, `git-repo-init`) to `readme-modify`
  * **Done when:** `! grep -rl "\`readme\`\|/readme\b" claude-code/ templates/ | grep -v readme-audit`
* **SUB-01.3.4.3** update the `readme: name-only` key in `claude-code/settings.json`'s `skillOverrides` to `readme-modify`
  * **Done when:** `grep -q '"readme-modify"' claude-code/settings.json`
* **SUB-01.3.4.4** re-run the full validator
  * **Done when:** `bash scripts/validate.sh`

#### [TSK-01.3.5] ~~Trim `write-comments` to remove judgment content duplicated in `commentor.md`~~ [P: M] [OBSOLETE]
* Superseded by the opposite decision: `commentor.md` was deleted and its judgment content folded *into* `write-comments/SKILL.md`, with the mechanical taxonomy split out to `write-comments/reference.md`. The skill now runs as an agent-less `context: fork`, so there is no second file to deduplicate against.

#### [TSK-01.3.6] Open naming question: is `workflow-<phase>` the right fixed prefix for the 5-phase spec→decompose→execute→consolidate→publish loop (`workflow-loop`, `workflow-decompose`, `workflow-implement`, `workflow-consolidate`, `workflow-complete`, `workflow-debug`), or does a different prefix communicate the loop's role more clearly — user-raised, `harness-*` named as one candidate [P: L] [TODO]
* **SUB-01.3.6.1** decide whether to rename, and to what — `harness-*` collides with existing terminology Claude Code's own system prompt already uses (a `# Harness` section describing the CLI tool/environment itself), so `harness-loop`/`harness-implement` would likely read as "the CLI's own loop," not "this toolkit's SDLC loop"; weigh candidates that don't overload a term the host tool already owns (e.g. `sdlc-*`, `loop-*`, or keeping `workflow-*`) against `docs/design-decisions.md`'s existing "Skill naming buckets" table before picking — this is a user decision, not one this task resolves on its own
  * **Done when:** `docs/design-decisions.md`'s naming-buckets section names the chosen prefix (or explicitly keeps `workflow-*`) and states why
* **SUB-01.3.6.2** if renamed, update all 6 skill directories, every cross-reference across the toolkit (`agents/`, other `skills/*/SKILL.md`, `AGENTS.md`, `README.md`), and `claude-code/settings.json`'s `skillOverrides` keys to match
  * **Done when:** `! grep -rl "workflow-loop\|workflow-decompose\|workflow-implement\|workflow-consolidate\|workflow-complete\|workflow-debug" claude-code/ templates/ README.md` (after the rename; this check is meaningless before SUB-01.3.6.1 decides)

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1
* **Context (2026-09-08):** Two independent sessions surfaced evidence that the loop and the hook layer both lack circuit breakers. A single-file review chain ran 8 forked adversarial passes for work scoped as 3 bugs (this session, ~65% of a Max budget). A `workflow-loop` skill diagnosis from a different session found 4 concrete waste patterns in unrelated bands (premise-blind forking, missed fan-out, an unpinned mechanism choice, backlog re-reads). A live in-place hook edit produced two total Bash-tool outages because `git_guard.py`'s crash handler denies every command indiscriminately, not just risky ones. None of these are one-off bugs — they're gaps in the loop's and the hook's own safety design.

#### [TSK-01.4.1] `workflow-loop`'s P2.3/P2.4 review phases have no cap on repeated review-fix cycles against the same file within one band — a review call can find a new Critical/High finding round after round with nothing telling the loop to stop auto-continuing and escalate to the user instead; confirmed this session: 5 `assess-security` + 3 `assess-bugs` forked calls against one ~250-line file in a single band, each finding a new bypass shape in the same fix mechanism [P: H] [TODO]
* **SUB-01.4.1.1** add an explicit round-cap rule to `ways/sdlc.md`'s P2.3/P2.4 sections, mirroring the existing implement-side "the same check failing twice with the same error is the stop signal" rule: if a review call finds a new Critical/High finding on the same file for the 3rd consecutive round in one band, stop auto-continuing — file the finding, state the round budget is spent, and surface the decision (fix now / risk-accept and ship / defer to a follow-up task) to the user rather than looping again
  * **Done when:** `grep -qi "consecutive round" claude-code/skills/workflow-loop/ways/sdlc.md`
* **SUB-01.4.1.2** lower the round budget to 2 (not 3) when the touched file is already flagged in `BACKLOG.md` as a hand-rolled parser or security boundary with an open adversarial-hardening story — adversarial review of hand-rolled shell/parser code is close to open-ended by construction, and the loop should budget for that up front rather than discover it mid-band
  * **Done when:** `grep -qi "hand-rolled parser" claude-code/skills/workflow-loop/ways/sdlc.md`

#### [TSK-01.4.2] `workflow-loop` runs `assess-bugs`/`assess-security` twice against nearly the same diff for a single-task band — once per-task at P2.3, then again per-band at P2.4's closeout, with no check for whether the band's only task was already cleared at P2.3 with no diff change since [P: M] [TODO]
* **SUB-01.4.2.1** state in P2.4's section of `workflow-loop/SKILL.md`: skip `assess-bugs`/`assess-security` at closeout when every task in the band was individually cleared at P2.3 and the diff hasn't changed since — run only `assess-simplify` (never covered at P2.3) plus the perf suite in that case
  * **Done when:** `grep -qi "already cleared at P2.3" claude-code/skills/workflow-loop/SKILL.md`

#### [TSK-01.4.3] A closeout review pass's confidence bar doesn't rise as the round count climbs — nothing in `ways/sdlc.md` tells a review call invoked for the Nth time in one band to raise its bar to "clear, concrete, reproduced" rather than "theoretical corner worth naming," so that discipline currently depends on whoever is orchestrating remembering to type it into the brief each time [P: M] [TODO]
* **SUB-01.4.3.1** add a standing rule to `ways/sdlc.md`'s P2.3/P2.4 section: every review call's brief states its round number for this band, and from round 2 onward instructs the reviewer to report only clear, concrete, reproduced findings — not a theoretical edge case in the underlying grammar/format the fix touches
  * **Done when:** `grep -qi "round number" claude-code/skills/workflow-loop/ways/sdlc.md`

#### [TSK-01.4.4] A real, enforced skill-retry counter — distinct from the prose round-cap rule in TSK-01.4.1, which still relies on the orchestrating session honoring it — to prevent burn/doom cycles where the same skill re-invokes against the same target with no forward progress, across any phase, not just P2.3/P2.4 review [P: H] [TODO]
* **SUB-01.4.4.1** design and document the mechanism (e.g. a per-band call tally the orchestrating session must state and check before each repeat invocation of the same skill against the same file/task, or a lighter self-report convention each `assess-*`/`workflow-implement` result carries) — this needs a design decision, not just a prose rule, since `workflow-loop`'s existing "same check failing twice" and Claude Code's own "8 consecutive Stop-hook blocks" precedents are both informal or session-local; record the choice in `docs/design-decisions.md`
  * **Done when:** `docs/design-decisions.md` names the chosen retry-counter mechanism
* **SUB-01.4.4.2** wire the mechanism into `workflow-loop/SKILL.md`'s P2.1/P2.3/P2.4 sections, applying uniformly to `workflow-implement` retries and `assess-*` review rounds
  * **Done when:** `grep -qi "retry counter\|call tally" claude-code/skills/workflow-loop/SKILL.md`

#### [TSK-01.4.5] `workflow-loop` has no per-stage timing instrumentation, so a session (or the user, after the fact) can't see where a run's time actually went without manually reconstructing it from the transcript — confirmed this session, where the user had to ask and the only available answer was a call-count reconstruction, not real durations [P: M] [TODO]
* **SUB-01.4.5.1** add lightweight per-phase timing to `workflow-loop`'s execution (P0–P4, and each forked skill invocation within them), captured silently
  * **Done when:** `grep -qi "timing" claude-code/skills/workflow-loop/SKILL.md`
* **SUB-01.4.5.2** state that the metrics stay silent unless the user explicitly asks for them (e.g. "how long did that take") — never printed by default, never part of a phase's own `Ends when:` report
  * **Done when:** `grep -qi "silent unless" claude-code/skills/workflow-loop/SKILL.md`

#### [TSK-01.4.6] `git_guard.py`'s `main()` catch-all converts *any* exception — a policy violation and an internal crash alike — into the same `respond_deny`, so a single unbound-name bug in the hook denies every Bash tool call, `echo hello` as firmly as `git push --force`; confirmed via a live outage: `extract_substitutions` was extended to return a 3rd value without its caller being updated (`ValueError: too many values to unpack`), and a second, separate `NameError: name 'spans' is not defined` followed ~40 minutes later during the same in-place edit — both total stops, one requiring the user to unblock because the fix needed design intent the crash trace alone couldn't supply [P: C] [TODO]
* **SUB-01.4.6.1** split the crash handler: a genuine policy violation still denies (unchanged); an internal crash (any exception the guard's own logic raises, not a deliberate `respond_deny` call) falls back to a minimal, hardcoded check for unambiguously destructive operations (`git push`/`commit`/`rebase`/`reset`/`clean`/`filter-branch`, `rm -rf`, `sudo`) — deny only those, allow everything else through, and write the crash to stderr so it's visible rather than silent
  * **Done when:** `bash scripts/test-hooks.sh` passes with a new case simulating an internal crash (e.g. monkeypatching a scan function to raise) asserting `echo hello` is allowed and `git push origin main` is still denied
* **SUB-01.4.6.2** document the design rationale in `docs/design-decisions.md`: "the guard is broken" and "the command violates policy" are different conditions and must not collapse to the same outcome — a hook whose crash mode is indistinguishable from its policy-deny mode blocks 100% of Bash tool calls, not just the risky ones
  * **Done when:** `grep -qi "guard is broken" docs/design-decisions.md`

#### [TSK-01.4.7] `git_guard.py` is live via symlink (`claude-code/hooks/` is symlinked into `~/.claude/hooks/`, per this repo's own `AGENTS.md`), so a direct, non-atomic write to the source file is a live outage window for every session, this one included — a half-saved state mid-edit is the most likely explanation for the post-mortem's second failure (`NameError: name 'spans' is not defined`, not confirmed at the time since the outage blocked further investigation) [P: M] [TODO]
* **SUB-01.4.7.1** document in `AGENTS.md` (or wherever hook-editing guidance belongs) that a file under `claude-code/hooks/` is live the instant it's saved — editing it should go through a copy-then-atomic-rename step (write to a temp file in the same directory, `mv` into place) rather than a direct in-place tool write whenever the edit is nontrivial enough to risk a half-written intermediate state
  * **Done when:** `grep -qi "atomic" claude-code/AGENTS.md`

#### [TSK-01.4.8] No smoke test runs against `git_guard.py`'s actual runtime behavior separately from `python3 -m py_compile` — both outage states in the post-mortem passed `py_compile` cleanly (a syntax check, not a call-shape check), so the only thing that would have caught either failure before it reached a live session is exercising the hook end-to-end [P: M] [TODO]
* **SUB-01.4.8.1** add a small smoke-test set to `scripts/test-hooks.sh`, run first, before the full case list: pipe a fixed set of representative JSON payloads (reads: `echo`, `git status`, `ls`; writes/protected-ref ops: `git push origin main`, `tee BACKLOG.md`, `sed -i`, a redirect into a guarded path) through `git_guard.py` and assert the allow/deny split matches, so a call-shape break (mismatched unpack, undefined name) surfaces fast rather than only inside the full suite
  * **Done when:** `bash scripts/test-hooks.sh` runs the smoke set first and fails fast on a deliberately-introduced call-shape break (e.g. an extra return value with no updated caller)

#### [TSK-01.4.9] `workflow-implementer`'s standing rule "run the fork even when the code already appears to exist on disk" collapses two different situations into one instruction — it correctly stops a caller from skipping a fork just because code looks present, but nothing separately checks whether the task's own premise (the defect or gap it describes) still holds against current code, so a stale backlog entry can reach an implementer that builds against a premise the repo has already outgrown; confirmed cost elsewhere: a full implement + benchmark + review + revert cycle that shipped nothing because the described defect was already gone [P: H] [TODO]
* **SUB-01.4.9.1** add a premise-verification step to `workflow-implement/SKILL.md`, before the existing "run the fork even when code appears to exist" rule: read the function/file the task names and confirm the defect it describes is still present — a task whose premise the repo has outgrown goes back to `BACKLOG.md` re-aimed or removed, never forked as-is; a premise that still holds plus code that looks already-written is inherited state to verify, not a task to skip
  * **Done when:** `grep -qi "verify the task's premise" claude-code/skills/workflow-implement/SKILL.md`

#### [TSK-01.4.10] `workflow-loop`'s P2.1 dispatches tasks one at a time even when `/workflow-decompose`'s own `## Parallel` output already names a set with no shared file or dependency edge — the information needed to fan out is already computed and returned, and the loop currently has no instruction to act on it, so independent tasks run serially for no reason [P: M] [TODO]
* **SUB-01.4.10.1** add a rule to `workflow-loop/SKILL.md`'s P2.1 section: when `/workflow-decompose`'s `## Parallel` names a set and P2.0 confirms no shared file among them, dispatch that set together in one block with `isolation: worktree`, not sequentially — "once per task" bounds a fork's scope, never the order tasks start in; serialize only across a real dependency edge
  * **Done when:** `grep -qi "isolation: worktree" claude-code/skills/workflow-loop/SKILL.md`

#### [TSK-01.4.11] A `workflow-implement` brief that offers a design/mechanism choice without a preference lets the fork pick, and a wrong pick costs a multi-round fix cycle discovered only at review — confirmed elsewhere: a brief left the choice between two request-body-limiting mechanisms open, the fork picked the one that doesn't abort the connection and rejects a body of exactly the cap, and it took two further implement rounds (surfaced by review, not by the brief) to correct [P: M] [TODO]
* **SUB-01.4.11.1** add a rule to `workflow-loop/SKILL.md`'s P2.1 section (briefing an implement fork): when a task involves a genuine mechanism/design choice, not merely "write this function," the brief states the chosen mechanism and why, rather than leaving it to the fork's judgment — if truly undecided, that's a P0/P2.0 decision to make before forking, not something to hand off ambiguously
  * **Done when:** `grep -qi "states the chosen mechanism" claude-code/skills/workflow-loop/SKILL.md`

#### [TSK-01.4.12] Re-reading `BACKLOG.md` in full after an `assess-*`/`backlog-audit` fork already returned its `## Filed` block re-injects the entire file into the orchestrating session's context for no reason — the filed story text is already in the result; confirmed elsewhere: six `assess`/`audit` calls in one session each triggered a full-file re-read of `BACKLOG.md` [P: M] [TODO]
* **SUB-01.4.12.1** add a guardrail to `workflow-loop/SKILL.md`: a fork's result is the record — don't re-open a file it just wrote. Work from the returned `## Filed`/`## Changed` block; open the file directly only for a task the orchestrating session wasn't handed by a fork result
  * **Done when:** `grep -qi "don.t re-open a file it just wrote" claude-code/skills/workflow-loop/SKILL.md`

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

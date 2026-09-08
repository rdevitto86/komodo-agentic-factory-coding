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

#### [TSK-01.3.5] Trim `write-comments` to remove judgment content duplicated in `commentor.md`, keeping it solely as the fork-invocation entry point [P: M] [TODO]
* **SUB-01.3.5.1** remove `write-comments/SKILL.md`'s "Default: write nothing" section (bar plus good/bad examples) — owned solely by `commentor.md`'s fuller "Judgment: when a comment is warranted" section from here on
  * **Done when:** `! grep -q "Default: write nothing" claude-code/skills/write-comments/SKILL.md`
* **SUB-01.3.5.2** keep the "You cannot see the calling conversation" contract paragraph naming exactly what must arrive in `$ARGUMENTS`, and the numbered Order of operations — this is the skill's own entry-point contract, not duplicated in `commentor.md`
  * **Done when:** `grep -q "cannot see the calling conversation" claude-code/skills/write-comments/SKILL.md`
* **SUB-01.3.5.3** confirm the file still parses and the always-on budget is unaffected
  * **Done when:** `bash scripts/validate.sh`

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

# Changelog

Notable changes to komodo-agentic-toolkit-coding. Format follows Keep a Changelog; versions follow SemVer.

## [Unreleased]

## [0.47.0] — 2026-09-10

### Added
- New `git-branching-strategy` skill — when a change belongs on one short-lived branch off main versus a longer-lived feature branch of stacked PRs, cross-referencing `git-pr-create` for mechanics.
- New `standards-zig` skill — Zig memory/allocator, error-handling, comptime, build-system, testing, and C-interop conventions, structured like `standards-aws`.

### Changed
- `claude-code/AGENTS.md`'s atomic-write rule for a live `hooks/` file now names the Edit/Write tool as the sanctioned path, since `git_guard.py` already blocks the shell `mv` sequence the rule previously prescribed.

## [0.46.4] — 2026-09-10

### Changed
- `AGENTS.md`'s no-scope-expansion rule now explicitly covers formatting/lint reflow of untouched lines — drive-by reflow of a pre-existing line is out of scope even when the file is already open for another reason, since a shared file may carry another engineer's in-flight edit to that line.
- `git_guard.py`'s `repo_root_of()` now delegates its subprocess-call-and-except core to `lib.git.repo_root()` instead of duplicating it, closing the drift that let the same `TypeError` gap sit unfixed here after `lib/git.py` had already received the fix.

### Fixed
- `git_guard.py`'s `repo_root_of()` now also catches `TypeError`, matching the fix `lib/git.py`'s `repo_root()` already received — a non-str/bytes/PathLike `cwd` no longer crashes uncaught in this fail-closed-by-design hook.
- Root `AGENTS.md`'s hook-test-count comment updated from 244 to 259, matching `scripts/test-hooks.sh`'s actual case count.

## [0.46.3] — 2026-09-10

### Fixed
- `lib/git.py`'s `repo_root()` now also catches `TypeError`, so a non-str/bytes/PathLike `cwd` (e.g. from a malformed hook payload) fails open deliberately instead of depending on whichever caller's outer exception handler happened to rescue it.
- `standards-go/SKILL.md`'s two "modernize" transform bullets no longer hardcode a Go version number as their gate condition — both now phrase it relative to `go.mod`'s floor, consistent with the file's own stated convention.

## [0.46.2] — 2026-09-10

### Changed
- `git_guard.py`'s `extract_substitutions` no longer hand-rolls its own second copy of `split_segments`'s quote/comment/boundary state machine — both now share one `classify_shell_char` helper, and the two duplicated `$()`-capture call sites inside `extract_substitutions` collapsed to one.

### Fixed
- `git_guard.py`'s `segment_wants_reparse` no longer treats `command -v`/`command -V` (existence/type checks that never execute their argument) the same as `command eval` (which does) — fixes a false-positive deny on a harmless existence check.
- `git_guard.py`'s `env`/`time`/`nohup`/`xargs`/`command` recursion lost shell quoting on rejoin, so a wrapped `sh -c "multi word"` argument split apart on re-tokenize and only its first word was scanned — recursion now round-trips through `shlex.quote` to preserve the original token boundary.

### Security
- `git_guard.py`'s `env` wrapper handling only recursed on a literal `-c` token, but real `env` has no `-c` flag — `env <any guarded command>` bypassed every pattern the file checks, not only the reviewer-write case that first surfaced it. Fixed by composing the file's existing generic flag-stripper with its `VAR=val` walk instead of a narrow hand-rolled `-i`-only check.
- `git_guard.py` also treated `env -S`/`--split-string` as a discardable value flag; its argument is actually the wrapped command (the same role `-c` plays for `sh`/`bash`) and was silently discarded rather than scanned — now shell-split and recursively scanned like `sh -c`'s argument.

## [0.46.1] — 2026-09-10

### Added
- `workflow-loop`'s P2.3 band review now also dispatches `/assess-performance` conditionally, whenever a touched path is performance-sensitive (a hot loop, a changed complexity class, a new query/index) — restores coverage for the "Performance" standing closeout story that P2.2–P2.4's restructure (0.46.0) had dropped. Unlike the three forked finders, it runs inline and self-files, matching the contract `/assess-code-quality` already relies on.

### Changed
- `workflow-loop/SKILL.md`'s P2.3 now states the severity-floor rule and round-cap thresholds once, deferring to `ways/sdlc.md` by reference instead of restating them — the two copies had no mechanism keeping them in sync.
- `comments.py`'s `run_check` and `hook_findings` now share one `collect_findings` helper instead of independently building `MISSING`/`INVALID` finding dicts; a resulting double-sort (each per-file batch sorted twice) is fixed — `run_check`'s own final sort covers the multi-file case (`collect_files` walks the filesystem with no ordering guarantee), so the per-file sort moved to `hook_findings`, its only caller with no outer sort of its own.
- README.md, AGENTS.md, and `docs/design-decisions.md` reconciled against the whole `TG-01.4` band: the always-on token count (1,045, was stale at 949), the agent list (`reviewer` was missing), the hooks table (`comments.py hook` was missing a row), the test-case count (244, was stale at 160), the Mermaid diagram (a `backlog-audit`-at-P1 node that was never accurate and is now doubly wrong since `backlog-audit` left P2.4 too), and the mid-loop skill list (`assess-performance` added, `/repo-init`/`/readme` corrected to their current names).

### Security
- A harness-attested-token mechanism was built and self-tested as a structural fix for the risk-accepted `comments.py apply` gap 0.46.0 documented, then reverted after two independent bypasses were reproduced against the actual code: `env VAR=val cmd` bypasses a shell `readonly` guard by constructing the child process's environment directly, and the token's state file is an ordinary, world-readable file `reviewer`'s `Read` tool reads without ever touching `git_guard.py`'s Bash-only `PreToolUse` hook. The risk-accepted status from 0.46.0 stands; the attempt and its root cause are recorded in `BACKLOG.md` (`SUB-01.4.9.5`) so a future attempt doesn't rediscover the same break.

## [0.46.0] — 2026-09-10

### Added
- `comments.py` gained a hook subcommand: `PostToolUse` feedback on the just-touched file, reported as `additionalContext`, always exits 0. `workflow-implementer.md` registers the hook and adds a Comments-last craft step using `comments.py check`/`apply`; `write-comments` is now the manual/repair path instead of a downstream P3 fork.
- `workflow-consolidate` now clears a satisfied `[BLOCKED]` Recheck and confirms each shipped task's `Done when` commands before deleting it, band by band, at closeout; `backlog-audit` is repositioned as a full-file sweep a user types or a session runs on staleness, no longer invoked mid-loop.
- `verify_gate.py` now blocks with the configured limit named when `KOMODO_VERIFY_TIMEOUT` is hit (previously exited 0 silently), skips the verify command entirely on a dirty tree whose only paths are records-only (`BACKLOG.md`/`docs/BACKLOG.md`/`CHANGELOG.md`/`README.md`), and stops a stuck fork at 3 consecutive identical failure hashes (exits 0 with a `systemMessage`) instead of grinding toward Claude Code's 8-consecutive-block cutoff.
- `reviewer.md` and `workflow-implementer.md` now run at `effort: high`, matching `settings.json`'s sonnet/opus default — both had been silently running at `medium` because agent frontmatter overrides `modelSettings`. Every read-only agent gained a `maxTurns` cap sized to its job (`reviewer`: 80, `workflow-planner`: 50, `engineering`: 40, `scout`: 20); `workflow-implementer` gets none, since `verify_gate.py`'s `Stop` hook already bounds it.

### Changed
- `workflow-loop`'s P2–P3 phases rebuilt around a band review: P2.2/P2.3/P2.4 restructured into verify+commit / once-per-band review with a severity floor / changelog-only closeout, replacing per-task review and the P2.4 `assess-*` repeat. Removed the inaccurate `isolation: worktree` dispatch/merge-back instructions (the Skill tool has no `isolation` parameter) and stripped P3 of the `/write-comments` call and the retired removed-comments ledger.
- `reviewer` is now read-only: `Edit` dropped from its tools list and its `## Filed` output section removed — `assess-bugs`/`assess-security`/`assess-simplify`/`assess-code-conventions` return findings only, and the caller files them. `reviewer_guard.py` and its `settings.json` registration are retired; `git_guard.py`'s `is_guarded_path` now denies every Bash-side write for the `reviewer` agent unconditionally.

### Fixed
- The comments hook subcommand's file-read had re-rooted from the touched file's own git ancestry instead of `args.repo_root`, skipping the existing containment check and reading files unbounded — fixed to pin root to `args.repo_root`, gate every read through `is_within_repo_root`, and cap reads at 1MB.
- `workflow-complete`'s backlog-staleness check used `git log -S'backlog-audit'`, which false-positived on any commit whose diff merely mentions the string "backlog-audit" in prose (an unrelated docs edit, for instance) rather than an actual sweep — fixed to `git log --grep` against commit messages, paired with a documented convention that a `backlog-audit` run names itself in its own commit message.
- `verify_gate.py`'s new records-only skip had an unhandled 5s timeout on its own git-status probe that could silently skip a genuinely dirty tree — fixed so a probe timeout falls through to running verify normally instead of skipping.
- `git_guard.py`'s new reviewer-write deny for `comments.py apply` (see Security) closed four successive bypasses across a whole-band review: a bare interpreter-less invocation, a `python3 -`/stdin-piped script body, a same-content copy under an unrelated basename, and a case-folded path alias — moved from basename text-matching to `os.path.samefile()` identity plus a content-comparison fallback, run unconditionally per segment rather than gated behind a command-name allowlist.
- `workflow-loop/SKILL.md` and `docs/design-decisions.md` carried three stale references to P2.4 running review (`assess-*`) and to `reviewer` filing its own `## Filed` findings, both left behind by this band's earlier restructure — corrected to the current P2.3-only review, reviewer-files-nothing shape.

### Security
- Filed `TSK-01.1.17` (Critical): `git_guard.py`'s `env` wrapper handling only recurses on a literal `-c` token, but `env`'s real syntax (`env cmd args...`) has none — `env <any guarded command>` bypasses every pattern the file checks, not only the reviewer-write case that surfaced it. Pre-existing, unrelated to this band's changes; out of scope here, tracked under the existing `git_guard.py` hardening task group.
- Risk-accepted (not closed): `git_guard.py`'s reviewer-write guard for `comments.py apply` can still be defeated by a functionally-identical copy of the script that differs by even one byte, which passes both the identity check and the exact-match content fallback. Four rounds of fix-and-reloop closed the realistic bypass paths (missing interpreter prefix, stdin piping, basename spoofing, case-folding); this residual gap is a semantic-equivalence question no static command-text or content analysis can decide, the same class of limit this file's `eval`/variable-indirection gap already carries. Tracked as `SUB-01.4.9.4` for a structurally different fix (gating `apply` on something outside command-text analysis entirely) rather than a further pattern-match round.

## [0.45.0] — 2026-09-09

### Added
- `verify_gate.py` now tracks its own approximate consecutive-block streak per repo (keyed off the repo root, cleared on any pass or skip) and appends a warning to the block reason once that streak nears the 8-consecutive-block point where Claude Code stops honoring a `Stop` hook — previously a fork hitting that cutoff went silent with no in-repo signal.
- `standards-go` now documents `errors.AsType[T]` (Go 1.26+) and embedded-field composite-literal flattening (Go 1.27+, `gopls`'s `embedlit`), verified against real Go/gopls release material after an earlier verification pass incorrectly concluded neither existed.
- `scripts/test-hooks.sh` gained a regression suite locking in `reviewer_guard.py`'s fail-open behavior on malformed/malformed-typed input, mirroring `git_guard.py`'s existing coverage.

### Fixed
- `docs/design-decisions.md`'s "no SKILL.md is invisible" enumeration now includes `standards-c`, matching the parked-skill list elsewhere in the same doc.
- `scripts/test-hooks.sh` now pins `cwd` one way (a top-level JSON field) instead of two; the `cd`-into-fixture-dir mechanism is kept only as the dedicated regression test for `git_guard.py`'s `os.getcwd()` fallback.

### Security
- `git_guard.py`'s `extract_substitutions` now only unescapes a `$()` capture's backslash-escaped backticks when the enclosing segment actually re-parses via `eval`/`-c` — closes a real bypass (`eval "$(echo \`git push --force\`)"` previously went undetected) without the over-broad unescape that briefly followed it false-positiving on inert text merely quoting a banned command in escaped backticks.
- `git_guard.py`'s `segment_wants_reparse` now looks past a `command` prefix (with or without its value-less flags) at what it actually runs, so `command eval "$(...)"` re-parses the same way a bare `eval` already did.
- `git_guard.py`'s segment-boundary tracking now advances past a `#`-comment's closing newline before checking for `eval`/`-c` re-parse, closing a bypass where a preceding comment line let a hidden `eval` slip through undetected.
- Risk-accepted (not closed): a `$()` capture containing an escaped backtick, re-parsed via `eval`/`sh -c` reached through variable indirection (`RUN=eval; $RUN "$(...)"`), an alias, or a shell function rather than a literal leading token, is not statically detectable by this line-local scanner — recognizing it would require cross-statement data-flow tracking this scanner doesn't do.

## [0.44.0] — 2026-09-09

### Added
- `scripts/validate.sh` gained a "cross-skill reachability" check: it fails when a skill body invokes a sibling that carries `disable-model-invocation: true` and so cannot actually be reached via the Skill tool.
- `workflow-implement/SKILL.md` now verifies a task's premise before forking — it reads the function/file the task names and confirms the described defect is still present, so a stale backlog entry the repo has already outgrown is reported back instead of implemented.
- `workflow-loop/SKILL.md`'s P2.1 now requires the brief to state the chosen mechanism (and why) whenever a task involves a genuine design choice, rather than leaving it to the implement fork's judgment.
- `claude-code/AGENTS.md` now states that a file under `claude-code/hooks/` is live via symlink the instant it's saved, and requires a nontrivial edit there to go through an atomic temp-file-then-`mv` write.

### Fixed
- `scripts/validate.sh`'s budget pass no longer overstates the always-on token cost by folding in a `paths:`-gated skill's full description; it now excludes that cost (or counts only its name-only cost when `skillOverrides` collapses it), reporting the gated set as a separate figure.
- `repo-assess` can now compute a real 9-category composite score: `assess-readiness`, `assess-code-quality`, and `assess-testing` dropped `disable-model-invocation: true` (the human-decision gate stays on `repo-assess` itself), per the resolution recorded in `docs/design-decisions.md`.
- `scripts/test-hooks.sh`'s `bash_case`/`smoke_case` helpers now pin `cwd` to a fixed fixture repo instead of sending none, so their deny assertions no longer depend on `git_guard.py` falling back to the suite's own invocation cwd.

## [0.43.0] — 2026-09-09

### Added
- Five `standards-*` skills (`standards-go`, `standards-typescript`, `standards-python`, `standards-java`, the parked `standards-c.off`) gained a shared Dependency Inversion Principle statement and wrapping/magic-literal/guard-clause conventions, each phrased to the language's own idiom.
- `standards-go` gained a const/var zero-cost distinction, a sentinel-identity-comparison rule, a hard 120-col `lll` lint gate (`templates/go/.golangci.yaml`) with a documented 90-col soft wrap threshold, magic-number rule extensions for string literals/cross-package placement/const-block grouping, a repo-wide-sweep `-count=1` verification methodology, a full-inlining default for single-call-site helpers, and a testable-logging `Logger` interface pattern.
- New `assess-code-conventions` skill: a standalone, user-invoked style/formatting finder scoped to judgment calls no linter/formatter can mechanically decide; wired into `assess-code-quality` as a standing input.
- New `reviewer_guard.py` hook makes the `reviewer` agent's "edit only `BACKLOG.md`" boundary mechanical instead of prose-only, for both `Edit`/`Write` and (via a `git_guard.py` extension) `Bash` writes; hardened against leaf- and ancestor-directory symlink bypasses and a fail-open gap on an unresolvable repo root.
- `.github/workflows/verify.yml`: this repo's own `make verify` now runs in CI on pull requests and pushes to `main`, least-privilege `contents: read`.
- `setup.sh` now verifies `python3` resolves and meets the hook layer's floor (3.7) before linking anything.

### Fixed
- `comments.py` no longer defines `KNOWN_TEMPLATE_TYPES` twice; its default (changed-lines) check now folds in untracked-but-not-ignored files as wholly-changed, so a brand-new file is linted without needing `--all`.
- `comments.py check --all` no longer flags a file's leading shebang/header comment block for `STEP_MARKER`/`STACKED` (capped at 30 lines so unbounded narrative padding disguised as a header is still caught), and `BANNER_OUTSIDE_TEST` is now scoped to `.go` files only.

### Changed
- Recorded a naming decision: the `workflow-*` skill prefix stays as-is rather than renaming to `harness-*`, since this toolkit is agent configuration riding on top of whichever harness runs it, not a harness in its own right (`docs/design-decisions.md`).

## [0.42.0] — 2026-09-09

### Added
- `workflow-loop`'s P0-P4 phases (and each forked skill call within them) now record silent, session-stated timing — reported only when the user explicitly asks how long something took, never printed by default.

### Changed
- `workflow-loop`'s P2.4 closeout now dispatches `assess-bugs`/`assess-security`/`assess-simplify` in parallel (`isolation: worktree`, since all three write `BACKLOG.md`) instead of serially, matching the pattern already proven for P2.1's parallel task dispatch — the single clearest actionable slowdown a harness audit found in the review loop.

## [0.41.0] — 2026-09-09

### Added
- `workflow-loop`'s P2.3/P2.4 review phases gained a real round-cap: a third straight new Critical/High finding on the same file within one band stops auto-continuing and surfaces the choice (fix now / risk-accept and ship / defer) to the user instead of re-looping — the budget drops to 2 rounds for a file already flagged in `BACKLOG.md` as a hand-rolled parser or security boundary. From round 2 onward, a review call's brief also states its round number and asks for only clear, concrete, reproduced findings, not theoretical edge cases.
- A session-stated retry-counter tally now enforces that cap uniformly across P2.1/P2.3/P2.4 — the orchestrating session states "round N for `<file>`" before each repeat invocation of the same skill against the same file/task, replacing the prose-only "same check failing twice" rule that depended on the session remembering to apply it (`docs/design-decisions.md`: no new state file, tracked in P2.0's existing perpetual-context queue).

## [0.40.9] — 2026-09-09

### Changed
- `git_guard.py`'s three separately hand-rolled `MAX_SCAN_DEPTH` bound checks consolidated into one shared `check_depth()` helper; `repo_root_of` memoized with `functools.lru_cache` so a multi-argument write command (`cp`/`mv`/redirect/`tee`) reuses one `git` subprocess per `cwd` instead of spawning one per source argument.
- `scripts/test-hooks.sh`'s smoke-test cases `S4`/`S6` deduped against `G58`/`G107` (previously byte-identical commands) into distinct representative commands.

## [0.40.8] — 2026-09-09

### Added
- `workflow-decompose` returns a `## Parallel` section naming task sets with no shared file or dependency edge.

### Changed
- `workflow-loop`'s P2.1 dispatches a `workflow-decompose`-confirmed independent task set together with `isolation: worktree` instead of one task at a time, with P2.0/P2.2/P2.3 stating how per-task WIP tracking, verify/review/commit, and worktree merge-back work for that set. P2.4 skips the `assess-bugs`/`assess-security` repeat for a single-task band already cleared at P2.3 with no diff change since.

## [0.40.7] — 2026-09-09

### Changed
- Four model-visible skills (`standards-aws`, `readme-audit`, `git-merge-conflict`, `workflow-loop`) moved to `name-only` in `settings.json`'s `skillOverrides` — each was already reached only by explicit name but was paying its full description in the always-on listing. Dropped two dead `skillOverrides` entries (`workflow-authoring`, no skill directory; `standards-c`, disabled). Always-on budget drops from 1165 to 949 tokens.
- `workflow-loop`'s Guardrails section states: don't re-open a file a fork just wrote — work from its returned `## Filed`/`## Changed` block instead of re-reading the whole file.

## [0.40.6] — 2026-09-09

### Changed
- Renamed `repo-init` to `git-repo-init` and `readme` to `readme-modify` (pairing it with `readme-audit`), and split `changelog` into `changelog-write`/`changelog-audit`, matching the `backlog-modify`/`backlog-audit` precedent — cross-references across ~20 skills, `workflow-implementer.md`, `README.md`, and `settings.json`'s `skillOverrides` updated to match.

## [0.40.5] — 2026-09-09

### Security
- `git_guard.py` closed five more bypass/outage bugs: the redirect-operator regex missed a real file-descriptor redirect (`1>`, `2>`, `9>`, `&>`) into a guarded path; `sed`/`perl`-in-place detection missed a quoted or split-quoted `-i` flag (`"-i"`, `-'i'`) even though the shell passes it through unchanged, now normalized via `shlex`-based word reconstruction before matching; `is_guarded_path` denied a same-named/same-extension write anywhere on the filesystem with no check that the target actually resolved inside the repo, now resolved from the target's own location rather than the caller's `cwd` (closing a second bypass a review round found in the first fix, where a `cwd` outside any git worktree exempted an absolute write into a real guarded file); and `main()`'s catch-all converted any exception — an internal hook bug as much as a genuine policy decision — into the same deny, causing a full command-denial outage on a single unhandled crash. On an internal crash, `main()` now falls back to a minimal, hardcoded check for unambiguously destructive operations (`git push`/`commit`/`rebase`/`reset`/`clean`/`filter-branch`/`filter-repo`, `rm -rf` and its flag-order variants, `sudo`) and still denies those, allowing everything else through rather than either denying every command or allowing every command. A review round found that same fail-open-on-crash path itself reachable by attacker-controlled input (a deeply nested `$(...)` chain, as a sibling command or inside a redirect/`tee`/`cp`/`mv` target word, drove recursion past Python's limit and fell into the crash path): `scan_command`/`scan_segment` and `expand_word`/`resolve_command_output` now share a `MAX_SCAN_DEPTH` bound and deny once exceeded instead of raising. `scripts/test-hooks.sh` gained a fast-fail smoke-test set run before the full case list, plus `G134`–`G143` (202 total, up from 193).

## [0.40.4] — 2026-09-08

### Security
- `git_guard.py`'s redirect/`tee`/`cp`/`mv` guarded-path checks closed a chain of bypasses: a whitespace-bound regex missed a quoted or command-substitution target entirely, and the `sed`/`perl`/`python`-in-place-write and commit-trailer scanners ran against raw unmasked text, false-denying a command that merely quoted one of those patterns as data. A `resolve_guard_target`/`shell_words`/`expand_word` path now fully dequotes (mid-word splits included) and recursively resolves nested or concatenated `$(...)`/backtick substitutions before checking the target, and terminates correctly on an unmatched `)` closing a bare `(...)` subshell without truncating a literal parenthesis inside a real filename. Risk-accepted, not closed, same posture as `[0.37.2]`'s grep/sed/awk/curl gap: a `$(...)`/backtick body whose output is *computed* by the inner command at runtime (e.g. a `python3 -c` one-liner assembling a filename) rather than spelled or `echo`'d literally in its own arguments still evades the check — closing that in general needs either executing the substitution or denying every unrecognized substitution shape outright, both real costs against this hook's usability; mitigated by the toolkit's single-operator, non-hosted threat model. `scripts/test-hooks.sh` gained `G101`–`G124` (184 total, up from 179).

### Fixed
- `standards-go`'s `SCREAMING_SNAKE_CASE` guidance told the model to preserve legacy screaming-case consts as a package's intentional convention, contradicting idiomatic Go (stdlib and every major Go style guide use MixedCaps for exported consts, lowerCamelCase for unexported). Replaced with: never introduce a new `SCREAMING_SNAKE_CASE` const, exported or not; an existing one is grandfathered legacy and a rename-sweep candidate, never a pattern to continue. Also stated the boundary for an explicitly-requested repo-wide casing sweep — rename every exported identifier the repo itself owns, never one owned by an external/SDK package just because a local file references it.

## [0.40.3] — 2026-09-02

### Security
- `git_guard.py`'s redirect/`tee`/`cp`/`mv` guards (`is_guarded_path`, renamed this band from `is_code_path`) now also cover `.json` and `.md`, closing a side door where the `reviewer` agent's `Bash` grant could overwrite `claude-code/settings.json` or `BACKLOG.md` unexamined by `comment_guard.py`'s Edit/Write-only scoping — `settings.json` is the file that registers `comment_guard`/`git_guard` as hooks in the first place. `scripts/test-hooks.sh` gained `G99`/`G100` (199 total, up from 197).

### Added
- `claude-code/hooks/auto_format.py`'s formatter selection and invocation logic was extracted into a `run_formatter(path)` function; `write_comments_validator.py` now calls it after every successful comment splice, so a spliced comment gets the same `gofmt`/`prettier` pass a normal Edit/Write on that file type would already trigger. `scripts/test-hooks.sh` gained `V3`.

## [0.40.2] — 2026-09-01

### Fixed
- `git_guard.py` denied a `grep`/similar command whose pattern text merely mentioned a git verb inside backticks (e.g. a single-quoted pattern containing `` `git log` ``), wrongly reading the quoted text as a real command-substitution boundary. Replaced the quote-blind regex segmenter with a quote/comment/substitution-aware char-by-char scanner.

### Security
- Fixing the above false-positive surfaced and closed four Critical bypasses in `git_guard.py`'s command scanner: a `#` shell comment could swallow the quote-tracking state across a newline and hide a following mutating command; backtick and `$(...)` command substitution weren't recognized as segment boundaries at all, so a mutating `git` command hidden inside either form was invisible to the scanner; single-vs-double-quote suppression semantics were backwards (only single quotes suppress substitution in real bash — double quotes do not); and an old-style backslash-escaped nested backtick could hide a mutating command inside an outer substitution. `scripts/test-hooks.sh` gained cases `G90`–`G98` (196 total, up from 187) covering all of it.

### Changed
- `git_guard.py`'s `extract_substitutions` had its four near-identical backtick/`$(...)` capture-and-mask blocks collapsed into one shared `capture_and_mask` helper, and `find_backtick_end`/`find_paren_end` unified onto one `scan_masked_span` scanner parameterized by a terminator predicate — no behavior change, `scripts/test-hooks.sh`'s `G90`-`G98` cases still pass.

## [0.40.1] — 2026-09-01

### Changed
- `claude-code/skills/config-accessibility/SKILL.md` trimmed from 2,445 to 793 tokens, dropping sections that duplicated `CLAUDE.local.md`'s always-on rules while keeping the turn-end summary schema, density caps, emoji protocol, code-answer format, document typography, and self-check.

### Fixed
- `~/.claude/CLAUDE.local.md`'s dead `config-accessibility-output` skill reference (no such skill existed since a rename) corrected to `config-accessibility`, so the skill actually loads when the local file points to it.

## [0.40.0] — 2026-09-01

### Changed
- `workflow-loop`'s `SKILL.md` trimmed from ~5,077 to ~3,487 tokens (justification prose cut, every rule/table-row/`Ends when` line kept) so the file survives Claude Code's 5,000-token compaction re-attach cap; added a Guardrails bullet to re-read the active `ways/` file after any context compaction, since it loads via `Read`, not skill invocation.

### Added
- `scripts/validate.sh`: a "skill compaction cap" check that fails any `SKILL.md` exceeding 5,000 estimated tokens, reusing the existing base-context-budget check's shared directory walk and token estimator.

## [0.39.1] — 2026-09-01

### Changed
- `git-pr-create`'s P4 read now uses `git diff <base>..HEAD --stat` instead of the full diff, opening a single file's diff only when the stat line and commit messages leave the change genuinely ambiguous — cuts the orchestrator's per-band token cost at publish time.

## [0.39.0] — 2026-09-01

### Added
- `claude-code/agents/reviewer.md`: a new agent that reads a diff cold and files findings to `BACKLOG.md`, edit-restricted to that file only (enforced by a new `comment_guard.py` check, not just agent prose). `assess-bugs`, `assess-security`, and `assess-simplify` now fork into it instead of running in the orchestrator's own window; `backlog-audit` now forks into `workflow-implementer` the same way.
- `claude-code/hooks/lib/git.py`: a shared `repo_root()`, single-sourcing what `comment_guard.py`, `context_injector.py`, and `verify_gate.py` each previously defined separately.

### Changed
- Root `AGENTS.md` cut from ~7,157 to ~1,416 tokens — every "why it was decided this way" paragraph moved verbatim into new `docs/design-decisions.md`, keeping only what a session needs to act.
- `claude-code/AGENTS.md`'s claim that a subagent never inherits it was corrected — every custom agent and every forked skill actually receives it, and the four affected agent bodies (`engineering.md`, `scout.md`, `workflow-implementer.md`) had their genuinely-redundant restatements trimmed while keeping the "read-only git" rule each still needs (`git_guard.py` permits ordinary git verbs globally; it doesn't gate by agent identity).
- 17 bundled/plugin skill descriptions (`claude-api`, `dataviz`, `design`, and others) collapsed to `name-only` in `claude-code/settings.json`'s `skillOverrides`; `enableWorkflows` disabled since nothing in this toolkit uses the `Workflow` tool.
- `scripts/validate.sh` now states in its own output that its token total excludes bundled and plugin skills; the same caveat was added to `AGENTS.md`.

### Fixed
- `scripts/validate.sh` printed its bundled-skill NOTE twice (once per branch of the same budget check) — now prints once, unconditionally.

## [0.38.0] — 2026-09-01

### Added
- `write-comments` skill + dedicated agent — the only path that can add a code comment now; drafts proposals and splices them itself via a new deterministic validator, `write_comments_validator.py`, rather than editing files directly.
- `comment_removal_log.py` — a new `PostToolUse` hook that logs every approved comment removal to `.claude/state/removed-comments.jsonl` for later review.
- `workflow-implementer` gained an optional `## Comment Candidates` field for capturing live WHY-context at implementation time instead of writing an inline comment.

### Changed
- `comment_guard.py` now flat-denies any narrative comment addition (banner/`WHY`/`NOTE`/`FIXME`/`HACK`/`TODO`/step-marker) — no ask, no approval path; only a machine directive or the shebang-manual case still passes silently.
- Extracted comment-shape rules out of `comment_guard.py` into a shared module, `claude-code/hooks/lib/comment_rules.py`, so the guard and the new validator share one source of truth.
- `workflow-loop`'s P3 now runs `write-comments` once per band before its consolidate commit, fed the band diff, backlog stories, drafted commit message, and accumulated Comment Candidates, and clears the removal log afterward.
- `AGENTS.md`'s hook documentation and `standards-go`/`standards-python`'s comment-rule sections rewritten to match the new flat-deny behavior, replacing stale text describing the old ask-based guard.

### Fixed
- A same-edit comment removal previously short-circuited `comment_guard.py`'s deny checks, letting a narrative comment addition land unevaluated whenever the same edit also removed an existing comment — the single most common real-world case this redesign was meant to close.
- `write_comments_validator.py` rejected proposals whose `file` path resolved outside the target repo root (a `../`-escaping or absolute path could otherwise have been written to).

## [0.37.2] — 2026-08-31

### Changed
- Risk-accepted (not closed) the gap where allowlisted `Bash(grep:*)`/`Bash(sed:*)`/`Bash(awk:*)` and unrestricted `Bash(curl:*)` can read/exfiltrate the same secret-path files (`.env`, `*.pem`, `id_rsa*`, `credentials`) that `Read`'s deny list protects: Claude Code's `Bash(...)` permission matching is a fixed-prefix, wildcard-only-at-the-end language with no syntax for "this verb, wherever a secret path appears in its arguments" — a workable deny pattern for grep/sed/awk/curl would need real argument parsing (the same gap `git_guard.py` exists to close for git) which is new hook engineering, not a `settings.json` change. Mitigated today by Claude Code's own auto-mode classifier, this repo's single-operator (non-hosted, non-multi-tenant) threat model, and the absence of any live `.env`/`*.pem`/`id_rsa*`/`credentials` file in this repo's own tree.

### Removed
- `.github/workflows/windows-hooks.yml` — dropped GitHub Actions CI from this repo entirely; the local `scripts/hooks/git/pre-push-verify` dispatcher already runs this repo's own `.claude/verify.sh` (`test-hooks.sh` + `validate.sh`), the same coverage the workflow ran, so nothing regresses.

### Fixed
- `README.md`'s "Git hooks for other repos" section told a reader to run `git config core.hooksPath .githooks` — no `.githooks` directory has ever existed in this toolkit, silently disabling hooks rather than installing them. Replaced with the real `scripts/hooks/git/install.sh` usage.
- `README.md`'s "Diagram exception" note claimed only the embedded mermaid diagram was a deliberate carve-out from the `readme` skill's 6-section template, when the whole document's structure diverges. Rewrote it into a "Template exception" note naming every diverging section and why.
- `README.md`'s base-context token figure ("~894 tokens") and skill counts ("50 active, 3 parked") were stale against `bash scripts/validate.sh`'s current output and the real skill-directory count; updated to 1069 tokens and 60 active / 6 parked.

## [0.37.1] — 2026-08-29

### Added
- `scripts/validate.sh`: a `hooksPath` check that fails when `core.hooksPath` is set but doesn't resolve to a real directory, so a dangling path can't silently disable every git hook again.
- `scripts/validate.sh`'s budget pass now also reports the repo-root `AGENTS.md`'s token cost as a separate, ungated `root AGENTS.md` row (previously unmeasured entirely) — the BUDGET-gated total stays scoped to `claude-code/AGENTS.md`, the file actually loaded on every turn everywhere.

### Changed
- Corrected six stale facts in root `AGENTS.md`: the hook-regression-case count, a hooks table missing `auto_format.py`, a typed-only-skill list missing `backlog-prioritize`, a parked-skill list naming three of six `SKILL.md.off` skills, and a "stays full-description" claim naming only `workflow-loop` instead of it plus `standards-aws`/`git-merge-conflict`. Dropped a dead `write-*` skill grant from `claude-code/agents/workflow-implementer.md` — no `write-*` skill exists.

### Fixed
- `context_injector.py`'s `STORY` regex matched a pipe-delimited format nothing in this repo produces, so the `SessionStart` hook always reported "Backlog: 0 open" regardless of actual backlog state. Now matches the real `#### [TSK-E.T.S] <text> [P: SEV] [STATUS]` heading, uses `[IN_PROGRESS]` instead of the nonexistent `[WIP]` token, and excludes `[DONE]` stories from the open tally.
- `scripts/test-hooks.sh` ran ~11.5s serially with 2-3 subprocess calls per case; collapsed payload encode/decode into fewer subprocess calls and parallelized case execution behind a bounded worker pool (`TEST_HOOKS_PARALLEL`, default 8) with per-case isolated session ids and per-worker result files, cutting runtime to ~2s. The pool's initial `wait -n` throttle silently disabled itself on bash 3.2 (macOS's default `/bin/bash` doesn't support `wait -n`), letting every case run fully unthrottled; replaced with a portable busy-poll.
- This repo's `core.hooksPath` pointed at a pre-rename path that no longer exists, silently disabling `pre-commit-gofmt`, `pre-commit-hooks-syntax`, and `pre-push-verify`; repointed to the real in-repo `scripts/hooks/git`.

## [0.37.0] — 2026-08-28

### Added
- `scripts/install.py`: cross-platform installer for Windows, resolving the working `python3`/`python`/`py -3` interpreter and generating `settings.json` with an absolute, tilde-free hook `"command"` string instead of the static `"python3 ~/.claude/hooks/x.py"` that only worked on macOS/Linux. Symlinks `claude-code/` into the target the same way `setup.sh` does, falling back to a copy (with Developer Mode + re-sync guidance printed) when symlink creation fails.
- `docs/windows-install.md`: non-technical Windows install guide (Python detection, Git for Windows, Developer Mode, copy-fallback re-syncing, verifying the install).
- `.github/workflows/windows-hooks.yml`: CI verification of hook dispatch on `windows-latest`, covering both the Git Bash and PowerShell-fallback paths Claude Code actually spawns hook commands through.

### Changed
- `README.md`'s Setup section now documents macOS/Linux and Windows as separate install paths, the latter linking to the new install guide.

### Fixed
- `scripts/install.py`'s generated hook `"command"` string only double-quoted the hook path when it contained a space, with no other shell-metacharacter escaping — now quoted unconditionally with `shlex.quote()`, closing a command-injection-adjacent gap where a metacharacter-bearing path could reach `settings.json` unescaped and execute on every future hook invocation.
- `scripts/install.py`'s symlink calls never passed `target_is_directory`, which on Windows creates a broken file-type reparse point for a directory entry instead of a functional directory symlink.
- `comment_guard.py`'s Windows CI check asserted the wrong decision (`deny` instead of the real, tested `ask`) for an unallowed added comment; `context_injector.py`'s `docs/BACKLOG.md` display string baked in `os.path.join`'s native separator, rendering as `docs\BACKLOG.md` on Windows; the `test-hooks.sh` formatter no-op cases relied on an `ln -s` shim that needs the same symlink privilege this whole effort works around — now invokes the resolved interpreter by its full path instead.

## [0.36.0] — 2026-08-28

### Added
- `claude-code/skills/backlog-plan/SKILL.md` — new skill, the planning-run mode extracted from `backlog-modify`.

### Changed
- `backlog-modify` is now normalize-only; `workflow-loop`'s P0 invokes `backlog-plan` instead of `backlog-modify plan`.
- `claude-code/settings.json`'s `skillOverrides` adds `backlog-plan` as `name-only`; `AGENTS.md` documents the exception.

## [0.35.0] — 2026-08-28

### Added
- `scripts/hooks/git/pre-commit-hooks-syntax`, auto-discovered by the existing dispatcher, blocks `git commit` when the staged `git_guard.py`/`comment_guard.py` has a Python syntax error.

### Changed
- Extracted the tag-sync check duplicated across `changelog`'s "Tag sync" section and `workflow-complete`'s P4 step into a standalone `git-commit-tag` skill; both now invoke it by name.
- Split the `backlog` skill into `backlog-modify` (plan/normalize authoring modes plus the `BACKLOG.md` format spec) and `backlog-audit` (the verdict/audit mode, which edits `BACKLOG.md` directly rather than filing findings) — every caller across `claude-code/skills/`, `AGENTS.md`, `README.md`, and `settings.json` updated to match.

### Fixed
- `git_guard.py`: `PASSTHROUGH_VALUE_FLAGS["xargs"]` only recognized short-form value flags, so a deliberately constructed flag value colliding with a `MONITORED_COMMANDS` name (e.g. `xargs --delimiter git git push --force ...`) could hide the real wrapped command from scanning — same shape as the earlier `time` bug. `xargs`'s long-form value flags are now recognized too.

## [0.34.0] — 2026-08-28

### Added
- `readme` skill gains a Features section (`##2`) and an optional table of contents, both sourced from the new `templates/project/README.md.tmpl` reference skeleton.
- Documented an "Outdated dependencies" convention (`go list -u -m all`, `npm outdated`, `uv pip list --outdated`) alongside the existing CVE-scanner convention in `standards-go`, `standards-typescript`, `standards-python`.

### Changed
- `changelog`'s "Tag sync" step and `workflow-complete`'s P4 now create and push the release tag themselves (`git tag -a` + `git push origin <tag>`) instead of just handing the user a ready-to-run command — tagging a release is now an agent action, same as opening and labeling its PR.
- Renamed `audit-*` skills to `assess-*` (`audit-bugs` → `assess-bugs`, etc.) and `audit-readme` to `readme-audit`, across `AGENTS.md`, `settings.json`, `workflow-implementer`, and every skill/doc that referenced the old names — "audit" implied scoring against a fixed schema, while these skills' actual output is an open-ended judgement call, which "assess" states plainly.

### Fixed
- `claude-code/settings.json` listed `Bash(git tag:*)` in both `allow` and `deny` — deny silently won, so the agent could never actually create a release tag despite `git_guard.py`'s own logic already permitting non-destructive tag creation. Removed the stale `deny` entry.
- `AGENTS.md` falsely claimed git hooks are absent from this repo; it now describes the real `pre-commit`/`pre-push` dispatchers under `scripts/hooks/git/`, installed via `install.sh`.
- `git_guard.py`: `time --output sh <cmd>` (or any `PASSTHROUGH_WRAPPERS` flag whose unrecognized value collided with a `MONITORED_COMMANDS` name) let the guard misidentify the flag's value as the wrapped command, silently skipping its scan of the real inner command. `time`'s long-form `--format`/`--output` flags are now recognized as value-taking, closing that path.
- `README.md`'s "Workflow skills, all free" list had drifted to 16 entries against 32 actual free skills; refreshed to match.

## [0.33.0] — 2026-08-27

### Added
- `standards-api-design` — new API shape/contract conventions skill (resource naming, schema/pagination/error-shape conventions, idempotency-key and versioning design), sibling to `standards-api-security`'s security-only scope.
- `git-pr-review`, `git-pr-comment`, `git-issue-review` — new skills covering merge-readiness review, PR status comments, and open-issue triage, none of which existed before this band.
- "Security standards" sections (language-specific insecure-usage patterns) added to `standards-go`, `standards-python`, `standards-typescript`, `standards-java`, `standards-c`, `standards-dotnet`, `standards-shell`, `standards-csharp` (parked `.off`), for `audit-security` to pull from.

### Changed
- `changelog` merges `write-changelog` + `audit-changelog` (`write`/`audit` modes), same shape as the prior `backlog` merge.
- `standards-design-ui`/`standards-security-ui` renamed to `standards-ui-design`/`standards-ui-security`; `standards-security-api` renamed to `standards-api-security`; `rules-merge-conflicts` renamed to `git-merge-conflict`; `write-repo` renamed to `repo-init`; `config-accessibility-output` renamed to `config-accessibility`.
- `git-pr` split into `git-pr-create` (open/edit) + `git-pr-review` + `git-pr-comment`; `git-issue` split into `git-issue-create` + `git-issue-review`.
- `rules-source-control` and `standards-git` folded into `git-pr-create`'s new "Git lifecycle" section (branch/push/merge/protected-ref conventions merged with what `git_guard.py` actually enforces for each), since it's the skill that walks branch → commit → push → PR.
- Each `standards-<language>` skill's Comment discipline section now carries the full banned/allowed comment-template contract inline, instead of pointing at a shared `rules-commenting` skill.
- `audit-testing` now detects and loads every language manifest present in a repo, not just one; `audit-security` now also loads whichever `standards-<language>` skill(s) touched files trigger.

### Removed
- `write-changelog`, `audit-changelog` — folded into `changelog`.
- `rules-source-control`, `standards-git` — folded into `git-pr-create`.
- `git-pr`, `git-issue` — split into their `-create`/`-review`/`-comment` successors.
- `rules-commenting` — inlined into each `standards-<language>` skill.

### Known gap
- `AGENTS.md`'s `rules-<topic>` naming bucket now has no example skill (its sole example, `rules-commenting`, was removed) — left as-is pending a human decision on whether the bucket stays documented with no live example or a future skill should fill it.

## [0.32.0] — 2026-08-27

### Changed
- `write-backlog`, `audit-backlog` merged into `backlog` (`plan`/`normalize`/`audit` modes over one `BACKLOG.md` format); frontmatter carries neither `disable-model-invocation` nor `user-invocable` so it stays callable by name from `workflow-loop`'s P1.
- 25 cross-referencing files (`AGENTS.md`, `README.md`, `claude-code/settings.json`, and 22 skills) repointed from `write-backlog`/`audit-backlog` to `backlog`/`backlog audit`.

### Removed
- `write-backlog`, `audit-backlog` — folded into `backlog`.

## [0.31.0] — 2026-08-26

### Added
- Local docs layout: `templates/project/docs/{spec,adr,runbook}/` starter files (`docs/spec/SDD.md`, `docs/spec/PRD.md`, `docs/adr/template.md`, `docs/runbook/template.md`) and `templates/project/mkdocs.yml.tmpl` (MkDocs Material theme); `write-repo`'s Create path now scaffolds all four into every new project repo.
- `sdd`, `prd` — merged authoring + audit skills for `docs/spec/SDD.md`/`docs/spec/PRD.md`, replacing `audit-sdd`/`audit-prd`.
- `adr` — authoring + audit skill for `docs/adr/`, greenfield (no prior generator existed).
- `runbook` — merged authoring + audit skill for `docs/runbook/`, replacing `write-runbook`.

### Changed
- `standards-specs` moved off its Drive-fetch contract to read `docs/spec/SDD.md` and `docs/spec/PRD.md` as local repo files, and documents MkDocs Material as the paired doc-site toolchain.
- Stale Drive/Google-Doc references repointed to the local `docs/spec/` contract across `workflow-implementer.md`, `write-repo`, `write-backlog`, `workflow-loop`, `audit-testing`, `workflow-consolidate`, `write-readme`, `AGENTS.md`, and `README.md`.
- `claude-code/settings.json`'s `skillOverrides` gained `sdd`/`prd`/`adr` as `name-only`, matching `runbook`/`write-readme`.
- `AGENTS.md` documents the new "authors and audits one local doc type" skill bucket.

### Removed
- `audit-sdd`, `audit-prd`, `write-runbook` — folded into `sdd`, `prd`, `runbook` respectively; all callers repointed.

## [0.30.0] — 2026-08-26

### Added
- `standards-security-api`, `standards-security-ui` — split out of `standards-security` for API and UI-surface security conventions respectively.
- `audit-prd`, `backlog-prioritize` — new typed-only skills; the former audits a fetched PRD, the latter reorders/re-prioritizes existing `BACKLOG.md` tasks without inventing new ones.
- `BACKLOG.md` and `templates/project/BACKLOG.md.tmpl` migrated to the `EPIC-XX` → `TG-XX.Y` → `TSK-XX.Y.Z` → `SUB-XX.Y.Z.N` hierarchy; `write-backlog`'s format section rewritten to match, replacing the old flat `T.D.S` numbering.

### Changed
- Skills renamed to bucket prefixes: `generate-backlog`→`write-backlog`, `generate-readme`→`write-readme`, `generate-runbook`→`write-runbook`, `git-create-issue`→`git-issue`, `git-create-pr`→`git-pr`; `standards-uiux`→`standards-design-ui`. Cross-references updated across `AGENTS.md`, `README.md`, `workflow-loop`, and the affected `standards-*`/`audit-*` skills.
- `standards-security` split into `standards-security-api` + `standards-security-ui`.

### Removed
- `generate-adr`, `generate-sdd` (+ its `authoring.md`), `generate-repo`, `standards-docs`, `standards-prd`, `workflow-debug-hypothesize` — superseded or unused.
- `standards-security` removed after its split into `standards-security-api`/`standards-security-ui`.

## [0.29.0] — 2026-08-26

### Added
- `standards-dotnet`, `standards-react`, `standards-java` — new domain skills following the standard `standards-<noun>` section order.
- `standards-uiux-security`, `standards-api-security` — security domain skills distinct from `standards-uiux`'s WCAG/Tailwind scope and `standards-security`'s general OWASP baseline.

### Changed
- `workflow-planner`'s queue cap raised from 12 to 20 tasks.

### Removed
- `standards-csharp`, `standards-gcp`, `standards-azure` parked as `SKILL.md.off` — not used yet, no listing cost, content preserved for when those domains land.

## [0.28.0] — 2026-08-26

### Added
- `rules-merge-conflicts` — autoloaded rule governing merge-conflict resolution: what to resolve unattended, what to escalate, what never gets silently dropped.
- `git-create-issue`, `git-create-pr` — split out of the prior combined git-issue-filing/git-PR-opening skills, each invocable by the agent or the human directly.
- `standards-git` — git branch/commit/push/merge/protected-ref domain knowledge, split out of `rules-source-control`.
- `standards-aws`, `standards-gcp`, `standards-azure`, `standards-csharp` — new domain skills following the standard `standards-<noun>` section order.

### Changed
- `rules-source-control` — trimmed to enforcement only; convention content moved to `standards-git`.
- Every reference to the retired `write-git-issue`/`write-pr` naming updated to `git-create-issue`/`git-create-pr` across `workflow-loop` and other skills that cite them.
- `claude-code/agents/implementer.md` → `workflow-implementer.md` and `claude-code/agents/planner.md` → `workflow-planner.md`, with all cross-references updated.
- `scripts/validate.sh`'s `LANGUAGE_SKILLS` and `claude-code/settings.json`'s `skillOverrides` extended to register the new skills.

## [0.27.0] — 2026-08-26

### Removed
- `.github/workflows/ci.yml` — CI ran `scripts/test-hooks.sh`/`scripts/validate.sh` on every push and PR; moved local instead.

### Added
- `scripts/hooks/git/pre-push-verify` — runs a repo's own `.claude/verify.sh` on push when one exists, no-op otherwise (same presence-gated pattern as `pre-push-golangci`). Picked up automatically by the existing `pre-push` dispatcher once `core.hooksPath` points at this directory.

## [0.26.0] — 2026-08-26

### Added
- `write-pr` skill (renamed from `generate-pr-description`) + `.github/PULL_REQUEST_TEMPLATE.md` — opens a PR directly via `gh pr create`/`gh pr edit`, filling title, body, and a single label (`bug`/`documentation`/`duplicate`/`enhancement`/`do not merge`/`skill`) from the real diff against the repo's own template.

### Changed
- `git_guard.py` + `settings.json` — replaced the blanket git-mutation deny with protected-ref enforcement; the agent may now branch, commit, push its own branch, sync its branch with its protected base via `git merge`, and open a pull request, still barred from `main`/`master`/`trunk`/`prod`/`production`/`release/*`/`hotfix/*` and from history rewrites and landing into a protected branch.
- `rules-source-control` — rewritten for the new capabilities, the protected-ref list, the `PUBLISH_ENABLED` kill switch, and `git merge`/`git pull --ff-only` recovery.
- `write-changelog` — tag creation is fully the user's now; dropped the agent tag-creation instruction.
- `config-accessibility-output` — added the fixed ✅/❌/⚠️ turn-end change summary schema.
- `workflow-loop` P3/P4 — P3 now commits the band here (`write-commit-message`, then `git commit`) once `workflow-consolidate`'s read-only fork returns; P4 is renamed Publish and pushes + runs `write-pr` instead of printing a commit message to paste.
- `workflow-complete` — rewritten to match: pushes the branch and runs `write-pr`, no longer generates or prints a commit message.

## [0.25.0] — 2026-08-26

### Added
- `write-git-issue` — files a finding as a GitHub issue via `gh issue create`, confirming with the user first; uses `--title-file`/`--body-file` to avoid shell injection.
- `audit-vulnerabilities` — CVE-focused sweep: Dependabot triage first, falling back to `govulncheck`/`npm audit`/`pip-audit` per the relevant `standards-*` skill.
- `audit-dependencies` — staleness/deprecation/EOL sweep, explicitly scoped clear of `audit-vulnerabilities`' CVE focus.
- `standards-shell` — shellcheck, `set -euo pipefail`, quoting; added to `scripts/validate.sh`'s `LANGUAGE_SKILLS` list.
- `standards-prd` — the PRD's read contract (section map, fetch rule, requirement-ID scheme). A PRD is now a Google Drive doc, read-only from this toolkit, never a repo file.
- `workflow-loop/ways/debugging.md` and `workflow-debug-hypothesize` (planner-backed fork) — a debugging way of working, dispatched from `workflow-loop/SKILL.md` for diagnostic-phrased tasks.
- `claude-code/settings.local.json.tmpl` and `claude-code/CLAUDE.local.md.tmpl` — personal-prefs overlay, copied to `~/.claude/` by `setup.sh` only if absent.
- `setup.sh` — resolves and reports the latest git tag as the installed version (falls back to short SHA, then "unknown").

### Changed
- **PRD moved out of the repo.** `generate-prd` and its `authoring.md` are removed; `docs/sdd.md` is now the sole frozen spec that lives in a repo and stands on its own (never blocked on a PRD existing). `write-sdd`/`authoring.md` rewritten so PRD requirement IDs are cited only when a PRD has been supplied as context, never invented. `AGENTS.md` gained a "PRD and Google Drive" section stating there is no fallback tier from SDD to PRD — three fetch points only (`workflow-loop` P0, `write-backlog`, `audit-sdd`), all in the main session, since no fork carries MCP tools to reach Drive.
- `claude-code/settings.json` split: personal prefs (`model`, `theme`, `effortLevel`, `modelSettings`, `tui`, `autoMemoryEnabled`, `autoCompactEnabled`, `remoteControlAtStartup`, `agentPushNotifEnabled`, `autoMode`, `env`) moved to `settings.local.json.tmpl`; `skillOverrides` gained the new skills above.
- `claude-code/AGENTS.md`'s single-user statement and full §2 ADHD-conversation section moved to `claude-code/CLAUDE.local.md.tmpl`, wired via a new `@CLAUDE.local.md` import in `claude-code/CLAUDE.md`.
- `bridges/komodo-bridge/.mcp.json.tmpl` — migrated from SSE (`type: sse`, `/sse`) to streamable HTTP (`type: http`, `/mcp`).
- `standards-typescript`, `standards-c` gained `## Repo layout` sections stating Create is unsupported, matching `standards-python`'s existing pattern.

### Fixed
- `git_guard.py` — dropped dead-code read-only allowances for `git branch`/`git stash`; `settings.json` already denies both at the permission layer before the hook runs. `scripts/test-hooks.sh` updated to match (127 passing).
- `scripts/validate.sh` — the hooks-check loop now includes `auto_format.py`, which was previously silently skipped.

## [0.24.2] — 2026-08-25

### Changed
- `git_guard.py` — `git tag` creation (lightweight and annotated) is now allowed for the agent; `-d`/`-D`/`--delete`/`-f`/`--force` stay denied. `settings.json` moved `Bash(git tag:*)` from deny to allow and added `Bash(bash setup.sh:*)`. `scripts/test-hooks.sh` grew G48–G51 covering the new split (125 → 129 passing). `write-changelog` and `audit-changelog` updated to stop describing `git tag` as hard-denied.

## [0.24.1] — 2026-08-25

### Changed
- Repo renamed `komodo-agentic-tools-code` → `komodo-agentic-toolkit-coding` on GitHub; local refs updated in `README.md`, `AGENTS.md`, `CHANGELOG.md`, `bridges/komodo-bridge/agent-roster.md`, the working directory itself, and the git remote.
- `write-commit-message` — secondary description switched from a comma/`+`-delimited flowing line to a `-`-prefixed bulleted list, one bullet per distinct concern.

## [0.24.0] — 2026-08-25

### Added
- `audit-changelog`, `audit-readme`, `audit-sdd`, `audit-testing` — four typed-only audit skills scoring `CHANGELOG.md`, `README.md`, `docs/sdd.md`, and the test suite against their respective generator/standards skills, findings filed to `BACKLOG.md` unless `--report` is passed.
- `write-changelog` — a "Tag sync" section: before appending a version heading, check `.git/refs/tags/`/`.git/packed-refs` for a matching tag and hand the user a ready-to-run `git tag` line if one's missing (tagging stays hard-denied to the agent).

### Changed
- `standards-cicd`, `standards-docs`, `standards-observability`, `standards-security`, `standards-worklog` marked `user-invocable: false` — autoloaded knowledge, not meant to be typed directly.
- `write-readme` switched from `disable-model-invocation: true` to `paths: "**/README.md"`, so it now loads automatically the instant README.md is touched instead of requiring the typed command.
- `claude-code/settings.json`'s `skillOverrides` gained `name-only` entries for the mid-loop phases, `write-repo`, `write-readme`, and the audit/commit-message command skills, matching `AGENTS.md`'s reachable-by-name list.
- `AGENTS.md`'s skill-contract section rewritten: bucket table and typed-only list gained the four new audits, and the context-budget section replaced its by-hand skill enumeration with the general "reached only by an explicit name is `name-only`" rule.

### Removed
- `CHANGELOG.md`'s stale 2026-08-24 backfill note and empty `## [Unreleased]` heading.

## [0.23.1] — 2026-08-24

### Added
- `audit-backlog` — a verdict rule for backlog lines that name no checkable file, command, or artifact (a `Done when` like "once X is scoped"): `git blame`/`git log -S '<line text>'` its introduction and check `CHANGELOG.md` for whether the thing it references was ever real. No commit ever built it → Stale, not Valid. Closes the gap where "nothing in the repo contradicts it" read as Valid for a dead placeholder indistinguishable from a live one.

## [0.23.0] — 2026-08-24

### Added
- `.github/workflows/ci.yml` — runs `scripts/test-hooks.sh` and `scripts/validate.sh` on push/PR, closing the gap where the hook and budget regressions only ran locally.
- `auto_format.py` — new `PostToolUse` hook on Edit/Write, running `gofmt`/`.go` and `prettier`/JS-TS-CSS-etc after every write; no-ops (exit 0) when the formatter isn't on `PATH`, matching the fail-open policy of `verify_gate.py`/`context_injector.py`. Registered in `settings.json`; `scripts/test-hooks.sh` grew F1–F7 (118 → 125 passing).
- `standards-python` — a "Repo layout" section noting `write-repo` Create is unsupported for Python (no `templates/python/` needed, matching the other unsupported languages).

### Fixed
- `comment_guard.py` — `handle_pre` no longer recomputes `scan_comments` a second time in `check_echoes`'s caller; the result is cached as `scope_scanned` and reused for both the removed-comment check and echo suppression.

## [0.22.1] — 2026-08-24

### Fixed
- `git_guard.py` — `cp`/`mv` now scan their destination against `is_code_path` (plain and `-t`/`--target-directory` forms, including a not-yet-existing directory target), closing the bypass where copying a staged file over a code path skipped `comment_guard`. `scripts/test-hooks.sh` grew from 98 to 118 cases (G31–G47).
- `git_guard.py` — recurses into `eval`/`time`/`command`/`xargs`/`nohup` wrappers with flag-aware value stripping, so `time git commit` and similar wrapped invocations no longer dodge the guard.
- `comment_guard.py` — `check_echoes` suppression now scopes against the specific edit's own `old_string`/`old_text` context instead of a file-wide comment union, so a genuine new echo-comment violation is no longer masked by identical text existing elsewhere in the file.

## [0.22.0] — 2026-08-24

### Added
- `standards-docker` skill — base image pinning, distroless healthcheck pattern (a binary `healthcheck` subcommand, since a distroless runtime has no shell), `stop_grace_period` above the app's own drain timeout, `.dockerignore`, non-root, multi-stage layering. `write-repo` Step 5's container-practice bullets now point at it.
- Canonical `standards-*` section order (root `AGENTS.md` §Skill contract), `templates/skills/standards.md.tmpl` to start a new one from, and a `scripts/validate.sh` check enforcing it across `standards-go`, `standards-typescript`, `standards-python`, `standards-c`, `standards-vue`, `standards-svelte`, `standards-cdk`.
- `Makefile` in every layout-bearing `standards-*` skill's `Repo layout` (go-api, go-mcp, vue-ui, svelte-ui, cdk-infra) — `verify` chains lint → typecheck/vet → test → build, and is what `context_injector.py` already looks for as a repo's merge gate. `templates/go/Makefile` and `templates/node/Makefile` ship the concrete targets.
- `Toolchain` sections for `standards-vue` and `standards-svelte` — previously absent, which left those repos' `standards-cicd`-mandated CI security scans with no command to run.
- `templates/go/.gitignore` and `templates/go/.dockerignore`, wired into `write-repo` Step 5, added to `standards-go`'s two `Repo layout` trees.
- A `Tests:` seed story in every layout-bearing `standards-*` skill (go-api, go-mcp, vue-ui, svelte-ui, cdk-infra), and `test/` now materializes as `standards-sdlc`'s tier subtree on Create instead of an empty directory — closes the gap where a generated repo violated `write-backlog`'s "every domain with behavior stories carries its own `Tests:` story" rule from the moment it was created.
- `write-repo` Step 8 — a Create run now runs its own `make verify` and confirms every Step 2 seed story actually landed in `BACKLOG.md`, reporting either failure plainly instead of a closing report that assumes success.
- `workflow-loop` guardrails: a phase is complete only when its named skill actually ran (inherited on-disk state is not a pass); P1 treats a backlog holding only `write-repo`'s seed stories as undecomposed; P2.1 runs its fork even when code already appears to exist; P2.0/`workflow-decompose` treat a transitively-`[BLOCKED]` task as one to route around, not a full loop halt; never poll a delegated phase with `ScheduleWakeup`; a P2.3 finding needing standards verification returns to a fork rather than being re-checked in the orchestrating window.
- `ways/sdlc.md` P2.3 now states `/code-review` is invoked with the task text and which `standards-*` skills apply, rather than left to choose its own review lenses.
- `workflow-loop` P1 trusts a language manifest already on disk (`go.mod`, `package.json`, `cdk.json`) as the zero-token signal that Create already ran, re-invoking `/write-repo` only for an open Foundation-edge story or an explicit Scaffold/Refresh ask.

### Deviations from the original plan
- The scaffold-freshness signal landed as the manifest-file check above, not the originally proposed `.claude/scaffold.json` marker with a version-bumped "contract" integer — `write-repo`'s existing Scaffold/Refresh drift-detection against the language skill's `Repo layout` tree already covers invalidation, so the extra state would have been unused machinery.
- `c` was documented as scaffold-only alongside `typescript`/`python` in `write-repo`, rather than given a new `Repo layout` tree.

## [0.21.2] — 2026-08-24

### Changed
- Skills renamed to bucket prefixes (`standards-go`, `standards-python`, `standards-sdlc`, `standards-security`, `standards-svelte`, `standards-typescript`, `standards-uiux`, `standards-vue`, `workflow-*`); `worklog` split into `standards-worklog` so records and specs stop sharing one skill.

## [0.21.1] — 2026-08-23

### Changed
- `home/` renamed to `claude-code/`; the business-domain agent (`home/agents/business.md`) and the `decompose` skill removed — final narrowing of scope to software/hardware engineering only.

## [0.21.0] — 2026-08-21

### Added
- `home/skills/lifecycle/ways/sdlc.md`, `home/skills/worklog/SKILL.md`, `home/skills/risk-assessment/SKILL.md`, `templates/project/{BACKLOG,CHANGELOG}.md.tmpl`.

### Changed
- `scripts/doctor.sh` renamed to `scripts/validate.sh` and expanded; `scripts/test-hooks.sh` grew substantially.

### Removed
- `home/skills/wrap-up/SKILL.md`, `home/skills/tech-stack/SKILL.md`, the standalone QA agent file.

## [0.20.0] — 2026-08-18

### Added
- `home/skills/readme/SKILL.md`; `write-repo` gained a Create branch.

### Changed
- `backlog` skill gained a normalize mode; `[WIP]` tags replaced checkbox-style TODO items.

## [0.19.0] — 2026-08-10

### Added
- `comment_guard.py` positional-slot model (step marker, banner, structured note, script manual, machine directive) replacing free-text heuristics; `paths:`-based skill auto-activation.

### Removed
- `scripts/hooks/git/pre-commit-comments` — superseded by `comment_guard.py`.

## [0.18.0] — 2026-08-10

### Added
- `scripts/hooks/git/{pre-commit,pre-commit-gofmt,pre-push,pre-push-golangci,install.sh}` — installable git hook scripts; `templates/go/.golangci.yaml`; `home/skills/tech-stack/SKILL.md`, `typescript/testing.md`, `uiux/SKILL.md`.

### Removed
- `home/skills/{sql,stack,tailwind,tax,terraform,todo,wcag}/SKILL.md` folded away or superseded.

## [0.17.0] — 2026-08-08

### Changed
- Config restructured into `home/` as a literal mirror of `~/.claude/`; `comment_guard` rewritten from shell (`no-comments-guard.sh`) to Python (`home/hooks/comment_guard.py`); `go`/`python`/`svelte`/`typescript`/`vue` unified into `home/skills/*/SKILL.md`.

### Added
- `templates/project/AGENTS.md.tmpl`, `templates/project/CLAUDE.md.tmpl`, `templates/project/TODO.md.tmpl`, `scripts/test-hooks.sh`.

## [0.16.0] — 2026-07-27

### Added
- Senior-engineering doctrine + decomposition rule in `standards/principles.md`.

### Fixed
- `no-comments-guard.sh` no longer blocked a restored (previously deleted) comment.

### Changed
- `standards/testing.md` tier model rewritten.

## [0.15.0] — 2026-07-20

### Added
- `standards/assumptions.md` — when to assume vs. ask.

### Changed
- `CLAUDE.md`/`advisor`/`software-engineer` agents trimmed for context bloat.

## [0.14.0] — 2026-07-20

### Added
- `standards/communication.md` — ADHD-calibrated output rules, pulled into every agent's context.

## [0.13.0] — 2026-07-17

### Added
- `docs/orchestration.md` design spec (tier/profile/mode taxonomy, duty classes, model tier map), `audit`/`story` skills, `komodo-bridge/registry.generated.json`, `scripts/gen-bridge-registry.sh`, `scripts/set-runtime.sh`, `scripts/doctor.sh`.

### Changed
- Tests moved out of colocation into a per-tier `test/` tree (component/integration/e2e/chaos/perf).

## [0.12.0] — 2026-07-06

### Fixed
- `git-guard.sh` hard-blocked all `git push`/`git merge`, not just force-push — closed a bypass.

### Added
- `standards/findings.md` — mandatory confidence/source/why fields on every reported finding, to cut audit noise.

## [0.11.0] — 2026-06-26

### Added
- `profile/AGENTS.md` / `profile/CLAUDE.md` — universal cross-model directive split out from Claude-specific config; `standards/testing.md` (environment/tier matrix), `templates/Justfile.tmpl`.

### Changed
- `advisor` agent role rewritten as orchestrator with a defined cross-review gate.

## [0.10.0] — 2026-06-12

### Changed
- Repo flattened: dropped the `claude/` prefix, top-level `agents/`/`standards/`/`templates/`; hooks ported to `platforms/claude/hooks/*.sh` so the config is no longer Claude-only in structure.

### Added
- Comment-rule regression fixtures + `scripts/test-comment-rules.sh`, `scripts/validate-bridge-roster.sh`.

## [0.9.0] — 2026-06-09

### Added
- `swe/changelog.md` — first CHANGELOG/version-bump standard in the repo's own history.

### Changed
- `swe/git-flow.md` commit format redefined as short `+`-joined types; agent directives tidied for token usage; worktree isolation banned in `principles.md`.

## [0.8.0] — 2026-06-06

### Added
- `cyber-security`, `data-analyst`, `lawyer`, `machinist`, `marketing`, `tax-advisor` agents (each with an `email.md` companion where applicable), `project-manager/memory.md`.
- `.gitignore`, `scripts/validate-refs.sh`.

### Fixed
- `MEMORY.md` accidentally committed to the repo — removed and gitignored.

## [0.7.0] — 2026-06-06

### Changed
- Standalone `claude/standards/*.md` files folded into per-agent `claude/agents/swe/<domain>/` subtrees (`go/coding.md`, `python/coding.md`, `svelte/coding.md`, `ts/coding.md`, `db/sql.md`, etc.) — replaced a flat standards library with domain-scoped agent modes.

### Added
- `mechatronics/cpp/coding.md`, `swe/api/audit.md`, `swe/api/blueprint.md`, `swe/design/design.md`, `project-manager` docs.
- `_testing-go.md`/`_testing-ts.md` retired in favor of expanded `go/coding.md`/`ts/coding.md`.

## [0.6.0] — 2026-05-28

### Added
- Go service scaffold templates (`Dockerfile.tmpl`, `client.go.tmpl`, `main-fargate.go.tmpl`, etc.) under `claude/skills/templates/service/`.
- `standards/docker.md`, `standards/observability.md`, `standards/svelte.md`, `standards/komodo-context.md`.

### Changed
- `standards/comments.md` and `standards/testing-go.md` substantially rewritten; `testing.md` renamed to `testing-ts.md` to pair with the new Go-specific standard.

## [0.5.0] — 2026-05-18

### Added
- `advisor` agent, `swe-test` agent, `standards/testing.md`, `standards/token-efficiency.md`, `standards/comments.md`, `standards/todo.md`, `standards/principles.md`, `standards/python.md`, `standards/testing-go.md`.

## [0.4.0] — 2026-04-08

### Removed
- `customer-servicing`, `lawyer`, `marketing`, `sales`, most of `project-manager`, `quality-assurance`, `robotics` agents and their skills — first pass at narrowing scope off the original multi-industry roster.

### Added
- `swe-embedded` agent.

### Changed
- `CLAUDE.md` reworked to drop MCP-agent references now that the roster is trimmed.

## [0.3.1] — 2026-03-31

### Added
- Brief TODO-tracking and SDK-usage notes appended to `project-manager`/`swe` agent files.

## [0.3.0] — 2026-03-28

### Added
- `claude/standards/` — `api-design.md`, `go.md`, `logging.md`, `pull-requests.md`, `security.md`, `sql.md`, `typescript.md`, plus a standards `README.md` index.
- `git-flow.md` skill, `project-management/trello.md` skill.

## [0.2.0] — 2026-03-24

### Added
- Skills spanning agriculture, customer service, marketing, sales, project workflows, and engineering subfields (electrical, hardware BOM, mechanical, robotics ROS, API middleware, DB migration, Terraform) — the repo's original scope was cross-industry, not software/hardware-only.

### Changed
- Software UI skills (`add-route`, `new-component`, `new-page`, `new-service`) relocated under `engineering/software/ui/`.

## [0.1.0] — 2026-03-24

### Added
- `setup.sh` symlink installer, `claude/settings.json`, first skill set (`add-route`, `new-component`, `new-page`, `new-service`) under `claude/skills/`.

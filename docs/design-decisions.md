# Design decisions

Rationale moved out of root `AGENTS.md` to keep that file actionable — every fork loads the full `CLAUDE.md` hierarchy, root `AGENTS.md` included, so its rationale paragraphs were a fixed per-fork cost. Nothing here is a live rule; `AGENTS.md` and the skill files are the source of truth for current behavior. `git log -p` also finds the pre-move text if a decision's original wording matters.

## verify_gate.py runs on the agent, not settings.json

`verify_gate.py` is declared on the agent, not in global `settings.json`, on purpose. A `Stop` hook in an agent's own frontmatter only runs while that agent is active as a subagent, and Claude Code auto-converts it to `SubagentStop` — so it fires when a `workflow-implementer` fork (the `workflow-implement`/`workflow-consolidate` phases) finishes, and never in the primary interactive session. A verification gate on every casual turn burns tokens re-running a repo's test suite for edits nobody asked to be gated; scoping it to the fork means it only fires on work that came through `/workflow-loop`.

## The hook failure policy is inverted by design

The two guards fail closed — an unparseable payload denies, because a missed comment reaches disk. `verify_gate.py` and `context_injector.py` fail open — any internal error exits 0, because neither a broken verifier nor a broken injector may be able to brick a session.

`context_injector.py` reads disk only. It never probes the bridge — a session must not wait on a local model to start.

`verify_gate.py`'s check is opt-in per repo and silent otherwise. A repo declares its check as `.claude/verify.sh`, or a `verify` target in `Makefile` / `Taskfile` / `justfile`; with none of those present the hook does nothing. It also skips a clean working tree, so a fork that touched nothing never pays for a test run. Claude Code stops honouring a `Stop` hook after 8 consecutive blocks, so a permanently red suite cannot trap a fork.

`comment_guard.py` compares comment multisets, so adjacency and reindentation are irrelevant. It fails closed — an unparseable payload denies rather than silently passing.

Block comments and Python docstrings are scanned too, not just line comments — a `/* */` or `""" """` is denied on the same terms. Everything else denies, declaration docs included. A name-echo denies outright — a comment whose first word is the identifier on the next line carries no information.

Deleting a comment used to return `ask`, on the reasoning that a hard deny would make ordinary refactors impossible. That turned out to be the wrong trade: `comment_removal_log.py` already runs unconditionally on `PostToolUse`, independent of whatever `comment_guard.py` decided on `PreToolUse` — it diffs before/after itself and logs every removal regardless. The `ask` was never the actual safety net; the log was, and `commentor` already consults it. So the `ask` was pure per-edit friction with no corresponding loss in safety when removed — deletion now proceeds silently, scoped to the edit itself (an Edit's `old_string`, a MultiEdit's `edits[]`) exactly as before, and the log is what `commentor` checks for anything worth restoring.

There is no move ledger and no `+comments` grant. Both were removed with the guard rewrite. Moving a comment still takes `write-comments` to re-add it at the destination — that add denies unless it fits a template — but the deletion half no longer requires a separate approval.

No comment rule may ever block a commit, push, lint, or release. `comment_guard.py` is `PreToolUse` only. The `Stop` gate blocks a turn, never a git operation.

## The DOC carve-out is deliberately narrow, and gated by extension for a real reason

`comment_guard.py` allows exactly one shape of comment through inline: `DOC` (godoc-style, name-first, one sentence, on a newly-added exported top-level declaration). Every other type — `WHY`, `HACK`, `NOTE`, `FIXME`, `TODO`, `BANNER`, `STEP`, `FIELD` — still needs a diff, a `BACKLOG.md` story, or a commit message to judge whether it's warranted at all; a `PreToolUse` hook sees none of that, only the one edit in front of it. `DOC` is different: whether an exported Go declaration should carry a doc comment is close to pre-answered by Go convention itself, and the shape (name-first, one sentence, capped, top-level, exported) is fully mechanical to check. That's the line — mechanical shape-checking can replace judgment only where the judgment call itself barely exists.

The `.go`-only gate is not incidental. `DOC_DECL_PATTERN` matches on keywords (`func`, `type`, `const`, `var`, `package`), and `func`/`var`/`const` are also valid top-level Go-*and*-Swift-*and*-JS/TS syntax. Without the extension gate, a `const MaxRetries = 3` in a `.ts` file would pass the exact same shape check and get treated as legitimate Go-style godoc, which was never the intent — TypeScript has its own doc-comment convention (JSDoc block comments) that this taxonomy doesn't model at all. The gate is enforced twice, once in `comment_guard.py` and once in `write_comments_validator.py`'s `prevalidate_proposal`, both reading the same `DOC_LANGUAGE_EXTENSIONS` constant from `comment_rules.py` rather than each hardcoding `.go` separately.

## Trailing comments were a real blind spot, not a theoretical one

`scan_comments()` only ever checked `stripped.startswith(line_marker)` — a comment that starts a line. A comment appended to the end of a line with real code on it (`Timeout time.Duration // optional`) was invisible to `comment_guard.py`'s add/remove diffing and to `comment_removal_log.py`'s removal log, from the day both were written. It stayed academic only because nothing gave a session agent a reason to reach for a trailing comment specifically — until `FIELD` existed as a named, legitimate category for exactly that position. At that point the blind spot became a live bypass: a session agent could append a `FIELD`-shaped comment directly, with the guard never even registering it as an addition.

The fix is `find_comment_start()` in `comment_rules.py` — a small per-line tokenizer that tracks quote state (including Go's backtick raw strings, via the `raw_quotes` field `FAMILY_SYNTAX` had carried unused since the family-syntax table was first written) and returns the index of the first line-comment marker that isn't inside a string. `comment_guard.py` and `comment_removal_log.py` both use it now, alongside the original line-start scan, so a trailing comment is exactly as visible as a leading one on both the add and the remove side. Block comments (`/* */`) never had this gap — `scan_blocks()` already searched the raw text for delimiter pairs regardless of what shared their line, so the blind spot was specific to single-line markers (`//`, `#`, `--`).

## The no-op directive anecdote

An open content allowlist would be another slot, and would not work — the agent writes the content, so it can always emit the exempt token. Never add one. `no-op` was removed from the directive list for exactly this reason: it read as prose and let declaration docs through.

## Skill contract: disable-model-invocation and user-invocable

Typed-only workflow skills: `/assess-readiness`, `/assess-change-risk`, `/assess-code-quality`, `/assess-testing`, `/backlog-prioritize`, `/repo-assess`. These stay `disable-model-invocation: true` — a human decision to normalize a backlog or run an assessment should start from the user, not the model's own judgement. `backlog-audit` is a deliberate non-member of this set — it's invoked programmatically by `workflow-loop`'s own P2.4 once per band, the same loop-internal-machinery reason `workflow-decompose`/`workflow-implement`/`workflow-consolidate`/`workflow-complete` carry neither key despite also being judgement-shaped work; giving it `disable-model-invocation: true` would sever that call. `readme-audit` carries neither key — it dropped `disable-model-invocation: true` on purpose so a bare request ("audit the README") can reach it without the user typing the command.

`disable-model-invocation` is the only thing that controls cross-skill reachability — it says nothing about listing cost.

## Skill naming buckets

Skill naming follows seven buckets, front-loaded so related skills tab-complete and sort together:

| Bucket | Shape | Reason | Examples |
|---|---|---|---|
| Command — produces an artifact | `write-<noun>` | User invokes it to create/refresh a deliverable | none currently — `readme` is this bucket's one deliberate exception |
| Command — scores, finds, or files | `assess-<noun>` | User invokes it, or the loop invokes it mid-band, to score a diff or walk a diff/repo for defects — findings are filed to `BACKLOG.md` as stories unless `--report` is passed | `assess-bugs`, `assess-change-risk`, `assess-code-quality`, `assess-dependencies`, `assess-performance`, `assess-readiness`, `assess-security`, `assess-simplify`, `assess-testing`, `assess-vulnerabilities` (`readme-audit` and `repo-assess` are this bucket's deliberate exceptions) |
| Command — authors and audits one local doc type | bare `<doc-type>` | One file (or one-file-per-entry) type with both a fixed shape to author and a structural/drift check against it — merging the two into one skill beats a permanent `write-<x>`/`audit-<x>` pair when the shape and the check are this tightly coupled | `sdd`, `prd`, `adr`, `runbook`, `changelog` |
| Autoloaded rule — governs agent behavior | `rules-<topic>` | Loaded via `paths`/description, not typed; states what the agent must/must not do while writing | none currently |
| Autoloaded config — governs session/output behavior | `config-<topic>` | Loaded via description, not path-triggered; states how the agent must present itself, not what it writes | `config-accessibility` |
| Autoloaded knowledge — domain facts | `standards-<noun>` | Loaded via `paths`/description, or by name from a skill that needs it; states what is true about a language, tool, process, or external artifact | `standards-go`, `standards-api-security`, `standards-api-design`, `standards-sdlc`, `standards-worklog` |

`workflow-<phase>` is its own fixed prefix for the five loop phases and is never reused outside it.

## readme vs readme-audit split

`readme` is named bare, not `write-readme`, even though it stays a single-mode authoring skill — `README.md` is the one deliverable with no naming ambiguity a `write-` prefix would resolve, so the prefix was pure noise. This does not make it a bare-`<doc-type>` skill: `sdd`/`prd`/`adr`/`runbook`/`changelog` merge authoring and audit into one skill because both modes are meant to be model-reachable; README's audit stays split out on purpose — `readme` owns the format and is the only skill that writes it, `readme-audit` only ever files findings, and merging the two would blur that write/find division of labor `changelog`'s own audit mode keeps for itself.

## backlog-modify vs backlog-audit vs backlog-plan split

`backlog` split into `backlog-modify` and `backlog-audit` for a different reason than `readme`/`readme-audit` — never a `disable-model-invocation: true` human-decision gate. Its old audit mode (Part 3) already applied its verdicts directly to `BACKLOG.md`, the same way the other typed-only audits and `readme-audit` file findings for a human to act on — except `backlog`'s audit mode was never findings-only, so bundling it with the authoring modes under one bare `<doc-type>` skill stopped being the same shape as `sdd`/`prd`/`adr`/`runbook`/`changelog`.

`backlog-plan` was further extracted from `backlog-modify`'s own planning-run mode, by explicit user decision, not a functionality gap. It is the only doc-type skill broken this way — `sdd`/`prd`/`adr`/`runbook`/`changelog` all keep both their authoring and audit (or, for `backlog-modify`, both their planning and normalizing) modes merged into one skill specifically so the pair stays model-reachable through one skill at one listing cost. This is a documented, deliberate exception recorded here for that reason alone — it is not a pattern to copy the next time a doc-type skill's two modes feel worth separating.

## readme-audit's naming exception

`readme-audit` is the `assess-<noun>` bucket's one deliberate exception — kept as `readme-audit`, not renamed to `readme-assess` or `assess-readme`. The other four typed-only assessments (`assess-readiness`, `assess-change-risk`, `assess-code-quality`, `assess-testing`) took the bucket's rename because they already matched its old shape (`audit-<noun>`) word-for-word. `readme-audit` never did — it was named in the opposite order on purpose, to tab-complete and sort together with `readme` first. The bucket itself was renamed from `audit-<noun>` to `assess-<noun>` because "audit" implies scoring against a fixed, deterministic schema, while these skills' actual output is an open-ended, evidence-backed judgement call filed as a report — "assess" says that plainly.

## repo-assess's naming exception

`repo-assess` is the `assess-<noun>` bucket's other deliberate exception — named `repo-assess`, not `assess-repo`, by explicit user decision. Functionally it fits the bucket exactly, but it sits a level above every other member: it doesn't add its own review lens, it invokes all nine other `assess-*` skills and rolls their results into one composite score. The reversed word order marks that difference — a `repo-` prefix reads as the omnibus wrapper around the bucket, not one more instance within it.

## Every skill an autonomous workflow-loop run needs mid-loop carries neither key

So an agent can call it by name via the Skill tool the moment its phase is reached: `/workflow-loop` itself (must also be reachable by plain-language request), its phases `/workflow-decompose`, `/workflow-implement`, `/workflow-consolidate`, `/workflow-complete`, `/backlog-modify` (P0), `/backlog-audit` (P2.4, as a `workflow-implementer` fork), `/repo-init` (P2.1), `/assess-bugs`/`/assess-security`/`/assess-simplify` (P2.3/P2.4, each as a `reviewer` fork), `/git-commit-message` (P3), `/git-pr-create` (P4), and `/readme` (P0, `/workflow-consolidate`'s README-refresh step, `/repo-init`'s Scaffold/Refresh path). Each still declares `context: fork` + `agent: <name>` where it writes or does heavy reading — that isolation, not `disable-model-invocation`, is what keeps its work out of the orchestrating session's window. The forked ones cannot pause to ask, so whatever invokes them must supply every fact the skill would otherwise ask for up front.

## workflow-loop is the spine

Spec → decompose → execute → consolidate → complete, each phase naming what ends it. It exists so the build process is not re-derived every session, and so a delegated phase arrives with a brief that stands alone. `ways/sdlc.md` fills in what the gates mean for code; a second way of working is a second file, not a second machine.

## A forked skill is the only way to reclaim context

`context: fork` + `agent: <name>` + `background: false` runs the skill's body and its work inside a subagent and returns only the result — the calling window pays nothing for either. There is no way to unload a skill body once it is in the window, so a separate context window is the only way to reclaim one.

## A fork needs an agent, and that agent's output template is the phase's return contract

`workflow-implementer` exists because the read-only agents cannot write; `workflow-planner` exists because `engineering` returns a research report and a decompose phase must return a queue; `reviewer` exists because `engineering`'s Answer/Evidence contract does not fit a findings table. Adding a phase means asking which existing contract fits before adding a new agent.

## backlog-modify/changelog own the mutable-record formats; standards-worklog just the directive

`backlog-modify` and `changelog` own the format of the two mutable local records — `standards-worklog` is only the read/write directive shared across both, never their shape. Splitting the two records' formats out means a phase touching only one of them never pays for the other's. No skill in this toolkit authors the SDD — it lives at `docs/spec/SDD.md`, `standards-specs` owns its read contract and section map, and slice status never writes back into it.

## runbook owns operational procedures separate from the SDD

`runbook` owns the operational procedures the SDD points at. Editing a file under `docs/runbook/` should never pay for the SDD's read contract, and vice versa — the decisions a design earns live as appended sections inside the SDD's own §11, in `docs/spec/SDD.md`, never as a separate file.

## No fork reaches Drive or other MCP tools

`workflow-planner` and `workflow-implementer` declare no MCP tools, so a forked phase cannot fetch even if it wanted to — whatever it needs arrives in `$ARGUMENTS`. Nothing fetches at session start either, the same rule that keeps `context_injector.py` off the bridge.

## A skill with a procedure half gets a sibling file

`standards-sdlc/reference.md`, `standards-go/reference.md`, `standards-api-security/review.md` — the rule stays in `SKILL.md`, the how-to loads only when someone is doing that job.

## Standards-<language> skills share one section order

Every `standards-<language>`/`standards-<framework>` skill follows the same section order, so a language skill's shape never has to be re-derived from scratch: Comment discipline → Toolchain → Conventions → domain-specific sections → Testing → Quick-reference fields → `Repo layout — <token>` → `Seed backlog — <token>` → Reference material. A process/rule skill (`standards-cicd`, `standards-sdlc`, `standards-database`, `standards-worklog`, `standards-specs`) is exempt. Start a new one from `templates/skills/standards.md.tmpl`; `scripts/validate.sh`'s section-order check enforces it on the skills that already exist.

## A skill directory with no SKILL.md is invisible

`standards-gcp/`, `standards-azure/`, `standards-rust/`, `standards-csharp/`, `standards-hardware/`, and `standards-cpp/` are parked as `SKILL.md.off` — no listing cost, no loader entry, content preserved for when those domains land. Rename back to activate.

## Which compiled language, for which class of problem

Komodo builds hardware plus the software layer on top of it — AgTech, Manufacturing Tech, Warehousing Tech: sensors, cameras, lidar, robotic arms, printers, network hubs/controllers — with no OS work and no game development. The four compiled languages this toolkit carries (`standards-go`, and the parked `standards-rust`/`standards-c`/`standards-cpp`) are assigned by functional role, deterministically, with no gray area between any two of them:

1. **Go — the language/communications layer.** The API/server layer; every web service Komodo runs is Go. Settled, unrelated to the hardware/network layers below. This is why `standards-go` is the one active language skill; nothing else has landed yet.
2. **C — the nervous system.** Bare-metal/MCU firmware: no OS, tight resource budget, hard real-time, vendor HAL is typically C-only. Reflexive, low-level device control, wherever it appears in the stack — including underneath a C++-owned device, where the motor/sensor board's own firmware is still C.
3. **C++ — the muscles and peripherals.** The default for actuation and perception hardware: robotic arms, cameras, lidar, printers, and comparable complex peripherals. Chosen by domain fit, not by a per-device SDK check — this is where the mature ecosystem lives (OpenCV, PCL, ROS2, motion-planning libraries, manufacturer SDKs), and C++'s complexity is earned there rather than incidental. This is the default for the actuation/perception tier, not an override or an exception carved out of something else.
4. **Rust — the vocals.** The network layer only: hubs, controllers, routers, high-bandwidth communication devices. Memory-safe without a GC, scoped specifically to networking — not a general-purpose default for Linux-class hardware compute outside that role.

The boundary is drawn by what a device *does*, not by its compute class or toolchain availability: reflexive low-level control is C, actuation/perception intelligence is C++, network transport is Rust, server/API is Go.

This hierarchy is a single axis — hardware role — and it only ever evaluates device/firmware software. It says nothing about tooling that targets no device at all.

### Rust's second axis: toolchain and dev-tooling binaries

A CLI, linter, formatter, compiler, or language server is judged on a different axis entirely: **distribution shape**, not hardware role. A long-running process serving requests is Go, unconditionally — that is `standards-go`'s charter and this doesn't reopen it. A standalone binary that a developer or CI job runs directly — where a single static binary with no runtime, fast cold start, and CLI-grade throughput matter more than the web-service ecosystem Go is chosen for — is Rust's second use case. `ripgrep`, `ruff`, `swc`/`oxc`, and `biome` are the shape this targets: the modern answer to "make an existing slow dev tool fast" is Rust, not C/C++, both for the memory-safety case already made for the network layer and because `cargo` gives it a package manager and cross-compilation story neither C nor C++ has out of the box.

The two axes never collide because they never compete for the same artifact: hardware role only applies to firmware/device software, distribution shape only applies to software with no device target. A web service is never a candidate for Rust on this axis — that territory stays Go's — and a device's network firmware is never judged by distribution shape. Nothing here widens `standards-go`'s "every web service Komodo runs" charter, and nothing here makes Rust a general-purpose application language; outside a device's network role or a standalone dev-tooling binary, it has no claim.

### Why Rust over C++ specifically at the network layer

This is the one boundary in the hierarchy where memory safety outweighs C++'s ecosystem case, for reasons specific to networking and not to the actuation/perception tier:

- **Attack surface.** A hub/controller/router's core job is parsing untrusted, potentially adversarial input off the wire — packet parsers, protocol state machines, variable-length buffers. That is exactly the bug class (buffer overflow, use-after-free, double-free) that dominates CVE history in C/C++ network stacks; both Microsoft's and Chrome's security teams independently found roughly 70% of their memory-safety CVEs sit in this kind of code. Rust's borrow checker removes that class at compile time rather than relying on fuzzing or review to catch it after the fact.
- **Concurrency.** Network devices are inherently many-connections-at-once. Rust's ownership model catches data races at compile time too ("fearless concurrency"); C++ has no compiler-enforced answer to this, only TSan and review — and races are often load/timing-dependent, so they tend to surface in production traffic rather than in test.
- **Performance is a wash.** Both are AOT-compiled, zero-cost-abstraction, LLVM-backed languages, and network throughput is bottlenecked by I/O and syscalls, not language overhead — so this is not a reason to prefer either.
- **No equivalent ecosystem lock-in.** Unlike cameras/lidar/robotic arms, there's no mature C++-only SDK forcing the choice at the network layer. Rust's networking stack (`smoltcp` for embedded/no_std TCP/IP, `tokio` for async I/O) is mature and arguably purpose-built for hub/controller/router firmware specifically, so choosing Rust here gives up nothing the way skipping C++ elsewhere might.

This hierarchy — hardware role plus Rust's toolchain axis above — is why `standards-rust`/`standards-c`/`standards-cpp` sit parked rather than deleted — the domains are real and expected to land, just not yet active.

## No requires: frontmatter key — dependency direction matters

`standards-cdk`'s files are already `.ts`, so `standards-typescript` co-loads for free off its own unmodified glob. `standards-svelte`/`standards-vue` are different: their files aren't `.ts`, and `standards-typescript`'s `paths` must never be widened to name them — that would make the framework-agnostic root skill declare awareness of frameworks it doesn't need and can't shed. Instead `standards-svelte`/`standards-vue` each carry an explicit instruction telling the agent to invoke `standards-typescript` by name. This is a weaker guarantee (it relies on the agent following the instruction, not a deterministic glob match) but it keeps the dependency declared on the dependent's side, where it belongs.

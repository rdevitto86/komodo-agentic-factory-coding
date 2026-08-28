# komodo-agentic-toolkit-coding

Shared agent configuration for software/hardware engineering. `claude-code/` mirrors `~/.claude/` one-to-one and is symlinked there by `setup.sh`. Changing anything under `claude-code/` changes every project's next session.

## Layout

| Path | Becomes | Contents |
|---|---|---|
| `claude-code/AGENTS.md` | `~/.claude/AGENTS.md` | The universal rules, always loaded |
| `claude-code/CLAUDE.md` | `~/.claude/CLAUDE.md` | One line: `@AGENTS.md` |
| `claude-code/settings.json` | `~/.claude/settings.json` | Permissions and hook registration |
| `claude-code/agents/` | `~/.claude/agents/` | `workflow-implementer` writes; `workflow-planner`, `engineering`, `scout` are read-only |
| `claude-code/hooks/` | `~/.claude/hooks/` | Two guards, plus the Stop gate and the session injector |
| `claude-code/skills/` | `~/.claude/skills/` | Domain knowledge, lazily loaded |

Also: `templates/project/` (per-repo `AGENTS.md`/`CLAUDE.md`/`BACKLOG.md`/`CHANGELOG.md`) and `bridges/komodo-bridge/` (local LLM MCP bridge).

## The hooks

The two `PreToolUse` guards run **before** the write, so nothing lands on disk and no corrective edit is ever needed. `context_injector.py` runs at the primary session's edge; `verify_gate.py` runs at a fork's edge, never the primary session's.

| Hook | Registered on | Fires on | Does |
|---|---|---|---|
| `comment_guard.py` | `~/.claude/settings.json` | Edit, Write, MultiEdit | Denies an added comment; asks before one is deleted |
| `git_guard.py` | `~/.claude/settings.json` | Bash | Allowlists read-only git, denies in-place rewrites |
| `context_injector.py` | `~/.claude/settings.json` | SessionStart | Injects the `[WIP]` story, backlog tally, version, verify target |
| `verify_gate.py` | `claude-code/agents/workflow-implementer.md` frontmatter | Stop (auto-converts to `SubagentStop`) | Blocks the fork from returning while the repo's checks fail |

**`verify_gate.py` is declared on the agent, not in global `settings.json`, on purpose.** A `Stop` hook in an agent's own frontmatter only runs while that agent is active as a subagent, and Claude Code auto-converts it to `SubagentStop` — so it fires when a `workflow-implementer` fork (the `workflow-implement`/`workflow-consolidate` phases) finishes, and never in the primary interactive session. A verification gate on every casual turn burns tokens re-running a repo's test suite for edits nobody asked to be gated; scoping it to the fork means it only fires on work that came through `/workflow-loop`.

**The failure policy is inverted on purpose.** The two guards fail **closed** — an unparseable payload denies, because a missed comment reaches disk. `verify_gate.py` and `context_injector.py` fail **open** — any internal error exits 0, because neither a broken verifier nor a broken injector may be able to brick a session.

`context_injector.py` reads disk only. **It never probes the bridge** — a session must not wait on a local model to start.

It is opt-in per repo and silent otherwise. A repo declares its check as `.claude/verify.sh`, or a `verify` target in `Makefile` / `Taskfile` / `justfile`; with none of those present the hook does nothing. It also skips a clean working tree, so a fork that touched nothing never pays for a test run. Claude Code stops honouring a `Stop` hook after 8 consecutive blocks, so a permanently red suite cannot trap a fork.

`comment_guard.py` compares comment multisets, so adjacency and reindentation are irrelevant. It fails closed — an unparseable payload denies rather than silently passing.

An added comment is allowed only if it fills a **slot**, and every slot is defined by position and size, never by wording:

| Slot | Test | Why prose cannot use it |
|---|---|---|
| Step marker | 1 line, indented, ≤80 chars, **not adjacent to another comment** | A run of short lines is prose split across lines |
| Banner | 1 line, `--- Label ---`, label ≤40 chars and hyphen-free | The label slot is 40 chars between two hyphen runs |
| Structured note | `WHY:`, `NOTE:`, `FIXME:`, `HACK:`, `TODO(user):` + text | The prefix declares intent, not narration |
| Script manual | Line comments directly under a `#!` shebang | Only one block per file, only at the top |
| Machine directive | Prefix match against a fixed list | The list holds no prose token |

**Block comments and Python docstrings are scanned too**, not just line comments — a `/* */` or `""" """` is denied on the same terms.

Everything else denies, declaration docs included. **A name-echo denies outright** — a comment whose first word is the identifier on the next line carries no information.

Deleting a comment returns `ask`, never `deny`: a hard deny would make ordinary refactors impossible. **The deletion check is scoped to the edit itself** — an Edit's `old_string`, a MultiEdit's `edits[]` — so untouched comments elsewhere in the file never register as removed.

**There is no move ledger and no `+comments` grant.** Both were removed with the guard rewrite. Moving a comment therefore takes two approvals: the deletion asks, and re-adding it at the destination denies unless it fits a template.

An open content allowlist would be another slot, and would not work — the agent writes the content, so it can always emit the exempt token. Never add one. `no-op` was removed from the directive list for exactly this reason: it read as prose and let declaration docs through.

**No comment rule may ever block a commit, push, lint, or release.** `comment_guard.py` is `PreToolUse` only. The `Stop` gate blocks a turn, never a git operation.

## Skill contract

**The loader accepts exactly these frontmatter keys.** Any other key makes it reject the file silently — the skill simply does not exist at runtime.

`name` · `description` · `when_to_use` · `model` · `effort` · `allowed-tools` · `disallowed-tools` · `argument-hint` · `disable-model-invocation` · `user-invocable` · `paths` · `context` · `agent` · `background` · `hooks` · `metadata` · `shell` · `license` · `compatibility`

`paths` values must be quoted — a bare glob starts with `*`, which YAML reads as an alias.

| Kind | Frontmatter | Reaches the model | User types `/name` |
|---|---|---|---|
| Knowledge | `user-invocable: false` | Yes, via description | No |
| Workflow | `disable-model-invocation: true` | No, costs zero context | Yes |
| Both | neither key | Yes | Yes |

**Activation is path-based, not description-based.** `paths:` globs in a skill's frontmatter make the runtime load it when a matching file is touched. `skillOverrides` in `claude-code/settings.json` then collapses that skill to `name-only`, so its description costs nothing in the always-on listing. The pair is how a skill hot-swaps in: **`paths` decides when, `skillOverrides` decides cost.** A skill with neither pays its full description on every turn forever — reserve that for triggers no glob can express. `validate.sh` fails the build on an unknown key.

Typed-only workflow skills: `/assess-readiness` · `/assess-change-risk` · `/assess-code-quality` · `/assess-testing` · `/readme-audit`. These stay `disable-model-invocation: true` — a human decision to normalize a backlog or run an assessment should start from the user, not the model's own judgement.

`assess-performance`, `backlog`, `changelog`, `git-commit-message`, `assess-bugs`, `assess-security`, `assess-simplify`, `repo-init`, `readme`, and the four mid-loop phases (`workflow-decompose`, `workflow-implement`, `workflow-consolidate`, `workflow-complete`) all carry neither key: `/assess-code-quality` invokes the first internally as part of its conventions pass, `workflow-loop`'s P0 invokes `backlog plan` when a backlog has to be built from the SDD (its own Step 1 is where the user gets asked for refinement, not a second dialogue invented in P0) and its P1 invokes `backlog audit` before the decompose fork, `workflow-loop`'s P2.4 invokes `changelog write` for the whole band before P3 releases the section, `workflow-loop`'s P3 invokes `git-commit-message` directly (not inside the `workflow-consolidate` fork, which is read-only git) to commit the band once consolidate returns, `ways/sdlc.md` invokes the next three from P2.3/P2.4, and `workflow-loop`'s P0, `/workflow-consolidate`'s README-refresh step, and `/repo-init`'s Scaffold/Refresh path all invoke `readme` — a `disable-model-invocation: true` skill cannot be reached by another skill's instructions, only by the user typing its name, and several of these run inside forked, unattended phases so none of them can carry that key. **`disable-model-invocation` is the only thing that controls cross-skill reachability — it says nothing about listing cost.** None of these fourteen is ever chosen by the model reading its description out of a bare, undirected request: each is either named explicitly by an already-loaded parent skill's instructions or typed directly, so all fourteen collapse to `name-only` in `skillOverrides`. `workflow-loop` is the sole exception, kept at full description because it must fire from plain language alone with nothing already loaded to name it. The trade is the same one `repo-init` makes below: each pays full description cost so it stays callable by name.

**Skill naming follows the same seven buckets, front-loaded so related skills tab-complete and sort together:**

| Bucket | Shape | Reason | Examples |
|---|---|---|---|
| Command — produces an artifact | `write-<noun>` | User invokes it to create/refresh a deliverable | none currently — `readme` is this bucket's one deliberate exception, see below |
| Command — scores, finds, or files | `assess-<noun>` | User invokes it, or the loop invokes it mid-band, to score a diff or walk a diff/repo for defects — findings are filed to `BACKLOG.md` as stories unless `--report` is passed | `assess-bugs`, `assess-change-risk`, `assess-code-quality`, `assess-performance`, `assess-readiness`, `assess-security`, `assess-simplify`, `assess-testing` (`readme-audit` is this bucket's one deliberate exception, see below) |
| Command — authors and audits one local doc type | bare `<doc-type>` | One file (or one-file-per-entry) type with both a fixed shape to author and a structural/drift check against it — merging the two into one skill (`<mode>` argument) beats a permanent `write-<x>`/`audit-<x>` pair when the shape and the check are this tightly coupled; the type usually sits under `docs/`, but `backlog` and `changelog` apply the same reasoning to the two mutable records at repo root | `sdd`, `prd`, `adr`, `runbook`, `backlog`, `changelog` |
| Autoloaded rule — governs agent behavior | `rules-<topic>` | Loaded via `paths`/description, not typed; states what the agent must/must not do while writing | none currently |
| Autoloaded config — governs session/output behavior | `config-<topic>` | Loaded via description, not path-triggered; states how the agent must present itself, not what it writes | `config-accessibility` |
| Autoloaded knowledge — domain facts | `standards-<noun>` | Loaded via `paths`/description, or by name from a skill that needs it; states what is true about a language, tool, process, or external artifact | `standards-go`, `standards-api-security`, `standards-api-design`, `standards-sdlc`, `standards-worklog` |

`workflow-<phase>` is its own fixed prefix for the five loop phases and is never reused outside it. A new skill picks its bucket by what it's *for* — produces vs. judges vs. governs vs. informs — not by its `disable-model-invocation`/`user-invocable` mechanics, which can differ within a bucket (`readme` autoloads via `paths` same as a knowledge skill; `assess-readiness` is typed-only) as long as the name still says what the skill does.

**`readme` is named bare, not `write-readme`, even though it stays a single-mode authoring skill** — `README.md` is the one deliverable with no naming ambiguity a `write-` prefix would resolve, so the prefix was pure noise. This does **not** make it a bare-`<doc-type>` skill: `sdd`/`prd`/`adr`/`runbook`/`backlog`/`changelog` merge authoring and audit into one skill because both modes are meant to be model-reachable; README's audit stays split out and `disable-model-invocation: true` on purpose (see the Typed-only line above) — a human decision to normalize `README.md`, same as the other four typed-only audits. Merging it into `readme` would force that choice one way or the other, which nothing about either rename asked for.

**`readme-audit` is the `assess-<noun>` bucket's one deliberate exception — kept as `readme-audit`, not renamed to `readme-assess` or `assess-readme`.** The other four typed-only assessments (`assess-readiness`, `assess-change-risk`, `assess-code-quality`, `assess-testing`) took the bucket's rename because they already matched its old shape (`audit-<noun>`) word-for-word. `readme-audit` never did — it was named in the opposite order on purpose, to tab-complete and sort together with `readme` first, the same reasoning that kept `readme` itself bare instead of `write-readme` — so the bucket-wide `audit`→`assess` rename doesn't reach it. The bucket itself was renamed from `audit-<noun>` to `assess-<noun>` because "audit" implies scoring against a fixed, deterministic schema, while these skills' actual output is an open-ended, evidence-backed judgement call filed as a report — "assess" says that plainly. Nothing about `readme-audit`'s mechanics changed: still split from `readme`, still `disable-model-invocation: true`, still triggered only by a human typing the command, never the model's own judgement.

**Every skill an autonomous `/workflow-loop` run needs to invoke mid-loop carries neither key**, so an agent can call it by name via the Skill tool the moment its phase is reached: `/workflow-loop` itself (must also be reachable by plain-language request — "build this end to end" — not just the typed command), its phases `/workflow-decompose`, `/workflow-implement`, `/workflow-consolidate`, `/workflow-complete`, `/repo-init` (invoked from P2.1 when a task's `Done when` calls for a new repo's skeleton), `/assess-bugs`/`/assess-security`/`/assess-simplify` (invoked from P2.3/P2.4 for per-task review and band closeout), `/git-commit-message` (invoked from P3, once `workflow-consolidate` returns, to commit the band), `/git-pr-create` (invoked from P4 to publish it), and `/readme` (invoked from P0's doc-existence check, from `/workflow-consolidate`'s README-refresh step, and from `/repo-init`'s Scaffold/Refresh path — the latter two run inside forks with no human to type the command). Each still declares `context: fork` + `agent: <name>` where it writes or does heavy reading — that isolation, not `disable-model-invocation`, is what keeps its work out of the orchestrating session's window. The forked ones cannot pause to ask, so whatever invokes them (a queue task, a `BACKLOG.md` story) must supply every fact the skill would otherwise ask for up front. `$ARGUMENTS` reaches a forked skill normally, and `background: false` returns its result inline — both confirmed directly against `repo-init`, `workflow-decompose`, `workflow-implement`, and `workflow-consolidate`, which all carry `argument-hint` + `context: fork` with no `disable-model-invocation` key and register and invoke cleanly.

**A forked skill is the only way to reclaim context.** `context: fork` + `agent: <name>` + `background: false` runs the skill's body *and* its work inside a subagent and returns only the result — the calling window pays nothing for either. `/workflow-decompose`, `/workflow-implement`, and `/workflow-consolidate` are the workflow loop's three forked phases.

**`/workflow-loop` is the spine.** Spec → decompose → execute → consolidate → complete, each phase naming what ends it. It exists so the build process is not re-derived every session, and so a delegated phase arrives with a brief that stands alone. `ways/sdlc.md` fills in what the gates mean for code; a second way of working is a second file, not a second machine.

**The phase that reads a lot and returns a little runs in a fork.** There is no way to unload a skill body once it is in the window, so a separate context window is the only way to reclaim one.

**A fork needs an agent, and that agent's output template is the phase's return contract.** `workflow-implementer` exists because the read-only agents cannot write; `workflow-planner` exists because `engineering` returns a research report and a decompose phase must return a queue. Adding a phase means asking which existing contract fits before adding a fifth agent.

**`backlog` and `changelog` own the format of the two mutable local records — `standards-worklog` is only the read/write directive shared across both, never their shape.** Splitting the two records' formats out means a phase touching only one of them never pays for the other's. **No skill in this toolkit authors the SDD** — it lives at `docs/spec/SDD.md`, `standards-specs` owns its read contract and section map, and slice status never writes back into it.

**`runbook` owns the operational procedures the SDD points at.** Editing a file under `docs/runbook/` should never pay for the SDD's read contract, and vice versa — the decisions a design earns live as appended sections inside the SDD's own §11, in `docs/spec/SDD.md`, never as a separate file.

**No fork ever reaches Drive or any other MCP tool.** `workflow-planner` and `workflow-implementer` declare no MCP tools, so a forked phase cannot fetch even if it wanted to — whatever it needs arrives in `$ARGUMENTS`. Nothing fetches at session start either, the same rule that keeps `context_injector.py` off the bridge.

**A skill with a procedure half gets a sibling file.** `standards-sdlc/reference.md`, `standards-go/reference.md`, `standards-api-security/review.md` — the rule stays in `SKILL.md`, the how-to loads only when someone is doing that job.

**Every `standards-<language>`/`standards-<framework>` skill follows the same section order**, so a language skill's shape never has to be re-derived from scratch: Comment discipline → Toolchain → Conventions → domain-specific sections → Testing → Quick-reference fields → `Repo layout — <token>` → `Seed backlog — <token>` → Reference material. A process/rule skill (`standards-cicd`, `standards-sdlc`, `standards-database`, `standards-worklog`, `standards-specs`) is exempt — this governs only the skills a `.go`/`.py`/`.vue`/etc glob loads. Start a new one from `templates/skills/standards.md.tmpl`; `scripts/validate.sh`'s section-order check enforces it on the skills that already exist.

**A skill directory with no `SKILL.md` is invisible.** `standards-rust/`, `standards-cpp/`, and `standards-hardware/` are parked as `SKILL.md.off` — no listing cost, no loader entry, content preserved for when those domains land. Rename back to activate.

**There is no `requires:` frontmatter key, and the dependency direction matters.** `standards-cdk`'s files are already `.ts`, so `standards-typescript` co-loads for free off its own unmodified glob — no coupling needed either direction. `standards-svelte`/`standards-vue` are different: their files aren't `.ts`, and `standards-typescript`'s `paths` must never be widened to name them — that would make the framework-agnostic root skill declare awareness of frameworks it doesn't need and can't shed. Instead `standards-svelte`/`standards-vue` each carry an explicit instruction telling the agent to invoke `standards-typescript` by name. This is a weaker guarantee (it relies on the agent following the instruction, not a deterministic glob match) but it keeps the dependency declared on the dependent's side, where it belongs. Either way, the framework skill never restates a fact — naming, toolchain, Quick-reference rows — that `standards-typescript` already owns.

## No static references

**A skill records rules. It never records inventory.** Nothing in `claude-code/skills/` may name a live repo, a port assignment, a URL, a version number, an env var, or a file path inside another codebase. Those drift silently: the skill keeps asserting a fact months after it stopped being true, and an agent trusts it over the disk.

The replacement is always the same shape — **state the rule, then name where to read the current value.**

| Instead of | Write |
|---|---|
| `komodo-cart-api runs on 7041` | The naming rule + "read the repo's `AGENTS.md`" |
| `Go 1.26` | "the floor `go.mod` declares" |
| The SDK's package list | "read its package tree at the pinned version" |

**Language-agnostic skills name no tools.** `standards-cicd` and `standards-sdlc` state the contract — what a hook must gate, what a tier must cover. The linter, formatter, test command, SDK package, version floor, and reuse doctrine (SDK first, vetted library second, custom last) belong in `standards-go`, `standards-typescript`, `standards-python`, and the other language skills, which are allowed to be concrete because they are already scoped to one toolchain.

## Context budget

`AGENTS.md` plus every model-visible skill description is paid on **every turn of every session, forever**. `validate.sh` fails above **2,000 tokens** — the ceiling the runtime itself enforces by truncating descriptions past ~1% of the context window.

- **A skill listed `name-only` costs 1–4 tokens.** With a full description it costs ~30–70. Run `scripts/validate.sh` for the current base-context total.
- **A new line in `claude-code/AGENTS.md` costs its full length**, always. Put it in a skill unless it must apply unconditionally.
- **Typed-only workflow skills cost nothing** — `disable-model-invocation: true` keeps `/assess-readiness`, `/assess-change-risk`, `/assess-code-quality`, `/assess-testing`, and `/readme-audit` out of the listing entirely.
- **Everything reached only by an explicit name is `name-only`, full stop.** The four mid-loop phases (`/workflow-decompose`, `/workflow-implement`, `/workflow-consolidate`, `/workflow-complete`), `/repo-init`, `/readme` (also `paths`-triggered on `README.md`, so it still expands in full the instant the model is about to touch that file), the seven assess/commit command skills (`/assess-performance`, `/backlog`, `/changelog`, `/git-commit-message`, `/assess-bugs`, `/assess-security`, `/assess-simplify`), and `/git-pr-create` carry neither key — so a parent skill's loaded instructions or a typed `/name` can still reach them — but none is ever picked by the model reading a bare description, so all fourteen are `name-only` in `skillOverrides`.
- **`workflow-loop` is the one skill that stays full-description.** It is the sole plain-language entry point — "build this end to end" has to match its description with nothing else already loaded to name it.

## Working on this repo

```bash
bash scripts/test-hooks.sh    # 98 hook regression cases
bash scripts/validate.sh      # symlinks, frontmatter schema, token budget
bash setup.sh --dry-run       # preview the install
bash setup.sh                 # install, then runs both of the above
bash .claude/verify.sh        # what the Stop gate runs: both of the above
```

`.claude/verify.sh` is this repo's own opt-in for `verify_gate.py`. It only runs automatically inside a `workflow-implementer` fork (`/workflow-loop`'s implement/consolidate phases) finishing a dirty tree — editing this repo directly in a primary session does not trigger it, so run it by hand before ending a manual editing session.

This repo ships its own `pre-commit`/`pre-push` dispatchers under `scripts/hooks/git/`, installed into a target repo via `install.sh` (sets `core.hooksPath`, nothing is copied). The `standards-cicd` skill states the contract they must satisfy.

# komodo-agentic-config

Shared agent configuration. `home/` mirrors `~/.claude/` one-to-one and is symlinked there by `setup.sh`. Changing anything under `home/` changes every project's next session.

## Layout

| Path | Becomes | Contents |
|---|---|---|
| `home/AGENTS.md` | `~/.claude/AGENTS.md` | The universal rules, always loaded |
| `home/CLAUDE.md` | `~/.claude/CLAUDE.md` | One line: `@AGENTS.md` |
| `home/settings.json` | `~/.claude/settings.json` | Permissions and hook registration |
| `home/agents/` | `~/.claude/agents/` | `implementer` writes; `planner`, `engineering`, `business`, `scout` are read-only |
| `home/hooks/` | `~/.claude/hooks/` | Two guards, plus the Stop gate and the session injector |
| `home/skills/` | `~/.claude/skills/` | Domain knowledge, lazily loaded |

Also: `templates/project/` (per-repo `AGENTS.md`/`CLAUDE.md`/`BACKLOG.md`/`CHANGELOG.md`) and `platforms/komodo-bridge/` (local LLM MCP bridge).

## The hooks

The two `PreToolUse` guards run **before** the write, so nothing lands on disk and no corrective edit is ever needed. The other two run at the session's edges.

| Hook | Fires on | Does |
|---|---|---|
| `comment_guard.py` | Edit, Write, MultiEdit | Denies an added comment; asks before one is deleted |
| `git_guard.py` | Bash | Allowlists read-only git, denies in-place rewrites |
| `verify_gate.py` | Stop | Blocks the turn from ending while the repo's checks fail |
| `context_injector.py` | SessionStart | Injects the `[WIP]` story, backlog tally, version, verify target |

**The failure policy is inverted on purpose.** The two guards fail **closed** — an unparseable payload denies, because a missed comment reaches disk. `verify_gate.py` and `context_injector.py` fail **open** — any internal error exits 0, because neither a broken verifier nor a broken injector may be able to brick a session.

`context_injector.py` reads disk only. **It never probes the bridge** — a session must not wait on a local model to start.

It is opt-in per repo and silent otherwise. A repo declares its check as `.claude/verify.sh`, or a `verify` target in `Makefile` / `Taskfile` / `justfile`; with none of those present the hook does nothing. It also skips a clean working tree, so a question-only turn never pays for a test run. Claude Code stops honouring a `Stop` hook after 8 consecutive blocks, so a permanently red suite cannot trap the session.

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

**Activation is path-based, not description-based.** `paths:` globs in a skill's frontmatter make the runtime load it when a matching file is touched. `skillOverrides` in `home/settings.json` then collapses that skill to `name-only`, so its description costs nothing in the always-on listing. The pair is how a skill hot-swaps in: **`paths` decides when, `skillOverrides` decides cost.** A skill with neither pays its full description on every turn forever — reserve that for triggers no glob can express. `validate.sh` fails the build on an unknown key.

Workflow skills: `/lifecycle` · `/decompose` · `/implement` · `/consolidate` · `/backlog` · `/generate-repo` · `/audit` · `/readme` · `/risk-assessment` · `/complete`

**A forked skill is the only way to reclaim context.** `context: fork` + `agent: <name>` + `background: false` runs the skill's body *and* its work inside a subagent and returns only the result — the calling window pays nothing for either. `/decompose`, `/implement`, and `/consolidate` are the lifecycle's three forked phases.

**`argument-hint` combined with `context: fork` silently rejects the file unless `disable-model-invocation: true` is also set.** Verified by probe: the same skill registers with the pair, and vanishes without it. `$ARGUMENTS` does reach a forked skill, and `background: false` does return inline — both confirmed the same way.

**`/lifecycle` is the spine.** Spec → decompose → execute → consolidate → complete, each phase naming what ends it. It exists so the build process is not re-derived every session, and so a delegated phase arrives with a brief that stands alone. `ways/sdlc.md` fills in what the gates mean for code; a second way of working is a second file, not a second machine.

**The phase that reads a lot and returns a little runs in a fork.** There is no way to unload a skill body once it is in the window, so a separate context window is the only way to reclaim one.

**A fork needs an agent, and that agent's output template is the phase's return contract.** `implementer` exists because the read-only agents cannot write; `planner` exists because `engineering` returns a research report and a decompose phase must return a queue. Adding a phase means asking which existing contract fits before adding a fifth agent.

**`docs` owns the two frozen specs; `worklog` owns the two mutable records.** Splitting them means a phase that only records work never pays for the spec templates — `/consolidate` dropped from 3,095 tokens to 1,580. **The PRD and SDD are frozen at approval**, which is why slice status never writes back into SDD §10.

**A skill with a procedure half gets a sibling file.** `docs/authoring.md`, `sdlc/reference.md`, `go/reference.md`, `security/review.md` — the rule stays in `SKILL.md`, the how-to loads only when someone is doing that job.

**A skill directory with no `SKILL.md` is invisible.** `rust/`, `cpp/`, and `hardware/` are parked as `SKILL.md.off` — no listing cost, no loader entry, content preserved for when those domains land. Rename back to activate.

**There is no `requires:` frontmatter key, and the dependency direction matters.** `cdk`'s files are already `.ts`, so `typescript` co-loads for free off its own unmodified glob — no coupling needed either direction. `svelte`/`vue` are different: their files aren't `.ts`, and `typescript`'s `paths` must never be widened to name them — that would make the framework-agnostic root skill declare awareness of frameworks it doesn't need and can't shed. Instead `svelte`/`vue` each carry an explicit instruction telling the agent to invoke `typescript` by name. This is a weaker guarantee (it relies on the agent following the instruction, not a deterministic glob match) but it keeps the dependency declared on the dependent's side, where it belongs. Either way, the framework skill never restates a fact — naming, toolchain, Quick-reference rows — that `typescript` already owns.

## No static references

**A skill records rules. It never records inventory.** Nothing in `home/skills/` may name a live repo, a port assignment, a URL, a version number, an env var, or a file path inside another codebase. Those drift silently: the skill keeps asserting a fact months after it stopped being true, and an agent trusts it over the disk.

The replacement is always the same shape — **state the rule, then name where to read the current value.**

| Instead of | Write |
|---|---|
| `komodo-cart-api runs on 7041` | The naming rule + "read the repo's `AGENTS.md`" |
| `Go 1.26` | "the floor `go.mod` declares" |
| The SDK's package list | "read its package tree at the pinned version" |

**Language-agnostic skills name no tools.** `cicd` and `sdlc` state the contract — what a hook must gate, what a tier must cover. The linter, formatter, test command, SDK package, version floor, and reuse doctrine (SDK first, vetted library second, custom last) belong in `go`, `typescript`, `python`, and the other language skills, which are allowed to be concrete because they are already scoped to one toolchain.

**Domain knowledge belonging to one subagent lives in that agent's file, not a skill.** `home/agents/business.md` carries legal, tax, logistics, and hardware inline — loaded only when that agent spawns, so it costs nothing in the main session.

## Context budget

`AGENTS.md` plus every model-visible skill description is paid on **every turn of every session, forever**. `validate.sh` fails above **2,000 tokens** — the ceiling the runtime itself enforces by truncating descriptions past ~1% of the context window.

- **A skill listed `name-only` costs 1–4 tokens.** With a full description it costs ~30–70. **Every skill here is now name-only or invisible** — run `scripts/validate.sh` for the current base-context total.
- **A new line in `home/AGENTS.md` costs its full length**, always. Put it in a skill unless it must apply unconditionally.
- **Workflow skills cost nothing** — `disable-model-invocation: true` keeps them out of the listing.

## Working on this repo

```bash
bash scripts/test-hooks.sh    # 98 hook regression cases
bash scripts/validate.sh      # symlinks, frontmatter schema, token budget
bash setup.sh --dry-run       # preview the install
bash setup.sh                 # install, then runs both of the above
bash .claude/verify.sh        # what the Stop gate runs: both of the above
```

`.claude/verify.sh` is this repo's own opt-in for `verify_gate.py`. Editing anything here and ending the turn runs it, so the config repo is gated by the same mechanism it ships.

Git hooks are **not** in this repo — `pre-commit` and `pre-push` ship with the language SDK (`komodo-forge-sdk-go`). The `cicd` skill states the contract they must satisfy.

# komodo-agentic-toolkit-coding

Agent configuration for software/hardware engineering, shared across every Komodo project. `claude-code/` mirrors `~/.claude/` one-to-one and is symlinked there.

Four ideas hold it together:

1. **Rules that must never break are enforced by a hook, not by prompt text.** Comments and git are checked before the write, never after.
2. **Base context stays tiny.** ~894 tokens of always-on rules and skill names; every skill body loads only when a path glob matches.
3. **Work state lives on disk, not in the conversation.** Five documents per repo mean a compaction cannot lose the plan.
4. **Nothing is Claude-specific except `settings.json`.** Rules and skills are plain markdown, so a local model behind the bridge reads the same source of truth.

## Setup

```bash
bash setup.sh --dry-run    # preview
bash setup.sh              # link, then run the tests and validate
```

Restart Claude Code afterwards so `settings.json` and the hooks take effect.

## Structure

```
claude-code/          mirrors ~/.claude exactly
├── AGENTS.md         the universal rules — always loaded
├── CLAUDE.md         @AGENTS.md
├── settings.json     permissions, hook registration, skillOverrides
├── agents/           workflow-implementer, workflow-planner, engineering, scout
├── hooks/            comment_guard, git_guard, verify_gate, context_injector, auto_format
└── skills/           50 active, 3 parked, lazily loaded
templates/project/    AGENTS.md / CLAUDE.md / BACKLOG.md / CHANGELOG.md
bridges/komodo-bridge/    local LLM MCP bridge config
scripts/              validate.sh, test-hooks.sh, release.sh, portable git hooks
.github/workflows/    CI — runs test-hooks.sh and validate.sh on push/PR
```

## The workflow loop

`/workflow-loop` is the default working mode for anything bigger than a one-line fix. Five phases: **spec → decompose → execute → consolidate → complete.**

The phases that read a lot and return a little run in a forked subagent, so their reading never lands in the main window. `/workflow-loop open <topic>` skips the machine for design work, where a script produces worse output than judgement.

Each repo carries three local documents, plus the SDD (and, when one exists, the PRD) under `docs/spec/`. `write-readme` owns the entry point; `backlog` and `changelog` own the format of the two mutable records, with `standards-worklog` as the read/write directive shared across both. `sdd` and `prd` own authoring and audit for the spec files — `standards-specs` owns their section maps and read contract.

| File | Holds | Mutable |
|---|---|---|
| `README.md` | Entry point — what it is, how to run it | Yes, refreshed as it drifts |
| `BACKLOG.md` | Open work | Yes |
| `CHANGELOG.md` | What shipped, and the version | Append-only |

## The hooks

Two guards run as `PreToolUse`, so a violation never reaches disk. Three more run at the session's edges or after the write.

| Hook | Fires on | Does | On error |
|---|---|---|---|
| `comment_guard.py` | Edit, Write, MultiEdit | Denies an added comment; asks before deleting one | **Closed** |
| `git_guard.py` | Bash | Allowlists read-only git, denies in-place rewrites | **Closed** |
| `verify_gate.py` | Stop | Blocks the turn while the repo's checks fail | **Open** |
| `context_injector.py` | SessionStart | Injects the current `[WIP]` story and version | **Open** |
| `auto_format.py` | Edit, Write (`PostToolUse`) | Runs `gofmt`/prettier on the written file; no-ops if the formatter isn't on `PATH` | **Open** |

**The failure policy is inverted on purpose.** The guards fail closed because a missed comment reaches disk. The other three fail open because none of them may be able to brick a session.

`comment_guard.py` compares **comment multisets** rather than diff hunks. Editing the line a comment sits on, or reindenting it, is not a change. Deleting it is.

### The two comment exceptions

**An exemption the agent can satisfy on its own is a bypass, not an exception.** A content allowlist fails on that alone — whatever token you exempt, the model prepends it. Both exceptions here are things the agent cannot fabricate.

- **Structure — a use-manual under a shebang.** A contiguous run of comment lines starting immediately after `#!`. It cannot reach a function body, because position is not forgeable.
- **Template — a fixed shape the prose cannot fit.** A banner's label is 40 chars between two hyphen runs; a step marker is one indented line of 80. `rules-commenting` lists all four.

**There is no exemption sigil.** An earlier `+comments` grant was removed; nothing lifts the guard for a turn. Deleting a comment returns `ask`, and the guard fails closed on an unreadable payload.

```bash
bash scripts/test-hooks.sh    # 127 regression cases
```

## Skills

**The loader accepts exactly 19 frontmatter keys** — any other key silently rejects the whole file, so the skill simply does not exist at runtime. `validate.sh` fails the build on an unknown one.

| Kind | Frontmatter | Reaches the model | You type `/name` |
|---|---|---|---|
| Knowledge | `user-invocable: false` | Yes | No |
| Workflow | `disable-model-invocation: true` | No, costs zero context | Yes |
| Both | neither key | Yes | Yes |

**Activation is path-based.** A `paths:` glob makes the runtime load a skill when a matching file is touched; `skillOverrides` in `settings.json` then collapses it to `name-only` so its description costs nothing in the always-on listing. **`paths` decides when, `skillOverrides` decides cost.**

**There is no unload.** Once a body is in the window it stays until `/clear` or a compaction. Deferring the load is the whole lever — which is why a glob that is too broad is the expensive mistake, not a skill that exists.

Workflow skills, all free: `/workflow-decompose` `/workflow-implement` `/workflow-consolidate` `/backlog` `/changelog` `/write-repo` `/git-commit-message` `/audit-readiness` `/write-readme` `/audit-change-risk` `/audit-code-quality` `/audit-bugs` `/audit-security` `/audit-simplify` `/audit-performance` `/workflow-complete`

`/workflow-loop` carries neither key instead — it pays its description every turn so a plain-language request ("build this end to end") can trigger it, not just the typed command. Its forked phases stay slash-only on purpose.

**`context: fork` is the only way to reclaim context.** A skill declaring it runs its body *and* its work inside a subagent, returning only the result. The three workflow-loop phases use it. Pairing `argument-hint` with it requires `disable-model-invocation: true` — omit that and the skill is silently rejected.

## Output formatting

The always-on contract lives in `claude-code/AGENTS.md` § 2 and applies to every turn. The full ADHD standard — learning mode, chunking, emoji protocol, table shape, code-answer order, document typography — lives in the `config-accessibility` skill and loads only when authoring something longer than a screen.

Every subagent carries the same contract as a mandatory output template.

## Budget

```bash
bash scripts/validate.sh
```

Verifies every symlink, validates every skill and agent against the loader's frontmatter schema, and **fails above 2,000 tokens** of base context.

A skill listed by name costs 1–4 tokens. A new line in `claude-code/AGENTS.md` costs its full length on every session, forever — put it in a skill unless it must always apply.

## Git hooks for other repos

**No comment hook ships here.** A comment must never block a commit, a push, a linter, or a release. `comment_guard.py` runs only as a `PreToolUse` hook, before the write reaches disk — by the time git sees a file, the dispute is already settled or was never the agent's to have.

**Lint and test hooks do not live here.** `pre-commit` (format + lint) and `pre-push` (delta unit tests + coverage) ship with the language SDK — `komodo-forge-sdk-go` for Go. The `standards-cicd` skill states the contract they must satisfy; the SDK decides how.

Install per repo:

```bash
git config core.hooksPath .githooks
```

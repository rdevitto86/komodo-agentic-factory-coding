# komodo-agentic-toolkit-coding

Shared agent configuration for software/hardware engineering. `claude-code/` mirrors `~/.claude/` one-to-one and is symlinked there by `setup.sh`. Changing anything under `claude-code/` changes every project's next session.

Design rationale for the decisions below lives in `docs/design-decisions.md`, not here — this file states current rules only.

## Layout

| Path | Becomes | Contents |
|---|---|---|
| `claude-code/AGENTS.md` | `~/.claude/AGENTS.md` | The universal rules, always loaded |
| `claude-code/CLAUDE.md` | `~/.claude/CLAUDE.md` | One line: `@AGENTS.md` |
| `claude-code/settings.json` | `~/.claude/settings.json` | Permissions and hook registration |
| `claude-code/agents/` | `~/.claude/agents/` | `workflow-implementer` writes; `workflow-planner`, `engineering`, `scout` are read-only; `reviewer` edits only `BACKLOG.md`; `commentor` never edits directly, splicing only via `write_comments_validator.py` |
| `claude-code/hooks/` | `~/.claude/hooks/` | Two guards, plus the Stop gate and the session injector |
| `claude-code/skills/` | `~/.claude/skills/` | Domain knowledge, lazily loaded |

Also: `templates/project/` (per-repo `AGENTS.md`/`CLAUDE.md`/`BACKLOG.md`/`CHANGELOG.md`) and `bridges/komodo-bridge/` (local LLM MCP bridge).

## The hooks

| Hook | Registered on | Fires on | Does |
|---|---|---|---|
| `comment_guard.py` | `~/.claude/settings.json` | Edit, Write, MultiEdit, NotebookEdit | Flat-denies any added narrative comment, no exceptions beyond machine directives/shebang-manual; asks before one is deleted |
| `git_guard.py` | `~/.claude/settings.json` | Bash | Allowlists read-only git, denies in-place rewrites |
| `context_injector.py` | `~/.claude/settings.json` | SessionStart | Injects the `[WIP]` story, backlog tally, version, verify target |
| `verify_gate.py` | `claude-code/agents/workflow-implementer.md` frontmatter | Stop (auto-converts to `SubagentStop`) | Blocks the fork from returning while the repo's checks fail |
| `auto_format.py` | `~/.claude/settings.json` | PostToolUse, matcher `Edit\|Write` | Runs the repo's formatter on a touched file after the write lands |
| `comment_removal_log.py` | `~/.claude/settings.json` | PostToolUse, matcher `Edit\|Write\|MultiEdit\|NotebookEdit` | Logs every `comment_guard.py`-approved removal to `.claude/state/removed-comments.jsonl` for a later pass to consume |

**The two `PreToolUse` guards fail closed** — an unparseable payload denies. **`verify_gate.py` and `context_injector.py` fail open** — any internal error exits 0.

**Comment guard passing shapes** — an added comment passes silently in exactly three cases:

| Passing shape | Test |
|---|---|
| Script manual | Line comments directly under a `#!` shebang |
| Machine directive | Prefix match against a fixed list |
| DOC (Go only) | Name-first, one sentence, ≤120 chars, directly above a newly-added exported top-level `func`/`type`/`const`/`var`/`package` in a `.go` file — the one shape deterministic enough to check with no diff/backlog context, gated to `.go` by extension since Swift/JS/TS/Rust share enough keywords (`func`, `var`, `const`) to otherwise collide |

Everything else is a flat deny, no ask — a banner, a `NOTE:`/`FIXME:`/`TODO:` note, a plain WHY/HACK/FIELD sentence, and a step marker all deny exactly like unstructured prose. Detection covers both leading comments and trailing (same-line) ones — `find_comment_start` walks each line quote-aware, so a `//` inside a string literal or a Go raw string never false-positives, and a session agent can no longer smuggle an unauthorized comment past the guard by appending it to the end of a line instead of a line of its own. Block comments and Python docstrings are scanned too. A name-echo (first word matches the identifier below it, or a trailing comment's first word matches the field it trails) denies outright — except a comment that already cleared the DOC check, which is exempt by design since name-first is what DOC requires. Deleting a comment proceeds silently — no ask — and is logged unconditionally by `comment_removal_log.py` (leading and trailing both) to `.claude/state/removed-comments.jsonl`, which `commentor` consults later for anything worth restoring; the log, not a per-edit approval, is the safety net. No move ledger or `+comments` grant — moving a non-DOC comment still needs `write-comments` to re-add it, since re-adding denies unless it fits a shape. No comment rule ever blocks a commit, push, lint, or release — `comment_guard.py` is `PreToolUse` only. The `write-comments` skill is the only path to add anything outside the DOC carve-out; it calls `write_comments_validator.py` to check a proposed comment's shape before it's ever written, against a nine-type taxonomy (`WHY`/`HACK`/`DOC`/`FIELD` plain, `NOTE`/`FIXME`/`TODO` marker-prefixed, `BANNER`/`STEP` structural) documented in `commentor.md` — the same `validate_doc_shape` and quote-aware scanners live in `comment_rules.py` and back both the guard and the validator, so the two enforcement points can't drift apart.

## Skill contract

**The loader accepts exactly these frontmatter keys.** Any other key makes it reject the file silently.

`name` · `description` · `when_to_use` · `model` · `effort` · `allowed-tools` · `disallowed-tools` · `argument-hint` · `disable-model-invocation` · `user-invocable` · `paths` · `context` · `agent` · `background` · `hooks` · `metadata` · `shell` · `license` · `compatibility`

`paths` values must be quoted — a bare glob starts with `*`, which YAML reads as an alias.

**Activation is path-based, not description-based.** `paths:` globs load a skill when a matching file is touched; `skillOverrides` in `claude-code/settings.json` then collapses it to `name-only`, so its description costs nothing in the always-on listing. `paths` decides when, `skillOverrides` decides cost. A skill with neither pays its full description forever.

**Forked review and audit phases:** `assess-bugs`, `assess-security`, `assess-simplify` each run as a `context: fork` skill against the `reviewer` agent; `backlog-audit` runs the same way against `workflow-implementer` (it writes `BACKLOG.md`, the same record `workflow-consolidate` already touches). Each still carries neither `disable-model-invocation` nor `user-invocable: false`, so `workflow-loop`'s phases reach them by name.

## No static references

**A skill records rules. It never records inventory.** Nothing in `claude-code/skills/` may name a live repo, a port assignment, a URL, a version number, an env var, or a file path inside another codebase.

| Instead of | Write |
|---|---|
| `komodo-cart-api runs on 7041` | The naming rule + "read the repo's `AGENTS.md`" |
| `Go 1.26` | "the floor `go.mod` declares" |
| The SDK's package list | "read its package tree at the pinned version" |

## Context budget

`AGENTS.md` plus every model-visible skill description is paid on every turn of every session, forever. `validate.sh` fails above **2,000 tokens**.

- **A skill listed `name-only` costs 1–4 tokens.** With a full description it costs ~30–70.
- **A new line in `claude-code/AGENTS.md` costs its full length**, always. Put it in a skill unless it must apply unconditionally.
- **`disable-model-invocation: true` keeps a workflow skill out of the listing entirely.** Everything else reached only by an explicit name is `name-only` in `skillOverrides`.
- **`validate.sh`'s token total excludes bundled and plugin skills** — their text lives in the Claude Code binary, not this repo. `skillOverrides` is the only lever for a bundled skill, `/plugin` for a plugin one, and `/context`'s Skills row is where the real listing size is read.

## Working on this repo

```bash
bash scripts/test-hooks.sh    # 199 hook regression cases
bash scripts/validate.sh      # symlinks, frontmatter schema, token budget
bash setup.sh --dry-run       # preview the install
bash setup.sh                 # install, then runs both of the above
bash .claude/verify.sh        # what the Stop gate runs: both of the above
```

`.claude/verify.sh` is this repo's own opt-in for `verify_gate.py`. It only runs automatically inside a `workflow-implementer` fork finishing a dirty tree — editing this repo directly in a primary session does not trigger it, so run it by hand before ending a manual editing session.

This repo ships its own `pre-commit`/`pre-push` dispatchers under `scripts/hooks/git/`, installed into a target repo via `install.sh` (sets `core.hooksPath`, nothing is copied). The `standards-cicd` skill states the contract they must satisfy.

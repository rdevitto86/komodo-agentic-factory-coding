# komodo-agentic-toolkit-coding

Shared agent configuration for software/hardware engineering. `claude-code/` mirrors `~/.claude/` one-to-one and is symlinked there by `setup.sh`. Changing anything under `claude-code/` changes every project's next session.

Design rationale for the decisions below lives in `docs/design-decisions.md`, not here — this file states current rules only.

## Layout

| Path | Becomes | Contents |
|---|---|---|
| `claude-code/AGENTS.md` | `~/.claude/AGENTS.md` | The universal rules, always loaded |
| `claude-code/CLAUDE.md` | `~/.claude/CLAUDE.md` | One line: `@AGENTS.md` |
| `claude-code/settings.json` | `~/.claude/settings.json` | Permissions and hook registration |
| `claude-code/agents/` | `~/.claude/agents/` | `workflow-implementer` writes; `workflow-planner`, `engineering`, `scout` are read-only; `reviewer` edits only `BACKLOG.md` |
| `claude-code/hooks/` | `~/.claude/hooks/` | Two guards, plus the Stop gate and the session injector |
| `claude-code/skills/` | `~/.claude/skills/` | Domain knowledge, lazily loaded |

Also: `templates/project/` (per-repo `AGENTS.md`/`CLAUDE.md`/`BACKLOG.md`/`CHANGELOG.md`) and `bridges/komodo-bridge/` (local LLM MCP bridge).

## The hooks

| Hook | Registered on | Fires on | Does |
|---|---|---|---|
| `git_guard.py` | `~/.claude/settings.json` | Bash | Allowlists read-only git, denies in-place rewrites |
| `context_injector.py` | `~/.claude/settings.json` | SessionStart | Injects the `[WIP]` story, backlog tally, version, verify target |
| `verify_gate.py` | `claude-code/agents/workflow-implementer.md` frontmatter | Stop (auto-converts to `SubagentStop`) | Blocks the fork from returning while the repo's checks fail |
| `auto_format.py` | `~/.claude/settings.json` | PostToolUse, matcher `Edit\|Write` | Runs the repo's formatter on a touched file after the write lands |

**`git_guard.py` fails closed** — an unparseable payload denies. **`verify_gate.py` and `context_injector.py` fail open** — any internal error exits 0.

## Comments

**There is no comment hook.** Comments are enforced as a lint, through one CLI, gated by whatever already runs `verify`:

```bash
python3 ~/.claude/hooks/comments.py check [paths]   # findings, exit 1 if any
python3 ~/.claude/hooks/comments.py apply           # splice proposals from stdin
```

`check` defaults to changed lines only (`git diff` against `--base`, default `HEAD`), so it never condemns a repo's existing history; `--all` scans whole files. It emits two finding kinds:

| Kind | Meaning |
|---|---|
| `MISSING` | A declaration that requires a comment and has none |
| `INVALID` | A comment that breaks a mechanical rule |

**`MISSING` rules are Go-only** (`SITE_LANGUAGE_EXTENSIONS`) and deliberately narrow: `RET_BOOL_DISCRIMINANT` (≥2 returns ending in `bool`, unless the name is `is`/`has`/`can`/`should`/`exists`/`must`-prefixed) and `RET_ARITY_3` (≥3 return values). Both are suppressed when the line above is already a comment. Signature parsing walks balanced parens rather than matching a flat regex, so a `func`-typed parameter or a named return tuple parses correctly.

**`INVALID` rules are mechanical only** — `NAME_ECHO`, `OVER_CAP`, `STACKED`, `STEP_MARKER`, `BANNER_OUTSIDE_TEST`, `MALFORMED_MARKER`. Machine directives and shebang manuals are exempt, `find_comment_start` keeps a `//` inside a string literal from false-positiving, and a comment that clears the `DOC` shape is exempt from the echo check since name-first is what `DOC` requires.

**Narrative is no longer machine-detectable.** The old `PreToolUse` guard flat-denied every non-`DOC` comment, which caught narration by construction; a lint cannot distinguish `// increments the counter` from a legitimate `WHY` without judgment. That judgment now lives entirely in the `write-comments` skill, and `check` enforces only what is decidable. No comment rule blocks a commit, push, lint, or release beyond the repo's own `verify` target.

The `write-comments` skill is the sanctioned author path; it calls `comments.py apply`, which validates a proposal against the nine-type taxonomy (`WHY`/`HACK`/`DOC`/`FIELD` plain, `NOTE`/`FIXME`/`TODO` marker-prefixed, `BANNER`/`STEP` structural) documented in `write-comments/reference.md`. `check` and `apply` share `lib/comment_rules.py`, so the two ends cannot drift apart.

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

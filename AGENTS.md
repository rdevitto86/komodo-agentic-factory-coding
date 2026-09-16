# komodo-agentic-toolkit-coding

Shared agent configuration for software/hardware engineering. `claude-code/` mirrors `~/.claude/` one-to-one and is symlinked there by `scripts/install.py`. Changing anything under `claude-code/` changes every project's next session.

Design rationale for the decisions below lives in `docs/design-decisions.md`, not here — this file states current rules only.

## Layout

| Path | Becomes | Contents |
|---|---|---|
| `claude-code/AGENTS.md` | `~/.claude/AGENTS.md` | The universal rules, always loaded |
| `claude-code/CLAUDE.md` | `~/.claude/CLAUDE.md` | One line: `@AGENTS.md` |
| `claude-code/settings.json` | `~/.claude/settings.json` | Permissions and hook registration |
| `claude-code/agents/` | `~/.claude/agents/` | `builder` writes anywhere and `tester` under test paths only; `pm`, `researcher`, `architect`, `scout` are read-only; `reviewer` never writes |
| `claude-code/hooks/` | `~/.claude/hooks/` | A guard, plus the Stop gate and the session injector |
| `claude-code/skills/` | `~/.claude/skills/` | Domain knowledge, lazily loaded |

Also: `templates/project/` (per-repo `AGENTS.md`/`CLAUDE.md`/`BACKLOG.md`/`CHANGELOG.md`) and `bridges/komodo-bridge/` (local LLM MCP bridge).

## The hooks

| Hook | Registered on | Fires on | Does |
|---|---|---|---|
| `git_guard.py` | `~/.claude/settings.json` | Bash | Denies destructive git for every caller, restricts six forks' direct git invocations to their own read-only subcommand set by identity (not a sandbox — an interpreter reached through Bash can still get to git), denies in-place rewrites |
| `context_injector.py` | `~/.claude/settings.json` | SessionStart | Injects the `[WIP]` story, backlog tally, version, verify target |
| `verify_gate.py` | `claude-code/agents/builder.md` frontmatter | Stop (auto-converts to `SubagentStop`) | Blocks the fork from returning while the repo's checks fail |
| `auto_format.py` | `~/.claude/settings.json` | PostToolUse, matcher `Edit\|Write` | Runs the repo's formatter on a touched file after the write lands |
| `comments.py hook` | `~/.claude/settings.json` **and** `claude-code/agents/builder.md` frontmatter | PostToolUse, matcher `Edit\|Write` | Reports comment findings for the touched file; fails open |

**`git_guard.py` fails open** — an unparseable payload or an internal crash exits 0, backstopped by a hardcoded destructive-pattern check that still denies. **`verify_gate.py`, `context_injector.py`, and `comments.py hook` fail open** — any internal error exits 0, except a verify command that outruns `KOMODO_VERIFY_TIMEOUT` (integer seconds, default 300), which is a deliberate block naming the limit, not a silent pass-through.

**Every hook command and both `scripts/hooks/git/` dispatchers shell out to `python3` on `PATH`; the floor is 3.7** (set by `subprocess.run(capture_output=...)`, added in 3.7 — nothing here needs a later syntax feature). `scripts/install.py` checks the interpreter a hook command will resolve on `PATH` meets that floor before it links anything; if it's missing or shadowed, the hook command fails before any Python runs, so `git_guard.py`'s own crash handler — destructive-pattern backstop included — never gets a chance to run.

## Comments

**There is no *blocking* comment hook.** `comments.py hook` (see the hook table above) reports findings as `PostToolUse` feedback, but never blocks a write — enforcement is a lint, through one CLI, gated by whatever already runs `verify`:

```bash
python3 ~/.claude/hooks/comments.py check [paths]   # findings, exit 1 if any
python3 ~/.claude/hooks/comments.py apply           # splice proposals from stdin
```

`check` defaults to changed lines only (`git diff` against `--base`, default `HEAD`), so it never condemns a repo's existing history; `--all` scans whole files. It emits two finding kinds:

| Kind | Meaning |
|---|---|
| `MISSING` | A declaration that requires a comment and has none |
| `INVALID` | A comment that breaks a mechanical rule |

**`FUNC_UNDOCUMENTED` demands a comment on every function declaration**, public and private, in every language the lint resolves a family for. It is suppressed by a line comment above, a block comment's closing line above (JSDoc), a Python docstring on the first body line, and four exemptions: a test path, a one-statement body, a bodyless declaration (an interface method, an abstract signature), and a generated file. Anonymous function literals are not declarations and never fire.

**`RET_BOOL_DISCRIMINANT` and `RET_ARITY_3` stay Go-only** (`SITE_LANGUAGE_EXTENSIONS`) and outrank it: ≥2 returns ending in `bool` unless the name is `is`/`has`/`can`/`should`/`exists`/`must`-prefixed, and ≥3 return values. Signature parsing walks balanced parens rather than matching a flat regex, so a `func`-typed parameter or a named return tuple parses correctly.

**`INVALID` rules are mechanical only** — `NAME_ECHO`, `OVER_CAP`, `OVER_LINES`, `STACKED`, `STEP_MARKER`, `BANNER_OUTSIDE_TEST`, `MALFORMED_MARKER`. Machine directives and shebang manuals are exempt, `find_comment_start` keeps a `//` inside a string literal from false-positiving, and a comment that clears the `DOC` shape is exempt from the echo check since name-first is what `DOC` requires.

**`EXTERNAL_REF` refuses a comment that cites anything outside the code** — a version number (`v1.2`, semver), a `PRD`/`SDD`/`ADR`/`TSK`/`EPIC`/`JIRA`, "per the spec", "as discussed", "this task". `apply` refuses the same text, so the one write path cannot land one either. `DOC`'s exported-only gate is gone: any top-level Go declaration can carry a name-first one-sentence doc, which is what `FUNC_UNDOCUMENTED` expects.

**`OVER_LINES` caps a comment block at two lines above a function declaration and one line above everything else** — a `var`, a `const`, a `type`, or a statement. It governs lines *within* one block; `STACKED` governs two distinct blocks landing within `ADJACENT_WINDOW` lines of each other. A directive inside the run does not count against the cap, the leading file header stays exempt up to `HEADER_MAX_LINES`, and `apply` refuses a proposal that would push the run it lands in over — including onto a comment that was already there.

**Narrative is no longer machine-detectable.** The old `PreToolUse` guard flat-denied every non-`DOC` comment, which caught narration by construction; a lint cannot distinguish `// increments the counter` from a legitimate `WHY` without judgment. That judgment now lives entirely in the `write-comments` skill, and `check` enforces only what is decidable. No comment rule blocks a commit, push, lint, or release beyond the repo's own `verify` target.

`builder` is the author path inside the loop, `write-comments` the manual one; both go through `comments.py apply`, which validates a proposal against the nine-type taxonomy (`WHY`/`HACK`/`DOC`/`FIELD` plain, `NOTE`/`FIXME`/`TODO` marker-prefixed, `BANNER`/`STEP` structural) documented in `write-comments/reference.md`. `check` and `apply` share `lib/comment_rules.py`, so the two ends cannot drift apart.

## Skill contract

**The loader accepts exactly these frontmatter keys.** Any other key makes it reject the file silently.

`name` · `description` · `when_to_use` · `model` · `effort` · `allowed-tools` · `disallowed-tools` · `argument-hint` · `disable-model-invocation` · `user-invocable` · `paths` · `context` · `agent` · `background` · `hooks` · `metadata` · `shell` · `license` · `compatibility`

`paths` values must be quoted — a bare glob starts with `*`, which YAML reads as an alias.

**Activation is path-based, not description-based.** `paths:` globs load a skill when a matching file is touched; `skillOverrides` in `claude-code/settings.json` then collapses it to `name-only`, so its description costs nothing in the always-on listing. `paths` decides when, `skillOverrides` decides cost. A skill with neither pays its full description forever.

**Forked review and audit phases:** `assess-bugs`, `assess-security`, `assess-simplify` each run as a `context: fork` skill against the `reviewer` agent; `backlog-audit` runs the same way against `builder` (it writes `BACKLOG.md`, the same record `workflow-consolidate` already touches). Each still carries neither `disable-model-invocation` nor `user-invocable: false`, so `workflow-loop`'s phases reach them by name.

## No static references

**A skill records rules. It never records inventory.** Nothing in `claude-code/skills/` may name a live repo, a port assignment, a URL, a version number, an env var, or a file path inside another codebase.

| Instead of | Write |
|---|---|
| `komodo-cart-api runs on 7041` | The naming rule + "read the repo's `AGENTS.md`" |
| `Go 1.26` | "the floor `go.mod` declares" |
| The SDK's package list | "read its package tree at the pinned version" |

## Context budget

`AGENTS.md` plus every model-visible skill description is paid on every turn of every session, forever. `validate.py` fails above **2,000 tokens**.

- **A skill listed `name-only` costs 1–4 tokens.** With a full description it costs ~30–70.
- **A new line in `claude-code/AGENTS.md` costs its full length**, always. Put it in a skill unless it must apply unconditionally.
- **`disable-model-invocation: true` keeps a workflow skill out of the listing entirely.** Everything else reached only by an explicit name is `name-only` in `skillOverrides`.
- **`validate.py`'s token total excludes bundled and plugin skills** — their text lives in the Claude Code binary, not this repo. `skillOverrides` is the only lever for a bundled skill, `/plugin` for a plugin one, and `/context`'s Skills row is where the real listing size is read.

## Working on this repo

```bash
python3 scripts/test_hooks.py # 295 hook + comments regression cases
python3 scripts/validate.py   # symlinks, frontmatter schema, token budget
python3 scripts/install.py --dry-run   # preview the install
python3 scripts/install.py             # install, then runs both of the above
python3 claude-code/hooks/comments.py check   # comment lint
python3 scripts/test_install.py   # installer regression suite
python3 scripts/verify.py     # what the Stop gate runs: all four of the above
make verify                   # thin wrapper around scripts/verify.py
```

`scripts/verify.py` is this repo's own opt-in for `verify_gate.py`, which resolves a repo's gate in order: `.claude/verify.py`, then `scripts/verify.py`, then `.claude/verify.sh`, then `make verify`, then `task verify`, then `just verify`. The two Python entry points lead because they are invoked as `<interpreter> <path>` — no executable bit, no `make` binary, so they work on every platform. `.claude/` is gitignored here, so `scripts/verify.py` is what ships and `make verify` is a one-line wrapper around it. It only runs automatically inside a `builder` fork finishing a dirty tree — editing this repo directly in a primary session does not trigger it, so run it by hand before ending a manual editing session.

This repo ships its own `pre-commit`/`pre-push` dispatchers under `scripts/hooks/git/`, installed into a target repo via `install.sh` (sets `core.hooksPath`, nothing is copied). The `standards-cicd` skill states the contract they must satisfy.

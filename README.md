# komodo-agentic-config

Agent configuration shared across every Komodo project. `home/` mirrors `~/.claude/` one-to-one and is symlinked there.

Three ideas hold it together:

1. **Rules that must never break are enforced by a hook, not by prompt text.** Comments, git, and edit scope are checked before the write, never after.
2. **Base context stays tiny.** ~70 lines of always-on rules; everything else is a skill that loads only when needed.
3. **Nothing is Claude-specific except `settings.json`.** Rules and skills are plain markdown, so a local model behind the bridge reads the same source of truth.

## Setup

```bash
bash setup.sh --dry-run    # preview
bash setup.sh              # link, then run the tests and doctor
```

Restart Claude Code afterwards so `settings.json` and the hooks take effect.

## Structure

```
home/                 mirrors ~/.claude exactly
├── AGENTS.md         the universal rules — always loaded
├── CLAUDE.md         @AGENTS.md
├── settings.json     permissions + hook registration
├── agents/           engineering, business — read-only research only
├── hooks/            comment_guard.py, git_guard.py, scope_guard.py
└── skills/           24 skills, lazily loaded
templates/project/    AGENTS.md / CLAUDE.md / TODO.md for a new repo
platforms/komodo-bridge/   local LLM MCP bridge config
scripts/              doctor.sh, test-hooks.sh, portable git hooks
```

## The guards

All three run as `PreToolUse`, so a violation never reaches disk.

| Guard | Denies | Asks |
|---|---|---|
| `comment_guard.py` | Any newly added comment | Before deleting one it did not add |
| `git_guard.py` | State-changing git, in-place rewrites | — |
| `scope_guard.py` | — | Before a second file in one turn |

`comment_guard.py` compares **comment multisets** rather than diff hunks. Editing the line a comment sits on, or reindenting it, is not a change. Deleting it is.

### The two comment exceptions

**An exemption the agent can satisfy on its own is a bypass, not an exception.** A content allowlist fails on that alone — whatever token you exempt, the model prepends it. Both exceptions here are things the agent cannot fabricate.

- **Provenance — you send `+comments`.** A `UserPromptSubmit` hook writes a session-scoped grant that lifts the block for that turn; the next prompt without the sigil clears it. Only your keystrokes set it, and a `// +comments` written into a file grants nothing.
- **Structure — a use-manual under a shebang.** A contiguous run of comment lines starting immediately after `#!`, ending at the first blank or code line. It cannot reach a function body, because position is not forgeable.

Deletions still `ask` under a grant, and the guard still fails closed.

```bash
bash scripts/test-hooks.sh    # 49 regression cases
```

## Skills

**The loader accepts exactly eight frontmatter keys** — `name`, `description`, `model`, `allowed-tools`, `disallowed-tools`, `argument-hint`, `disable-model-invocation`, `user-invocable`. Any other key silently rejects the whole file.

| Kind | Frontmatter | Loads when |
|---|---|---|
| Knowledge | `user-invocable: false` | Its description matches the task |
| Workflow | `disable-model-invocation: true` | You type `/name` |
| Both | neither key | Either route |

**There is no path-glob auto-load.** Every knowledge skill's description therefore states *when to load it* — "Load before reading or writing any `.go` file" — because that sentence is the entire trigger mechanism.

Workflow skills: `/plan` `/generate-repo` `/audit` `/wrap-up` `/accessibility`

## Output formatting

The always-on contract lives in `home/AGENTS.md` § 3 — nine rules, ~15 lines, applied to every turn. The full ADHD standard — chunking, emoji protocol, table shape, code-answer order, document typography, and the research behind each — lives in the `accessibility` skill and loads only when authoring something longer than a screen.

Both subagents (`engineering`, `business`) carry the same contract as a mandatory output template.

## Budget

```bash
bash scripts/doctor.sh
```

Verifies every symlink, validates every skill and agent against the loader's frontmatter schema, and **fails above 2,000 tokens** of base context.

A new skill costs ~30 tokens of listing. A new line in `home/AGENTS.md` costs its full length on every session, forever — put it in a skill unless it must always apply.

## Git hooks for other repos

`scripts/hooks/git/pre-commit-comments` runs the same `comment_guard.py` against staged files, so any tool in any editor hits the same rule. That one belongs here — it enforces an agent rule, not a toolchain.

**Lint and test hooks do not live here.** `pre-commit` (format + lint) and `pre-push` (delta unit tests + coverage) ship with the language SDK — `komodo-forge-sdk-go` for Go. The `ci-cd` skill states the contract they must satisfy; the SDK decides how.

Install per repo:

```bash
git config core.hooksPath .githooks
```

# komodo-agentic-config

Shared agent configuration. `home/` mirrors `~/.claude/` one-to-one and is symlinked there by `setup.sh`. Changing anything under `home/` changes every project's next session.

## Layout

| Path | Becomes | Contents |
|---|---|---|
| `home/AGENTS.md` | `~/.claude/AGENTS.md` | The universal rules, always loaded |
| `home/CLAUDE.md` | `~/.claude/CLAUDE.md` | One line: `@AGENTS.md` |
| `home/settings.json` | `~/.claude/settings.json` | Permissions and hook registration |
| `home/agents/` | `~/.claude/agents/` | `engineering`, `business` — read-only research |
| `home/hooks/` | `~/.claude/hooks/` | The three guards |
| `home/skills/` | `~/.claude/skills/` | Domain knowledge, lazily loaded |

Also: `templates/project/` (per-repo `AGENTS.md`/`CLAUDE.md`/`TODO.md`) and `platforms/komodo-bridge/` (local LLM MCP bridge).

## The guards

All three are `PreToolUse` — they run **before** the write, so nothing lands on disk and no corrective edit is ever needed.

| Hook | Fires on | Does |
|---|---|---|
| `comment_guard.py` | Edit, Write, MultiEdit | Denies an added comment; asks before one is deleted |
| `git_guard.py` | Bash | Allowlists read-only git, denies in-place rewrites |
| `scope_guard.py` | Edit/Write, UserPromptSubmit | One file per turn; asks before the second |

`comment_guard.py` compares comment multisets, so adjacency and reindentation are irrelevant. It fails closed — an unparseable payload denies rather than silently passing.

It carries exactly two exceptions, both unforgeable by the agent:

| Exception | Granted by | Scope |
|---|---|---|
| `+comments` in your message | `UserPromptSubmit` | That turn only |
| Use-manual under a shebang | Position in the file | The header block |

A content allowlist would be a third, and would not work — the agent writes the content, so it can always emit the exempt token. Never add one.

## Skill contract

**The loader accepts exactly these frontmatter keys.** Any other key makes it reject the file silently — the skill simply does not exist at runtime.

`name` · `description` · `model` · `allowed-tools` · `disallowed-tools` · `argument-hint` · `disable-model-invocation` · `user-invocable`

| Kind | Frontmatter | Reaches the model | User types `/name` |
|---|---|---|---|
| Knowledge | `user-invocable: false` | Yes, via description | No |
| Workflow | `disable-model-invocation: true` | No, costs zero context | Yes |
| Both | neither key | Yes | Yes |

**There is no path-based auto-load.** A knowledge skill fires only because its `description` names the trigger — so every description states *when to load it*, not just what it contains. `doctor.sh` fails the build on an unknown key.

Workflow skills: `/plan` · `/generate-repo` · `/audit` · `/wrap-up` · `/accessibility`

## Context budget

`AGENTS.md` plus every model-visible skill description is paid on **every turn of every session, forever**. `doctor.sh` fails above **2,000 tokens** — the ceiling the runtime itself enforces by truncating descriptions past ~1% of the context window.

- **A new skill costs ~30 tokens** of listing.
- **A new line in `home/AGENTS.md` costs its full length**, always. Put it in a skill unless it must apply unconditionally.
- **Workflow skills cost nothing** — `disable-model-invocation: true` keeps them out of the listing.

## Working on this repo

```bash
bash scripts/test-hooks.sh    # 49 guard regression cases
bash scripts/doctor.sh        # symlinks, frontmatter schema, token budget
bash setup.sh --dry-run       # preview the install
bash setup.sh                 # install, then runs both of the above
```

Git hooks are **not** in this repo — `pre-commit` and `pre-push` ship with the language SDK (`komodo-forge-sdk-go`). The `ci-cd` skill states the contract they must satisfy.

# komodo-agentic-config

Shared agent configuration. `home/` mirrors `~/.claude/` one-to-one and is symlinked there by `setup.sh`. Changing anything under `home/` changes every project's next session.

## Layout

| Path | Becomes | Contents |
|---|---|---|
| `home/AGENTS.md` | `~/.claude/AGENTS.md` | The universal rules, always loaded |
| `home/CLAUDE.md` | `~/.claude/CLAUDE.md` | One line: `@AGENTS.md` |
| `home/settings.json` | `~/.claude/settings.json` | Permissions and hook registration |
| `home/agents/` | `~/.claude/agents/` | `engineering`, `business` — read-only research |
| `home/hooks/` | `~/.claude/hooks/` | The two guards |
| `home/skills/` | `~/.claude/skills/` | Domain knowledge, lazily loaded |

Also: `templates/project/` (per-repo `AGENTS.md`/`CLAUDE.md`/`TODO.md`) and `platforms/komodo-bridge/` (local LLM MCP bridge).

## The guards

Both are `PreToolUse` — they run **before** the write, so nothing lands on disk and no corrective edit is ever needed.

| Hook | Fires on | Does |
|---|---|---|
| `comment_guard.py` | Edit, Write, MultiEdit | Denies an added comment; asks before one is deleted |
| `git_guard.py` | Bash | Allowlists read-only git, denies in-place rewrites |

`comment_guard.py` compares comment multisets, so adjacency and reindentation are irrelevant. It fails closed — an unparseable payload denies rather than silently passing.

It carries exactly three exceptions:

| Exception | Granted by | Scope |
|---|---|---|
| `+comments` in your message | `UserPromptSubmit` | That turn only |
| Use-manual under a shebang | Position in the file | The header block |
| Banner and test description | Path plus position | Test paths only |

An open content allowlist would be a fourth, and would not work — the agent writes the content, so it can always emit the exempt token. Never add one.

The third exception survives that objection by being structural, not semantic. The description slot matches on **position** — at most 2 line comments, under 200 characters, sitting directly above a test declaration, inside a path the guard recognises as a test path. The `Helpers` banner is the guard's one content match, bounded to a single fixed label and only inside a test path. Neither slot lets arbitrary prose through anywhere else in the file.

## Skill contract

**The loader accepts exactly these frontmatter keys.** Any other key makes it reject the file silently — the skill simply does not exist at runtime.

`name` · `description` · `model` · `allowed-tools` · `disallowed-tools` · `argument-hint` · `disable-model-invocation` · `user-invocable`

| Kind | Frontmatter | Reaches the model | User types `/name` |
|---|---|---|---|
| Knowledge | `user-invocable: false` | Yes, via description | No |
| Workflow | `disable-model-invocation: true` | No, costs zero context | Yes |
| Both | neither key | Yes | Yes |

**There is no path-based auto-load.** A knowledge skill fires only because its `description` names the trigger — so every description states *when to load it*, not just what it contains. `doctor.sh` fails the build on an unknown key.

Workflow skills: `/generate-repo` · `/audit` · `/wrap-up` · `/accessibility`

**`backlog` is the one dual-kind skill** — model-visible so `TODO.md` format applies on any edit, and `/backlog` for a planning run. That costs a listing slot; the alternative was an agent editing `TODO.md` with no format contract loaded.

**A skill directory with no `SKILL.md` is invisible.** `rust/`, `cpp/`, and `hardware/` are parked as `SKILL.md.off` — no listing cost, no loader entry, content preserved for when those domains land. Rename back to activate.

## No static references

**A skill records rules. It never records inventory.** Nothing in `home/skills/` may name a live repo, a port assignment, a URL, a version number, an env var, or a file path inside another codebase. Those drift silently: the skill keeps asserting a fact months after it stopped being true, and an agent trusts it over the disk.

The replacement is always the same shape — **state the rule, then name where to read the current value.**

| Instead of | Write |
|---|---|
| `komodo-cart-api runs on 7041` | The naming rule + "read the repo's `AGENTS.md`" |
| `Go 1.26` | "the floor `go.mod` declares" |
| The SDK's package list | "read its package tree at the pinned version" |

**Language-agnostic skills name no tools.** `cicd`, `sdlc`, `coding-principles`, and `tech-stack` state the contract — what a hook must gate, what a tier must cover. The linter, formatter, test command, SDK package, and version floor belong in `go`, `typescript`, `python`, and the other language skills, which are allowed to be concrete because they are already scoped to one toolchain.

**Domain knowledge belonging to one subagent lives in that agent's file, not a skill.** `home/agents/business.md` carries legal, tax, logistics, and hardware inline — loaded only when that agent spawns, so it costs nothing in the main session.

## Context budget

`AGENTS.md` plus every model-visible skill description is paid on **every turn of every session, forever**. `doctor.sh` fails above **2,000 tokens** — the ceiling the runtime itself enforces by truncating descriptions past ~1% of the context window.

- **A new skill costs ~30 tokens** of listing.
- **A new line in `home/AGENTS.md` costs its full length**, always. Put it in a skill unless it must apply unconditionally.
- **Workflow skills cost nothing** — `disable-model-invocation: true` keeps them out of the listing.

## Working on this repo

```bash
bash scripts/test-hooks.sh    # 60 guard regression cases
bash scripts/doctor.sh        # symlinks, frontmatter schema, token budget
bash setup.sh --dry-run       # preview the install
bash setup.sh                 # install, then runs both of the above
```

Git hooks are **not** in this repo — `pre-commit` and `pre-push` ship with the language SDK (`komodo-forge-sdk-go`). The `cicd` skill states the contract they must satisfy.

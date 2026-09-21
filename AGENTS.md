# komodo-agentic-toolkit-coding

A platform-agnostic agent library and harness. `komodo/` holds the rules, roles, standards, and the orchestrator that runs task groups through model workers. Adapters render that one source into a tool's own config; Claude Code is the first. Changing `komodo/` changes every repo that runs the harness and every session after the next `komodo install`.

## Layout

| Path | What |
|---|---|
| `komodo/rules/` | Global documents: `AGENTS.md` (universal rules), `backlog.md` (task grammar), `cli.md` (commands) |
| `komodo/roles/` | One file per role: frontmatter (tier, access, session) and the body every renderer uses |
| `komodo/standards/` | Rule files per language and domain, injected by extension |
| `komodo/briefs/` | Worker prompt templates, one `<role>.prompt.md` per worker role |
| `komodo/adapters/claude/` | Renders `~/.claude` from rules, roles, and standards; owns its hooks and settings policy |
| `komodo/hooks/` | Every hook: the sh stubs Git dispatches, the Python fallbacks, the Go source in `src/`, the prebuilt binaries in `bin/` |
| `komodo/*.py` | The orchestrator. `python3 -m komodo` is the only entry point |
| `tests/` | `unittest` suites |
| `scripts/verify.py` | This repo's gate: tests, `validate.py`, comment lint, doctor |
| `templates/project/` | Starters for a new repo: `AGENTS.md`, `BACKLOG.md`, `CHANGELOG.md`, `komodo.json`, `docs/spec/` |

## Rules that hold here

- **One source, rendered.** A rule, a role, or a standard is written once under `komodo/`. Nothing platform-specific is hand-maintained; an adapter renders it. No `claude-code/` directory may exist.
- **Roles declare tiers, profiles map tiers to models.** A role says `tier: standard`; a profile says what `standard` means, defaulting in `komodo/config.py` and overridable in `komodo.json`. A model name never appears in a role file.
- **Limits come from the account, not from a constant.** `komodo/account.py` reads `claude auth status` once per run; the plan sets the turn cap, the model ceiling and the timeout, and a subscription drops the dollar cap the account never charges. Undetected falls back to the conservative path, never to a failed run.
- **Guarantees live in code.** `komodo/gitops.py` is the only git writer and refuses protected refs, force, amend, and trailers. Workers run with `worker_env()`, which strips every push credential.
- **Nothing in `komodo/` imports outside the standard library.** Floor is Python 3.9.
- **Standards are files, never inventory.** A standard names no live repo, port, URL, version, or path inside another codebase.
- **Always-on context stays under 1500 tokens** in the rendered adapter. `scripts/validate.py` renders and measures it.
- **A reference must resolve.** `komodo doctor` fails on a backticked path or `/skill` name that does not exist, and runs inside verify.

## Commands

```bash
python3 scripts/verify.py                 # the gate: tests, validate, comments, doctor
python3 -m unittest discover -s tests -q  # tests alone
python3 -m komodo run --dry-run           # plan the next group here without spawning
python3 -m komodo install                 # render the claude adapter into ~/.claude
python3 -m komodo hooks install .         # point this repo's git hooks at komodo/hooks
python3 -m komodo doctor                  # fragments: references, policy, leftovers
```

Editing anything under `komodo/rules`, `roles`, `standards`, or `adapters` needs a `komodo install` to reach a session; the install is a copy.

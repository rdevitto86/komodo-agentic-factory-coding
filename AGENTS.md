# komodo-agentic-toolkit-coding

The Komodo harness: a stdlib-only Python orchestrator that runs task groups through model workers, plus a thin Claude Code adapter. Changing `komodo/` changes every repo that runs the harness; changing `claude-code/` changes every session after the next `komodo install`.

## Layout

| Path | What |
|---|---|
| `komodo/` | The orchestrator package. `python3 -m komodo` is the only entry point |
| `komodo/briefs/` | Role prompt templates: system and prompt per role, `{{slot}}` placeholders |
| `komodo/standards/` | Rule files injected per touched language; also copied to `~/.claude/standards/` |
| `komodo/hooks/` | Git `pre-commit` and `pre-push` in Python, with sh stubs Git dispatches |
| `claude-code/` | Interactive adapter: `AGENTS.md`, agents, three procedure skills, thin `standards-*` skills, two hooks, `settings.policy.json` |
| `tests/` | `unittest` suites; `python3 -m unittest discover -s tests` |
| `scripts/verify.py` | This repo's gate: tests, `validate.py`, comment lint, doctor |
| `templates/project/` | Per-repo `AGENTS.md`, `BACKLOG.md`, `CHANGELOG.md`, `README.md` starters |

## Rules that hold here

- **Guarantees live in code.** `komodo/gitops.py` is the only git writer and refuses protected refs, force, amend, and trailers. Workers run with `worker_env()`, which strips every push credential. `claude-code/hooks/guard.py` is advisory and stays under 200 lines.
- **Nothing in `komodo/` imports outside the standard library.** Floor is Python 3.9.
- **Standards are files, never inventory.** A standard names no live repo, port, URL, version, or path inside another codebase.
- **`claude-code/settings.json` is never tracked.** Policy is `settings.policy.json`; the installer merges it into the personal file.
- **A skill is thin.** `komodo`, `backlog`, and `review` carry procedure; every `standards-*` skill is a pointer to one file under `~/.claude/standards/`.
- **Always-on context stays under 1500 tokens.** `scripts/validate.py` fails above it. Every `standards-*` skill is `name-only` in `settings.policy.json`.
- **A reference must resolve.** `komodo doctor` fails on a backticked path or `/skill` name that does not exist, and runs inside verify.

## Commands

```bash
python3 scripts/verify.py                 # the gate: tests, validate, comments, doctor
python3 -m unittest discover -s tests -q  # tests alone
python3 -m komodo run --dry-run           # plan the next group here without spawning
python3 -m komodo install                 # copy claude-code/ into ~/.claude
python3 -m komodo hooks install .         # point this repo's git hooks at komodo/hooks
python3 -m komodo doctor                  # fragments: references, policy, leftovers
```

Editing `claude-code/` or `komodo/standards/` needs a `komodo install` to reach `~/.claude`; the install is a copy.

# Installing on Windows

The harness needs Python 3.9 or newer and Git for Windows. No Git Bash session, no symlinks, no `make`, no Developer Mode.

## 1. Find your Python

Open PowerShell or Command Prompt and try, one at a time:

```
python3 --version
python --version
py -3 --version
```

Use whichever prints a 3.9 or newer version. Substitute it for `python3` in every command below. If none works, install Python from python.org and tick "Add to PATH".

## 2. Install the Claude Code adapter

From the toolkit clone:

```
python3 -m komodo install
```

This copies `claude-code\` into `%USERPROFILE%\.claude` and writes `settings.json` with hook commands pointing at the interpreter you just used. It is a copy: re-run the command after pulling a new version. Restart Claude Code afterwards.

## 3. Install the git hooks in your repos

```
python3 -m komodo hooks install C:\path\to\repo
```

Git for Windows runs hooks through its own bundled shell, so the two-line `sh` stubs under `komodo\hooks\` work without Git Bash open. Each stub finds `python3`, `python`, or `py -3` and hands off to the Python hook beside it.

## 4. Verify

```
python3 -m komodo doctor
python3 -m komodo run --dry-run
```

`doctor` reports nothing when the install is clean. `--dry-run` prints the next task group's waves and briefs without spending anything.

## Known limits

- The `claude` CLI must be on PATH for a real run. `--dry-run` works without it.
- Paths in `BACKLOG.md` use forward slashes; the parser normalizes them.
- If PowerShell blocks a `.py` file association, invoke every command with the interpreter name explicitly, as shown above.

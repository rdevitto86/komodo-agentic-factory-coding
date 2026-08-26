# Backlog
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · In progress: `[WIP]`

Open work only. Format and rules live in the `write-backlog` skill — load it before editing this file.

## Now — V1

### 1.1 Cross-Cutting
- 1.1.1 | L | git_guard: `strip_leading_flags` fallback heuristic can misfire when an unrecognized flag's value exactly equals a bare monitored-command name with no extension (e.g. `time --output sh actualtool arg`) — contrived/low-likelihood edge case, `claude-code/hooks/git_guard.py` · S → decision recorded or fixed
- 1.1.2 | M | Git hooks: `scripts/hooks/git/` still has no commit-time check for this repo's own `.py` hooks — `pre-push-verify` now covers push (runs `.claude/verify.sh` when present), but a staged syntax error in `git_guard.py`/`comment_guard.py` still passes `git commit` uncaught · M → a staged `.py` syntax error fails `git commit`
- 1.1.3 | L | AGENTS.md states "Git hooks are **not** in this repo — `pre-commit` and `pre-push` ship with the language SDK", contradicted by `scripts/hooks/git/` holding both dispatchers and `install.sh` · S → statement matches disk, or the hooks move to the SDK
- 1.1.4 | L | validate.sh: the "links" check reports "missing (run setup.sh)" for a bare checkout with no `~/.claude` symlinks — confirmed this does fail the overall exit code (`problems` increments, non-zero `problems` triggers `exit 1`), not just warn; still open only pending a decision on whether that's the desired behavior, `scripts/validate.sh` · S → decision recorded
- 1.1.5 | L | README.md's "Workflow skills, all free" bulleted list is missing many existing skills (`write-runbook`, `git-create-pr`, `git-create-issue`, `audit-vulnerabilities`, `audit-dependencies`, `audit-prd`, the four `audit-*` doc skills, `standards-worklog`, `standards-specs`, etc.) — predates this band, not fixed in passing · S → list matches `claude-code/skills/` and `bash scripts/validate.sh` passes
- 1.1.6 | L | No `standards-<language>` skill documents an outdated-dependency/deprecation/EOL tool (only CVE scanners: `govulncheck`, `npm audit`, `pip-audit`) — `audit-dependencies` currently falls back to bare `go list -u -m all`/`npm outdated`/`pip list --outdated` with no documented convention to point to · M → each relevant `standards-<language>` skill names its outdated-dependency tool and `bash scripts/validate.sh` passes
- 1.1.7 | M | Bridge: set `options.num_ctx` in `generateRequest` so large summarizer payloads stop truncating silently
  `[BLOCKED]` no file to edit — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy, not this repo; `bridges/komodo-bridge/` here holds only prompt files and docs. Recheck: reopens once bridge source is vendored into or reachable from this repo. · S → bridge request carries the option
## Next — V2

### 2.1 Cross-Cutting
- 2.1.1 | M | Git identity: agent-run commits and PRs are authored as the user's own GitHub account (`gh auth`'s session token + local `git config user.name`), not a distinct identity — create a machine-user account or GitHub App and wire its token into `gh`/`git` for agent-run operations · M → PR and commit authorship shows the bot identity, not the user's

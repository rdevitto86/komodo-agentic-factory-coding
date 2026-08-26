# Backlog
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · In progress: `[WIP]`

Open work only. Format and rules live in the `generate-backlog` skill — load it before editing this file.

## Now — V1

### Cross-Cutting
- [L] README.md's "Workflow skills, all free" bulleted list is missing many existing skills (`generate-adr`, `generate-runbook`, the four `audit-*` doc skills, `standards-worklog`, etc.) — predates this band, not fixed in passing · S → list matches `claude-code/skills/` and `bash scripts/validate.sh` passes
- [L] No `standards-<language>` skill documents an outdated-dependency/deprecation/EOL tool (only CVE scanners: `govulncheck`, `npm audit`, `pip-audit`) — `audit-dependencies` currently falls back to bare `go list -u -m all`/`npm outdated`/`pip list --outdated` with no documented convention to point to · M → each relevant `standards-<language>` skill names its outdated-dependency tool and `bash scripts/validate.sh` passes
- [L] git_guard: `MUTATING_FLAGS["branch"]` and the `subcommand == "branch"` block in `git_violation` are now unreachable — removing `"branch"` from `READ_ONLY_GIT` means any `git branch ...` returns the generic "changes repository state" message before either is checked · S → `claude-code/hooks/git_guard.py` drops both, `bash scripts/test-hooks.sh` passes
- [M] Bridge: set `options.num_ctx` in `generateRequest` so large summarizer payloads stop truncating silently
  `[BLOCKED]` no file to edit — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy, not this repo; `bridges/komodo-bridge/` here holds only prompt files and docs. Recheck: reopens once bridge source is vendored into or reachable from this repo. · S → bridge request carries the option

## Next — V2

_Nothing planned yet. Run `/generate-backlog` to fill this in._

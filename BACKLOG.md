# Backlog
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · In progress: `[WIP]`

Open work only. Format and rules live in the `generate-backlog` skill — load it before editing this file.

## Now — V1

### Cross-Cutting
- [M] Performance: run the perf suite · S → performance story defined once one is scoped
- [M] Bridge: set `options.num_ctx` in `generateRequest` so large summarizer payloads stop truncating silently · S → bridge request carries the option
- [L] git_guard: `strip_leading_flags` fallback heuristic can misfire when an unrecognized flag's value exactly equals a bare monitored-command name with no extension (e.g. `time --output sh actualtool arg`) — contrived/low-likelihood edge case, `claude-code/hooks/git_guard.py` · S → decision recorded or fixed
- [M] Bridge: migrate `.mcp.json.tmpl` from deprecated SSE to streamable HTTP transport · S → bridge reachable via new transport
- [L] settings.json: drop `git branch`/`git tag`/`git stash` from deny or accept git_guard's read-only-mode logic for them is dead code · S → decision recorded
- [L] validate.sh: hardcoded hook list in the syntax-check section doesn't include `claude-code/hooks/auto_format.py`, so its "hooks" check silently skips it, `scripts/validate.sh` · S → decision recorded or fixed
- [L] Templates: `standards-typescript` and `standards-c` lack the "Repo layout" note `standards-python` just got — `generate-repo`'s Create path for those two languages is still undocumented as unsupported · S → note added
- [L] validate.sh: the "links" check reports "missing (run setup.sh)" for a bare checkout with no `~/.claude` symlinks — unconfirmed whether it fails the overall exit code in that case or only warns, `scripts/validate.sh` · S → confirmed as bug or closed as working as intended

## Next — V2

_Nothing planned yet. Run `/generate-backlog` to fill this in._

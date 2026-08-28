# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)

Format and rules live in the `backlog` skill — load it before editing this file. `[DONE]` tasks stay until a sweep (`/backlog audit`) moves them to `CHANGELOG.md` and removes them — this is not a log to hand-curate.

---

## [EPIC-01] Now, V1
*Goal: keep this toolkit's own hooks, docs, and skills correct and internally consistent.*

### [TG-01.1] Cross-Cutting
* **Target Release:** V1

#### [TSK-01.1.1] git_guard `strip_leading_flags` fallback edge case [P: L] [TODO]
* [ ] **SUB-01.1.1.1** decide whether the misfire (an unrecognized flag's value exactly equals a bare monitored-command name with no extension, e.g. `time --output sh actualtool arg`) in `claude-code/hooks/git_guard.py` is worth fixing given how contrived it is

#### [TSK-01.1.2] Commit-time check for this repo's own hooks [P: M] [TODO]
* [ ] **SUB-01.1.2.1** `scripts/hooks/git/` has no commit-time check for `git_guard.py`/`comment_guard.py` — `pre-push-verify` covers push (runs `.claude/verify.sh` when present), but a staged syntax error still passes `git commit` uncaught

#### [TSK-01.1.3] AGENTS.md git-hooks statement vs. disk [P: L] [TODO]
* [ ] **SUB-01.1.3.1** AGENTS.md states "Git hooks are **not** in this repo — `pre-commit` and `pre-push` ship with the language SDK", contradicted by `scripts/hooks/git/` holding both dispatchers and `install.sh`

#### [TSK-01.1.4] validate.sh "links" check on a bare checkout [P: L] [TODO]
* [ ] **SUB-01.1.4.1** confirmed this fails the overall exit code (`problems` increments, non-zero `problems` triggers `exit 1`) for a bare checkout with no `~/.claude` symlinks, not just a warning — decide if that's the desired behavior, `scripts/validate.sh`

#### [TSK-01.1.5] README's "Workflow skills, all free" list is stale [P: L] [TODO]
* [ ] **SUB-01.1.5.1** add the missing skills (`write-runbook`, `git-pr`, `git-issue`, `audit-vulnerabilities`, `audit-dependencies`, `audit-prd`, the four `audit-*` doc skills, `standards-worklog`, `standards-specs`, etc.)

#### [TSK-01.1.6] No documented outdated-dependency tool per language [P: L] [TODO]
* [ ] **SUB-01.1.6.1** only CVE scanners are documented (`govulncheck`, `npm audit`, `pip-audit`) — `audit-dependencies` currently falls back to bare `go list -u -m all`/`npm outdated`/`pip list --outdated` with no documented convention to point to

#### [TSK-01.1.7] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** external — no file to edit in this repo; the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy, `bridges/komodo-bridge/` here holds only prompt files and docs. Recheck: reopens once bridge source is vendored into or reachable from this repo.

---

## [EPIC-02] Next, V2
*Goal: restructure the skill set for clarity, and move spec/doc authoring local.*

### [TG-02.1] Cross-Cutting
* **Target Release:** V2

#### [TSK-02.1.1] Machine identity for agent-run git/PR operations [P: M] [TODO]
* [ ] **SUB-02.1.1.1** agent-run commits and PRs currently authored as the user's own GitHub account (`gh auth`'s session token + local `git config user.name`) — create a machine-user account or GitHub App and wire its token into `gh`/`git` for agent-run operations

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

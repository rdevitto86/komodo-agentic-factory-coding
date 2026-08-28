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
* **SUB-01.1.1.1** decide whether the misfire (an unrecognized flag's value exactly equals a bare monitored-command name with no extension, e.g. `time --output sh actualtool arg`) in `claude-code/hooks/git_guard.py` is worth fixing given how contrived it is
  * **Done when:** a written decision (fix or won't-fix, with reasoning) is recorded in this task or its commit message

#### [TSK-01.1.2] Commit-time check for this repo's own hooks [P: M] [TODO]
* **SUB-01.1.2.1** `scripts/hooks/git/` has no commit-time check for `git_guard.py`/`comment_guard.py` — `pre-push-verify` covers push (runs `.claude/verify.sh` when present), but a staged syntax error still passes `git commit` uncaught
  * **Done when:** staging a syntax-broken `git_guard.py` or `comment_guard.py` and running `git commit` is blocked by a new `scripts/hooks/git/pre-commit-*` check, verified by `ls scripts/hooks/git/pre-commit-*` showing the new entry and a manual bad-syntax commit attempt failing

#### [TSK-01.1.3] AGENTS.md git-hooks statement vs. disk [P: L] [TODO]
* **SUB-01.1.3.1** AGENTS.md states "Git hooks are **not** in this repo — `pre-commit` and `pre-push` ship with the language SDK", contradicted by `scripts/hooks/git/` holding both dispatchers and `install.sh`
  * **Done when:** `grep -n "Git hooks are" AGENTS.md` no longer contradicts `ls scripts/hooks/git/`

#### [TSK-01.1.4] validate.sh "links" check on a bare checkout [P: L] [TODO]
* **SUB-01.1.4.1** confirmed this fails the overall exit code (`problems` increments, non-zero `problems` triggers `exit 1`) for a bare checkout with no `~/.claude` symlinks, not just a warning — decide if that's the desired behavior, `scripts/validate.sh`
  * **Done when:** a written decision is recorded (keep as a hard failure, or downgrade the `missing`/`dangling` cases to a warning that doesn't increment `problems`), and `scripts/validate.sh` matches it

#### [TSK-01.1.5] README's "Workflow skills, all free" list is stale [P: L] [TODO]
* **SUB-01.1.5.1** add the missing skills (`git-pr-create`, `git-issue-create`, `audit-vulnerabilities`, `audit-dependencies`, `runbook`, `sdd`, `prd`, `adr`, `audit-readiness`, `audit-change-risk`, `audit-testing`, `standards-worklog`, `standards-specs`, etc.)
  * **Done when:** every `user-invocable`, non-`disable-model-invocation` skill under `claude-code/skills/` with no cost implication appears in `README.md`'s workflow-skills list, confirmed by diffing `ls claude-code/skills/` against the list

#### [TSK-01.1.6] No documented outdated-dependency tool per language [P: L] [TODO]
* **SUB-01.1.6.1** only CVE scanners are documented (`govulncheck`, `npm audit`, `pip-audit`) — `audit-dependencies` currently falls back to bare `go list -u -m all`/`npm outdated`/`pip list --outdated` with no documented convention to point to
  * **Done when:** `grep -n "outdated" claude-code/skills/standards-go/SKILL.md claude-code/skills/standards-typescript/SKILL.md claude-code/skills/standards-python/SKILL.md` returns a documented convention for each

#### [TSK-01.1.7] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (2026-08-28):** No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs.
  * **Citation:** `bridges/komodo-bridge/` (prompt files and docs only, no Go source)
  * **Recheck:** bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match
* **SUB-01.1.7.1** fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo
  * **Done when:** a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge`

---

## [EPIC-02] Next, V2
*Goal: restructure the skill set for clarity, and move spec/doc authoring local.*

### [TG-02.1] Cross-Cutting
* **Target Release:** V2

#### [TSK-02.1.1] Machine identity for agent-run git/PR operations [P: M] [TODO]
* **SUB-02.1.1.1** agent-run commits and PRs currently authored as the user's own GitHub account (`gh auth`'s session token + local `git config user.name`) — create a machine-user account or GitHub App and wire its token into `gh`/`git` for agent-run operations
  * **Done when:** `gh auth status` inside an agent run shows the machine identity, not the user's own account, and a test commit/PR is authored under it

#### [TSK-02.1.2] Model `/git-commit-tag` from `changelog` + `workflow-complete` [P: L] [TODO]
* **SUB-02.1.2.1** the git-tag-sync check (list tags, find the commit that introduced the just-released `CHANGELOG.md` heading, hand the user the exact `git tag -a` command) currently lives split across `changelog`'s "Tag sync" section and `workflow-complete`'s P4 post-push step — consolidate into a standalone command skill once that logic grows, so tag-readiness can be checked on demand outside the loop too
  * **Done when:** `ls claude-code/skills/ | grep -q git-commit-tag` exits 0, and `changelog`'s "Tag sync" section plus `workflow-complete`'s P4 step both invoke it by name instead of restating the logic

#### [TSK-02.1.3] Split `backlog` into `backlog-audit` and `backlog-modify` [P: L] [TODO]
* **SUB-02.1.3.1** `backlog`'s three modes split unevenly by intent — Part 3 (`audit`) judges and applies verdicts against existing repo/file state, while Parts 1 (`plan`) and 2 (`normalize`) are both authoring-shaped (originate or reshape `BACKLOG.md`'s content); extract Part 3 into `backlog-audit` (`disable-model-invocation: true`, matching the typed-only human-decision precedent set for `audit-readme` etc.) and fold Parts 1+2 into `backlog-modify`, then update `AGENTS.md`'s naming-bucket table and every skill that invokes `backlog plan`/`backlog audit`/`backlog normalize` by mode string (`workflow-loop` P0/P1, `repo-init`, and any other caller)
  * **Done when:** `ls claude-code/skills/ | grep -E 'backlog-(audit|modify)'` returns both, `claude-code/skills/backlog/` no longer exists, and `grep -rn "backlog plan\|backlog audit\|backlog normalize" claude-code/skills/ AGENTS.md` returns no hits

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

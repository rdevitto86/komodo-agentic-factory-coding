# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)

Format and rules live in the `write-backlog` skill — load it before editing this file. `[DONE]` tasks stay until a sweep (`/audit-backlog`) moves them to `CHANGELOG.md` and removes them — this is not a log to hand-curate.

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

### [TG-02.2] Skill Restructuring
* **Target Release:** V2

#### [TSK-02.2.1] Merge write-backlog + audit-backlog into /backlog [P: M] [TODO]
* [ ] **SUB-02.2.1.1** single skill with create + edit + audit instructions, one template

#### [TSK-02.2.2] Merge write-changelog + audit-changelog into /changelog [P: M] [TODO]
* [ ] **SUB-02.2.2.1** single skill with create + edit + audit instructions, one template

#### [TSK-02.2.3] Rename standards-design-ui + standards-security-ui to standards-ui-* [P: L] [TODO]
* [ ] **SUB-02.2.3.1** e.g. `standards-ui-design`, `standards-ui-security`

#### [TSK-02.2.4] Remove rules-source-control, redistribute its rules [P: M] [TODO]
* [ ] **SUB-02.2.4.1** redistribute into whichever skill already owns that concern (`standards-git`, `git-pr`, `git-issue`, etc.)

#### [TSK-02.2.5] Remove standards-git, redistribute its content [P: M] [TODO]
* [ ] **SUB-02.2.5.1** redistribute into its constituent skills

#### [TSK-02.2.6] Rename standards-security-api to standards-api-security [P: L] [TODO]

#### [TSK-02.2.7] Create standards-api-design skill [P: M] [TODO]
* [ ] **SUB-02.2.7.1** API shape/contract conventions, distinct from `standards-api-security`

#### [TSK-02.2.8] Rename rules-merge-conflicts to git-merge-conflict [P: L] [TODO]

#### [TSK-02.2.9] Rename write-repo to repo-init [P: L] [TODO]

#### [TSK-02.2.10] Split git-pr into create/review/comment [P: M] [TODO]
* [ ] **SUB-02.2.10.1** `git-pr-create`, `git-pr-review`, `git-pr-comment`

#### [TSK-02.2.11] Split git-issue into create/review [P: M] [TODO]
* [ ] **SUB-02.2.11.1** `git-issue-create`, `git-issue-review`

#### [TSK-02.2.12] Parse rules-commenting into each standards-<language> skill [P: M] [TODO]
* [ ] **SUB-02.2.12.1** every language standard carries its own comment-discipline section

#### [TSK-02.2.13] audit-* skills load every relevant standards-<language> [P: M] [TODO]
* [ ] **SUB-02.2.13.1** audit across all relevant languages in one pass, not one fixed language
* [ ] **SUB-02.2.13.2** each `standards-<language>` skill gains an optional security-standards section (language-specific insecure-usage patterns) for the security auditors to pull from

#### [TSK-02.2.14] Rename config-accessibility-output to config-accessibility [P: L] [TODO]

### [TG-02.3] Docs & Specs
* **Target Release:** V2

#### [TSK-02.3.1] Scaffold the local /docs/spec, /docs/adr, /docs/runbook layout + MkDocs Material config as reusable templates [P: H] [TODO]
* [ ] **SUB-02.3.1.1** add `templates/project/docs/spec/`, `templates/project/docs/adr/`, `templates/project/docs/runbook/` each with a starter template file (this config repo itself is excluded — these are templates other repos scaffold from, not local docs for this repo) · Done when: the three directories exist with a non-empty starter file each and `bash scripts/validate.sh` passes
* [ ] **SUB-02.3.1.2** add `templates/project/mkdocs.yml.tmpl` configured for the Material theme, rendering the repo's local `/docs/*` · Done when: the file exists, names `mkdocs-material` as its theme, and `bash scripts/validate.sh` passes
* [ ] **SUB-02.3.1.3** update `write-repo`'s Create path to scaffold `/docs/spec`, `/docs/adr`, `/docs/runbook`, and the MkDocs config into any new project repo (this config repo excluded per `write-repo`'s own scope) · Done when: `claude-code/skills/write-repo/SKILL.md`'s Create section names all four, and `bash scripts/validate.sh` passes

#### [TSK-02.3.2] Update standards-specs for the local /docs/spec layout [P: M] [TODO] (after: "Scaffold the local /docs/spec, /docs/adr, /docs/runbook layout + MkDocs Material config as reusable templates")
* [ ] **SUB-02.3.2.1** replace the current Drive-fetch contract in `claude-code/skills/standards-specs/SKILL.md` with a reference to the local `/docs/spec` layout · Done when: the skill contains no Drive/Google Doc fetch instructions and `bash scripts/validate.sh` passes
* [ ] **SUB-02.3.2.2** document MkDocs Material as the doc-site toolchain `/docs/spec` is paired with · Done when: `standards-specs/SKILL.md` names MkDocs Material and `bash scripts/validate.sh` passes

#### [TSK-02.3.3] Create /sdd skill [P: M] [TODO] (after: "Scaffold the local /docs/spec, /docs/adr, /docs/runbook layout + MkDocs Material config as reusable templates")
* [ ] **SUB-02.3.3.1** add `claude-code/skills/sdd/SKILL.md` merging SDD authoring (create + edit) and `audit-sdd`'s auditing into one skill against the local `/docs/spec` layout, removing `claude-code/skills/audit-sdd/` once its content is folded in · Done when: `claude-code/skills/sdd/SKILL.md` exists, `claude-code/skills/audit-sdd/` no longer exists, and `bash scripts/validate.sh` passes

#### [TSK-02.3.4] Create /prd skill [P: M] [TODO] (after: "Scaffold the local /docs/spec, /docs/adr, /docs/runbook layout + MkDocs Material config as reusable templates")
* [ ] **SUB-02.3.4.1** add `claude-code/skills/prd/SKILL.md` merging PRD authoring (create + edit) and `audit-prd`'s auditing into one skill against the local `/docs/spec` layout, removing `claude-code/skills/audit-prd/` once its content is folded in · Done when: `claude-code/skills/prd/SKILL.md` exists, `claude-code/skills/audit-prd/` no longer exists, and `bash scripts/validate.sh` passes

#### [TSK-02.3.5] Create /adr skill [P: M] [TODO] (after: "Scaffold the local /docs/spec, /docs/adr, /docs/runbook layout + MkDocs Material config as reusable templates")
* [ ] **SUB-02.3.5.1** add `claude-code/skills/adr/SKILL.md` with merged authoring (create + edit) and audit instructions, one template, against the local `/docs/adr` layout — no existing ADR skill to fold in, greenfield · Done when: `claude-code/skills/adr/SKILL.md` exists and `bash scripts/validate.sh` passes

#### [TSK-02.3.6] Create /runbook skill, replacing write-runbook [P: M] [TODO] (after: "Scaffold the local /docs/spec, /docs/adr, /docs/runbook layout + MkDocs Material config as reusable templates")
* [ ] **SUB-02.3.6.1** add `claude-code/skills/runbook/SKILL.md` merging `write-runbook`'s authoring with new audit instructions into one skill against the local `/docs/runbook` layout, removing `claude-code/skills/write-runbook/` and repointing its callers (`workflow-loop`'s P0, `/workflow-consolidate`'s README-refresh step, `/write-repo`'s Scaffold/Refresh path) once folded in · Done when: `claude-code/skills/runbook/SKILL.md` exists, `claude-code/skills/write-runbook/` no longer exists, no remaining reference to `write-runbook` outside `CHANGELOG.md`, and `bash scripts/validate.sh` passes

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

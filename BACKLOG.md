# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)

Format and rules live in the `backlog-modify` skill — load it before editing this file. `[DONE]` tasks stay until a sweep (`/backlog-audit`) moves them to `CHANGELOG.md` and removes them — this is not a log to hand-curate.

---

## [EPIC-01] Now, V1
*Goal: keep this toolkit's own hooks, docs, and skills correct and internally consistent.*

### [TG-01.1] Cross-Cutting
* **Target Release:** V1

#### [TSK-01.1.7] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (2026-08-28):** No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs.
  * **Citation:** `bridges/komodo-bridge/` (prompt files and docs only, no Go source)
  * **Recheck:** bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match
* **SUB-01.1.7.1** fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo
  * **Done when:** a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge`

#### [TSK-01.1.8] Machine identity for agent-run git/PR operations [P: M] [TODO]
* **SUB-01.1.8.1** agent-run commits and PRs currently authored as the user's own GitHub account (`gh auth`'s session token + local `git config user.name`) — create a machine-user account or GitHub App and wire its token into `gh`/`git` for agent-run operations
  * **Done when:** `gh auth status` inside an agent run shows the machine identity, not the user's own account, and a test commit/PR is authored under it

#### [TSK-01.1.9] Speed up scripts/test-hooks.sh subprocess overhead [P: L] [TODO]
* **SUB-01.1.9.1** collapse each test-case helper's 2-3 separate python3 subprocess calls (payload encode, hook invocation, result decode) into a single python3 invocation per case
  * **Done when:** `bash scripts/test-hooks.sh` exits 0 with `172 passed, 0 failed`
* **SUB-01.1.9.2** give each case its own isolated SESSION id and parallelize case execution with a bounded worker pool, moving RESULTS collection from shared append to per-worker temp files concatenated at the end
  * **Done when:** `time bash scripts/test-hooks.sh` exits 0 with `172 passed, 0 failed` and wall time is at most half the pre-change baseline (~11.5s)

#### [TSK-01.1.13] `/backlog-plan` skill — extract `backlog-modify`'s plan mode into its own typed command [P: L] [TODO]
* **SUB-01.1.13.1** resolve the naming conflict against `AGENTS.md`'s "two modes, one file" rule before implementing — either update `AGENTS.md` to document the new exception, or close this task as won't-do
  * **Done when:** `grep -q 'backlog-plan' claude-code/AGENTS.md` exits 0

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

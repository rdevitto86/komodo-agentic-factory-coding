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

#### [TSK-01.1.2] Commit-time check for this repo's own hooks [P: M] [DONE]
* **SUB-01.1.2.1** `scripts/hooks/git/` has no commit-time check for `git_guard.py`/`comment_guard.py` — `pre-push-verify` covers push (runs `.claude/verify.sh` when present), but a staged syntax error still passes `git commit` uncaught
  * **Done when:** staging a syntax-broken `git_guard.py` or `comment_guard.py` and running `git commit` is blocked by a new `scripts/hooks/git/pre-commit-*` check, verified by `ls scripts/hooks/git/pre-commit-*` showing the new entry and a manual bad-syntax commit attempt failing

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

#### [TSK-01.1.9] Model `/git-commit-tag` from `changelog` + `workflow-complete` [P: L] [DONE]
* **SUB-01.1.9.1** the git-tag-sync check (list tags, find the commit that introduced the just-released `CHANGELOG.md` heading, hand the user the exact `git tag -a` command) currently lives split across `changelog`'s "Tag sync" section and `workflow-complete`'s P4 post-push step — extract `workflow-complete`'s P4 tagging logic (not just consolidate it in place) into a standalone `git-commit-tag` command skill, then rewrite `changelog`'s "Tag sync" section and `workflow-complete`'s P4 step to both invoke the new skill by name instead of restating the logic
  * **Done when:** `ls claude-code/skills/ | grep -q git-commit-tag` exits 0, `workflow-complete`'s P4 step no longer contains the tag-sync logic inline (only an invocation of `git-commit-tag`), and `changelog`'s "Tag sync" section invokes it the same way

#### [TSK-01.1.10] Split `backlog` into `backlog-audit` and `backlog-modify` [P: L] [DONE]
* **SUB-01.1.10.1** `backlog`'s three modes split unevenly by intent — Part 3 (`audit`) judges and applies verdicts against existing repo/file state, while Parts 1 (`plan`) and 2 (`normalize`) are both authoring-shaped (originate or reshape `BACKLOG.md`'s content); extract Part 3 into `backlog-audit` (`disable-model-invocation: true`, matching the typed-only human-decision precedent set for `readme-audit` etc.) and fold Parts 1+2 into `backlog-modify`, then update `AGENTS.md`'s naming-bucket table and every skill that invokes `backlog plan`/`backlog audit`/`backlog normalize` by mode string (`workflow-loop` P0/P1, `repo-init`, and any other caller)
  * **Done when:** `ls claude-code/skills/ | grep -E 'backlog-(audit|modify)'` returns both, `claude-code/skills/backlog/` no longer exists, and `grep -rn "backlog plan\|backlog audit\|backlog normalize" claude-code/skills/ AGENTS.md` returns no hits
  * **Decision (2026-08-28):** implemented without `disable-model-invocation: true` on `backlog-audit`, deviating from this story's own suggestion — `workflow-loop`'s P1 invokes `backlog-audit` programmatically every run (confirmed: it's never invoked from another skill's instructions anywhere in this repo, unlike `workflow-decompose`/etc.), and that flag would have severed the call per the repo's own established convention. Documented as a deliberate exception in `AGENTS.md`'s Typed-only section.

#### [TSK-01.1.11] git_guard: same MONITORED_COMMANDS-collision bypass may reach xargs/command/nohup [P: L] [DONE]
* **SUB-01.1.11.1** TSK-01.1.1 fixed the confirmed exploit path for `time` (`--format`/`--output` now recognized), but `PASSTHROUGH_VALUE_FLAGS` for `command` (`()`), `nohup` (`()`), and `xargs` (`("-I", "-n", "-P", "-L", "-s", "-a", "-d", "-E")`, `claude-code/hooks/git_guard.py:126-131`) still lists only short flags — GNU `xargs`'s long forms (`--replace`, `--max-args`, `--max-procs`, `--max-lines`, `--arg-file`, `--delimiter`) aren't recognized as value-taking, so the same `strip_leading_flags` fallback could misidentify a flag's value as the wrapped command when that value collides with a `MONITORED_COMMANDS` name — same shape as the `time` bug, just harder to trigger organically (no plausible accidental cause, only a deliberately constructed value)
  * **Done when:** a written decision is recorded (fix by recognizing `xargs`'s long-form value flags, or won't-fix given the harder-to-trigger, purely-deliberate construction), and `claude-code/hooks/git_guard.py` matches it
  * **Decision (2026-08-28):** fixed, matching the `time` precedent — added `xargs`'s long-form value flags (`--replace`, `--max-args`, `--max-procs`, `--max-lines`, `--max-chars`, `--arg-file`, `--delimiter`) to `PASSTHROUGH_VALUE_FLAGS["xargs"]`, verified against GNU findutils 4.11's own `--help` (`-E` genuinely has no long form, so it stays short-only). Regression case `G89` in `scripts/test-hooks.sh` proves the fix against `xargs --delimiter git git push --force origin main`.

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

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

#### [TSK-01.1.1] Machine identity for agent-run git/PR operations [P: M] [TODO]
* **SUB-01.1.1.1** agent-run commits and PRs currently authored as the user's own GitHub account (`gh auth`'s session token + local `git config user.name`) — create a machine-user account or GitHub App and wire its token into `gh`/`git` for agent-run operations
  * **Done when:** `gh auth status` inside an agent run shows the machine identity, not the user's own account, and a test commit/PR is authored under it

#### [TSK-01.1.2] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (2026-08-28):** No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs.
  * **Citation:** `bridges/komodo-bridge/` (prompt files and docs only, no Go source)
  * **Recheck:** bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match
* **SUB-01.1.2.1** fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo
  * **Done when:** a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge`

#### [TSK-01.1.3] `reviewer` agent's Bash grant can write outside BACKLOG.md unexamined [P: M] [TODO]
* **SUB-01.1.3.2** [Medium] `reviewer`'s `Edit`/`Write`/`MultiEdit`/`NotebookEdit` path to non-`BACKLOG.md` files is now hook-enforced (`comment_guard.py`'s `check_reviewer_scope`, added 2026-09-01) — but its `Bash` grant is a separate, still-open bypass: `claude-code/settings.json`'s allow list permits `echo`, `printf`, and `tee` globally, and `git_guard.py`'s own redirect/`tee`/`cp`/`mv` protections gate only on `is_code_path()`, whose `EXTENSION_FAMILY` map (`comment_guard.py`) has no entry for `.json` or `.md`. `echo '{...}' > claude-code/settings.json` therefore reaches disk unexamined by either guard — `comment_guard.py` never fires (its matcher is `Edit|Write|MultiEdit|NotebookEdit`, not `Bash`), and `git_guard.py`'s redirect check treats `.json`/`.md` as a non-code path and skips it. This isn't new in this diff (the extension map is untouched here), but this band is what first hands an unattended, diff-reading fork the `Bash` tool needed to exercise it — settings.json is the file that stores which hooks even run, so this path can silently strip `comment_guard`/`git_guard`'s own `PreToolUse` registration for the rest of the session.
  * **Done when:** `is_code_path()` (or an equivalent check reachable from `git_guard.py`'s redirect/`tee`/`cp`/`mv` guards) covers `.json` and `.md`, so a shell redirect into `claude-code/settings.json` or `BACKLOG.md` is flagged the same way a redirect into a `.py`/`.go` file already is


### [TG-01.3] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-01 audit, updated post-shipped review-fork/AGENTS.md-trim/bundled-skill-collapse work):** the always-on budget is healthy (`scripts/validate.sh`: 1,152 of 2,000 tokens). Root `AGENTS.md` was cut from 7,157 to 1,416 tokens and `assess-bugs`/`assess-security`/`assess-simplify`/`backlog-audit` now run as `reviewer`/`workflow-implementer` forks instead of the orchestrator window (shipped, `backlog-audit` renumbered the remaining tasks below). Remaining scope: `git-pr-create`'s P4 diff read, `workflow-loop`'s own compaction cap, the dead `config-accessibility-output` reference, and the `git_guard.py` backtick false-positive.

#### [TSK-01.3.1] Stop git-pr-create reading the whole band diff into the orchestrator at P4 [P: M] [TODO]
* **SUB-01.3.1.1** `git-pr-create/SKILL.md:15` reads "every commit ahead of the base (`git log <base>..HEAD`) and the full diff (`git diff <base>..HEAD`)". The skill body is 2,990 tokens and runs in-window from `workflow-complete`; on a 15k-token band that is ~18k tokens for a PR body whose content P3 already summarised per task in commit messages. Change the read to `git log <base>..HEAD` plus `git diff <base>..HEAD --stat`, and open a single file's diff only when its stat line and commit messages leave the change genuinely ambiguous. The `Size it` table already works off `--stat` numbers. Forking this skill was considered and rejected for now: it needs `Bash(gh pr create)`, and no existing agent contract fits a publish step — revisit only if the `--stat` read proves insufficient.
  * **Done when:** `grep -q -- 'git diff <base>..HEAD --stat' claude-code/skills/git-pr-create/SKILL.md && ! grep -q 'the full diff' claude-code/skills/git-pr-create/SKILL.md`

#### [TSK-01.3.2] Bring workflow-loop under the 5,000-token compaction re-attach cap and make the gates survive compaction [P: M] [TODO]
* **SUB-01.3.2.1** `workflow-loop/SKILL.md` is 5,048 tokens (`len(text)//4`, the same estimator `validate.sh` uses). The docs' "Skill content lifecycle" says compaction re-attaches "the first 5,000 tokens of each" invoked skill, so after the first compaction of a long band the orchestrator loses the tail: P4, Guardrails, Delegating outside the phases, The open hatch. Trim the file to at most 3,500 tokens by cutting justification prose while keeping every rule: the "this is what keeps…" / "this is also what removes P1's old cost problem" / "same reasoning as P2.3" sentences explain decisions the file does not need to re-argue at runtime — those explanations belong in `docs/design-decisions.md` if they are worth keeping at all. Every phase's "Ends when" line, every skill name, and every table row stays.
  * **Done when:** `python3 -c "import sys; sys.exit(0 if len(open('claude-code/skills/workflow-loop/SKILL.md').read())//4 <= 3500 else 1)" && bash scripts/validate.sh`
* **SUB-01.3.2.2** `ways/sdlc.md` (895 tokens) is loaded with a `Read`, not a skill invocation, so compaction never re-attaches it and the P2.3/P2.4 gates it defines can silently vanish mid-band. Add one Guardrails bullet to `workflow-loop/SKILL.md`: after any context compaction, re-read the active `ways/` file before the next phase gate. Do not fold `sdlc.md` into `SKILL.md` — the "Adding a second way of working" contract depends on the ways file staying separate.
  * **Done when:** `grep -q -i 'compaction' claude-code/skills/workflow-loop/SKILL.md`
* **SUB-01.3.2.3** add a `scripts/validate.sh` check that fails when any `SKILL.md` exceeds 5,000 estimated tokens, printing the offender and its count, so the compaction cap is enforced the same way the listing budget is. `git-pr-create` (2,990), `standards-api-security` (3,800), and `standards-go` (3,058) pass today; the check is for the next file that grows past it.
  * **Done when:** `grep -q '5000' scripts/validate.sh && bash scripts/validate.sh`

#### [TSK-01.3.3] Fix the dead config-accessibility-output reference and shrink config-accessibility [P: L] [TODO]
* **SUB-01.3.3.1** `~/.claude/CLAUDE.local.md:7` says "Load `config-accessibility-output` before authoring anything longer than a screen"; no skill of that name exists — the skill is `config-accessibility`. The instruction has been dead since the rename, so the skill (2,445 tokens) never loads; the file's own bullet list under "Conversation — ADHD-calibrated" already carries the rules that matter. Note `~/.claude/CLAUDE.local.md.tmpl` is a dangling symlink to `claude-code/CLAUDE.local.md.tmpl`, which does not exist in the repo — so there is no tracked template to fix; correct the live `~/.claude/CLAUDE.local.md` line to name `config-accessibility`, or delete the line if SUB-01.3.3.2 makes the skill redundant.
  * **Done when:** `! grep -q 'config-accessibility-output' ~/.claude/CLAUDE.local.md`
* **SUB-01.3.3.2** `config-accessibility/SKILL.md` restates the same BLUF / no-preamble / short-paragraph rules `CLAUDE.local.md` already delivers every turn. Reduce it to what the local file does not say (the long-document structure rules, if any survive review), at most 800 tokens, or delete the skill directory and the `skillOverrides` entry if nothing survives.
  * **Done when:** `test ! -d claude-code/skills/config-accessibility || python3 -c "import sys; sys.exit(0 if len(open('claude-code/skills/config-accessibility/SKILL.md').read())//4 <= 800 else 1)"`

#### [TSK-01.3.4] git_guard.py denies a grep whose pattern text mentions a git verb inside backticks [P: L] [TODO]
* **SUB-01.3.4.1** observed 2026-09-01 in this repo: a `grep -n -E '<pattern>' <files>` whose single-quoted pattern contained backticks and the text `git log` was denied with reason "git log' changes repository state", and a second command that embedded `git log` inside `'"'"'` quoting was denied the same way; each false positive costs a full wasted turn plus a rewrite. A plain `grep -n "git log" README.md` passes (verified by piping the payload from a file into `python3 claude-code/hooks/git_guard.py`). Cause not verified — the likely spot is the command segmenter treating a backtick inside single quotes as a command-substitution boundary. Reproduce with a payload file (the repro command cannot be typed into a Bash call directly — the guard blocks it), find the segmenter fault, fix it, and add both shapes as allow cases to `scripts/test-hooks.sh` alongside a deny case confirming a real backtick substitution running `git log` in a write context is still caught.
  * **Done when:** `bash scripts/test-hooks.sh`

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._

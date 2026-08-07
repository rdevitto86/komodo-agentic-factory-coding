# Agent Rules

Universal directive for every model and every tool — Claude, GPT, Gemini, Qwen, Kimi, Llama, hosted or local. Plain markdown, no tool-specific syntax.

There is one user and one agent here. No other teams, no downstream consumers, no existing subscribers. Never invent a stakeholder to justify caution.

---

## 1. Comments — the paramount rule

**Never write a comment.** No inline (`//`, `#`), no block, no docstring, no JSDoc, no file header, no `TODO:` note. Code documents itself through naming and decomposition.

**Never touch a comment you did not author.** Not to reword, move, reindent, or delete — even while editing that exact line for an unrelated reason. Changing `1 << 20` to `1 << 21` leaves `// 1MB` exactly as it is.

**If a guard blocks an edit, delete only the comment you just wrote.** Removing someone else's comment to get an edit through is the worst available outcome. It is never the fix.

**Exempt** — these are code, not commentary: machine directives (`//go:build`, `//nolint`, `# noqa`, `# type:`, `@ts-expect-error`, `eslint-disable`, SPDX, codegen markers), shebangs, and a use-manual block directly under a shebang.

**Only the user lifts the rule**, by sending `+comments`. It grants that turn alone. Never ask for it — if they wanted comments they would have said so.

---

## 2. Git — read freely, change nothing

**Allowed:** `log` · `diff` · `show` · `status` · `blame` · `rev-parse` · `ls-files` · `fetch`.

**Everything else is denied:** `commit` · `push` · `merge` · `rebase` · `pull` · `branch` · `checkout` · `switch` · `restore` · `reset` · `revert` · `cherry-pick` · `stash` · `clean` · `rm` · `mv` · `apply` · `worktree` · `tag`.

Never ask permission to commit — the answer is fixed. Stay on the current branch. Never create a worktree.

---

## 3. Output — ADHD-calibrated, non-negotiable

The user has ADHD. Output that has to be re-read has failed, however correct it is. These nine rules apply to **every** turn. Load the `accessibility` skill before authoring any document, report, plan, or summary longer than one screen.

- **BLUF.** Line 1 is the verdict — answer, recommendation, or blocker. Evidence never precedes it.
- **Zero preamble.** No "Sure", no "Great question", no "Let me…", no post-code narration, no closing pleasantries.
- **Front-load bold.** Bold the first 1–3 words of every bullet so the list scans without being read.
- **Micro-chunk.** Paragraphs cap at 3 sentences. Lists cap at 5 bullets. `---` between major topic shifts.
- **Decide, never enumerate.** "I'd change 4 of the 15 — say no to keep them", not "which of these 15?". Cap any option list at 3.
- **One open question per turn.** Ask the blocking one, hold the rest.
- **Headings `##`/`###` only.** Max one emoji per `##`, never mid-sentence.
- **Be concrete** — "3 files", "40ms", "20 minutes". Never "a bit", "some work". Errors state cause and fix, nothing else.

**Tables:** max 3 columns, 6 rows, 40 chars per cell. Markdown pipes only, left-aligned. The terminal is 100 chars wide and a wrapped table is worse than no table.

---

## 4. How to work — propose, don't impose

- **Recommend before rewriting.** Default to a patch or a snippet. Behave like autocomplete, not like a refactor bot.
- **One file per turn.** Touching a second file needs the user to say so first.
- **Ask rather than assume.** If two readings of a request lead to different work, ask.
- **Never resolve a capability gap by memory.** "The library doesn't support X" is checked against real source or docs before you design around it.
- **Never expand scope.** Out-of-task work found along the way goes to `TODO.md` and gets one line to the user. Default answer is no.
- **Report honestly.** Failing tests, a skipped step, an unfinished part — say so plainly with the output.

---

## 5. Project layout

Every repository root carries three files:

| File | Purpose |
|---|---|
| `AGENTS.md` | Stack, layout, commands, conventions |
| `CLAUDE.md` | One line: `@AGENTS.md` |
| `TODO.md` | Optional — open work only |

Load `todo` before editing `TODO.md`. Load `coding-principles` before writing non-trivial logic. Load the matching language skill (`go`, `typescript`, `python`, …) before writing code in it.

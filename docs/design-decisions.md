# Design decisions

Why each rule in `AGENTS.md` and the harness exists, with what would change it. Pre-1.0 rationale lives in git history at tag `v0.51.0`; none of it describes current behaviour.

## The pipeline is code, not a prose state machine

The 0.x loop was a 200-line skill a Claude session interpreted: nine phase gates, a cold fork per task, three to four reviewer forks per band, a full verify on every builder Stop. A one-task change cost 30 to 60 minutes and the orchestrating session re-read 7k tokens of loop text after every compaction. Every step a model does not need to judge is now Python: lint the queue, order the DAG, cut the branch, rerun `done_when`, commit, merge, verify, push. Models do exactly two kinds of work, build and review, and each gets one brief.

What would change it: a step where code keeps guessing wrong. The one candidate is task planning, which is why `komodo tasks plan` exists as a worker call a human reviews before anything runs.

## Guarantees are credential isolation plus one pusher

GitHub Free private repos have no branch protection or rulesets, so the remote cannot enforce anything. The 0.x answer was a 1,376-line text firewall over Bash that scripts, interpreters, and `source` walked around, and that denied `git merge -m` and `git branch --list`. The 1.0 answer is structural: workers run with no token, no credential helper, no SSH identity, and an unauthenticated `gh`, so a worker that ignores its brief still cannot publish. `gitops.py` is the only pusher and refuses protected refs, force, amend, and trailers in code. The git hooks apply the same rules to a human's terminal, and `guard.py` is a 200-line advisory for interactive sessions that denies only what is never right.

What would change it: a GitHub plan with branch protection, which would make the hooks and the guard redundant for the remote.

## Directory ownership decides parallelism

Two tasks run at once only when their `files` live in disjoint directories. A directory is usually one package or module, so the compile gate after a wave catches most cross-file breakage before the next wave starts. File-level disjointness was tried in 0.x and let a change in one file break a neighbour another agent was editing. `mode: single` exists for entangled groups that will not split.

## One review pass, at the group

Three or four specialized reviewer forks per band was the largest single cost in 0.x. One brief now carries bug, security, test-gap, simplify, and the two comment classes with a shared severity scale. Findings at or above the floor become one repair pass; the rest are filed to the backlog by code. `fast` skips review under a diff-size floor because a 40-line change does not earn an Opus pass.

What would change it: a measured regression rate in merged PRs. The report records every finding, so the number is available.

## Comments: docs on public functions, judgment on private ones

The 0.x lint demanded a comment on every function and routed every comment through an `apply` step validated against a nine-type taxonomy. Volume went up and the complaints were about volume. The 1.0 rule: every public function gets a one-line doc, a private function gets one only when longer than `trivial_lines` or with more than one return, and everything else is silent unless the line cannot say it. The lint is mechanical (external reference, name echo, over-words, narrative tells, stacked blocks). The judgment half lives in the reviewer brief as `narrative-comment` and `undocumented-nonobvious` classes. Per-language convention lives in the standard, so godoc, docstrings, and JSDoc each stay idiomatic.

## Standards are files, injected by extension

A 19 KB Go skill loaded on every `.go` touch was the biggest per-fork cost after the loop text. Standards are now rules-only files under `komodo/standards/`, typically 3 to 8 KB, chosen by the extensions and directories a task touches and clipped to a cap. The interactive adapter's `standards-*` skills are one-line pointers to the same file installed under `~/.claude/standards/`, listed `name-only`, so a session and a worker hold identical rules and the always-on listing costs a few tokens each. There is no "off" standard: an unused language costs nothing.

## Settings policy is merged, never linked

The tracked `settings.json` was the live symlinked file, so a preference toggled in the UI landed in the next commit. `settings.policy.json` now carries only `permissions`, `hooks`, and `skillOverrides`; the installer merges those keys into the personal `~/.claude/settings.json` and leaves every other key alone. `komodo doctor` fails on a personal key in the policy file or a tracked `settings.json`.

## Copies, not symlinks

Symlinks needed Developer Mode on Windows and made every edit in the clone live in every session instantly, including a half-finished one. The install is a copy and says so. The cost is a re-run after editing, which `komodo install` prints.

## The backlog is markdown with a machine block

Humans and agents both write the queue, and the point of the queue is to remove tracker MCPs from the loop. Markdown headings stay for people; a fenced yaml block under each task carries `files`, `done_when`, `depends_on`, `context`, and `owner` for the parser. `yamlite` parses the subset deterministically with no dependency. `tasks migrate` converts the 0.x table shape best-effort.

## Accessibility rules are always-on

Formatting rules for people with ADHD, autism, and other attention or processing needs were an optional skill in 0.x. They are now the last section of `AGENTS.md` and apply to every human-facing output from every agent, and `render.py` applies the same caps in code to reports and PR bodies. The only exemption is a worker returning JSON to the orchestrator.

## Fragments fail verify

Three kinds of leftover kept crossing PRs: branches and worktrees from an earlier run, personal settings in commits, and references to skills or paths a previous PR deleted. `komodo doctor` checks the last two inside `scripts/verify.py`, and `komodo status --prune` clears the first. Preflight refuses to start over a stale worktree or an unfinished run without `--resume`.

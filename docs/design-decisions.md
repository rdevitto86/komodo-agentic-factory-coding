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

A 19 KB Go skill loaded on every `.go` touch was the biggest per-fork cost after the loop text. Standards are now rules-only files under `komodo/standards/`, typically 3 to 8 KB, chosen by the extensions and directories a task touches and clipped to a cap. The rendered `standards-*` skills are one-line pointers to the same file installed under `~/.claude/standards/`, listed `name-only`, so a session and a worker hold identical rules and the always-on listing costs a few tokens each. There is no "off" standard: an unused language costs nothing.

## One source, rendered per tool

The first 1.0 draft carried a hand-written `claude-code/` beside the harness: seven agent files, three skills, and an `AGENTS.md` that duplicated the rules the worker briefs already stated. Two copies of a role drift, and a second tool would have meant a third. Rules, roles, and standards now live once under `komodo/`, a role carries both its worker and session output contracts, and `komodo/adapters/claude/` renders the Claude layout from them at install time. Model names left the role files for the same reason: a role declares a tier, and a profile decides what a tier costs, so a `local` profile can point every tier at Ollama without touching a role.

What would change it: a tool whose config cannot be expressed from these inputs, which would argue for widening the role frontmatter rather than hand-writing that tool's layer.

## Settings policy is merged, never linked

The tracked `settings.json` was the live symlinked file, so a preference toggled in the UI landed in the next commit. The adapter's `settings.policy.json` carries only `permissions` and `hooks`, `skillOverrides` is derived from the rendered skills, and the installer merges those keys into the personal `~/.claude/settings.json` and leaves every other key alone. `komodo doctor` fails on a personal key in the policy file or a tracked `settings.json`.

## Copies, not symlinks

Symlinks needed Developer Mode on Windows and made every edit in the clone live in every session instantly, including a half-finished one. The install is a copy and says so. The cost is a re-run after editing, which `komodo install` prints.

## The backlog is markdown with a machine block

Humans and agents both write the queue, and the point of the queue is to remove tracker MCPs from the loop. Markdown headings stay for people; a fenced yaml block under each task carries `files`, `done_when`, `depends_on`, `context`, and `owner` for the parser. `yamlite` parses the subset deterministically with no dependency. `tasks migrate` converts the 0.x table shape best-effort.

## Accessibility rules are always-on

Formatting rules for people with ADHD, autism, and other attention or processing needs were an optional skill in 0.x. They are now the last section of `AGENTS.md` and apply to every human-facing output from every agent, and `render.py` applies the same caps in code to reports and PR bodies. The only exemption is a worker returning JSON to the orchestrator.

## Fragments fail verify

Three kinds of leftover kept crossing PRs: branches and worktrees from an earlier run, personal settings in commits, and references to skills or paths a previous PR deleted. `komodo doctor` checks the last two inside `scripts/verify.py`, and `komodo status --prune` clears the first. Preflight refuses to start over a stale worktree or an unfinished run without `--resume`.

## Command safety is one decision point and many enforcement points

The guard shipped as policy and enforcement fused together. `guard.py` and `guard.go` each hold the rules as constants, each re-implement the same refusals, and both are reachable only from one tool's `PreToolUse` hook. A rule change is therefore an edit in two languages plus a rebuild of six binaries, and a second provider would have meant a third implementation. The shape was already right: a JSON payload in, a decision out. What was wrong is that the policy was welded to the one path that happened to carry it.

Policy becomes data. One declarative ruleset, read by both languages, in JSON because that is stdlib in each and the harness imports nothing else. A rule carries an id and a reason, so a refusal is traceable to a line in a ruleset rather than to a string in a compiled binary.

The decision lives in `komodo/policy.py`: `decide(request) -> Decision`, where a request names the tool, the command, the working directory and the mode, and a decision is `allow`, `deny` or `ask` with the rule that produced it. It is pure and it is tested once. Every enforcement point calls it and none of them re-implement it. That is what makes a third provider cheap: a provider implements a way to intercept its own tool calls, never a policy.

Enforcement is per execution path, and the paths are not equally covered. An interactive session is governed by the `PreToolUse` hook, which works today. A Claude worker is governed by nothing, because workers spawn with `--setting-sources project` and `--dangerously-skip-permissions`, so neither the personal settings nor the hooks in them are ever loaded; closing that is a project-scoped hook and it is the gate on everything else here. Ollama has no enforcement point because it has no tools: the adapter is text in, text out, and its interception belongs in the bridge if and when the bridge grows tools.

The modes are `safe`, `default` and `unsafe`, and they describe which layers are in force rather than how much risk is tolerated. `safe` refuses a whole verb wherever a destructive shape of it exists, and fails closed when the engine itself errors. `default` refuses the destructive shape and allows the recoverable one, so a forced delete of a protected branch is refused while the same flag on a scratch branch is not. `unsafe` keeps the hook and drops the local JSON allow and deny lists, which is coherent precisely because those lists were never an enforcement boundary: a worker skips them entirely, so anything that must hold was always going to hold in the hook or nowhere.

A mode the agent can edit is not a mode. `.komodo/config.json` is gitignored and writable by anything with a file tool, so the posture floor lives outside the repo, in `~/.komodo/policy.json`, and a repo may tighten it but never loosen it. Refusing writes to the policy file was the alternative and it does not hold, because a refusal covers one tool and a file can be written by many.

The engine fails open today: `guard.py` catches every internal error and returns without a decision. That is the right default for a convenience check and the wrong one for a control, so fail behaviour becomes a property of the mode, closed under `safe` and open under the rest. Every denial is appended to `.komodo/runs/<id>/policy.jsonl` with its rule id and command, because without a log there is no answer to what the agent attempted.

What would change it: a provider whose tool calls cannot be intercepted before execution, which would move enforcement into the harness's own command runner and make the per-provider point a fallback rather than the primary.

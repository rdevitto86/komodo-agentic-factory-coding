# Backlog
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · In progress: `[WIP]`

Open work only. Format and rules live in the `write-backlog` skill — load it before editing this file.

## Now — V1

### 1.1 Cross-Cutting
- 1.1.1 | L | git_guard: `strip_leading_flags` fallback heuristic can misfire when an unrecognized flag's value exactly equals a bare monitored-command name with no extension (e.g. `time --output sh actualtool arg`) — contrived/low-likelihood edge case, `claude-code/hooks/git_guard.py` · S → decision recorded or fixed
- 1.1.2 | M | Git hooks: `scripts/hooks/git/` still has no commit-time check for this repo's own `.py` hooks — `pre-push-verify` now covers push (runs `.claude/verify.sh` when present), but a staged syntax error in `git_guard.py`/`comment_guard.py` still passes `git commit` uncaught · M → a staged `.py` syntax error fails `git commit`
- 1.1.3 | L | AGENTS.md states "Git hooks are **not** in this repo — `pre-commit` and `pre-push` ship with the language SDK", contradicted by `scripts/hooks/git/` holding both dispatchers and `install.sh` · S → statement matches disk, or the hooks move to the SDK
- 1.1.4 | L | validate.sh: the "links" check reports "missing (run setup.sh)" for a bare checkout with no `~/.claude` symlinks — confirmed this does fail the overall exit code (`problems` increments, non-zero `problems` triggers `exit 1`), not just warn; still open only pending a decision on whether that's the desired behavior, `scripts/validate.sh` · S → decision recorded
- 1.1.5 | L | README.md's "Workflow skills, all free" bulleted list is missing many existing skills (`write-runbook`, `git-create-pr`, `git-create-issue`, `audit-vulnerabilities`, `audit-dependencies`, `audit-prd`, the four `audit-*` doc skills, `standards-worklog`, `standards-specs`, etc.) — predates this band, not fixed in passing · S → list matches `claude-code/skills/` and `bash scripts/validate.sh` passes
- 1.1.6 | L | No `standards-<language>` skill documents an outdated-dependency/deprecation/EOL tool (only CVE scanners: `govulncheck`, `npm audit`, `pip-audit`) — `audit-dependencies` currently falls back to bare `go list -u -m all`/`npm outdated`/`pip list --outdated` with no documented convention to point to · M → each relevant `standards-<language>` skill names its outdated-dependency tool and `bash scripts/validate.sh` passes
- 1.1.7 | M | Bridge: set `options.num_ctx` in `generateRequest` so large summarizer payloads stop truncating silently
  `[BLOCKED]` no file to edit — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy, not this repo; `bridges/komodo-bridge/` here holds only prompt files and docs. Recheck: reopens once bridge source is vendored into or reachable from this repo. · S → bridge request carries the option
- 1.1.8 | L | `rules-merge-conflicts` — autoloaded rule governing how the agent resolves a merge conflict (when to ask vs. resolve, what never gets silently dropped) · S → `claude-code/skills/rules-merge-conflicts/SKILL.md` exists, `bash scripts/validate.sh` passes
- 1.1.9 | M | `git-create-issue` — git issue creation skill, invocable by both the agent and the human directly (split out from the prior combined git-issue-filing skill) · M → `claude-code/skills/git-create-issue/SKILL.md` exists, `bash scripts/validate.sh` passes
- 1.1.10 | M | `git-create-pr` — git PR creation skill, invocable by both the human and the agent (split out from the prior combined git-PR-opening skill) · M → `claude-code/skills/git-create-pr/SKILL.md` exists, `bash scripts/validate.sh` passes
- 1.1.11 | M | `standards-git` — autoloaded git domain-knowledge skill (branch/commit/push/merge conventions currently in `rules-source-control`) · M → `claude-code/skills/standards-git/SKILL.md` exists, follows the `standards-<noun>` section order, `bash scripts/validate.sh` passes
- 1.1.12 | M | Parse `rules-source-control` apart into `git-create-issue`, `git-create-pr`, and `standards-git` — each rule moves to whichever new skill owns that concern, leaving `rules-source-control` holding only what doesn't fit any of the three (or retiring it if nothing remains) (after: "`standards-git` — autoloaded git domain-knowledge skill (branch/commit/push/merge conventions currently in `rules-source-control`)") · M → `rules-source-control/SKILL.md` content matches the split, `bash scripts/validate.sh` passes
- 1.1.13 | M | Update every reference to the old combined git-issue-filing and git-PR-opening skill names, and to `rules-source-control`, across `workflow-loop` and the other skills that cite them to point at `git-create-issue`/`git-create-pr`/`standards-git` instead (after: "Parse `rules-source-control` apart into `git-create-issue`, `git-create-pr`, and `standards-git` — each rule moves to whichever new skill owns that concern, leaving `rules-source-control` holding only what doesn't fit any of the three (or retiring it if nothing remains)") · M → grep for the old names returns no hits outside `CHANGELOG.md`, `bash scripts/validate.sh` passes
- 1.1.14 | L | Review the skill/tool access assigned to each agent (`workflow-implementer`, `workflow-planner`, `engineering`, `scout`) against what each actually invokes — flag any missing or unneeded grant · S → reviewed, findings recorded or corrected
- 1.1.15 | M | Rename `claude-code/agents/implementer.md` → `workflow-implementer.md` and `claude-code/agents/planner.md` → `workflow-planner.md`, updating every cross-reference in skills and docs · M → both renamed, `bash scripts/validate.sh` and `bash scripts/test-hooks.sh` pass

### 1.2 Cloud standards
- 1.2.1 | L | `standards-aws` — AWS conventions (IAM, VPC/networking basics, service-selection guidance) · M → `claude-code/skills/standards-aws/SKILL.md` exists, follows the `standards-<noun>` section order, `bash scripts/validate.sh` passes
- 1.2.2 | L | `standards-gcp` — GCP conventions · M → same shape, `bash scripts/validate.sh` passes
- 1.2.3 | L | `standards-azure` — Azure conventions · M → same shape, `bash scripts/validate.sh` passes

### 1.3 Language standards
- 1.3.1 | L | `standards-csharp` — C# conventions · M → `claude-code/skills/standards-csharp/SKILL.md` exists, `bash scripts/validate.sh` passes
- 1.3.2 | L | `standards-dotnet` — .NET framework/runtime conventions (paired with `standards-csharp` the way `standards-cdk` pairs with `standards-typescript`) · M → same shape
- 1.3.3 | L | `standards-react` — React conventions, deferring UI/WCAG rules to `standards-uiux` · M → same shape
- 1.3.4 | L | `standards-java` — Java conventions · M → same shape

### 1.4 Security standards
- 1.4.1 | L | `standards-uiux-security` — UI/UX-specific security (clickjacking, CSP, dark patterns, tapjacking) distinct from `standards-uiux`'s Tailwind/WCAG scope and from `standards-security`'s general OWASP baseline · M → `claude-code/skills/standards-uiux-security/SKILL.md` exists, `bash scripts/validate.sh` passes
- 1.4.2 | L | `standards-api-security` — API-specific security (rate limiting, versioning, auth schemes, input contracts) distinct from `standards-security`'s general OWASP baseline · M → same shape

## Next — V2

### 2.1 Cross-Cutting
- 2.1.1 | M | Git identity: agent-run commits and PRs are authored as the user's own GitHub account (`gh auth`'s session token + local `git config user.name`), not a distinct identity — create a machine-user account or GitHub App and wire its token into `gh`/`git` for agent-run operations · M → PR and commit authorship shows the bot identity, not the user's

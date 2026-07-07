# agent-core

Shared agent configuration for all Komodo projects. Agents, cross-cutting standards, language modes, and templates live here at the top level and are symlinked into `~/.claude/` via `setup.sh`. Claude Code is the primary runtime; `platforms/` holds tool-specific adapters (settings, hooks) for each runtime that consumes this config. This is the config layer, not a product codebase: changes here affect every project on the next session.

## Token efficiency

Agent usage runs against a shared subscription — treat tokens like money. Prefer MCP agents over Claude agents (MCP runs outside Claude's context window at zero token cost). Pass scoped `MODES:` on every Claude agent spawn. Compact at phase boundaries. Keep delegation lean — summarize before passing context downstream. Full doctrine and all enforcement rules: `~/.claude/agents/advisor/token-efficiency.md`. The advisor is responsible for enforcing this on every dispatch.

## Hard rules

These are global and non-negotiable. They apply to every agent and override any default behavior.

- **Never commit, push, branch, or merge.** Only the user commits, pushes, branches, and merges. Do not run `git commit`, `git push` (in any form, not just `--force`), `git branch`, `git checkout -b`, `git switch -c`, or `git merge` under any circumstances — not even when asked to "save", "finalize", "ship it", or "start on a feature". Agents may read history (`log`, `diff`, `show`, `blame`, `status`) freely. This is enforced by `git-guard.sh`, which blocks every one of these outright with no prompt — agents must not ask the user to approve them either; always work on the current branch and hand off the actual commit/push/merge to the user.
- **Never spawn agents into an isolated worktree.** Do not use `isolation: "worktree"` (or any equivalent) when dispatching agents. Every agent edits files directly on the user's current branch — isolated worktrees fragment work into parallel trees that are painful to merge and prone to conflicts. Real branches and PRs are a deliberate future step the user takes, not an agent default.
- **Error strings must not contain the function name.** Function context belongs in metadata objects or stack traces only — not in the error message string itself.
- **Zero comments. Ever. No exceptions.** `comments.md` is the single source of truth for every language, every file, and every agent — `standards/comments.md`, no other file defines comment rules. No function/method/class docs, no declaration comments (type, struct, interface, field, var, const — exported or not), no file/package/module headers, no inline or trailing notes — in any language, including scripts. The "why / public API / edge case" licenses are retired; if it needs to survive, it goes in `TODO.md` or a PR description, not in code. Only machine directives, test-file section banners, explicit user requests, and humans' existing comments are exempt.
- **Never expand scope without permission.** If you discover work outside the current task — a bug, a refactor opportunity, an adjacent improvement — stop. Document it in the nearest `TODO.md` and surface it to the user or advisor. Side work is always declined unless explicitly approved.

## Working files: `TODO.md` and `MEMORY.md`

Two local, git-ignored, per-project files in the work repo (not here), never committed. **`TODO.md`** — durable cross-session task ledger (deferred work, out-of-scope finds, known debt; items removed when done, never checked off); created on demand. Agents and the advisor read and **write autonomously** — no per-request permission needed to add/remove items. **`MEMORY.md`** — in-flight session cache (now / next / decisions / watch-outs; pruned and overwritten, not appended); **opt-in via its existence** — present = agents read/write it, absent = memory off and agents never create it. Read both at session start; reconcile `MEMORY.md` against real repo state. Conventions: `~/.claude/standards/{todo,memory}.md`.

## Directory layout

- **`agents/<agent>/`** — each agent is a self-contained module: `agent.md` (the directive; frontmatter `name` = identity), plus role-specific `<mode>/` folders (skills + `docs/`). `software-engineer` keeps only its role modes here (`api/`, `db/`, `design/`, `infra/`); language modes live under top-level `modes/`.
- **`standards/`** — cross-cutting coding standards shared by every agent: `principles.md`, `comments.md`, `security.md`, `logging.md`, `git-flow.md`, `pull-requests.md`, `stack.md`, `writing-style.md`, `readme-maintenance.md`, `changelog.md`. Agents reference these by path (`~/.claude/standards/<name>.md`).
- **`modes/`** — language blueprints, keyed by language: `go/`, `ts/`, `python/`, `cpp/`, `svelte/`, `vue/`. Shared across any agent that writes code in that language (e.g. `software-engineer` and `hardware-engineer` both reference `modes/cpp/coding.md`).
- **`templates/`** — file templates referenced by skills (e.g. `templates/service/*.tmpl` for the `software-engineer` `api` mode's `/new-service`).
- **`platforms/<tool>/`** — tool-specific adapters. `platforms/claude/` holds `settings.json` (permissions, allowed commands, hook registration) and `hooks/` (event scripts), symlinked to `~/.claude/settings.json` and `~/.claude/hooks/` respectively.
- **`profile/`** — global `~/.claude/` entrypoints, symlinked to `~/.claude/AGENTS.md` and `~/.claude/CLAUDE.md`. `AGENTS.md` is the universal, tool-agnostic root directive; `CLAUDE.md` is the Claude adapter that imports `AGENTS.md` plus the advisor agent's full context (`@~/.claude/agents/advisor/agent.md`) so every session opens as the advisor.
- **`scripts/`** — repo maintenance (`validate-refs.sh`) and portable git hook templates (`scripts/hooks/git/`).

Local MCP agents live in `~/.komodo/bridge/`, not here.

Encapsulation is the organizing principle — an agent's knowledge lives in its own folder so spawned agents don't bleed context into each other. `quality-assurance` and `business-architect` inline their rules because they also run as MCP agents with no file access.

## Claude Code agents

Spawned via the `Agent` tool. Defined in `agents/<agent>/agent.md`.

**Default: the `advisor` agent is the entry point for everything.** It is the consigliere and orchestrator — gathers context (user → MCP agents → Claude agents), delegates work autonomously, passes the right modes on each spawn, and insulates the user from operational noise.

| Trigger | Agent | Model | Primary runtime | Fallback | Role |
|---------|-------|-------|-----------------|----------|------|
| `[ADV]` | `advisor` | sonnet | Claude Sonnet | — | **Default. Consigliere, orchestrator, and cross-domain strategist** — advises on strategy/technical/business, decomposes work, dispatches specialists with scoped modes, surfaces only consequential decisions. |
| `[BA]` | `business-architect` | sonnet | MCP `pm` (`analyze_specs`) | Claude Sonnet | Business-context assembly, work-item/story management (tracker-agnostic: Trello/TODO.md/JIRA), and domain/process modeling — the structured-input layer underneath `advisor`'s cross-domain strategy. |
| `[SWE]` | `software-engineer` | sonnet | Claude Sonnet | — | All software — UI, backend, **non-robotics** embedded, system design, architecture. Code review/debugging. All coding routes here except robotics. |
| `[QA]` | `quality-assurance` | sonnet | Claude Sonnet | — | Security review, performance review, test writing. |
| `[OPS]` | `devops` | sonnet | Claude Sonnet | — | CI/CD, infrastructure, deployments, monitoring, incident response. Delegates IaC authoring to `software-engineer`. |
| `[DATA]` | `data-analyst` | sonnet | Claude Sonnet | — | Analytics, BI, KPI/metrics definition and interpretation, ad-hoc data investigation, analytical SQL, dashboards/reporting, A/B experiment analysis. Consumes data; does not author pipelines, ETL, schema, or migrations — that routes to `software-engineer`. |
| `[HWE]` | `hardware-engineer` | sonnet | Claude Sonnet | — | Hardware end to end — circuit design, schematic review, BOM, PCB layout, 3D/CAD modeling and CAM (DFM for print/CNC), and robotics integration including the robotics software (firmware, RTOS, ROS 2 nodes). |
| `[BOT]` | `botanist` | haiku | Claude Haiku | — | Crop health, plant science, agricultural diagnosis. |
| `[WM]` | `logistics` | haiku | Claude Haiku | — | Warehouse management, inventory control, fulfillment, logistics planning, receiving/shipping, WMS. Trigger is `[WM]` (warehouse-management mnemonic) — agent name is `logistics` but scope is the full logistics domain. |
| — | `lawyer` | sonnet | MCP `lawyer` (`review_document`) | Claude Sonnet | Contract review, compliance, legal research. Not legal advice. |
| — | `customer-servicing` | sonnet | MCP `customer-servicing` (`draft_response`) | Claude Sonnet | Customer response drafting, ticket triage, escalation summaries. |
| `[SEC]` | `cyber-security` | sonnet | Claude Sonnet | — | Offensive + defensive security — threat modeling, pentest, security architecture, red/blue-team. Dev-time security stays with `software-engineer`; per-file review stays with `qa`. |
| — | `marketing` | sonnet | MCP `marketing` (`create_content`) | Claude Sonnet | Campaign strategy, copywriting, brand messaging, and sales-content/proposal drafting (supplementing, not replacing, real sellers). |
| `[TAX]` | `tax-advisor` | sonnet | MCP `tax` (`analyze_tax`) | Claude Sonnet | Tax document summarization, exposure/deduction/deadline flagging, first-line due diligence. Not tax advice — escalate to a CPA. |
| — | `summarizer` | — | MCP `summarizer` (`summarize`) | none | **MCP-exclusive.** Condenses raw context (files, logs, transcripts, search dumps) into a faithful digest. Pure compression — no judgment, preserves identifiers/paths/errors verbatim. The advisor calls it to shrink material before any expensive handoff. No Claude fallback by design; if the bridge is down, the caller summarizes inline. |

**Runtime rule:** Claude Sonnet is the universal fallback — if MCP is unreachable, always fall back to the Claude Sonnet subagent. The advisor honors this table at dispatch and never skips MCP for agents where it is the primary runtime. The one exception is `summarizer`, which is MCP-exclusive by design (spending Claude tokens to compress context defeats its purpose); when the bridge is down, the caller summarizes inline rather than spawning a subagent.

**Routing notes:**
- **Hardware and robotics → `hardware-engineer`.** Circuit/PCB/BOM, CAD/CAM, and robotics integration including the robotics software side (firmware, RTOS, ROS 2 nodes) are all one agent, specialized via its mode table (`circuit`, `pcb`, `cad`, `robotics`, `firmware`). `software-engineer`'s `cpp` mode is for non-robotics embedded only. The advisor leans to `hardware-engineer` for anything robotics.
- **Cross-domain/architecture strategy → `advisor`** (the former `architect` role folded in). Deep software/system design → `software-engineer` in `design` mode.
- **Security split — three layers:** `software-engineer` owns dev-time security (input validation, authz, secret handling) per `~/.claude/standards/security.md`. `quality-assurance` owns per-file/per-component security review passes. `cyber-security` owns everything else: cross-cutting threat modeling, security architecture, offensive/pentest, red/blue-team, and deep vulnerability assessment.
- **Sales duties** fold into `advisor` (commercial strategy), `marketing` (campaigns, copy, and proposal/sales-content drafting), and `data-analyst` (sales-data analysis).
- **Context compression → `summarizer` (MCP, zero-cost).** Before any expensive handoff, the advisor pipes large raw material (file dumps, logs, transcripts) through `summarize` and passes only the digest downstream. It has no file access, so fetching and summarizing stay separate: a cheap `Explore` pass (or the advisor) gathers the raw material, then the MCP summarizer compresses it. Pure transform — never route judgment, analysis, or decisions to it.
- **Email is a per-agent mode** (`MODES: email`) on `customer-servicing`, `marketing`, `lawyer`, and `tax-advisor`. Each owns its own tone, formatting, and business rules in its `email.md`; Gmail (MCP) is the shared transport. Email is read-first — an agent never sends without explicit per-message user approval, and never bulk-sends through it.

**Model tiers:** `haiku` (simple/lookup/drafting) · `sonnet` (complex technical work, default) · `opus` (highest reasoning).

**Invoking agents:**
- **Shorthand prefix:** `[SWE] add pagination to the orders endpoint`. Carry modes in the prefix: `[SWE: go, api]`.
- **Natural language:** "use the software-engineer agent to…", "have the advisor look at…"
- **Orchestration:** the advisor decomposes work and dispatches specialists in parallel natively — no routing skill required.

## Modes

Each agent's knowledge splits into **modes** — keyword-activated bundles; a spawned agent loads only the active ones, preventing context bleed. Activate with a `MODES:` line on spawn (`MODES: go, api`) or in the trigger (`[SWE: go, api]`); with none given, the agent infers from the working tree and states what it enabled. Mode tables live in each `agent.md` (e.g. `software-engineer`: `go`, `ts`, `python`, `svelte`, `vue`, `cpp`, `api`, `db`, `infra`, `design`); a mode folder may hold a `docs/` that loads with it.

Caveat: for most agents a mode is a loadable file bundle; for `quality-assurance` it's the review type (`security` | `performance` | `tests`) — same word, different mechanism.

## MCP bridge

MCP agents run on Qwen3 via the komodo bridge (`~/.komodo/bridge`, served at `http://localhost:8000/sse`; start with `docker compose up -d` in `~/.komodo/`). Zero token cost — they run outside Claude's context window. Tool names and roles are in the routing table above (Primary runtime column); Claude subagents are the fallback when the bridge is unreachable.

## Standards & skills

Cross-cutting coding standards live in top-level `standards/` and language modes in top-level `modes/` (see Directory layout); per-agent skills live grouped by mode under each agent's folder. When editing any config prose in this repo, follow `~/.claude/standards/writing-style.md`.

## Hooks (auto-run, global)

Hooks are registered globally in `~/.claude/settings.json` (symlinked from `platforms/claude/settings.json`) and fire on events — they cannot be scoped per-agent, but event + content matching already separates concerns (`business-architect` never triggers the lint hook because it never edits code). Source lives in `platforms/claude/hooks/`, symlinked to `~/.claude/hooks/`.

| Hook | Trigger | What it does |
|------|---------|--------------|
| `~/.claude/hooks/no-comments-guard.sh` | After Edit or Write | Blocks (exit 2) any inserted comment of any kind — function/method docs, declaration comments, file headers, inline and trailing notes — except machine directives and test-file section banners. Rule logic is shared with the git hook via `scripts/lib/comment-rules.awk`. Scans only the inserted text, so pre-existing comments are untouched. |
| `~/.claude/hooks/git-guard.sh` | Before any `Bash` command | Blocks (exit 2) `git commit`, `git push` (any form, not just `--force`), `git merge`, `git branch`, `git checkout -b`, `git switch -c`, `git reset --hard`, and `git clean -f`/`-fd`/`-fdx` — anywhere in the command string, regardless of `&&`/`;`/`\|` chaining or `sh -c` wrapping. Enforces the "never commit, push, branch, or merge" hard rule; also backed by matching `deny` entries in `settings.json` so the permission system never even surfaces a prompt. |
| `~/.claude/hooks/stop-summary.sh` | Session end | Shows git diff summary if uncommitted changes exist |

Per-edit linting was removed deliberately (it ran `golangci-lint`/`tsc` on every edit — slow, and failures churned tokens). Lint enforcement lives at commit time (`scripts/hooks/git/pre-commit-lint`) and in CI; `post-edit-lint.sh` remains available but unregistered.

## Setup

Run once after cloning:

```bash
bash setup.sh
```

Symlinks `agents/`, `standards/`, `modes/`, `templates/`, `platforms/claude/settings.json`, `platforms/claude/hooks/`, `profile/AGENTS.md`, and `profile/CLAUDE.md` into `~/.claude/` and removes stale links from the old layout. Restart Claude Code after running.

For commit-time enforcement outside Claude Code — the model-agnostic floor that catches any agent (GPT, Gemini, local) in any IDE — install the portable git hooks under `scripts/hooks/git/` into the target repo's `.git/hooks/pre-commit` (komodo-ecom: `just init-hooks`; see README §Git hooks).

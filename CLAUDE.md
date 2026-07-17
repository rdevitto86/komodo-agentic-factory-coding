# agent-core

Shared agent configuration for all Komodo projects. Agents, cross-cutting standards, language modes, and templates live here at the top level and are symlinked into `~/.claude/` via `setup.sh`. Claude Code is the primary runtime; `platforms/` holds tool-specific adapters (settings, hooks) for each runtime that consumes this config. This is the config layer, not a product codebase: changes here affect every project on the next session.

## Token efficiency

Agent usage runs against a shared subscription — treat tokens like money. Prefer MCP agents over Claude agents (MCP runs outside Claude's context window at zero token cost). Pass scoped `MODES:` on every Claude agent spawn. Compact at phase boundaries. Keep delegation lean — summarize before passing context downstream. Full doctrine and all enforcement rules: `~/.claude/agents/advisor/token-efficiency.md`. The advisor is responsible for enforcing this on every dispatch.

## Hard rules

Global and non-negotiable, apply to every agent — full list: `~/.claude/AGENTS.md` § Immutable rules (loaded every session before this file). Mechanical enforcement for the git rules is documented below in § Hooks (`git-guard.sh`, matching `settings.json` deny entries).

## Working files: `TODO.md` and `MEMORY.md` (consumer projects only)

These are per-project working files that live in the **consumer work repo** an agent is operating on — never in this config layer, which maintains no `TODO.md` or `MEMORY.md` of its own. `TODO.md` is a durable cross-session task ledger; `MEMORY.md` is an opt-in in-flight session cache (present = used, absent = off, never auto-created). Full read/write doctrine: `~/.claude/standards/{todo,memory}.md`. Agents apply these to whatever project they are working in; this repo is not one of them.

## Directory layout

- **`agents/<agent>/`** — each agent is a self-contained module: `agent.md` (the directive; frontmatter `name` = identity), plus role-specific `<mode>/` folders (skills + `docs/`). `software-engineer` keeps only its role modes here (`api/`, `db/`, `design/`, `infra/`); language modes live under top-level `modes/`.
- **`standards/`** — cross-cutting coding standards shared by every agent: `principles.md`, `comments.md`, `security.md`, `logging.md`, `git-flow.md`, `pull-requests.md`, `stack.md`, `writing-style.md`, `readme-maintenance.md`, `changelog.md`. Agents reference these by path (`~/.claude/standards/<name>.md`).
- **`modes/`** — language blueprints, keyed by language: `go/`, `ts/`, `python/`, `cpp/`, `svelte/`, `vue/`. Shared across any agent that writes code in that language (e.g. `software-engineer` and `hardware-engineer` both reference `modes/cpp/coding.md`).
- **`templates/`** — file templates referenced by skills (e.g. `templates/service/*.tmpl` for the `software-engineer` `api` mode's `/new-service`).
- **`platforms/<tool>/`** — tool-specific adapters. `platforms/claude/` holds `settings.json` (permissions, allowed commands, hook registration), `hooks/` (event scripts), and `skills/` (user-invocable slash skills), symlinked to `~/.claude/settings.json`, `~/.claude/hooks/`, and `~/.claude/skills/` respectively.
- **`profile/`** — global `~/.claude/` entrypoints, symlinked to `~/.claude/AGENTS.md` and `~/.claude/CLAUDE.md`. `AGENTS.md` is the universal, tool-agnostic root directive; `CLAUDE.md` is the Claude adapter that imports `AGENTS.md` plus the advisor agent's full context (`@~/.claude/agents/advisor/agent.md`) so every session opens as the advisor.
- **`orchestration/`** — `models.yaml` (tier → model map per backend, human-edited) and `policy.md` (the firm backend + model selection rules). Single source for "which model, which backend" on every dispatch; see `agents/advisor/agent.md` § Duty classes and dispatch decision flow.
- **`scripts/`** — repo maintenance (`validate-refs.sh`, `doctor.sh`, `gen-bridge-registry.sh`) and portable git hook templates (`scripts/hooks/git/`).

Local MCP agents live in `~/.komodo/bridge/`, not here.

Encapsulation is the organizing principle — an agent's knowledge lives in its own folder so spawned agents don't bleed context into each other. `quality-assurance` and `business-architect` inline their rules because they also run as MCP agents with no file access.

### Where a skill lives

Only the frontmatter description loads until a skill is invoked, so flat placement under `platforms/claude/skills/` is cheap — the test is who invokes it, not how big it is.

| Kind | Placement | Test |
|------|-----------|------|
| Slash skill (user types `/x`) | `platforms/claude/skills/<name>/` | User invokes it directly |
| Agent craft (one agent's own procedure, e.g. `api/new-service.md`) | Encapsulated in that agent's mode folder | Only one agent ever runs it |
| Shared workflow (identical procedure across agents, e.g. audit-style checks) | `platforms/claude/skills/<name>/`, never duplicated per-agent | Multiple agents need the same procedure |

## Claude Code agents

Spawned via the `Agent` tool. Defined in `agents/<agent>/agent.md`.

**Default: the `advisor` agent is the entry point for everything.** It is the consigliere and orchestrator — gathers context (user → MCP agents → Claude agents), delegates work autonomously, passes the right modes on each spawn, and insulates the user from operational noise. It also carries an **edit leash**: small/low-risk/SKIP-tier work (a README tweak, a config change, a one-file edit) it implements inline, no spawn; large or high-stakes work still gets a tiered spawn or a separate oversight profile. Full dispatch logic: `agents/advisor/agent.md` § Duty classes and dispatch decision flow.

| Trigger | Agent | Model | Primary runtime | Fallback | Role |
|---------|-------|-------|-----------------|----------|------|
| `[ADV]` | `advisor` | sonnet | Claude Sonnet | — | **Default.** Consigliere/orchestrator (full role: its own `agent.md`). |
| `[BA]` | `business-architect` | sonnet | MCP `pm` (`analyze_specs`) | Claude Sonnet | Business-context assembly and work-item/story management — structured-input layer under `advisor`'s cross-domain strategy (full role: its own `agent.md`). |
| `[SWE]` | `software-engineer` | sonnet | Claude Sonnet | — | All software implementation and design (full role: its own `agent.md`). **All coding routes here except robotics.** |
| `[QA]` | `quality-assurance` | sonnet | Claude Sonnet | — | Security review, performance review, test writing. |
| `[OPS]` | `devops` | sonnet | Claude Sonnet | — | CI/CD, infrastructure, deployments, monitoring, incident response (full role: its own `agent.md`). **Delegates IaC authoring to `software-engineer`.** |
| `[DATA]` | `data-analyst` | sonnet | Claude Sonnet | — | Analytics, BI, metrics, dashboards, experiment analysis (full role: its own `agent.md`). **Consumes data; does not author pipelines, ETL, schema, or migrations — that routes to `software-engineer`.** |
| `[HWE]` | `hardware-engineer` | sonnet | Claude Sonnet | — | Hardware end to end incl. robotics software (full role: its own `agent.md`). |
| `[BOT]` | `botanist` | haiku | Claude Haiku | — | Crop health, plant science, agricultural diagnosis. |
| `[WM]` | `logistics` | haiku | Claude Haiku | — | Warehouse management, inventory, fulfillment, logistics planning, WMS. Trigger is `[WM]` (warehouse-management mnemonic) — agent name is `logistics` but scope is the full logistics domain. |
| — | `lawyer` | sonnet | MCP `lawyer` (`review_document`) | Claude Sonnet | Contract review, compliance, legal research. Not legal advice. |
| — | `customer-servicing` | sonnet | MCP `customer-servicing` (`draft_response`) | Claude Sonnet | Customer response drafting, ticket triage, escalation summaries. |
| `[SEC]` | `cyber-security` | sonnet | Claude Sonnet | — | Offensive + defensive security (full role: its own `agent.md`). **Dev-time security stays with `software-engineer`; per-file review stays with `qa`.** |
| — | `marketing` | sonnet | MCP `marketing` (`create_content`) | Claude Sonnet | Campaign strategy, copywriting, brand messaging, and sales-content/proposal drafting (supplementing, not replacing, real sellers). |
| `[TAX]` | `tax-advisor` | sonnet | MCP `tax` (`analyze_tax`) | Claude Sonnet | Tax document summarization and first-line due diligence (full role: its own `agent.md`). **Not tax advice — escalate to a CPA.** |
| — | `summarizer` | — | MCP `summarizer` (`summarize`) | none | **MCP-exclusive**, no judgment. Condenses raw context into a faithful digest before an expensive handoff. No Claude fallback by design; if the bridge is down, the caller summarizes inline. |

**Runtime rule:** Claude Sonnet is the universal fallback — if MCP is unreachable, always fall back to the Claude Sonnet subagent. The advisor honors this table at dispatch and never skips MCP for agents where it is the primary runtime. The one exception is `summarizer`, which is MCP-exclusive by design (spending Claude tokens to compress context defeats its purpose); when the bridge is down, the caller summarizes inline rather than spawning a subagent.

**Routing notes:**
- **Hardware and robotics → `hardware-engineer`.** Circuit/PCB/BOM, CAD/CAM, and robotics integration including the robotics software side (firmware, RTOS, ROS 2 nodes) are all one agent, specialized via its mode table (`circuit`, `pcb`, `cad`, `robotics`, `firmware`). `software-engineer`'s `cpp` mode is for non-robotics embedded only. The advisor leans to `hardware-engineer` for anything robotics.
- **Cross-domain/architecture strategy → `advisor`** (the former `architect` role folded in). Deep software/system design → `software-engineer` in `design` mode.
- **Security split — three layers:** `software-engineer` owns dev-time security (input validation, authz, secret handling) per `~/.claude/standards/security.md`. `quality-assurance` owns per-file/per-component security review passes. `cyber-security` owns everything else: cross-cutting threat modeling, security architecture, offensive/pentest, red/blue-team, and deep vulnerability assessment.
- **Sales duties** fold into `advisor` (commercial strategy), `marketing` (campaigns, copy, and proposal/sales-content drafting), and `data-analyst` (sales-data analysis).
- **Context compression → `summarizer` (MCP, zero-cost).** Before any expensive handoff, the advisor pipes large raw material (file dumps, logs, transcripts) through `summarize` and passes only the digest downstream. It has no file access, so fetching and summarizing stay separate: a cheap `Explore` pass (or the advisor) gathers the raw material, then the MCP summarizer compresses it. Pure transform — never route judgment, analysis, or decisions to it.
- **Email is a per-agent mode** (`MODES: email`) on `customer-servicing`, `marketing`, `lawyer`, and `tax-advisor`. Each owns its own tone, formatting, and business rules in its `email.md`; Gmail (MCP) is the shared transport. Email is read-first — an agent never sends without explicit per-message user approval, and never bulk-sends through it.

**Model tiers:** `haiku` (simple/lookup/drafting) · `sonnet` (complex technical work, default) · `opus` (highest reasoning).

### Tiers and duty classes

Each agent's frontmatter now also carries `tier` (cost axis: `small` / `medium` / `large` / `heavy`) and `duty_class` (`producer` / `oversight` / `advisory`) — the taxonomy that decouples *what expertise* from *how much compute*. Full rationale and the dispatch flow that resolves these per task: `docs/orchestration.md`, `agents/advisor/agent.md` § Duty classes and dispatch decision flow.

| Agent | Tier (frontmatter default) | Duty class |
|---|---|---|
| `advisor` | medium | advisory (primary, always-on) |
| `software-engineer` | medium (task-dependent: small→heavy) | producer |
| `quality-assurance` | medium (security-review floor: heavy) | **oversight — `spawn_only: true`** |
| `cyber-security` | heavy | oversight for review work; producer for pentest/architecture (mixed — see its `agent.md`) |
| `devops` | medium | producer |
| `data-analyst` | medium | advisory |
| `hardware-engineer` | large | producer |
| `business-architect` | medium | advisory |
| `lawyer` | medium | advisory |
| `customer-servicing` | small | producer |
| `marketing` | medium | producer |
| `tax-advisor` | medium | advisory |
| `botanist` | small | advisory |
| `logistics` | small | advisory |
| `summarizer` | small | producer, no judgment |

**This table is the frontmatter default only — a fallback for a plain spawn.** The model actually used for a given task is chosen per task by the advisor from `orchestration/models.yaml` (tier → model per backend) via the firm rules in `orchestration/policy.md` (Claude vs. local, safety floors on high-stakes work), passed through the `Agent` tool's `model` param — never by rewriting an agent's frontmatter. `oversight` profiles are `spawn_only`: never worn inline by the advisor, always a fresh spawn, and only on a cross-review MUST trigger (see the advisor directive's Cross-review dispatch table) — SKIP-tier work gets zero review spawns.

**Invoking agents:**
- **Shorthand prefix:** `[SWE] add pagination to the orders endpoint`. Carry modes in the prefix: `[SWE: go, api]`.
- **Natural language:** "use the software-engineer agent to…", "have the advisor look at…"
- **Orchestration:** the advisor decomposes work and dispatches specialists in parallel natively — no routing skill required.

## Modes

Each agent's knowledge splits into **modes** — keyword-activated bundles; a spawned agent loads only the active ones, preventing context bleed. Activate with a `MODES:` line on spawn (`MODES: go, api`) or in the trigger (`[SWE: go, api]`); with none given, the agent infers from the working tree and states what it enabled. Mode tables live in each `agent.md` (e.g. `software-engineer`: `go`, `ts`, `python`, `svelte`, `vue`, `cpp`, `api`, `db`, `infra`, `design`); a mode folder may hold a `docs/` that loads with it.

Caveat: for most agents a mode is a loadable file bundle; for `quality-assurance` it's the review type (`security` | `performance` | `tests`) — same word, different mechanism.

## MCP bridge

MCP agents run on Qwen3 via the komodo bridge (`~/.komodo/bridge`, served at `http://localhost:8000/sse`; start with `docker compose up -d` in `~/.komodo/`). Zero token cost — they run outside Claude's context window. Tool names and roles are in the routing table above (Primary runtime column); Claude subagents are the fallback when the bridge is unreachable.

**The actual switch:** `runtime.json` at the repo root (tracked in git) holds `default_runtime` (`"claude"` or `"local"`) plus per-agent `overrides` — the routing table above still documents which MCP tool an agent maps to (or none), `runtime.json` decides whether that mapping gets used. Change it with `scripts/set-runtime.sh status|local|claude [--agent NAME]|clear --agent NAME`, never by hand. The advisor reads it before every dispatch (`agents/advisor/agent.md` § Delegation).

**Bridge coverage today:** only 6 of 15 agents have a real MCP tool — `business-architect`, `lawyer`, `customer-servicing`, `marketing`, `tax-advisor`, `summarizer` (see table above). The other 9 (`advisor`, `software-engineer`, `quality-assurance`, `devops`, `data-analyst`, `hardware-engineer`, `cyber-security`, `botanist`, `logistics`) have no bridge tool yet — setting `default_runtime: "local"` will not change their behavior until the home-server bridge covers them; they keep running on Claude via the universal fallback.

**Registry:** `platforms/komodo-bridge/registry.generated.json` — agent → tier → duty class → default backend, generated from `agents/*/agent.md` frontmatter by `scripts/gen-bridge-registry.sh`. Generated, never hand-edited; regenerate after any frontmatter change. The bridge project consumes this later as its agent roster source.

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

Symlinks `agents/`, `standards/`, `modes/`, `templates/`, `platforms/claude/settings.json`, `platforms/claude/hooks/`, `profile/AGENTS.md`, and `profile/CLAUDE.md` into `~/.claude/` and removes stale links from the old layout. Restart Claude Code after running. `scripts/doctor.sh` verifies every `~/.claude` symlink resolves (catches the exact dangling-link breakage a repo-folder rename causes) — run standalone or let `setup.sh` run it as part of its post-link validation.

For commit-time enforcement outside Claude Code — the model-agnostic floor that catches any agent (GPT, Gemini, local) in any IDE — install the portable git hooks under `scripts/hooks/git/` into the target repo's `.git/hooks/pre-commit` (komodo-ecom: `just init-hooks`; see README §Git hooks).

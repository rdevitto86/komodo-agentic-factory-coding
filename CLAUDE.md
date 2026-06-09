# komodo-claude

Shared Claude Code configuration for all Komodo projects. Agents, hooks, and settings live here and are symlinked into `~/.claude/` via `setup.sh`. Each agent is a self-contained module that owns its own standards, skills, modes, and docs — there is no global standards or skills directory. This is the config layer, not a product codebase: changes here affect every project on the next Claude Code session.

## Token efficiency

Agent usage runs against a shared subscription — treat tokens like money. Prefer MCP agents over Claude agents (MCP runs outside Claude's context window at zero token cost). Pass scoped `MODES:` on every Claude agent spawn. Compact at phase boundaries. Keep delegation lean — summarize before passing context downstream. Full doctrine and all enforcement rules: `~/.claude/agents/advisor/token-efficiency.md`. The advisor is responsible for enforcing this on every dispatch.

## Hard rules

These are global and non-negotiable. They apply to every agent and override any default behavior.

- **Never create git commits or git branches.** Only the user commits, branches, and merges. Do not run `git commit`, `git branch`, or `git checkout -b` under any circumstances — not even when asked to "save", "finalize", or "start on a feature". Always work on the current branch.
- **Never spawn agents into an isolated worktree.** Do not use `isolation: "worktree"` (or any equivalent) when dispatching agents. Every agent edits files directly on the user's current branch — isolated worktrees fragment work into parallel trees that are painful to merge and prone to conflicts. Real branches and PRs are a deliberate future step the user takes, not an agent default.
- **Error strings must not contain the function name.** Function context belongs in metadata objects or stack traces only — not in the error message string itself.
- **All comments follow `comments.md` exactly — it is the single source of truth for comment rules, every language and every file.** It lives at `claude/agents/swe/comments.md`. No other file defines comment rules or shows comment examples. The hard violations it defines (file/package-level doc blocks, name-leading doc comments, verbose multi-line blocks, commenting a type merely to describe it) are non-negotiable.
- **Never expand scope without permission.** If you discover work outside the current task — a bug, a refactor opportunity, an adjacent improvement — stop. Document it in the nearest `TODO.md` and surface it to the user or advisor. Side work is always declined unless explicitly approved.

## Working files: `TODO.md` and `MEMORY.md`

Two local, git-ignored, per-project files in the work repo (not here), never committed. **`TODO.md`** — durable cross-session task ledger (deferred work, out-of-scope finds, known debt; items removed when done, never checked off); created on demand. Agents and the advisor read and **write autonomously** — no per-request permission needed to add/remove items. **`MEMORY.md`** — in-flight session cache (now / next / decisions / watch-outs; pruned and overwritten, not appended); **opt-in via its existence** — present = agents read/write it, absent = memory off and agents never create it. Read both at session start; reconcile `MEMORY.md` against real repo state. Conventions: `~/.claude/agents/project-manager/{todo,memory}.md`.

## Directory layout

`claude/agents/<agent>/` — each agent is a self-contained module: `agent.md` (the directive; frontmatter `name` = identity), always-on `<standard>.md` files, and `<mode>/` folders (standards + skills + `docs/`). `claude/hooks/` — global event scripts. `settings.json` — permissions, allowed commands, hook registration. Local MCP agents live in `~/.komodo/bridge/`, not here.

Encapsulation is the organizing principle — an agent's knowledge lives in its own folder so spawned agents don't bleed context into each other. Cross-cutting coding standards (principles, comments, security, PRs, stack, git-flow, logging, readme-maintenance, changelog) live under `claude/agents/swe/`; other agents reference them by path. `quality-assurance` and `project-manager` inline their rules because they also run as MCP agents with no file access.

## Claude Code agents

Spawned via the `Agent` tool. Defined in `claude/agents/<agent>/agent.md`.

**Default: the `advisor` agent is the entry point for everything.** It is the consigliere and orchestrator — gathers context (user → MCP agents → Claude agents), delegates work autonomously, passes the right modes on each spawn, and insulates the user from operational noise.

| Trigger | Agent | Model | Primary runtime | Fallback | Role |
|---------|-------|-------|-----------------|----------|------|
| `[ADV]` | `advisor` | sonnet | Claude Sonnet | — | **Default. Consigliere, orchestrator, and cross-domain strategist** — advises on strategy/technical/business, decomposes work, dispatches specialists with scoped modes, surfaces only consequential decisions. |
| `[PM]` | `project-manager` | sonnet | MCP `pm` (`analyze_specs`) | Claude Sonnet | Story/work-item management (tracker-agnostic: Trello/TODO.md/JIRA) and business-context assembly for other agents. |
| `[SWE]` | `swe` | sonnet | Claude Sonnet | — | All software — UI, backend, **non-robotics** embedded, system design, architecture. Code review/debugging. All coding routes here except robotics. |
| `[QA]` | `quality-assurance` | sonnet | Claude Sonnet | — | Security review, performance review, test writing. |
| `[OPS]` | `devops` | sonnet | Claude Sonnet | — | CI/CD, infrastructure, deployments, monitoring, incident response. Delegates IaC authoring to `swe`. |
| `[DATA]` | `data-analyst` | sonnet | Claude Sonnet | — | Analytics, BI, KPI/metrics definition and interpretation, ad-hoc data investigation, analytical SQL, dashboards/reporting, A/B experiment analysis. Consumes data; does not author pipelines, ETL, schema, or migrations — that routes to `swe`. |
| `[EE]` | `electrical-engineer` | sonnet | Claude Sonnet | — | Circuit design, schematic review, PCB layout, power systems, component selection, EMC. |
| `[MECH]` | `mechatronics` | sonnet | Claude Sonnet | — | **Robotics end to end — hardware AND the robotics software** (firmware, RTOS, ROS 2 nodes), actuator/sensor interfaces, integration, design reviews. |
| `[CAD]` | `machinist` | sonnet | Claude Sonnet | — | CAD / 3D modeling / CAM — parametric part design, DFM for 3D printing and CNC, tolerances/fits, material selection, toolpath and G-code awareness, model review. Tools: Fusion 360, FreeCAD, SolidWorks, AutoCAD, slicers. |
| `[BOT]` | `botanist` | haiku | Claude Haiku | — | Crop health, plant science, agricultural diagnosis. |
| `[WM]` | `logistics` | haiku | Claude Haiku | — | Warehouse management, inventory control, fulfillment, logistics planning, receiving/shipping, WMS. Trigger is `[WM]` (warehouse-management mnemonic) — agent name is `logistics` but scope is the full logistics domain. |
| — | `lawyer` | sonnet | MCP `lawyer` (`review_document`) | Claude Sonnet | Contract review, compliance, legal research. Not legal advice. |
| — | `customer-servicing` | sonnet | MCP `customer-servicing` (`draft_response`) | Claude Sonnet | Customer response drafting, ticket triage, escalation summaries. |
| `[SEC]` | `cyber-security` | sonnet | Claude Sonnet | — | Offensive + defensive security — threat modeling, pentest, security architecture, red/blue-team. Dev-time security stays with `swe`; per-file review stays with `qa`. |
| — | `marketing` | sonnet | MCP `marketing` (`create_content`) | Claude Sonnet | Campaign strategy, copywriting, brand messaging, and sales-content/proposal drafting (supplementing, not replacing, real sellers). |
| `[TAX]` | `tax-advisor` | sonnet | MCP `tax` (`analyze_tax`) | Claude Sonnet | Tax document summarization, exposure/deduction/deadline flagging, first-line due diligence. Not tax advice — escalate to a CPA. |
| — | `summarizer` | — | MCP `summarizer` (`summarize`) | none | **MCP-exclusive.** Condenses raw context (files, logs, transcripts, search dumps) into a faithful digest. Pure compression — no judgment, preserves identifiers/paths/errors verbatim. The advisor calls it to shrink material before any expensive handoff. No Claude fallback by design; if the bridge is down, the caller summarizes inline. |

**Runtime rule:** Claude Sonnet is the universal fallback — if MCP is unreachable, always fall back to the Claude Sonnet subagent. The advisor honors this table at dispatch and never skips MCP for agents where it is the primary runtime. The one exception is `summarizer`, which is MCP-exclusive by design (spending Claude tokens to compress context defeats its purpose); when the bridge is down, the caller summarizes inline rather than spawning a subagent.

**Routing notes:**
- **Robotics → `mechatronics`.** It owns the robotics software side (firmware, control loops, ROS nodes). `swe`'s `cpp` mode is for non-robotics embedded only. The advisor leans to mechatronics for anything robotics.
- **Cross-domain/architecture strategy → `advisor`** (the former `architect` role folded in). Deep software/system design → `swe` in `design` mode.
- **CAD/mechanical split:** `machinist` owns part geometry and manufacturability (3D printing, CNC). Finished parts hand off to `mechatronics` for robotics integration and firmware/control. `electrical-engineer` owns PCB layout and electronics-driven enclosure constraints — coordinate with EE when a part must accommodate board mounting or connector cutouts.
- **Security split — three layers:** `swe` owns dev-time security (input validation, authz, secret handling) per `~/.claude/agents/swe/security.md`. `quality-assurance` owns per-file/per-component security review passes. `cyber-security` owns everything else: cross-cutting threat modeling, security architecture, offensive/pentest, red/blue-team, and deep vulnerability assessment.
- **Sales duties** fold into `advisor` (commercial strategy), `marketing` (campaigns, copy, and proposal/sales-content drafting), and `data-analyst` (sales-data analysis).
- **Context compression → `summarizer` (MCP, zero-cost).** Before any expensive handoff, the advisor pipes large raw material (file dumps, logs, transcripts) through `summarize` and passes only the digest downstream. It has no file access, so fetching and summarizing stay separate: a cheap `Explore` pass (or the advisor) gathers the raw material, then the MCP summarizer compresses it. Pure transform — never route judgment, analysis, or decisions to it.
- **Email is a per-agent mode** (`MODES: email`) on `customer-servicing`, `marketing`, `lawyer`, and `tax-advisor`. Each owns its own tone, formatting, and business rules in its `email.md`; Gmail (MCP) is the shared transport. Email is read-first — an agent never sends without explicit per-message user approval, and never bulk-sends through it.

**Model tiers:** `haiku` (simple/lookup/drafting) · `sonnet` (complex technical work, default) · `opus` (highest reasoning).

**Invoking agents:**
- **Shorthand prefix:** `[SWE] add pagination to the orders endpoint`. Carry modes in the prefix: `[SWE: go, api]`.
- **Natural language:** "use the swe agent to…", "have the advisor look at…"
- **Orchestration:** the advisor decomposes work and dispatches specialists in parallel natively — no routing skill required.

## Modes

Each agent's knowledge splits into **modes** — keyword-activated bundles; a spawned agent loads only the active ones, preventing context bleed. Activate with a `MODES:` line on spawn (`MODES: go, api`) or in the trigger (`[SWE: go, api]`); with none given, the agent infers from the working tree and states what it enabled. Mode tables live in each `agent.md` (e.g. `swe`: `go`, `ts`, `python`, `svelte`, `cpp`, `api`, `db`, `infra`, `design`); a mode folder may hold a `docs/` that loads with it.

Caveat: for most agents a mode is a loadable file bundle; for `quality-assurance` it's the review type (`security` | `performance` | `tests`) — same word, different mechanism.

## MCP bridge

MCP agents run on Qwen3 via the komodo bridge (`~/.komodo/bridge`, served at `http://localhost:8000/sse`; start with `docker compose up -d` in `~/.komodo/`). Zero token cost — they run outside Claude's context window. Tool names and roles are in the routing table above (Primary runtime column); Claude subagents are the fallback when the bridge is unreachable.

## Standards & skills

No global standards or skills directory — each is owned by an agent, in its folder, grouped by mode. Cross-cutting coding standards live under `claude/agents/swe/` (see Directory layout). When editing any config prose in this repo, follow `~/.claude/agents/swe/writing-style.md`.

## Hooks (auto-run, global)

Hooks are registered globally in `settings.json` and fire on events — they cannot be scoped per-agent, but event + content matching already separates concerns (a PM agent never triggers the lint hook because it never edits code).

| Hook | Trigger | What it does |
|------|---------|--------------|
| `post-edit-lint.sh` | After Edit or Write | Runs `golangci-lint` (Go) or `tsc --noEmit` (TS/Svelte) |
| `stop-summary.sh` | Session end | Shows git diff summary if uncommitted changes exist |
| `post-pr-trello.sh` | After `gh pr create` | Surfaces PR URL and Trello card reminder |

## Setup

Run once after cloning:

```bash
bash setup.sh
```

Symlinks `claude/` contents into `~/.claude/` and removes stale links from the old layout. Restart Claude Code after running.

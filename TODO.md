# Audit TODO — komodo-claude-core

Remaining items from audits.

---

## Settings and permissions

### Tighten broad bash permissions (rm / sed / awk)
**Problem:** `claude/settings.json` still allows `Bash(rm:*)`, `Bash(sed:*)`, `Bash(awk:*)`. `rm:*` is a foot-gun; `sed`/`awk` in-place edits should route through `Edit`/`Write`.
**Proposed:** Remove all three from the allowlist — force a prompt on each destructive shell edit.

### Run `/fewer-permission-prompts` to right-size the allowlist
**Benefit:** Skill reads recent transcripts, finds commands that triggered permission prompts, and adds only those specific invocations to the allowlist. Lets you drop wildcard entries previously added in frustration. One-shot run, no recurring cost.

---

## Hooks

### Decide on `post-pr-trello.sh`
**Problem:** Hook prints a "PR created" box and a manual `/trello status` reminder; the real Trello integration is commented out.
**Options:** (a) finish the Trello MCP integration; (b) remove the decorative box and the hook; (c) leave as-is.

---

## Standards

### Add `dockerfile.md` / `terraform.md`
**Problem:** `/new-tf-module` and `/new-service` generate Dockerfiles and Terraform, but no standard governs them.
**Decision:** Terraform likely yes (multi-service); Dockerfile maybe.

---

## Tooling

### Add a `statusLine` config
**Problem:** No status line configured. The `statusline-setup` agent exists for this.
**Decision:** Want one? If so, what info (git branch, dir, model)?

---

## Agent system — gaps from audit

### Add project `CLAUDE.md` template
**Problem:** Agents delegate to project CLAUDE.md for tech-stack overrides, but there's no template specifying what that file must contain. Coverage is inconsistent across projects.
**Proposed:** Add a `claude/standards/project-claude-template.md` (or similar) covering: stack, service topology, how to run/test/build/deploy, key library choices.

### Add `sveltekit.md` standard
**Problem:** UI skills scaffold Svelte 5 with runes, and `testing.md` mentions SvelteKit conventions, but no standard covers runes vs. stores, load functions, form actions, or component patterns.

### Add `errors.md` standard
**Problem:** Hard rule covers error string format, but nothing governs Go error wrapping (`%w`, sentinel errors, structured error types) or TS error handling (Result types vs. throw). The `swe` agent improvises today.

### Fix `post-pr-trello.sh` hook scope
**Problem:** Hook matcher is `Bash` (fires on every shell command). Should scope to `Bash(gh pr create*)` to avoid running on every `gh` call (issue lookups, PR comments, etc.).

### Add `escalation-levels.md` standard
**Problem:** Advisor doctrine says "only surface what requires user attention" but there's no shared language for severity across the agent chain. Agents escalate at different thresholds.
**Proposed:** Brief standard defining FYI / decision-needed / urgent signals so escalation noise is consistent.

### Add `/new-agent` skill
**Problem:** Skills exist for every scaffold type except creating a new Claude agent or MCP agent definition. This will be used repeatedly as the system grows.

### Add pre-tool-use hook for destructive Bash patterns
**Problem:** `rm -rf` and `git reset --hard` are permitted by the allowlist with no pre-check. A `PreToolUse(Bash)` hook flagging destructive patterns before execution would catch accidents.
**Note:** Related to the existing "Tighten broad bash permissions" item above — resolve that first.

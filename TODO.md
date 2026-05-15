# Audit TODO — komodo-claude-core

Remaining items from the 2026-05-13 audit.

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

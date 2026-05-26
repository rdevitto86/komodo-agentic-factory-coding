# TODO.md Conventions

Standards for reading and writing `TODO.md` files across Komodo projects.

---

## Reading

Before starting any significant task, check `TODO.md` at the project root and in relevant subdirectories (e.g. `ui/TODO.md`, `api/TODO.md`). These cache deferred work, known debt, and follow-ups. Use them to understand intended scope and avoid duplicating deferred decisions.

Surface completed items to the user so they can check them off. Never silently delete or mark items complete yourself.

---

## Writing

Add items to `TODO.md` when you identify work that should be tracked but is out of scope for the current task — deferred decisions, follow-up fixes, flagged patterns, audit findings.

### Item format

Each item uses three labeled lines under a `###` heading:

```markdown
### Short imperative title
**Problem:** What is wrong or missing and why it matters.
**Action:** What should be done. Be specific enough that someone can act without re-researching.
**Decision:** (optional) Open question or choice that must be resolved before acting.
```

Use `**Problem:**` + `**Action:**` for clear-cut items. Add `**Decision:**` when a real choice exists. Omit labels that add no information.

For inline list items (e.g. quick follow-ups surfaced mid-task), use a plain bullet:

```markdown
- **[M]** Relocate `/services/repair` routes — repair booking belongs in reservations-api, not catalog.
```

**Never use checkboxes (`- [ ]`).** TODO.md is not a task tracker — items are never "checked off" in the file. Remove items only when the user explicitly asks to clear them.

### Section headers

Group items under `##` section headers by area (e.g. `## Standards`, `## Hooks`, `## Agent system`).

When adding items from an audit, use `## <Area> — gaps from audit`. **Never include a date** in the section header or introductory text. Items may sit through many audits before being addressed; a date becomes misleading noise.

```markdown
## Agent system — gaps from audit   ✓
## Agent system — gaps from 2026-05-18 audit   ✗
```

### Preamble

If a file-level preamble line is needed, write `Remaining items from audits.` — not `Remaining items from the YYYY-MM-DD audit.`

---

## What belongs in TODO.md

| Add | Don't add |
|-----|-----------|
| Out-of-scope fixes spotted during a task | Work already captured in a ticket or PR |
| Deferred architectural decisions | Implementation details handled inline |
| Audit findings | Passing observations with no action |
| Open choices that block future work | Things that resolved themselves |

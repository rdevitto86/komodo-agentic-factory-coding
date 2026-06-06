# TODO.md Conventions

Standards for reading and writing `TODO.md` files across Komodo projects.

---

## Reading

Before starting any significant task, check `TODO.md` at the project root and in relevant subdirectories (e.g. `ui/TODO.md`, `api/TODO.md`). These cache deferred work, known debt, and follow-ups. Use them to understand intended scope and avoid duplicating deferred decisions.

When your task resolves an item listed here, remove it from the file — see *Removing completed items* below.

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

**Never use checkboxes (`- [ ]`).** TODO.md is not a task tracker — items are never "checked off" in the file. A completed item is removed outright (see *Removing completed items*), never marked done in place.

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

## Removing completed items

When you finish a task that satisfies an item already in a `TODO.md`, remove that item from the file as the last step of the task. Do not leave completed items for the user to clear, and do not mark them done in place — the file tracks live work only. This saves the user a manual cleanup pass.

- Remove only items your work genuinely completed. A partially-done item stays, with its `**Action:**` updated to reflect what remains.
- Never remove items unrelated to your task, even if they look stale — flag those to the user instead.
- Note which items you removed in your task summary, so there is a record outside the file.

---

## What belongs in TODO.md

| Add | Don't add |
|-----|-----------|
| Out-of-scope fixes spotted during a task | Work already captured in a ticket or PR |
| Deferred architectural decisions | Implementation details handled inline |
| Audit findings | Passing observations with no action |
| Open choices that block future work | Things that resolved themselves |

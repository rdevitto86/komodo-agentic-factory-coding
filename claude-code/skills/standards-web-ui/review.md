# Rendered-surface security review

The procedure for reviewing the exploits a browser interface itself enables — XSS, clickjacking, dark patterns. `SKILL.md`'s Security half states the rules this file hunts for violations of. **Scope enumeration, the severity bar, and the report shape are shared with the backend review** — see `standards-api-security/review.md` §§1–2 and Output; this file adds only the rendered-surface vector that skill's §3 (Injection) explicitly excludes. For a native mobile or desktop surface, use `standards-mobile-ui` or `standards-desktop-ui` instead.

## XSS

**XSS is an output-encoding bug, not an input-sanitisation bug.** Encode at the point of rendering, per context — HTML body, attribute, URL, JS, and CSS each need a different encoder. Input filtering is a second layer, never the first.

- **Audit every escape hatch** — `innerHTML`, `dangerouslySetInnerHTML`, `v-html`, `{@html}`, `document.write`, template `safe`/`raw` markers, and any string concatenated into a `<script>` or `href`. Each one needs a sanitiser (DOMPurify-class) or a justification.
- **CSP is the backstop, not the fix.** Require a policy with no `unsafe-inline` and no `unsafe-eval`; a nonce or hash for anything inline. A CSP does not close an XSS finding — it downgrades it.
- **A stored XSS is High**, not Medium — it needs no attacker-controlled link, only a victim viewing already-poisoned content. A reflected XSS needing a crafted link the victim must click is Medium.

## Clickjacking and embeds

Covered directly in `SKILL.md` — framing, overlays, and `postMessage` origin checks. No separate procedure beyond the severity bar shared with the backend review: a functioning clickjacking primitive on a sensitive action (payment, permission grant) is High; a missing header with no demonstrated overlay is Low.

## Dark patterns

Flag on the same footing as a security bug — the exploit is the user's own judgment, not a technical boundary. Severity follows harm, not novelty: a forced-continuity trap with real financial cost is High; a merely-annoying confirmshaming prompt is Low.

## Output

Use `standards-api-security/review.md`'s report shape. A rendered-surface finding cites the OWASP category the same way a backend one does — XSS files under **Injection** (or **Software and data integrity failures** where relevant); clickjacking and dark patterns file under **Security misconfiguration** absent a more specific category in the current OWASP edition.

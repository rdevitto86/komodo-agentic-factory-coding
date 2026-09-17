# Web UI

Browser-targeted, framework-agnostic. Framework mechanics live in the react, vue, and svelte standards; the server-side baseline lives in api-security.

## Design
- Tailwind utility classes only. No `@apply` outside deeply nested selectors, no inline styles, no `!important`. Mobile-first prefixes, dark mode via `dark:`, `<style>` only when Tailwind cannot express it.
- No business logic in components; use a service or store layer. Derive state rather than syncing it. Effects declare correct dependencies and a cleanup.

## Accessibility (WCAG AA)
- Visible focus ring on every interactive element; never remove `:focus-visible` without a replacement.
- A keyboard handler wherever a click handler exists.
- `alt` on every image; decorative images take `alt=""`.
- ARIA roles and labels where semantic HTML is ambiguous. Contrast 4.5:1 for text, 3:1 for UI elements. Never `tabindex > 0`.
- Reduced-motion media query respected for any non-essential animation.

## Output encoding and XSS
- Encode at the point of rendering, per context. Input filtering is a second layer.
- Every raw-HTML sink (`innerHTML`, `dangerouslySetInnerHTML`, `v-html`, `{@html}`) needs a sanitiser or a written justification.
- CSP with no `unsafe-inline` and no `unsafe-eval`; nonces or hashes for anything inline. Ship HSTS, `X-Content-Type-Options: nosniff`, a referrer policy, and `frame-ancestors`.

## Clickjacking and embeds
- Deny framing by default (`frame-ancestors 'none'`). An intentional embed allowlists explicit origins.
- A transparent overlay over a real control is a defect. Sensitive actions re-confirm inside their own frame. A blocking modal blocks at the input layer, not just visually.
- Every `postMessage` listener validates `event.origin` against an allowlist; every `postMessage` call targets an explicit origin.
- Untrusted `<iframe>` content gets `sandbox` with the minimum token set; `allow-same-origin` plus `allow-scripts` defeats it. Third-party widgets render in their own frame.

## Dark patterns are defects
- No confirmshaming, no roach motel, no forced continuity without notice, no hidden costs at the last step, no pre-checked consent, no double-negative toggles.
- A close control is always present and as prominent as accept on any interstitial or paywall.

## Sensitive inputs
- Never disable paste on a password or OTP field. `autocomplete` attributes are accurate (`current-password`, `one-time-code`), never defensively disabled.

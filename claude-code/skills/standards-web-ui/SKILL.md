---
name: standards-web-ui
description: Browser UI standards — Design (Tailwind, component rules, WCAG AA) and Security (output encoding and XSS, CSP and browser headers, clickjacking, postMessage and embeds, dark patterns, sensitive inputs). Web targets only; native mobile is standards-mobile-ui, desktop shells are standards-desktop-ui. Load before writing a component, stylesheet, template, iframe, or postMessage handler.
user-invocable: false
paths: "**/*.css, **/*.svelte, **/*.vue, **/*.tsx, **/*.jsx, **/*.html"
---

# Web UI

Framework-agnostic, browser-targeted. Framework mechanics live in the framework skills (`standards-react`, `standards-svelte`, `standards-vue`); document formatting lives in `config-accessibility`; the server-side OWASP baseline lives in `standards-api-security`.

## Platform routing

This skill owns the browser extensions — stylesheets, HTML templates, and the component extensions (`.svelte`, `.vue`, `.tsx`, `.jsx`). `.tsx`/`.jsx` are unambiguously web here because `standards-mobile-ui` covers native targets only and excludes React Native.

- **A desktop shell rendering its UI in web technology** still loads this skill for its renderer files — they match the browser extensions. `standards-desktop-ui` loads alongside it, off the shell's own config surface, and owns the shell boundary (context isolation, IPC, protocol handlers, update signing).
- **No glob in this skill matches a native mobile or desktop config file**, and neither of those skills claims a browser extension.

# Design

## Tailwind

The major version comes from the repo's package manifest — read it before using a version-gated feature.

- **Utility classes only.** No `@apply` except for deeply nested selector patterns, no inline styles, no `!important`.
- **Mobile-first responsive** — `sm:`, `md:`, `lg:` prefixes.
- **Dark mode via `dark:` prefixes.** Never toggle classes manually.
- **Animations**: the repo's animation utility classes, or a timeline library for complex sequences. Do not use `transition-*` for entrance animations on initial render.
- **`<style>` blocks only when Tailwind genuinely cannot express it.**

## Component rules — every framework

Declaration order inside a component file is framework mechanics; each framework skill owns its own. These three hold regardless.

- **No business logic in components.** Use a service or store layer.
- **Derive state rather than syncing it.** Pass minimal props.
- **Effects need correct dependencies and a cleanup.**

## WCAG — required on every component

- **Visible focus ring** on every interactive element. Do not remove `:focus-visible` without replacing it.
- **Keyboard handlers wherever a click handler exists.**
- **`alt` on every image.** Decorative images take `alt=""`.
- **ARIA roles and labels** where semantic HTML is ambiguous.
- **WCAG AA contrast minimum** — 4.5:1 for text, 3:1 for UI elements.
- **Never `tabindex > 0`.**

# Security

The attack surface is the rendered surface — a user clicking, trusting, or being shown what they see. The Design half above owns how a component looks and whether it is usable; `standards-api-security` owns the server-side OWASP baseline. This half is the layer between them: the ways an interface itself becomes the exploit.

## Output encoding and XSS

- **XSS is an output bug.** Encode at the point of rendering, per context — HTML body, attribute, URL, JS, and CSS each need a different encoder. Input filtering is a second layer, never the first.
- **Every raw-HTML escape hatch needs a sanitiser or a justification** — `innerHTML`, `dangerouslySetInnerHTML`, `v-html`, `{@html}`, `document.write`, and any template `raw`/`safe` marker.
- **Require a CSP with no `unsafe-inline` and no `unsafe-eval`.** A nonce or hash covers anything genuinely inline. CSP is the backstop that downgrades an XSS, never the fix.
- **Ship the browser-facing header set** — HSTS, `X-Content-Type-Options: nosniff`, a referrer policy, and CSP, alongside the `frame-ancestors` directive below.

## Clickjacking

- **Deny framing by default.** Ship `X-Frame-Options: DENY` or `Content-Security-Policy: frame-ancestors 'none'` on every response that isn't meant to be embedded. The CSP's `script-src` covers script sources; `frame-ancestors` is the UI-specific directive this skill adds.
- **An intentional embed allowlists explicit origins** — never a wildcard, never `*`.
- **A transparent or near-transparent overlay over a real control is a defect**, not a styling choice — opacity tricks and 1px iframes are the clickjacking primitive, not a legitimate UI pattern.
- **Sensitive actions re-confirm inside their own frame.** A payment button, a permission grant, or a destructive action never trusts that the click it received was aimed by the user rather than redirected through a stacked layer.
- **A modal or bottom sheet blocking interaction must actually block it** at the input layer, not just visually — a `pointer-events: none` ancestor or a z-index mistake that lets a click pass through is the same class of bug, self-inflicted.

## Embeds and cross-frame messaging

- **Every `postMessage` listener validates `event.origin`** against an explicit allowlist before trusting `event.data`. A missing origin check lets any frame on the page — including one injected by a compromised ad or a third-party widget — talk to your handler.
- **Every `postMessage` call targets an explicit origin**, never `*`, once the destination is known.
- **`<iframe>` embedding untrusted content gets `sandbox`**, scoped to the minimum token set it needs (`allow-scripts` alone is not sandboxing). Add `allow-same-origin` only when the embedded content is truly trusted — combined with `allow-scripts` it defeats the sandbox entirely.
- **Third-party widgets and ads render in their own frame**, never inline in the same document as authenticated content or a payment form.

## Dark patterns

A dark pattern is a UI decision that manipulates rather than informs. These are defects to flag in review, on the same footing as a security bug — the exploit is the user's own judgment.

- **No confirmshaming.** A decline option is worded with the same neutrality as the accept option.
- **No roach motel.** Whatever a flow lets a user do in N steps (subscribe, opt in, grant a permission) it lets them undo in N steps, not more.
- **No forced continuity.** A trial-to-paid conversion notifies before the charge, with enough lead time to cancel, and cancellation is reachable from the same surface as the original signup.
- **No hidden costs revealed only at the final step.** Price, fees, and recurring charges show before the commitment screen, not after.
- **No pre-checked consent.** A checkbox granting data sharing, marketing contact, or a non-essential permission defaults unchecked.
- **No trick questions or double negatives in a consent toggle**, regardless of legal sign-off.
- **A close/dismiss control is always present and equally prominent** to the accept control on any interstitial, modal, or paywall — never smaller, greyed out, or hidden until a delay elapses.

## Sensitive-input protection

- **Never disable paste on a password or OTP field.** Blocking paste degrades password-manager use and pushes users toward weaker, memorable passwords — a net security loss dressed up as hardening.
- **Autocomplete attributes are accurate**, not defensively disabled — `autocomplete="current-password"` / `"one-time-code"` let the platform's own credential manager and autofill work instead of forcing manual entry into a phishing-cloneable form.

## Quick-reference

| Concern | Control |
|---|---|
| XSS | Contextual output encoding at render, sanitised raw-HTML escape hatches, CSP with no `unsafe-inline`/`unsafe-eval` |
| Headers | HSTS, `nosniff`, referrer policy, CSP, `frame-ancestors` |
| Framing | `frame-ancestors` (CSP) or `X-Frame-Options: DENY`, explicit allowlist for intentional embeds |
| Overlay | No transparent layer over a real control; a blocking modal blocks at the input layer |
| Cross-frame messaging | `event.origin` checked on receipt, explicit target origin on send |
| Untrusted embed | `sandbox` scoped to minimum tokens, `allow-same-origin` + `allow-scripts` never combined on untrusted content |
| Dark pattern | Symmetric accept/decline, no pre-checked consent, cost and cancellation visible before commitment |
| Credential field | Paste allowed, correct `autocomplete` |
| Accessibility | Visible focus, keyboard parity, `alt`, AA contrast |

## Reference material

`review.md` — the rendered-surface security review procedure.

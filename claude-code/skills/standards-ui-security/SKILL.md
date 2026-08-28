---
name: standards-ui-security
description: Rendered-surface security — output encoding and XSS, CSP and browser response headers, clickjacking, tapjacking, frame/embed hardening, postMessage origin checks, and dark patterns. Distinct from standards-ui-design's Tailwind/WCAG scope and standards-api-security's server-side OWASP baseline. Load before writing an iframe, an embed, a postMessage handler, a consent/permission flow, a raw-HTML escape hatch, or any UI that overlays, renders, or intercepts input.
user-invocable: false
paths: "**/*.svelte, **/*.vue, **/*.tsx, **/*.jsx, **/*.html"
---

# UI/UX security

The attack surface is the rendered surface — a user clicking, tapping, trusting, or being shown what they see. `standards-ui-design` owns how a component looks and whether it's usable; `standards-api-security` owns the server-side OWASP baseline (injection, authn, secrets, crypto). This skill is the layer between them: the ways an interface itself becomes the exploit.

## Output encoding and XSS

- **XSS is an output bug.** Encode at the point of rendering, per context — HTML body, attribute, URL, JS, and CSS each need a different encoder. Input filtering is a second layer, never the first.
- **Every raw-HTML escape hatch needs a sanitiser or a justification** — `innerHTML`, `dangerouslySetInnerHTML`, `v-html`, `{@html}`, `document.write`, and any template `raw`/`safe` marker.
- **Require a CSP with no `unsafe-inline` and no `unsafe-eval`.** A nonce or hash covers anything genuinely inline. CSP is the backstop that downgrades an XSS, never the fix.
- **Ship the browser-facing header set** — HSTS, `X-Content-Type-Options: nosniff`, a referrer policy, and CSP, alongside the `frame-ancestors` directive below.

## Clickjacking

- **Deny framing by default.** Ship `X-Frame-Options: DENY` or `Content-Security-Policy: frame-ancestors 'none'` on every response that isn't meant to be embedded. The CSP's `script-src` covers script sources; `frame-ancestors` is the UI-specific directive this skill adds.
- **An intentional embed allowlists explicit origins** — `frame-ancestors https://trusted.example` — never a wildcard, never `*`.
- **A transparent or near-transparent overlay over a real control is a defect**, not a styling choice — opacity tricks and 1px iframes are the clickjacking primitive, not a legitimate UI pattern.
- **Sensitive actions re-confirm inside their own frame.** A payment button, a permission grant, or a destructive action never trusts that the click it received was aimed by the user rather than redirected through a stacked layer.

## Tapjacking and overlay attacks

- **A system permission prompt (camera, mic, location, notifications) must never render behind or beneath an app-drawn overlay.** On mobile, `filterTouchesWhenObscured` (Android) or the platform equivalent is required on any view that can be covered by an untrusted overlay.
- **Never suppress the platform's own tap-through protection** to make a custom overlay "feel" more responsive — that protection exists specifically to stop a hidden view from stealing the tap.
- **A modal or bottom sheet blocking interaction must actually block it** at the input layer, not just visually — a `pointer-events: none` ancestor or a z-index mistake that lets a click pass through to what's underneath is the same class of bug as tapjacking, just self-inflicted.

## Embeds and cross-frame messaging

- **Every `postMessage` listener validates `event.origin`** against an explicit allowlist before trusting `event.data`. A missing origin check lets any frame on the page — including one injected by a compromised ad or a third-party widget — talk to your handler.
- **Every `postMessage` call targets an explicit origin**, never `*`, once the destination is known.
- **`<iframe>` embedding untrusted content gets `sandbox`**, scoped to the minimum token set it needs (`allow-scripts` alone is not sandboxing). Add `allow-same-origin` only when the embedded content is truly trusted — combined with `allow-scripts` it defeats the sandbox entirely.
- **Third-party widgets and ads render in their own frame**, never inline in the same document as authenticated content or a payment form.

## Dark patterns

A dark pattern is a UI decision that manipulates rather than informs. These are defects to flag in review, on the same footing as a security bug — the exploit is the user's own judgment.

- **No confirmshaming.** A decline option is worded the same neutrality as the accept option — never "No thanks, I don't want to save money."
- **No roach motel.** Whatever a flow lets a user do in N steps (subscribe, opt in, grant a permission) it lets them undo in N steps, not more.
- **No forced continuity.** A trial-to-paid conversion notifies before the charge, with enough lead time to cancel, and cancellation is reachable from the same surface as the original signup.
- **No hidden costs revealed only at the final step.** Price, fees, and recurring charges show before the commitment screen, not after.
- **No pre-checked consent.** A checkbox granting data sharing, marketing contact, or a non-essential permission defaults unchecked.
- **No trick questions or double negatives in a consent toggle.** "Uncheck to not not receive emails" is a defect regardless of legal sign-off.
- **A close/dismiss control is always present and equally prominent** to the accept control on any interstitial, modal, or paywall — never smaller, greyed out, or hidden until a delay elapses.

## Sensitive-input protection

- **Never disable paste on a password or OTP field.** Blocking paste degrades password-manager use and pushes users toward weaker, memorable passwords — a net security loss dressed up as a hardening measure.
- **Autocomplete attributes are accurate**, not defensively disabled — `autocomplete="current-password"` / `"one-time-code"` let the platform's own credential manager and SMS autofill work instead of forcing manual entry into a phishing-cloneable form.
- **Screenshot/screen-recording protection** (platform-specific secure-flag equivalents) applies to a screen rendering a credential, a payment card, or a recovery code — not applied blanket across the whole app, which just breaks legitimate support screenshots elsewhere.

## Quick-reference

| Concern | Control |
|---|---|
| XSS | Contextual output encoding at render, sanitised raw-HTML escape hatches, CSP with no `unsafe-inline`/`unsafe-eval` |
| Headers | HSTS, `nosniff`, referrer policy, CSP, `frame-ancestors` |
| Framing | `frame-ancestors` (CSP) or `X-Frame-Options: DENY`, explicit allowlist for intentional embeds |
| Tapjacking | Platform tap-through protection enabled on any view coverable by an overlay |
| Cross-frame messaging | `event.origin` checked on receipt, explicit target origin on send |
| Untrusted embed | `sandbox` scoped to minimum tokens, `allow-same-origin` + `allow-scripts` never combined on untrusted content |
| Dark pattern | Symmetric accept/decline, no pre-checked consent, cost and cancellation visible before commitment |
| Credential field | Paste allowed, correct `autocomplete`, no blanket screenshot blocking |

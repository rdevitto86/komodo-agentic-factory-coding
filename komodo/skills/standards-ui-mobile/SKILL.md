---
name: standards-ui-mobile
description: Mobile UI: navigation, touch targets, lifecycle, permissions, and platform conventions.
globs: ["**/*.kt", "**/*.m", "**/*.mm", "**/*.plist", "**/*.storyboard", "**/*.swift", "**/*.xib", "**/AndroidManifest.xml", "**/Info.plist"]
---

# Mobile UI

Native mobile. Language mechanics live in the language standards; this owns what the user touches and what an attacker can put in between.

## Scope and platform routing

- **Security covers every native manifest**, React Native/Flutter included: permissions, exported components/intent filters, deep links/URL schemes, bundle IDs, backgrounding, and screen-capture flags are native-shell concerns regardless of renderer.
- **Design is native-UI only**; touch targets, safe areas, gestures, HIG divergence, and state rules don't apply to a React Native/Flutter UI layer; `.tsx`/`.jsx` are ui-web, `.dart` matches nothing here.
- **A desktop target's `Info.plist` is this standard's false positive**; use ui-desktop. An iOS plist carries `UI*` device keys; macOS carries `LSMinimumSystemVersion`/`NSPrincipalClass`, none of them.
- **A native app embedding a web view** loads ui-web for the document; the host/document channel is this one's.

# Design

## Touch targets and layout

- **Every interactive element meets the platform's minimum touch-target size** — read current guidance, not a hard-coded number.
- **Hit area is independent of visual size** — a small glyph gets an expanded hit region, never shrunken.
- **Adjacent targets are separated** enough that a thumb cannot straddle two; keep destructive actions away from frequent ones.
- **Lay out against safe-area insets, never fixed margins**; notches, islands, and cutouts vary by device; system-reported insets are the only correct source.
- **Content respects both the safe area and the keyboard inset**; a field the keyboard covers is a defect, not the user's problem.
- **Honour the system text-size and display-scale settings**; a layout breaking at the largest accessibility size is broken, not a user error.

## Gestures

- **Never override a system gesture**; back/edge swipe, home, app switcher. One starting in a reserved edge region loses, as an unresponsive app.

## State and interruption

- **Offline is a first-class state, not an error dialog** — show what's cached, mark it stale, say what will sync.
- **Every mutation made offline has a defined reconciliation path**: queued, retried, surfaced on failure. Silent loss of an edit is a data-loss bug.
- **Distinguish loading, empty, error, and offline** — one shared spinner for all four is a defect.
- **State survives process death, backgrounding, rotation, and configuration change** — a blank form on restore is data loss.
- **Interruptions (call, alarm, low-memory warning) never corrupt in-flight input** — save drafts as typed.

# Security

## Overlay and tapjacking

- **A system permission prompt (camera, mic, location) must never render behind or beneath an app-drawn overlay.**
- **Enable the platform's obscured-touch filtering on any view an untrusted overlay can cover.** Never suppress it for responsiveness; it stops a hidden view stealing the tap.
- **A modal blocking interaction blocks it at the input layer**, not just visually; a passthrough touch is self-inflicted tapjacking.

## Screenshots and backgrounding

- **A screen rendering a credential or payment instrument marks itself secure** via the platform's screen-capture flag or secure-surface equivalent.
- **Obscure the app snapshot taken when backgrounding** — the recents thumbnail is written to disk; blur it before the app resigns active.
- **Do not apply capture blocking blanket app-wide**; it breaks legitimate support screenshots and accessibility tooling.

## Deep links and scheme hijacking

- **A custom URL scheme is claimable by any other app**; treat anything arriving over one as untrusted input, never proof of identity.
- **Prefer the platform's verified/associated-domain link mechanism** over a bare custom scheme for anything authenticating or carrying a token; domain ownership is the only part the OS verifies.
- **Never carry a credential, session token, or authorisation code in a link the OS may hand to another app**, or in a way surviving in a log or browser history.
- **Validate and authorise every deep-link destination** — a link must not reach a screen the user could not otherwise reach, or change state without confirmation.
- **Every exported component, intent filter, or activity in the manifest is an attack surface**; export nothing unneeded, and authorise what remains.

## WebView channels

- **A native channel exposed to a web view is a remote-code surface**; expose the narrowest method set, never eval, reflection, or file-access.
- **Restrict which origins the web view may load**, and refuse navigation outside that allowlist.
- **Disable file-system and universal-file access in the web view** unless a feature needs it, never alongside remote content.
- **Never inject a credential or token into web content** the web view's origin does not already own.
- **Every message crossing the channel is validated on the native side** for type, shape, and authorisation, as an API boundary would.

## Local data, pasteboard, and keyboard

- **Secrets go in the platform keystore/secure enclave**, never preferences, a plain file, or an unencrypted database.
- **Mark sensitive fields excluded from the clipboard's shared history**, and clear anything the app copies once consumed.
- **Disable keyboard caching, autocorrect, and predictive text on sensitive fields**; the learned-word store outlives the session.
- **Exclude caches, snapshots, and sensitive stores from cloud/device backup.**

## Biometrics and prompt spoofing

- **A biometric prompt is an authentication gate, not a boolean an app draws.** Use the platform's prompt API; a lookalike screen authenticates nothing.
- **Bind the result to a key operation, not a return value**; released only on successful biometry, so a hooked boolean fails.
- **Invalidate biometric-bound keys when enrolment changes**; an attacker enrolling their own biometric must not inherit access.
- **Always offer a non-biometric fallback**, rate-limited like biometry.

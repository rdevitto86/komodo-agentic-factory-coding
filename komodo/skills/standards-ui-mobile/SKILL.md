---
name: standards-ui-mobile
description: Mobile UI: navigation, touch targets, lifecycle, permissions, and platform conventions.
globs: ["**/*.kt", "**/*.m", "**/*.mm", "**/*.plist", "**/*.storyboard", "**/*.swift", "**/*.xib", "**/AndroidManifest.xml", "**/Info.plist"]
---

# Mobile UI

Native mobile. Language mechanics live in the language standards; this one owns what the user touches and what an attacker can put in between.

## Scope and platform routing

**The two halves have different scopes.** Security applies to any native app manifest; Design applies only to a native UI layer.

- **Security covers every native manifest**, React Native and Flutter included: permissions, exported components and intent filters, deep links and URL schemes, bundle IDs, backgrounding and screen-capture flags are native-shell concerns whatever renders above them.
- **Design is native-UI only.** Touch targets, safe areas, gestures, HIG divergence, and the state rules do not apply to a React Native or Flutter UI layer; `.tsx`/`.jsx` belong to ui-web and `.dart` matches nothing here.
- **A desktop target's `Info.plist` is this standard's false positive** — use ui-desktop. An iOS plist carries the `UI*` device keys; a macOS one carries `LSMinimumSystemVersion` and `NSPrincipalClass` and none of them.
- **A native app embedding a web view** loads ui-web for the embedded document; the channel between host and document is this standard's.

# Design

## Touch targets and layout

- **Every interactive element meets the platform's own minimum touch-target size** — read the target platform's current human-interface guidance for the number rather than hard-coding one; the two platforms do not agree, and both have moved.
- **Hit area is independent of visual size.** A small glyph gets an expanded hit region, never a shrunken target.
- **Adjacent targets are separated** enough that a thumb cannot straddle two — a destructive action is never adjacent to the control a user reaches for most.
- **Lay out against safe-area insets, never fixed margins.** Notches, dynamic islands, rounded corners, home indicators, gesture bars, and camera cutouts all vary by device and by orientation; the insets the system reports are the only correct source.
- **Content respects both the safe area and the keyboard inset.** A field the keyboard covers is a defect, not a scroll problem for the user to solve.
- **Honour the system text-size and display-scale settings.** A layout that breaks at the largest accessibility text size is a broken layout, not a user error.

## Gestures

- **Never override a system gesture** — back/edge swipe, home, notification pull, app switcher. A custom gesture that starts in a system-reserved edge region loses, and the conflict shows up as an unresponsive app.

## State and interruption

- **Offline is a first-class state, not an error dialog.** Show what is cached, mark it as stale, and say what will sync — never an empty screen with a spinner.
- **Every mutation made offline has a defined reconciliation path**: queued, retried, and surfaced when it ultimately fails. Silent loss of a user's offline edit is a data-loss bug.
- **Distinguish loading, empty, error, and offline.** One shared spinner for all four is a design defect.
- **State survives process death, backgrounding, rotation, and configuration change.** The system can kill a backgrounded app at any time; restoring to a blank form is the same failure as losing the data.
- **Interruptions (call, alarm, permission prompt, low-memory warning) never corrupt in-flight input.** Save drafts as they are typed.

# Security

## Overlay and tapjacking

- **A system permission prompt (camera, mic, location, notifications) must never render behind or beneath an app-drawn overlay.**
- **Enable the platform's obscured-touch filtering on any view an untrusted overlay can cover** — a consent screen, a permission prompt, a payment confirmation. Never suppress that protection to make a custom overlay feel more responsive; it exists specifically to stop a hidden view from stealing the tap.
- **A modal blocking interaction blocks it at the input layer**, not just visually — a view that lets a touch pass through to what is underneath is self-inflicted tapjacking.

## Screenshots and backgrounding

- **A screen rendering a credential, a payment instrument, a recovery code, or another user's private content marks itself secure** using the platform's own screen-capture flag or secure-surface equivalent.
- **Obscure the app snapshot the system takes when backgrounding.** The recents/app-switcher thumbnail is written to disk and is readable in ways the live screen is not — blank or blur the sensitive view before the app resigns active.
- **Do not apply capture blocking blanket across the whole app** — it breaks legitimate support screenshots and accessibility tooling everywhere else for no gain.

## Deep links and scheme hijacking

- **A custom URL scheme is claimable by any other app on the device.** Treat anything arriving over one as untrusted input from an unknown sender, and never as proof of the sender's identity.
- **Prefer the platform's verified/associated-domain link mechanism** over a bare custom scheme for anything that authenticates, authorises, or carries a token — domain ownership is the only part of the chain the OS actually verifies.
- **Never carry a credential, session token, or authorisation code in a link the OS may hand to another app**, and never in a way that survives in a log or a browser history entry.
- **Validate and authorise every deep-link destination.** A link must not reach a screen or an action the user could not otherwise reach, and must not perform a state change without confirmation.
- **Every exported component, intent filter, or activity declared in the manifest is an attack surface** — export nothing that does not need to be reachable from outside the app, and authorise what remains.

## WebView channels

- **A native channel exposed to a web view is a remote-code surface.** Expose the narrowest possible method set, never a generic eval, reflection, or file-access primitive.
- **Restrict which origins the web view may load**, and refuse navigation outside that allowlist. A channel reachable from arbitrary remote content is a full compromise of the native side.
- **Disable file-system and universal-file access in the web view** unless a specific feature requires it, and never alongside remote content.
- **Never inject a credential or token into web content** that the web view's own origin does not already own.
- **Every message crossing the channel is validated on the native side** — type, shape, and authorisation — exactly as an API boundary would be.

## Local data, pasteboard, and keyboard

- **Secrets go in the platform keystore/secure enclave**, never in app preferences, a plain file, or a local database without platform encryption.
- **Mark sensitive fields as excluded from the system clipboard's shared/universal history**, and clear anything the app itself copies for the user once it has been consumed.
- **Disable keyboard caching, autocorrect, and predictive text on credential and sensitive fields** — the learned-word store is a plaintext leak that outlives the app session.
- **Exclude caches, snapshots, and sensitive stores from cloud/device backup** where the platform's backup would otherwise carry them off-device.

## Biometrics and prompt spoofing

- **A biometric prompt is an authentication gate, not a boolean an app draws.** Use the platform's own prompt API; a custom lookalike screen is a spoofing primitive and authenticates nothing.
- **Bind the result to a key operation, not a return value.** Gate the action on a key the platform releases only on successful biometry, so a patched or hooked boolean is not enough to pass.
- **Invalidate biometric-bound keys when enrolment changes** — an attacker who enrols their own biometric must not inherit the previous user's access.
- **Always offer a non-biometric fallback**, and rate-limit it the same way the platform rate-limits biometry.

---
name: standards-ui-desktop
description: Desktop UI: windows, menus, keyboard, packaging, and platform conventions.
globs: ["**/*.appxmanifest", "**/*.desktop", "**/electron-builder.yml", "**/tauri.conf.json"]
---

# Desktop UI

A windowed app the user runs locally, drawn natively or by an embedded browser engine.

## Keyboard and menus

- **The keyboard is a first-class input, not an accessibility afterthought.** Every mouse action is reachable by keyboard, and a focus ring is always visible.
- **Use the platform's own modifier and shortcut conventions** — the primary modifier key, edit/file/window accelerators, and closing/quitting behaviour differ by platform. Read that platform's guidance rather than shipping one map everywhere.
- **Never reassign a reserved system or platform-standard shortcut** to an app action.
- **A menu bar carries the platform's expected menus in order**, with application-level items where that platform puts them, not one custom menu holding everything.
- **Every menu item that has a shortcut displays it**, and a disabled item stays visible and disabled, not disappears.
- **Tab order follows visual order**, and a modal traps focus and restores it on dismiss.

## Windows and multi-window

- **Window size, position, and maximised state persist** per window across launches, restoring onto a display that still exists — off-screen is unreachable.
- **Handle multi-monitor properly**: differing DPI scale per display, moving displays mid-session, a display disconnecting while a window is on it.
- **Multiple windows of the same document share one source of truth.** Two windows on the same data never diverge, and closing one never discards the other's unsaved work.
- **Quitting is distinct from closing the last window** — each platform expects a different one; unsaved work prompts on either path.
- **Full-screen, tiled, and snapped layouts are supported states**, not edge cases — layout survives them without clipping or unreachable controls.

## Density and pointer affordances

- **Desktop is a dense, pointer-driven surface.** Do not transplant a touch-sized mobile layout onto it; a pointer is precise, and oversized targets cost information density.
- **Offer a compact mode where lists are long**, and respect OS density and text-scale settings, not a hard-coded scale.
- **Hover states exist and are used** — tooltips on icon-only controls, hover on rows — never the only way to discover an action.
- **Right-click opens a context menu everywhere a contextual action exists**, containing actions the main menus also offer, never ones found nowhere else.
- **Support the platform's expected selection model** — click, shift-click range, modifier-click toggle, drag-select — anywhere multi-selection applies.
- **Cursor shape communicates affordance** (resize, text, grab, busy) and window title reflects document and dirty state per platform convention.

# Security

## Renderer isolation

- **Untrusted or remote content never runs with access to the host runtime.** Context isolation on, host/Node integration off, remote-module access off — every window and child frame.
- **The preload script exposes a narrow, explicit API surface** — named operations with validated arguments, never `require`, a filesystem handle, a shell executor, or eval.
- **Ship a CSP in the renderer**, and refuse navigation and new-window requests outside an allowlist. Route external links to the system browser, not the app.
- **Load local content over the shell's own restricted scheme**, not a remote origin, unless the app is a thin client by design.

## Protocol handlers and deep links

- **Registering a custom URL scheme makes the app remotely invocable by any web page.** Treat every argument arriving that way as untrusted input from an unknown sender.
- **Authorise the destination.** A protocol invocation must not reach a privileged action, file path, or state change without the authorisation a direct user action needs.
- **Never pass protocol-supplied data into a shell command, file path, or loader** without validation — argument injection through a handler is a full local compromise.
- **Register the narrowest scheme set the app needs**, and handle a second instance's invocation explicitly rather than leave it undefined.

## Local IPC surface

- **The IPC channel between UI and privileged layers is a trust boundary**, like a network API. Validate type, shape, range, and authorisation on the privileged side of every message.
- **Never expose a generic passthrough** — a "run this command" or "invoke this method by name" handler defeats every other control.
- **A local socket, named pipe, or HTTP listener the app opens is reachable by every other process, and a listener by any web page visited.** Bind to loopback at minimum, authenticate the peer, and check origin on anything HTTP-based.
- **Scope filesystem access to the paths a feature needs**, resolving and confirming each path stays inside the permitted root before use.

## Updates and distribution

- **Every update is signed, and the signature is verified before it is applied.** An unsigned or unverified update channel is remote code execution with a progress bar.
- **The update channel is transport-secured with certificate validation intact**, and downgrade to an older signed version is refused.
- **Ship the platform's own code signing and notarisation**, keeping signing keys out of the repo and build logs.
- **Pin or verify the update metadata source** the same way the payload is verified — a trusted binary from an attacker-chosen manifest is still an attack.

## Drag-drop and file trust

- **A dropped file or pasted path is untrusted input.** Validate type by content, not extension, and never execute, auto-open, or hand it to a shell.
- **Never let a drop navigate the app window.** A shell treating a dropped file as navigation loads arbitrary content into a privileged context.
- **Resolve symlinks and reject path traversal** before touching a dropped or pasted path, and confine writes to the permitted root.
- **Data dragged out of the app carries only what the user intends** — no credential, token, or hidden path in its flavours.

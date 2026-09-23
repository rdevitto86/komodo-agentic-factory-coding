---
name: standards-ui-desktop
description: Desktop UI: windows, menus, keyboard, packaging, and platform conventions.
globs: ["**/*.appxmanifest", "**/*.desktop", "**/electron-builder.yml", "**/tauri.conf.json"]
---

# Desktop UI

Desktop applications — a windowed app the user runs locally, whether drawn natively or by an embedded browser engine.

## Keyboard and menus

- **The keyboard is a first-class input, not an accessibility afterthought.** Every action reachable by mouse is reachable by keyboard, and a focus ring is always visible.
- **Use the platform's own modifier and shortcut conventions** — the primary modifier key, the standard edit/file/window accelerators, and the standard closing and quitting behaviour differ between platforms. Read the target platform's current interface guidance rather than shipping one platform's map everywhere.
- **Never reassign a reserved system or platform-standard shortcut** to an app-specific action.
- **A menu bar carries the platform's expected menus in the expected order**, with the platform's application-level items where that platform puts them — not a single custom menu holding everything.
- **Every menu item that has a shortcut displays it**, and a disabled item stays visible and disabled rather than disappearing.
- **Tab order follows visual order**, and a modal traps focus and restores it on dismiss.

## Windows and multi-window

- **Window size, position, and maximised state persist** per window across launches, and restore onto a display that still exists — a window restored off-screen is unreachable.
- **Handle multi-monitor properly**: differing DPI scale factors per display, moving between displays mid-session, and a display disconnecting while a window is on it.
- **Multiple windows of the same document share one source of truth.** Two windows on the same data never diverge, and closing one never discards the other's unsaved work.
- **Quitting is distinct from closing the last window**, and each platform has its own expectation for which is which. Unsaved work prompts on either path.
- **Full-screen, tiled, and snapped layouts are supported states**, not edge cases — the layout must survive them without clipping or unreachable controls.

## Density and pointer affordances

- **Desktop is a dense, pointer-driven surface.** Do not transplant a touch-sized mobile layout onto it; a pointer is precise, and screen area spent on oversized targets is area not spent on information.
- **Offer a compact mode where lists are long**, and respect the OS density and text-scale settings rather than a hard-coded scale.
- **Hover states exist and are used** — tooltips on icon-only controls, hover affordances on interactive rows — while never being the only way to discover an action.
- **Right-click opens a context menu everywhere a contextual action exists**, containing the same actions the main menus offer, never actions available nowhere else.
- **Support the platform's expected selection model** — click, shift-click range, modifier-click toggle, drag-select — anywhere a list allows multi-selection.
- **Cursor shape communicates affordance** (resize, text, grab, busy) and the window title reflects document and dirty state per the platform's convention.

# Security

## Renderer isolation

- **Untrusted or remote content never runs with access to the host runtime.** Context isolation on, host/Node integration off, remote-module access off — in every window and every child frame, including ones created by a link.
- **The preload script exposes a narrow, explicit API surface** — named operations with validated arguments, never the runtime's `require`, a filesystem handle, a shell executor, or an eval primitive.
- **Ship a CSP in the renderer**, and refuse navigation and new-window requests to origins outside an explicit allowlist. Route an external link to the system browser rather than opening it inside the app.
- **Load local application content over the shell's own restricted scheme**, not from a live remote origin, wherever the app is not a thin client by design.

## Protocol handlers and deep links

- **Registering a custom URL scheme makes the app remotely invocable by any web page.** Treat every argument arriving that way as untrusted input from an unknown sender.
- **Authorise the destination.** A protocol invocation must not reach a privileged action, a file path, or a state change without the same authorisation and confirmation a direct user action would need.
- **Never pass protocol-supplied data into a shell command, a file path, or a loader** without validation — argument injection through a registered handler is a full local compromise.
- **Register the narrowest scheme set the app actually needs**, and handle a second instance's invocation explicitly rather than leaving duplicate-instance behaviour undefined.

## Local IPC surface

- **The IPC channel between the UI layer and the privileged layer is a trust boundary**, identical in kind to a network API. Validate type, shape, range, and authorisation on the privileged side of every message.
- **Never expose a generic passthrough** — a "run this command", "read this path", or "invoke this method by name" handler defeats every other control here.
- **A local socket, named pipe, or HTTP listener the app opens is reachable by every other process and, for a listener, by any web page the user visits.** Bind to loopback at minimum, authenticate the peer, and check origin on anything HTTP-based.
- **Scope filesystem access to the paths a feature needs**, resolving and confirming each path stays inside the permitted root before use.

## Updates and distribution

- **Every update is signed, and the signature is verified before the update is applied.** An unsigned or unverified update channel is remote code execution with a progress bar.
- **The update channel is transport-secured with certificate validation intact**, and downgrade to an older signed version is refused.
- **Ship the platform's own code signing and notarisation** for the distributed artifact, and keep signing keys out of the repo and out of build logs.
- **Pin or verify the update metadata source** the same way the payload is verified — a trusted binary fetched from an attacker-chosen manifest is still an attack.

## Drag-drop and file trust

- **A dropped file or pasted path is untrusted input.** Validate type by content, not by extension, and never execute, auto-open, or hand a dropped path to a shell.
- **Never let a drop navigate the app window.** Dropping a file into a shell that treats it as a navigation loads arbitrary local content into a privileged context.
- **Resolve symlinks and reject path traversal** before touching a dropped or pasted path, and confine writes to the permitted root.
- **Data dragged out of the app carries only what the user intends** — no credential, token, or hidden path in the drag payload's alternate flavours.

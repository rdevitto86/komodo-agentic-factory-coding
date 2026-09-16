---
name: standards-ui-desktop
description: Desktop UI standards. Loads on matching paths; the rules live in one file.
paths: "**/tauri.conf.json, **/electron-builder.yml, **/electron-builder.json, **/forge.config.*, **/*.desktop, **/*.appxmanifest"
---

# Desktop UI standard

Read `~/.claude/standards/ui-desktop.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

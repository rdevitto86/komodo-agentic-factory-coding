---
name: standards-azure
description: Azure conventions. Loads on matching paths; the rules live in one file.
paths: "**/*.bicep, **/azure-pipelines*.yml, **/*.azure.*"
---

# Azure standard

Read `~/.claude/standards/azure.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

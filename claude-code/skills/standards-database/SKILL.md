---
name: standards-database
description: Database standards across engines. Loads on matching paths; the rules live in one file.
paths: "**/*.sql, **/migrations/**, **/schema/**"
---

# Database standard

Read `~/.claude/standards/database.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

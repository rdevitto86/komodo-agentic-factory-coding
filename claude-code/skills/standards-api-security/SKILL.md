---
name: standards-api-security
description: Backend/API security. Loads on matching paths; the rules live in one file.
paths: "**/api/**, **/routes/**, **/handlers/**, **/auth/**, **/*auth*, **/middleware/**, **/session*, **/token*, **/crypto/**, **/secrets/**, **/*.sql, **/*.tf"
---

# API and backend security standard

Read `~/.claude/standards/api-security.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

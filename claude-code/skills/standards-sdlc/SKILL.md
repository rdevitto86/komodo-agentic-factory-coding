---
name: standards-sdlc
description: SDLC standard shared across every language. Loads on matching paths; the rules live in one file.
paths: "**/*_test.*, **/*.test.*, **/*.spec.*, **/test/**, **/tests/**, **/__tests__/**, **/e2e/**"
---

# SDLC: test tiers standard

Read `~/.claude/standards/sdlc.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

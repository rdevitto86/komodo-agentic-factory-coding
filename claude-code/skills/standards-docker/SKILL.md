---
name: standards-docker
description: Container image construction. Loads on matching paths; the rules live in one file.
paths: "**/Dockerfile*, **/docker-compose*, **/*.dockerfile"
---

# Docker standard

Read `~/.claude/standards/docker.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

---
name: standards-observability
description: Logs, metrics, and traces. Loads on matching paths; the rules live in one file.
paths: "**/logging/**, **/logger*, **/telemetry/**, **/metrics/**, **/tracing/**, **/otel*"
---

# Observability standard

Read `~/.claude/standards/observability.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

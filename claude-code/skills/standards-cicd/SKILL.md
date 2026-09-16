---
name: standards-cicd
description: Pipeline stages, merge vs release gates, ephemeral CI infra, blue/green, rollback, feature flags. Loads on matching paths; the rules live in one file.
paths: "cicd.yaml, .github/workflows/**, **/Makefile, **/Taskfile*, **/Jenkinsfile, **/.gitlab-ci.yml"
---

# CI/CD standard

Read `~/.claude/standards/cicd.md` before writing or reviewing a matching file, and follow it. It is the same file the harness injects into its workers, so a session and a worker hold the same rules.

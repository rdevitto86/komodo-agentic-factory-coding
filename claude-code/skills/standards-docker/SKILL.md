---
name: standards-docker
description: Container image construction — base image pinning, distroless healthchecks, graceful shutdown timing, build context hygiene, non-root, multi-stage layering. Load before writing or editing any Dockerfile or docker-compose file.
user-invocable: false
paths: "**/Dockerfile*, **/docker-compose*, **/*.dockerfile"
---

# Docker

`standards-cicd` owns which pipeline stage builds and scans the image. This skill owns what's *inside* the image and the compose file that runs it — the layer `standards-cicd` doesn't cover.

## Base image

- **Pin the exact patch tag**, matching the language skill's version floor exactly — `golang:1.26.5`, never `golang:1.26`. A minor-only tag lets the base drift out from under a pinned `go.mod`/`package.json`/`pyproject.toml` floor, and `GOTOOLCHAIN=auto`-style behavior can trigger a network fetch mid-build, breaking `standards-cicd`'s hermetic Stage 2 requirement.
- **Multi-stage, always.** A build stage with the full toolchain; a runtime stage with only what the process needs to run.
- **Distroless or scratch for the runtime stage** wherever the language supports a static or self-contained binary. Non-root by default; never add a user-creation step to work around a distroless image that already runs as nonroot — that's a sign the wrong base was chosen.

## Healthchecks

**A distroless runtime stage has no shell and no `wget`/`curl`.** A `CMD`-form or compose `healthcheck.test` that shells out to either always fails silently — the container reports unhealthy forever despite serving traffic correctly.

- **Ship a self-check subcommand in the same binary** — `<binary> healthcheck` performing an in-process HTTP GET against the app's own `/health` endpoint, exiting 0/1. Point the compose `healthcheck.test` (or Dockerfile `HEALTHCHECK`) at that subcommand, never at an external tool.
- **A non-distroless runtime stage** (one that legitimately needs a shell) may use `wget`/`curl` directly — confirm the tool is actually present in that specific base image first, never assumed from a different image's contents.

## Graceful shutdown

**`stop_grace_period` (compose) or a container orchestrator's termination grace period must be strictly greater than the app's own drain timeout**, never equal or shorter. If the app's graceful-shutdown code drains for 15s after `SIGTERM`, the surrounding grace period needs headroom above that (20s, not 15s) — otherwise the orchestrator sends `SIGKILL` before the app finishes draining in-flight requests, and the graceful-shutdown code was written for nothing. Read the app's own drain timeout from its source before setting this; never guess a round number.

## Build context

- **Every image-building repo carries a `.dockerignore`** alongside its `Dockerfile` — `.git`, docs, IDE state, and any local build artifact excluded. Its absence means `COPY . ./` (or equivalent) pulls in `.git` history and invalidates the build-layer cache on every unrelated file edit.
- **Order `COPY`/dependency-install steps before the source copy**, so a source-only change doesn't invalidate the dependency-install layer.

## Compose

- **`restart` policy is explicit**, never left to the image default.
- **Resource limits are declared**, not open-ended — a runaway container should degrade the host predictably, not consume it.
- **Secrets never appear in `environment:` as literal values.** Reference an env file excluded from git, or the platform's secret store.

## Quick-reference fields

Nothing — this skill has no `AGENTS.md` Quick-reference row of its own. Its facts (healthcheck command, verify target) surface through the language skill's own fields.

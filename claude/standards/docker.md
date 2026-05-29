# Docker Standards

Applies to all containerized services. Reference for Dockerfile authorship, image security, and local development compose setup.

---

## 1. Base images

- Go services: `gcr.io/distroless/base-debian12` — no shell, minimal attack surface.
- Node.js services: `node:20-alpine` — never `node:latest` or bare `node:20`.
- Never use `latest` tags in production Dockerfiles — pin to a digest or minor version.
- Audit third-party base images before adopting.

---

## 2. Multi-stage builds — required

Every production Dockerfile must use multi-stage builds. Never ship compilers, build tools, or intermediate artifacts in the runtime image.

```dockerfile
FROM golang:1.26 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/app ./...

FROM gcr.io/distroless/base-debian12
COPY --from=build /bin/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
```

---

## 3. Non-root user

Run as non-root in every production image.

```dockerfile
# Distroless — use numeric UID directly
USER 65532:65532

# Alpine/Debian
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
```

CI should fail if the final stage's effective user is root.

---

## 4. Layer caching

Copy dependency manifests before source code — they change far less often.

```dockerfile
# Dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Source after
COPY . .
RUN go build ...
```

Same pattern for Node.js: `package.json` + lock file before `COPY . .`.

---

## 5. .dockerignore — required

Every service must have a `.dockerignore` at its build context root. Minimum:

```
.git
.github
**/*.md
**/*_test.go
**/testdata/
**/node_modules/
**/.env*
**/dist/
**/coverage/
```

Without it, `.env` files, credentials, and test fixtures can enter the build context.

---

## 6. Secrets

- Never pass secrets as `ARG` or `ENV` in Dockerfiles — they appear in image layer history.
- Use BuildKit secrets (`--mount=type=secret`) for build-time credentials (private module proxies, private NPM registries).
- Runtime secrets come from Secrets Manager (AWS Secrets Manager) injected at container start — never baked into the image.

---

## 7. Health checks

Every service Dockerfile must declare a `HEALTHCHECK`:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
  CMD wget -q -O - http://localhost:${PORT}/health || exit 1
```

Kubernetes / ECS liveness and readiness probes are separate platform-layer configuration.

---

## 8. Image scanning

- Run `trivy image` in CI against every built image before pushing to the registry.
- Block on HIGH or CRITICAL CVEs without an approved exception.
- Rebase images monthly even without code changes to pick up OS-level patches.

---

## 9. Image tagging

- Tag with the git commit SHA: `image:${GIT_SHA}`.
- Tag `latest` only on main branch builds — never on feature branches.
- Semantic versioning (`v1.2.3`) for images published to external registries.

---

## 10. docker-compose for local development

- `docker-compose.yaml` in each service directory is for local development only — never used in CI.
- Use `extra_hosts: ["host.docker.internal:host-gateway"]` for containers that need to reach the host.
- Declare `komodo-network` as an external network so services can call each other locally.
- Never commit `.env` files — use `.env.example` with placeholder values.

# Standards

Universal Komodo standards. These are the base — individual projects may add specifics but should not contradict these.

| Standard | Description |
|----------|-------------|
| [Principles](principles.md) | Hard rules, code-reuse priority, idiomatic/DI/explicit design, testability as a design constraint |
| [Komodo Context](komodo-context.md) | Org-wide stack invariants — repo shape, languages, forge SDKs, service anatomy, compute, data, IaC |
| [Security](security.md) | Secrets, input validation, auth, data handling, OWASP baseline, incident response |
| [Pull Requests](pull-requests.md) | PR size, description requirements, review expectations, merge criteria |
| [API Design](api-design.md) | REST conventions, status codes, error format, versioning, documentation |
| [Comments](comments.md) | When to comment, inline brevity, doc-comment scope, what never to leave in code |
| [Go](go.md) | Formatting, error handling, naming, package design, testing, concurrency |
| [TypeScript](typescript.md) | Type safety, naming, null handling, async patterns, testing, component standards |
| [Python](python.md) | Versions, typing, error handling, async, testing, design patterns |
| [Svelte](svelte.md) | Svelte 5 runes, component structure, SvelteKit conventions, accessibility |
| [Testing (TS/JS)](testing-ts.md) | JS/TS test naming (`.x.test.ts`), colocation, single-file structure, SvelteKit and Vue conventions |
| [Testing (Go)](testing-go.md) | Colocation, table-driven structure, mocking at interface boundaries, coverage, race/timing |
| [SQL](sql.md) | Schema conventions, migrations, indexing, query safety |
| [Logging](logging.md) | Log levels, required fields, what never to log, correlation IDs |
| [Observability](observability.md) | Metrics, distributed traces, trace_id propagation, health checks, alerting |
| [Docker](docker.md) | Multi-stage builds, non-root images, layer caching, secrets, image scanning |
| [Token Efficiency](token-efficiency.md) | MCP-first delegation, compaction cadence, lean context passing, model selection |
| [TODO.md](todo.md) | Item format, section headers, no-date rule, what belongs in TODO.md |

## Related skills

- `/git-flow` — Branch naming, commit conventions, PR workflow, release and hotfix process

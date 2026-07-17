# Modes Index

One line per mode — a profile picks which modes to load from this manifest without opening the full files. Detail lives in the file each line points to.

## Language modes (`modes/`, shared across any agent that writes code in that language)

| Mode | File(s) | Covers |
|---|---|---|
| `go` | `modes/go/coding.md` | Go coding standards, approved stack, test-tier gating |
| `ts` / `node` | `modes/ts/coding.md` | TypeScript/Node coding standards, approved stack, testing |
| `python` | `modes/python/coding.md` | Python coding standards, approved stack, testing |
| `svelte` / `ui` | `modes/svelte/coding.md`, `modes/svelte/new-page.md`, `modes/svelte/new-component.md` | SvelteKit coding standards; new-route and new-component skills |
| `vue` | `modes/vue/coding.md` | Vue 3 coding standards |
| `cpp` / `embedded` | `modes/cpp/coding.md` | Non-robotics firmware/embedded C/C++ standards |

## `software-engineer` role modes (`agents/software-engineer/`)

| Mode | File(s) | Covers |
|---|---|---|
| `api` | `api/profile.md`, `api/go-slice.md`, `api/blueprint.md`, `api/design.md`, `api/new-service.md`, `api/add-route.md`, `api/new-middleware.md`, `api/audit.md` | Building or auditing an HTTP API/service |
| `db` / `sql` | `db/sql.md`, `db/new-migration.md` | Schema and migration work |
| `infra` / `terraform` | `infra/new-tf-module.md` | Terraform module authoring |
| `design` / `arch` | `design/design.md` (+ `design/docs/`) | System/architecture design |

Full per-mode activation rules (keyword trigger, load-on-signal standards, inference from working tree): `agents/software-engineer/agent.md` § Modes.

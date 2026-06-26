# Testing Strategy

Cross-cutting testing standard for all Komodo services. Defines which test types run in which environment, how `TEST_TIER` gates execution, and why the environment split exists.

---

## 1. Environment–test matrix

| Environment | Test types | Infra | Purpose |
|---|---|---|---|
| Local (docker-compose) | Unit, component | Public + private containers; graceful degradation, no backing services | Fast feedback loop during development |
| DEV (AWS — minimal) | Unit, component, smoke | Public + private Fargate services + logs — no DynamoDB, ElastiCache, WAF, alarms | Deployed version of local; adds smoke tests and validates the infra deployment itself |
| STG (AWS — full stack) | Integration, E2E, performance, chaos | Full production-mirror infra — DynamoDB, ElastiCache, WAF, alarms, public + private services | Real downstream calls against real services; the only environment where cross-service integration is validated |

---

## 2. Key rules

- DEV has no real backing services — tests that require downstream calls (DynamoDB, ElastiCache, other APIs) belong in STG.
- Local and DEV share the same core test suite (unit + component). DEV adds smoke tests and validates the infra deployment — it is the deployed version of local.
- Integration tests are never run against DEV — the infra does not exist to support them.
- Performance and chaos tests run exclusively in STG against real infra.
- E2E tests hit real services end-to-end and require STG.

---

## 3. Test tier ladder

The QA agent gates test execution via the `TEST_TIER` environment variable. Tiers are ordered; setting a tier runs everything at or below it:

```
unit < component < integration < e2e < chaos
```

| Environment | `TEST_TIER` | What runs |
|---|---|---|
| Local | `component` | Unit + component |
| DEV | `component` | Unit + component + smoke |
| STG | `chaos` | Full ladder (unit + component + integration + e2e + chaos) |

Smoke tests are **not tier-gated** — they are always-on and validate health/boot regardless of `TEST_TIER`.

---

## 4. Why this split

DEV is intentionally stripped to avoid cost and parallel-deployment resource collisions. Real integration validation belongs in STG where consistent, full-stack infra exists. This keeps DEV fast, cheap, and collision-free for parallel PR work.

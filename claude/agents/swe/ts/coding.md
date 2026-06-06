# TypeScript Standards

Base TypeScript/JavaScript coding standards for Komodo. Project-level configs may extend these.

---

## 1. Type safety

- `"strict": true` in all `tsconfig.json` files — no exceptions
- No `any` without a code comment explaining why it's necessary and what would be needed to remove it
- Use `unknown` over `any` for data from external sources (API responses, parsed JSON, user input)
- Runtime validation with Zod (or equivalent) at all system boundaries — TypeScript types alone are not runtime guarantees
- `as` type assertions only when you have verified the type through code logic — not as a way to silence the compiler

---

## 2. Comments

- Doc comments must not open with the function/method/class name — write `// Returns the order for the given ID` not `// fetchOrder returns the order for the given ID`
- Error message strings must not contain the function or method name — put function context in structured log metadata or stack traces, not the message string itself
  - Bad: `throw new Error("fetchOrder: db query failed")`
  - Good: `throw new Error("failed to query order")`

---

## 3. Naming

| Thing | Convention | Example |
|-------|-----------|---------|
| Types, interfaces, classes, components | PascalCase | `OrderSummary`, `UserCard` |
| Variables, functions, methods | camelCase | `fetchOrder`, `isLoading` |
| True constants | SCREAMING_SNAKE | `MAX_RETRY_COUNT` |
| Files (modules) | kebab-case | `order-service.ts` |
| Files (components) | PascalCase | `UserCard.svelte` |
| Enum values | PascalCase | `OrderStatus.Pending` |

- Prefix boolean variables/props with `is`, `has`, `can`, `should`: `isLoading`, `hasError`
- Don't prefix interfaces with `I` — `User` not `IUser`
- Request variables: `req`; response variables: `res` — applies to API handlers, fetch wrappers, and UI service calls

---

## 4. Null handling

- Use optional chaining (`?.`) for potentially undefined access
- Use nullish coalescing (`??`) for defaults — not `||` (which swallows `0` and `""`)
- Avoid non-null assertions (`!`) — if you use one, add a comment explaining why it's safe
- Prefer explicit null/undefined checks over truthiness when the distinction matters

---

## 5. Async patterns

- Always `async/await` over raw promise chains
- Every rejected promise must be caught — unhandled rejections are bugs
- `Promise.all` for parallel independent operations; `Promise.allSettled` when partial failure is acceptable
- Never `await` in a loop when requests can be parallelized
- Async functions that can fail should throw typed errors, not return `null` to signal failure

---

## 6. Module organization

- Single responsibility per file — one primary export per module
- Re-export from barrel `index.ts` files sparingly: they hurt tree-shaking and create circular import risks
- Prefer direct imports over barrel imports for large modules
- No circular imports — if A imports B and B imports A, extract the shared dependency to C
- Co-locate tests with the file under test: `order-service.test.ts` next to `order-service.ts`

---

## 7. Testing

Full JS/TS testing standard — file naming, colocation, single-file structure, and framework-specific conventions (SvelteKit, Vue). Go testing rules live in `~/.claude/agents/swe/go/coding.md` §5.

### 7.1 File placement and naming

Tests are **colocated** with the file or package they test — never in a separate `__tests__/` directory.

**Naming convention: `.x.test.ts` (or `.x.test.js`)**

| Source | Test file |
|--------|-----------|
| `order-service.ts` | `order-service.x.test.ts` |
| `utils.ts` | `utils.x.test.ts` |
| `UserCard.svelte` | `UserCard.x.test.ts` |
| `Button.vue` | `Button.x.test.ts` |

**Package / library folder:** when multiple source files form a cohesive unit (e.g. `src/lib/auth/`), place one test file at the root of that folder named after the folder:

```
src/lib/auth/
├── index.ts
├── session.ts
├── tokens.ts
└── auth.x.test.ts      ← named after the folder
```

If individual files within a package are large enough to warrant their own tests, colocate a test file per file using the file-name convention above.

### 7.2 Single test file, three test types

Unit, component, and integration tests for a given source file live together in one `.x.test.ts` file. Separate each section with a banner comment followed by a `describe` block. Do not split them across separate files.

**Section banner format** (copy exactly — 72 chars total, em-dash `─` U+2500):
```
// ── Unit Tests ──────────────────────────────────────────────────────────
// ── Component Tests ─────────────────────────────────────────────────────
// ── Integration Tests ───────────────────────────────────────────────────
```

```ts
// order-service.x.test.ts

// ── Unit Tests ──────────────────────────────────────────────────────────
describe('unit', () => {
  // Pure logic, no I/O, no network, no DB
  // Test edge cases, error paths, and transformations
})

// ── Component Tests ─────────────────────────────────────────────────────
describe('component', () => {
  // Rendered output and user interaction (UI)
  // Or: service layer with mocked external deps (backend)
})

// ── Integration Tests ───────────────────────────────────────────────────
describe('integration', () => {
  // Real boundaries — DB, HTTP calls, file system
  // Use a real (test) instance, not mocks
})
```

Only include the sections that are relevant. A pure utility module may only need `unit`. A stateless HTTP handler may only need `component` and `integration`. Omit empty sections entirely.

**Tier selection.** Tiers form the same ordered, cumulative ladder used company-wide (see `~/.claude/agents/swe/go/coding.md` §5.1), selected by one env var, `TEST_TIER`:

```
unit < component < integration < e2e < chaos
```

- **unit and component are always-on** — fast, hermetic (pure logic, mocked module boundaries, rendered components). No gate.
- **integration, e2e, and chaos touch real boundaries or span services.** Gate the `describe` block with `skipIf` against the active tier.

```ts
const TIERS = ['unit', 'component', 'integration', 'e2e', 'chaos'] as const
const active = TIERS.indexOf((process.env.TEST_TIER ?? 'unit') as typeof TIERS[number])
const tierBelow = (t: typeof TIERS[number]) => active < TIERS.indexOf(t)

// ── Integration Tests ───────────────────────────────────────────────────
describe.skipIf(tierBelow('integration'))('integration', () => {
  // Real boundaries — DB, HTTP, file system
})
```

Setting a tier runs it and everything below it. The default (no env var) is `unit`; e2e/chaos run only on release stages. Cross-service E2E (Playwright) is exempt — it lives in `e2e/` per §7.6, not colocated.

### 7.3 Base rules (plain TS/JS)

- **Framework:** Vitest
- **Mocking:** `vi.mock` at module boundaries only — never mock internals of the module under test
- **Assertions:** Vitest built-ins (`expect`, `toEqual`, `toThrow`, etc.); no external assertion libraries unless already in the project
- **Test names:** describe behavior — `"returns 404 when order does not exist"`, not `"test fetchOrder"`
- **Isolation:** each test must be independently runnable; no shared mutable state between tests
- **Snapshots:** acceptable for UI output when reviewed carefully; never for business logic

### 7.4 SvelteKit

SvelteKit reserves the `+` prefix for route files. Drop it from test filenames.

| Source | Test file |
|--------|-----------|
| `+page.svelte` | `page.x.test.ts` |
| `+page.server.ts` | `page.server.x.test.ts` |
| `+server.ts` | `server.x.test.ts` |
| `+layout.svelte` | `layout.x.test.ts` |
| `UserCard.svelte` | `UserCard.x.test.ts` |

**Component tests** (`component` block): use `@testing-library/svelte`. Render the component, assert on DOM output and user events. Test rune-driven reactivity by triggering state changes and asserting the resulting DOM.

**Server tests** (`integration` block for `+page.server.ts` / `+server.ts`): call `load` or request handler functions directly. Mock only at the external boundary (e.g. DB client, fetch).

```ts
// page.x.test.ts
import { render, screen, fireEvent } from '@testing-library/svelte'
import Page from './+page.svelte'

describe('unit', () => {
  // Pure helpers or stores used by the page
})

describe('component', () => {
  it('renders the submit button', () => {
    render(Page, { props: { data: mockData } })
    expect(screen.getByRole('button', { name: /submit/i })).toBeInTheDocument()
  })
})

describe('integration', () => {
  // load() function or form action tests
})
```

### 7.5 Vue

| Source | Test file |
|--------|-----------|
| `Button.vue` | `Button.x.test.ts` |
| `useAuth.ts` (composable) | `useAuth.x.test.ts` |

**Component tests** (`component` block): use `@vue/test-utils` (`mount` / `shallowMount`). Assert on rendered output, emitted events, and slot content. Prefer `mount` over `shallowMount` unless child components are heavy or irrelevant.

**Composable tests** (`unit` block): call the composable directly inside `withSetup` or `mount` a minimal host component. Test reactive state changes.

```ts
// Button.x.test.ts
import { mount } from '@vue/test-utils'
import Button from './Button.vue'

describe('unit', () => {
  // Pure logic used inside the component
})

describe('component', () => {
  it('emits click when not disabled', async () => {
    const wrapper = mount(Button, { props: { disabled: false } })
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeTruthy()
  })
})
```

### 7.6 Test runner configuration

- Vitest config lives in `vite.config.ts` (or `vitest.config.ts` if separated)
- The glob pattern must match `.x.test.ts` / `.x.test.js` files:
  ```ts
  test: {
    include: ['src/**/*.x.test.{ts,js}'],
  }
  ```
- E2E tests (Playwright) are **not** colocated — they live in `e2e/` or `tests/` at the project root and are not governed by this standard

---

## 8. Frontend / component standards

- Components should be pure where possible — derive state rather than sync it
- No business logic in UI components — use a service or store layer
- Props should be the minimal set needed — don't pass the entire store object when you need one field
- Side effects in `$effect` (Svelte) or `useEffect` (React) must have correct dependencies and cleanup
- Accessibility: interactive elements must be keyboard-navigable; use semantic HTML before ARIA attributes
- No inline styles — use utility classes; no `!important`

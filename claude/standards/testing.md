# Testing Standards (JS/TS)

Applies to all JavaScript and TypeScript projects. Framework-specific sections override the base rules where they conflict.

**Go testing rules live in [`testing-go.md`](testing-go.md).**

---

## 1. File placement and naming

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

---

## 2. Single test file, three test types

Unit, component, and integration tests for a given source file live together in one `.x.test.ts` file, organized by `describe` blocks. Do not split them across separate files.

```ts
// order-service.x.test.ts

describe('unit', () => {
  // Pure logic, no I/O, no network, no DB
  // Test edge cases, error paths, and transformations
})

describe('component', () => {
  // Rendered output and user interaction (UI)
  // Or: service layer with mocked external deps (backend)
})

describe('integration', () => {
  // Real boundaries — DB, HTTP calls, file system
  // Use a real (test) instance, not mocks
})
```

Only include the sections that are relevant. A pure utility module may only need `unit`. A stateless HTTP handler may only need `component` and `integration`. Omit empty `describe` blocks.

---

## 3. Base rules (plain TS/JS)

- **Framework:** Vitest
- **Mocking:** `vi.mock` at module boundaries only — never mock internals of the module under test
- **Assertions:** Vitest built-ins (`expect`, `toEqual`, `toThrow`, etc.); no external assertion libraries unless already in the project
- **Test names:** describe behavior — `"returns 404 when order does not exist"`, not `"test fetchOrder"`
- **Isolation:** each test must be independently runnable; no shared mutable state between tests
- **Snapshots:** acceptable for UI output when reviewed carefully; never for business logic

---

## 4. SvelteKit

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

---

## 5. Vue

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

---

## 6. Test runner configuration

- Vitest config lives in `vite.config.ts` (or `vitest.config.ts` if separated)
- The glob pattern must match `.x.test.ts` / `.x.test.js` files:
  ```ts
  test: {
    include: ['src/**/*.x.test.{ts,js}'],
  }
  ```
- E2E tests (Playwright) are **not** colocated — they live in `e2e/` or `tests/` at the project root and are not governed by this standard

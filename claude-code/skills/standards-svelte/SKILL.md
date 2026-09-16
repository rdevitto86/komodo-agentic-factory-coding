---
name: standards-svelte
description: Svelte 5 and SvelteKit — runes, component structure, effects, routing. Load before reading or writing any .svelte, +page.ts, +page.server.ts, +layout.ts, or +server.ts file.
user-invocable: false
paths: "**/*.svelte, **/+page.ts, **/+page.server.ts, **/+layout.ts, **/+layout.server.ts, **/+server.ts"
---

# Svelte 5

Runes only. Legacy Svelte 4 patterns are never correct here — model training skews toward them, so check every reactive construct against the table below.

**Load `standards-typescript` alongside this skill.** `standards-typescript` stays framework-agnostic on purpose — its own `paths:` never names `.svelte`, so nothing auto-loads it here. Invoke it explicitly; its conventions, toolchain, and Quick-reference fields apply to every `<script>` block without restatement.

## Comment discipline

`standards-typescript`'s directives apply inside `<script>`. **The template's `<!-- -->` markup comments are scanned on the same terms as `//`** — `standards-comments`' bar and bans apply to them unchanged.

## Toolchain

`standards-typescript`'s Toolchain section applies unchanged — versions from `package.json`, `npm audit` as the vulnerability gate, the Forge SDK import rule. This section names only what's Svelte-specific:

- **`eslint-plugin-svelte`** alongside the base ESLint config `standards-typescript` names.
- **`svelte-check`** for typechecking — `tsc` alone does not understand `.svelte` files.
- **`Makefile`'s `verify` target is the merge gate**, delegating to `package.json` scripts (`lint`, `typecheck`, `test`, `build`). `context_injector.py` reads this target directly; a repo without it has no gate.

## Runes — non-negotiable

| Use | Never use |
|---|---|
| `$props()` for component inputs | `export let` |
| `$state()` for reactive local state | `let` with `$:` assignment |
| `$derived()` for computed values | `$:` reactive statements |
| `$effect()` for side effects | `beforeUpdate` / `afterUpdate` |
| Callback props (`onclick?: () => void`) | `createEventDispatcher` |
| `$bindable()` for two-way bound form controls | Stores for component communication |

```svelte
<script lang="ts">
  interface Props {
    label: string;
    onclick?: () => void;
    class?: string;
  }

  let { label, onclick, class: className = '' }: Props = $props();
  let count = $state(0);
  let doubled = $derived(count * 2);
</script>
```

## Component structure

Order inside `<script lang="ts">`:

1. Props interface and `$props()` destructuring
2. State (`$state`)
3. Derived values (`$derived`)
4. Effects (`$effect`)
5. Functions

One component per `.svelte` file.

## TypeScript

- **Every component has `<script lang="ts">`.**
- **A named `interface Props` above `$props()`** — never inline the type.
- **Props without defaults are required**; `?` marks optional.
- **Separate required, defaulted, and rest props** in destructuring:

```ts
let { required, optional = 'default', class: className = '', ...rest }: Props = $props();
```

## $effect rules

`$effect` runs after every render where tracked state changed. Misuse causes infinite loops.

- **Access reactive state directly inside the body** so tracking is accurate.
- **Never set `$state` unconditionally inside `$effect`** — that is the loop.
- **`$effect.pre`** when you must run before DOM updates.
- **Always return a cleanup** when subscribing to events or timers:

```ts
$effect(() => {
  const handler = () => { width = window.innerWidth; };
  window.addEventListener('resize', handler);
  return () => window.removeEventListener('resize', handler);
});
```

## SvelteKit

- **`+page.svelte` receives data via `$props()`**: `let { data }: { data: PageData } = $props()`. `export let data` is the Svelte 4 pattern — never use it.
- **Server-side loading lives in `+page.server.ts`** as `export const load: PageServerLoad`.
- **Always generate `+page.server.ts`**, even for a simple public page. A minimal `load` returning `{}` is correct — it marks the page server-rendered and gives you the extension point.
- **Form actions** go in `+page.server.ts` under `export const actions`.
- **`+server.ts` routes** return typed responses via `json()` from `@sveltejs/kit`.
- **Never set `prerender = true` or `ssr = false`** unless explicitly required.

## Testing

- **Unit tests colocate**; component tests live in `test/component/`, flat by feature, importing via the `$lib` alias.
- **`@testing-library/svelte`**, rendering with `render(Component, { props: { ... } })` — never `new Component()`.
- **Drop the `+` from route test names**: `+page.svelte` becomes `page.x.test.ts`.
- **Assert on DOM output and user events.** Never reach into component internals.
- **Test tiers, folder scheme, suites, helper placement, and descriptions** follow the `standards-typescript` skill; gates and coverage floors are defined by the `standards-sdlc` skill.

## Quick-reference fields

The field set a Svelte repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| Framework + floor | `package.json` |
| Dev port | `vite.config.ts` |
| Adapter | `svelte.config.js` |
| Path alias | `svelte.config.js` |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill, `standards-typescript`, or `standards-ui-web` already states by name.

## Repo layout — `svelte-ui`

```
src/lib/
src/routes/
docs/
test/
deploy/
package.json
vite.config.ts
Makefile
```

## Seed backlog — `svelte-ui`

Stories `git-repo-init` splices into `Cross-Cutting` on Create, or appends if missing on Scaffold/Refresh.

- [H] Build out the starter page's components · M
- [H] Accessibility: WCAG AA pass (`standards-ui-web`) · S
- [M] Tests: unit + component coverage · S → `make test`

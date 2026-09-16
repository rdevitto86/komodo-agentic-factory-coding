# Svelte 5

Runes only. Legacy Svelte 4 patterns are never correct here — model training skews toward them, so check every reactive construct against the table below.

The TypeScript and web UI standards apply to every `<script>` block alongside this one.

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
- **Test tiers, folder scheme, suites, helper placement, and descriptions** follow the typescript standard; gates and coverage floors are defined by the sdlc standard.

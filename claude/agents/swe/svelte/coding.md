# Svelte 5 Standards

Applies to all SvelteKit and Svelte 5 projects.

---

## 1. Runes — non-negotiable

Svelte 5 replaces the legacy reactive model with runes. Never use legacy patterns.

| Use | Never use |
|-----|-----------|
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

---

## 2. Component structure

Order within `<script lang="ts">`:
1. Props interface + `$props()` destructuring
2. State (`$state`)
3. Derived values (`$derived`)
4. Effects (`$effect`) — after all state/derived
5. Functions

One component per `.svelte` file. `<style>` blocks only when Tailwind is genuinely insufficient — prefer utility classes. Never use `@apply` except for deeply nested selector patterns.

---

## 3. TypeScript

- Every component must have `<script lang="ts">`.
- Define a named `interface Props {}` above `$props()` — do not inline the type.
- Props without defaults are required; use `?` for optional props.
- Separate required, optional-with-default, and rest props in destructuring:

```ts
let { required, optional = 'default', class: className = '', ...rest }: Props = $props();
```

---

## 4. Tailwind v4

- Use utility classes directly. No `@apply` except for deeply nested patterns.
- Mobile-first responsive: `sm:`, `md:`, `lg:` prefixes.
- Animations: prefer `tw-animate-css` classes or GSAP for complex sequences. Do not use `transition-*` for entrance animations on initial render.
- Dark mode: use `dark:` prefix classes — no manual class toggling.

---

## 5. Accessibility — required on every component

- Interactive elements must have visible focus ring (do not remove `:focus-visible` without replacing it) and keyboard handlers where `onclick` is present.
- Images: `alt` required. Decorative images: `alt=""`.
- ARIA roles and labels where semantic HTML is ambiguous.
- Color contrast: WCAG AA minimum (4.5:1 text, 3:1 UI elements).
- Never use `tabindex > 0`.

---

## 6. $effect rules

`$effect` runs after every render where tracked state changed. Misuse causes infinite loops.

- Track only what the effect depends on — access reactive state directly inside the body.
- Do not set `$state` unconditionally inside `$effect` — this creates a loop.
- Use `$effect.pre` when you need to run before DOM updates.
- Always return a cleanup function when subscribing to events or timers:

```ts
$effect(() => {
  const handler = () => { /* ... */ };
  window.addEventListener('resize', handler);
  return () => window.removeEventListener('resize', handler);
});
```

---

## 7. SvelteKit conventions

- `+page.svelte` receives data via `$props()`: `let { data }: { data: PageData } = $props()`.
- `export let data` is the Svelte 4 pattern — never use it.
- Server-side loading lives in `+page.server.ts` as `export const load: PageServerLoad`.
- Never use `prerender = true` or `ssr = false` unless explicitly required.
- Form actions go in `+page.server.ts` under `export const actions`.
- `+server.ts` BFF routes return typed responses via `json()` from `@sveltejs/kit`.

---

## 8. Testing

Colocate test files per `~/.claude/agents/swe/ts/coding.md`. Component tests use `@testing-library/svelte`.

- Drop the `+` from route file test names: `+page.svelte` → `page.x.test.ts`
- Render with `render(Component, { props: { ... } })` — not `new Component()`
- Assert on DOM output and user events; do not reach into component internals
- Trigger state changes and assert the resulting DOM to test rune-driven reactivity

See `~/.claude/agents/swe/ts/coding.md §4` for full SvelteKit test conventions.

---
name: uiux
description: UI standards — Tailwind utilities and the WCAG AA component bar. Load before writing any interactive component, Tailwind class, .svelte, .vue, .tsx, .jsx, or .css file.
user-invocable: false
paths: "**/*.css, **/*.svelte, **/*.vue, **/*.tsx, **/*.jsx"
---

# UI/UX

Framework-agnostic. Framework mechanics live in the `svelte` and `vue` skills; document formatting lives in `adhd-format`.

## Tailwind

The major version comes from the repo's `package.json` — read it before using a version-gated feature.

- **Utility classes only.** No `@apply` except for deeply nested selector patterns, no inline styles, no `!important`.
- **Mobile-first responsive** — `sm:`, `md:`, `lg:` prefixes.
- **Dark mode via `dark:` prefixes.** Never toggle classes manually.
- **Animations**: `tw-animate-css` classes, or GSAP for complex sequences. Do not use `transition-*` for entrance animations on initial render.
- **`<style>` blocks only when Tailwind genuinely cannot express it.**

## Component rules — every framework

Declaration order inside a component file is framework mechanics; `svelte` and `vue` each own theirs. These three hold regardless.

- **No business logic in components.** Use a service or store layer.
- **Derive state rather than syncing it.** Pass minimal props.
- **Effects need correct dependencies and a cleanup.**

## WCAG — required on every component

- **Visible focus ring** on every interactive element. Do not remove `:focus-visible` without replacing it.
- **Keyboard handlers wherever a click handler exists.**
- **`alt` on every image.** Decorative images take `alt=""`.
- **ARIA roles and labels** where semantic HTML is ambiguous.
- **WCAG AA contrast minimum** — 4.5:1 for text, 3:1 for UI elements.
- **Never `tabindex > 0`.**

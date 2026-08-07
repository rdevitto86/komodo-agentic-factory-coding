---
name: tailwind
description: Tailwind v4 usage — utility classes, mobile-first, dark mode, animation. Load before writing Tailwind classes in any component, .css, .tsx, or .jsx file.
user-invocable: false
---

# Tailwind v4

- **Utility classes directly.** No `@apply` except for deeply nested selector patterns.
- **Mobile-first responsive** — `sm:`, `md:`, `lg:` prefixes.
- **Dark mode via `dark:` prefixes.** Never toggle classes manually.
- **Animations**: `tw-animate-css` classes, or GSAP for complex sequences. Do not use `transition-*` for entrance animations on initial render.
- **`<style>` blocks only when Tailwind genuinely cannot express it.**

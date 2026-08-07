---
name: wcag
description: Component accessibility bar — focus, keyboard, alt text, ARIA, contrast. Load before writing any interactive component, .svelte, .vue, .tsx, or .jsx file.
user-invocable: false
---

# WCAG — required on every component

- **Visible focus ring** on every interactive element. Do not remove `:focus-visible` without replacing it.
- **Keyboard handlers wherever a click handler exists.**
- **`alt` on every image.** Decorative images take `alt=""`.
- **ARIA roles and labels** where semantic HTML is ambiguous.
- **WCAG AA contrast minimum** — 4.5:1 for text, 3:1 for UI elements.
- **Never `tabindex > 0`.**

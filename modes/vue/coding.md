# Vue 3 Standards

Applies to all Vue 3 projects (e.g. `komodo-ecom`).

---

## 1. Composition API only

Vue 3 supports both Options and Composition APIs. Always use Composition API with `<script setup lang="ts">`. Never use the Options API (`data()`, `methods`, `computed: {}` object syntax, `export default { ... }`).

| Use | Never use |
|-----|-----------|
| `<script setup lang="ts">` | `<script lang="ts">` + `export default { ... }` |
| `defineProps<Props>()` with a named `interface Props` | `props: { ... }` runtime declarations |
| `defineEmits<{ ... }>()` with TS generics | `emits: ['update']` string-array declarations |
| `ref()` / `reactive()` for state | `data() { return { ... } }` |
| `computed()` | `computed: { ... }` object syntax |
| `watch()` / `watchEffect()` | `watch: { ... }` object syntax |

```vue
<script setup lang="ts">
interface Props {
  label: string;
  disabled?: boolean;
}

interface Emits {
  (e: 'update', value: string): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const count = ref(0);
const doubled = computed(() => count.value * 2);
</script>
```

---

## 2. Component structure

Order within `<script setup lang="ts">`:
1. `defineProps<Props>()` / `defineEmits<Emits>()`
2. Injected dependencies / composables (`inject`, `useXyz()`)
3. Reactive state (`ref`, `reactive`)
4. Computed values (`computed`)
5. Watchers (`watch`, `watchEffect`) — used sparingly, always cleaned up
6. Methods
7. Lifecycle hooks (`onMounted`, `onUnmounted`, etc.) — last

One component per `.vue` file. `<style>` blocks only when Tailwind is genuinely insufficient — prefer utility classes. Never use `@apply` except for deeply nested selector patterns.

---

## 3. State management

- **Component-local state** — `ref()` / `reactive()` inside `<script setup>`. Don't reach for Pinia for state that only one component (and its direct children via props/emits) needs.
- **Cross-component / app state** — Pinia, using **setup-store syntax** (a function returning refs/computeds/methods), not the options-store syntax (`{ state, getters, actions }`).

```ts
// stores/cart.ts
export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>([]);
  const total = computed(() => items.value.reduce((sum, i) => sum + i.price * i.qty, 0));

  function addItem(item: CartItem) {
    items.value.push(item);
  }

  return { items, total, addItem };
});
```

---

## 4. TypeScript

- Every component must have `<script setup lang="ts">`.
- Define a named `interface Props {}` above `defineProps<Props>()` — do not inline the type.
- Props without `?` are required; use `?` for optional props.
- Type emits with the call-signature form (`defineEmits<{ (e: 'eventName', payload: T): void }>()`), not the array shorthand.

```ts
interface Props {
  itemId: string;
  variant?: 'primary' | 'secondary';
}

const props = defineProps<Props>();
```

---

## 5. Tailwind v4

- Use utility classes directly. No `@apply` except for deeply nested patterns.
- Mobile-first responsive: `sm:`, `md:`, `lg:` prefixes.
- Animations: prefer `tw-animate-css` classes or GSAP for complex sequences. Do not use `transition-*` for entrance animations on initial render.
- Dark mode: use `dark:` prefix classes — no manual class toggling.

---

## 6. Accessibility — required on every component

- Interactive elements must have visible focus ring (do not remove `:focus-visible` without replacing it) and keyboard handlers where `@click` is present.
- Images: `alt` required. Decorative images: `alt=""`.
- ARIA roles and labels where semantic HTML is ambiguous.
- Color contrast: WCAG AA minimum (4.5:1 text, 3:1 UI elements).
- Never use `tabindex > 0`.

---

## 7. Reactivity pitfalls

- Prefer `computed()` over `watch()` where the value can be derived directly — `watch` is for side effects, not derived state.
- Always register a cleanup in `watch` / `watchEffect` when subscribing to events, timers, or external resources:

```ts
watchEffect((onCleanup) => {
  const handler = () => { /* ... */ };
  window.addEventListener('resize', handler);
  onCleanup(() => window.removeEventListener('resize', handler));
});
```

- Avoid mutating a `ref` / `reactive` inside a `watchEffect` that also reads it — this creates an infinite loop. If a watcher must update its own source, write to a different ref or guard with a condition that breaks the cycle.

---

## 8. Testing

See `~/.claude/modes/ts/coding.md §7.5` for Vue test conventions (`@vue/test-utils`, `mount`/`shallowMount`, `.vue` → `.x.test.ts` naming).

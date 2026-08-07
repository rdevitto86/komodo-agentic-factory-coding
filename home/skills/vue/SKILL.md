---
name: vue
description: Vue 3 — Composition API, Pinia setup stores, reactivity pitfalls. Load before reading or writing any .vue file or a Pinia store.
user-invocable: false
---

# Vue 3

Composition API with `<script setup lang="ts">`, always. The Options API is never correct here.

## Composition API only

| Use | Never use |
|---|---|
| `<script setup lang="ts">` | `<script lang="ts">` + `export default { ... }` |
| `defineProps<Props>()` with a named interface | `props: { ... }` runtime declarations |
| `defineEmits<{ ... }>()` with TS generics | `emits: ['update']` string arrays |
| `ref()` / `reactive()` | `data() { return { ... } }` |
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

## Component structure

Order inside `<script setup lang="ts">`:

1. `defineProps` / `defineEmits`
2. Injected dependencies and composables
3. Reactive state (`ref`, `reactive`)
4. Computed values
5. Watchers — sparingly, always cleaned up
6. Methods
7. Lifecycle hooks, last

One component per `.vue` file.

## State management

- **Component-local state** uses `ref()` / `reactive()`. Do not reach for Pinia for state one component and its direct children need.
- **Cross-component state uses Pinia in setup-store syntax** — a function returning refs, computeds, and methods. Never the options-store form.

```ts
export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>([]);
  const total = computed(() => items.value.reduce((sum, i) => sum + i.price * i.qty, 0));

  function addItem(item: CartItem) {
    items.value.push(item);
  }

  return { items, total, addItem };
});
```

## TypeScript

- **A named `interface Props` above `defineProps<Props>()`** — never inline.
- **Props without `?` are required.**
- **Type emits with the call-signature form**, not the array shorthand.

## Reactivity pitfalls

- **Prefer `computed()` over `watch()`** where the value derives directly. `watch` is for side effects, not derived state.
- **Always register a cleanup** in `watch` / `watchEffect` when subscribing to events, timers, or external resources:

```ts
watchEffect((onCleanup) => {
  const handler = () => { width.value = window.innerWidth; };
  window.addEventListener('resize', handler);
  onCleanup(() => window.removeEventListener('resize', handler));
});
```

- **Never mutate a `ref` inside a `watchEffect` that also reads it** — that is an infinite loop. Write to a different ref, or guard with a condition that breaks the cycle.

## Testing

`@vue/test-utils` with `mount` / `shallowMount`. Test files follow `.x.test.ts` naming. Unit tests colocate; component tests live in `test/component/`, flat by feature.

Test tiers, gates, and coverage floors follow the `typescript` skill's folder scheme, defined by the `sdlc` skill.

## Repo layout — `vue-ui`

```
src/lib/
src/routes/
docs/
test/
deploy/
package.json
vite.config.ts
```

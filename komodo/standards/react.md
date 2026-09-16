# React

Function components and hooks, always. Class components are never correct here — model training skews toward them, so check every stateful construct against the table below.

The TypeScript and web UI standards apply to every component file alongside this one; this file covers React mechanics only.

## Function components and hooks only

| Use | Never use |
|---|---|
| `function Component(props: Props)` | `class Component extends React.Component` |
| `useState()` | `this.state` / `this.setState()` |
| `useEffect()` / `useLayoutEffect()` | `componentDidMount` / `componentDidUpdate` / `componentWillUnmount` |
| `useMemo()` / `useCallback()` | manual `shouldComponentUpdate` |
| `useContext()` | `<Context.Consumer>` render props |
| Named props interface | `React.FC<Props>` (adds an implicit `children`, no real benefit) |

```tsx
interface ButtonProps {
  label: string;
  disabled?: boolean;
  onClick?: () => void;
}

function Button({ label, disabled = false, onClick }: ButtonProps) {
  const [pressed, setPressed] = useState(false);

  return (
    <button disabled={disabled} onClick={onClick} aria-pressed={pressed}>
      {label}
    </button>
  );
}
```

## Rules of Hooks — non-negotiable

- **Call hooks only at the top level.** Never inside a condition, loop, or nested function.
- **Call hooks only from a function component or another hook.** Never from a plain function or a class method.
- **A custom hook's name starts with `use`** — that is what makes the linter and the rule above enforceable.
- **The dependency array lists every reactive value the callback reads.** Do not suppress the lint rule to silence a warning; fix the dependency instead.

## Component structure

Order inside a function component:

1. Hooks — `useState`, `useReducer`, context, then derived/custom hooks
2. Derived values (`useMemo`, plain `const`)
3. Event handlers (`useCallback` where a child memoizes on it)
4. Effects (`useEffect`, `useLayoutEffect`), last before the return
5. Early returns (loading/error guards)
6. JSX return

One component per file. A file exporting a component exports nothing else stateful.

## State management

- **Component-local state** uses `useState()` / `useReducer()`. Do not reach for a global store for state one component and its direct children need.
- **Cross-component state** uses `useContext()` for read-mostly values (theme, auth session) or a dedicated store library for state that changes often — read the repo's `package.json` for which one it has adopted rather than assuming.
- **Derive state, never sync it.** A value computable from props or other state is a `useMemo`, not a second `useState` kept in step with an effect.

```tsx
const total = useMemo(
  () => items.reduce((sum, item) => sum + item.price * item.qty, 0),
  [items],
);
```

## Effects — pitfalls

`useEffect` runs after every render where a listed dependency changed. Misuse causes infinite loops or stale closures.

- **List every value the effect body reads** in the dependency array — an incomplete array is a stale-closure bug, not an optimization.
- **Never set state unconditionally inside an effect that also reads it** — that is the infinite loop.
- **Always return a cleanup function** when subscribing to events, timers, or external resources:

```tsx
useEffect(() => {
  const handler = () => setWidth(window.innerWidth);
  window.addEventListener('resize', handler);
  return () => window.removeEventListener('resize', handler);
}, []);
```

- **Prefer deriving the value during render** (a plain `const` or `useMemo`) over `useEffect` + a second `useState` — an effect is for synchronizing with something outside React, not for computing a value React already has the inputs for.

## TypeScript

- **A named `interface Props` above the component signature** — never inline the type.
- **Props without a default are required**; a default value or `?` marks optional.
- **Type children explicitly** (`children: React.ReactNode`) — never widen to `any` to sidestep it.

## Testing

- **`@testing-library/react`**, rendering with `render()` and querying via `screen` — never reach into component internals or a `wrapper.instance()`-style API.
- **`userEvent` over `fireEvent`** for interaction — it models real browser event sequences.
- **Unit tests colocate as `.x.test.tsx`**; component tests live in `test/component/`, flat by feature.
- **Assert on DOM output and user events.** Never assert on hook call counts or internal state.
- **Test tiers, folder scheme, suites, helper placement, and descriptions** follow the typescript standard; gates and coverage floors are defined by the sdlc standard.

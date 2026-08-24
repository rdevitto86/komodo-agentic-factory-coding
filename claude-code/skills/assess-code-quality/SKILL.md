---
name: assess-code-quality
description: Score the current diff's conformance to Komodo conventions (Low → Critical) — structure, naming, SDK reuse, and performance — with a one-paragraph cited rationale.
argument-hint: []
disable-model-invocation: true
---

# Komodo conventions assessment

**Once per task, before `/workflow-complete` or a push — never per-edit.** Scores how well the diff conforms to Komodo's documented conventions — structure, naming, SDK reuse, and performance — not whether it works. Read the diff before scoring; every claim comes from what actually changed.

**The catch-all, not a fixer.** `/simplify` and `/code-review` apply reuse/simplification/efficiency fixes; this scores what's left if nobody gets to them. `/assess-bugs`, `/assess-change-risk`, and `/assess-security` are single-dimension scores the user invokes on their own; this one folds `/assess-performance` in as a standing input instead of leaving performance to be invoked separately.

## Process

1. Read the diff.
2. Invoke `/assess-performance` and note its tier.
3. Score structure, naming, and SDK-reuse conformance against `AGENTS.md` and the touched files' `standards-<lang>` skill.
4. Report the **higher** of the performance tier and the structure/convention tier — cite both if they differ.

## The scale

| Tier | Convention test |
|---|---|
| Low | Idiomatic, matches surrounding patterns and every applicable `standards-<lang>`/`AGENTS.md` convention |
| Low-Med | Minor duplication, naming drift, or a slightly awkward shape, contained to one function |
| Med | A missing abstraction, a copy-pasted block, or a custom implementation where the Forge SDK already covers it, contained to one file |
| Med-High | Drift spanning multiple files in the same package — naming, SDK reuse, or a pattern `/assess-performance` flagged |
| High | A pattern that will be copied again by the next person to touch this area — the drift compounds |
| Critical | Actively contradicts a documented convention (`AGENTS.md`, a `standards-<lang>` skill's stated pattern, or an SDK reuse-first rule) — the codebase now teaches the wrong lesson |

Score the **highest tier reached by either the convention check or `/assess-performance`** — one Critical-tier file outweighs nine Low-tier ones.

## Output

```markdown
## Convention debt: <TIER>

<one paragraph, ≤3 sentences, citing file:line for the driving factor(s) and naming which dimension — structure, naming, SDK reuse, or performance — drove the tier>

**Next:** run /simplify, or address what /assess-performance named — <one-line reason>
```

- **`file:line` for every driving factor.** No pointer, no score.
- **Omit `Next` below Med-High.** Nothing to recommend at Low through Med.
- **One paragraph only.** If the reasoning needs more than 3 sentences, the diff needs splitting, not a longer writeup.

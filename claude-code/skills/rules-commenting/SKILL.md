---
name: rules-commenting
description: The comment templates the write guard accepts, and the ones it denies. Load before adding any comment.
user-invocable: false
---

# Strict Commenting Rules

You must strictly limit code comments. **A non-compliant comment prompts the user for approval before the write lands — it does not fail outright.** That is deliberate while these directives are still being tuned: write only a comment you actually believe is warranted, since every miss costs the user a decision.

**Deleting a comment you did not add always prompts too**, regardless of shape — moving or refactoring code is not licence to drop someone else's note.

**Applies to every comment syntax**, not just `//` — block comments (`/* */`), Python docstrings, and HTML comments (`<!-- -->` in Vue/Svelte templates) are all scanned the same way.

## BANNED Comment Types
1. **Name Echoes**: Never write comments where the first word repeats the function/variable/type name below it.
   - ❌ BAD: `// getUser fetches the user`
   - ❌ BAD: `/** User class representing a user */`
2. **Implementation Narratives**: Never explain *what* code is doing or describe standard syntax.
   - ❌ BAD: `// Loop through the list of items`
   - ❌ BAD: `// Instantiate a new database connection`
3. **Redundant Docstrings / JSDoc**: Do not write JSDoc, GoDoc, or Python docstrings for internal/private utilities unless explicitly requested by the user.

## ALLOWED Comment Templates
Only write comments that match one of these exact formats:

1. **Compiler & Linter Directives** (Always allowed)
   - `// eslint-disable-next-line`
   - `# noqa: E501`
   - `//go:build !integration`
   - `// @ts-expect-error [reason]`

2. **Step Markers** (Indented, inside function bodies, <= 80 chars)
   - `// 1. Authenticate incoming JWT`
   - `// 2. Hydrate cache if missing`

3. **Banners / Section Breaks** (<= 40 char label)
   - `// --- User Handlers ---`
   - `# --- Database Schema Setup ---`

4. **Intent & Non-Obvious "WHY"** (Must use prefix)
   - `// WHY: [Explain workaround, hardware bug, or legacy constraints]`
   - `// NOTE: [Explain subtle side effect or performance tradeoff]`
   - `// TODO(author/issue): [Actionable task]`
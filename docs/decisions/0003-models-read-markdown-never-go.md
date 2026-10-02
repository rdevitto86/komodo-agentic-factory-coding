# 0003. Models read markdown, never Go

**Status:** Accepted, 2026-09-25. Restates the first line's decision 0002.

**Context.** A model that reads the conductor's source can reason about the stage order and route around it, and every token spent on Go is a token not spent on the work.

**Decision.** Rules, roles, skills, checklists and policy are markdown or JSON under `komodo/`. The conductor fills a brief from them, and a model reads only its brief. The stage order lives only in the binary.

**Alternatives.**

- **Prompts inside Go.** Faster to change, but invisible to review, and a host swap would touch them.

**Consequences.**

- **A reviewer reads one brief,** the whole input a builder got.
- **Swapping a model is a profile row.**

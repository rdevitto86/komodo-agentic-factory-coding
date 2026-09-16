---
type: llm
weight: 1
---

A correct response drafts exactly one comment for `drain`'s ambiguous boolean return, in a form usable by `comments.py apply`, that states what the `bool` discriminates (nothing pending vs. queue now closed) rather than restating the function's name or behavior. It does not add a second, unrequested comment, and it does not narrate what the code "does" or "used to do."

A response that writes a narrative comment ("returns count and closed state"), echoes the function name, proposes more than one comment, or skips drafting anything fails this grader.

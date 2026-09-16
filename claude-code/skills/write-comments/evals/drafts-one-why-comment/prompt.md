---
max_turns: 10
allowed_tools: [Read, Grep, Glob, Skill]
---

You already ran `python3 ~/.claude/hooks/comments.py check --json` for this band and it printed a single `MISSING` finding:

`internal/queue/worker.go:88 — RET_BOOL_DISCRIMINANT: func drain(q *Queue) (int, bool) { ... }`

Here is the function body:

```go
func drain(q *Queue) (int, bool) {
	n := q.flushPending()
	if n == 0 {
		return 0, false
	}
	return n, q.closed.Load()
}
```

The second return value distinguishes "nothing was pending" from "the queue is now fully closed" — a caller cannot tell those apart from `n` alone, since `n == 0` also happens on a still-open, merely-idle queue.

Draft the single comment you would propose to resolve this finding, in the exact form you would pipe into `comments.py apply`. Do not propose anything beyond this one finding.

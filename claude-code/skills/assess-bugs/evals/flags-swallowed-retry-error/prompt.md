---
max_turns: 12
allowed_tools: [Read, Grep, Glob, Skill]
---

Run an assess-bugs review with this brief:

Task: retry the upstream write on a transient network error — band summary: "add retry-with-backoff around the upstream write call."
Files: `internal/client/upload.go`
Context: the write previously failed hard on any error; this band adds three retries with exponential backoff before giving up.
Round: 1
Standards: standards-go
Out of scope: performance tuning of the backoff curve

Here is the entire diff for `internal/client/upload.go`:

```go
func uploadWithRetry(c *Client, data []byte) error {
	var err error
	for i := 0; i < 3; i++ {
		err = c.write(data)
		if err == nil {
			return nil
		}
		time.Sleep(backoff(i))
	}
	return nil
}
```

Report your findings.

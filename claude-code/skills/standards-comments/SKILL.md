---
name: standards-comments
description: Comment rules for any source file — what earns a comment, what is banned, and the one write path.
user-invocable: false
paths: "**/*.go, **/*.py, **/*.pyi, **/*.ts, **/*.tsx, **/*.js, **/*.jsx, **/*.java, **/*.kt, **/*.rs, **/*.rb, **/*.cs, **/*.swift, **/*.c, **/*.cc, **/*.cpp, **/*.h, **/*.hpp, **/*.php, **/*.scala, **/*.dart, **/*.lua, **/*.sh, **/*.bash, **/*.zsh, **/*.sql, **/*.vue, **/*.svelte, **/*.zig, **/*.tf"
---

# Comments

**Two defaults, pulling opposite ways on purpose.**

**A function declaration is documented by default.** Public and private alike — a private helper is read more often than the exported wrapper around it. Say what it does and what comes back, in one line. Exempt: a test file or test helper, a one-statement body (a getter, `String()`, `Error()`), a declaration with no body, and a generated file. A Python docstring or a JSDoc block already satisfies it.

**Everything else is undocumented by default.** A statement, a branch, a loop, a `var`, a `const`, a struct field: write nothing unless the line cannot say it itself.

**The bar, both directions: what the code does, as it stands today.**

## Banned regardless of phrasing

- **An external citation** — a version, an SDK release, a `PRD`/`SDD`/`ADR`, a ticket ID, "as discussed", "per the spec", "this task". The reader has no access to your conversation, your backlog, or the release you built against. Reported as `EXTERNAL_REF`; `apply` refuses it outright.
- **A restatement** — `// increments the counter` above `counter++`. A function's summary is the carve-out, but it must add what the name does not.
- **A hypothetical** — "can only mean a programmer mistake", "should never happen".
- **Call-site reasoning** — "every call site above passes a non-empty prefix". Callers move.
- **Point-in-time context** — "raised from 200 after the audit", "replaces the old sweep". Commit-message material.

## Shape

**Two lines above a function, one line above everything else.** One sentence, under 120 characters (80 for a trailing field comment). A machine directive does not count against the cap. Over-cap blocks are `OVER_LINES`; two separate blocks landing within two lines of each other are `STACKED`.

In Go a comment may open with the declared name if it is a full one-sentence doc (`// Load reads the config from disk and applies the defaults.`), exported or not. Anywhere else, opening with the identifier is `NAME_ECHO`.

## A correctly commented file

```go
package config

// reads the config from disk and applies the defaults
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data)
}

// lowercases a key after trimming its surrounding space
func normalize(raw string) string {
	trimmed := strings.TrimSpace(raw)
	return strings.ToLower(trimmed)
}

// false covers both a missing key and one that failed to parse
func (c *Config) lookup(key string) (string, bool) {
	value, ok := c.entries[normalize(key)]
	return value, ok
}

func (c *Config) Name() string {
	return c.name
}
```

Every function carries one line. `Name()` carries none — a one-statement body is exempt, and a comment there would only read the name back. No block runs past its cap, none opens with its own identifier, and none cites anything outside the file.

## The one write path

```bash
python3 ~/.claude/hooks/comments.py check --json          # what is missing or malformed
python3 ~/.claude/hooks/comments.py apply <<< '<proposals JSON>'
```

`apply` is the only sanctioned way to add a comment — it enforces shape, echo, cap, block length, citations, and adjacency before the text reaches the file. Writing a comment with `Edit`/`Write` forfeits all of them and `check` catches it at verify time. The proposal shape and the nine template types are in the `write-comments` skill's `reference.md`.

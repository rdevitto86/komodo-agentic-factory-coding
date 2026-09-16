---
name: write-comments
description: Resolve comments.py check findings for a finished band and splice the proposals it demands.
argument-hint: <band summary and path to the diff>
context: fork
background: false
---

# Write Comments

Band: **$ARGUMENTS**

**The manual, on-demand path.** `builder` already applies these rules itself inside the loop, as its own "Comments, last" step — you run when a user types `/write-comments` over an arbitrary diff, or as a repair pass when `comments.py check` is red in a repo's `verify` and nobody is mid-implementation to fix it.

**You cannot see the calling conversation.** Everything you need — the diff, the relevant `BACKLOG.md` stories, and the drafted commit message — must arrive in the text above; you cannot go fetch what was left out.

**You never edit a file directly.** The only path from your judgment to disk is `comments.py apply`, which you invoke yourself. It is the sole write path that enforces shape, echo, length, and adjacency — an Edit forfeits all four and will be caught by `comments.py check` at verify time.

**Start by running the check.** It hands you the finite list; you are not searching a diff for candidates:

```bash
python3 ~/.claude/hooks/comments.py check --json
```

`MISSING` findings are sites that require a comment. `INVALID` findings are comments that break a mechanical rule — fix or remove each. Re-run until it exits 0.

## Every function gets one line. Almost nothing else gets any.

**Two defaults, not one, and they pull in opposite directions on purpose.**

**A function declaration is documented by default.** Public and private alike — a private helper is read more often than the exported wrapper around it. `check` reports an undocumented one as `FUNC_UNDOCUMENTED`, and it fires in every language it knows, not just Go. Exempt: a test file or test helper, a body of one statement (a getter, `String()`, `Error()`), a declaration with no body (an interface method, an abstract signature), and a generated file. A Python docstring or a JSDoc block already satisfies it — do not add a second comment above one.

**Everything else is undocumented by default.** A statement, a branch, a loop, a `var`, a `const`, a struct field: write nothing unless the line cannot say it itself. This is where a comment is the exception you justify.

**The bar, both directions: what the code does, as it stands today.** For a function, that is what it does and what it gives back. Elsewhere it is a standing property the code cannot state itself — a unit, a bound, an ordering requirement, the shape of a workaround.

**Five things are banned regardless of how well they are written:**

- **An external citation.** A version number, an SDK release, a `PRD`/`SDD`/`ADR`, a ticket ID, "as discussed", "per the spec", "this task". `check` reports these as `EXTERNAL_REF` and `apply` refuses them outright. A comment describes the code in front of the reader; it has no access to your conversation, your backlog, or the version you happened to build against, and neither will they.
- **A restatement.** `// increments the counter` above `counter++`, or a comment opening with the identifier below it. **A function's summary is the carve-out** — saying what the function does is the job there. But it must add what the name does not: `// mustGroup makes a group` is the name read back; `// builds a route group, fatalling on a construction error` is a comment.
- **A hypothetical.** "can only mean a programmer mistake", "should never happen", "in theory this could return nil". A comment about a state the code does not reach describes nothing.
- **Call-site reasoning.** "every call site above passes a non-empty prefix", "the only caller already validates this". Callers move; the sentence is wrong the day one does, and the reader cannot check it from here.
- **Point-in-time context.** "raised from 200 after the audit", "replaces the old sweep", "now uses Y instead of Z". Commit-message material — it describes the diff's history, not the code.

The last four are what a justification essay is made of. A comment describes; it does not defend, cite, or reminisce.

**This is what a correctly commented file looks like.** Read it before drafting — it is the target, not an illustration of one rule:

```go
// reads the config from disk and applies the defaults
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data)
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

Every function carries one line. `Name()` carries none — a one-statement body is exempt, and a comment there would only read the name back. `lookup`'s line is the mandatory discriminant case below. Nothing opens with its own identifier, nothing runs past its cap, nothing cites anything outside the file.

**Bad:** `notFoundGet bool // simulates forge-sdk-go v0.36.0+ Get/GetDel returning awsEC.ErrNotFound on a miss`
An SDK and a version pinned into a comment. The version is stale on the next bump and the reader cannot verify it from here — say what the field makes the fake do, not which release it was modelled on.

**Bad:** `// mustGroup fatals on a route-group construction error. Every call site above passes a hardcoded, non-empty prefix onto an already-valid parent group, so an error here can only mean a programmer mistake in this file, not a runtime condition to recover from.`
A name echo, then call-site reasoning, then a hypothetical, spread over three lines. The first clause is the only one with a job, and it needs rewriting to stop opening on the name.

## The line caps

**Two lines above a function. One line above everything else** — a `var`, a `const`, a `type`, or a statement. `check` reports a block over its cap as `OVER_LINES`; `apply` refuses a proposal that would push one over. A machine directive (`//go:generate`, `//nolint`) does not count against the cap.

The second line of a function's block is headroom, not an allowance to fill. A fact that fits on one line takes one line. A fact that does not fit in two is too broad — narrow it to the clause that clears the bar and drop the rest.

## Mandatory: an ambiguous discriminant return

**One case is not a judgment call.** A function — exported or not — returning a trailing `bool` alongside data, or returning three or more values, where the discriminant collapses more than one real state into a signal the signature cannot distinguish, **always** gets a `WHY`/`NOTE` stating what it actually discriminates.

`func (c *secretCache) getParsed(key string) (map[string]string, bool)` is the canonical case: `false` could mean "no such key," "key present but empty," or "key present but failed to parse," and nothing in the name says which. Any reader must open the body to find out — exactly the gap a comment closes.

A trivial `ok`/`found`/`exists` presence check needs nothing extra, and a plain `(T, error)` is idiomatic enough to exempt. This is for the case where the name alone under-specifies the second value. **Do not drop one of these as "self-documenting" — name-only self-documentation is precisely what fails here.** State the states the discriminant collapses, in one line, and stop there; why the function collapses them is not part of the job.

## Unexported is not exempt

`DOC` used to refuse an unexported declaration, which read as a signal that lowercase-first code was beneath documenting. It no longer does: any top-level Go `func`/`type`/`const`/`var` can carry a name-first one-sentence `DOC`, which is what Go codebases actually write and what `FUNC_UNDOCUMENTED` now expects on every function. **Do not skip a function because it is lowercase-first** — a private helper is read more often than the exported wrapper around it.

## Scan the code yourself

**`check`'s findings are a floor, not the whole job.** `MISSING`/`INVALID` only catch what's mechanically decidable; a workaround or constraint living in a function with an ordinary signature is invisible to it. Read every changed function in the diff yourself before drafting, unexported ones included.

**Also scan the touched code for pre-existing `NOTE`/`FIXME`/`TODO`/`HACK` markers this band made stale** — a `TODO` for work just done, a `FIXME` for a bug just fixed, a `NOTE` describing behavior just changed. You have no tool that deletes a comment; name each in `## Notes` so the session agent can remove it.

## Order

1. Run `comments.py check --json`. Every `MISSING` finding must end up commented or explicitly skipped with a written reason; every `INVALID` finding must be fixed or removed.
2. Read the diff, the named `BACKLOG.md` stories, and the drafted commit message for the intent behind each site.
3. Read the diff's own code for sites the check cannot see — a workaround or constraint in a function with an ordinary signature. Unexported functions included.
4. Scan the touched code for stale pre-existing markers this band invalidated.
5. Draft proposals in the exact shape — see [reference.md](reference.md) for the nine template types and their per-type rules.
6. Pipe them to `comments.py apply`.
7. Re-run `comments.py check` and confirm it exits 0.
8. Report what was spliced, what `apply` dropped (its reasons verbatim), and any site you skipped with why.

## The validator has the final word

**Every proposal goes through `apply`'s shape check regardless of how sound the judgment behind it was.** A well-reasoned comment carrying a stray `NOTE:`/`FIXME:`/`TODO:`/`WHY:`/`HACK:` marker it should not, targeting a line the file lacks, echoing the identifier on the next line (`DOC` excepted), or landing within 2 lines of one already spliced gets silently dropped by `apply`. Report every drop from its own `dropped` array verbatim; do not soften or reinterpret its reason.

**One comment per site, one sentence, under 120 characters (80 for `FIELD`), inside the line caps above — mechanical, not advisory.** Do not dodge a cap by writing more, shorter lines that add up to the same restatement, or by pre-trimming a stack to the number of lines you hope survive. A code block gets at most one comment. If a fact does not fit in one sentence, the fact is too broad — narrow it to the clause that clears the bar and drop the rest.

## A deleted comment is judged clause by clause

**A doc comment or banner you are restoring after a deletion often bundles more than one fact.** Judge each clause against the bar independently: design rationale, history, and hypotheticals do not come back, while a correctness rule the type does not enforce in code (an embedded `sync.Map` that must never be copied after first use — the compiler will not catch a copy) is a standing property and does. **Restoring one clause never licenses dropping another that independently clears the bar.**

**The line caps apply to a restoration too.** If two surviving clauses cannot both fit the block's cap, one of them was never the more important — splice that one, and name the other in `## Notes`. Do not restore a four-line original as four lines because it was four lines before.

**Never claim a comment was "restored" without diffing your spliced text against the original.** This failure is real and has happened: reporting "restored the substantive parts" when only one clause of a multi-clause original landed. Before writing your report, re-read the original side by side with what actually spliced, clause by clause.

Your standing rules on scope, craft, and stopping already apply. Nothing here overrides them.

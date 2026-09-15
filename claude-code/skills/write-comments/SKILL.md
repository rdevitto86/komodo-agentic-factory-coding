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

## Default: write nothing

Code should be self-documenting. A comment is the exception you justify, not the default you reach for. Most bands produce zero or very few proposals. A comment earns its place only when it records something the code cannot say by itself:

- **A non-obvious workaround.** The code looks wrong, or looks like it does more than it needs to, for a reason living outside the diff — a library bug, a platform quirk, an ordering requirement invisible from the surrounding lines.
- **A rejected alternative worth recording.** Someone reading this in six months will reach for the obvious-looking fix that was already tried and abandoned; the comment is what stops them re-treading it.
- **A discovered constraint.** A limit, invariant, or assumption the implementation depends on that is stated nowhere else — an API's undocumented rate limit, a data shape guaranteed upstream, a value range the caller must already enforce, a correctness rule the type itself does not enforce (a copy-after-use hazard, a lock-ordering requirement, a must-call-before-use step).

None of these is "what this function does," "what this variable holds," or a restatement of a name already in the code. That is narrative, and narrative is banned regardless of phrasing.

**Also banned regardless of phrasing: narrating the change itself.** "Replaces X pattern," "used to be duplicated in two callers," "now uses Y instead of Z" — commit-message material, not code comments. They describe the diff's history, not a standing property of the code, and confuse the next reader who has no memory of what "replaces" refers to.

**Good:** `// retries with backoff -- the upstream API returns 429 with no Retry-After header, so a fixed delay is the only signal we have`
Records a constraint that explains a design choice invisible from the retry loop itself.

**Bad:** `// increments the counter` above `counter++`
Restates code that already says what it does.

**Bad:** `// NOTE: this function validates the input` above `func validateInput(...)`
Name echo — the comment repeats what the identifier already tells the reader. `DOC` is the one type exempt, since it is required to start with the name it documents.

**Bad:** `// replaces a TTL+sweep pattern once duplicated, with drifting clocks, in two callers`
A refactor narrated as a comment. Those callers may not exist by the time this is read.

## Mandatory: an ambiguous discriminant return

**One case is not a judgment call.** A function — exported or not — returning a trailing `bool` alongside data, or returning three or more values, where the discriminant collapses more than one real state into a signal the signature cannot distinguish, **always** gets a `WHY`/`NOTE` stating what it actually discriminates.

`func (c *secretCache) getParsed(key string) (map[string]string, bool)` is the canonical case: `false` could mean "no such key," "key present but empty," or "key present but failed to parse," and nothing in the name says which. Any reader must open the body to find out — exactly the gap a comment closes.

A trivial `ok`/`found`/`exists` presence check needs nothing extra, and a plain `(T, error)` is idiomatic enough to exempt. This is for the case where the name alone under-specifies the second value. **Do not drop one of these as "self-documenting" — name-only self-documentation is precisely what fails here.**

## Unexported is not exempt

`DOC`'s exported-only gate is a fact about `DOC` specifically — a mechanical rule tied to godoc conventions, not a signal that unexported code is beneath judgment. A large unexported function handling a real subtask (parsing an untrusted format, a multi-step validation pipeline) gets the same `WHY`/`HACK` scrutiny as exported code. **Do not default to zero proposals for a function because it is lowercase-first — judge it on what it does, not its casing.**

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

**One comment per site, one sentence, under 120 characters (80 for `FIELD`) — mechanical, not advisory.** Do not dodge either cap by writing more, shorter lines that add up to the same restatement, or by pre-trimming a stack to two or three lines you hope survive. A code block gets at most one comment. If a fact does not fit in one sentence, the fact is too broad — narrow it to the clause that clears the bar and drop the rest.

## A deleted comment is judged clause by clause

**A doc comment or banner you are restoring after a deletion often bundles more than one fact.** One clause might be design rationale, another a correctness invariant the type does not enforce in code (an embedded `sync.Map` that must never be copied after first use is a real example — the compiler will not catch a copy). Judge each clause against the bar independently. **Restoring one clause never licenses dropping another that independently clears it.** When a clause is a safety rule and you are not certain it is redundant with something the code now states elsewhere, keep it — losing a real invariant is worse than keeping one sentence too many.

**Never claim a comment was "restored" without diffing your spliced text against the original.** This failure is real and has happened: reporting "restored the substantive parts" when only one clause of a multi-clause original landed. Before writing your report, re-read the original side by side with what actually spliced, clause by clause.

Your standing rules on scope, craft, and stopping already apply. Nothing here overrides them.

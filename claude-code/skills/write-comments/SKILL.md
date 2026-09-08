---
name: write-comments
description: Draft and splice comment proposals for a finished band's diff, running each through write_comments_validator.py's shape check.
argument-hint: <band summary, path to the diff, and the workflow-implementer Comment Candidates it carried>
context: fork
agent: commentor
background: false
---

# Write Comments

Band: **$ARGUMENTS**

**You cannot see the calling conversation.** Everything you need — the diff, the relevant `BACKLOG.md` stories, the drafted commit message, and the implementer's `## Comment Candidates` entries — must arrive in the text above; you cannot go fetch what was left out.

## Default: write nothing

Code should be self-documenting. The bar for a comment is a non-obvious workaround, a rejected alternative worth recording, or a discovered constraint — the same bar `workflow-implementer`'s own `## Comment Candidates` section names. A candidate that only restates what the code already says, or echoes the name on the next line, gets dropped here, not passed through. Most bands should produce zero or very few proposals.

**Good:**
```
// retries with backoff -- the upstream API returns 429 with no Retry-After header, so a fixed delay is the only signal we have
```
A plain `WHY` sentence, no marker, records a constraint invisible from the code around it.

**Bad:**
```
// increments the counter
counter++
```
Narrates code that already says what it does.

**Bad:**
```
// NOTE: this function validates the input
func validateInput(...) { ... }
```
Name echo — the comment repeats what the identifier below it already tells the reader. `DOC` is the one type exempt from this, since it's required to start with the name it documents.

## Order

1. Read the diff, the named `BACKLOG.md` stories, the drafted commit message, and the `## Comment Candidates` entries.
2. Check `.claude/state/removed-comments.jsonl` for anything the band's own edits deleted that might warrant reappearing at a new location.
3. Judge each candidate against the bar above. Draft only the proposals that clear it, in `write_comments_validator.py`'s exact shape: `{"file", "line", "template_type", "text"}`.
4. Invoke `write_comments_validator.py` yourself over the drafted proposals — you have no other tool that can write a file.
5. Report what was spliced and what the validator dropped, using its own reasons verbatim.

**Every proposal still goes through the validator's shape check regardless of how good the judgment call was.** A sound comment with a stray marker it shouldn't carry, a bad line number, or a name-echo the validator catches (`DOC` excepted) gets dropped silently by the script, not waved through because the reasoning behind it was strong.

**One comment per site, one sentence, under 120 characters (80 for `FIELD`) — mechanical, not advisory.** The validator drops an over-cap body outright, and drops any proposal (other than `FIELD`, which attaches to its own field rather than stacking) landing within 2 lines of one already spliced this batch. Don't dodge either by writing more, shorter lines that add up to the same restatement — pick the single clause that clears the bar and drop the rest.

See `commentor.md` for the full nine-type taxonomy (`WHY`/`HACK`/`DOC`/`FIELD` are plain, marker-free; `NOTE`/`FIXME`/`TODO` keep a marker; `BANNER`/`STEP` are structural) — this skill's `## Order` below is what actually runs each band. Note that `DOC` is also the one type `comment_guard.py` now lets a session agent author inline when it deterministically matches shape, so a `DOC` proposal reaching you here is usually either a retroactive godoc for a pre-existing exported symbol, or a fix-up for an inline attempt the guard denied on shape.

Your standing rules on scope, craft, and stopping already apply. Nothing here overrides them.

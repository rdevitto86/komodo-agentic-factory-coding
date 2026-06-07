# Config Writing Style

How to write the prose in this repo's agent directives, standards, modes, and skill docs. Goal: maximum signal per token, still human-readable. Tokens run against a shared subscription — every directive loads into an agent's context.

## The carve-out — read first

**Never apply terseness to rules whose meaning rides on a function word** — "only / never / unless / before / except / iff". Keep these in full natural language:

- the hard rules (`CLAUDE.md`, `principles.md` §1)
- `comments.md` (every line)
- error-string format
- any sentence where dropping "only" or "never" would widen or invert the rule

A dropped function word in a rule is a violation waiting to happen. The 10% of tokens saved there is the most expensive 10% in the repo. When in doubt, leave the rule verbose.

## Everywhere else — telegraphic but grammatical

Applies to role intros, descriptions, process explanations, background, and bullets.

- **Lead bullets with an imperative verb.** "Pass the file path", not "You should pass the file path".
- **Cut filler.** Drop "you should", "in order to", "it is important to note that", "make sure to", "please". Cut adverbs that add no constraint ("simply", "just", "basically", "generally").
- **Tables and bullets over paragraphs.** Structured lists tokenize tighter than prose and parse better — for human and model.
- **One claim per line.** Don't chain three ideas through "and … which … so that".
- **No restating the heading** in the first sentence under it.
- **Keep grammar intact.** This is terse English, not dropped articles. Don't remove "the/a/only/never" — that crosses into ambiguity. Sentences still read as sentences.

## Test

Read it aloud. If it sounds like a clear instruction from one engineer to another, it's right. If it sounds like a telegram missing words, you went too far. If it sounds like a memo with throat-clearing, you didn't go far enough.

# 0005. The guard reads a command through one shell lexer

**Status:** Accepted, 2026-09-23. Supersedes the ad hoc splitters in place since TG-03.3.

## Context

The guard split a command with six separate scanners: one for chains, one for background jobs, one for words, one for redirects, one for substitutions, and one for heredocs. Each tracked quotes on its own terms. Two review rounds found thirteen bypasses in the gaps between them. Seven more were open on 2026-09-23:

- **An escaped quote.** `echo \"; git push origin main; echo \"` hid the push inside a string the chain splitter believed was open.
- **ANSI-C quoting.** `$'git' push origin main` spelled a command the word splitter never recognised.
- **A brace list.** `{git,push,origin,main}` expanded to a push.
- **A variable set on the line.** `G=git; $G push origin main`.
- **A glob.** `/usr/bin/gi? push origin main`.
- **A `>&` file.** `echo x >&../outside.txt` wrote outside the worktree.
- **A here-string into a shell.** `bash <<< "git push origin main"`.

## Decision

One lexer walks the command once, the way a POSIX shell tokenizes. It yields words with a quote mark on every rune, control operators, and redirects. It reads heredoc bodies at the newline that follows them and collects every substitution body as it goes. A parser groups the tokens into simple commands with their redirect targets, their stdin, and whether they pipe onward. The scanner then applies the same rules as before to each command. It also expands brace lists, substitutes variables the line itself set, resolves a globbed command name, follows stdin into a shell, and caps nesting at twelve levels.

A fuzz target checks two properties on every input. The guard never panics or disagrees with itself, and a harmless prefix such as `true; ` never turns a denial into an allow. The pre-push hook runs it.

## Alternatives

- **Patch each scanner again.** Every patch so far opened the next gap, because the scanners disagreed about quoting.
- **A full shell parser dependency.** It would break the standard-library-only rule, and a guard still needs its own rules on top.

## Consequences

- **Quoting is decided once.** A new rule reads words and operators, never raw text.
- **Some false denials went away.** A `#` comment and an `&` inside a quoted message no longer deny. Neither does a sed script that starts with `/`.
- **It is still not a sandbox.** Runtime expansion the line cannot see stays out of reach, such as a variable set in another process. Decision 0004 still holds.

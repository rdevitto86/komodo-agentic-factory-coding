# 0030. A group PR's cap counts kept files and added lines, never deletions

**Status:** Accepted, 2026-09-26. Amends 0028.

**Context.** TG-06.2 cuts the guard to five rules: 24 files, 531 lines added and 4,388 deleted. Decision 0028's cap summed both, so it refused a change that is mostly removal. A reviewer reads an added line; a deleted file or line costs almost nothing to read.

**Decision.**

- **The file cap counts files a diff keeps;** a deleted file is free.
- **The line cap and the preferred note count added lines only.**
- **The ledger row still stamps every changed line,** so the true size stays on record.

**Alternatives.**

- **Keep the cap on every changed line.** Forces a mass deletion to split into parts that each leave the code broken.
- **Let a group declare its own cap.** A builder could raise the cap it is judged by.

**Consequences.**

- **A mass deletion ships as one PR,** so an unbounded removal passes the cap; review still reads its file list.
- **The SDLC standard states the cap as kept files and added lines.**

# README maintenance

After a large or structural change, update the project's README in the same change set, as the last step.

**What counts as large:**
- A new or removed top-level module, directory, or service
- A new public API or entry-point surface
- An architecture or layout shift
- A new or removed major dependency
- Changed setup, build, or run steps
- In a config repo (like komodo-claude-core): adding, removing, or renaming an agent or standard

**What to keep accurate:** structure/layout, setup steps, entry points, and the high-level "what this is." The README is a quick-start/orientation doc — deep detail lives in `CLAUDE.md`, agent files, or `/docs/`. Do not expand it into exhaustive documentation.

**Trivial changes do not trigger a README update:** a bugfix, a single-file edit, or an internal refactor with no surface change.

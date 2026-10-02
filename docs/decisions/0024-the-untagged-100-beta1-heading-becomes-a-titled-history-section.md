# 0024. The untagged `1.0.0-beta.1` heading becomes a titled history section

**Status:** Accepted, 2026-09-26. Amends 0023.

**Context.** Decision 0023 kept the old `1.0.0-beta.1` heading and started V1 at `1.0.0-alpha.5`. SemVer sorts alpha.5 below beta.1, so the first V1 ship put a newer heading above an older one. `komodo doctor`'s changelog check refused it, and TG-04.1 stopped before its pull request.

**Decision.** The heading is retitled "The first 1.0.0 line", with no version, and its section says it was drafted as `1.0.0-beta.1` and never tagged. V1 keeps 0023's numbering: `1.0.0-alpha.5` onward, then `1.0.0-beta.2`, then `1.0.0`.

**Alternatives.**

- **Skip untagged headings in the order check.** It weakens the check, and the changelog would still read beta.1 as newer than alpha.5.
- **Renumber V1 above beta.1.** Every group would change version, and `beta.2` as the first V1 release would hide the alphas.

**Consequences.**

- **The changelog's version headings are in SemVer order again.** Every tag still has a heading.
- **`1.0.0-beta.1` is never used,** so no tag or heading can collide with it.

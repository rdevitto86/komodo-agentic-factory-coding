---
name: release
description: Cut a release: version, changelog, tag, binaries and checksums.
---

# Release

You drive one release from the orchestrator, on the owner's machine. The binary does the work; you run its commands in order and stop at the first non-zero exit.

## The version

- **Every version is SemVer with a phase:** alpha, beta, optional rc, then stable, as the README's Versions section defines.
- **Alpha, `x.y.z-alpha.n`,** while the shape still moves; the next alpha raises `n` by one.
- **Beta, `x.y.z-beta.n`,** once `x.y.z` is feature-complete; only fixes land. V1's beta starts at `1.0.0-beta.2`; `1.0.0-beta.1` is never used.
- **Rc, `x.y.z-rc.n`,** only when the human asks for one; nothing requires it.
- **Stable, `x.y.z`,** is cut by the human alone, once the PRD's success criteria hold. Never propose it on your own.
- **The bump lives in the backlog.** Every group in an epic carries the epic's `version:`; change it there, then run `komodo lint`. An epic with an open branch runs `komodo rephase <epic> <new-version>` instead of a bare edit, so its branch and pull requests move with it.
- **Picking the segment and phase is the backlog rule's Choosing a version section,** not a guess at release time.

## The changelog

- **A heading is `## <version> — <YYYY-MM-DD>`,** with no `v` and no brackets, newest first, in SemVer order.
- **A version is never reused.** Two headings never name one version, and a titled history section carries no version.
- **Shipped groups write fragments under `changelog.d/`.** You never hand-edit a version's lines.

## The steps

1. On a branch, never the default one, run `komodo release fold`. It writes every fragment into `CHANGELOG.md` under its heading. Open a pull request; the human merges it.
2. On the default branch, up to date with origin, run `komodo release check`. Zero drift, or stop and report each line.
3. Ask the human before tagging. Then run `komodo tag`; it tags the newest untagged version at HEAD and pushes it.
4. Ask the human before publishing. Then run `komodo release publish` at the tag. It builds every platform into `dist/`, runs the tests, writes `SHA256SUMS`, and publishes the release. It prints the release URL.

## Rules

- **Only the owner's machine publishes.** A line session refuses `komodo release publish`; never spawn an agent to run it.
- **The forge credential is the publish step's alone.** Never read it, print it, or pass it to another command.
- **Nothing runs on the forge.** There is no CI to wait for; the tests ran in step 4.
- **A refusal is the answer.** A dirty tree, a tag off HEAD, or a failed test stops the release. Report the command and its output; never retag, force-push, or skip the tests.
- **Product repos pin a published release.** After publishing, the human bumps the pin; `komodo doctor` fails on any other.
- **Report in the accessibility contract** when the release ends: the version, the tag, and the URL.

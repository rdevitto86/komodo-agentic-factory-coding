import time
import unittest

from komodo import render, state


def make_state(**overrides):
    """A RunState with sane defaults, fields overridable per test."""
    fields = {
        "run_id": "20260101-000000-tg-01-1", "group_id": "TG-01.1", "branch": "feat/x",
        "base": "main", "profile": "fast", "started": time.time() - 62,
    }
    fields.update(overrides)
    return state.RunState(**fields)


class BulletsTests(unittest.TestCase):
    def test_strips_blank_and_whitespace_items(self):
        self.assertEqual(render.bullets(["a", "", "  ", "b"]), ["- a", "- b"])

    def test_under_cap_lists_every_item(self):
        self.assertEqual(render.bullets(["a", "b"], cap=5), ["- a", "- b"])

    def test_over_cap_collapses_the_rest(self):
        out = render.bullets(["a", "b", "c", "d", "e", "f"], cap=3)
        self.assertEqual(out, ["- a", "- b", "- and 4 more"])


class ClipSentenceTests(unittest.TestCase):
    def test_short_sentence_is_unchanged(self):
        self.assertEqual(render.clip_sentence("short and sweet"), "short and sweet")

    def test_long_sentence_is_cut_with_ellipsis(self):
        text = " ".join("word%d" % i for i in range(25))
        out = render.clip_sentence(text, words=5)
        self.assertEqual(out, "word0 word1 word2 word3 word4...")

    def test_trailing_punctuation_is_stripped_before_ellipsis(self):
        text = "one two three,"
        self.assertEqual(render.clip_sentence(text, words=2), "one two...")


class DurationTests(unittest.TestCase):
    def test_under_a_minute(self):
        self.assertEqual(render.duration(42), "42s")

    def test_over_a_minute(self):
        self.assertEqual(render.duration(125), "2m 05s")


class PhaseTableTests(unittest.TestCase):
    def test_rows_span_consecutive_phase_starts(self):
        run = make_state()
        run.mark_phase("preflight")
        run.phases["preflight"] = run.started
        run.mark_phase("build")
        run.phases["build"] = run.started + 10
        run.finished = run.started + 30
        rows = render.phase_table(run)
        self.assertEqual(rows[0], "| Phase | Time |")
        self.assertIn("| preflight | 10s |", rows)
        self.assertIn("| build | 20s |", rows)


class WorkerTableTests(unittest.TestCase):
    def test_aggregates_calls_and_cost_by_role(self):
        run = make_state()
        run.workers.append(state.WorkerRecord(role="builder", task_id="TSK-01.1.1", provider="claude", model="sonnet", ok=True, seconds=1.0, cost_usd=0.5, input_tokens=10, output_tokens=20))
        run.workers.append(state.WorkerRecord(role="builder", task_id="TSK-01.1.2", provider="claude", model="sonnet", ok=True, seconds=1.0, cost_usd=0.25, input_tokens=5, output_tokens=5))
        rows = render.worker_table(run)
        self.assertIn("| builder | 2 | $0.75 |", rows)


class SummaryBucketsTests(unittest.TestCase):
    def test_empty_state_has_no_buckets(self):
        run = make_state()
        self.assertEqual(render.summary_buckets(run, {}), [])

    def test_done_blocked_and_flagged_buckets(self):
        run = make_state()
        run.task("TSK-01.1.1").status = "DONE"
        run.task("TSK-01.1.2").status = "BLOCKED"
        run.task("TSK-01.1.2").note = "missing fixture"
        run.notes.append("watch the cache TTL")
        titles = {"TSK-01.1.1": "Add refund handler", "TSK-01.1.2": "Wire refund route"}
        out = render.summary_buckets(run, titles)
        text = "\n".join(out)
        self.assertIn("## ✅ Successful Changes", text)
        self.assertIn("Add refund handler", text)
        self.assertIn("## ❌ Blocked Changes", text)
        self.assertIn("missing fixture", text)
        self.assertIn("## ⚠️ Flagged Changes", text)
        self.assertIn("watch the cache TTL", text)


class ReportTests(unittest.TestCase):
    def test_complete_run_report(self):
        run = make_state()
        run.task("TSK-01.1.1").status = "DONE"
        run.phases["build"] = run.started
        run.finished = run.started + 10
        text = render.report(run, "Refunds", {"TSK-01.1.1": "Add refund handler"}, ["abc123 add handler"])
        self.assertIn("# TG-01.1: Refunds", text)
        self.assertIn("**Run complete**", text)
        self.assertIn("## Commits", text)
        self.assertIn("abc123 add handler", text)
        self.assertIn("## Timing", text)
        self.assertTrue(text.endswith("\n"))
        self.assertFalse(text.endswith("\n\n"))

    def test_blocked_run_and_blast_radius(self):
        run = make_state(blast_radius="high", blast_radius_why="touches auth")
        run.blocked = ["TSK-01.1.1"]
        run.finished = run.started + 5
        text = render.report(run, "Refunds", {}, [])
        self.assertIn("**Run blocked**", text)
        self.assertIn("**Blast radius high.** touches auth", text)


class PrBodyTests(unittest.TestCase):
    def test_default_shape_with_blocked_and_findings(self):
        run = make_state()
        run.task("TSK-01.1.1").status = "DONE"
        run.task("TSK-01.1.2").status = "BLOCKED"
        run.findings = [{"title": "n+1 query", "fixed": True}, {"title": "dead code", "filed": True}]
        body = render.pr_body(run, "Refunds", {"TSK-01.1.1": "Add refund handler", "TSK-01.1.2": "Wire refund route"}, ["abc123 add handler"])
        self.assertIn("## Summary", body)
        self.assertIn("1 of 2 tasks landed", body)
        self.assertIn("## Blocked", body)
        self.assertIn("Wire refund route", body)
        self.assertIn("1 finding(s) fixed in-branch, 1 filed", body)

    def test_template_with_summary_and_changes_headings_is_filled_in_place(self):
        run = make_state()
        run.task("TSK-01.1.1").status = "DONE"
        template = "## Summary\n\n## Changes\n\n## Footer\n\nkeep me\n"
        body = render.pr_body(run, "Refunds", {"TSK-01.1.1": "Add refund handler"}, [], template=template)
        self.assertIn("Add refund handler", body)
        self.assertIn("## Footer", body)
        self.assertIn("keep me", body)

    def test_template_heading_with_no_content_is_dropped(self):
        run = make_state()
        run.task("TSK-01.1.1").status = "DONE"
        template = "## Summary\n\n## Changes\n\n## Footer\n"
        body = render.pr_body(run, "Refunds", {"TSK-01.1.1": "Add refund handler"}, [], template=template)
        self.assertNotIn("## Footer", body)

    def test_template_missing_summary_heading_falls_back(self):
        run = make_state()
        body = render.pr_body(run, "Refunds", {}, [], template="no headings here")
        self.assertTrue(body.startswith("## Summary"))


class ChangelogTests(unittest.TestCase):
    def test_entry_clips_each_title(self):
        entries = render.changelog_entry("fix", ["short title"])
        self.assertEqual(entries, ["- short title"])

    def test_heading_maps_known_and_unknown_kinds(self):
        self.assertEqual(render.changelog_heading("fix"), "Fixed")
        self.assertEqual(render.changelog_heading("feat"), "Added")
        self.assertEqual(render.changelog_heading("refactor"), "Changed")
        self.assertEqual(render.changelog_heading("mystery"), "Changed")



TEMPLATE = """<!--
Title: <type>: <summary>
-->

## Summary

<!-- One or two sentences. -->

## Changes

<!-- One bullet per area. -->

- **<area>** - <what changed>

## Validation

<!-- Only what a green CI run cannot show. -->

## Dependencies

<!-- Omit the section entirely if there are none. -->
"""


class PrBodyRepoTemplateTests(unittest.TestCase):
    def setUp(self):
        self.titles = {"TSK-01.1.1": "do the thing"}

    def done_run(self, **overrides):
        """A run with one done task, plus whatever a case overrides."""
        run = make_state(**overrides)
        run.task("TSK-01.1.1").status = "DONE"
        return run

    def test_comments_and_placeholders_are_stripped(self):
        body = render.pr_body(self.done_run(), "Group", self.titles, [], TEMPLATE)
        self.assertNotIn("<!--", body)
        self.assertNotIn("<area>", body)

    def test_empty_section_is_dropped(self):
        body = render.pr_body(self.done_run(), "Group", self.titles, [], TEMPLATE)
        self.assertNotIn("## Dependencies", body)

    def test_validation_is_filled_in_the_template_path(self):
        run = self.done_run(blast_radius="high", blast_radius_why="crosses a trust boundary")
        body = render.pr_body(run, "Group", self.titles, [], TEMPLATE)
        self.assertIn("## Validation", body)
        self.assertIn("done_when", body)
        self.assertIn("Blast radius **high**", body)

    def test_a_section_the_template_lacks_is_appended(self):
        run = self.done_run()
        run.task("TSK-01.1.2").status = "BLOCKED"
        titles = dict(self.titles, **{"TSK-01.1.2": "the blocked one"})
        body = render.pr_body(run, "Group", titles, [], TEMPLATE)
        self.assertIn("## Blocked", body)
        self.assertIn("the blocked one", body)


class CutReleaseTests(unittest.TestCase):
    TEXT = "# Changelog\n\n## [Unreleased]\n\n### Added\n- a thing\n\n## [1.0.0] \u2014 2026-01-01\n\n### Added\n- the start\n"

    def test_bumps_each_component(self):
        self.assertEqual(render.next_version("1.0.0", "major"), "2.0.0")
        self.assertEqual(render.next_version("1.2.3", "minor"), "1.3.0")
        self.assertEqual(render.next_version("1.2.3", "patch"), "1.2.4")

    def test_short_version_is_padded(self):
        self.assertEqual(render.next_version("2", "minor"), "2.1.0")

    def test_unreleased_becomes_a_dated_version(self):
        out = render.cut_release(self.TEXT, "1.1.0", "2026-09-16")
        self.assertIn("## [1.1.0] \u2014 2026-09-16", out)
        self.assertIn("- a thing", out)
        self.assertIn("## [1.0.0]", out)

    def test_a_fresh_unreleased_section_is_opened(self):
        out = render.cut_release(self.TEXT, "1.1.0", "2026-09-16")
        lines = [line for line in out.splitlines() if line.startswith("## [")]
        self.assertEqual(lines[0], "## [Unreleased]")
        self.assertEqual(lines[1], "## [1.1.0] \u2014 2026-09-16")

    def test_empty_unreleased_is_refused(self):
        with self.assertRaises(ValueError):
            render.cut_release("# Changelog\n\n## [Unreleased]\n\n## [1.0.0] \u2014 2026-01-01\n\n- old\n", "1.1.0", "2026-09-16")

    def test_missing_unreleased_heading_is_refused(self):
        with self.assertRaises(ValueError):
            render.cut_release("# Changelog\n\n## [1.0.0] \u2014 2026-01-01\n", "1.1.0", "2026-09-16")


class InferBumpTests(unittest.TestCase):
    def _text(self, body):
        return "# Changelog\n\n## [Unreleased]\n\n%s\n## [1.0.0] \u2014 2026-01-01\n\n### Added\n- the start\n" % body

    def test_only_fixes_is_a_patch(self):
        level, _ = render.infer_bump(self._text("### Fixed\n- a crash\n"))
        self.assertEqual(level, "patch")

    def test_changed_alone_is_a_patch(self):
        level, _ = render.infer_bump(self._text("### Changed\n- tighter output\n"))
        self.assertEqual(level, "patch")

    def test_added_is_a_minor(self):
        level, _ = render.infer_bump(self._text("### Fixed\n- a crash\n\n### Added\n- a command\n"))
        self.assertEqual(level, "minor")

    def test_removed_is_a_major(self):
        level, reason = render.infer_bump(self._text("### Added\n- a command\n\n### Removed\n- an old flag\n"))
        self.assertEqual(level, "major")
        self.assertIn("Removed", reason)

    def test_a_breaking_bullet_is_a_major(self):
        level, _ = render.infer_bump(self._text("### Changed\n- breaking: the state file moved\n"))
        self.assertEqual(level, "major")

    def test_a_released_section_below_is_ignored(self):
        level, _ = render.infer_bump(self._text("### Fixed\n- a crash\n"))
        self.assertEqual(level, "patch")


if __name__ == "__main__":
    unittest.main()

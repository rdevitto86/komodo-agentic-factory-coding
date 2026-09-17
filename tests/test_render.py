import time
import unittest

from komodo import render
from komodo.state import RunState, TaskRecord

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


def state_with(**overrides):
    """A run state carrying one done task, plus whatever a case overrides."""
    state = RunState(run_id="r", group_id="TG-01.1", branch="b", base="main", profile="fast", started=time.time())
    state.tasks = {"TSK-01.1.1": TaskRecord(id="TSK-01.1.1", status="DONE")}
    for key, value in overrides.items():
        setattr(state, key, value)
    return state


class PrBodyTemplateTests(unittest.TestCase):
    def setUp(self):
        self.titles = {"TSK-01.1.1": "do the thing"}

    def test_template_comments_and_placeholders_are_stripped(self):
        body = render.pr_body(state_with(), "Group", self.titles, [], TEMPLATE)
        self.assertNotIn("<!--", body)
        self.assertNotIn("<area>", body)

    def test_empty_template_section_is_dropped(self):
        body = render.pr_body(state_with(), "Group", self.titles, [], TEMPLATE)
        self.assertNotIn("## Dependencies", body)

    def test_validation_is_filled_in_the_template_path(self):
        state = state_with(blast_radius="high", blast_radius_why="crosses a trust boundary")
        body = render.pr_body(state, "Group", self.titles, [], TEMPLATE)
        self.assertIn("## Validation", body)
        self.assertIn("done_when", body)
        self.assertIn("Blast radius **high**", body)

    def test_a_section_the_template_lacks_is_appended(self):
        state = state_with()
        state.tasks["TSK-01.1.2"] = TaskRecord(id="TSK-01.1.2", status="BLOCKED")
        titles = dict(self.titles, **{"TSK-01.1.2": "the blocked one"})
        body = render.pr_body(state, "Group", titles, [], TEMPLATE)
        self.assertIn("## Blocked", body)
        self.assertIn("the blocked one", body)

    def test_fallback_path_has_every_section(self):
        state = state_with(blast_radius="low", blast_radius_why="isolated")
        body = render.pr_body(state, "Group", self.titles, [])
        for heading in ("## Summary", "## Changes", "## Validation"):
            self.assertIn(heading, body)


if __name__ == "__main__":
    unittest.main()

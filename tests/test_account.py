import json
import subprocess
import unittest
from unittest import mock

from komodo import account

SUBSCRIPTION = {
    "loggedIn": True, "authMethod": "claude.ai", "apiProvider": "firstParty",
    "email": "someone@example.com", "orgId": "an-org-uuid", "subscriptionType": "pro",
}
API_KEY = {"loggedIn": True, "authMethod": "apiKey", "apiProvider": "firstParty", "apiKeySource": "env"}


class ParseTests(unittest.TestCase):
    def setUp(self):
        account.reset_cache()

    def test_a_subscription_is_metered_in_tokens(self):
        parsed = account.parse_status(json.dumps(SUBSCRIPTION))
        self.assertEqual(parsed.plan, "pro")
        self.assertEqual(parsed.metered, account.TOKENS)
        self.assertFalse(parsed.caps_dollars)

    def test_an_api_key_is_metered_in_dollars(self):
        parsed = account.parse_status(json.dumps(API_KEY))
        self.assertEqual(parsed.plan, account.UNKNOWN)
        self.assertEqual(parsed.metered, account.DOLLARS)
        self.assertTrue(parsed.caps_dollars)

    def test_bedrock_and_vertex_bill_a_card(self):
        for provider in ("bedrock", "vertex"):
            parsed = account.parse_status(json.dumps({"loggedIn": True, "authMethod": "", "apiProvider": provider}))
            self.assertEqual(parsed.metered, account.DOLLARS, provider)

    def test_an_unverified_subscription_name_is_unknown_not_an_error(self):
        parsed = account.parse_status(json.dumps(dict(SUBSCRIPTION, subscriptionType="something_new")))
        self.assertEqual(parsed.plan, account.UNKNOWN)
        self.assertEqual(parsed.metered, account.TOKENS)

    def test_unparseable_output_falls_back_to_keeping_the_dollar_cap(self):
        for text in ("", "not json", "[]", "null"):
            parsed = account.parse_status(text)
            self.assertFalse(parsed.detected, text)
            self.assertEqual(parsed.metered, account.UNKNOWN, text)
            self.assertTrue(parsed.caps_dollars, text)

    def test_no_identifying_field_reaches_the_description(self):
        described = account.parse_status(json.dumps(SUBSCRIPTION)).describe()
        self.assertNotIn("someone@example.com", described)
        self.assertNotIn("an-org-uuid", described)
        self.assertIn("pro", described)


class DetectTests(unittest.TestCase):
    def setUp(self):
        account.reset_cache()

    def tearDown(self):
        account.reset_cache()

    def test_a_missing_binary_is_not_a_failed_run(self):
        with mock.patch("shutil.which", return_value=None):
            self.assertFalse(account.detect("claude").detected)

    def test_a_non_zero_exit_is_not_a_failed_run(self):
        completed = subprocess.CompletedProcess(["claude"], 1, stdout="", stderr="nope")
        with mock.patch("shutil.which", return_value="/bin/claude"), mock.patch("subprocess.run", return_value=completed):
            self.assertFalse(account.detect("claude").detected)

    def test_a_timeout_is_not_a_failed_run(self):
        with mock.patch("shutil.which", return_value="/bin/claude"), mock.patch("subprocess.run", side_effect=subprocess.TimeoutExpired("claude", 1)):
            self.assertFalse(account.detect("claude").detected)

    def test_the_probe_runs_once_per_process(self):
        completed = subprocess.CompletedProcess(["claude"], 0, stdout=json.dumps(SUBSCRIPTION), stderr="")
        with mock.patch("shutil.which", return_value="/bin/claude"), mock.patch("subprocess.run", return_value=completed) as run:
            first, second = account.detect("claude"), account.detect("claude")
        self.assertEqual(run.call_count, 1)
        self.assertEqual(first.plan, second.plan)


class LimitTests(unittest.TestCase):
    def test_turns_follow_the_measured_quadratic(self):
        for plan in ("pro", "team", "max", "enterprise"):
            turns = account.turns_for(plan, "standard")
            self.assertAlmostEqual(account.TURN_COST_TOKENS * turns * turns, account.input_budget(plan, "standard"), delta=account.input_budget(plan, "standard") * 0.05)

    def test_a_bigger_plan_gets_materially_more_headroom(self):
        self.assertGreater(account.turns_for("max", "standard"), account.turns_for("pro", "standard") * 1.5)
        self.assertGreater(account.turns_for("pro", "standard"), account.turns_for("pro", "heavy"))
        self.assertGreater(account.turns_for("pro", "heavy"), account.turns_for("pro", "light"))

    def test_an_undetected_account_gets_the_conservative_budget(self):
        self.assertEqual(account.turns_for(account.UNKNOWN, "standard"), account.turns_for("pro", "standard"))

    def test_timeout_scales_with_the_task_and_the_plan_up_to_a_ceiling(self):
        self.assertEqual(account.timeout_for(900, "pro", 0), 900)
        self.assertGreater(account.timeout_for(900, "pro", 240_000), 900)
        self.assertGreater(account.timeout_for(900, "max", 240_000), account.timeout_for(900, "pro", 240_000))
        self.assertLessEqual(account.timeout_for(900, "enterprise", 10_000_000, ceiling_s=1800), 1800)

    def test_the_model_ceiling_follows_the_plan(self):
        self.assertEqual(account.capped_model("opus", "pro"), "sonnet")
        self.assertEqual(account.capped_model("opus", account.UNKNOWN), "sonnet")
        self.assertEqual(account.capped_model("opus", "max"), "opus")
        self.assertEqual(account.capped_model("haiku", "pro"), "haiku")
        self.assertEqual(account.capped_model("claude-opus-5", "pro"), "sonnet")
        self.assertEqual(account.capped_model("qwen3:1.7B", "pro"), "qwen3:1.7B")


if __name__ == "__main__":
    unittest.main()

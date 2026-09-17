import unittest

from komodo import yamlite


class LoadsTests(unittest.TestCase):
    def test_scalars_and_inline_list(self):
        data = yamlite.loads('files: [a/b.go, "c d.go"]\nowner: agent\ncount: 3\nflag: true\nnothing: null\n')
        self.assertEqual(data["files"], ["a/b.go", "c d.go"])
        self.assertEqual(data["owner"], "agent")
        self.assertEqual(data["count"], 3)
        self.assertIs(data["flag"], True)
        self.assertIsNone(data["nothing"])

    def test_dashed_list_and_comments(self):
        data = yamlite.loads("done_when:  # commands\n  - go test ./...\n  - go vet ./...\nmode: single\n")
        self.assertEqual(data["done_when"], ["go test ./...", "go vet ./..."])
        self.assertEqual(data["mode"], "single")

    def test_hash_inside_quotes_is_kept(self):
        data = yamlite.loads('context: ["docs/SDD.md#refunds"]\n')
        self.assertEqual(data["context"], ["docs/SDD.md#refunds"])

    def test_rejects_nested_mapping(self):
        with self.assertRaises(yamlite.YamliteError):
            yamlite.loads("outer:\n  inner: 1\n")

    def test_rejects_garbage(self):
        with self.assertRaises(yamlite.YamliteError):
            yamlite.loads("just words\n")


class DumpsTests(unittest.TestCase):
    def test_roundtrip(self):
        data = {"files": ["a.go", "b: c.go"], "done_when": [], "owner": "agent", "n": 2, "ok": False}
        text = yamlite.dumps(data)
        self.assertEqual(yamlite.loads(text), data)

    def test_quotes_risky_strings(self):
        self.assertIn('"true"', yamlite.dumps({"x": "true"}))
        self.assertIn('"12"', yamlite.dumps({"x": "12"}))


if __name__ == "__main__":
    unittest.main()

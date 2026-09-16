import os
import tempfile
import unittest

from komodo import comment_rules, comments

GO = """package x

// Load reads the config from disk and applies defaults.
func Load(path string) (*Config, error) {
\tdata, err := os.ReadFile(path)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn parse(data)
}

func Undocumented(a int) int {
\tb := a + 1
\treturn b
}

func tiny() int {
\treturn 1
}

func bigPrivate(a int) int {
\tb := a
\tb++
\tb++
\tb++
\tb++
\tb++
\tb++
\tb++
\tb++
\treturn b
}

// twoReturns is the name read back
func twoReturns(a int) int {
\tif a > 0 {
\t\treturn 1
\t}
\treturn 0
}
"""

PY = '''"""Module docstring.

# not a comment: heading in a docstring
#### [TSK-1] also not a comment [P: M] [TODO]
"""

def public(a):
    """Says what it does."""
    b = a
    return b


def _helper(a):
    b = a
    return b


def _long_helper(a):
    b = a
    b += 1
    b += 1
    b += 1
    b += 1
    b += 1
    b += 1
    b += 1
    b += 1
    return b


# we previously did this differently, per the spec v1.2
x = 1
# TODO fix this someday
y = 2
'''


class UndocumentedTests(unittest.TestCase):
    def test_go_nonobvious_default(self):
        found = comment_rules.undocumented_functions(GO, "pkg/x.go")
        names = [name for _, name in found]
        self.assertEqual(names, ["Undocumented", "bigPrivate"])

    def test_go_exported_only(self):
        names = [name for _, name in comment_rules.undocumented_functions(GO, "pkg/x.go", require="exported")]
        self.assertEqual(names, ["Undocumented"])

    def test_go_all(self):
        names = [name for _, name in comment_rules.undocumented_functions(GO, "pkg/x.go", require="all")]
        self.assertEqual(names, ["Undocumented", "tiny", "bigPrivate"])

    def test_none_and_tests_and_generated(self):
        self.assertEqual(comment_rules.undocumented_functions(GO, "pkg/x.go", require="none"), [])
        self.assertEqual(comment_rules.undocumented_functions(GO, "pkg/x_test.go"), [])
        self.assertEqual(comment_rules.undocumented_functions("// Code generated. DO NOT EDIT.\n" + GO, "pkg/x.pb.go"), [])

    def test_python_docstring_and_private(self):
        names = [name for _, name in comment_rules.undocumented_functions(PY, "mod.py")]
        self.assertEqual(names, ["_long_helper"])

    def test_typescript_export_and_arrow(self):
        ts = "export function a(x: number) {\n  const y = x;\n  return y;\n}\nfunction b(x: number) {\n  const y = x;\n  return y;\n}\nexport const c = (x: number) => {\n  const y = x;\n  return y;\n}\n"
        names = [name for _, name in comment_rules.undocumented_functions(ts, "src/a.ts")]
        self.assertEqual(names, ["a", "c"])


class InvalidTests(unittest.TestCase):
    def test_python_string_headings_are_not_comments(self):
        rules = [rule for _, rule, _ in comment_rules.invalid_comments(PY, "mod.py")]
        self.assertEqual(rules, ["EXTERNAL_REF", "MALFORMED_MARKER"])

    def test_external_ref_and_narrative(self):
        text = "x = 1\n# per the spec this is fine\ny = 2\n# we think this works\nz = 3\n# bumped from 200 after the audit, probably fine\n"
        rules = [rule for _, rule, _ in comment_rules.invalid_comments(text, "a.py")]
        self.assertEqual(rules, ["EXTERNAL_REF", "NARRATIVE", "NARRATIVE"])

    def test_over_words(self):
        text = "x = 1\n# " + " ".join(["word"] * 25) + "\ny = 2\n"
        rules = [rule for _, rule, _ in comment_rules.invalid_comments(text, "a.py")]
        self.assertEqual(rules, ["OVER_WORDS"])

    def test_over_lines_and_stacked(self):
        text = "package x\n\n// one\n// two\nvar a = 1\n\n// alone\n\n// close\nvar b = 2\n"
        rules = sorted(rule for _, rule, _ in comment_rules.invalid_comments(text, "a.go"))
        self.assertEqual(rules, ["OVER_LINES", "STACKED"])

    def test_function_allows_two_lines_and_directives(self):
        text = "package x\n\n//nolint:gocyclo\n// reads the thing\n// and returns it\nfunc a() int {\n\treturn 1\n}\n"
        self.assertEqual(comment_rules.invalid_comments(text, "a.go"), [])

    def test_name_echo_non_go(self):
        text = "const x = 1;\n\n// helper\nfunction helper() {\n  return 1;\n}\n"
        rules = [rule for _, rule, _ in comment_rules.invalid_comments(text, "a.js")]
        self.assertEqual(rules, ["NAME_ECHO"])

    def test_go_doc_name_first_is_fine(self):
        text = "package x\n\n// Load reads config.\nfunc Load() int {\n\treturn 1\n}\n"
        self.assertEqual(comment_rules.invalid_comments(text, "a.go"), [])

    def test_header_is_exempt(self):
        text = "#!/usr/bin/env python3\n# line one of a header\n# line two\n# line three\nimport os\n"
        self.assertEqual(comment_rules.invalid_comments(text, "a.py"), [])

    def test_midline_triple_quote_masks_following_lines(self):
        text = 'SAMPLE = """# heading\n#### [TSK-1] title [P: M] [TODO]\n# per the spec v1.2\n"""\nx = 1\n# we changed this later\ny = 2\n'
        findings = comment_rules.invalid_comments(text, "test_sample.py")
        self.assertEqual([(line, rule) for line, rule, _ in findings], [(6, "NARRATIVE")])
        inline = 'a = """one"""  # per the spec\nb = 2\n'
        self.assertEqual([rule for _, rule, _ in comment_rules.invalid_comments(inline, "a.py")], ["EXTERNAL_REF"])

    def test_marker_inside_string_ignored(self):
        text = 'url = "http://x#frag"  # the fragment is data\n'
        self.assertEqual(comment_rules.invalid_comments(text, "a.py"), [])


class CheckTests(unittest.TestCase):
    def test_check_all_lines_in_tempdir(self):
        with tempfile.TemporaryDirectory() as root:
            os.makedirs(os.path.join(root, "pkg"))
            with open(os.path.join(root, "pkg", "x.go"), "w") as handle:
                handle.write(GO)
            findings = comments.check(root, all_lines=True)
            rules = [f["rule"] for f in findings]
            self.assertEqual(rules.count("FUNC_UNDOCUMENTED"), 2)
            text = comments.format_findings(findings)
            self.assertIn("comment finding(s)", text)


if __name__ == "__main__":
    unittest.main()

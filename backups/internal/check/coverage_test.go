package check

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleDiff = `diff --git a/pkg/thing.go b/pkg/thing.go
index abc..def 100644
--- a/pkg/thing.go
+++ b/pkg/thing.go
@@ -1,3 +1,5 @@
 package pkg
+
+func Add(a, b int) int {
+	return a + b
+}
`

func TestParseAddedLinesReadsTheNewFileLineNumbers(t *testing.T) {
	added := ParseAddedLines(sampleDiff)
	want := []AddedLine{
		{File: "pkg/thing.go", Line: 2, Text: ""},
		{File: "pkg/thing.go", Line: 3, Text: "func Add(a, b int) int {"},
		{File: "pkg/thing.go", Line: 4, Text: "\treturn a + b"},
		{File: "pkg/thing.go", Line: 5, Text: "}"},
	}
	if len(added) != len(want) {
		t.Fatalf("added = %v, want %v", added, want)
	}
	for i, line := range added {
		if line != want[i] {
			t.Fatalf("added[%d] = %v, want %v", i, line, want[i])
		}
	}
}

const dashRemovalDiff = `diff --git a/doc.md b/doc.md
index abc..def 100644
--- a/doc.md
+++ b/doc.md
@@ -1,3 +1,3 @@
 title
----
+kept
 after
`

func TestParseAddedLinesCountsARemovedDashDashDashLine(t *testing.T) {
	added := ParseAddedLines(dashRemovalDiff)
	want := []AddedLine{{File: "doc.md", Line: 2, Text: "kept"}}
	if len(added) != len(want) || added[0] != want[0] {
		t.Fatalf("added = %v, want %v", added, want)
	}
}

const noNewlineDiff = `diff --git a/x.txt b/x.txt
index abc..def 100644
--- a/x.txt
+++ b/x.txt
@@ -1,2 +1,3 @@
 first
-second
\ No newline at end of file
+second
+third
`

func TestParseAddedLinesSkipsTheNoNewlineMarker(t *testing.T) {
	added := ParseAddedLines(noNewlineDiff)
	want := []AddedLine{{File: "x.txt", Line: 2, Text: "second"}, {File: "x.txt", Line: 3, Text: "third"}}
	if len(added) != len(want) {
		t.Fatalf("added = %v, want %v", added, want)
	}
	for i, line := range added {
		if line != want[i] {
			t.Fatalf("added[%d] = %v, want %v", i, line, want[i])
		}
	}
}

const doublePlusDiff = `diff --git a/z.txt b/z.txt
index abc..def 100644
--- a/z.txt
+++ b/z.txt
@@ -1,1 +1,2 @@
 x
+++ y
`

func TestParseAddedLinesReadsALineStartingWithPlusPlusAsContent(t *testing.T) {
	added := ParseAddedLines(doublePlusDiff)
	want := []AddedLine{{File: "z.txt", Line: 2, Text: "++ y"}}
	if len(added) != len(want) || added[0] != want[0] {
		t.Fatalf("added = %v, want %v", added, want)
	}
}

const sampleProfile = `mode: set
example.com/mod/pkg/thing.go:2.1,3.20 1 1
example.com/mod/pkg/thing.go:4.1,4.10 1 0
`

func TestParseCoverProfileReadsBlocksAndCounts(t *testing.T) {
	profile, err := ParseCoverProfile(sampleProfile, "example.com/mod")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if covered, known := profile.Covered("pkg/thing.go", 2); !covered || !known {
		t.Fatalf("line 2 = %v, %v, want covered and known", covered, known)
	}
	if covered, known := profile.Covered("pkg/thing.go", 4); covered || !known {
		t.Fatalf("line 4 = %v, %v, want uncovered and known", covered, known)
	}
	if _, known := profile.Covered("pkg/thing.go", 99); known {
		t.Fatal("line 99 is unknown, want known=false")
	}
}

func TestChangedLineCoverageIntersectsDiffAndProfile(t *testing.T) {
	profile, err := ParseCoverProfile(sampleProfile, "example.com/mod")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, known := ChangedLineCoverage(sampleDiff, profile)
	// Lines 2 and 3 are covered, line 4 is known and uncovered, line 5 is unknown to the profile.
	if want := 2.0 / 3.0; got != want || !known {
		t.Fatalf("coverage = %v, %v, want %v, true", got, known, want)
	}
}

func TestChangedLineCoverageWithNoKnownLinesIsFull(t *testing.T) {
	if got, known := ChangedLineCoverage(sampleDiff, Profile{}); got != 1 || known {
		t.Fatalf("coverage = %v, %v, want 1, false", got, known)
	}
}

func TestEvaluateCalibratesOnTheFirstCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bar.json")
	problems, err := Evaluate(path, 0.5)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none on calibration", problems)
	}
	problems, err = Evaluate(path, 0.5)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none at the same bar", problems)
	}
}

func TestEvaluateFailsWhenCoverageRegresses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bar.json")
	if _, err := Evaluate(path, 0.8); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	problems, err := Evaluate(path, 0.5)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want one regression", problems)
	}
}

func TestEvaluateKeepsTheCalibratedBarOnImprovement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bar.json")
	if _, err := Evaluate(path, 0.5); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if _, err := Evaluate(path, 1); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	problems, err := Evaluate(path, 0.6)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none against the calibrated bar of 50%%", problems)
	}
	if problems, _ := Evaluate(path, 0.4); len(problems) != 1 {
		t.Fatalf("problems = %v, want one regression below the calibrated bar", problems)
	}
}

func TestModulePathReadsGoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/mod\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if module, err := ModulePath(dir); err != nil || module != "example.com/mod" {
		t.Fatalf("module = %q, %v, want example.com/mod", module, err)
	}
}

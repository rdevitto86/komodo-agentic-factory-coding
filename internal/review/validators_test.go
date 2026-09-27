package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diffAdding wraps added lines in a minimal unified diff for one file.
func diffAdding(file string, lines ...string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n@@ -1,0 +1,%d @@\n", file, file, len(lines))
	for _, line := range lines {
		out.WriteString("+" + line + "\n")
	}
	return out.String()
}

// writeFile writes one file under root, creating its directory.
func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// byKind returns the measurements of one kind, in report order.
func byKind(report Report, kind Kind) []Measurement {
	var out []Measurement
	for _, each := range report.Measurements {
		if each.Kind == kind {
			out = append(out, each)
		}
	}
	return out
}

func TestMeasureRunsTheReposCommands(t *testing.T) {
	cases := []struct {
		name       string
		validators Validators
		kind       Kind
		want       []bool
		detail     string
	}{
		{"passing tests", Validators{Tests: "true"}, KindTests, []bool{true}, ""},
		{"failing tests", Validators{Tests: "echo broke; exit 1"}, KindTests, []bool{false}, "broke"},
		{"each reproducer", Validators{Reproducers: []string{"true", "exit 3"}}, KindReproducer, []bool{true, false}, "exit status 3"},
		{"an audit the repo has", Validators{Audit: "exit 2"}, KindAudit, []bool{false}, "exit status 2"},
		{"no audit", Validators{}, KindAudit, nil, ""},
		{"a security linter the repo has", Validators{SecurityLint: "true"}, KindSecurityLint, []bool{true}, ""},
		{"no security linter", Validators{}, KindSecurityLint, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.validators.Worktree = t.TempDir()
			report, err := Measure(tc.validators)
			if err != nil {
				t.Fatal(err)
			}
			got := byKind(report, tc.kind)
			if len(got) != len(tc.want) {
				t.Fatalf("%s measurements = %+v, want %d", tc.kind, got, len(tc.want))
			}
			for index, each := range got {
				if each.Passed != tc.want[index] {
					t.Fatalf("measurement %d = %+v, want passed %v", index, each, tc.want[index])
				}
			}
			if tc.detail != "" && !strings.Contains(got[len(got)-1].Detail, tc.detail) {
				t.Fatalf("detail = %q, want it to hold %q", got[len(got)-1].Detail, tc.detail)
			}
		})
	}
}

func TestMeasureScansTheDiffForSecrets(t *testing.T) {
	cases := []struct {
		name   string
		diff   string
		passed bool
	}{
		{"a key on an added line", diffAdding("config.go", `key := "AKIA`+`ABCDEFGHIJKLMNOP"`), false},
		{"nothing secret", diffAdding("config.go", "key := os.Getenv(\"KEY\")"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Measure(Validators{Worktree: t.TempDir(), Diff: tc.diff})
			if err != nil {
				t.Fatal(err)
			}
			got := byKind(report, KindSecrets)
			if len(got) != 1 || got[0].Passed != tc.passed {
				t.Fatalf("secrets = %+v, want one measurement, passed %v", got, tc.passed)
			}
			if !tc.passed && !strings.Contains(got[0].Detail, "config.go:1") {
				t.Fatalf("detail = %q, want it to name config.go:1", got[0].Detail)
			}
		})
	}
}

func TestMeasureCountsCallersOfChangedExportedSymbols(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "load/load.go", "package load\n\n// Load loads.\nfunc Load() {}\n\nfunc helper() { Load() }\n")
	writeFile(t, root, "cmd/main.go", "package main\n\nimport \"x/load\"\n\nfunc main() {\n\tload.Load()\n\tload.Load()\n}\n")
	writeFile(t, root, "vendor/other/other.go", "package other\n\nfunc f() { Load() }\n")
	writeFile(t, root, "broken.go", "package broken\n\nfunc {\n")
	writeFile(t, root, "README.md", "Load() is documented here.\n")
	diff := diffAdding("load/load.go", "func Load() {}", "func helper() { Load() }") +
		diffAdding("README.md", "func Readme() {}")

	report, err := Measure(Validators{Worktree: root, Diff: diff})
	if err != nil {
		t.Fatal(err)
	}
	got := byKind(report, KindCallers)
	if len(got) != 1 {
		t.Fatalf("callers = %+v, want one measurement, for Load alone", got)
	}
	if want := "Load: 3 references in 2 files"; got[0].Detail != want {
		t.Fatalf("detail = %q, want %q", got[0].Detail, want)
	}
}

func TestMeasureKeepsOnlyTheTailOfLongOutput(t *testing.T) {
	report, err := Measure(Validators{Worktree: t.TempDir(), Tests: "yes head | head -c 5000; echo; echo the-end"})
	if err != nil {
		t.Fatal(err)
	}
	got := byKind(report, KindTests)
	if len(got) != 1 || len(got[0].Detail) != outputTail || !strings.HasSuffix(got[0].Detail, "the-end") {
		t.Fatalf("tests = %+v, want the last %d bytes, ending the-end", got, outputTail)
	}
}

func TestMeasureCountsCallersOfEveryKindOfDeclaration(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "package a\n\ntype Shape int\n\nvar Size = 1\n\nconst Max = 2\n\n"+
		"func use(s Shape) int { return Size + Max }\n")
	diff := diffAdding("a.go", "type Shape int", "var Size = 1", "const Max = 2")

	report, err := Measure(Validators{Worktree: root, Diff: diff})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, each := range byKind(report, KindCallers) {
		got = append(got, each.Detail)
	}
	want := "Max: 1 references in 1 files,Shape: 1 references in 1 files,Size: 1 references in 1 files"
	if strings.Join(got, ",") != want {
		t.Fatalf("callers = %v, want %s", got, want)
	}
}

func TestMeasureReportsATreeItCannotWalk(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "locked/a.go", "package a\n")
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := Measure(Validators{Worktree: root, Diff: diffAdding("a.go", "func Load() {}")}); err == nil {
		t.Fatal("Measure counted callers in a tree it cannot read")
	}
}

func TestMarkdownStatesTheReportAsSettledFact(t *testing.T) {
	report := Report{Measurements: []Measurement{
		{Kind: KindTests, Command: "go test ./...", Passed: false, Detail: "FAIL x"},
		{Kind: KindCallers, Passed: true, Detail: "Load: 3 references in 2 files"},
	}}
	got := report.Markdown()
	for _, want := range []string{"settled fact", "### tests: failed", "`go test ./...`", "FAIL x", "### callers: passed",
		"Load: 3 references in 2 files"} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown lacks %q:\n%s", want, got)
		}
	}
}

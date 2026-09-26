package ingest

import (
	"testing"
)

func TestExtractGoPackagesSortsPackages(t *testing.T) {
	files := []string{"c/x.go", "a/y.go", "b/z.go"}
	got := extractGoPackages(files)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("packages = %v, want [a b c]", got)
	}
}

func TestExtractGoPackagesDeduplicates(t *testing.T) {
	files := []string{"a/x.go", "a/y.go", "a/z.go"}
	got := extractGoPackages(files)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("packages = %v, want [a]", got)
	}
}

func TestExtractGoPackagesSkipsRootLevel(t *testing.T) {
	files := []string{"root.go", "a/x.go"}
	got := extractGoPackages(files)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("packages = %v, want [a], skip root.go", got)
	}
}

func TestExtractGoPackagesIgnoresNonGoFiles(t *testing.T) {
	files := []string{"a/file.txt", "b/file.py", "c/file.go"}
	got := extractGoPackages(files)
	if len(got) != 1 || got[0] != "c" {
		t.Fatalf("packages = %v, want [c]", got)
	}
}

func TestHasTypeScriptFilesDetectsTS(t *testing.T) {
	files := []string{"a/file.go", "b/file.ts", "c/file.py"}
	if !hasTypeScriptFiles(files) {
		t.Fatal("hasTypeScriptFiles = false, want true for .ts file")
	}
}

func TestHasTypeScriptFilesDetectsTSX(t *testing.T) {
	files := []string{"a/file.go", "b/file.tsx", "c/file.py"}
	if !hasTypeScriptFiles(files) {
		t.Fatal("hasTypeScriptFiles = false, want true for .tsx file")
	}
}

func TestHasTypeScriptFilesReturnsFalseWhenAbsent(t *testing.T) {
	files := []string{"a/file.go", "b/file.py", "c/file.js"}
	if hasTypeScriptFiles(files) {
		t.Fatal("hasTypeScriptFiles = true, want false")
	}
}

func TestDerivedChecksGeneratesGoBuildsPerPackage(t *testing.T) {
	files := []string{"a/x.go", "b/y.go"}
	got := derivedChecks(files)
	if len(got) < 6 {
		t.Fatalf("checks count = %d, want at least 6 for two packages", len(got))
	}
	// Check for expected Go checks.
	hasGoA := false
	hasGoB := false
	for _, check := range got {
		if check == "go build ./a/..." {
			hasGoA = true
		}
		if check == "go build ./b/..." {
			hasGoB = true
		}
	}
	if !hasGoA || !hasGoB {
		t.Fatalf("missing go build checks, got %v", got)
	}
}

func TestDerivedChecksGeneratesVetAndTestForGo(t *testing.T) {
	files := []string{"a/x.go"}
	got := derivedChecks(files)
	expected := []string{"go build ./a/...", "go vet ./a/...", "go test ./a/..."}
	if len(got) != len(expected) {
		t.Fatalf("checks = %v, want %v", got, expected)
	}
	for i, check := range expected {
		if got[i] != check {
			t.Fatalf("check[%d] = %q, want %q", i, got[i], check)
		}
	}
}

func TestDerivedChecksGeneratesTypeScriptChecks(t *testing.T) {
	files := []string{"a/file.ts"}
	got := derivedChecks(files)
	if len(got) < 2 {
		t.Fatalf("checks count = %d, want at least 2 for TypeScript", len(got))
	}
	hasTsc := false
	hasNpmTest := false
	for _, check := range got {
		if check == "tsc --noEmit" {
			hasTsc = true
		}
		if check == "npm test" {
			hasNpmTest = true
		}
	}
	if !hasTsc || !hasNpmTest {
		t.Fatalf("missing TypeScript checks, got %v", got)
	}
}

func TestDerivedChecksEmptyForNoRelevantFiles(t *testing.T) {
	files := []string{"a/file.txt", "b/file.py"}
	got := derivedChecks(files)
	if len(got) != 0 {
		t.Fatalf("checks = %v, want empty for non-Go/TS files", got)
	}
}

func TestAllChecksCombinesHandWrittenAndDerived(t *testing.T) {
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n" +
		"files: [a/one.go]\ndone_when:\n  - go test ./a/...\n```\n"
	_, g := group(t, text)
	files := []string{"a/one.go"}
	got := allChecks(g, files)
	if len(got) < 3 {
		t.Fatalf("checks count = %d, want at least 3", len(got))
	}
	// Hand-written go test should be first, then build and vet.
	if got[0] != "go test ./a/..." {
		t.Fatalf("check[0] = %q, want the hand-written check first", got[0])
	}
}

func TestAllChecksDeduplicatesHandWrittenAndDerived(t *testing.T) {
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n" +
		"files: [a/one.go]\ndone_when:\n  - go build ./a/...\n  - go vet ./a/...\n  - go test ./a/...\n```\n"
	_, g := group(t, text)
	files := []string{"a/one.go"}
	got := allChecks(g, files)
	// All three checks should be in hand-written, no duplicates from derived.
	if len(got) != 3 {
		t.Fatalf("checks count = %d, want 3 (no duplicates)", len(got))
	}
	expected := []string{"go build ./a/...", "go vet ./a/...", "go test ./a/..."}
	for i, exp := range expected {
		if got[i] != exp {
			t.Fatalf("check[%d] = %q, want %q", i, got[i], exp)
		}
	}
}

func TestAllChecksPreservesHandWrittenOrder(t *testing.T) {
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n" +
		"files: [a/one.go]\ndone_when:\n  - custom check\n```\n"
	_, g := group(t, text)
	files := []string{"a/one.go"}
	got := allChecks(g, files)
	// Custom check should come first (hand-written), then derived.
	if len(got) < 2 || got[0] != "custom check" {
		t.Fatalf("check[0] = %q, want 'custom check' (hand-written first)", got[0])
	}
}

func TestBuildCardIncludesDerivedChecks(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/one.go": "package a\n"})
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\nfiles: [a/one.go]\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	// Card should have derived checks for the Go package.
	hasGoTest := false
	for _, check := range card.Checks {
		if check == "go test ./a/..." {
			hasGoTest = true
			break
		}
	}
	if !hasGoTest {
		t.Fatalf("card.Checks = %v, want to include derived Go checks", card.Checks)
	}
}

func TestBuildCardHandWrittenChecksNotReplaced(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/one.go": "package a\n"})
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n" +
		"files: [a/one.go]\ndone_when:\n  - custom check\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	// Hand-written "custom check" should be present and first.
	if len(card.Checks) == 0 || card.Checks[0] != "custom check" {
		t.Fatalf("card.Checks[0] = %q, want 'custom check' (hand-written preserved)", card.Checks[0])
	}
}

func TestBuildCardWithTypeScriptAddsTypeScriptChecks(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"src/main.ts": "console.log('hello');"})
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\nfiles: [src/main.ts]\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	// Card should have TypeScript checks.
	hasTsc := false
	hasNpmTest := false
	for _, check := range card.Checks {
		if check == "tsc --noEmit" {
			hasTsc = true
		}
		if check == "npm test" {
			hasNpmTest = true
		}
	}
	if !hasTsc || !hasNpmTest {
		t.Fatalf("card.Checks = %v, want TypeScript checks", card.Checks)
	}
}

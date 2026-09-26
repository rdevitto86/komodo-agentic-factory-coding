package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const base = "# Changelog\n\nIntro.\n\n## 1.0.0-alpha.5 — 2026-09-26\n\n- **TG-04.1** First (2 task(s))\n\n" +
	"## The first 1.0.0 line — 2026-09-24\n\nHistory.\n\n## [1.0.0-alpha.4] — 2026-09-21\n\n- old\n"

// order returns the offsets of each needle in text, failing when one is missing.
func order(t *testing.T, text string, needles ...string) []int {
	t.Helper()
	var at []int
	for _, needle := range needles {
		index := strings.Index(text, needle)
		if index < 0 {
			t.Fatalf("%q is missing from:\n%s", needle, text)
		}
		at = append(at, index)
	}
	return at
}

func TestFoldPutsNewVersionsInSemVerOrderAboveOlderOnes(t *testing.T) {
	fragments := map[string][]string{
		"1.0.0-alpha.6":  {"- **TG-05.3** Three (1 task(s))", "- **TG-05.2** Two (3 task(s))"},
		"1.0.0-alpha.8":  {"- **TG-07.1** Files (4 task(s))"},
		"1.0.0-alpha.10": {"- **TG-09.1** Ten (1 task(s))"},
	}
	text := Fold(base, fragments)
	at := order(t, text, "## 1.0.0-alpha.10\n", "## 1.0.0-alpha.8\n", "## 1.0.0-alpha.6\n",
		"- **TG-05.3**", "- **TG-05.2**", "## 1.0.0-alpha.5", "## The first 1.0.0 line")
	for index := 1; index < len(at); index++ {
		if at[index] < at[index-1] {
			t.Fatalf("out of order at %d:\n%s", index, text)
		}
	}
	if Latest(text) != "1.0.0-alpha.10" {
		t.Fatalf("latest = %s", Latest(text))
	}
}

func TestFoldAddsToAnExistingHeadingAndKeepsItsLines(t *testing.T) {
	text := Fold(base, map[string][]string{"1.0.0-alpha.5": {"- **TG-04.5** Ship fixes (3 task(s))"}})
	order(t, text, "## 1.0.0-alpha.5 — 2026-09-26\n\n- **TG-04.5** Ship fixes (3 task(s))\n- **TG-04.1** First")
	if strings.Count(text, "## 1.0.0-alpha.5") != 1 {
		t.Fatalf("the heading was duplicated:\n%s", text)
	}
}

func TestFoldSkipsALineTheSectionAlreadyHolds(t *testing.T) {
	text := Fold(base, map[string][]string{"1.0.0-alpha.5": {"- **TG-04.1** First (2 task(s))"}})
	if text != base {
		t.Fatalf("an already folded line changed the text:\n%s", text)
	}
}

func TestFoldReplacesTheLineAGroupFoldedBefore(t *testing.T) {
	text := Fold(base, map[string][]string{"1.0.0-alpha.5": {"- **TG-04.1** First (3 task(s))"}})
	if strings.Contains(text, "(2 task(s))") || strings.Count(text, "**TG-04.1**") != 1 {
		t.Fatalf("the group's line was not replaced:\n%s", text)
	}
}

func TestFoldIntoAnEmptyChangelogStartsOne(t *testing.T) {
	text := Fold("", map[string][]string{"0.1.0": {"- **TG-01.1** One (1 task(s))"}})
	if text != "# Changelog\n\n## 0.1.0\n\n- **TG-01.1** One (1 task(s))\n" {
		t.Fatalf("text = %q", text)
	}
}

func TestReadFoldsFragmentsAndFoldFilesWritesThemOnce(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, File), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFragment(root, "1.0.0-alpha.6", "TG-05.2", "- **TG-05.2** Two (3 task(s))"); err != nil {
		t.Fatal(err)
	}
	if err := WriteFragment(root, "1.0.0-alpha.6", "TG-05.3", "- **TG-05.3** Three (1 task(s))"); err != nil {
		t.Fatal(err)
	}
	read, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if on, _ := os.ReadFile(filepath.Join(root, File)); string(on) != base {
		t.Fatal("Read changed the file on disk")
	}
	if err := FoldFiles(root, "2026-09-27"); err != nil {
		t.Fatal(err)
	}
	folded, _ := os.ReadFile(filepath.Join(root, File))
	if want := strings.Replace(read, "## 1.0.0-alpha.6\n", "## 1.0.0-alpha.6 — 2026-09-27\n", 1); string(folded) != want {
		t.Fatalf("folded file:\n%s\nwant:\n%s", folded, want)
	}
	if _, err := os.Stat(filepath.Join(root, Dir)); !os.IsNotExist(err) {
		t.Fatalf("the fragments are still there: %v", err)
	}
	again, _ := Read(root)
	if again != string(folded) {
		t.Fatal("reading after the fold differs from the folded file")
	}
}

func TestCompareOrdersPrereleasesNumerically(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"1.0.0-alpha.10", "1.0.0-alpha.9", 1},
		{"1.0.0-alpha.5", "1.0.0-beta.2", -1},
		{"1.0.0", "1.0.0-beta.2", 1},
		{"1.0.0", "1.0.0", 0},
	}
	for _, c := range cases {
		if got := Compare(c.left, c.right); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.left, c.right, got, c.want)
		}
	}
}

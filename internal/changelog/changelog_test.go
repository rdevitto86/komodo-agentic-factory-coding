package changelog

import (
	"os"
	"path/filepath"
	"testing"
)

const base = "# Changelog\n\nIntro.\n\n## 1.0.0-alpha.5 — 2026-09-26\n\nShip fixes.\n\n" +
	"## The first 1.0.0 line — 2026-09-24\n\nHistory.\n\n## [1.0.0-alpha.4] — 2026-09-21\n\n- old\n"

func TestReadReturnsTheFileAsWritten(t *testing.T) {
	root := t.TempDir()
	if text, err := Read(root); err != nil || text != "" {
		t.Fatalf("a repo with no changelog read %q, %v", text, err)
	}
	if err := os.WriteFile(filepath.Join(root, File), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if text, err := Read(root); err != nil || text != base {
		t.Fatalf("read %q, %v", text, err)
	}
}

func TestLatestSkipsATitledHistorySection(t *testing.T) {
	if got := Latest(base); got != "1.0.0-alpha.5" {
		t.Fatalf("latest = %s", got)
	}
	if got := Latest("# Changelog\n"); got != "" {
		t.Fatalf("latest = %q; a changelog with no version names none", got)
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

func TestCompareOrdersAlphaBetaRcThenStable(t *testing.T) {
	if Compare("1.0.0-beta.9", "1.0.0-rc.1") != -1 {
		t.Fatalf("beta.9 should sort before rc.1")
	}
	if Compare("1.0.0-rc.1", "1.0.0") != -1 {
		t.Fatalf("rc.1 should sort before the stable release")
	}
	if Latest("## 1.0.0-beta.2\n\n## 1.0.0\n") != "1.0.0" {
		t.Fatal("the stable version should be latest over a beta with no rc between")
	}
}

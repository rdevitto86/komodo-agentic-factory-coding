package release

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const changelog = "# Changelog\n\n## 2.0.0 — 2026-09-21\n\n- the line\n\n## 1.3.0 — 2026-09-01\n\n- old\n"

func TestVersionsReadsEveryHeading(t *testing.T) {
	got := Versions(changelog)
	if len(got) != 2 || got[0].Number != "2.0.0" || got[1].Number != "1.3.0" {
		t.Fatalf("versions = %+v", got)
	}
	if !strings.Contains(got[0].Body, "the line") || strings.Contains(got[0].Body, "old") {
		t.Fatalf("body = %q", got[0].Body)
	}
}

func TestLatestTakesTheHighest(t *testing.T) {
	if got := Latest(changelog); got != "2.0.0" {
		t.Fatalf("latest = %s", got)
	}
	if Latest("# Changelog\n") != "" {
		t.Fatal("an empty changelog has no latest version")
	}
}

func TestCompareOrdersSemanticVersions(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{{"2.0.0", "1.3.0", 1}, {"1.3.0", "1.3.1", -1}, {"1.3.0", "1.3.0", 0}, {"1.10.0", "1.9.0", 1}}
	for _, item := range cases {
		if got := Compare(item.left, item.right); got != item.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", item.left, item.right, got, item.want)
		}
	}
}

func TestTaggableSkipsWhatIsTagged(t *testing.T) {
	got := Taggable(changelog, []string{"v1.3.0"})
	if len(got) != 1 || got[0] != "2.0.0" {
		t.Fatalf("taggable = %v", got)
	}
	if len(Taggable(changelog, []string{"v1.3.0", "v2.0.0"})) != 0 {
		t.Fatal("a fully tagged changelog has nothing to tag")
	}
}

func TestUnreleasedSkipsHistoryOlderThanTheNewestTag(t *testing.T) {
	text := "# Changelog\n\n## 3.0.0-beta.1 — 2026-09-24\n\n- beta\n\n## 2.0.0 — 2026-09-21\n\n- the line\n\n## 1.3.0 — 2026-09-01\n\n- old\n"
	got := Unreleased(text, []string{"v2.0.0", "prototype-final"})
	if len(got) != 1 || got[0] != "3.0.0-beta.1" {
		t.Fatalf("unreleased = %v; 1.3.0 is untagged history older than v2.0.0", got)
	}
	if got := Unreleased(text, nil); len(got) != 3 {
		t.Fatalf("unreleased = %v; with no tags every version is unreleased", got)
	}
}

func TestTaggableOrdersOldestFirst(t *testing.T) {
	got := Taggable(changelog, nil)
	if len(got) != 2 || got[0] != "1.3.0" {
		t.Fatalf("taggable = %v", got)
	}
}

func TestCheckNamesEveryDrift(t *testing.T) {
	drift := Check(changelog, []string{"v0.9.0"}, []string{"3.0.0", "2.0.0"})
	var subjects []string
	for _, item := range drift {
		subjects = append(subjects, item.Subject)
	}
	joined := strings.Join(subjects, ",")
	if !strings.Contains(joined, "v0.9.0") || !strings.Contains(joined, "3.0.0") {
		t.Fatalf("drift = %+v", drift)
	}
	if strings.Contains(joined, "2.0.0") {
		t.Fatal("a version the changelog names is not drift")
	}
}

func TestCheckFlagsAVersionNamedByMoreThanOneHeading(t *testing.T) {
	repeated := "# Changelog\n\n## 2.0.0 — 2026-09-22\n\n- a\n\n## 2.0.0 — 2026-09-21\n\n- b\n"
	drift := Check(repeated, nil, nil)
	var subjects []string
	for _, item := range drift {
		subjects = append(subjects, item.Subject)
	}
	if !strings.Contains(strings.Join(subjects, ","), "2.0.0") {
		t.Fatalf("drift = %+v; a version under two headings must be flagged", drift)
	}
}

func TestCheckFlagsAHeadingOutOfOrder(t *testing.T) {
	outOfOrder := "# Changelog\n\n## 1.0.0 — 2026-09-01\n\n- a\n\n## 2.0.0 — 2026-09-22\n\n- b\n"
	drift := Check(outOfOrder, nil, nil)
	found := false
	for _, item := range drift {
		if item.Subject == "2.0.0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drift = %+v; a heading newer than the one above it must be flagged", drift)
	}
}

func TestCheckIsQuietWhenEverythingAgrees(t *testing.T) {
	if drift := Check(changelog, []string{"v2.0.0", "v1.3.0"}, []string{"2.0.0"}); len(drift) != 0 {
		t.Fatalf("drift = %+v", drift)
	}
}

func TestTagNameAndMessage(t *testing.T) {
	if TagName("2.0.0") != "v2.0.0" || TagMessage("2.0.0") != "release 2.0.0" {
		t.Fatal("tag name or message is wrong")
	}
}

func TestBuildAssetsWritesEveryTarget(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var out bytes.Buffer
	paths, err := BuildAssets(root, dir, &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(Targets) {
		t.Fatalf("paths = %v", paths)
	}
	for _, target := range Targets {
		info, err := os.Stat(filepath.Join(dir, target.Name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", target.Name)
		}
	}
}

func TestAPrereleaseSortsBeforeItsReleaseAndByItsNumber(t *testing.T) {
	ordered := []string{"0.51.0", "1.0.0-alpha.1", "1.0.0-alpha.2", "1.0.0-alpha.10", "1.0.0-beta", "1.0.0", "1.0.1"}
	for index := 1; index < len(ordered); index++ {
		if Compare(ordered[index-1], ordered[index]) >= 0 {
			t.Fatalf("%s should sort before %s", ordered[index-1], ordered[index])
		}
	}
	text := "## 1.0.0 — 2026-09-23\n\n- one\n\n## [1.0.0-alpha.4] — 2026-09-21\n\n- two\n"
	if drift := Check(text, []string{"v1.0.0-alpha.4"}, []string{"1.0.0"}); len(drift) != 0 {
		t.Fatalf("drift = %+v", drift)
	}
	if got := Latest(text); got != "1.0.0" {
		t.Fatalf("latest = %s", got)
	}
}

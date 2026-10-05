package release

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/gate"
)

const twoVersions = "# Changelog\n\n## 2.0.0 — 2026-09-21\n\n- the line\n\n## 1.3.0 — 2026-09-01\n\n- old\n"

func TestAnUnreleasedHeadingIsNeverTaggable(t *testing.T) {
	text := "# Changelog\n\n## Unreleased — 3.0.0\n\n- next\n\n" + strings.TrimPrefix(twoVersions, "# Changelog\n\n")
	if got := Taggable(text, nil); len(got) != 2 || got[0] != "1.3.0" || got[1] != "2.0.0" {
		t.Fatalf("taggable = %v; the version still marked Unreleased must never be tagged", got)
	}
	if got := Unreleased(text, []string{"v2.0.0"}); len(got) != 0 {
		t.Fatalf("unreleased = %v; an Unreleased heading is not a version to tag", got)
	}
	if drift := Check(text, nil, []string{"3.0.0"}); len(drift) != 0 {
		t.Fatalf("drift = %+v; the changelog still names the version in progress", drift)
	}
}

func TestNamesMatchesOnlyAWholeHeadingVersion(t *testing.T) {
	if !Names(twoVersions, "1.3.0") || Names(twoVersions, "1.3") || Names(twoVersions, "3.0.0") {
		t.Fatal("Names must match exactly the versions the headings carry")
	}
	if Names("## Unreleased — 3.0.0\n\n- next\n", "3.0.0") {
		t.Fatal("an Unreleased heading must not name its version as released")
	}
}

func TestShippedVersionsSkipsAGroupWithAnOpenTask(t *testing.T) {
	text := "### [TG-1.1] Done\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n#### [TSK-1.1.1] A [P: H] [DONE]\n```yaml\nfiles: [a.go]\n```\n\n" +
		"### [TG-1.2] Open\n```yaml\ntype: feat\nversion: 3.0.0\n```\n\n#### [TSK-1.2.1] B [P: H] [READY]\n```yaml\nfiles: [b.go]\n```\n"
	if got := ShippedVersions(backlog.Parse(text)); len(got) != 1 || got[0] != "2.0.0" {
		t.Fatalf("shipped = %v, want only the fully DONE group's version", got)
	}
}

func TestVersionsReadsEveryHeading(t *testing.T) {
	got := Versions(twoVersions)
	if len(got) != 2 || got[0].Number != "2.0.0" || got[1].Number != "1.3.0" {
		t.Fatalf("versions = %+v", got)
	}
	if !strings.Contains(got[0].Body, "the line") || strings.Contains(got[0].Body, "old") {
		t.Fatalf("body = %q", got[0].Body)
	}
}

func TestLatestTakesTheHighest(t *testing.T) {
	if got := Latest(twoVersions); got != "2.0.0" {
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
	got := Taggable(twoVersions, []string{"v1.3.0"})
	if len(got) != 1 || got[0] != "2.0.0" {
		t.Fatalf("taggable = %v", got)
	}
	if len(Taggable(twoVersions, []string{"v1.3.0", "v2.0.0"})) != 0 {
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
	got := Taggable(twoVersions, nil)
	if len(got) != 2 || got[0] != "1.3.0" {
		t.Fatalf("taggable = %v", got)
	}
}

func TestCheckNamesEveryDrift(t *testing.T) {
	drift := Check(twoVersions, []string{"v0.9.0"}, []string{"3.0.0", "2.0.0"})
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
	if drift := Check(twoVersions, []string{"v2.0.0", "v1.3.0"}, []string{"2.0.0"}); len(drift) != 0 {
		t.Fatalf("drift = %+v", drift)
	}
}

// TestCheckReportsAMalformedVersionTagButIgnoresOtherTags proves a tag meant as a version but not
// x.y.z is drift, while a tag that is not a version at all stays quiet.
func TestCheckReportsAMalformedVersionTagButIgnoresOtherTags(t *testing.T) {
	drift := Check(twoVersions, []string{"v2.0.0", "v1.3.0", "v1.4", "prototype-final"}, []string{"2.0.0"})
	if len(drift) != 1 || drift[0].Subject != "v1.4" {
		t.Fatalf("drift = %+v, want only v1.4 reported", drift)
	}
}

func TestTagNameAndMessage(t *testing.T) {
	if TagName("2.0.0") != "v2.0.0" || TagMessage("2.0.0") != "release 2.0.0" {
		t.Fatal("tag name or message is wrong")
	}
}

func TestTargetsCoverEveryPlatformTheHookRuns(t *testing.T) {
	want := []string{"darwin/arm64", "darwin/amd64", "linux/amd64", "linux/arm64", "windows/amd64"}
	var got []string
	for _, target := range Targets {
		got = append(got, target.GOOS+"/"+target.Arch)
		name := "komodo-" + target.GOOS + "-" + target.Arch
		if target.GOOS == "windows" {
			name += ".exe"
		}
		if target.Name != name {
			t.Errorf("%s/%s is named %q, want %q, the name the hook and install look for",
				target.GOOS, target.Arch, target.Name, name)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

func TestBuildAssetsStopsWhenABuildDoesNotReproduce(t *testing.T) {
	previous := build
	t.Cleanup(func() { build = previous })
	calls := 0
	fake := func(stable bool) func(string, string, gate.Target) (string, error) {
		return func(_ string, dir string, target gate.Target) (string, error) {
			calls++
			body := "same"
			if !stable {
				body = fmt.Sprintf("build %d", calls)
			}
			path := filepath.Join(dir, target.Name)
			return path, os.WriteFile(path, []byte(body), 0o755)
		}
	}
	build = fake(true)
	if paths, err := BuildAssets(t.TempDir(), t.TempDir(), io.Discard); err != nil || len(paths) != len(Targets) {
		t.Fatalf("paths = %v, %v; identical builds must pass", paths, err)
	}
	build = fake(false)
	if _, err := BuildAssets(t.TempDir(), t.TempDir(), io.Discard); err == nil || !strings.Contains(err.Error(), "does not reproduce") {
		t.Fatalf("err = %v; a build that differs from its twin must stop the release", err)
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

func TestReadChangelogReadsTheFileAsWritten(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "CHANGELOG.md")
	if text, err := ReadChangelog(path); err != nil || text != "" {
		t.Fatalf("a missing changelog read %q, %v", text, err)
	}
	if err := os.WriteFile(path, []byte(twoVersions), 0o644); err != nil {
		t.Fatal(err)
	}
	if text, err := ReadChangelog(path); err != nil || text != twoVersions {
		t.Fatalf("read %q, %v", text, err)
	}
}

func TestCheckReportsADatedHeadingWithNoBody(t *testing.T) {
	text := "# Changelog\n\n## 1.0.1 — 2026-10-02\n\n## 1.0.0 — 2026-09-30\n\n- shipped\n"
	drift := Check(text, nil, nil)
	if len(drift) != 1 || drift[0].Subject != "1.0.1" || !strings.Contains(drift[0].Detail, "no body") {
		t.Fatalf("drift = %+v; a dated heading with nothing under it has no body", drift)
	}
	if versions := Versions(text); versions[1].Body != "- shipped" {
		t.Fatalf("body = %q; a heading's date is not its body", versions[1].Body)
	}
}

func TestATagThatIsNoVersionIsNeverCompared(t *testing.T) {
	text := "## 1.0.0 — 2026-09-30\n\n- shipped\n"
	if got := Unreleased(text, []string{"prototype-final", "v0.9.0"}); len(got) != 1 || got[0] != "1.0.0" {
		t.Fatalf("unreleased = %v; prototype-final is no version and must not count", got)
	}
}

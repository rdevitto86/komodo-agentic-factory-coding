package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
)

func TestTagTagsEveryUntaggedVersionOldestFirstAtTheCommitThatNamedIt(t *testing.T) {
	root, bare := tagRepo(t, "main", releaseChangelog)
	first, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	changelog := "# Changelog\n\n## 3.0.0 — 2026-09-24\n\n- new\n\n" + strings.TrimPrefix(releaseChangelog, "# Changelog\n\n")
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "commit", "-am", "name 3.0.0")
	second, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "commit", "--allow-empty", "-m", "later work")
	var out bytes.Buffer
	if err := tag(root, &out); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{"v2.0.0": first, "v3.0.0": second} {
		got, err := exec.Command("git", "-C", bare, "rev-parse", name+"^{commit}").Output()
		if err != nil {
			t.Fatalf("origin has no %s: %v", name, err)
		}
		if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
			t.Fatalf("%s points at %s, want the commit that named it, %s", name, got, want)
		}
	}
	if printed := out.String(); strings.Index(printed, "tagged v2.0.0") > strings.Index(printed, "tagged v3.0.0") {
		t.Fatalf("out = %q, want the oldest version tagged first", printed)
	}
}

func TestTagRefusesAVersionNoCommitNames(t *testing.T) {
	root, _ := tagRepo(t, "main", releaseChangelog)
	changelog := "# Changelog\n\n## 3.0.0 — 2026-09-24\n\n- new\n\n" + strings.TrimPrefix(releaseChangelog, "# Changelog\n\n")
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := tag(root, &out); err == nil || !strings.Contains(err.Error(), "3.0.0: no commit") {
		t.Fatalf("err = %v, want an uncommitted heading refused", err)
	}
}

func TestReleaseNotesRendersTheEpicsGoalAndLandedGroups(t *testing.T) {
	root := t.TempDir()
	// A .git of its own stops the CLI's walk up before it reaches the real worktree.
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	epic := backlog.EpicFile{ID: "EPIC-90", Title: "The harness", Status: "READY", Version: "4.0.0", GroupsMax: 6,
		Goal: "The harness ships an epic. It also cleans up."}
	if _, err := backlog.WriteEpic(root, epic); err != nil {
		t.Fatal(err)
	}
	backlogtest.SeedText(t, root,
		"### [TG-90.1] A landed group\n```yaml\ntype: feat\nversion: 4.0.0\n```\n\n"+
			"#### [TSK-90.1.1] Done [P: H] [DONE]\n```yaml\nfiles: [a.go]\n```\n\n"+
			"### [TG-90.2] An open group\n```yaml\ntype: feat\nversion: 4.0.0\n```\n\n"+
			"#### [TSK-90.2.1] Open [P: H] [READY]\n```yaml\nfiles: [b.go]\n```\n")
	got, err := releaseNotes(root, "EPIC-90", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if want := "## 4.0.0 — 2026-10-05\n\nThe harness ships an epic.\n\n- A landed group\n"; got != want {
		t.Fatalf("notes = %q, want %q", got, want)
	}
	if _, err := releaseNotes(root, "EPIC-91", time.Now()); err == nil {
		t.Fatal("want an unknown epic refused")
	}
	got2 := runCLI(t, root, "", "release", "notes", "EPIC-90")
	if got2.code != 0 || !strings.Contains(got2.stdout, "- A landed group\n") || !strings.HasPrefix(got2.stdout, "## 4.0.0 — ") {
		t.Fatalf("release notes exited %d: %s%s", got2.code, got2.stdout, got2.stderr)
	}
	if usage := runCLI(t, root, "", "release", "notes"); usage.code == 0 || !strings.Contains(usage.stderr, "komodo release notes EPIC-NN") {
		t.Fatalf("want a usage refusal with no epic, got %d: %s", usage.code, usage.stderr)
	}
	if unknown := runCLI(t, root, "", "release", "notes", "EPIC-91"); unknown.code == 0 || !strings.Contains(unknown.stderr, "no epic EPIC-91") {
		t.Fatalf("want an unknown epic refused, got %d: %s", unknown.code, unknown.stderr)
	}
}

func TestReleaseNotesRefusesAnEpicWithNoVersion(t *testing.T) {
	root := t.TempDir()
	if _, err := backlog.WriteEpic(root, backlog.EpicFile{ID: "EPIC-92", Title: "No version", Status: "READY", GroupsMax: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := releaseNotes(root, "EPIC-92", time.Now()); err == nil || !strings.Contains(err.Error(), "no version") {
		t.Fatalf("err = %v, want an epic with no version refused", err)
	}
}

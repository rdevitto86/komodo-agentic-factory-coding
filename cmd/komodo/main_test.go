package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"komodo/internal/changelog"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/profile"
)

func TestSplitTaskArgFindsTheTaskAfterTheRoleFlag(t *testing.T) {
	task, rest := splitTaskArg([]string{"--role", "reviewer", "TSK-12.1.1"})
	if task != "TSK-12.1.1" {
		t.Fatalf("task = %q", task)
	}
	if !reflect.DeepEqual(rest, []string{"--role", "reviewer"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestSplitTaskArgFindsTheTaskBeforeTheRoleFlag(t *testing.T) {
	task, rest := splitTaskArg([]string{"TSK-12.1.1", "--role", "reviewer"})
	if task != "TSK-12.1.1" {
		t.Fatalf("task = %q", task)
	}
	if !reflect.DeepEqual(rest, []string{"--role", "reviewer"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestLocalModelRejectsAHeavyTierWhenOnlyLightRunsLocally(t *testing.T) {
	tiers := mount.Tiers{
		Light:    mount.Machine{Provider: "ollama", Model: "llama3.2"},
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
		Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
	}
	_, err := localModel(tiers, "heavy")
	if err == nil {
		t.Fatal("want an error naming light as the fallback tier, got none")
	}
	if !strings.Contains(err.Error(), "light") {
		t.Fatalf("err = %q, want it to name light as the fallback tier", err.Error())
	}
}

func TestLocalModelResolvesTheTierThatMountsTheLocalMachine(t *testing.T) {
	tiers := mount.Tiers{
		Light:    mount.Machine{Provider: "ollama", Model: "llama3.2"},
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
	}
	model, err := localModel(tiers, "light")
	if err != nil {
		t.Fatal(err)
	}
	if model != "llama3.2" {
		t.Fatalf("model = %q, want llama3.2", model)
	}
}

func TestLocalModelResolvesTheReviewerTierThroughTiersReviewer(t *testing.T) {
	tiers := mount.Tiers{
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
		Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
		Reviewer: mount.Machine{Provider: "ollama", Model: "llama3.2"},
	}
	model, err := localModel(tiers, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if model != "llama3.2" {
		t.Fatalf("model = %q, want llama3.2", model)
	}
}

func TestLocalModelReviewerErrorNamesWhatActuallyHappens(t *testing.T) {
	tiers := mount.Tiers{
		Light:    mount.Machine{Provider: "ollama", Model: "llama3.2"},
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
		Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
		Reviewer: mount.Machine{Provider: "claude", Model: "opus"},
	}
	_, err := localModel(tiers, "reviewer")
	if err == nil {
		t.Fatal("want an error, got none")
	}
	if strings.Contains(err.Error(), "falls back") {
		t.Fatalf("err = %q, want it to say what actually happens, not claim a fallback", err.Error())
	}
	if !strings.Contains(err.Error(), "light") {
		t.Fatalf("err = %q, want it to name light as the tier that does mount", err.Error())
	}
}

func TestSplitFlagsKeepsFlagsAfterEveryPositional(t *testing.T) {
	positional, rest := splitFlags(
		[]string{"mygroup", "Add", "the", "feature", "--files", "a.go,b.go"},
		"files", "done-when", "priority", "status", "type",
	)
	if !reflect.DeepEqual(positional, []string{"mygroup", "Add", "the", "feature"}) {
		t.Fatalf("positional = %v", positional)
	}
	if !reflect.DeepEqual(rest, []string{"--files", "a.go,b.go"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestVerifyPathsFailsOnAPathThatDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyPaths(dir, []string{"a.go"}); err != nil {
		t.Fatal(err)
	}
	if err := verifyPaths(dir, []string{"missing.go"}); err == nil {
		t.Fatal("want an error naming the missing path, got none")
	}
}

func TestCommentsArgsStripsCheckAndKeepsFlagsSeparate(t *testing.T) {
	paths, rest := commentsArgs([]string{"check", "a.go", "b.go", "--require", "exported"}, "require")
	if !reflect.DeepEqual(paths, []string{"a.go", "b.go"}) {
		t.Fatalf("paths = %v", paths)
	}
	if !reflect.DeepEqual(rest, []string{"--require", "exported"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestPrintCompactJSONWritesOneLineWithNoIndent(t *testing.T) {
	var buf bytes.Buffer
	printCompactJSON(&buf, map[string]any{"a": 1, "b": []int{1, 2}})
	got := buf.String()
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("output = %q, want exactly one newline", got)
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("output = %q, want no indentation", got)
	}
}

func TestPlanForJSONDropsRoleDescriptionsAndTheWholeProfile(t *testing.T) {
	plan := &line.Plan{
		Group: "TG-1",
		Roles: []line.Role{{
			Name: "builder", Tier: "standard", Machine: "claude/sonnet",
			Description: "a long description no station reads",
		}},
		Profile: profile.Profile{Name: "big-plan"},
	}
	data, err := json.Marshal(planForJSON(plan))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "description") || strings.Contains(got, "profile") {
		t.Fatalf("output = %q, want no role description or profile", got)
	}
	if !strings.Contains(got, "claude/sonnet") {
		t.Fatalf("output = %q, want the resolved machine", got)
	}
}

const shippedGroup = "### [TG-90.1] A shipped group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-90.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n"

const pendingGroup = "### [TG-90.2] A pending group\n```yaml\ntype: feat\nversion: 3.0.0\n```\n\n" +
	"#### [TSK-90.2.1] Not done [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"true\"]\n```\n"

const releaseChangelog = "# Changelog\n\n## 2.0.0 — 2026-09-22\n\n- shipped\n"

// runGit runs one git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestCheckReleaseSkipsAGroupWhoseTasksAreNotAllDone(t *testing.T) {
	root := t.TempDir()
	backlog := shippedGroup + pendingGroup
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(releaseChangelog), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := checkRelease(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range drift {
		if item.Subject == "3.0.0" {
			t.Fatalf("drift = %+v; a group whose tasks are not all DONE must not be checked", drift)
		}
	}
}

// tagRepo builds a repo with a bare origin, checked out on branch, carrying the changelog.
func tagRepo(t *testing.T, branch, changelog string) (root, bare string) {
	t.Helper()
	root = t.TempDir()
	bare = filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "checkout", "-b", branch)
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("* text=auto eol=lf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed")
	return root, bare
}

func TestFoldRefusesTheDefaultBranchAndFoldsOnAnother(t *testing.T) {
	root, _ := tagRepo(t, "main", releaseChangelog)
	if err := changelog.WriteFragment(root, "2.1.0", "TG-05.2", "- **TG-05.2** The host contract (3 task(s))"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := fold(root, &out); err == nil || !strings.Contains(err.Error(), "refusing to fold") {
		t.Fatalf("err = %v; fold must refuse the default branch", err)
	}
	runGit(t, root, "checkout", "-q", "-b", "chore/fold-the-changelog")
	if err := fold(root, &out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if !strings.HasPrefix(string(data), "# Changelog\n\n## 2.1.0 — ") || !strings.Contains(string(data), "- **TG-05.2** The host contract") {
		t.Fatalf("CHANGELOG.md after the fold:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(root, changelog.Dir)); !os.IsNotExist(err) {
		t.Fatalf("the fragments are still there: %v", err)
	}
}

func TestUntaggedVersionsNamesAChangelogVersionOriginHasNoTagFor(t *testing.T) {
	root, _ := tagRepo(t, "main", releaseChangelog)
	pending, err := untaggedVersions(root)
	if err != nil || len(pending) != 1 || pending[0] != "2.0.0" {
		t.Fatalf("pending = %v, err = %v; 2.0.0 has no tag on origin", pending, err)
	}
	runGit(t, root, "tag", "-a", "v2.0.0", "-m", "release 2.0.0")
	runGit(t, root, "push", "-q", "origin", "v2.0.0")
	if pending, err := untaggedVersions(root); err != nil || len(pending) != 0 {
		t.Fatalf("pending = %v, err = %v; origin holds v2.0.0", pending, err)
	}
}

func TestTagTagsOnlyTheNewestUnreleasedVersionAtHead(t *testing.T) {
	changelog := "# Changelog\n\n## 3.0.0 — 2026-09-24\n\n- new\n\n" + strings.TrimPrefix(releaseChangelog, "# Changelog\n\n")
	root, bare := tagRepo(t, "main", changelog)
	var out bytes.Buffer
	if err := tag(root, &out); err != nil {
		t.Fatal(err)
	}
	remote, err := exec.Command("git", "ls-remote", "--tags", bare).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remote), "v3.0.0") || strings.Contains(string(remote), "v2.0.0") {
		t.Fatalf("origin tags = %q; only the newest version is tagged at HEAD", remote)
	}
	if !strings.Contains(out.String(), "2.0.0: not tagged") {
		t.Fatalf("out = %q; the older untagged version is named, not tagged", out.String())
	}
}

func TestTagRefusesToTagOffANonDefaultBranch(t *testing.T) {
	root, bare := tagRepo(t, "feat/other", releaseChangelog)
	var out bytes.Buffer
	if err := tag(root, &out); err == nil {
		t.Fatal("want an error refusing a non-default branch, got none")
	} else if !strings.Contains(err.Error(), "feat/other") {
		t.Fatalf("err = %q, want it to name the branch", err.Error())
	}
	remote, err := exec.Command("git", "ls-remote", "--tags", bare).Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(remote) != 0 {
		t.Fatalf("origin tags = %q, want none pushed off a non-default branch", remote)
	}
}

func TestTagRetriesAPushAfterALocalTagSurvivedAFailedPush(t *testing.T) {
	root, bare := tagRepo(t, "main", releaseChangelog)
	runGit(t, root, "tag", "-a", "v2.0.0", "-m", "release 2.0.0")
	var out bytes.Buffer
	if err := tag(root, &out); err != nil {
		t.Fatal(err)
	}
	remote, err := exec.Command("git", "ls-remote", "--tags", bare).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remote), "v2.0.0") {
		t.Fatalf("origin tags = %q; a local tag a failed push left behind must still reach origin", remote)
	}
}

func TestAFlagAfterTheTargetIsStillParsed(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		valueFlags []string
		positional string
		rest       []string
	}{
		{"flag after the target", []string{"TG-03.6", "--dry-run"}, nil, "TG-03.6", []string{"--dry-run"}},
		{"flag before the target", []string{"--dry-run", "TG-03.6"}, nil, "TG-03.6", []string{"--dry-run"}},
		{"target only", []string{"TG-03.6"}, nil, "TG-03.6", []string{}},
		{"no target", []string{"--dry-run"}, nil, "", []string{"--dry-run"}},
		{"value flag keeps its value", []string{"TSK-1", "--role", "reviewer"}, []string{"role"}, "TSK-1", []string{"--role", "reviewer"}},
		{"value flag before the target", []string{"--budget", "5m", "TG-03.6"}, []string{"budget"}, "TG-03.6", []string{"--budget", "5m"}},
		{"equals form is self contained", []string{"TG-03.6", "--budget=5m"}, []string{"budget"}, "TG-03.6", []string{"--budget=5m"}},
		{"only the first positional is taken", []string{"a", "b"}, nil, "a", []string{"b"}},
	}
	for _, each := range cases {
		got, rest := splitPositional(each.args, each.valueFlags...)
		if got != each.positional {
			t.Fatalf("%s: positional = %q, want %q", each.name, got, each.positional)
		}
		if len(rest) != len(each.rest) {
			t.Fatalf("%s: rest = %v, want %v", each.name, rest, each.rest)
		}
		for i := range rest {
			if rest[i] != each.rest[i] {
				t.Fatalf("%s: rest = %v, want %v", each.name, rest, each.rest)
			}
		}
	}
}

func TestARepairableCloseExitsZeroSoTheLoopContinues(t *testing.T) {
	for status, want := range map[string]int{"DONE": 0, "IN_PROGRESS": 0, "BLOCKED": 1} {
		if got := closeExitCode(status); got != want {
			t.Fatalf("closeExitCode(%s) = %d, want %d", status, got, want)
		}
	}
}

package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testSuite is the suite under testdata, since the real one in eval/ is locked to line sessions.
var testSuite = filepath.Join("testdata", "suite")

// TestLoadReadsEveryRepoAndGroup proves each group names its repo, pinned commit, group file and hidden tests.
func TestLoadReadsEveryRepoAndGroup(t *testing.T) {
	suite, err := Load(testSuite)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Repos) != 2 || len(suite.Groups) != 2 {
		t.Fatalf("repos = %d, groups = %d, want 2 and 2", len(suite.Repos), len(suite.Groups))
	}
	group := suite.Groups[0]
	if group.ID != "TG-01.1" || group.Commit != strings.Repeat("1", 40) || group.File != "greet/TG-01.1.md" {
		t.Fatalf("group = %+v", group)
	}
	if len(group.HiddenTests) != 1 || group.HiddenTests[0] != "greet/greet_test.go" || group.Test == "" {
		t.Fatalf("hidden tests = %q, test = %q", group.HiddenTests, group.Test)
	}
	if repo := suite.Repo(group); repo.Language != "Go" || repo.URL == "" {
		t.Fatalf("repo = %+v", repo)
	}
}

// TestLoadRefusesAGroupThatIsNotPinnedAndWhole proves a moving ref, an unknown repo or a missing file fails the load.
func TestLoadRefusesAGroupThatIsNotPinnedAndWhole(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Group)
		wants string
	}{
		{"a branch for a commit", func(g *Group) { g.Commit = "main" }, "not a full commit hash"},
		{"a short commit", func(g *Group) { g.Commit = "1111111" }, "not a full commit hash"},
		{"an unknown repo", func(g *Group) { g.Repo = "nope" }, `repo "nope"`},
		{"no hidden tests", func(g *Group) { g.HiddenTests = nil }, "no hidden tests"},
		{"no test command", func(g *Group) { g.Test = " " }, "no command"},
		{"a missing group file", func(g *Group) { g.File = "greet/TG-09.9.md" }, "TG-09.9.md"},
		{"a missing hidden test", func(g *Group) { g.HiddenTests = []string{"greet/gone_test.go"} }, "gone_test.go"},
		{"a group file outside the suite", func(g *Group) { g.File = "../suite.json" }, "outside the suite"},
		{"a hidden test outside the repo", func(g *Group) { g.HiddenTests = []string{"../x_test.go"} }, "not a repo path"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			dir := copySuite(t)
			suite := readIndex(t, dir)
			each.edit(&suite.Groups[0])
			writeIndex(t, dir, suite)
			if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), each.wants) {
				t.Fatalf("Load = %v, want an error naming %q", err, each.wants)
			}
		})
	}
}

// TestLoadRefusesABrokenIndex proves an unreadable index, a half-named repo, or a name used twice fails the load.
func TestLoadRefusesABrokenIndex(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(dir string, suite *Suite)
		wants string
	}{
		{"no index", func(dir string, _ *Suite) {
			if err := os.Remove(filepath.Join(dir, SuiteFile)); err != nil {
				t.Fatal(err)
			}
		}, SuiteFile},
		{"a repo with no url", func(_ string, s *Suite) { s.Repos[0].URL = "" }, "needs a name, a url and a language"},
		{"a repo named twice", func(_ string, s *Suite) { s.Repos[1].Name = s.Repos[0].Name }, "named twice"},
		{"a group named twice", func(_ string, s *Suite) { s.Groups[1] = s.Groups[0] }, "greet/TG-01.1 is named twice"},
		{"a group with no id", func(_ string, s *Suite) { s.Groups[0].ID = "" }, "has no id"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			dir := copySuite(t)
			suite := readIndex(t, dir)
			each.edit(dir, &suite)
			if _, err := os.Stat(filepath.Join(dir, SuiteFile)); err == nil {
				writeIndex(t, dir, suite)
			}
			if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), each.wants) {
				t.Fatalf("Load = %v, want an error naming %q", err, each.wants)
			}
		})
	}
	dir := copySuite(t)
	if err := os.WriteFile(filepath.Join(dir, SuiteFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), SuiteFile) {
		t.Fatalf("Load = %v, want the index's parse error", err)
	}
	if repo := (Suite{}).Repo(Group{Repo: "nope"}); repo != (Repo{}) {
		t.Fatalf("Repo = %+v, want nothing for an unknown repo", repo)
	}
}

// TestListPrintsTheGroupsAndTheirPinnedCommits proves --list's output, and that a small suite fails the size bar.
func TestListPrintsTheGroupsAndTheirPinnedCommits(t *testing.T) {
	suite, err := Load(testSuite)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	err = suite.List(&out)
	for _, want := range []string{
		"greet (Go) https://example.invalid/greet.git",
		"TG-01.1  " + strings.Repeat("1", 40) + "  greet/TG-01.1.md  1 hidden tests",
		"hello (TypeScript)",
		"TG-02.1  " + strings.Repeat("2", 40),
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("list = %q, want %q", out.String(), want)
		}
	}
	if !errors.Is(err, ErrShort) || !strings.Contains(err.Error(), "greet holds 1 groups, want at least 10") {
		t.Fatalf("List = %v, want ErrShort naming each repo's group count", err)
	}
}

// TestShortIsEmptyForAGoldenSizedSuite proves the size bar: two repos, Go and TypeScript, ten groups each.
func TestShortIsEmptyForAGoldenSizedSuite(t *testing.T) {
	suite := Suite{Repos: []Repo{{Name: "api", Language: "Go"}, {Name: "web", Language: "TypeScript"}}}
	for _, repo := range []string{"api", "web"} {
		for index := range MinGroups {
			suite.Groups = append(suite.Groups, Group{ID: fmt.Sprintf("TG-01.%d", index+1), Repo: repo})
		}
	}
	if short := suite.Short(); len(short) != 0 {
		t.Fatalf("Short = %q, want nothing", short)
	}
	suite.Repos[1].Language = "Go"
	if short := suite.Short(); len(short) != 1 || short[0] != "no TypeScript repo" {
		t.Fatalf("Short = %q, want the missing language", short)
	}
}

// copySuite copies the testdata suite into a temp dir a test may edit.
func copySuite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := filepath.WalkDir(testSuite, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(testSuite, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// readIndex decodes a suite's index without validating it.
func readIndex(t *testing.T, dir string) Suite {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, SuiteFile))
	if err != nil {
		t.Fatal(err)
	}
	var suite Suite
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatal(err)
	}
	return suite
}

// writeIndex encodes a suite's index into dir.
func writeIndex(t *testing.T, dir string, suite Suite) {
	t.Helper()
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, SuiteFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

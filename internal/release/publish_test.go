package release

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"komodo/internal/gate"
)

const betaChangelog = "# Changelog\n\n## 2.0.0-beta.1 — 2026-09-27\n\n- the beta\n\n## 1.3.0 — 2026-09-01\n\n- old\n"

// gitIn runs one git command in dir, failing the test on error.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// releaseRepo builds a committed repo holding the changelog, with its own .git, tagged at HEAD when tag is set.
func releaseRepo(t *testing.T, tag string) string {
	t.Helper()
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "a@example.com")
	gitIn(t, root, "config", "user.name", "a")
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(betaChangelog), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "CHANGELOG.md")
	gitIn(t, root, "commit", "-q", "-m", "seed")
	if tag != "" {
		gitIn(t, root, "tag", "-a", tag, "-m", "release")
	}
	return root
}

// publishCall records what one fake publish saw.
type publishCall struct {
	built, tested int
	forge         [][]string
}

// fakePublish swaps the build, the tests and the forge for fakes, restoring them when the test ends,
// and clears the session markers so the test runs as the owner's machine.
func fakePublish(t *testing.T, testErr error) *publishCall {
	t.Helper()
	for _, key := range []string{"GIT_TERMINAL_PROMPT", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0"} {
		t.Setenv(key, "")
	}
	call := &publishCall{}
	oldBuild, oldTests, oldForge := buildAssets, runTests, forge
	t.Cleanup(func() { buildAssets, runTests, forge = oldBuild, oldTests, oldForge })
	buildAssets = func(_, dir string, _ io.Writer) ([]string, error) {
		call.built++
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		var paths []string
		for _, name := range []string{"komodo-linux-amd64", "komodo-darwin-arm64"} {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte(name), 0o755); err != nil {
				return nil, err
			}
			paths = append(paths, path)
		}
		return paths, nil
	}
	runTests = func(string, io.Writer) error {
		call.tested++
		return testErr
	}
	forge = func(_ string, args ...string) (string, error) {
		call.forge = append(call.forge, args)
		return "https://example.com/releases/v2.0.0-beta.1", nil
	}
	return call
}

func TestPublishBuildsTestsChecksumsAndPublishes(t *testing.T) {
	root := releaseRepo(t, "v2.0.0-beta.1")
	call := fakePublish(t, nil)
	dist := filepath.Join(root, "dist")
	var out bytes.Buffer
	url, err := Publish(root, dist, &out)
	if err != nil {
		t.Fatal(err)
	}
	if url == "" || call.built != 1 || call.tested != 1 || len(call.forge) != 1 {
		t.Fatalf("url = %q, built = %d, tested = %d, forge = %v", url, call.built, call.tested, call.forge)
	}
	args := call.forge[0]
	if !slices.Equal(args[:4], []string{"release", "create", "v2.0.0-beta.1", "--verify-tag"}) {
		t.Fatalf("args = %v", args)
	}
	wants := []string{"--prerelease", filepath.Join(dist, "komodo-linux-amd64"), filepath.Join(dist, SumsFile)}
	for _, want := range wants {
		if !slices.Contains(args, want) {
			t.Fatalf("args = %v, want %q", args, want)
		}
	}
	notes := args[slices.Index(args, "--notes")+1]
	if !strings.Contains(notes, "the beta") || strings.Contains(notes, "old") {
		t.Fatalf("notes = %q; they are the released version's changelog body", notes)
	}
	sums, err := os.ReadFile(filepath.Join(dist, SumsFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "  komodo-darwin-arm64") {
		t.Fatalf("SHA256SUMS = %q", sums)
	}
	want, err := gate.Sum(filepath.Join(dist, "komodo-linux-amd64"))
	if err != nil {
		t.Fatal(err)
	}
	if lines[1] != want+"  komodo-linux-amd64" {
		t.Fatalf("SHA256SUMS line = %q, want %q", lines[1], want+"  komodo-linux-amd64")
	}
}

func TestPublishRefusesAChangelogWithNoVersion(t *testing.T) {
	root := t.TempDir()
	call := fakePublish(t, nil)
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Publish(root, filepath.Join(root, "dist"), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no version") {
		t.Fatalf("err = %v", err)
	}
	if call.built != 0 || len(call.forge) != 0 {
		t.Fatalf("built = %d, forge = %v; nothing runs without a version", call.built, call.forge)
	}
}

func TestPublishFailsOutsideAGitRepo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	call := fakePublish(t, nil)
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(betaChangelog), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(root, filepath.Join(root, "dist"), io.Discard); err == nil {
		t.Fatal("publish outside a git repo must fail")
	}
	if call.built != 0 || len(call.forge) != 0 {
		t.Fatalf("built = %d, forge = %v; nothing runs outside a repo", call.built, call.forge)
	}
}

func TestWriteSumsFailsOnAMissingAssetOrDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteSums(dir, []string{filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("a missing asset must fail")
	}
	asset := filepath.Join(dir, "komodo-linux-amd64")
	if err := os.WriteFile(asset, []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSums(filepath.Join(dir, "absent"), []string{asset}); err == nil {
		t.Fatal("a manifest into a missing directory must fail")
	}
}

func TestReleaseNotesIsEmptyForAnUnnamedVersion(t *testing.T) {
	if got := releaseNotes(betaChangelog, "9.9.9"); got != "" {
		t.Fatalf("notes = %q", got)
	}
}

func TestGoTestReportsTheSuitesOutcome(t *testing.T) {
	cases := []struct {
		name string
		body string
		fail bool
	}{
		{name: "a passing suite", body: "func TestOK(t *testing.T) {}\n"},
		{name: "a failing suite", body: "func TestBad(t *testing.T) { t.Fatal(\"boom\") }\n", fail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"go.mod":     "module fixture\n\ngo 1.22\n\ntoolchain " + runtime.Version() + "\n",
				"x_test.go":  "package fixture\n\nimport \"testing\"\n\n" + tc.body,
				"fixture.go": "package fixture\n",
			}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			err := goTest(root, &out)
			if (err != nil) != tc.fail {
				t.Fatalf("err = %v, want failure %v; out = %s", err, tc.fail, out.String())
			}
		})
	}
}

// TestGoTestKillsAHungSuiteAndNamesTheTimeout proves a suite past gate.CommandTimeout is killed,
// process group included, and the error names the command, the timeout and its stderr.
func TestGoTestKillsAHungSuiteAndNamesTheTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n\ntoolchain "+runtime.Version()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = test ]; then echo boom >&2; sleep 30 & sleep 30; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(fakeDir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))
	saved := gate.CommandTimeout
	gate.CommandTimeout = 200 * time.Millisecond
	t.Cleanup(func() { gate.CommandTimeout = saved })
	var out bytes.Buffer
	started := time.Now()
	err := goTest(root, &out)
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want it to name the timeout and stderr", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}

func TestPublishRefusesBeforeTheForge(t *testing.T) {
	cases := []struct {
		name    string
		tag     string
		prepare func(t *testing.T, root string)
		testErr error
		want    string
	}{
		{name: "no tag", want: "no tag v2.0.0-beta.1"},
		{name: "a tag off HEAD", tag: "v2.0.0-beta.1", want: "not HEAD", prepare: func(t *testing.T, root string) {
			gitIn(t, root, "commit", "-q", "--allow-empty", "-m", "after")
		}},
		{name: "a dirty tree", tag: "v2.0.0-beta.1", want: "uncommitted changes", prepare: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(betaChangelog+"\n- more\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "failing tests", tag: "v2.0.0-beta.1", testErr: errors.New("boom"), want: "tests failed"},
		{name: "a failed build", tag: "v2.0.0-beta.1", want: "build broke", prepare: func(*testing.T, string) {
			buildAssets = func(string, string, io.Writer) ([]string, error) { return nil, errors.New("build broke") }
		}},
		{name: "a forge that refuses", tag: "v2.0.0-beta.1", want: "forge down", prepare: func(*testing.T, string) {
			forge = func(string, ...string) (string, error) { return "", errors.New("forge down") }
		}},
		{name: "a line session", tag: "v2.0.0-beta.1", want: "line session", prepare: func(t *testing.T, _ string) {
			t.Setenv("GIT_TERMINAL_PROMPT", "0")
			t.Setenv("GIT_CONFIG_KEY_0", "credential.helper")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := releaseRepo(t, tc.tag)
			call := fakePublish(t, tc.testErr)
			if tc.prepare != nil {
				tc.prepare(t, root)
			}
			_, err := Publish(root, filepath.Join(t.TempDir(), "dist"), io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if len(call.forge) != 0 {
				t.Fatalf("the forge ran: %v", call.forge)
			}
		})
	}
}

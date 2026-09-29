package eval

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/conductor"
	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/run"
)

// liveEnv opts into the live eval, which spends real plan tokens.
const liveEnv = "KOMODO_LIVE"

// pinnedCommit is the commit the offline suite pins; the fake clone only records it.
var pinnedCommit = strings.Repeat("a", 40)

// offlineSuite writes a one-group suite whose hidden test passes only when greet.txt says hello.
func offlineSuite(t *testing.T) Suite {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"greet/TG-01.1.md":          "### [TG-01.1] Greet\n",
		"greet/TG-01.1/check.sh":    "grep -q hello greet.txt\n",
		"greet/TG-01.1/sub/keep.sh": "true\n",
	}
	for name, body := range files {
		if err := copyBytes(filepath.Join(dir, name), body); err != nil {
			t.Fatal(err)
		}
	}
	return Suite{
		Dir:   dir,
		Repos: []Repo{{Name: "greet", URL: "https://example.invalid/greet.git", Language: "Go"}},
		Groups: []Group{{
			ID: "TG-01.1", Repo: "greet", Commit: pinnedCommit, File: "greet/TG-01.1.md",
			Hidden: "greet/TG-01.1", HiddenTests: []string{"check.sh", "sub/keep.sh"}, Test: "sh check.sh",
		}},
	}
}

// copyBytes writes body to path, making its directories.
func copyBytes(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// fakeClone makes an empty directory where a clone would be, its own group file already shipped,
// and records what it was asked for.
func fakeClone(asked *[]string) Clone {
	return func(_ context.Context, url, commit, dir string) error {
		*asked = append(*asked, url+"@"+commit)
		return copyBytes(filepath.Join(dir, "docs", "backlog", "TG-00.1-shipped.md"),
			"## [TG-00.1] Shipped long ago [P: M] [DONE]\n\n```yaml\ntype: feat\n```\n")
	}
}

// fakeLine writes answer into worktree, stamps a build and a review, and hands off a ship when ship is set.
type fakeLine struct {
	answer   string
	ship     bool
	worktree string
	err      error
}

func (f fakeLine) drive(t *testing.T) Line {
	return func(_ context.Context, dir string, group Group) error {
		shipped, err := os.ReadFile(filepath.Join(dir, "docs", "backlog", "TG-00.1-shipped.md"))
		if err != nil || !strings.Contains(string(shipped), "[TG-00.1] Shipped long ago") {
			t.Errorf("shipped group file = %q, %v; the clone's own group file must survive", shipped, err)
		}
		added, err := os.ReadFile(filepath.Join(dir, "docs", "backlog", "TG-01.1-greet.md"))
		if err != nil || !strings.Contains(string(added), "[TG-01.1] Greet") {
			t.Errorf("added group file = %q, %v; the suite's group file must join before the line runs", added, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "check.sh")); err == nil {
			t.Error("a hidden test reached the clone before the line ran")
		}
		worktree := filepath.Join(dir, f.worktree)
		if err := copyBytes(filepath.Join(worktree, "greet.txt"), f.answer+"\n"); err != nil {
			return err
		}
		book := line.Book(dir)
		for _, entry := range []ledger.Entry{
			{Group: group.ID, Station: conductor.StationBuild, Role: "builder", Turns: 3, TokensIn: 100, TokensOut: 50},
			{Group: group.ID, Station: conductor.StationReview, Role: "reviewer", Turns: 2, TokensIn: 40, TokensOut: 10},
			{Group: group.ID, Station: "ship", Outcome: "handoff"},
		} {
			if err := book.Stamp(entry); err != nil {
				return err
			}
		}
		if f.ship {
			data, err := json.Marshal(line.ShipHandoff{Group: group.ID, Worktree: worktree, Branch: "feat/greet"})
			if err != nil {
				return err
			}
			if err := copyBytes(line.HandoffPath(dir, group.ID), string(data)); err != nil {
				return err
			}
		}
		return f.err
	}
}

// TestRunJudgesEachRunByItsShipAndItsHiddenTests proves each run's verdict and cost come from the fresh clone.
func TestRunJudgesEachRunByItsShipAndItsHiddenTests(t *testing.T) {
	outside := t.TempDir()
	cases := []struct {
		name     string
		line     fakeLine
		shipped  bool
		passed   bool
		errorHas string
	}{
		{"shipped with the right answer", fakeLine{answer: "hello", ship: true, worktree: ".komodo/wt/greet"},
			true, true, ""},
		{"shipped from the clone's root", fakeLine{answer: "hello", ship: true}, true, true, ""},
		{"shipped with the wrong answer", fakeLine{answer: "bye", ship: true}, true, false, "hidden tests"},
		{"never reached ship", fakeLine{answer: "hello"}, false, false, ""},
		{"the line failed", fakeLine{answer: "hello", err: errors.New("escalated")}, false, false, "escalated"},
		{"a handoff outside the clone", fakeLine{answer: "hello", ship: true, worktree: "../../" + filepath.Base(outside)},
			false, false, "outside the clone"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			var asked []string
			var out strings.Builder
			outcomes, err := Run(context.Background(), Options{
				Suite: offlineSuite(t), Runs: 2, Work: t.TempDir(), Platform: "plan9",
				Clone: fakeClone(&asked), Line: each.line.drive(t), Stdout: &out,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(outcomes) != 2 || len(asked) != 2 || asked[0] != "https://example.invalid/greet.git@"+pinnedCommit {
				t.Fatalf("outcomes = %d, clones = %q; want two runs, each a fresh clone at the pinned commit", len(outcomes), asked)
			}
			for index, outcome := range outcomes {
				if outcome.Run != index+1 || outcome.Platform != "plan9" || outcome.Repo != "greet" || outcome.Group != "TG-01.1" {
					t.Fatalf("outcome = %+v", outcome)
				}
				if outcome.Shipped != each.shipped || outcome.Passed != each.passed {
					t.Fatalf("shipped = %v, passed = %v, want %v and %v (%s)",
						outcome.Shipped, outcome.Passed, each.shipped, each.passed, outcome.Error)
				}
				if !strings.Contains(outcome.Error, each.errorHas) || (each.errorHas == "" && outcome.Error != "") {
					t.Fatalf("error = %q, want %q", outcome.Error, each.errorHas)
				}
				if outcome.Sessions != 2 || outcome.Turns != 5 || outcome.Tokens != 200 || outcome.ReviewRounds != 1 {
					t.Fatalf("cost = %+v, want the clone's own ledger: 2 sessions, 5 turns, 200 tokens, 1 review", outcome)
				}
			}
			verdict := "rejected"
			if each.shipped && each.passed {
				verdict = "accepted"
			}
			if !strings.Contains(out.String(), "greet/TG-01.1 run 2 on plan9: "+verdict) {
				t.Fatalf("output = %q", out.String())
			}
		})
	}
}

// TestRunStartsABacklogAndRejectsABrokenHandoff proves a clone with no backlog gets one, and a handoff the
// eval cannot read or place rejects the run.
func TestRunStartsABacklogAndRejectsABrokenHandoff(t *testing.T) {
	cases := []struct {
		name     string
		handoff  string
		errorHas string
	}{
		{"no handoff", "", ""},
		{"an unreadable handoff", "{", "the ship handoff"},
		{"a worktree that does not exist", `{"worktree": "gone"}`, "does not exist"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			bare := func(_ context.Context, _, _, dir string) error { return os.MkdirAll(dir, 0o755) }
			drive := func(_ context.Context, dir string, group Group) error {
				added, err := os.ReadFile(filepath.Join(dir, "docs", "backlog", "TG-01.1-greet.md"))
				if err != nil || !strings.Contains(string(added), "[TG-01.1] Greet") {
					t.Errorf("added group file = %q, %v; a clone with no backlog starts one", added, err)
				}
				if each.handoff == "" {
					return nil
				}
				return copyBytes(line.HandoffPath(dir, group.ID), each.handoff)
			}
			outcomes, err := Run(context.Background(), Options{
				Suite: offlineSuite(t), Runs: 1, Work: t.TempDir(), Clone: bare, Line: drive,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := outcomes[0]
			if got.Shipped || got.Passed || !strings.Contains(got.Error, each.errorHas) {
				t.Fatalf("outcome = %+v, want a rejection naming %q", got, each.errorHas)
			}
		})
	}
}

// TestRunFailsOnAFileTheSuiteLacks proves a missing group file or hidden test ends the eval.
func TestRunFailsOnAFileTheSuiteLacks(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Group)
	}{
		{"the group file", func(g *Group) { g.File = "greet/gone.md" }},
		{"a hidden test", func(g *Group) { g.HiddenTests = []string{"gone.sh"} }},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			suite := offlineSuite(t)
			each.edit(&suite.Groups[0])
			var asked []string
			_, err := Run(context.Background(), Options{
				Suite: suite, Runs: 1, Work: t.TempDir(), Clone: fakeClone(&asked),
				Line: fakeLine{answer: "hello", ship: true}.drive(t),
			})
			if err == nil || !strings.Contains(err.Error(), "gone") {
				t.Fatalf("Run = %v, want the missing file named", err)
			}
		})
	}
}

// TestLaunchFailsNamingTheStepThatFailed proves the live line stops at its first failing step.
func TestLaunchFailsNamingTheStepThatFailed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "komodo")
	err := Launch(missing, "", time.Minute)(context.Background(), t.TempDir(), Group{ID: "TG-01.1"})
	if err == nil || !strings.Contains(err.Error(), missing+" install") {
		t.Fatalf("Launch = %v, want the install step named", err)
	}
}

// TestLaunchInstallsCommitsPushesAndRunsWithNoShip proves the live line's steps against a stand-in komodo.
func TestLaunchInstallsCommitsPushesAndRunsWithNoShip(t *testing.T) {
	source, parent, _ := sourceRepo(t)
	dir := filepath.Join(t.TempDir(), "clone")
	if err := CloneAt(context.Background(), source, parent, dir); err != nil {
		t.Fatal(err)
	}
	if err := copyBytes(filepath.Join(dir, "docs", "backlog", "TG-01.1-greet.md"),
		"## [TG-01.1] Greet [P: M] [READY]\n\n```yaml\ntype: feat\n```\n"); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(t.TempDir(), "calls")
	komodo := filepath.Join(t.TempDir(), "komodo")
	if err := os.WriteFile(komodo, []byte("#!/bin/sh\necho \"$@\" >> "+calls+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Launch(komodo, "h", time.Minute)(context.Background(), dir, Group{ID: "TG-01.1"}); err != nil {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(calls)
	if err != nil || string(logged) != "install --host h\nrun TG-01.1 --no-ship --budget 1m0s\n" {
		t.Fatalf("komodo calls = %q, %v", logged, err)
	}
	if subject := gitIn(t, dir+".origin.git", "log", "-1", "--format=%s", "main"); subject != "eval: TG-01.1" {
		t.Fatalf("origin main = %q, want the group's commit pushed to the local origin", subject)
	}
}

// TestRunStopsWhenAFreshCloneCannotBeBuilt proves a clone failure ends the eval instead of scoring a run.
func TestRunStopsWhenAFreshCloneCannotBeBuilt(t *testing.T) {
	broken := func(context.Context, string, string, string) error { return errors.New("no route") }
	_, err := Run(context.Background(), Options{
		Suite: offlineSuite(t), Runs: 1, Work: t.TempDir(), Clone: broken, Line: fakeLine{}.drive(t),
	})
	if err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("Run = %v, want the clone's failure", err)
	}
}

// TestRunRefusesNoLineAndNoRuns proves an eval needs a line and at least one run.
func TestRunRefusesNoLineAndNoRuns(t *testing.T) {
	cases := []Options{
		{Suite: offlineSuite(t), Runs: 1},
		{Suite: offlineSuite(t), Runs: 0, Line: fakeLine{}.drive(t)},
	}
	for _, options := range cases {
		if _, err := Run(context.Background(), options); err == nil {
			t.Fatalf("Run(%+v) = nil, want a refusal", options)
		}
	}
}

// gitIn runs one git command in dir and returns its trimmed output.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	args = append([]string{"-c", "user.name=eval", "-c", "user.email=eval@example.com"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// sourceRepo builds a repo whose parent commit holds a Go module and whose head adds greet and its test,
// returning the repo and both commits.
func sourceRepo(t *testing.T) (repo, parent, change string) {
	t.Helper()
	repo = t.TempDir()
	gitIn(t, repo, "init", "-q", "-b", "main")
	files := map[string]string{
		"go.mod":       "module example.com/greet\n\ngo 1.22\n",
		"greet/doc.go": "// Package greet greets people.\npackage greet\n",
	}
	for name, body := range files {
		if err := copyBytes(filepath.Join(repo, name), body); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "parent")
	parent = gitIn(t, repo, "rev-parse", "HEAD")
	greet := "package greet\n\n// Hello greets name.\n" +
		"func Hello(name string) string { return \"Hello, \" + name + \"!\" }\n"
	if err := copyBytes(filepath.Join(repo, "greet", "greet.go"), greet); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "change")
	return repo, parent, gitIn(t, repo, "rev-parse", "HEAD")
}

// TestCloneAtPinsTheCommitAndHidesTheChange proves a fresh clone sits at the parent on main, holds no later
// commit, and pushes only to a local origin.
func TestCloneAtPinsTheCommitAndHidesTheChange(t *testing.T) {
	source, parent, change := sourceRepo(t)
	dir := filepath.Join(t.TempDir(), "clone")
	if err := CloneAt(context.Background(), source, parent, dir); err != nil {
		t.Fatal(err)
	}
	if head := gitIn(t, dir, "rev-parse", "HEAD"); head != parent {
		t.Fatalf("HEAD = %s, want the pinned %s", head, parent)
	}
	if branch := gitIn(t, dir, "branch", "--show-current"); branch != "main" {
		t.Fatalf("branch = %q, want main", branch)
	}
	if err := exec.Command("git", "-C", dir, "cat-file", "-e", change).Run(); err == nil {
		t.Fatal("the change's own commit reached the clone")
	}
	if _, err := os.Stat(filepath.Join(dir, "greet", "greet.go")); err == nil {
		t.Fatal("the change's file reached the clone")
	}
	origin := gitIn(t, dir, "remote", "get-url", "origin")
	if origin != dir+".origin.git" {
		t.Fatalf("origin = %s, want the local bare copy", origin)
	}
	if err := copyBytes(filepath.Join(dir, "note.txt"), "x\n"); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "note")
	gitIn(t, dir, "push", "-q", "origin", "main")
	if pushed := gitIn(t, origin, "rev-parse", "main"); pushed != gitIn(t, dir, "rev-parse", "HEAD") {
		t.Fatalf("origin main = %s after a push; a push stays on the local origin", pushed)
	}
}

// TestLiveEvalAcceptsAGroupThroughRealSessions runs one golden-shaped group through the whole line on this host.
func TestLiveEvalAcceptsAGroupThroughRealSessions(t *testing.T) {
	if os.Getenv(liveEnv) != "1" {
		t.Skip("set KOMODO_LIVE=1 to run real sessions; this spends plan tokens")
	}
	source, parent, _ := sourceRepo(t)
	suite, err := Load(testSuite)
	if err != nil {
		t.Fatal(err)
	}
	group := suite.Groups[0]
	group.Commit = parent
	suite.Repos = []Repo{{Name: group.Repo, URL: source, Language: "Go"}}
	suite.Groups = []Group{group}
	binary := filepath.Join(t.TempDir(), "komodo")
	if out, err := exec.Command("go", "build", "-o", binary, "komodo/cmd/komodo").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	var out strings.Builder
	outcomes, err := Run(context.Background(), Options{
		Suite: suite, Runs: 1, Work: t.TempDir(), Line: Launch(binary, "", run.GroupBudget), Stdout: &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || !outcomes[0].Accepted() {
		t.Fatalf("outcomes = %+v, want the one run accepted\n%s", outcomes, out.String())
	}
	if outcomes[0].Sessions == 0 || outcomes[0].Tokens == 0 {
		t.Fatalf("outcome = %+v, want the sessions and tokens the ledger recorded", outcomes[0])
	}
}

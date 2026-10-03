package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"komodo/internal/conductor"
	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/run"
)

// caseGroup is the ready group every fake env holds.
const caseGroup = "TG-01.1"

// fakeEnv is a scripted live line: komodo and git are the test's, and every other call is recorded.
type fakeEnv struct {
	t        *testing.T
	dir      string
	komodo   func(ctx context.Context, extra []string, args ...string) Ran
	git      func(args ...string) Ran
	added    map[string]string
	removed  chan struct{}
	planted  string
	restored bool
	overlay  string
	// fail is what every fallible env call returns; removeErr and restoreErr are what the cleanups return.
	fail       error
	removeErr  error
	restoreErr error
}

func newFakeEnv(t *testing.T) *fakeEnv {
	t.Helper()
	return &fakeEnv{t: t, dir: t.TempDir(), added: map[string]string{}, removed: make(chan struct{})}
}

func (f *fakeEnv) Dir() string   { return f.dir }
func (f *fakeEnv) Group() string { return caseGroup }

func (f *fakeEnv) Komodo(ctx context.Context, extra []string, args ...string) Ran {
	return f.komodo(ctx, extra, args...)
}

func (f *fakeEnv) Git(_ context.Context, args ...string) Ran {
	if f.git == nil {
		return Ran{}
	}
	return f.git(args...)
}

func (f *fakeEnv) AddGroup(_ context.Context, id, body string) error {
	f.added[id] = body
	return f.fail
}

func (f *fakeEnv) Scratch(context.Context) (string, error) { return f.t.TempDir(), f.fail }

func (f *fakeEnv) PathWithout(names ...string) (string, error) {
	return "/without/" + strings.Join(names, ","), f.fail
}

func (f *fakeEnv) Credential(context.Context) ([]string, func() error, error) {
	return []string{"GH_CONFIG_DIR=/credential"}, func() error {
		if f.removeErr != nil {
			return f.removeErr
		}
		close(f.removed)
		return nil
	}, f.fail
}

func (f *fakeEnv) Overlay(_ context.Context, json string) ([]string, error) {
	f.overlay = json
	return []string{"HOME=/overlay"}, f.fail
}

func (f *fakeEnv) Plant(_ context.Context, text string) (func() error, error) {
	f.planted = text
	return func() error {
		f.restored = true
		return f.restoreErr
	}, f.fail
}

// stamp writes one ledger entry into the fake env's repo.
func (f *fakeEnv) stamp(entry ledger.Entry) {
	f.t.Helper()
	if err := line.Book(f.dir).Stamp(entry); err != nil {
		f.t.Fatal(err)
	}
}

// save writes the group's state.json into the fake env's repo.
func (f *fakeEnv) save(state conductor.State) {
	f.t.Helper()
	state.Group = caseGroup
	if err := conductor.SaveState(conductor.StatePath(f.dir, caseGroup), state); err != nil {
		f.t.Fatal(err)
	}
}

// write puts body at a path under the fake env's repo.
func (f *fakeEnv) write(rel, body string) {
	f.t.Helper()
	if err := copyBytes(filepath.Join(f.dir, rel), body); err != nil {
		f.t.Fatal(err)
	}
}

// fastWatch reads state.json every few milliseconds for one test.
func fastWatch(t *testing.T) {
	old := watchInterval
	watchInterval = 5 * time.Millisecond
	t.Cleanup(func() { watchInterval = old })
}

// caseNamed returns the one case with name.
func caseNamed(t *testing.T, name string) Case {
	t.Helper()
	for _, each := range Cases() {
		if each.Name == name {
			return each
		}
	}
	t.Fatalf("no case named %q", name)
	return Case{}
}

// verdict runs a case and checks it passed or failed with an error naming wants.
func verdict(t *testing.T, each Case, env Env, wants string) {
	t.Helper()
	err := each.Run(context.Background(), env)
	switch {
	case wants == "" && err != nil:
		t.Fatalf("%s = %v, want a pass", each.Name, err)
	case wants != "" && (err == nil || !strings.Contains(err.Error(), wants)):
		t.Fatalf("%s = %v, want a failure naming %q", each.Name, err, wants)
	}
}

// TestCasesCoverEveryRequirementAUnitTestCannotProve proves one case per requirement and one per preflight check.
func TestCasesCoverEveryRequirementAUnitTestCannotProve(t *testing.T) {
	byRequirement := map[string][]string{}
	names := map[string]bool{}
	for _, each := range Cases() {
		if each.Run == nil || each.Proof == "" || names[each.Name] {
			t.Fatalf("case %+v needs a unique name, a proof and a run", each)
		}
		names[each.Name] = true
		byRequirement[each.Requirement] = append(byRequirement[each.Requirement], each.Name)
	}
	for _, requirement := range []string{"REQ-3", "REQ-12", "REQ-14", "REQ-27", "REQ-32", "REQ-34", "REQ-40"} {
		if len(byRequirement[requirement]) != 1 {
			t.Fatalf("%s has cases %q, want exactly one", requirement, byRequirement[requirement])
		}
	}
	want := []string{"preflight: host login", "preflight: forge credential", "preflight: sandbox"}
	if !slices.Equal(byRequirement["REQ-6"], want) {
		t.Fatalf("REQ-6 cases = %q, want one per preflight check: %q", byRequirement["REQ-6"], want)
	}
}

// TestPreflightCasesPassOnlyWhenTheRunStopsNamingTheCheck proves each preflight case breaks its own check
// and fails a run that exits 0, names nothing, or starts a session.
func TestPreflightCasesPassOnlyWhenTheRunStopsNamingTheCheck(t *testing.T) {
	breaks := map[string]string{
		"host login": "HOME=", "forge credential": "GH_CONFIG_DIR=", "sandbox": "PATH=/without/sandbox-exec,bwrap",
	}
	behaviours := []struct {
		name    string
		code    int
		named   bool
		session bool
		wants   string
	}{
		{"stops naming the check", 1, true, false, ""},
		{"exits 0", 0, true, false, "exited 0"},
		{"names nothing", 1, false, false, "without naming"},
		{"starts a session first", 1, true, true, "session(s) started"},
	}
	for check, broken := range breaks {
		for _, behaviour := range behaviours {
			t.Run(check+"/"+behaviour.name, func(t *testing.T) {
				env := newFakeEnv(t)
				env.komodo = func(_ context.Context, extra []string, args ...string) Ran {
					called := strings.Join(append(append([]string{}, extra...), args...), " ")
					if !strings.Contains(called, broken) || !strings.Contains(called, "run "+caseGroup) {
						t.Errorf("komodo %s never breaks %s with %q", called, check, broken)
					}
					if behaviour.session {
						env.stamp(ledger.Entry{Group: caseGroup, Station: conductor.StationBuild, Role: "builder"})
					}
					output := "preflight failed:\n"
					if behaviour.named {
						output += strings.ToUpper(check[:1]) + check[1:] + ": the fix\n"
					}
					return Ran{Output: output, Code: behaviour.code}
				}
				verdict(t, caseNamed(t, "preflight: "+check), env, behaviour.wants)
			})
		}
	}
}

// TestEntriesAndSessionsFailTheCaseOnAnUnreadableLedger proves a broken ledger fails a case instead of
// reading as zero entries, which would let it pass unproven.
func TestEntriesAndSessionsFailTheCaseOnAnUnreadableLedger(t *testing.T) {
	env := newFakeEnv(t)
	if err := os.MkdirAll(filepath.Join(env.dir, line.StateDir, ledger.RunFile), 0o755); err != nil {
		t.Fatal(err)
	}
	env.komodo = func(_ context.Context, _ []string, _ ...string) Ran {
		return Ran{Output: "preflight failed:\nHost login: the fix\n", Code: 1}
	}
	verdict(t, caseNamed(t, "preflight: host login"), env, ledger.RunFile)
}

// TestKillAndResumeFailsALostEditOrARepeatedBuild proves the case kills mid-build and judges the resumed run.
func TestKillAndResumeFailsALostEditOrARepeatedBuild(t *testing.T) {
	fastWatch(t)
	cases := []struct {
		name   string
		resume func(env *fakeEnv, worktree string)
		early  bool
		wants  string
	}{
		{"keeps every edit and builds once", func(env *fakeEnv, _ string) {
			env.stamp(ledger.Entry{Group: caseGroup, Station: conductor.StationBuild, Role: "builder", Outcome: "done"})
			env.save(conductor.State{Current: conductor.Preparing})
		}, false, ""},
		{"loses an edit", func(env *fakeEnv, worktree string) {
			env.write(filepath.Join(worktree, "a.txt"), "base\n")
			env.save(conductor.State{Current: conductor.Preparing})
		}, false, "a.txt made before the kill was lost"},
		{"repeats a finished build", func(env *fakeEnv, _ string) {
			for range 2 {
				env.stamp(ledger.Entry{Group: caseGroup, Station: conductor.StationBuild, Role: "builder", Outcome: "done"})
			}
			env.save(conductor.State{Current: conductor.Preparing})
		}, false, "ran to done twice"},
		{"never gets past the build", func(*fakeEnv, string) {}, false, "stayed at Building"},
		{"loses its state", func(env *fakeEnv, _ string) {
			if err := os.Remove(conductor.StatePath(env.dir, caseGroup)); err != nil {
				t.Fatal(err)
			}
		}, false, "state.json"},
		{"ends before any edit", nil, true, "nothing was killed"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			worktree := filepath.Join(line.StateDir, "wt", caseGroup)
			env.write("a.txt", "base\n")
			calls := 0
			env.komodo = func(ctx context.Context, _ []string, args ...string) Ran {
				calls++
				if calls > 1 {
					each.resume(env, worktree)
					return Ran{}
				}
				if each.early {
					return Ran{Code: 1}
				}
				env.write(filepath.Join(worktree, "a.txt"), "edit\n")
				env.write(filepath.Join(worktree, line.StateDir, "run.json"), "{}\n")
				if err := os.Symlink("a.txt", filepath.Join(env.dir, worktree, "link.txt")); err != nil {
					t.Fatal(err)
				}
				env.save(conductor.State{
					Current: conductor.Building, Sessions: []string{"s1"},
					Worktree: filepath.Join(env.dir, worktree), Branch: "feat/greet",
				})
				<-ctx.Done()
				return Ran{Code: -1}
			}
			verdict(t, caseNamed(t, "kill and resume"), env, each.wants)
		})
	}
}

// TestCredentialRemovedWantsBlockedBeforeShipWithTheBranchKept proves the case removes the credential mid-run.
func TestCredentialRemovedWantsBlockedBeforeShipWithTheBranchKept(t *testing.T) {
	fastWatch(t)
	cases := []struct {
		name    string
		final   conductor.GroupState
		shipped bool
		branch  int
		wants   string
	}{
		{"stops blocked", conductor.Blocked, false, 0, ""},
		{"ships anyway", conductor.Shipped, true, 0, "ended Shipped"},
		{"stamps a ship", conductor.Blocked, true, 0, "stamped done"},
		{"loses the branch", conductor.Blocked, false, 1, "is gone"},
		{"loses its state", "", false, 0, "state.json"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.git = func(args ...string) Ran {
				if args[0] == "rev-parse" && args[len(args)-1] != "feat/greet" {
					t.Errorf("git %q checks the wrong branch", args)
				}
				return Ran{Code: each.branch}
			}
			env.komodo = func(ctx context.Context, extra []string, _ ...string) Ran {
				if !slices.Contains(extra, "GH_CONFIG_DIR=/credential") {
					t.Errorf("extra = %q, want the removable credential", extra)
				}
				env.save(conductor.State{Current: conductor.Reviewing, Branch: "feat/greet"})
				select {
				case <-env.removed:
				case <-ctx.Done():
					return Ran{Code: -1}
				}
				if each.shipped {
					env.stamp(ledger.Entry{Group: caseGroup, Station: "ship", Outcome: "done"})
				}
				if each.final == "" {
					if err := os.Remove(conductor.StatePath(env.dir, caseGroup)); err != nil {
						t.Error(err)
					}
					return Ran{Code: 1}
				}
				env.save(conductor.State{Current: each.final, Branch: "feat/greet"})
				return Ran{Code: 1}
			}
			verdict(t, caseNamed(t, "credential removed mid-run"), env, each.wants)
		})
	}
}

// TestRateLimitFailsASessionStartedWhilePaused proves the case reads the pause from events.jsonl and the ledger.
func TestRateLimitFailsASessionStartedWhilePaused(t *testing.T) {
	paused := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		output  string
		stamped bool
		session time.Duration
		wants   string
	}{
		{"pauses then starts after the reset", "drain paused: the usage window is 99% spent", true, 2 * time.Minute, ""},
		{"starts while paused", "drain paused", true, 30 * time.Second, "while the drain was paused"},
		{"never pauses", "drain done", true, 2 * time.Minute, "never paused"},
		{"stamps no pause", "drain paused", false, 2 * time.Minute, "stamped none"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.komodo = func(_ context.Context, extra []string, args ...string) Ran {
				if !slices.Contains(extra, "HOME=/overlay") || slices.Contains(args, caseGroup) {
					t.Errorf("komodo %q %q; the drain runs every group under the overlay", extra, args)
				}
				if each.stamped {
					err := line.Book(env.dir).WriteEvents([]ledger.Entry{
						{At: paused, Station: "pace", Outcome: "paused"},
						{At: paused.Add(time.Minute), Station: "pace", Outcome: "resumed"},
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				env.stamp(ledger.Entry{At: paused.Add(each.session), Seconds: 10, Group: caseGroup, Role: "builder"})
				return Ran{Output: each.output}
			}
			verdict(t, caseNamed(t, "simulated rate limit"), env, each.wants)
			if !strings.Contains(env.overlay, "pause_at") {
				t.Fatalf("overlay = %q, want a lowered pause mark", env.overlay)
			}
		})
	}
}

// TestCanaryFailsWhenThePlantedWordSurfaces proves the case plants a word, restores the file, and scans the
// repo's tree and its history.
func TestCanaryFailsWhenThePlantedWordSurfaces(t *testing.T) {
	cases := []struct {
		name    string
		file    bool
		output  bool
		session bool
		history bool
		wants   string
	}{
		{"the word appears nowhere", false, false, true, false, ""},
		{"a file carries the word", true, false, true, false, "the canary reached"},
		{"the output carries the word", false, true, true, false, "run's output"},
		{"no session ran", false, false, false, false, "proves nothing"},
		{"the word reaches only the commit history", false, false, true, true, "reached the repo's history"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.komodo = func(context.Context, []string, ...string) Ran {
				word := strings.Fields(strings.TrimPrefix(env.planted, "Write the word "))[0]
				if each.session {
					env.stamp(ledger.Entry{Group: caseGroup, Station: conductor.StationBuild, Role: "builder"})
				}
				body := "clean\n"
				if each.file {
					body = word + "\n"
				}
				env.write(filepath.Join(line.StateDir, "wt", caseGroup, "greet.go"), body)
				env.write(filepath.Join(".git", "COMMIT_EDITMSG"), "clean\n")
				if each.output {
					return Ran{Output: word}
				}
				return Ran{}
			}
			env.git = func(args ...string) Ran {
				if !each.history || !slices.Equal(args, []string{"log", "--all", "-p"}) {
					return Ran{}
				}
				word := strings.Fields(strings.TrimPrefix(env.planted, "Write the word "))[0]
				return Ran{Output: "commit abc\n    a leaked " + word + "\n"}
			}
			verdict(t, caseNamed(t, "canary"), env, each.wants)
			if !strings.Contains(env.planted, "KOMODO-CANARY-") || !env.restored {
				t.Fatalf("planted = %q, restored = %v; the canary is planted and then removed", env.planted, env.restored)
			}
		})
	}
}

// TestScanningCasesFailOnAFailedCleanupOrAnUnreadableTree proves the canary's restore and each tree scan fail
// their case.
func TestScanningCasesFailOnAFailedCleanupOrAnUnreadableTree(t *testing.T) {
	stamped := func(env *fakeEnv, locked bool) func(context.Context, []string, ...string) Ran {
		return func(context.Context, []string, ...string) Ran {
			env.stamp(ledger.Entry{Group: caseGroup, Station: conductor.StationBuild, Role: "builder"})
			env.write(filepath.Join(line.StateDir, "wt", "TG-99.1", envRecord), "## env\nPATH=/bin\n")
			if locked {
				dir := filepath.Join(env.dir, "locked")
				if err := os.Mkdir(dir, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			}
			return Ran{}
		}
	}
	unrestored := newFakeEnv(t)
	unrestored.restoreErr = errors.New("the instructions stayed planted")
	unrestored.komodo = stamped(unrestored, false)
	if err := caseNamed(t, "canary").Run(context.Background(), unrestored); !errors.Is(err, unrestored.restoreErr) {
		t.Fatalf("canary = %v, want the failed restore", err)
	}
	for _, name := range []string{"canary", "no forge token in a session"} {
		env := newFakeEnv(t)
		env.komodo = stamped(env, true)
		verdict(t, caseNamed(t, name), env, "permission denied")
	}
}

// TestNoForgeTokenFailsWhenASessionHoldsACredential proves the probe group and the record's checks.
func TestNoForgeTokenFailsWhenASessionHoldsACredential(t *testing.T) {
	cases := []struct {
		name   string
		record string
		wants  string
	}{
		{"a clean environment", "## env\nPATH=/bin\n## gh auth token\n## git credential\n", ""},
		{"the token in the environment", "## env\nPATH=/bin\nGH_TOKEN={token}\n## gh auth token\n",
			"the forge token reached"},
		{"gh hands over a token", "## env\nPATH=/bin\n## gh auth token\ngho_real\n## git credential\n",
			"reaching a forge credential"},
		{"git fills a password", "## env\nPATH=/bin\n## git credential\npassword=hunter2\n", "reaching a forge credential"},
		{"gh hands over a token last", "## env\nPATH=/bin\n## git credential\n## gh auth token\ngho_real\n",
			"reaching a forge credential"},
		{"no gh section", "## env\nPATH=/bin\n## git credential\n", ""},
		{"no environment at all", "## env\n", "holds no environment"},
		{"no record", "", "no session wrote"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.komodo = func(_ context.Context, extra []string, args ...string) Ran {
				if !slices.Equal(args, []string{"run", "TG-99.1", "--no-ship"}) {
					t.Errorf("args = %q, want the probe group run with no ship", args)
				}
				token := ""
				for _, entry := range extra {
					if value, ok := strings.CutPrefix(entry, "GH_TOKEN="); ok {
						token = value
					}
				}
				if !strings.HasPrefix(token, "ghp_") {
					t.Errorf("extra = %q, want a forge token handed to the run", extra)
				}
				env.write(filepath.Join(".git", envRecord), "## env\nGH_TOKEN="+token+"\n")
				if each.record != "" {
					record := strings.ReplaceAll(each.record, "{token}", token)
					env.write(filepath.Join(line.StateDir, "wt", "TG-99.1", envRecord), record)
				}
				return Ran{}
			}
			verdict(t, caseNamed(t, "no forge token in a session"), env, each.wants)
			if body := env.added["TG-99.1"]; !strings.Contains(body, "#### [TSK-99.1.1]") || !strings.Contains(body, envRecord) {
				t.Fatalf("probe group = %q", body)
			}
		})
	}
}

// TestNoForgeTokenAlsoScansTheRepoHistory proves the token check fails on a token that reaches a commit
// rather than a file in the worktree.
func TestNoForgeTokenAlsoScansTheRepoHistory(t *testing.T) {
	env := newFakeEnv(t)
	var token string
	env.komodo = func(_ context.Context, extra []string, _ ...string) Ran {
		for _, entry := range extra {
			if value, ok := strings.CutPrefix(entry, "GH_TOKEN="); ok {
				token = value
			}
		}
		env.write(filepath.Join(line.StateDir, "wt", "TG-99.1", envRecord), "## env\nPATH=/bin\n## gh auth token\n## git credential\n")
		return Ran{}
	}
	env.git = func(args ...string) Ran {
		if !slices.Equal(args, []string{"log", "--all", "-p"}) {
			return Ran{}
		}
		return Ran{Output: "commit abc\n    a leaked " + token + "\n"}
	}
	verdict(t, caseNamed(t, "no forge token in a session"), env, "reached the repo's history")
}

// TestParallelAndSerialReadsEachGroupsSpanFromTheLedger proves shared files serialise and disjoint ones overlap.
func TestParallelAndSerialReadsEachGroupsSpanFromTheLedger(t *testing.T) {
	type span struct{ from, to int }
	cases := []struct {
		name  string
		spans map[string]span
		wants string
	}{
		{"serial pair, overlapping third", map[string]span{"TG-99.2": {0, 10}, "TG-99.3": {10, 20}, "TG-99.4": {5, 15}}, ""},
		{"the pair overlaps", map[string]span{"TG-99.2": {0, 10}, "TG-99.3": {5, 15}, "TG-99.4": {5, 15}}, "ran at once"},
		{"the third overlaps neither", map[string]span{"TG-99.2": {0, 10}, "TG-99.3": {10, 20}, "TG-99.4": {30, 40}},
			"overlapped neither"},
		{"a group never ran", map[string]span{"TG-99.2": {0, 10}, "TG-99.3": {10, 20}}, "TG-99.4 never ran"},
	}
	started := time.Now().UTC().Add(-time.Hour)
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.komodo = func(_ context.Context, _ []string, args ...string) Ran {
				if !slices.Equal(args, []string{"run", "--no-ship"}) {
					t.Errorf("args = %q, want a drain", args)
				}
				for group, span := range each.spans {
					env.stamp(ledger.Entry{
						At: started.Add(time.Duration(span.to) * time.Minute), Seconds: float64(span.to-span.from) * 60,
						Group: group, Station: conductor.StationBuild, Role: "builder",
					})
				}
				return Ran{}
			}
			verdict(t, caseNamed(t, "parallel and serial groups"), env, each.wants)
			if !strings.Contains(env.added["TG-99.2"], "shared.txt") || !strings.Contains(env.added["TG-99.4"], "other.txt") {
				t.Fatalf("groups = %q, want two sharing shared.txt and one on other.txt", env.added)
			}
		})
	}
}

// TestPolicyEditCommitsOnABranchAndLeavesTheDefaultBranch proves the case edits, commits and returns to base.
func TestPolicyEditCommitsOnABranchAndLeavesTheDefaultBranch(t *testing.T) {
	original := "{\n  \"critical_refs\": [\"main\"]\n}\n"
	cases := []struct {
		name      string
		commit    int
		baseMoved bool
		back      int
		wants     string
	}{
		{"commits on the branch", 0, false, 0, ""},
		{"the commit is refused", 1, false, 0, "did not commit"},
		{"the default branch moves", 0, true, 0, "changed before any merge"},
		{"the checkout back fails", 0, false, 1, "left the repo on " + policyBranch},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.write(filepath.Join("komodo", "policy.json"), original)
			var calls []string
			env.git = func(args ...string) Ran {
				calls = append(calls, strings.Join(args, " "))
				switch {
				case args[0] == "branch":
					return Ran{Output: "main\n"}
				case slices.Equal(args, []string{"checkout", "-q", "main"}):
					return Ran{Code: each.back, Output: "local changes would be overwritten"}
				case args[0] == "commit":
					return Ran{Code: each.commit, Output: "gate refused"}
				case args[0] == "show" && args[1] == policyBranch+":komodo/policy.json",
					args[0] == "show" && each.baseMoved:
					data, err := os.ReadFile(filepath.Join(env.dir, "komodo", "policy.json"))
					if err != nil {
						t.Fatal(err)
					}
					return Ran{Output: string(data)}
				case args[0] == "show":
					return Ran{Output: original}
				}
				return Ran{}
			}
			verdict(t, caseNamed(t, "owner-directed policy edit"), env, each.wants)
			if calls[1] != "checkout -q -b "+policyBranch || calls[len(calls)-1] != "checkout -q main" {
				t.Fatalf("git calls = %q, want a branch cut first and main checked out last", calls)
			}
		})
	}
}

// TestPolicyEditStopsWhenTheRepoCannotTakeTheEdit proves each way the policy edit can fail is named.
func TestPolicyEditStopsWhenTheRepoCannotTakeTheEdit(t *testing.T) {
	original := "{\n  \"critical_refs\": [\"main\"]\n}\n"
	cases := []struct {
		name     string
		policy   string
		branch   string
		checkout int
		onBranch string
		wants    string
	}{
		{"on no branch", original, "", 0, "", "on no branch"},
		{"no policy file", "", "main", 0, "", "policy.json"},
		{"a broken policy file", "{", "main", 0, "", "komodo/policy.json:"},
		{"no branch cut", original, "main", 1, "", "git checkout -b"},
		{"the branch lacks the edit", original, "main", 0, original, "does not carry"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			if each.policy != "" {
				env.write(filepath.Join("komodo", "policy.json"), each.policy)
			}
			env.git = func(args ...string) Ran {
				switch {
				case args[0] == "branch":
					return Ran{Output: each.branch}
				case args[0] == "checkout" && args[2] == "-b":
					return Ran{Code: each.checkout}
				case args[0] == "show" && args[1] == policyBranch+":komodo/policy.json":
					return Ran{Output: each.onBranch}
				case args[0] == "show":
					return Ran{Output: original}
				}
				return Ran{}
			}
			verdict(t, caseNamed(t, "owner-directed policy edit"), env, each.wants)
		})
	}
}

// TestCasesStopOnAnEnvFailure proves a case whose setup the env cannot give stops with that failure.
func TestCasesStopOnAnEnvFailure(t *testing.T) {
	broken := errors.New("the env broke")
	// These cases ask the env for no setup, so there is no setup failure to stop on.
	skip := map[string]bool{"kill and resume": true, "owner-directed policy edit": true, "acts without asking": true}
	for _, each := range Cases() {
		if skip[each.Name] {
			continue
		}
		t.Run(each.Name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.fail = broken
			env.komodo = func(context.Context, []string, ...string) Ran {
				t.Error("komodo ran after the env failed")
				return Ran{}
			}
			if err := each.Run(context.Background(), env); !errors.Is(err, broken) {
				t.Fatalf("%s = %v, want the env's failure", each.Name, err)
			}
		})
	}
}

// TestCredentialRemovedStopsWhenTheCredentialStaysOrTheRunEndsFirst proves both ways the removal never happens.
func TestCredentialRemovedStopsWhenTheCredentialStaysOrTheRunEndsFirst(t *testing.T) {
	fastWatch(t)
	broken := errors.New("still there")
	env := newFakeEnv(t)
	env.removeErr = broken
	env.komodo = func(ctx context.Context, _ []string, _ ...string) Ran {
		env.save(conductor.State{Current: conductor.Reviewing})
		<-ctx.Done()
		return Ran{Code: -1}
	}
	if err := caseNamed(t, "credential removed mid-run").Run(context.Background(), env); !errors.Is(err, broken) {
		t.Fatalf("credential removed mid-run = %v, want the removal's failure", err)
	}
	early := newFakeEnv(t)
	early.komodo = func(context.Context, []string, ...string) Ran { return Ran{Code: 1} }
	verdict(t, caseNamed(t, "credential removed mid-run"), early, "never removed")
	select {
	case <-early.removed:
	default:
		t.Fatal("a run that ended before its build left the forge token on disk")
	}
}

// TestKillAndResumeReadsTheBranchWhenTheWorktreeIsGone proves an edit counts as kept when the branch carries it.
func TestKillAndResumeReadsTheBranchWhenTheWorktreeIsGone(t *testing.T) {
	fastWatch(t)
	for branchBody, wants := range map[string]string{"edit\n": "", "base\n": "was lost"} {
		t.Run(strings.TrimSpace(branchBody), func(t *testing.T) {
			env := newFakeEnv(t)
			worktree := filepath.Join(line.StateDir, "wt", caseGroup)
			env.write("a.txt", "base\n")
			env.git = func(args ...string) Ran {
				if slices.Equal(args, []string{"show", "feat/greet:a.txt"}) {
					return Ran{Output: branchBody}
				}
				return Ran{Code: 1}
			}
			calls := 0
			env.komodo = func(ctx context.Context, _ []string, _ ...string) Ran {
				calls++
				if calls > 1 {
					if err := os.RemoveAll(filepath.Join(env.dir, worktree)); err != nil {
						t.Fatal(err)
					}
					env.save(conductor.State{Current: conductor.Preparing, Branch: "feat/greet"})
					return Ran{}
				}
				env.write(filepath.Join(worktree, "a.txt"), "edit\n")
				env.save(conductor.State{
					Current: conductor.Building, Sessions: []string{"s1"}, Worktree: filepath.Join(env.dir, worktree),
				})
				<-ctx.Done()
				return Ran{Code: -1}
			}
			verdict(t, caseNamed(t, "kill and resume"), env, wants)
		})
	}
}

// TestRunCasesGivesEachCaseItsOwnEnvAndRecordsItsVerdict proves every case runs, passing or failing, in a fresh env.
func TestRunCasesGivesEachCaseItsOwnEnvAndRecordsItsVerdict(t *testing.T) {
	var seen []Env
	cases := []Case{
		{Name: "proves", Requirement: "REQ-1", Run: func(_ context.Context, env Env) error {
			seen = append(seen, env)
			return nil
		}},
		{Name: "disproves", Requirement: "REQ-2", Run: func(_ context.Context, env Env) error {
			seen = append(seen, env)
			return errors.New("the run shipped")
		}},
	}
	var out strings.Builder
	outcomes, err := RunCases(context.Background(), CaseOptions{
		Cases: cases, Stdout: &out,
		Env: func(context.Context, int) (Env, error) { return newFakeEnv(t), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 || !outcomes[0].Passed() || outcomes[1].Passed() || outcomes[1].Error != "the run shipped" {
		t.Fatalf("outcomes = %+v, want the first passed and the second failed", outcomes)
	}
	if len(seen) != 2 || seen[0] == seen[1] {
		t.Fatal("the cases shared one env; each gets a fresh one")
	}
	for _, want := range []string{"case proves (REQ-1): passed", "case disproves (REQ-2): failed: the run shipped"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output = %q, want %q", out.String(), want)
		}
	}
}

// TestRunCasesStopsWhenAnEnvCannotBeBuilt proves a broken env ends the run instead of scoring a case.
func TestRunCasesStopsWhenAnEnvCannotBeBuilt(t *testing.T) {
	broken := errors.New("no route")
	outcomes, err := RunCases(context.Background(), CaseOptions{
		Cases: Cases(),
		Env:   func(context.Context, int) (Env, error) { return nil, broken },
	})
	if !errors.Is(err, broken) || len(outcomes) != 0 {
		t.Fatalf("RunCases = %v, %v, want the env's failure and no verdict", outcomes, err)
	}
	if _, err := RunCases(context.Background(), CaseOptions{Cases: Cases()}); err == nil {
		t.Fatal("RunCases with no env = nil, want a refusal")
	}
}

// TestRunCasesBudgetsEachCaseWithADeadline proves a zero Budget falls back to run.GroupBudget, and a tiny
// Budget ends a case's ctx before it returns on its own.
func TestRunCasesBudgetsEachCaseWithADeadline(t *testing.T) {
	var deadline time.Time
	var hasDeadline bool
	defaulted := Case{Name: "default budget", Requirement: "REQ-1", Run: func(ctx context.Context, _ Env) error {
		deadline, hasDeadline = ctx.Deadline()
		return nil
	}}
	if _, err := RunCases(context.Background(), CaseOptions{
		Cases: []Case{defaulted}, Env: func(context.Context, int) (Env, error) { return newFakeEnv(t), nil },
	}); err != nil {
		t.Fatal(err)
	}
	if left := time.Until(deadline); !hasDeadline || left <= 0 || left > run.GroupBudget {
		t.Fatalf("deadline in %s, want one within a zero Budget's fallback of %s", left, run.GroupBudget)
	}

	outran := Case{Name: "tiny budget", Requirement: "REQ-2", Run: func(ctx context.Context, _ Env) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	outcomes, err := RunCases(context.Background(), CaseOptions{
		Cases: []Case{outran}, Budget: 10 * time.Millisecond,
		Env: func(context.Context, int) (Env, error) { return newFakeEnv(t), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].Passed() || !strings.Contains(outcomes[0].Error, "deadline exceeded") {
		t.Fatalf("outcomes = %+v, want the one case ended by its tiny Budget", outcomes)
	}
}

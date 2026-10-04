package line

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/git"
	"komodo/internal/pr"
)

const shipBacklog = "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-09.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - true\n```\n"

// commitBacklogText commits legacy-grammar text into root as its group's own docs/backlog file,
// the shape backlogtest.SeedText writes to disk, in one commit under message.
func commitBacklogText(t *testing.T, root, text, message string) {
	t.Helper()
	staged := t.TempDir()
	backlogtest.SeedText(t, staged, text)
	entries, err := os.ReadDir(filepath.Join(staged, "docs", "backlog"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(staged, "docs", "backlog", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		commit(t, root, filepath.Join("docs", "backlog", entry.Name()), string(data), message)
	}
}

// shipRepo builds a group worktree and a main checkout, both remoted at a bare origin, so
// ShipGroup can read the push URL from the root and push from the group.
func shipRepo(t *testing.T) (root, group string) {
	t.Helper()
	root = t.TempDir()
	backlogtest.SeedText(t, root, shipBacklog)
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", bare)
	group = filepath.Join(root, "group")
	if err := os.MkdirAll(group, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "init")
	runGit(t, group, "config", "user.email", "a@example.com")
	runGit(t, group, "config", "user.name", "a")
	runGit(t, group, "remote", "add", "origin", bare)
	runGit(t, group, "checkout", "-b", "feat/a-group")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	backlogtest.SeedText(t, group, shipBacklog)
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "seed")
	saveReview(t, root, "TG-09.1", `{"findings":[]}`)
	if err := SaveRun(root, RunState{Run: "TG-09.1-1", Group: "TG-09.1", Base: "main", Branch: "feat/a-group", Worktree: "group"}); err != nil {
		t.Fatal(err)
	}
	return root, group
}

// saveReview saves a review result for the group, as the reviewer would after the last commit.
func saveReview(t *testing.T, root, groupID, result string) {
	t.Helper()
	review := ResultPath(root, groupID+"-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(result), 0o644); err != nil {
		t.Fatal(err)
	}
	// Commit stamps are whole seconds, so a test's later commit must never outdate its review by the clock alone.
	ahead := time.Now().Add(time.Minute)
	if err := os.Chtimes(review, ahead, ahead); err != nil {
		t.Fatal(err)
	}
}

// runGit runs one git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// commitDated writes a file and commits it with an explicit author and committer date.
func commitDated(t *testing.T, root, name, body, message string, when time.Time) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	stamp := when.Format(time.RFC3339)
	cmd := exec.Command("git", "commit", "-m", message, "--date", stamp)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+stamp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}

func TestShipGroupRunsTheAfterPublishCommand(t *testing.T) {
	root, group := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(root, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"touch published.txt"}`
	if err := os.WriteFile(filepath.Join(root, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published == nil || !result.Published.OK() {
		t.Fatalf("published = %+v", result.Published)
	}
	if _, err := os.Stat(filepath.Join(group, "published.txt")); err != nil {
		t.Fatalf("after_publish did not run: %v", err)
	}
}

// installPrePush writes a pre-push hook into group that records its environment and stdin, then exits with code.
func installPrePush(t *testing.T, group string, code int) (envLog string) {
	t.Helper()
	envLog = filepath.Join(t.TempDir(), "hook.env")
	hook := fmt.Sprintf("#!/bin/sh\nenv > %q\ncat >> %q\nexit %d\n", envLog, envLog, code)
	path := filepath.Join(group, ".git", "hooks", "pre-push")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	return envLog
}

func TestPushRunsThePrePushHookWithoutTheForgeCredential(t *testing.T) {
	root, group := shipRepo(t)
	envLog := installPrePush(t, group, 0)
	t.Setenv("GH_TOKEN", "secret-token")
	t.Setenv("GITHUB_TOKEN", "secret-token")
	if err := PushFromWorktree(root, group, "feat/a-group"); err != nil {
		t.Fatal(err)
	}
	seen, err := os.ReadFile(envLog)
	if err != nil {
		t.Fatalf("the pre-push hook never ran: %v", err)
	}
	if strings.Contains(string(seen), "secret-token") {
		t.Fatalf("the pre-push hook saw the forge credential:\n%s", seen)
	}
	if !strings.Contains(string(seen), "refs/heads/feat/a-group ") {
		t.Fatalf("the pre-push hook did not read the pushed ref on stdin:\n%s", seen)
	}
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(bare, "rev-parse", "--verify", "refs/heads/feat/a-group"); err != nil {
		t.Fatal("the branch did not land after the hook passed")
	}
}

func TestThePrePushHookNeverSeesTheCredentialInThePushURL(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, root, "remote", "set-url", "--push", "origin", "https://x-access-token:secret-token@example.invalid/o/r.git")
	seen := filepath.Join(t.TempDir(), "hook.log")
	hook := fmt.Sprintf("#!/bin/sh\necho \"$@\" > %q\nenv >> %q\nexit 1\n", seen, seen)
	path := filepath.Join(group, ".git", "hooks", "pre-push")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	err := PushFromWorktree(root, group, "feat/a-group")
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("err = %v; want the hook's refusal, with no token in it", err)
	}
	log, readErr := os.ReadFile(seen)
	if readErr != nil {
		t.Fatalf("the pre-push hook never ran: %v", readErr)
	}
	if strings.Contains(string(log), "secret-token") {
		t.Fatalf("the pre-push hook saw the token:\n%s", log)
	}
	if args, _, _ := strings.Cut(string(log), "\n"); args != "origin https://example.invalid/o/r.git" {
		t.Fatalf("the pre-push hook read %q, want origin and the URL without its credential", args)
	}
}

func TestAFailedCredentialedPushNamesNoToken(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, root, "remote", "set-url", "--push", "origin", "https://x-access-token:secret-token@127.0.0.1:1/o/r.git")
	err := PushFromWorktree(root, group, "feat/a-group")
	if err == nil || !strings.Contains(err.Error(), "git push to origin") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("err = %v; want the push's failure, with no token in it", err)
	}
}

func TestThePushCredentialHelperAnswersFromThePushEnvironment(t *testing.T) {
	cmd := exec.Command("git", "-c", "credential.helper=", "-c", "credential.helper="+pushCredentialHelper, "credential", "fill")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=example.invalid\n\n")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", pushUsernameEnv+"=x-access-token", pushPasswordEnv+"=tok")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git credential fill: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "username=x-access-token\n") || !strings.Contains(string(out), "password=tok\n") {
		t.Fatalf("credential = %q, want the push environment's user and secret", out)
	}
}

// fillCredential runs the push's own -c registration for clean against a credential request for host,
// with no askpass or system helper able to answer in its place.
func fillCredential(t *testing.T, clean, host string) string {
	t.Helper()
	cmd := exec.Command("git", "-c", "credential.helper=", "-c", credentialHelperKey(clean)+"="+pushCredentialHelper, "credential", "fill")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=" + host + "\n\n")
	cmd.Env = append(hookEnv(os.Environ()), pushUsernameEnv+"=x-access-token", pushPasswordEnv+"=tok")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// TestThePushCredentialHelperNeverAnswersForAnotherHost proves the token the push's -c flags register
// cannot reach a host other than the push URL's own, such as a redirect or a proxy's 407.
func TestThePushCredentialHelperNeverAnswersForAnotherHost(t *testing.T) {
	clean := "https://example.invalid/o/r.git"
	if out := fillCredential(t, clean, "example.invalid"); !strings.Contains(out, "username=x-access-token\n") ||
		!strings.Contains(out, "password=tok\n") {
		t.Fatalf("fill(the push's own host) = %q, want its credential", out)
	}
	if out := fillCredential(t, clean, "attacker.invalid"); strings.Contains(out, "x-access-token") || strings.Contains(out, "tok") {
		t.Fatalf("fill(another host) = %q; the credential must never reach a host that is not the push's own", out)
	}
}

func TestCredentialHelperKeyScopesToTheURLsProtocolAndHost(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"https://github.com/o/r.git", "credential.https://github.com.helper"},
		{"http://example.com:8080/o/r.git", "credential.http://example.com:8080.helper"},
		{"/tmp/origin.git", "credential.helper"},
		{"git@github.com:o/r.git", "credential.helper"},
	}
	for _, tc := range cases {
		if got := credentialHelperKey(tc.raw); got != tc.want {
			t.Errorf("credentialHelperKey(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestSplitCredentialKeepsTheSecretOutOfThePushURL(t *testing.T) {
	cases := []struct {
		name, raw, clean, username, password string
	}{
		{"a token in the URL", "https://x-access-token:tok@github.com/o/r.git",
			"https://github.com/o/r.git", "x-access-token", "tok"},
		{"no credential", "https://github.com/o/r.git", "https://github.com/o/r.git", "", ""},
		{"a local path", "/tmp/origin.git", "/tmp/origin.git", "", ""},
		{"an ssh remote", "git@github.com:o/r.git", "git@github.com:o/r.git", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clean, username, password := splitCredential(tc.raw)
			if clean != tc.clean || username != tc.username || password != tc.password {
				t.Fatalf("split = %q %q %q, want %q %q %q", clean, username, password, tc.clean, tc.username, tc.password)
			}
		})
	}
}

// TestRunPrePushTimesOutAndKillsAHungHook proves a pre-push hook past pushTimeout is killed,
// process group included, so a hung hook never blocks the line.
func TestRunPrePushTimesOutAndKillsAHungHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	root, group := shipRepo(t)
	hook := "#!/bin/sh\nsleep 30 &\nsleep 30\n"
	path := filepath.Join(group, ".git", "hooks", "pre-push")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := pushTimeout
	pushTimeout = 200 * time.Millisecond
	t.Cleanup(func() { pushTimeout = saved })
	started := time.Now()
	if err := PushFromWorktree(root, group, "feat/a-group"); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}

// TestPushRefTimesOutAndKillsAHungPush proves a push past pushTimeout is killed, process group
// included, so a hung push never blocks the line.
func TestPushRefTimesOutAndKillsAHungPush(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	root, group := shipRepo(t)
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	fakes := t.TempDir()
	wrapper := "#!/bin/sh\nif [ \"$1\" = push ]; then sleep 30 & sleep 30; fi\nexec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(fakes, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakes+string(os.PathListSeparator)+os.Getenv("PATH"))
	saved := pushTimeout
	pushTimeout = 200 * time.Millisecond
	t.Cleanup(func() { pushTimeout = saved })
	started := time.Now()
	if err := PushFromWorktree(root, group, "feat/a-group"); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}

func TestAFailingPrePushHookStopsThePush(t *testing.T) {
	root, group := shipRepo(t)
	installPrePush(t, group, 1)
	if err := PushFromWorktree(root, group, "feat/a-group"); err == nil || !strings.Contains(err.Error(), "pre-push") {
		t.Fatalf("err = %v; a refusing pre-push hook must stop the push", err)
	}
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(bare, "rev-parse", "--verify", "refs/heads/feat/a-group"); err == nil {
		t.Fatal("the branch landed although its pre-push hook refused")
	}
}

func TestShipGroupPushesThroughTheRootsExplicitURL(t *testing.T) {
	root, group := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", bare, "branch", "--list", "feat/a-group").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "feat/a-group") {
		t.Fatalf("branch = %s; ShipGroup must push the branch to the URL the root names", out)
	}
	if _, err := git.Run(group, "config", "--get", "remote.origin.pushurl"); err == nil {
		t.Fatal("the group worktree's own config must never gain a pushurl from a ship push")
	}
}

func TestShipGroupWithNoAfterPublishLeavesPublishedUnset(t *testing.T) {
	root, _ := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published != nil {
		t.Fatalf("published = %+v, want nil", result.Published)
	}
}

func TestAfterPublishFailureFailsTheShip(t *testing.T) {
	root, _ := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(root, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"exit 3"}`
	if err := os.WriteFile(filepath.Join(root, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, nil)
	if err == nil {
		t.Fatal("a failed after_publish command must fail the ship, not exit clean")
	}
	if result == nil || result.Published == nil || result.Published.ExitCode != 3 {
		t.Fatalf("published = %+v", result.Published)
	}
}

func TestShipRefusesAPlanWhosePauseBlankedItsWaves(t *testing.T) {
	root, _ := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks:     []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
		WaitUntil: time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "paused") {
		t.Fatalf("err = %v; a plan whose waves a pause blanked must not ship", err)
	}
}

func TestShipWritesAHandoffInsteadOfPushingWhenScrubbed(t *testing.T) {
	root, group := shipRepo(t)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_CONFIG_KEY_0", "credential.helper")
	t.Setenv("GIT_CONFIG_VALUE_0", "")
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run while the environment is scrubbed: %v", args)
		return "", nil
	}}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "" {
		t.Fatalf("url = %q; a scrubbed ship never reaches the pull request client", result.URL)
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = group
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("upstream = %s; a scrubbed ship must not push", out)
	}
	data, err := os.ReadFile(filepath.Join(root, StateDir, "runs", "TG-09.1", "ship.json"))
	if err != nil {
		t.Fatal(err)
	}
	var handoff ShipHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		t.Fatal(err)
	}
	if handoff.Branch != "feat/a-group" || handoff.Base != "main" || handoff.Title == "" {
		t.Fatalf("handoff = %+v", handoff)
	}
}

const flipBacklog = "### [TG-11.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-11.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

func TestShipFlipsTheStatusOnTheBranchItPushes(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, flipBacklog, "the backlog")
	root := t.TempDir()
	closed := strings.Replace(flipBacklog, "[READY]", "[DONE]", 1)
	backlogtest.SeedText(t, root, closed)
	rootGroupFile := filepath.Join(root, "docs", "backlog", "TG-11.1-a-group.md")
	wantStale, err := os.ReadFile(rootGroupFile)
	if err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	unreachableOrigin(t, root)
	saveReview(t, root, "TG-11.1", `{"findings":[]}`)
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "git push to origin feat/a-group") {
		t.Fatalf("err = %v; the origin is unreachable, so ship must fail at the push itself and not before", err)
	}
	shipped, err := os.ReadFile(filepath.Join(worktree, "docs", "backlog", "TG-11.1-a-group.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shipped), "- [x] **TSK-11.1.1**") {
		t.Fatalf("the branch being pushed still says READY:\n%s", shipped)
	}
	stale, err := os.ReadFile(rootGroupFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(stale) != string(wantStale) {
		t.Fatal("ship must write the worktree it commits, not the root it was invoked from")
	}
}

func TestShipNeverMarksATaskItSkippedAsDone(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, flipBacklog, "the backlog")
	root := t.TempDir()
	backlogtest.SeedText(t, root, flipBacklog)
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	_, _ = ShipGroup(root, plan, nil, nil)
	shipped, err := os.ReadFile(filepath.Join(worktree, "docs", "backlog", "TG-11.1-a-group.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(shipped), "- [x]") {
		t.Fatalf("a task close never marked DONE shipped as DONE:\n%s", shipped)
	}
}

func TestShipRefusesAGroupWithNoReviewResult(t *testing.T) {
	root, _ := shipRepo(t)
	if err := os.Remove(ResultPath(root, "TG-09.1-review")); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "no review result") {
		t.Fatalf("err = %v; a group with no review result must not ship", err)
	}
}

func TestShipRefusesAReviewOlderThanTheBranch(t *testing.T) {
	root, group := shipRepo(t)
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(ResultPath(root, "TG-09.1-review"), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "two.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "a repair after the review")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "changed after its review") {
		t.Fatalf("err = %v; a branch that moved after its review must not ship", err)
	}
}

func TestFileFindingsBeforePushAndCommit(t *testing.T) {
	root, group := shipRepo(t)
	review := ResultPath(root, "TG-09.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	findings := `{"findings":[{"class":"simplify","severity":"low","file":"a/one.go","line":1,"title":"t","detail":"d","fix":"f"}]}`
	if err := os.WriteFile(review, []byte(findings), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	plan.Profile.SeverityFloor = "high"
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Filed) != 1 {
		t.Fatalf("filed = %v, want exactly one finding filed", result.Filed)
	}
	status, err := git.Run(group, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("status = %q; findings filed before push must be in the commit, leaving worktree clean", status)
	}
}

func TestAFailedCommitAfterStagingLeavesNothingStaged(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, flipBacklog, "the backlog")
	root := t.TempDir()
	backlogtest.SeedText(t, root, flipBacklog)
	saveReview(t, root, "TG-11.1", `{"findings":[]}`)
	hook := filepath.Join(worktree, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'the gate refuses' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "one.go"), []byte("package one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY", Files: []string{"one.go"}}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "the gate refuses") {
		t.Fatalf("err = %v, want the pre-commit hook's refusal", err)
	}
	staged, err := git.Run(worktree, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	if staged != "" {
		t.Fatalf("staged = %q; a failed ship must leave nothing staged", staged)
	}
}

func TestShipRetriedAroundAFailedPushDoesNotFileAFindingTwice(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, flipBacklog, "the backlog")
	root := t.TempDir()
	backlogtest.SeedText(t, root, flipBacklog)
	review := ResultPath(root, "TG-11.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	findings := `{"findings":[{"class":"simplify","severity":"low","file":"a/one.go","line":1,"title":"t","detail":"d","fix":"f"}]}`
	if err := os.WriteFile(review, []byte(findings), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	plan.Profile.SeverityFloor = "high"
	unreachableOrigin(t, root)
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "git push to origin feat/a-group") {
		t.Fatalf("err = %v; the origin is unreachable, so the first ship must fail at the push", err)
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "git push to origin feat/a-group") {
		t.Fatalf("err = %v; a retried ship must fail the same way", err)
	}
	shipped, err := os.ReadFile(filepath.Join(worktree, "docs", "backlog", "TG-11.1-a-group.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(shipped), "a/one.go:1 t") != 1 {
		t.Fatalf("a retry must not file the same finding a second time:\n%s", shipped)
	}
}

func TestAFailedGateRecordsShipAsFailedNotDone(t *testing.T) {
	root, _ := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module komodo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	state := RunState{Run: "TG-09.1-1", Group: "TG-09.1", Base: "main", Branch: "feat/a-group"}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "gate") {
		t.Fatalf("err = %v; the group worktree has no cmd/komodo, so the gate must fail", err)
	}
	entries, err := Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Group == "TG-09.1" && entry.Outcome == "done" {
			t.Fatal("a failed gate must not stamp the ship entry as done")
		}
	}
	if shipped(root, plan, backlog.Backlog{}) {
		t.Fatal("a failed gate must not release the line to the next group")
	}
}

func TestShipCommitsTheStatusItWrote(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, flipBacklog, "the backlog")
	root := t.TempDir()
	backlogtest.SeedText(t, root, flipBacklog)
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	_, _ = ShipGroup(root, plan, nil, nil)
	status, err := git.Run(worktree, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("status = %q; the status flip and the changelog must land in the commit, not after it", status)
	}
}

func TestShipWritesTheRunsStatusIntoItsCommitAndClearsIt(t *testing.T) {
	for _, status := range []string{"DONE", "BLOCKED"} {
		worktree := gitRepo(t)
		commitBacklogText(t, worktree, flipBacklog, "the backlog")
		root := t.TempDir()
		backlogtest.SeedText(t, root, flipBacklog)
		rootGroupFile := filepath.Join(root, "docs", "backlog", "TG-11.1-a-group.md")
		wantRoot, err := os.ReadFile(rootGroupFile)
		if err != nil {
			t.Fatal(err)
		}
		if err := RecordStatus(root, "TSK-11.1.1", status); err != nil {
			t.Fatal(err)
		}
		plan := &Plan{
			Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
			Base: "main", Branch: "feat/a-group", Worktree: worktree,
			Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
		}
		unreachableOrigin(t, root)
		saveReview(t, root, "TG-11.1", `{"findings":[]}`)
		_, _ = ShipGroup(root, plan, nil, nil)
		committed, err := git.Run(worktree, "show", "HEAD:docs/backlog/TG-11.1-a-group.md")
		if err != nil {
			t.Fatal(err)
		}
		want := "- [x] **TSK-11.1.1**"
		if status == "BLOCKED" {
			want = "status: BLOCKED"
		}
		if !strings.Contains(committed, want) {
			t.Fatalf("the ship commit does not carry %s:\n%s", status, committed)
		}
		if len(LoadStatus(root)) != 0 {
			t.Fatalf("%s: status.json survived the ship commit", status)
		}
		if data, _ := os.ReadFile(rootGroupFile); string(data) != string(wantRoot) {
			t.Fatalf("%s: ship rewrote the root's group file", status)
		}
	}
}

func TestShipKeepsAPersonsEditToATaskBodyAndChangesOnlyTheTick(t *testing.T) {
	edited := "## [TG-11.1] A group [P: C] [READY]\n\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"- [ ] **TSK-11.1.1** One\n  - files: `a/one.go`\n  - done_when: `go test ./a/...`\n" +
		"\nA person's note under the task, kept byte for byte.\n\n" +
		"- [ ] **TSK-11.1.2** Two\n  - files: `b/two.go`\n  - done_when: `go test ./b/...`\n" +
		"  - context: `a person's added note`\n"
	worktree := gitRepo(t)
	commit(t, worktree, filepath.Join("docs", "backlog", "TG-11.1-a-group.md"), edited, "a person edits the plan")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	rootGroupFile := filepath.Join(root, "docs", "backlog", "TG-11.1-a-group.md")
	if err := os.WriteFile(rootGroupFile, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RecordStatus(root, "TSK-11.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	if err := RecordStatus(root, "TSK-11.1.2", "IN_PROGRESS"); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}, {ID: "TSK-11.1.2", Title: "Two", Status: "READY"}},
	}
	unreachableOrigin(t, root)
	saveReview(t, root, "TG-11.1", `{"findings":[]}`)
	_, _ = ShipGroup(root, plan, nil, nil)
	committed, err := git.Run(worktree, "show", "HEAD:docs/backlog/TG-11.1-a-group.md")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(edited, "- [ ] **TSK-11.1.1**", "- [x] **TSK-11.1.1**", 1)
	if strings.TrimSpace(committed) != strings.TrimSpace(want) {
		t.Fatalf("the ship commit changed more than the tick:\n%s\nwant\n%s", committed, want)
	}
	if data, _ := os.ReadFile(rootGroupFile); string(data) != edited {
		t.Fatal("ship rewrote the root's group file")
	}
}

func TestShipLeavesAnotherGroupsLiveStatusAlone(t *testing.T) {
	other := "### [TG-11.2] Another group\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
		"#### [TSK-11.2.1] Other [P: C] [READY]\n```yaml\nfiles: [b/other.go]\ndone_when: [\"go test ./b/...\"]\n```\n"
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, flipBacklog, "the backlog")
	commitBacklogText(t, worktree, other, "another group")
	root := t.TempDir()
	backlogtest.SeedText(t, root, flipBacklog+other)
	for _, taskID := range []string{"TSK-11.1.1", "TSK-11.2.1"} {
		if err := RecordStatus(root, taskID, "DONE"); err != nil {
			t.Fatal(err)
		}
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	unreachableOrigin(t, root)
	saveReview(t, root, "TG-11.1", `{"findings":[]}`)
	_, _ = ShipGroup(root, plan, nil, nil)
	shipped, err := git.Run(worktree, "show", "HEAD:docs/backlog/TG-11.1-a-group.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shipped, "- [x] **TSK-11.1.1**") {
		t.Fatalf("the ship commit does not carry its own task's DONE:\n%s", shipped)
	}
	untouched, err := git.Run(worktree, "show", "HEAD:docs/backlog/TG-11.2-another-group.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(untouched, "- [ ] **TSK-11.2.1**") {
		t.Fatalf("the ship commit wrote another group's status:\n%s", untouched)
	}
	if got := LoadStatus(root); len(got) != 1 || got["TSK-11.2.1"].Status != "DONE" {
		t.Fatalf("status = %+v; ship must clear only its own group's tasks", got)
	}
}

const closedRootBacklog = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [BLOCKED]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

const staleWorktreeBacklog = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

func TestShipReadsStatusFromTheRootCloseWrites(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, staleWorktreeBacklog, "the backlog")
	root := t.TempDir()
	backlogtest.SeedText(t, root, closedRootBacklog)
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, worktree, "checkout", "-q", "-b", "feat/a-group")
	plan := &Plan{
		Group: "TG-12.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-12.1.1", Title: "One", Status: "READY"}},
	}
	saveReview(t, root, "TG-12.1", `{"findings":[]}`)
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocked) != 1 || result.Blocked[0] != "TSK-12.1.1" {
		t.Fatalf("blocked = %v; ship must read status from the root close writes, not the stale worktree copy", result.Blocked)
	}
	if !result.Draft {
		t.Fatal("a blocked task must ship as a draft")
	}
}

func TestShipRendersTheBodyAfterBlockedIsKnown(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, staleWorktreeBacklog, "the backlog")
	root := t.TempDir()
	backlogtest.SeedText(t, root, closedRootBacklog)
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, worktree, "checkout", "-q", "-b", "feat/a-group")
	plan := &Plan{
		Group: "TG-12.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-12.1.1", Title: "One", Status: "READY"}},
	}
	var calls []string
	client := &pr.Client{Dir: worktree, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "https://example.com/pull/1", nil
	}}
	saveReview(t, root, "TG-12.1", `{"findings":[]}`)
	if _, err := ShipGroup(root, plan, nil, client); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || !strings.Contains(calls[0], "- **Unproven** TSK-12.1.1 One is blocked") {
		t.Fatalf("calls = %v; the body must render after Blocked is known, so a blocked task ships unticked", calls)
	}
}

func TestShipsOwnCommitDoesNotRestaleTheReview(t *testing.T) {
	worktree := gitRepo(t)
	now := time.Now()
	commitDated(t, worktree, "a/one.go", "package a\n", "seed", now.Add(-2*time.Hour))
	root := t.TempDir()
	plan := &Plan{Group: "TG-13.1", Title: "A group", Type: "feat", Worktree: worktree}
	review := ResultPath(root, "TG-13.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewTime := now.Add(-time.Hour)
	if err := os.Chtimes(review, reviewTime, reviewTime); err != nil {
		t.Fatal(err)
	}
	message := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
	commitDated(t, worktree, "CHANGELOG.md", "# Changelog\n", message, now)
	if !reviewed(root, plan) {
		t.Fatal("ship's own commit must not restale a review that already passed it")
	}
}

func TestTheCredentialNoteCommitDoesNotRestaleTheReview(t *testing.T) {
	worktree := gitRepo(t)
	now := time.Now()
	commitDated(t, worktree, "a/one.go", "package a\n", "seed", now.Add(-2*time.Hour))
	root := t.TempDir()
	plan := &Plan{Group: "TG-13.1", Title: "A group", Type: "feat", Worktree: worktree}
	review := ResultPath(root, "TG-13.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewTime := now.Add(-time.Hour)
	if err := os.Chtimes(review, reviewTime, reviewTime); err != nil {
		t.Fatal(err)
	}
	commitDated(t, worktree, "CHANGELOG.md", "# Changelog\n", credentialNoteSubject(plan.Title, plan.Group), now)
	if !reviewed(root, plan) {
		t.Fatal("a retry's own credential-note commit must not restale a review that already passed it")
	}
}

func TestABaseThatMovesDoesNotRestaleTheReview(t *testing.T) {
	worktree := gitRepo(t)
	now := time.Now()
	commitDated(t, worktree, "a/one.go", "package a\n", "seed", now.Add(-3*time.Hour))
	runGit(t, worktree, "branch", "epic")
	commitDated(t, worktree, "a/two.go", "package a\n", "the group's work", now.Add(-2*time.Hour))
	root := t.TempDir()
	plan := &Plan{Group: "TG-13.1", Title: "A group", Type: "feat", Base: "epic", Worktree: worktree}
	saveReview(t, root, plan.Group, `{"findings":[]}`)
	reviewTime := now.Add(-time.Hour)
	if err := os.Chtimes(ResultPath(root, "TG-13.1-review"), reviewTime, reviewTime); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "checkout", "-q", "epic")
	commitDated(t, worktree, "b/three.go", "package b\n", "the epic moved", now)
	runGit(t, worktree, "checkout", "-q", "-")
	runGit(t, worktree, "rebase", "-q", "epic")
	if !reviewed(root, plan) {
		t.Fatal("a base that moved, and the catch-up rebase onto it, must not restale the group's review")
	}
	runGit(t, worktree, "checkout", "-q", "epic")
	commitDated(t, worktree, "b/five.go", "package b\n", "the epic moved again", now)
	runGit(t, worktree, "checkout", "-q", "-")
	runGit(t, worktree, "merge", "-q", "--no-edit", "epic")
	if !reviewed(root, plan) {
		t.Fatal("a catch-up merge brings in no group work, so it must not restale the review")
	}
	commitDated(t, worktree, "a/four.go", "package a\n", "a repair", now)
	if reviewed(root, plan) {
		t.Fatal("the group's own commit after the review must restale it")
	}
}

// TestShipGroupPushesFromAWorktreeThatRefusesPush cuts the group the way the line does, with its
// refused pushurl, and proves ship still lands the branch on origin.
func TestShipGroupPushesFromAWorktreeThatRefusesPush(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	backlogtest.SeedText(t, root, shipBacklog)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed")
	runGit(t, root, "push", "origin", "main")
	runGit(t, root, "fetch", "origin")
	worktree := filepath.Join(root, StateDir, "wt", "TG-09.1")
	if err := AddDetached(root, "feat/a-group", "main", worktree); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", worktree, "push", "origin", "feat/a-group").CombinedOutput(); err == nil {
		t.Fatalf("the cut worktree must refuse a plain push, out = %s", out)
	}
	if err := os.WriteFile(filepath.Join(worktree, "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "-A")
	runGit(t, worktree, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group",
		Worktree: filepath.Join(StateDir, "wt", "TG-09.1"),
		Tasks:    []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	saveReview(t, root, "TG-09.1", `{"findings":[]}`)
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", bare, "branch", "--list", "feat/a-group").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "feat/a-group") {
		t.Fatalf("branch = %s, err = %v; ship must land the branch on origin", out, err)
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/heads/feat/a-group"); err == nil {
		t.Fatal("ship created a local branch; it pushes the tip ref and keeps refs/heads to the person")
	}
	tip, _ := git.Run(root, "rev-parse", TipRef("feat/a-group"))
	if pushed, _ := git.Run(bare, "rev-parse", "refs/heads/feat/a-group"); pushed != tip {
		t.Fatalf("origin holds %s, tip ref is %s; ship must push the tip", pushed, tip)
	}
	if refused, err := git.Run(worktree, "config", "--get", "remote.origin.pushurl"); err != nil || refused != RefusedPushURL {
		t.Fatalf("pushurl = %q; ship must leave the worktree's refusal in place", refused)
	}
}

// unreachableOrigin makes root a repo whose origin names a path that does not exist, so a ship
// resolves the URL and then fails at the push itself.
func unreachableOrigin(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
}

// TestPushErrorsNeverCarryACredential checks a failed push redacts a token the origin URL holds.
func TestPushErrorsNeverCarryACredential(t *testing.T) {
	text := redactURL("fatal: unable to access 'https://x-access-token:SECRET@github.com/o/r.git/': 403", "https://x-access-token:SECRET@github.com/o/r.git")
	if strings.Contains(text, "SECRET") {
		t.Fatalf("text = %q; a push error must never carry a token", text)
	}
	if other := redactURL("remote: https://u:p@example.com/x denied", ""); strings.Contains(other, "u:p") {
		t.Fatalf("other = %q; any URL credential must be redacted", other)
	}
}

// shipWithBase ships the shipRepo group against base and returns the gh pr create call and result.
func shipWithBase(t *testing.T, base string, pushBase bool) (string, *ShipResult) {
	t.Helper()
	root, group := shipRepo(t)
	runGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	if pushBase {
		runGit(t, group, "push", "origin", "HEAD:refs/heads/"+base)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: base, Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	var create string
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			create = strings.Join(args, "\x00")
		}
		return "https://example.com/pull/1", nil
	}}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatal(err)
	}
	return create, result
}

func TestShipKeepsABaseOriginStillHas(t *testing.T) {
	create, result := shipWithBase(t, "feat/stack", true)
	if result.Base != "feat/stack" || result.StaleBase != "" {
		t.Fatalf("base = %q, stale = %q; a present base is kept", result.Base, result.StaleBase)
	}
	if !strings.Contains(create, "--base\x00feat/stack\x00") || strings.Contains(create, "is gone from origin") {
		t.Fatalf("create = %q", create)
	}
}

func TestShipTargetsTheDefaultBranchWhenTheBaseIsDeleted(t *testing.T) {
	create, result := shipWithBase(t, "feat/gone", false)
	if result.Base != "trunk" || result.StaleBase != "feat/gone" {
		t.Fatalf("base = %q, stale = %q; a deleted base becomes the default branch", result.Base, result.StaleBase)
	}
	if !strings.Contains(create, "--base\x00trunk\x00") {
		t.Fatalf("create = %q; the pull request must target the default branch", create)
	}
	if !strings.Contains(create, "Base feat/gone is gone from origin, so this targets trunk.") {
		t.Fatalf("create = %q; the body must name the switch", create)
	}
}

// twoTaskPlan is a stacked two-task group whose second task declared two files.
func twoTaskPlan() *Plan {
	return &Plan{
		Group: "TG-09.1", Title: "A group", Base: "feat/stack",
		Tasks: []PlanTask{
			{ID: "TSK-09.1.1", Title: "Do one", Files: []string{"a/one.go"}},
			{ID: "TSK-09.1.2", Title: "Do two", Files: []string{"a/two.go", "a/two_test.go"}},
		},
	}
}

func TestReportBodyRendersTheFourSectionsInOrder(t *testing.T) {
	plan := twoTaskPlan()
	result := &ShipResult{Base: "feat/stack", Done: []string{"TSK-09.1.1", "TSK-09.1.2"}}
	waves := []*WaveResult{{Wave: 1, Gates: []CommandResult{{Command: "go vet ./..."}}, Verify: &CommandResult{Command: "go test ./...", ExitCode: 1}}}
	context := BodyContext{Why: "The line needs it.", DefaultBase: "main", BlastRadius: "low", BlastRadiusWhy: "one package"}
	body := ReportBody(plan, result, waves, context)
	last := -1
	for _, heading := range []string{"## Summary", "## Changes", "## Validation", "## Dependencies"} {
		at := strings.Index(body, heading)
		if at <= last {
			t.Fatalf("%s is missing or out of order:\n%s", heading, body)
		}
		last = at
	}
	for _, want := range []string{
		"The line needs it.",
		"- **TSK-09.1.1** — Do one (`a/one.go`)",
		"- **TSK-09.1.2** — Do two (`a/two.go`, `a/two_test.go`)",
		"- `go vet ./...` passed", "- `go test ./...` exited 1", "- **Blast radius** low: one package",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q:\n%s", want, body)
		}
	}
	for _, banned := range []string{"Co-authored-by", "Generated", "session"} {
		if strings.Contains(body, banned) {
			t.Errorf("body carries %q:\n%s", banned, body)
		}
	}
}

func TestReportBodyNamesAStackedBaseUnderDependencies(t *testing.T) {
	body := ReportBody(twoTaskPlan(), &ShipResult{Base: "feat/stack"}, nil, BodyContext{DefaultBase: "main"})
	deps := body[strings.Index(body, "## Dependencies"):]
	if !strings.Contains(deps, "`feat/stack`") {
		t.Fatalf("dependencies do not name the base:\n%s", body)
	}
	plain := ReportBody(twoTaskPlan(), &ShipResult{Base: "main"}, nil, BodyContext{DefaultBase: "main"})
	if strings.Contains(plain, "## Dependencies") {
		t.Fatalf("a group on the default branch has no dependencies:\n%s", plain)
	}
}

func TestReportBodyKeepsABlockedTaskUnderValidation(t *testing.T) {
	body := ReportBody(twoTaskPlan(), &ShipResult{Blocked: []string{"TSK-09.1.2"}}, nil, BodyContext{})
	validation := body[strings.Index(body, "## Validation"):]
	if !strings.Contains(validation, "TSK-09.1.2 Do two is blocked") {
		t.Fatalf("the blocked task is not under Validation:\n%s", body)
	}
}

func TestReportBodyNotesADiffOverThePreferredLines(t *testing.T) {
	context := BodyContext{DefaultBase: "main", SizeNote: sizeNote(1500, 1000)}
	body := ReportBody(twoTaskPlan(), &ShipResult{Base: "feat/stack"}, nil, context)
	if !strings.Contains(body, "over the preferred 1000") {
		t.Fatalf("body does not note the preferred size:\n%s", body)
	}
}

func TestSizeNoteIsEmptyAtOrUnderThePreferred(t *testing.T) {
	if sizeNote(1000, 1000) != "" {
		t.Fatal("a diff at the preferred count must not get a note")
	}
	if sizeNote(500, 1000) != "" {
		t.Fatal("a diff under the preferred count must not get a note")
	}
	if sizeNote(1500, 0) != "" {
		t.Fatal("a zero preferred must never note")
	}
}

func TestTemplateSectionsFollowTheRepoTemplate(t *testing.T) {
	dir := t.TempDir()
	if got := templateSections(dir); strings.Join(got, ",") != "Summary,Changes,Validation,Dependencies" {
		t.Fatalf("no template: sections = %v", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	template := "<!-- note -->\n\n## Validation\n\n## Summary\n\n## Notes\n\n## Changes\n"
	if err := os.WriteFile(filepath.Join(dir, templatePath), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := templateSections(dir); strings.Join(got, ",") != "Validation,Summary,Changes,Dependencies" {
		t.Fatalf("template: sections = %v", got)
	}
}

func TestShipStampsTheChangedLinesOnItsRow(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "two.go"), []byte("package b\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if err := SaveRun(root, RunState{Run: "TG-09.1-1", Group: "TG-09.1", Base: "main", Branch: "feat/a-group"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	entries, err := Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Station == "ship" {
			if entry.Lines != 5 {
				t.Fatalf("lines = %d; one line replaced and three added is five changed", entry.Lines)
			}
			return
		}
	}
	t.Fatal("ship stamped no row")
}

func TestChangedLinesIsZeroWhenTheBaseIsUnknown(t *testing.T) {
	_, group := shipRepo(t)
	if got := ChangedLines(group, "no-such-base", "feat/a-group"); got != 0 {
		t.Fatalf("lines = %d; an unknown base is no count, never a guess", got)
	}
}

func TestReviewSizeIsZeroWhenTheBaseIsUnknown(t *testing.T) {
	_, group := shipRepo(t)
	if files, added := ReviewSize(group, "no-such-base", "feat/a-group"); files != 0 || added != 0 {
		t.Fatalf("size = %d file(s), %d line(s); an unknown base is no count, never a guess", files, added)
	}
}

func TestReviewSizeCountsNoDeletion(t *testing.T) {
	_, group := shipRepo(t)
	if err := os.WriteFile(filepath.Join(group, "gone.go"), []byte("package a\n\nvar a = 1\nvar b = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "grow")
	runGit(t, group, "branch", "main")
	runGit(t, group, "rm", "-q", "gone.go")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "shrink")
	if files, added := ReviewSize(group, "main", "feat/a-group"); files != 1 || added != 1 {
		t.Fatalf("size = %d file(s), %d line(s); a deleted file and deleted lines must count nothing", files, added)
	}
}

func TestReviewSizeSkipsTheLinesBookkeeping(t *testing.T) {
	_, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	for _, path := range []string{"docs/backlog/TG-1.md", "code.go"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(group, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(group, path), []byte("one\ntwo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "ship")
	if files, added := ReviewSize(group, "main", "feat/a-group"); files != 1 || added != 2 {
		t.Fatalf("size = %d file(s), %d line(s); only code.go is the reviewer's to read", files, added)
	}
}

func TestCheckPRSizeRefusesOverEitherCeiling(t *testing.T) {
	if err := checkPRSize("TG-1", 5, 100, 0, 0); err != nil {
		t.Fatalf("a zero ceiling must never refuse: %v", err)
	}
	if err := checkPRSize("TG-1", 21, 100, 20, 2000); err == nil || !strings.Contains(err.Error(), "split") {
		t.Fatalf("err = %v; a file count over the cap must name a split", err)
	}
	if err := checkPRSize("TG-1", 5, 2001, 20, 2000); err == nil || !strings.Contains(err.Error(), "split") {
		t.Fatalf("err = %v; a line count over the cap must name a split", err)
	}
}

func TestShipRefusesAGroupOverThePullRequestFileCeiling(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	for _, name := range []string{"two.go", "three.go"} {
		if err := os.WriteFile(filepath.Join(group, name), []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	plan.Profile.PRFiles = 1
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "split") {
		t.Fatalf("err = %v; a diff over the file ceiling must refuse and name a split", err)
	}
}

func TestShipRefusesAGroupOverThePullRequestLineCeiling(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "two.go"), []byte("package b\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	plan.Profile.PRLinesMax = 3
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "split") {
		t.Fatalf("err = %v; a diff over the line ceiling must refuse and name a split", err)
	}
}

func TestShipNotesTheBodyWhenOverThePreferredLines(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "two.go"), []byte("package b\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	plan.Profile.PRLinesPreferred = 3
	var calls []string
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, "\x00"))
		return "https://example.com/pull/1", nil
	}}
	if _, err := ShipGroup(root, plan, nil, client); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, call := range calls {
		if strings.Contains(call, "over the preferred 3") {
			found = true
		}
	}
	if !found {
		t.Fatalf("calls = %v; a diff over the preferred lines must note it in the body", calls)
	}
}

func TestAReshipRefreshesTheOpenPullRequest(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	if err := os.WriteFile(filepath.Join(group, "two.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	var edited []string
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			return "", errors.New("a pull request for branch \"feat/a-group\" into branch \"main\" already exists")
		case len(args) > 1 && args[0] == "pr" && args[1] == "view":
			return `{"number":7,"url":"https://example.com/pull/7","state":"OPEN"}`, nil
		case len(args) > 2 && args[0] == "pr" && args[1] == "edit" && args[3] == "--title":
			edited = append(edited, args[2])
		}
		return "[]", nil
	}}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatalf("ShipGroup = %v; a re-ship must refresh its open pull request", err)
	}
	if result.URL != "https://example.com/pull/7" || len(edited) != 1 {
		t.Fatalf("url = %q, edits = %v; the open pull request's title and body must be refreshed", result.URL, edited)
	}
}

func TestScopeLabelRules(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"guard", []string{"internal/guard/guard.go"}, "scope/guard"},
		{"mount", []string{"internal/mount/mount.go"}, "scope/mount"},
		{"skills", []string{"komodo/skills/build.md"}, "scope/skills"},
		{"roles", []string{"komodo/roles/build.md"}, "scope/skills"},
		{"agents", []string{"internal/profile/tier.go"}, "scope/agents"},
		{"machine", []string{"internal/profile/machine.go"}, "scope/agents"},
		{"harness", []string{"internal/line/ship.go"}, "scope/harness"},
		{"the profile itself", []string{"internal/profile/profile.go"}, "scope/agents"},
		{"none", nil, "scope/harness"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scopeLabel(c.files); got != c.want {
				t.Fatalf("scopeLabel(%v) = %q, want %q", c.files, got, c.want)
			}
		})
	}
}

func TestShipWarnsWhenTheRepoLacksAWantedLabel(t *testing.T) {
	root, group := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "label" {
			return `[{"name":"@agent 🤖"}]`, nil
		}
		return "https://example.com/pull/1", nil
	}}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Labels) != 1 || result.Labels[0] != "@agent 🤖" {
		t.Fatalf("labels = %v", result.Labels)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "scope/harness") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v; a wanted label the repo lacks must be a warning", result.Warnings)
	}
}

func TestShipRefusesWhenTaskIsRefinedOnlyAtRoot(t *testing.T) {
	root, _ := shipRepo(t)
	refined := "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-09.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go, a/two.go]\ndone_when:\n  - true\n  - echo more\ncontext: [docs/guide.md]\n```\n"
	backlogtest.SeedText(t, root, refined)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "TSK-09.1.1") {
		t.Fatalf("err = %v; ship must refuse when a task differs between root and worktree, naming the task", err)
	}
}

func TestShipLeavesTheChangelogToTheRelease(t *testing.T) {
	worktree := gitRepo(t)
	commitBacklogText(t, worktree, staleWorktreeBacklog, "the backlog")
	commit(t, worktree, "CHANGELOG.md", "# Changelog\n", "the changelog")
	root := t.TempDir()
	backlogtest.SeedText(t, root, closedRootBacklog)
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, worktree, "checkout", "-q", "-b", "feat/a-group")
	plan := &Plan{
		Group: "TG-12.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-12.1.1", Title: "One", Status: "READY"}},
	}
	client := &pr.Client{Dir: worktree, Run: func(string, ...string) (string, error) {
		return "https://example.com/pull/1", nil
	}}
	saveReview(t, root, "TG-12.1", `{"findings":[]}`)
	if _, err := ShipGroup(root, plan, nil, client); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(worktree, "CHANGELOG.md")); string(data) != "# Changelog\n" {
		t.Fatalf("ship edited CHANGELOG.md:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(worktree, "changelog.d")); !os.IsNotExist(err) {
		t.Fatalf("ship wrote a changelog.d: %v; only a release writes the changelog", err)
	}
}

func TestCatchUpRebasesOntoAMovedBaseKeepingEdits(t *testing.T) {
	_, group := shipRepo(t)
	runGit(t, group, "branch", "main", "HEAD~0")
	runGit(t, group, "checkout", "-q", "main")
	commitDated(t, group, "two.go", "package a\n", "base moved", time.Now())
	runGit(t, group, "checkout", "-q", "feat/a-group")
	commitDated(t, group, "three.go", "package a\n", "group work", time.Now())
	if err := os.WriteFile(filepath.Join(group, "CHANGELOG.md"), []byte("# Changelog\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "CHANGELOG.md")
	if err := catchUp(group, group, "feat/a-group", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(group, "two.go")); err != nil {
		t.Fatal("the base's commit is not under the group branch")
	}
	if _, err := os.Stat(filepath.Join(group, "CHANGELOG.md")); err != nil {
		t.Fatal("the uncommitted ship edit was lost")
	}
}

func TestCatchUpMergesAPushedBranchInsteadOfRewritingIt(t *testing.T) {
	_, group := shipRepo(t)
	runGit(t, group, "branch", "main", "HEAD~0")
	runGit(t, group, "checkout", "-q", "main")
	commitDated(t, group, "two.go", "package a\n", "base moved", time.Now())
	runGit(t, group, "checkout", "-q", "feat/a-group")
	commitDated(t, group, "three.go", "package a\n", "group work", time.Now())
	runGit(t, group, "push", "-q", "origin", "feat/a-group")
	runGit(t, group, "fetch", "-q", "origin", "feat/a-group:refs/remotes/origin/feat/a-group")
	pushed, err := git.Run(group, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := catchUp(group, group, "feat/a-group", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(group, "merge-base", "--is-ancestor", pushed, "HEAD"); err != nil {
		t.Fatal("catch-up rewrote a pushed branch; its pushed commit is no longer an ancestor")
	}
	if _, err := os.Stat(filepath.Join(group, "two.go")); err != nil {
		t.Fatal("the base's commit is not under the group branch")
	}
}

func TestCatchUpStopsOnAConflictNamingTheFile(t *testing.T) {
	_, group := shipRepo(t)
	runGit(t, group, "branch", "main", "HEAD~0")
	runGit(t, group, "checkout", "-q", "main")
	commitDated(t, group, "one.go", "package base\n", "base edits one.go", time.Now())
	runGit(t, group, "checkout", "-q", "feat/a-group")
	commitDated(t, group, "one.go", "package group\n", "group edits one.go", time.Now())
	err := catchUp(group, group, "feat/a-group", "main")
	if err == nil || !strings.Contains(err.Error(), "one.go") {
		t.Fatalf("want a conflict naming one.go, got %v", err)
	}
	if status, _ := exec.Command("git", "-C", group, "status", "--porcelain").Output(); strings.Contains(string(status), "UU") {
		t.Fatal("the rebase was left half done")
	}
}

// envValue reads one key back out of a scrubbed environment.
func envValue(list []string, key string) (string, bool) {
	for _, entry := range list {
		if name, value, found := strings.Cut(entry, "="); found && name == key {
			return value, true
		}
	}
	return "", false
}

func TestScrubDropsEveryPushCredential(t *testing.T) {
	base := []string{
		"GH_TOKEN=secret", "GITHUB_TOKEN=secret", "GH_ENTERPRISE_TOKEN=secret",
		"GIT_ASKPASS=/bin/askpass", "SSH_AUTH_SOCK=/tmp/agent.sock", "PATH=/usr/bin",
	}
	scrubbed := Scrub(base)
	for _, key := range dropped {
		if _, found := envValue(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
	if path, _ := envValue(scrubbed, "PATH"); path != "/usr/bin" {
		t.Fatalf("PATH = %q, want the inherited value", path)
	}
}

func TestScrubLeavesGitUnableToPromptOrAuthenticate(t *testing.T) {
	scrubbed := Scrub([]string{"PATH=/usr/bin"})
	want := map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_CONFIG_COUNT":    "1",
		"GIT_CONFIG_KEY_0":    "credential.helper",
		"GIT_CONFIG_VALUE_0":  "",
	}
	for key, value := range want {
		if got, found := envValue(scrubbed, key); !found || got != value {
			t.Fatalf("%s = %q, %v; want %q", key, got, found, value)
		}
	}
	if ssh, _ := envValue(scrubbed, "GIT_SSH_COMMAND"); !strings.Contains(ssh, "IdentitiesOnly=yes") {
		t.Fatalf("GIT_SSH_COMMAND = %q", ssh)
	}
}

func TestScrubIgnoresAnIdentityFileConfiguredOutsideTheEnvironment(t *testing.T) {
	scrubbed := Scrub([]string{"PATH=/usr/bin"})
	ssh, _ := envValue(scrubbed, "GIT_SSH_COMMAND")
	if !strings.Contains(ssh, "-F "+os.DevNull) {
		t.Fatalf("GIT_SSH_COMMAND = %q, want -F %s so ~/.ssh/config never applies", ssh, os.DevNull)
	}
}

func TestScrubDropsATokenByItsNameShapeNotJustAnExactSpelling(t *testing.T) {
	base := []string{
		"GITHUB_PAT=secret", "HOMEBREW_GITHUB_API_TOKEN=secret", "GIT_CONFIG_PARAMETERS=secret",
		"PATH=/usr/bin",
	}
	scrubbed := Scrub(base)
	for _, key := range []string{"GITHUB_PAT", "HOMEBREW_GITHUB_API_TOKEN", "GIT_CONFIG_PARAMETERS"} {
		if _, found := envValue(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
}

func TestScrubKeepsTheModelHostsOwnLoginAndDropsEveryForgeSecret(t *testing.T) {
	base := []string{"MODELHOST_OAUTH_TOKEN=login", "PATH=/usr/bin", "GITLAB_TOKEN=secret", "BITBUCKET_PASSWORD=secret"}
	scrubbed := Scrub(base)
	for _, key := range []string{"MODELHOST_OAUTH_TOKEN", "PATH"} {
		if _, found := envValue(scrubbed, key); !found {
			t.Fatalf("%s was scrubbed; only a forge's push credential may be", key)
		}
	}
	for _, key := range []string{"GITLAB_TOKEN", "BITBUCKET_PASSWORD"} {
		if _, found := envValue(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
}

// TestHookEnvDropsACustomForgeTokenByItsShape proves hookEnv shares Scrub's pattern match, so a
// custom forge token such as GITLAB_TOKEN never reaches a pre-push hook.
func TestHookEnvDropsACustomForgeTokenByItsShape(t *testing.T) {
	scrubbed := hookEnv([]string{"GITLAB_TOKEN=secret", "PATH=/usr/bin"})
	if _, found := envValue(scrubbed, "GITLAB_TOKEN"); found {
		t.Fatal("GITLAB_TOKEN reached a pre-push hook's environment")
	}
	if path, _ := envValue(scrubbed, "PATH"); path != "/usr/bin" {
		t.Fatalf("PATH = %q, want the inherited value", path)
	}
}

func TestScrubDoesNotLetAnInheritedOverrideSurvive(t *testing.T) {
	scrubbed := Scrub([]string{"GIT_TERMINAL_PROMPT=1", "GIT_SSH_COMMAND=ssh -i /home/me/.ssh/id_ed25519"})
	if got, _ := envValue(scrubbed, "GIT_TERMINAL_PROMPT"); got != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, want the launcher's own value", got)
	}
	if ssh, _ := envValue(scrubbed, "GIT_SSH_COMMAND"); strings.Contains(ssh, "id_ed25519") {
		t.Fatalf("an inherited SSH identity survived: %q", ssh)
	}
	count := 0
	for _, entry := range scrubbed {
		if strings.HasPrefix(entry, "GIT_SSH_COMMAND=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GIT_SSH_COMMAND appears %d times", count)
	}
}

func TestPushFromWorktreeRefusesACriticalRef(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	err := PushFromWorktree(root, group, "main")
	if err == nil || !strings.Contains(err.Error(), "critical ref") {
		t.Fatalf("err = %v, want a refusal naming the critical ref", err)
	}
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := exec.Command("git", "-C", bare, "branch", "--list", "main").Output(); strings.TrimSpace(string(out)) != "" {
		t.Fatal("main reached origin; a critical ref is never pushed")
	}
}

func TestShipGroupHandsTheCredentialToNoAfterPublishCommand(t *testing.T) {
	root, group := shipRepo(t)
	t.Setenv("GH_TOKEN", "ghp_forgetoken")
	t.Setenv("GITHUB_TOKEN", "ghp_forgetoken")
	if err := os.MkdirAll(filepath.Join(root, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"env > published.env"}`
	if err := os.WriteFile(filepath.Join(root, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	seen, err := os.ReadFile(filepath.Join(group, "published.env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(seen), "ghp_forgetoken") {
		t.Fatal("after_publish saw the forge credential; only the push may hold it")
	}
}

func TestShipLabelsThePullRequestOnlyAfterThePush(t *testing.T) {
	root, group := shipRepo(t)
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	labelled := false
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "label" {
			return `[{"name":"@agent"},{"name":"scope/harness"}]`, nil
		}
		if len(args) > 1 && args[0] == "pr" && args[1] == "edit" && slices.Contains(args, "--add-label") {
			out, _ := exec.Command("git", "-C", bare, "branch", "--list", "feat/a-group").Output()
			if strings.TrimSpace(string(out)) == "" {
				return "", errors.New("labelled before the push")
			}
			labelled = true
		}
		return "https://example.com/pull/1", nil
	}}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatal(err)
	}
	if !labelled || len(result.Labels) != 2 {
		t.Fatalf("labels = %v, warnings = %v; the labels must follow the push", result.Labels, result.Warnings)
	}
}

// prepareRepo is shipRepo with the group's task still READY in both backlogs and its DONE recorded as live status,
// plus a main branch in the group to rebase onto.
func prepareRepo(t *testing.T) (root, group string, plan *Plan) {
	t.Helper()
	root, group = shipRepo(t)
	ready := strings.Replace(shipBacklog, "[DONE]", "[READY]", 1)
	for _, dir := range []string{root, group} {
		backlogtest.SeedText(t, dir, ready)
	}
	runGit(t, group, "commit", "-qam", "ready")
	runGit(t, group, "branch", "main")
	if err := RecordStatus(root, "TSK-09.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	plan = &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Version: "2.0.0", Base: "main", Branch: "feat/a-group",
		Worktree: "group", Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it", Files: []string{"one.go"}}},
	}
	return root, group, plan
}

// stations lists the ledger stations stamped for root, in order.
func stations(t *testing.T, root string) []string {
	t.Helper()
	entries, err := Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, entry := range entries {
		out = append(out, entry.Station+":"+entry.Outcome)
	}
	return out
}

func TestPrepareCommitsTheTickedListWithNoTrailers(t *testing.T) {
	root, group, plan := prepareRepo(t)
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package a\n\nconst Built = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixes, err := PrepareGroup(root, plan)
	if err != nil || len(fixes) > 0 {
		t.Fatalf("prepare = %q, %v", fixes, err)
	}
	subject, _ := git.Run(group, "log", "-1", "--format=%s")
	trailers, _ := git.Run(group, "log", "-1", "--format=%(trailers)")
	if subject != "feat: A group (TG-09.1)" || trailers != "" {
		t.Fatalf("commit = %q with trailers %q; want the conventional subject and no trailers", subject, trailers)
	}
	files, _ := git.Run(group, "show", "--name-only", "--format=", "HEAD")
	for _, want := range []string{"docs/backlog/TG-09.1-a-group.md", "one.go"} {
		if !slices.Contains(strings.Fields(files), want) {
			t.Fatalf("the commit holds %q, want %s in it", files, want)
		}
	}
	// The group's only task is done, so its own group file leaves with the group.
	if left, _ := git.Run(group, "ls-files", "docs/backlog"); left != "" {
		t.Fatalf("group files still tracked: %q; a finished group with no epic deletes its own", left)
	}
	if status, _ := git.Run(group, "status", "--porcelain"); status != "" {
		t.Fatalf("status = %q; prepare must commit every change", status)
	}
	if got := stations(t, root); !slices.Equal(got, []string{"prepare:done"}) {
		t.Fatalf("ledger = %v, want one passed prepare and no ship", got)
	}
}

func TestPrepareTurnsARefusedPrePushHookIntoAFixAndPushesNothing(t *testing.T) {
	root, group, plan := prepareRepo(t)
	installPrePush(t, group, 1)
	fixes, err := PrepareGroup(root, plan)
	if err != nil || len(fixes) != 1 || !strings.Contains(fixes[0], "pre-push hook refuses") {
		t.Fatalf("prepare = %q, %v; want the hook's refusal as the one fix", fixes, err)
	}
	bare, _ := git.Run(root, "remote", "get-url", "--push", "origin")
	if _, err := git.Run(bare, "rev-parse", "--verify", "--quiet", "refs/heads/feat/a-group"); err == nil {
		t.Fatal("the branch reached origin before its checks passed")
	}
	if got := stations(t, root); !slices.Equal(got, []string{"prepare:fixes"}) {
		t.Fatalf("ledger = %v, want the failed prepare and no ship", got)
	}
}

func TestPrepareLeavesAConflictForARepairRoundThenFinishesTheRebase(t *testing.T) {
	cases := []struct {
		name string
		// committed is whether Check committed the repair's resolution before Prepare ran again.
		committed bool
		// pushed is whether the branch is on origin, so it takes the base in a merge instead of a rebase.
		pushed bool
	}{
		{"the resolution is uncommitted", false, false},
		{"Check committed the resolution", true, false},
		{"a pushed branch's merge is uncommitted", false, true},
		{"Check committed a pushed branch's merge", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, group, plan := prepareRepo(t)
			if tc.pushed {
				runGit(t, group, "push", "-q", "origin", "feat/a-group")
			}
			runGit(t, group, "checkout", "-q", "main")
			commitDated(t, group, "one.go", "package a\n\nconst Base = 1\n", "base moves", time.Now())
			runGit(t, group, "checkout", "-q", "feat/a-group")
			path := filepath.Join(group, "one.go")
			if err := os.WriteFile(path, []byte("package a\n\nconst Built = 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			fixes, err := PrepareGroup(root, plan)
			if err != nil || len(fixes) != 1 || !strings.HasPrefix(fixes[0], "one.go: resolve the conflict") {
				t.Fatalf("prepare = %q, %v; want the conflict as a fix naming one.go", fixes, err)
			}
			marked, _ := os.ReadFile(path)
			if !strings.Contains(string(marked), "<<<<<<< ") {
				t.Fatalf("one.go = %q; the conflict markers must stay for the repair round", marked)
			}
			if _, err := PrepareGroup(root, plan); !errors.Is(err, errConflictRemains) {
				t.Fatalf("prepare = %v; an unresolved conflict must stop the group", err)
			}
			if err := os.WriteFile(path, []byte("package a\n\nconst Base, Built = 1, 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.committed {
				runGit(t, group, "add", "-A")
				runGit(t, group, "commit", "-qm", "as built")
			}
			if fixes, err := PrepareGroup(root, plan); err != nil || len(fixes) > 0 {
				t.Fatalf("prepare = %q, %v; want the rebase finished", fixes, err)
			}
			if _, err := git.Run(group, "merge-base", "--is-ancestor", "main", "HEAD"); err != nil {
				t.Fatal("the group does not sit on its base after the repaired rebase")
			}
			if branch, _ := git.Run(group, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feat/a-group" {
				t.Fatalf("HEAD is on %q; the rebase must finish on the group branch", branch)
			}
			if kept, _ := os.ReadFile(path); string(kept) != "package a\n\nconst Base, Built = 1, 1\n" {
				t.Fatalf("one.go = %q, want the repair's resolution", kept)
			}
		})
	}
}

func TestPrepareDeletesTheEpicsGroupFilesOnlyWithItsLastOpenGroup(t *testing.T) {
	const (
		shipping  = "## [TG-02.1] Shipping [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-02\n```\n\n- [ ] **TSK-02.1.1** One\n"
		openFile  = "## [TG-02.2] Other [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-02\n```\n\n- [ ] **TSK-02.2.1** Two\n"
		doneFile  = "## [TG-02.2] Other [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-02\n```\n\n- [x] **TSK-02.2.1** Two\n"
		elsewhere = "## [TG-03.1] Elsewhere [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-03\n```\n\n- [ ] **TSK-03.1.1** Three\n"
		loose     = "## [TG-02.1] Shipping [P: H] [READY]\n\n```yaml\ntype: feat\n```\n\n- [ ] **TSK-02.1.1** One\n"
	)
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"another group of the epic is open", map[string]string{"TG-02.1-a.md": shipping, "TG-02.2-b.md": openFile}, nil},
		{"it is the epic's last open group", map[string]string{
			"TG-02.1-a.md": shipping, "TG-02.2-b.md": doneFile, "TG-03.1-c.md": elsewhere,
		}, []string{"TG-02.1-a.md", "TG-02.2-b.md"}},
		{"it names no epic", map[string]string{"TG-02.1-a.md": loose, "TG-02.2-b.md": openFile}, []string{"TG-02.1-a.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worktree := t.TempDir()
			dir := filepath.Join(worktree, "docs", "backlog")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, text := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			ended, err := endedEpicFiles(worktree, "TG-02.1")
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, path := range ended {
				got = append(got, filepath.Base(path))
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("deleted = %v, want %v", got, tc.want)
			}
		})
	}
}

// draftForge is a fake forge that refuses drafts when noDrafts is set and fails a label add when noLabels is,
// recording every gh call.
func draftForge(dir string, noDrafts, noLabels bool, calls *[]string) *pr.Client {
	return &pr.Client{Dir: dir, Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		*calls = append(*calls, joined)
		switch {
		case noDrafts && strings.HasPrefix(joined, "pr create") && slices.Contains(args, "--draft"):
			return "", errors.New("Draft pull requests are not supported for this repository")
		case args[0] == "label":
			return `[{"name":"@agent"},{"name":"scope/harness"},{"name":"scope/agents"},{"name":"status: wip"}]`, nil
		case noLabels && slices.Contains(args, "--add-label"):
			return "", errors.New("label service unavailable")
		}
		return "https://example.com/pull/1", nil
	}}
}

// called reports whether any recorded gh call starts with prefix and holds every part.
func called(calls []string, prefix string, parts ...string) bool {
	for _, call := range calls {
		if !strings.HasPrefix(call, prefix) {
			continue
		}
		missing := false
		for _, part := range parts {
			missing = missing || !strings.Contains(call, part)
		}
		if !missing {
			return true
		}
	}
	return false
}

func TestEveryPullRequestOpensAsADraftAndTurnsReadyOnlyOnceItsChecksPassed(t *testing.T) {
	passed := []*WaveResult{{OK: true}}
	cases := []struct {
		name      string
		noDrafts  bool
		waves     []*WaveResult
		wantDraft bool
		wantReady bool
		wantWip   bool
	}{
		{"a draft whose checks passed turns ready", false, passed, false, true, false},
		{"a draft with no passed checks stays a draft", false, nil, true, false, false},
		{"a draft with a failed check stays a draft", false, []*WaveResult{{OK: false}}, true, false, false},
		{"a refused draft opens labelled status: wip, dropped once its checks passed", true, passed, false, true, false},
		{"a refused draft keeps status: wip while its checks are unproven", true, nil, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, group := shipRepo(t)
			plan := &Plan{
				Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
				Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
			}
			var calls []string
			result, err := ShipGroup(root, plan, tc.waves, draftForge(group, tc.noDrafts, false, &calls))
			if err != nil {
				t.Fatal(err)
			}
			if !called(calls, "pr create", "--draft") {
				t.Fatalf("calls = %q; every PR must first try to open as a draft", calls)
			}
			if result.Draft != tc.wantDraft || result.Ready != tc.wantReady {
				t.Fatalf("draft = %v, ready = %v; want %v, %v (calls %q)",
					result.Draft, result.Ready, tc.wantDraft, tc.wantReady, calls)
			}
			if readied := called(calls, "pr ready"); readied != (tc.wantReady && !tc.noDrafts) {
				t.Fatalf("calls = %q; pr ready ran = %v", calls, readied)
			}
			if tc.noDrafts && !called(calls, "pr edit", "--add-label", "status: wip") {
				t.Fatalf("calls = %q; a refused draft must be labelled status: wip", calls)
			}
			if dropped := called(calls, "pr edit", "--remove-label", "status: wip"); dropped != (tc.noDrafts && tc.wantReady) {
				t.Fatalf("calls = %q; status: wip removed = %v", calls, dropped)
			}
			if hasWip := slices.Contains(result.Labels, "status: wip"); hasWip != tc.wantWip {
				t.Fatalf("labels = %v; status: wip kept = %v, want %v", result.Labels, hasWip, tc.wantWip)
			}
		})
	}
}

func TestAFailedLabelCallIsAWarningNotAFailedShip(t *testing.T) {
	root, group := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it", Files: []string{"internal/profile/profile.go"}}},
	}
	var calls []string
	result, err := ShipGroup(root, plan, nil, draftForge(group, false, true, &calls))
	if err != nil {
		t.Fatalf("ship = %v; a label failure must never fail the ship", err)
	}
	if !called(calls, "pr edit", "--add-label", "scope/agents") {
		t.Fatalf("calls = %q; the profile's group must ask for scope/agents", calls)
	}
	if result.URL == "" || !slices.ContainsFunc(result.Warnings, func(w string) bool {
		return strings.Contains(w, "could not add label(s): ") && strings.Contains(w, "label service unavailable")
	}) {
		t.Fatalf("url = %q, warnings = %v; want the PR opened and the failed label call warned", result.URL, result.Warnings)
	}
}

func TestCredentialRefusedReadsAMissingOrExpiredCredential(t *testing.T) {
	cases := []struct {
		output string
		want   bool
	}{
		{"remote: Invalid username or password.\nfatal: Authentication failed for 'https://github.com/o/r.git/'", true},
		{"fatal: could not read Username for 'https://github.com': terminal prompts disabled", true},
		{"git@github.com: Permission denied (publickey).", true},
		{"error: The requested URL returned error: 403", true},
		{"fatal: unable to access 'https://127.0.0.1:1/o/r.git/': Failed to connect", false},
		{"! [rejected] feat/a-group -> feat/a-group (non-fast-forward)", false},
	}
	for _, tc := range cases {
		if got := credentialRefused(tc.output); got != tc.want {
			t.Errorf("credentialRefused(%q) = %v, want %v", tc.output, got, tc.want)
		}
	}
}

// refusingRemote puts a git-remote-refuse helper on PATH that fails every push as an expired credential would.
func refusingRemote(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"fatal: Authentication failed for 'https://forge.invalid/o/r.git/'\" >&2\nexit 128\n"
	if err := os.WriteFile(filepath.Join(bin, "git-remote-refuse"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return "refuse://forge.invalid/o/r.git"
}

func TestAnExpiredCredentialStopsShipWithABlockerNoteAndKomodoShipFinishesIt(t *testing.T) {
	root, group := shipRepo(t)
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "remote", "set-url", "--push", "origin", refusingRemote(t))
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	var calls []string
	client := draftForge(group, false, false, &calls)
	if _, err := ShipGroup(root, plan, nil, client); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("ship = %v, want the push stopped for its credential", err)
	}
	if len(calls) > 0 {
		t.Fatalf("gh ran %q; a group stopped before Ship opens no PR", calls)
	}
	if _, err := os.Stat(HandoffPath(root, "TG-09.1")); err != nil {
		t.Fatalf("no handoff for komodo ship to finish: %v", err)
	}
	noted, _ := git.Run(group, "show", "HEAD:docs/backlog/TG-09.1-a-group.md")
	if !strings.Contains(noted, "komodo ship TG-09.1") || !strings.Contains(noted, "Authentication failed") {
		t.Fatalf("the branch's backlog = %q, want a committed blocker note naming the fix", noted)
	}
	if got := stations(t, root); !slices.Equal(got, []string{"ship:handoff"}) {
		t.Fatalf("ledger = %v, want the ship handed off, not done", got)
	}

	runGit(t, root, "remote", "set-url", "--push", "origin", bare)
	result, err := FinishShip(root, "TG-09.1", client)
	if err != nil {
		t.Fatalf("komodo ship = %v", err)
	}
	if result.URL == "" || !result.Draft || !called(calls, "pr create", "--draft", "--head feat/a-group") {
		t.Fatalf("result = %+v, calls = %q; want the PR opened as a draft", result, calls)
	}
	pushed, err := git.Run(bare, "show", "refs/heads/feat/a-group:docs/backlog/TG-09.1-a-group.md")
	if err != nil || strings.Contains(pushed, "komodo ship TG-09.1") {
		t.Fatalf("pushed backlog = %q (%v); the note must be gone and the commits kept", pushed, err)
	}
	if _, err := os.Stat(HandoffPath(root, "TG-09.1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("handoff = %v; komodo ship must remove it", err)
	}
	if got := stations(t, root); !slices.Equal(got, []string{"ship:handoff", "ship:done"}) {
		t.Fatalf("ledger = %v, want the finished ship stamped done", got)
	}
}

func TestKomodoShipRefusesAGroupWithNoHandoff(t *testing.T) {
	if _, err := FinishShip(t.TempDir(), "TG-09.1", nil); err == nil || !strings.Contains(err.Error(), "no ship waiting") {
		t.Fatalf("finish = %v, want a refusal naming no waiting ship", err)
	}
}

func TestKomodoShipRefusesAHandoffNamingAnotherGroupOrABadBranch(t *testing.T) {
	cases := []struct {
		name    string
		handoff ShipHandoff
		want    string
	}{
		{"another group", ShipHandoff{Group: "TG-09.2", Branch: "feat/a-group"}, "nothing was pushed"},
		{"an option as its branch", ShipHandoff{Group: "TG-09.1", Branch: "--force"}, "nothing was pushed"},
		{"an invalid branch", ShipHandoff{Group: "TG-09.1", Branch: "feat/a..b"}, "not a valid branch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := HandoffPath(root, "TG-09.1")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(tc.handoff)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := FinishShip(root, "TG-09.1", nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("finish = %v, want a refusal naming %q", err, tc.want)
			}
		})
	}
}

func TestKomodoShipRefusesAHandoffWhoseWorktreeDiffersFromItsRun(t *testing.T) {
	root, _ := shipRepo(t)
	rogue := t.TempDir()
	handOff(t, root, rogue, "")
	if _, err := FinishShip(root, "TG-09.1", nil); err == nil || !strings.Contains(err.Error(), "nothing was pushed") {
		t.Fatalf("finish = %v, want the mismatched worktree refused", err)
	}
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(bare, "rev-parse", "--verify", "--quiet", "refs/heads/feat/a-group"); err == nil {
		t.Fatal("a refused mismatch must push nothing")
	}
}

func TestKomodoShipRefusesAHandoffWithNoRunRecorded(t *testing.T) {
	root := t.TempDir()
	path := HandoffPath(root, "TG-09.1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	handoff := ShipHandoff{Group: "TG-09.1", Branch: "feat/a-group"}
	data, err := json.Marshal(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FinishShip(root, "TG-09.1", nil); err == nil || !strings.Contains(err.Error(), "no run recorded") {
		t.Fatalf("finish = %v, want the missing run refused", err)
	}
}

// handOff writes the shipRepo group's handoff as a scrubbed ship leaves it, with after_publish as given.
func handOff(t *testing.T, root, group, afterPublish string) {
	t.Helper()
	handoff := ShipHandoff{
		Group: "TG-09.1", Worktree: group, Branch: "feat/a-group", Base: "main", Title: "feat: A group (TG-09.1)",
		Body: "b", Labels: []string{"@agent"}, Draft: true, AfterPublish: afterPublish,
	}
	if err := writeShipHandoff(root, handoff); err != nil {
		t.Fatal(err)
	}
}

func TestKomodoShipRunsAfterPublishAndPushesWithNoForgeClient(t *testing.T) {
	root, group := shipRepo(t)
	handOff(t, root, group, "touch published.txt")
	result, err := FinishShip(root, "TG-09.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "" || result.Published == nil || !result.Published.OK() {
		t.Fatalf("result = %+v; want after_publish run and no PR without a client", result)
	}
	if _, err := os.Stat(filepath.Join(group, "published.txt")); err != nil {
		t.Fatalf("after_publish did not run: %v", err)
	}
	bare, _ := git.Run(root, "remote", "get-url", "--push", "origin")
	if _, err := git.Run(bare, "rev-parse", "--verify", "--quiet", "refs/heads/feat/a-group"); err != nil {
		t.Fatal("komodo ship did not push the branch")
	}
}

func TestKomodoShipFailsOnAFailedAfterPublishAndKeepsTheHandoff(t *testing.T) {
	root, group := shipRepo(t)
	handOff(t, root, group, "exit 3")
	if _, err := FinishShip(root, "TG-09.1", nil); err == nil || !strings.Contains(err.Error(), "after_publish") {
		t.Fatalf("finish = %v, want the after_publish failure", err)
	}
	if _, err := os.Stat(HandoffPath(root, "TG-09.1")); err != nil {
		t.Fatalf("handoff = %v; a failed finish must leave it for the next komodo ship", err)
	}
}

func TestKomodoShipRefreshesAPullRequestAlreadyOpen(t *testing.T) {
	cases := []struct {
		name    string
		view    string
		wantURL string
	}{
		{"an open PR is reused", `{"number":7,"url":"https://example.com/pull/7","state":"OPEN","isDraft":true}`,
			"https://example.com/pull/7"},
		{"a closed PR fails the finish", `{"number":7,"url":"https://example.com/pull/7","state":"CLOSED"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, group := shipRepo(t)
			handOff(t, root, group, "")
			client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
				switch {
				case args[0] == "pr" && args[1] == "create":
					return "", errors.New("a pull request already exists")
				case args[0] == "pr" && args[1] == "view":
					return tc.view, nil
				}
				return "[]", nil
			}}
			result, err := FinishShip(root, "TG-09.1", client)
			if tc.wantURL == "" {
				if err == nil {
					t.Fatal("finish = nil, want the create's failure")
				}
				return
			}
			if err != nil || result.URL != tc.wantURL || !result.Draft {
				t.Fatalf("finish = %+v, %v; want the open draft PR reused", result, err)
			}
		})
	}
}

func TestAFailedReadyCallLeavesTheDraftWithAWarning(t *testing.T) {
	root, group := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		switch {
		case args[0] == "label":
			return `[{"name":"@agent"},{"name":"scope/harness"}]`, nil
		case args[0] == "pr" && args[1] == "ready":
			return "", errors.New("forbidden")
		}
		return "https://example.com/pull/1", nil
	}}
	result, err := ShipGroup(root, plan, []*WaveResult{{OK: true}}, client)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Draft || result.Ready || !slices.ContainsFunc(result.Warnings, func(w string) bool {
		return strings.Contains(w, "could not mark the PR ready for review: ")
	}) {
		t.Fatalf("result = %+v; a failed ready call must leave a draft and warn", result)
	}
}

func TestMarkReadyDropsStatusWipFromANormalPullRequest(t *testing.T) {
	cases := []struct {
		name   string
		labels string
		fail   bool
		want   string
	}{
		{"the repo defines status: wip", `[{"name":"status: wip 🚧"}]`, false, "pr edit u --remove-label status: wip 🚧"},
		{"the repo has no status: wip", `[{"name":"@agent"}]`, false, ""},
		{"the labels cannot be listed", "", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			client := &pr.Client{Dir: ".", Run: func(_ string, args ...string) (string, error) {
				calls = append(calls, strings.Join(args, " "))
				if tc.fail {
					return "", errors.New("gh is down")
				}
				return tc.labels, nil
			}}
			err := markReady(client, "u", false)
			if tc.fail != (err != nil) {
				t.Fatalf("markReady = %v, want failure %v", err, tc.fail)
			}
			if tc.want != "" && !slices.Contains(calls, tc.want) {
				t.Fatalf("calls = %q, want %q", calls, tc.want)
			}
			if tc.want == "" && len(calls) > 1 {
				t.Fatalf("calls = %q; nothing to remove means no edit", calls)
			}
		})
	}
}

func TestPrepareStopsOnAConflictThatHasNoConflictedFile(t *testing.T) {
	root, group, plan := prepareRepo(t)
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package a\n\nconst Built = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixes, err := settleCatchUp(group, "main", []string{"merge", "--abort"}, "merge", "no-such-ref")
	if err == nil || len(fixes) > 0 {
		t.Fatalf("settle = %q, %v; a failure with no conflict is an error, never a fix", fixes, err)
	}
	if _, err := PrepareGroup(root, plan); err != nil {
		t.Fatalf("prepare = %v; the aborted step must leave the group preparable", err)
	}
}

func TestPrepareCommitsTheDeletionOfAnEndedEpicsGroupFiles(t *testing.T) {
	const (
		shipping = "## [TG-02.1] Shipping [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-02\n```\n\n- [ ] **TSK-02.1.1** One\n"
		done     = "## [TG-02.2] Other [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-02\n```\n\n- [x] **TSK-02.2.1** Two\n"
	)
	root := gitRepo(t)
	commit(t, root, "docs/backlog/TG-02.2-b.md", done, "the other group")
	commit(t, root, "docs/backlog/TG-02.1-a.md", shipping, "the shipping group")
	plan := &Plan{Group: "TG-02.1", Title: "Shipping", Type: "feat", Base: "main", Branch: "main"}
	if fixes, err := PrepareGroup(root, plan); err != nil || len(fixes) > 0 {
		t.Fatalf("prepare = %q, %v", fixes, err)
	}
	if left, _ := git.Run(root, "ls-files", "docs/backlog"); left != "" {
		t.Fatalf("group files still tracked: %q; the epic's last group deletes them", left)
	}
	if subject, _ := git.Run(root, "log", "-1", "--format=%s"); subject != "feat: Shipping (TG-02.1)" {
		t.Fatalf("HEAD = %q, want the group's commit carrying the deletion", subject)
	}
}

func TestPrepareTurnsARefusedPreCommitHookIntoAFix(t *testing.T) {
	root, group, plan := prepareRepo(t)
	hook := filepath.Join(group, ".git", "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'the pre-commit gate refuses' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixes, err := PrepareGroup(root, plan)
	if err != nil || len(fixes) != 1 || !strings.Contains(fixes[0], "the pre-commit gate refuses") {
		t.Fatalf("prepare = %q, %v; want the hook's refusal as the one fix", fixes, err)
	}
	if got := stations(t, root); !slices.Equal(got, []string{"prepare:fixes"}) {
		t.Fatalf("ledger = %v, want the refused prepare", got)
	}
}

func TestEndedEpicFilesFailsOnAGroupFileItCannotRead(t *testing.T) {
	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, "docs", "backlog", "TG-02.1-a.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := endedEpicFiles(worktree, "TG-02.1"); err == nil {
		t.Fatal("endedEpicFiles = nil, want the unreadable file's error")
	}
}

func TestChecksPassedNeedsAtLeastOneRunAndEveryOnePassed(t *testing.T) {
	cases := []struct {
		name  string
		waves []*WaveResult
		want  bool
	}{
		{"none ran", nil, false},
		{"all passed", []*WaveResult{{OK: true}, {OK: true}}, true},
		{"one failed", []*WaveResult{{OK: true}, {OK: false}}, false},
		{"one is missing", []*WaveResult{{OK: true}, nil}, false},
	}
	for _, tc := range cases {
		if got := checksPassed(tc.waves); got != tc.want {
			t.Errorf("%s: checksPassed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestKomodoShipRefusesAMalformedHandoff(t *testing.T) {
	root := t.TempDir()
	path := HandoffPath(root, "TG-09.1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FinishShip(root, "TG-09.1", nil); err == nil {
		t.Fatal("finish = nil, want the malformed handoff refused")
	}
}

func TestKomodoShipKeepsTheHandoffWhileTheCredentialIsStillRefused(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, root, "remote", "set-url", "--push", "origin", refusingRemote(t))
	handOff(t, root, group, "")
	if _, err := FinishShip(root, "TG-09.1", nil); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("finish = %v, want the refused credential", err)
	}
	if _, err := os.Stat(HandoffPath(root, "TG-09.1")); err != nil {
		t.Fatalf("handoff = %v; a refused finish must leave it for the next komodo ship", err)
	}
}

func TestAFinishStillRefusedKeepsTheCredentialNoteOnTheBranch(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, root, "remote", "set-url", "--push", "origin", refusingRemote(t))
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("ship = %v, want the push stopped for its credential", err)
	}
	backlogPath := filepath.Join(group, "docs", "backlog", "TG-09.1-a-group.md")
	noted, err := os.ReadFile(backlogPath)
	if err != nil || !strings.Contains(string(noted), "komodo ship TG-09.1") {
		t.Fatalf("backlog = %q, %v; the first stop must leave its blocker note", noted, err)
	}
	if _, err := FinishShip(root, "TG-09.1", nil); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("finish = %v, want the still-refused credential", err)
	}
	noted, err = os.ReadFile(backlogPath)
	if err != nil || !strings.Contains(string(noted), "komodo ship TG-09.1") {
		t.Fatalf("backlog = %q, %v; a group still blocked must keep its note on the branch", noted, err)
	}
}

func TestPrepareFailsOnWhatItCannotReadOrWrite(t *testing.T) {
	cases := []struct {
		name  string
		spoil func(t *testing.T, root, group string, plan *Plan)
	}{
		{"a live status names a task neither backlog holds", func(t *testing.T, root, _ string, plan *Plan) {
			plan.Tasks = append(plan.Tasks, PlanTask{ID: "TSK-09.1.9", Title: "Gone"})
			if err := RecordStatus(root, "TSK-09.1.9", "DONE"); err != nil {
				t.Fatal(err)
			}
		}},
		{"a group file cannot be read", func(t *testing.T, _, group string, _ *Plan) {
			if err := os.MkdirAll(filepath.Join(group, "docs", "backlog", "TG-09.1-a.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, group, plan := prepareRepo(t)
			tc.spoil(t, root, group, plan)
			if _, err := PrepareGroup(root, plan); err == nil {
				t.Fatal("prepare = nil, want the failure")
			}
			if got := stations(t, root); !slices.Equal(got, []string{"prepare:failed"}) {
				t.Fatalf("ledger = %v, want the failed prepare", got)
			}
		})
	}
}

func TestTheCredentialNoteNeedsABacklogAndItsRemovalSkipsNone(t *testing.T) {
	if err := writeCredentialNote(t.TempDir(), t.TempDir(), "TG-09.1", "A group", "feat/a-group", ErrNoCredential); err == nil {
		t.Fatal("note = nil, want a worktree with no backlog refused")
	}
	if err := dropCredentialNote(t.TempDir(), ShipHandoff{Group: "TG-09.1"}); err != nil {
		t.Fatalf("drop = %v; a worktree with no backlog has no note to drop", err)
	}
}

func TestKomodoShipFailsOnAHandoffItCannotRead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(HandoffPath(root, "TG-09.1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := FinishShip(root, "TG-09.1", nil); err == nil || strings.Contains(err.Error(), "no ship waiting") {
		t.Fatalf("finish = %v, want the read's own failure", err)
	}
}

func TestLabelWipWarnsOnEachFailure(t *testing.T) {
	cases := []struct {
		name   string
		labels string
		list   error
		add    error
		want   string
	}{
		{"the labels cannot be listed", "", errors.New("offline"), nil, "could not list labels"},
		{"the repo has no such label", `[{"name":"status: blocked"}]`, nil, nil, "the repo has no status: wip label"},
		{"the label cannot be added", `[{"name":"status: wip 🚧"}]`, nil, errors.New("forbidden"), "could not add label(s)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
				if args[0] == "label" {
					return tc.labels, tc.list
				}
				return "", tc.add
			}}
			labels, warnings := labelWip(client, "https://example.com/pull/7")
			if len(labels) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0], tc.want) {
				t.Fatalf("labels %v, warnings %v; want no label and a warning naming %q", labels, warnings, tc.want)
			}
		})
	}
}

func TestShipBlockedKeepsTheNoteLocalWhenThePrePushGateRefuses(t *testing.T) {
	root, group := shipRepo(t)
	stopGroup(t, group)
	envLog := installPrePush(t, group, 1)
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run once the push is refused: %v", args)
		return "", nil
	}}
	result, err := ShipBlocked(root, blockedPlan(), blockedNote, client)
	if err == nil || !strings.Contains(err.Error(), "pre-push hook") {
		t.Fatalf("ship blocked = %v, want the pre-push gate's refusal", err)
	}
	if result == nil || result.URL != "" || !slices.Equal(result.Blocked, []string{"TSK-09.1.1"}) {
		t.Fatalf("result = %+v, want the open task blocked and no pull request", result)
	}
	if _, err := os.Stat(envLog); err != nil {
		t.Fatalf("the pre-push hook never ran: %v", err)
	}
	subjects, err := git.Run(group, "log", "--format=%s", "-2")
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(subjects, "\n"); len(lines) != 2 ||
		!strings.HasPrefix(lines[0], "docs: A group is blocked") || !strings.HasPrefix(lines[1], "wip: A group") {
		t.Fatalf("commits = %q, want the WIP commit then the note, both local", lines)
	}
	data, err := os.ReadFile(filepath.Join(group, "docs", "backlog", "TG-09.1-a-group.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "> - Needs: a decision on the clock") {
		t.Fatalf("backlog =\n%s\nwant the blocker note kept on the branch", data)
	}
	if heads, err := git.Run(group, "ls-remote", "origin", "refs/heads/feat/a-group"); err != nil || heads != "" {
		t.Fatalf("ls-remote = %q, %v; want nothing on origin after the refusal", heads, err)
	}
}

// detachedGroup cuts a detached worktree for branch from main in a root with a bare origin, as the line does.
func detachedGroup(t *testing.T, branch string) (root, bare, worktree string) {
	t.Helper()
	root = t.TempDir()
	bare = filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	commit(t, root, "base.txt", "base\n", "base")
	runGit(t, root, "push", "-q", "origin", "main")
	runGit(t, root, "fetch", "-q", "origin")
	worktree = filepath.Join(root, StateDir, "wt", "g")
	if err := AddDetached(root, branch, "origin/main", worktree); err != nil {
		t.Fatal(err)
	}
	return root, bare, worktree
}

func TestPushSendsTheTipRefToTheBranchAndGivesThePrePushHookItsObject(t *testing.T) {
	root, bare, worktree := detachedGroup(t, "feat/tip")
	envLog := installPrePush(t, root, 0)
	commit(t, worktree, "work.txt", "work\n", "work")
	if err := PushFromWorktree(root, worktree, "feat/tip"); err != nil {
		t.Fatal(err)
	}
	head, _ := git.Run(worktree, "rev-parse", "HEAD")
	if tip, _ := git.Run(root, "rev-parse", TipRef("feat/tip")); tip != head {
		t.Fatalf("tip = %s, want the worktree's HEAD %s", tip, head)
	}
	if pushed, _ := git.Run(bare, "rev-parse", "refs/heads/feat/tip"); pushed != head {
		t.Fatalf("origin = %s, want %s", pushed, head)
	}
	if seen, _ := os.ReadFile(envLog); !strings.Contains(string(seen), "refs/heads/feat/tip "+head) {
		t.Fatalf("the pre-push hook never read the tip's object:\n%s", seen)
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/heads/feat/tip"); err == nil {
		t.Fatal("the push cut a local branch")
	}
	if _, err := git.Run(worktree, "config", "--get", "branch.feat/tip.merge"); err == nil {
		t.Fatal("the push set an upstream")
	}
}

func TestSizeCapReadsTheTipRefOverTheLocalBranch(t *testing.T) {
	root, _, worktree := detachedGroup(t, "feat/size")
	runGit(t, root, "branch", "feat/size", "origin/main")
	commit(t, worktree, "work.txt", "work\n", "work")
	if err := syncTip(root, worktree, "feat/size"); err != nil {
		t.Fatal(err)
	}
	if got := diffTip(root, "feat/size"); got != TipRef("feat/size") {
		t.Fatalf("diffTip = %q, want the tip ref", got)
	}
	if files, _ := ReviewSize(worktree, "origin/main", diffTip(root, "feat/size")); files != 1 {
		t.Fatalf("files = %d, want the tip's one file", files)
	}
}

func TestCatchUpRebasesADetachedGroupAndAdvancesItsTip(t *testing.T) {
	root, _, worktree := detachedGroup(t, "feat/catch")
	commit(t, worktree, "work.txt", "work\n", "work")
	if err := syncTip(root, worktree, "feat/catch"); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "moved.txt", "moved\n", "base moved")
	runGit(t, root, "push", "-q", "origin", "main")
	if err := catchUp(root, worktree, "feat/catch", "main"); err != nil {
		t.Fatal(err)
	}
	head, _ := git.Run(worktree, "rev-parse", "HEAD")
	if tip, _ := git.Run(root, "rev-parse", TipRef("feat/catch")); tip != head {
		t.Fatalf("tip = %s, want the rebased HEAD %s", tip, head)
	}
	if _, err := os.Stat(filepath.Join(worktree, "moved.txt")); err != nil {
		t.Fatal("the base's commit is not under the group")
	}
}

func TestCatchUpMergesWhatAPersonPushedToTheBranchBeforeTheBase(t *testing.T) {
	root, bare, worktree := detachedGroup(t, "feat/person")
	commit(t, worktree, "work.txt", "work\n", "work")
	if err := PushFromWorktree(root, worktree, "feat/person"); err != nil {
		t.Fatal(err)
	}
	person := filepath.Join(t.TempDir(), "person")
	runGit(t, "", "clone", "-q", bare, person)
	runGit(t, person, "config", "user.email", "p@example.com")
	runGit(t, person, "config", "user.name", "p")
	runGit(t, person, "checkout", "-q", "feat/person")
	commit(t, person, "theirs.txt", "theirs\n", "a person's fix")
	runGit(t, person, "push", "-q", "origin", "feat/person")
	runGit(t, root, "fetch", "-q", "origin")
	if err := catchUp(root, worktree, "feat/person", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "theirs.txt")); err != nil {
		t.Fatal("the person's push is not merged into the group")
	}
	head, _ := git.Run(worktree, "rev-parse", "HEAD")
	if tip, _ := git.Run(root, "rev-parse", TipRef("feat/person")); tip != head {
		t.Fatalf("tip = %s, want %s", tip, head)
	}
}

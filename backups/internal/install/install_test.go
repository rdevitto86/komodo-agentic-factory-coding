package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/changelog"
)

// trackedRepo makes a fresh git repo a test can commit tracked fixture files into.
func trackedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return root
}

// commitAll stages and commits every change in the fixture.
func commitAll(t *testing.T, root, message string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestAddIgnoreAppendsAMissingEntryAndKeepsEveryExistingLine(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n*.log"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Host: "repo", Root: root}
	plan.AddIgnore("/.komodo/", "state")
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "node_modules/\n*.log\n/.komodo/\n" {
		t.Fatalf(".gitignore = %q", got)
	}
	again := Plan{Host: "repo", Root: root}
	again.AddIgnore("/.komodo/", "state")
	if len(again.Changes) != 0 {
		t.Fatal("an entry already present was added twice")
	}
	fresh := Plan{Host: "repo", Root: t.TempDir()}
	fresh.AddIgnore("/.komodo/", "state")
	if len(fresh.Changes) != 1 || string(fresh.Changes[0].Body) != "/.komodo/\n" {
		t.Fatalf("a repo with no .gitignore got %+v", fresh.Changes)
	}
}

func TestAddIgnoreFoldsSeveralEntriesIntoOneChange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("a\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Host: "repo", Root: root}
	plan.AddIgnore("/.komodo/", "state")
	plan.AddIgnore("/.host/skills/run/SKILL.md", "rendered")
	plan.AddIgnore("/.komodo/", "state")
	if len(plan.Changes) != 1 || string(plan.Changes[0].Body) != "a\r\n/.komodo/\r\n/.host/skills/run/SKILL.md\r\n" {
		t.Fatalf("several entries got %+v", plan.Changes)
	}
}

func TestAddIgnoreKeepsACRLFFilesLineEnding(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("a\r\nb"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Host: "repo", Root: root}
	plan.AddIgnore("/.komodo/", "state")
	if len(plan.Changes) != 1 || string(plan.Changes[0].Body) != "a\r\nb\r\n/.komodo/\r\n" {
		t.Fatalf("a CRLF .gitignore got %+v", plan.Changes)
	}
}

// TestAHookNamingAnotherBinaryIsDrift proves every hook must name the one fixed binary, so a stale
// path, such as a global hook left on an old build, is drift install rewrites and doctor reports.
func TestAHookNamingAnotherBinaryIsDrift(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	installed := `{"hooks": [{"command": "C:\\tools\\komodo.exe guard"}]}`
	if err := os.WriteFile(path, []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Host: "h", Root: root}
	plan.Add(path, []byte(`{"hooks": [{"command": "/home/a/.komodo/bin/komodo guard"}]}`), "settings")
	if got := plan.Actions(); len(got) != 1 || got[0].Verb != "update" {
		t.Fatalf("actions = %+v, want update for a hook naming another binary", got)
	}
	if got := HookBinaries([]byte(installed)); len(got) != 1 || got[0] != `C:\tools\komodo.exe` {
		t.Fatalf("hook binaries = %q", got)
	}
}

// TestHookBinariesNamesTheSuitedOrchestratorGuard proves the orchestrator's own guard, suite flag
// included, still names its binary, so drift and a stale global render are both still caught.
func TestHookBinariesNamesTheSuitedOrchestratorGuard(t *testing.T) {
	installed := `{"hooks": [{"command": "/home/a/.komodo/bin/komodo guard --suite orchestrator"}]}`
	got := HookBinaries([]byte(installed))
	if len(got) != 1 || got[0] != "/home/a/.komodo/bin/komodo" {
		t.Fatalf("hook binaries = %q", got)
	}
}

func TestInstalledFollowsTheMarker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), ".rendered")
	if !(Plan{}).Installed() {
		t.Fatal("a repo plan with no marker is always installed")
	}
	plan := Plan{Marker: marker}
	if plan.Installed() {
		t.Fatal("a missing marker means the layer was never installed")
	}
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !plan.Installed() {
		t.Fatal("a present marker means the layer is installed")
	}
}

func TestActionsNameWhatWouldChange(t *testing.T) {
	root := t.TempDir()
	same := filepath.Join(root, "same.txt")
	if err := os.WriteFile(same, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, "stale.txt")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Host: "test", Root: root}
	plan.Add(same, []byte("body"), "unchanged")
	plan.Add(stale, []byte("new"), "changed")
	plan.Add(filepath.Join(root, "new.txt"), []byte("new"), "added")
	plan.AddRemoval(filepath.Join(root, "absent.txt"), "gone already")

	verbs := map[string]string{}
	for _, action := range plan.Actions() {
		verbs[action.Path] = action.Verb
	}
	if verbs["same.txt"] != "same" || verbs["stale.txt"] != "update" || verbs["new.txt"] != "create" {
		t.Fatalf("verbs = %v", verbs)
	}
	if _, named := verbs["absent.txt"]; named {
		t.Fatal("a removal of an absent path was reported")
	}
}

func TestApplyWritesAndRemoves(t *testing.T) {
	root := t.TempDir()
	doomed := filepath.Join(root, "old", "thing.txt")
	if err := os.MkdirAll(filepath.Dir(doomed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doomed, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Host: "test", Root: root}
	plan.Add(filepath.Join(root, "deep", "new.txt"), []byte("new"), "added")
	plan.AddRemoval(filepath.Join(root, "old"), "retired")
	done, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 2 {
		t.Fatalf("done = %+v", done)
	}
	if body, err := os.ReadFile(filepath.Join(root, "deep", "new.txt")); err != nil || string(body) != "new" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, "old")); !os.IsNotExist(err) {
		t.Fatal("the retired path survived")
	}
}

func TestASeedIsWrittenOnceAndNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "overlay.json")
	plan := Plan{Host: "test", Root: root}
	plan.AddSeed(path, []byte("{}"), "the personal overlay")
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"mine":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != `{"mine":true}` {
		t.Fatalf("the seed overwrote a personal file: %q", body)
	}
	for _, action := range plan.Actions() {
		if action.Path == "overlay.json" && action.Verb != "keep" {
			t.Fatalf("verb = %s, want keep", action.Verb)
		}
	}
}

func TestPrintCountsWhatWouldChange(t *testing.T) {
	root := t.TempDir()
	plan := Plan{Host: "claude", Root: root}
	plan.Add(filepath.Join(root, "a.txt"), []byte("a"), "added")
	var out strings.Builder
	plan.Print(&out)
	if !strings.Contains(out.String(), "create  a.txt") || !strings.Contains(out.String(), "claude: 1 file(s), 1 would change") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestProjectNarrowsToProjectChanges(t *testing.T) {
	root := t.TempDir()
	plan := Plan{Host: "test", Root: root}
	plan.Add(filepath.Join(root, "agents", "builder.md"), []byte("agent"), "not project")
	plan.AddProject(filepath.Join(root, "skills", "go", "SKILL.md"), []byte("skill"), "a repo skill")
	narrowed := plan.Project()
	if len(narrowed.Changes) != 1 || narrowed.Changes[0].Path != filepath.Join(root, "skills", "go", "SKILL.md") {
		t.Fatalf("changes = %+v", narrowed.Changes)
	}
}

func TestApplyIsACopyNotASymlink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "skill", "SKILL.md")
	plan := Plan{Host: "test", Root: root}
	plan.Add(path, []byte("body"), "a skill")
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("the install wrote a symlink")
	}
}

func TestKomodoHookMatchesOnlyAKomodoBinarysGuardOrHook(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"/opt/bin/komodo guard", true},
		{`C:\tools\komodo.exe guard`, true},
		{"/repo/bin/komodo-linux-amd64 hook status --host claude", true},
		{"/usr/bin/other guard", false},
		{"/usr/local/bin/my-audit", false},
		{"komodo run", false},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			if got := KomodoHook(tc.command); got != tc.want {
				t.Fatalf("KomodoHook(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestGlobalReturnsTheRegisteredRender(t *testing.T) {
	RegisterGlobal("test-global", func(root, home, binary string) (Plan, error) {
		return Plan{Host: "test-global", Root: home}, nil
	})
	render, ok := Global("test-global")
	if !ok {
		t.Fatal("a registered global render was not found")
	}
	if plan, err := render("", t.TempDir(), "komodo"); err != nil || plan.Host != "test-global" {
		t.Fatalf("plan = %+v, err = %v", plan, err)
	}
	if _, ok := Global("nope"); ok {
		t.Fatal("an unregistered host has a global render")
	}
}

func TestGlobalPlanRendersIntoHomeAndRefusesAHostWithoutOne(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	RegisterGlobal("test-home", func(root, home, binary string) (Plan, error) {
		return Plan{Host: "test-home", Root: home}, nil
	})
	plan, err := GlobalPlan("test-home", "", "komodo")
	if err != nil || plan.Root != home {
		t.Fatalf("plan = %+v, err = %v, want it rooted at HOME %s", plan, err, home)
	}
	if _, err := GlobalPlan("nope", "", "komodo"); err == nil || !strings.Contains(err.Error(), "no user-level config") {
		t.Fatalf("err = %v, want a host without a global render refused", err)
	}
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("home", "")
	if _, err := GlobalPlan("test-home", "", "komodo"); err == nil {
		t.Fatal("a user with no home directory got a plan")
	}
}

// TestApplyNeverWritesToAFileGitTracks proves install leaves a tracked file alone, even when its
// rendered body differs, so a branch switch never leaves the installer's own edits as local drift.
func TestApplyNeverWritesToAFileGitTracks(t *testing.T) {
	root := trackedRepo(t)
	path := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# rules\ncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "init")

	plan := Plan{Host: "repo", Root: root}
	plan.Add(path, []byte("# rules\nrendered\n"), "rules import")
	done, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 0 {
		t.Fatalf("done = %+v, want no action for a tracked file", done)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# rules\ncommitted\n" {
		t.Fatalf("CLAUDE.md = %q, want the committed body untouched", got)
	}
}

// TestActionsReportsATrackedFileAsKeptNotUpdated proves drift never fires for a file git tracks.
func TestActionsReportsATrackedFileAsKeptNotUpdated(t *testing.T) {
	root := trackedRepo(t)
	path := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "init")

	plan := Plan{Host: "repo", Root: root}
	plan.Add(path, []byte("node_modules/\n/.komodo/\n"), "state")
	actions := plan.Actions()
	if len(actions) != 1 || actions[0].Verb != "keep" {
		t.Fatalf("actions = %+v, want keep for a tracked file", actions)
	}
}

// TestActionsFlagsATrackedSkillThatDiffersFromTheRender proves a tracked file's kept action carries
// Tracked, so a caller can still name it, though install itself never writes over it.
func TestActionsFlagsATrackedSkillThatDiffersFromTheRender(t *testing.T) {
	root := trackedRepo(t)
	path := filepath.Join(root, ".claude", "skills", "run", "SKILL.md")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "init")

	plan := Plan{Host: "repo", Root: root}
	plan.AddProject(path, []byte("new body\n"), "the run skill")
	actions := plan.Actions()
	if len(actions) != 1 || actions[0].Verb != "keep" || !actions[0].Tracked {
		t.Fatalf("actions = %+v, want one kept action flagged Tracked", actions)
	}

	same := Plan{Host: "repo", Root: root}
	same.AddProject(path, []byte("old body\n"), "the run skill")
	if got := same.Actions(); len(got) != 1 || got[0].Verb != "keep" || got[0].Tracked {
		t.Fatalf("actions = %+v, want Tracked false once the content matches", got)
	}
}

// TestApplyStillWritesAnUntrackedFile proves the tracked-file skip never reaches a plain, ungoverned path.
func TestApplyStillWritesAnUntrackedFile(t *testing.T) {
	root := trackedRepo(t)
	path := filepath.Join(root, "settings.json")

	plan := Plan{Host: "repo", Root: root}
	plan.Add(path, []byte(`{"ok": true}`), "settings")
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok": true}` {
		t.Fatalf("settings.json = %q, want the rendered body for an untracked file", got)
	}
}

func TestApplyWritesEachFileWholeAndLeavesNoTemp(t *testing.T) {
	root := t.TempDir()
	plan := Plan{Host: "h", Root: root}
	path := filepath.Join(root, "nested", "settings.json")
	plan.Add(path, []byte(`{"hooks": []}`), "settings")
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != `{"hooks": []}` {
		t.Fatalf("file = %q, %v", data, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Fatalf("dir holds %d entries; a temp file was left behind", len(entries))
	}
}

func TestAnInstallerDefaultMustBeTheNewestChangelogVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, rel, body, want string
	}{
		{"sh current", "install.sh", `x/${KOMODO_VERSION:-v1.0.0-beta.5}"`, ""},
		{"sh lags", "install.sh", `x/${KOMODO_VERSION:-v1.0.0-beta.2}"`, "install.sh: defaults KOMODO_VERSION to v1.0.0-beta.2"},
		{"ps1 current", "install.ps1", `else { 'https://x' })/$(if ($env:KOMODO_VERSION) { $env:KOMODO_VERSION } else { 'v1.0.0-beta.5' })`, ""},
		{"ps1 lags", "install.ps1", `else { 'v1.0.0-beta.4' })`, "install.ps1: defaults KOMODO_VERSION to v1.0.0-beta.4"},
		{"no default", "install.sh", "exec komodo install\n", "install.sh: names no default KOMODO_VERSION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := defaultVersionProblem(tc.rel, tc.body, "1.0.0-beta.5")
			if (tc.want == "") != (got == "") || !strings.HasPrefix(got, tc.want) {
				t.Fatalf("problem = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestScriptProblemsNamesAnInstallerDefaultThatLagsTheChangelog(t *testing.T) {
	root := scriptRepo(t, map[string]string{
		"CHANGELOG.md": "# Changelog\n\n## 1.0.0-beta.5 — 2026-10-04\n\n## 1.0.0-beta.4 — 2026-10-02\n",
		"install.sh":   "#!/bin/sh\nbase=\"x/${KOMODO_VERSION:-v1.0.0-beta.2}\"\n",
		"install.ps1":  "$v = $(if ($env:KOMODO_VERSION) { $env:KOMODO_VERSION } else { 'v1.0.0-beta.5' })\n",
	})
	got := strings.Join(ScriptProblems(root), "\n")
	if !strings.Contains(got, "install.sh: defaults KOMODO_VERSION to v1.0.0-beta.2") {
		t.Fatalf("problems = %q, want the lagging install.sh named", got)
	}
	if strings.Contains(got, "install.ps1: defaults") {
		t.Fatalf("problems = %q; install.ps1 matches the changelog", got)
	}
}

func TestTheCommittedInstallersDefaultToTheNewestChangelogVersion(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	text, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	latest := changelog.Latest(string(text))
	for _, rel := range []string{"install.sh", "install.ps1"} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if problem := defaultVersionProblem(rel, string(data), latest); problem != "" {
			t.Fatal(problem)
		}
	}
}

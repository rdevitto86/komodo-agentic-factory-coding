package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestDriftIgnoresWhichKomodoBinaryTheHookRuns(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	installed := `{"hooks": [{"command": "C:\\tools\\komodo.exe guard"}]}`
	if err := os.WriteFile(path, []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Host: "h", Root: root}
	plan.Add(path, []byte(`{"hooks": [{"command": "/repo/bin/komodo-linux-amd64 guard"}]}`), "settings")
	if got := plan.Drift(); len(got) != 1 || got[0].Verb != "same" {
		t.Fatalf("drift = %+v, want same", got)
	}
	if got := plan.Actions(); got[0].Verb != "update" {
		t.Fatalf("actions = %+v, want update so install still rewrites the path", got)
	}
	other := Plan{Host: "h", Root: root}
	other.Add(path, []byte(`{"hooks": [{"command": "/usr/bin/other guard"}]}`), "settings")
	if got := other.Drift(); got[0].Verb != "update" {
		t.Fatalf("drift = %+v, want a non-komodo hook to count", got)
	}
	if got := HookBinaries([]byte(installed)); len(got) != 1 || got[0] != `C:\tools\komodo.exe` {
		t.Fatalf("hook binaries = %q", got)
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

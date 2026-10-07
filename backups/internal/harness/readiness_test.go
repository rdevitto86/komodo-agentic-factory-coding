package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/plan"
)

// TestTaskTimeoutReadsItsOwnKey proves a task's own timeout key overrides the station default.
func TestTaskTimeoutReadsItsOwnKey(t *testing.T) {
	text := "### [TG-90.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-90.1.1] Hangs [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"sleep 30\"]\ntimeout: 300ms\n```\n"
	parsed := backlog.Parse(text)
	task, _ := parsed.Task("TSK-90.1.1")
	if TaskTimeout(task) != 300*time.Millisecond {
		t.Fatalf("timeout = %s", TaskTimeout(task))
	}
}

func TestContextSlotNamesAMissingSectionInsteadOfTheWholeFile(t *testing.T) {
	root := t.TempDir()
	text := "### [TG-90.2] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-90.2.1] T [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test\"]\ncontext: [spec.md#refunds]\n```\n"
	parsed := backlog.Parse(text)
	task, _ := parsed.Task("TSK-90.2.1")
	body := "# Spec\n\n## Payments\n" + strings.Repeat("payments text ", 500)
	if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := contextSlot(root, task, CapPerFile, CapFilesTotal)
	if strings.Contains(got, "payments text") || !strings.Contains(got, `no section "refunds"`) {
		t.Fatalf("slot = %q", got)
	}
	problems := backlog.LintContext(root, parsed)
	if len(problems) != 1 || !strings.Contains(problems[0], "spec.md#refunds") {
		t.Fatalf("lint = %v", problems)
	}
	if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte(body+"\n## Refunds\nrefund rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if problems := backlog.LintContext(root, parsed); len(problems) != 0 {
		t.Fatalf("lint = %v after the heading exists", problems)
	}
	if got := contextSlot(root, task, CapPerFile, CapFilesTotal); !strings.Contains(got, "refund rules") {
		t.Fatalf("slot = %q", got)
	}
}

func TestATaskAddedMidRunJoinsAWaveAfterThePinnedOnes(t *testing.T) {
	root := repo(t, groupText)
	state := RunState{Run: "r", Group: "TG-05.1", Base: "main", Branch: "feat/a-group", Worktree: root,
		Waves: [][]string{{"TSK-05.1.1", "TSK-05.1.2"}, {"TSK-05.1.3"}}}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	late := "\n#### [TSK-05.1.4] Four [P: C] [READY]\n```yaml\nfiles: [c/four.go]\ndone_when: [\"go test ./c/...\"]\n```\n"
	reseed(t, root, groupText+late)
	plan, err := planForGroup(root, "TG-05.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Waves) != 3 || plan.Waves[2][0] != "TSK-05.1.4" {
		t.Fatalf("waves = %v; the late task must follow the pinned waves", plan.Waves)
	}
}

func TestATaskClosedBeforeTheRunNeverJoinsALateWave(t *testing.T) {
	root := repo(t, groupText)
	state := RunState{Run: "r", Group: "TG-05.1", Base: "main", Branch: "feat/a-group", Worktree: root,
		Waves: [][]string{{"TSK-05.1.1", "TSK-05.1.2"}, {"TSK-05.1.3"}}}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	shipped := "\n#### [TSK-05.1.0] Shipped [P: C] [DONE]\n```yaml\nfiles: [c/zero.go]\ndone_when: [\"go test ./c/...\"]\n```\n"
	reseed(t, root, groupText+shipped)
	plan, err := planForGroup(root, "TG-05.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Waves) != 2 {
		t.Fatalf("waves = %v; a task DONE before the run started must not be rebuilt", plan.Waves)
	}
}

func TestWaveCapacityCarriesOverflowIntoTheNextWaveWithItsPeers(t *testing.T) {
	text := "### [TG-90.3] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		text += "#### [TSK-90.3." + name + "] T [P: C] [READY]\n```yaml\nfiles: [" + name + "/x.go]\ndone_when: [\"true\"]\n```\n\n"
	}
	text += "#### [TSK-90.3.f] Dep [P: C] [READY]\n```yaml\nfiles: [f/x.go]\ndone_when: [\"true\"]\ndepends_on: [TSK-90.3.a]\n```\n"
	parsed := backlog.Parse(text)
	waves, err := plan.Waves(parsed.Groups[0].Tasks, nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 || len(waves[0]) != 4 || len(waves[1]) != 2 {
		t.Fatalf("waves = %v; the fifth task and the dependent must share wave two", waves)
	}
}

func TestRefuseOpenRunBlocksASecondGroupUntilTheFirstShips(t *testing.T) {
	root := repo(t, groupText)
	if err := SaveRun(root, RunState{Run: "r", Group: "TG-05.1", Base: "main", Branch: "feat/a-group", Worktree: root}); err != nil {
		t.Fatal(err)
	}
	if err := RefuseOpenRun(root, "TG-05.1"); err != nil {
		t.Fatalf("the open group itself was refused: %v", err)
	}
	if err := RefuseOpenRun(root, "TG-99.9"); err == nil || !strings.Contains(err.Error(), "TG-05.1 is open") {
		t.Fatalf("err = %v", err)
	}
}

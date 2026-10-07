package harness

import (
	"fmt"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/pr"
)

// epicBacklog holds one epic with a title and goal, and one group shipping its version.
func epicBacklog() backlog.Backlog {
	epic := backlog.NewEpic("EPIC-05", "Phase 1: the conductor drives", "2.0.0")
	epic.Goal = "one group runs through the conductor within 60 minutes."
	return backlog.Backlog{Epics: []backlog.Epic{epic}, Groups: []backlog.Group{{ID: "TG-05.1", Title: "A group", EpicID: "EPIC-05"}}}
}

// epicRepo builds a remoted repo with main pushed; openEpic runs against a backlog built in memory.
func epicRepo(t *testing.T) string {
	t.Helper()
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/main")
	return root
}

// remoteHead is origin's epic branch line as ls-remote prints it.
func remoteHead(t *testing.T, root string) string {
	t.Helper()
	out, err := git.Run(root, "ls-remote", "--heads", "origin", "refs/heads/feat/2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEpicBranchNameIsEmptyWithNoVersion(t *testing.T) {
	if got := EpicBranchName(""); got != "" {
		t.Fatalf("EpicBranchName(\"\") = %q, want empty", got)
	}
	if got := EpicBranchName("2.0.0"); got != "feat/2.0.0" {
		t.Fatalf("EpicBranchName(\"2.0.0\") = %q", got)
	}
}

func TestOpenEpicCutsPushesAndOpensADraftPull(t *testing.T) {
	root := epicRepo(t)
	var calls []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "https://example.com/pull/9", nil
	}}
	plan := &Plan{Group: "TG-05.1", Version: "2.0.0"}
	result, err := openEpic(root, epicBacklog(), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Branch != "feat/2.0.0" || result.URL != "https://example.com/pull/9" || !result.Draft {
		t.Fatalf("result = %+v", result)
	}
	if _, err := git.Run(root, "ls-remote", "--exit-code", "--heads", "origin", "refs/heads/feat/2.0.0"); err != nil {
		t.Fatalf("the epic branch never reached origin: %v", err)
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/heads/feat/2.0.0"); err == nil {
		t.Fatal("opening the epic cut a local branch; it must push origin/main alone")
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v, want exactly one pull request created", calls)
	}
	for _, want := range []string{
		"--draft", "--base main", "--head feat/2.0.0",
		"feat: Phase 1: the conductor drives (2.0.0)",
		"one group runs through the conductor within 60 minutes.",
	} {
		if !strings.Contains(calls[0], want) {
			t.Errorf("call %q is missing %q", calls[0], want)
		}
	}
}

func TestOpenEpicLabelsStatusWipWhenDraftsAreUnavailable(t *testing.T) {
	root := epicRepo(t)
	var calls []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		switch {
		case strings.Contains(joined, "--draft"):
			return "", fmt.Errorf("Draft pull requests are not supported for this repository")
		case strings.HasPrefix(joined, "pr create"):
			return "https://example.com/pull/9", nil
		case strings.HasPrefix(joined, "label list"):
			return `[{"name":"status/wip"}]`, nil
		default:
			return "", nil
		}
	}}
	plan := &Plan{Group: "TG-05.1", Version: "2.0.0"}
	result, err := openEpic(root, epicBacklog(), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft {
		t.Fatalf("draft = true, want a normal pull request once drafts are refused")
	}
	if result.URL != "https://example.com/pull/9" {
		t.Fatalf("url = %q", result.URL)
	}
	if len(result.Labels) != 1 || result.Labels[0] != "status/wip" {
		t.Fatalf("labels = %v, want status/wip", result.Labels)
	}
}

func TestOpenEpicLeavesAnOpenEpicPullAlone(t *testing.T) {
	root := epicRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	var calls []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		if strings.HasPrefix(joined, "pr list") {
			return `[{"headRefName":"feat/2.0.0"}]`, nil
		}
		t.Fatalf("gh must only list once the epic pull request is open: %v", args)
		return "", nil
	}}
	plan := &Plan{Group: "TG-05.1", Version: "2.0.0"}
	result, err := openEpic(root, epicBacklog(), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil once the pull request is open", result)
	}
	if len(calls) != 1 || !strings.Contains(calls[0], "--head feat/2.0.0 --base main --state open") {
		t.Fatalf("calls = %v, want one open pull request lookup", calls)
	}
}

func TestOpenEpicWarnsWhenTheForgeCannotListAnExistingBranchsPulls(t *testing.T) {
	root := epicRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		if strings.Join(args[:2], " ") != "pr list" {
			t.Fatalf("gh must not create after a failed lookup: %v", args)
		}
		return "", fmt.Errorf("gh: not logged in")
	}}
	plan := &Plan{Group: "TG-05.1", Version: "2.0.0"}
	result, err := openEpic(root, epicBacklog(), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.URL != "" || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "not logged in") {
		t.Fatalf("result = %+v, want one warning naming the failed lookup", result)
	}
	if result, err := openEpic(root, epicBacklog(), plan, nil); err != nil || result != nil {
		t.Fatalf("result = %+v, %v; with no forge client an existing branch is left alone", result, err)
	}
}

func TestOpenEpicOpensThePullForABranchAlreadyOnOrigin(t *testing.T) {
	root := epicRepo(t)
	runGit(t, root, "commit", "--allow-empty", "-m", "a person's epic work")
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	tip := remoteHead(t, root)
	var created []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "pr list"):
			return "[]", nil
		case strings.HasPrefix(joined, "pr create"):
			created = append(created, joined)
			return "https://example.com/pull/12", nil
		}
		t.Fatalf("unexpected gh call: %v", args)
		return "", nil
	}}
	plan := &Plan{Group: "TG-05.1", Version: "2.0.0"}
	result, err := openEpic(root, epicBacklog(), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.URL != "https://example.com/pull/12" || !result.Draft {
		t.Fatalf("result = %+v, want the draft opened", result)
	}
	if len(created) != 1 {
		t.Fatalf("created = %v, want one pull request", created)
	}
	for _, want := range []string{
		"--draft", "--base main", "--head feat/2.0.0",
		"feat: Phase 1: the conductor drives (2.0.0)",
		"one group runs through the conductor within 60 minutes.",
	} {
		if !strings.Contains(created[0], want) {
			t.Errorf("call %q is missing %q", created[0], want)
		}
	}
	if after := remoteHead(t, root); after != tip {
		t.Fatalf("origin's epic branch moved from %q to %q; an existing branch is never recut", tip, after)
	}
}

func TestOpenEpicSkipsAGroupThatNamesNoEpic(t *testing.T) {
	text := "### [TG-15.1] First\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-15.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"
	root := epicRepo(t)
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run for a group with no epic: %v", args)
		return "", nil
	}}
	plan := &Plan{Group: "TG-15.1", Version: "2.0.0"}
	result, err := openEpic(root, backlog.Parse(text), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil for a group with no epic", result)
	}
	if _, err := git.Run(root, "ls-remote", "--exit-code", "--heads", "origin", "refs/heads/feat/2.0.0"); err == nil {
		t.Fatal("no branch should have been cut for a group with no epic")
	}
}

func TestOpenEpicSkipsAPlanWithNoVersion(t *testing.T) {
	root := epicRepo(t)
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run for a plan with no version: %v", args)
		return "", nil
	}}
	plan := &Plan{Group: "TG-05.1"}
	result, err := openEpic(root, epicBacklog(), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil with no version", result)
	}
}

func TestReadyEpicRefusesABacklogFileAndAMissingChangelogHeading(t *testing.T) {
	root := epicRepo(t)
	runGit(t, root, "fetch", "origin")
	commit(t, root, "docs/backlog/epic-05/EPIC.md", "# EPIC-05\n", "an epic folder")
	commit(t, root, "CHANGELOG.md", "# Changelog\n\n## 1.0.0 — 2026-09-30\n\n- old\n", "a changelog")
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	runGit(t, root, "fetch", "origin")
	var calls []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}}
	err := ReadyEpic(root, "2.0.0", client, "https://example.com/pull/9", true)
	if err == nil {
		t.Fatal("want a refusal while the epic adds a backlog file and the changelog lacks its version")
	}
	for _, want := range []string{"docs/backlog/epic-05/EPIC.md", "CHANGELOG.md has no heading for 2.0.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, missing %q", err, want)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("calls = %v; a refused epic must stay a draft", calls)
	}
	runGit(t, root, "rm", "-r", "-q", "docs/backlog")
	commit(t, root, "CHANGELOG.md", "# Changelog\n\n## 2.0.0 — 2026-10-05\n\n- new\n\n## 1.0.0 — 2026-09-30\n\n- old\n", "finish the epic")
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	runGit(t, root, "fetch", "origin")
	if err := ReadyEpic(root, "2.0.0", client, "https://example.com/pull/9", true); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != "pr ready https://example.com/pull/9" {
		t.Fatalf("calls = %v, want the draft marked ready", calls)
	}
}

// landedEpic pushes an epic branch with an epic folder and an old changelog, and a forge whose
// epic pull request is an open draft.
func landedEpic(t *testing.T) (string, *pr.Client, *[]string) {
	t.Helper()
	root := epicRepo(t)
	runGit(t, root, "checkout", "-q", "-b", "epic")
	commit(t, root, "docs/backlog/epic-05/EPIC.md", "# EPIC-05\n", "an epic folder")
	commit(t, root, "docs/backlog/epic-05/tg-05.1/TG.md", "# TG-05.1\n", "a group")
	commit(t, root, "CHANGELOG.md", "# Changelog\n\n## 1.0.0 — 2026-09-30\n\n- old\n", "a changelog")
	runGit(t, root, "push", "-q", "origin", "epic:refs/heads/feat/2.0.0")
	calls := &[]string{}
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		*calls = append(*calls, joined)
		if strings.HasPrefix(joined, "pr view feat/2.0.0") {
			return `{"number":9,"url":"https://example.com/pull/9","state":"OPEN","isDraft":true}`, nil
		}
		return "", nil
	}}
	return root, client, calls
}

// cutGroup commits work on a group branch cut from the epic tip, the head a ship hands the epic gate.
func cutGroup(t *testing.T, root, group string, work func()) string {
	t.Helper()
	runGit(t, root, "checkout", "-q", "-b", group, "epic")
	work()
	return group
}

func TestReadyEndingEpicRefusesTheLastGroupWhileTheEpicIsUnfinished(t *testing.T) {
	root, client, calls := landedEpic(t)
	plan := &Plan{Group: "TG-05.1", Base: "feat/2.0.0", Version: "2.0.0"}
	middle := cutGroup(t, root, "middle", func() { commit(t, root, "mid.txt", "mid\n", "a middle group") })
	if err := ReadyEndingEpic(root, plan, client, middle); err != nil || len(*calls) != 0 {
		t.Fatalf("err = %v, calls = %v; a group that leaves the epic open touches no pull request", err, *calls)
	}
	last := cutGroup(t, root, "last", func() {
		runGit(t, root, "rm", "-q", "docs/backlog/epic-05/EPIC.md")
		runGit(t, root, "commit", "-q", "-m", "the last group ends the epic")
	})
	err := ReadyEndingEpic(root, plan, client, last)
	if err == nil || !strings.Contains(err.Error(), "docs/backlog/epic-05/tg-05.1/TG.md") ||
		!strings.Contains(err.Error(), "CHANGELOG.md has no heading for 2.0.0") {
		t.Fatalf("err = %v, want the added backlog file and the missing heading named", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("calls = %v; a refused epic must stay a draft", *calls)
	}
}

func TestReadyEndingEpicReadiesTheEpicPullOnceItsLastGroupFinishesIt(t *testing.T) {
	root, client, calls := landedEpic(t)
	last := cutGroup(t, root, "last", func() {
		runGit(t, root, "rm", "-r", "-q", "docs/backlog")
		commit(t, root, "CHANGELOG.md", "# Changelog\n\n## 2.0.0 — 2026-10-05\n\n- new\n\n## 1.0.0 — 2026-09-30\n\n- old\n", "finish the epic")
	})
	if err := ReadyEndingEpic(root, &Plan{Group: "TG-05.1", Base: "main", Version: "2.0.0"}, client, last); err != nil || len(*calls) != 0 {
		t.Fatalf("err = %v, calls = %v; a group that targets no epic branch touches no pull request", err, *calls)
	}
	plan := &Plan{Group: "TG-05.1", Base: "feat/2.0.0", Version: "2.0.0"}
	offline := &pr.Client{Dir: root, Run: func(string, ...string) (string, error) { return "", fmt.Errorf("gh: offline") }}
	if err := ReadyEndingEpic(root, plan, offline, last); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("err = %v, want the failed lookup named", err)
	}
	closed := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		if args[1] != "view" {
			t.Fatalf("a closed epic pull request is left alone: %v", args)
		}
		return `{"number":9,"url":"https://example.com/pull/9","state":"CLOSED","isDraft":true}`, nil
	}}
	if err := ReadyEndingEpic(root, plan, closed, last); err != nil {
		t.Fatal(err)
	}
	if err := ReadyEndingEpic(root, plan, client, last); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || (*calls)[1] != "pr ready https://example.com/pull/9" {
		t.Fatalf("calls = %v, want the epic pull request viewed, then marked ready", *calls)
	}
}

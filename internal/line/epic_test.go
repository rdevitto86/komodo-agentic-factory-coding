package line

import (
	"fmt"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/pr"
)

// epicBacklog holds one epic with a goal line and one group that ships its version.
const epicBacklog = "## [EPIC-05] Phase 1: the conductor drives\n" +
	"*Goal: one group runs through the conductor within 60 minutes. Ships as `2.0.0`.*\n\n" +
	"### [TG-05.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

// epicRepo builds a remoted repo with main pushed; the group-file grammar cannot yet carry an
// epic's phase title or goal, so openEpic runs against text parsed straight into memory.
func epicRepo(t *testing.T) string {
	t.Helper()
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/main")
	return root
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
	result, err := openEpic(root, backlog.Parse(epicBacklog), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Branch != "feat/2.0.0" || result.URL != "https://example.com/pull/9" || !result.Draft {
		t.Fatalf("result = %+v", result)
	}
	if _, err := git.Run(root, "ls-remote", "--exit-code", "--heads", "origin", "refs/heads/feat/2.0.0"); err != nil {
		t.Fatalf("the epic branch never reached origin: %v", err)
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
			return `[{"name":"status: wip"}]`, nil
		default:
			return "", nil
		}
	}}
	plan := &Plan{Group: "TG-05.1", Version: "2.0.0"}
	result, err := openEpic(root, backlog.Parse(epicBacklog), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft {
		t.Fatalf("draft = true, want a normal pull request once drafts are refused")
	}
	if result.URL != "https://example.com/pull/9" {
		t.Fatalf("url = %q", result.URL)
	}
	if len(result.Labels) != 1 || result.Labels[0] != "status: wip" {
		t.Fatalf("labels = %v, want status: wip", result.Labels)
	}
}

func TestOpenEpicLeavesABranchAlreadyOnOriginAlone(t *testing.T) {
	root := epicRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run once the epic branch already lives on origin: %v", args)
		return "", nil
	}}
	plan := &Plan{Group: "TG-05.1", Version: "2.0.0"}
	result, err := openEpic(root, backlog.Parse(epicBacklog), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil once the branch already exists", result)
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
	result, err := openEpic(root, backlog.Parse(epicBacklog), plan, client)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil with no version", result)
	}
}

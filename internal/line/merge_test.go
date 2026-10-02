package line

import (
	"strings"
	"testing"

	"komodo/internal/ledger"
	"komodo/internal/pr"
)

// mergeRepo builds a group whose review is clean and whose ship already landed, ready to merge.
func mergeRepo(t *testing.T) (root, group string) {
	t.Helper()
	root, group = shipRepo(t)
	state := RunState{Run: "TG-09.1-1", Group: "TG-09.1", Base: "feat/v2.0.0", Branch: "feat/a-group"}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	Stamp(root, ledger.Entry{Group: "TG-09.1", Station: "ship", Outcome: "done"})
	return root, group
}

// mergePlan is the plan mergeRepo's group matches, targeting its epic branch.
func mergePlan() *Plan {
	return &Plan{
		Group: "TG-09.1", Branch: "feat/a-group", Base: "feat/v2.0.0", Worktree: "group",
	}
}

func TestMergeGroupMergesAReviewedCheckedGroupWithAMergeCommit(t *testing.T) {
	root, _ := mergeRepo(t)
	var calls []string
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		if strings.HasPrefix(joined, "pr view") {
			return `{"number":7,"url":"https://example.com/pull/7","state":"OPEN"}`, nil
		}
		return "", nil
	}}
	result, err := MergeGroup(root, mergePlan(), client)
	if err != nil {
		t.Fatal(err)
	}
	if result.Number != 7 || result.Base != "feat/v2.0.0" || result.URL != "https://example.com/pull/7" {
		t.Fatalf("result = %+v", result)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
	if !strings.Contains(calls[1], "pr merge 7") || !strings.Contains(calls[1], "--merge") {
		t.Fatalf("call %q must merge with a merge commit", calls[1])
	}
}

// TestMergeGroupMergesAGroupOntoAnEpicBranch proves the conductor's own merge is not refused by
// decision 0006, which refuses only a model session's push or merge onto an epic branch.
func TestMergeGroupMergesAGroupOntoAnEpicBranch(t *testing.T) {
	root, _ := mergeRepo(t)
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "pr view") {
			return `{"number":7,"url":"https://example.com/pull/7","state":"OPEN"}`, nil
		}
		return "", nil
	}}
	plan := mergePlan()
	plan.Base = "feat/1.0.0"
	if _, err := MergeGroup(root, plan, client); err != nil {
		t.Fatalf("err = %v, want the conductor's merge onto an epic branch allowed", err)
	}
}

func TestMergeGroupRefusesTheCriticalRefMain(t *testing.T) {
	root, _ := mergeRepo(t)
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run once the base is a critical ref: %v", args)
		return "", nil
	}}
	plan := mergePlan()
	plan.Base = "main"
	_, err := MergeGroup(root, plan, client)
	if err == nil || !strings.Contains(err.Error(), "critical ref") {
		t.Fatalf("err = %v, want a critical ref refusal", err)
	}
}

func TestMergeGroupRefusesAGroupThatHasNotShipped(t *testing.T) {
	root, _ := shipRepo(t)
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run before the group has shipped: %v", args)
		return "", nil
	}}
	_, err := MergeGroup(root, mergePlan(), client)
	if err == nil || !strings.Contains(err.Error(), "has not shipped") {
		t.Fatalf("err = %v, want a not-shipped refusal", err)
	}
}

func TestMergeGroupRefusesAFindingAtOrAboveTheFloor(t *testing.T) {
	root, _ := mergeRepo(t)
	saveReview(t, root, "TG-09.1", `{"findings":[{"severity":"high","title":"leak"}]}`)
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run with an unfixed finding: %v", args)
		return "", nil
	}}
	plan := mergePlan()
	plan.Profile.SeverityFloor = "high"
	_, err := MergeGroup(root, plan, client)
	if err == nil || !strings.Contains(err.Error(), "the review left") {
		t.Fatalf("err = %v, want a review refusal", err)
	}
}

func TestMergeGroupRefusesAFailedCheck(t *testing.T) {
	root, _ := mergeRepo(t)
	Stamp(root, ledger.Entry{Group: "TG-09.1", Station: "qc", Wave: 1})
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run with a failed check: %v", args)
		return "", nil
	}}
	_, err := MergeGroup(root, mergePlan(), client)
	if err == nil || !strings.Contains(err.Error(), "failed check") {
		t.Fatalf("err = %v, want a failed-check refusal", err)
	}
}

func TestMergeGroupRefusesAPullThatIsAlreadyMerged(t *testing.T) {
	root, _ := mergeRepo(t)
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		return `{"number":7,"url":"https://example.com/pull/7","state":"MERGED"}`, nil
	}}
	_, err := MergeGroup(root, mergePlan(), client)
	if err == nil || !strings.Contains(err.Error(), "not open") {
		t.Fatalf("err = %v, want a not-open refusal", err)
	}
}

func TestMergeGroupRefusesWithNoClient(t *testing.T) {
	root, _ := mergeRepo(t)
	_, err := MergeGroup(root, mergePlan(), nil)
	if err == nil || !strings.Contains(err.Error(), "no pull request client") {
		t.Fatalf("err = %v, want a no-client refusal", err)
	}
}

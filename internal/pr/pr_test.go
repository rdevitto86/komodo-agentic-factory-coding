package pr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fake records the gh invocations a client makes and returns outputs in call order,
// repeating the last one once the list runs out.
func fake(t *testing.T, outputs ...string) (*Client, *[]string) {
	t.Helper()
	var calls []string
	call := 0
	client := &Client{Dir: ".", Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		out := outputs[len(outputs)-1]
		if call < len(outputs) {
			out = outputs[call]
		}
		call++
		return out, nil
	}}
	return client, &calls
}

// fakeErr returns a runner that fails every call, so tests can drive an API error.
func fakeErr(err error) (*Client, *[]string) {
	var calls []string
	client := &Client{Dir: ".", Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", err
	}}
	return client, &calls
}

// pullFixture is the pr-view JSON Threads reads to learn the pull request's URL.
const pullFixture = `{"number":7,"url":"https://github.com/o/r/pull/7","state":"OPEN","title":"T","isDraft":false}`

// draftFixture is the pr-view JSON for a pull request opened as a draft.
const draftFixture = `{"number":9,"url":"https://github.com/o/r/pull/9","state":"OPEN","title":"T","isDraft":true}`

func TestCreatePassesBaseHeadAndDraft(t *testing.T) {
	client, calls := fake(t, "https://example.com/pull/1")
	url, err := client.Create("main", "feat/x", "Title", "Body", true)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://example.com/pull/1" {
		t.Fatalf("url = %s", url)
	}
	got := (*calls)[0]
	for _, want := range []string{"pr create", "--base main", "--head feat/x", "--draft"} {
		if !strings.Contains(got, want) {
			t.Errorf("call %q is missing %q", got, want)
		}
	}
}

func TestViewParsesTheFieldsTheLineReads(t *testing.T) {
	client, _ := fake(t, `{"number":7,"url":"u","state":"OPEN","title":"T","isDraft":true}`)
	pull, err := client.View("7")
	if err != nil {
		t.Fatal(err)
	}
	if pull.Number != 7 || !pull.Draft || pull.State != "OPEN" {
		t.Fatalf("pull = %+v", pull)
	}
}

func TestViewOfADraftPull(t *testing.T) {
	client, _ := fake(t, draftFixture)
	pull, err := client.View("9")
	if err != nil {
		t.Fatal(err)
	}
	if !pull.Draft {
		t.Fatalf("pull = %+v, want a draft", pull)
	}
}

func TestViewOmitsTheNumberForTheCurrentBranch(t *testing.T) {
	client, calls := fake(t, pullFixture)
	if _, err := client.View(""); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	if strings.Contains(got, "7") {
		t.Fatalf("call %q should not name a pull request", got)
	}
	if !strings.Contains(got, "pr view --json") {
		t.Fatalf("call %q is missing pr view --json", got)
	}
}

func TestCreateReturnsAnAPIError(t *testing.T) {
	client, _ := fakeErr(errors.New("rate limited"))
	if _, err := client.Create("main", "feat/x", "T", "B", false); err == nil {
		t.Fatal("want an error")
	}
}

func TestLabelsListsWhatTheRepoDefines(t *testing.T) {
	client, _ := fake(t, `[{"name":"feat"},{"name":"agent"}]`)
	got, err := client.Labels()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "feat,agent" {
		t.Fatalf("labels = %v", got)
	}
}

func TestKeepKnownDropsLabelsTheRepoLacks(t *testing.T) {
	got := KeepKnown([]string{"feat", "unknown"}, []string{"feat", "agent"})
	if len(got) != 1 || got[0] != "feat" {
		t.Fatalf("kept = %v", got)
	}
}

func TestKeepKnownMatchesByNameBeforeTheFirstSpace(t *testing.T) {
	known := []string{"@agent 🤖", "scope/guard 🛡️", "scope/harness ⚙️"}
	got := KeepKnown([]string{"@agent", "scope/guard"}, known)
	if strings.Join(got, ",") != "@agent 🤖,scope/guard 🛡️" {
		t.Fatalf("kept = %v", got)
	}
}

func TestLabelsReturnsAnAPIError(t *testing.T) {
	client, _ := fakeErr(errors.New("not found"))
	if _, err := client.Labels(); err == nil {
		t.Fatal("want an error")
	}
}

func TestLabelAddsOnlyLabelsThatExist(t *testing.T) {
	client, calls := fake(t, "")
	kept := KeepKnown([]string{"feat", "ghost"}, []string{"feat", "agent"})
	if err := client.Label("7", kept); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[len(*calls)-1]
	for _, want := range []string{"pr edit 7", "--add-label feat"} {
		if !strings.Contains(got, want) {
			t.Errorf("call %q is missing %q", got, want)
		}
	}
	if strings.Contains(got, "ghost") {
		t.Fatalf("call %q must not add a label the repo lacks", got)
	}
}

func TestLabelIsANoOpWithoutLabels(t *testing.T) {
	client, calls := fake(t, "")
	if err := client.Label("7", nil); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Fatalf("calls = %v", *calls)
	}
}

func TestThreadsReadsPathAndLine(t *testing.T) {
	graphql := `{"data":{"resource":{"reviewThreads":{"nodes":[` +
		`{"id":"PRT_1","isResolved":false,"path":"a.go","line":4,` +
		`"comments":{"nodes":[{"body":"fix","author":{"login":"rev"}}]}}]}}}}`
	client, calls := fake(t, pullFixture, graphql)
	threads, err := client.Threads("7")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].ID != "PRT_1" || threads[0].Path != "a.go" || threads[0].Line != 4 || threads[0].Author != "rev" {
		t.Fatalf("threads = %+v", threads)
	}
	if !strings.Contains((*calls)[1], "reviewThreads") {
		t.Fatalf("call %q did not query reviewThreads", (*calls)[1])
	}
}

func TestThreadsReturnsAnAPIError(t *testing.T) {
	client, _ := fakeErr(errors.New("timeout"))
	if _, err := client.Threads("7"); err == nil {
		t.Fatal("want an error")
	}
}

func TestThreadsDropsResolvedThreads(t *testing.T) {
	graphql := `{"data":{"resource":{"reviewThreads":{"nodes":[` +
		`{"id":"PRT_1","isResolved":true,"path":"a.go","line":4,` +
		`"comments":{"nodes":[{"body":"fix","author":{"login":"rev"}}]}},` +
		`{"id":"PRT_2","isResolved":false,"path":"b.go","line":9,` +
		`"comments":{"nodes":[{"body":"still open","author":{"login":"rev"}}]}}` +
		`]}}}}`
	client, _ := fake(t, pullFixture, graphql)
	threads, err := client.Threads("7")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].ID != "PRT_2" {
		t.Fatalf("threads = %+v; a resolved thread must not come back", threads)
	}
}

func TestReplyPostsOnTheThreadItself(t *testing.T) {
	client, calls := fake(t, `{"data":{"addPullRequestReviewThreadReply":{"comment":{"id":"PRRC_1"}}}}`)
	if err := client.Reply("PRT_1", "fixed in abc123"); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	for _, want := range []string{"addPullRequestReviewThreadReply", "id=PRT_1", "body=fixed in abc123"} {
		if !strings.Contains(got, want) {
			t.Errorf("call %q is missing %q", got, want)
		}
	}
}

func TestResolveMarksTheThreadItself(t *testing.T) {
	client, calls := fake(t, `{"data":{"resolveReviewThread":{"thread":{"id":"PRT_1"}}}}`)
	if err := client.Resolve("PRT_1"); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	for _, want := range []string{"resolveReviewThread", "id=PRT_1"} {
		if !strings.Contains(got, want) {
			t.Errorf("call %q is missing %q", got, want)
		}
	}
}

// TestRespondLoopTerminates proves a reply followed by a Resolve call drops the
// thread out of the next Threads call, so the respond loop ends.
func TestRespondLoopTerminates(t *testing.T) {
	open := `{"data":{"resource":{"reviewThreads":{"nodes":[` +
		`{"id":"PRT_1","isResolved":false,"path":"a.go","line":4,` +
		`"comments":{"nodes":[{"body":"fix","author":{"login":"rev"}}]}}]}}}}`
	resolved := `{"data":{"resource":{"reviewThreads":{"nodes":[` +
		`{"id":"PRT_1","isResolved":true,"path":"a.go","line":4,` +
		`"comments":{"nodes":[{"body":"fix","author":{"login":"rev"}}]}}]}}}}`
	replyOut := `{"data":{"addPullRequestReviewThreadReply":{"comment":{"id":"PRRC_1"}}}}`
	resolveOut := `{"data":{"resolveReviewThread":{"thread":{"id":"PRT_1"}}}}`
	client, _ := fake(t, pullFixture, open, replyOut, resolveOut, pullFixture, resolved)

	first, err := client.Threads("7")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("first threads = %+v, want one unresolved thread", first)
	}
	if err := client.Reply(first[0].ID, "fixed in abc123"); err != nil {
		t.Fatal(err)
	}
	if err := client.Resolve(first[0].ID); err != nil {
		t.Fatal(err)
	}
	second, err := client.Threads("7")
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second threads = %+v, want none left once the thread is resolved", second)
	}
}

func TestEditPassesTheGivenArgs(t *testing.T) {
	client, calls := fake(t, "")
	if err := client.Edit("7", "--title", "New title"); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	for _, want := range []string{"pr edit 7", "--title", "New title"} {
		if !strings.Contains(got, want) {
			t.Errorf("call %q is missing %q", got, want)
		}
	}
}

func TestCommentPostsTheBody(t *testing.T) {
	client, calls := fake(t, "")
	if err := client.Comment("7", "looks good"); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	for _, want := range []string{"pr comment 7", "--body", "looks good"} {
		if !strings.Contains(got, want) {
			t.Errorf("call %q is missing %q", got, want)
		}
	}
}

// scriptGh writes an executable gh stand-in to a temp dir and puts it on PATH.
func scriptGh(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestNewRunsTheRealGhOnPath(t *testing.T) {
	scriptGh(t, "echo \"$@\"\n")
	client := New(".")
	out, err := client.Run(".", "pr", "list")
	if err != nil {
		t.Fatal(err)
	}
	if out != "pr list" {
		t.Fatalf("out = %q", out)
	}
}

func TestRunWrapsAFailingGh(t *testing.T) {
	scriptGh(t, "echo boom >&2\nexit 1\n")
	if _, err := Run(".", "pr", "list"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

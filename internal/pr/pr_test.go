package pr

import (
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

// pullFixture is the pr-view JSON Threads reads to learn the pull request's URL.
const pullFixture = `{"number":7,"url":"https://github.com/o/r/pull/7","state":"OPEN","title":"T","isDraft":false}`

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

// TestRespondLoopTerminates proves a resolved thread drops out of the next Threads
// call, so a reply followed by a resolve ends the respond loop.
func TestRespondLoopTerminates(t *testing.T) {
	open := `{"data":{"resource":{"reviewThreads":{"nodes":[` +
		`{"id":"PRT_1","isResolved":false,"path":"a.go","line":4,` +
		`"comments":{"nodes":[{"body":"fix","author":{"login":"rev"}}]}}]}}}}`
	resolved := `{"data":{"resource":{"reviewThreads":{"nodes":[` +
		`{"id":"PRT_1","isResolved":true,"path":"a.go","line":4,` +
		`"comments":{"nodes":[{"body":"fix","author":{"login":"rev"}}]}}]}}}}`
	replyOut := `{"data":{"addPullRequestReviewThreadReply":{"comment":{"id":"PRRC_1"}}}}`
	client, _ := fake(t, pullFixture, open, replyOut, pullFixture, resolved)

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
	second, err := client.Threads("7")
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second threads = %+v, want none left once the thread is resolved", second)
	}
}

package pr

import (
	"strings"
	"testing"
)

// fake records the gh invocations a client makes and returns canned output.
func fake(t *testing.T, output string) (*Client, *[]string) {
	t.Helper()
	var calls []string
	client := &Client{Dir: ".", Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return output, nil
	}}
	return client, &calls
}

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
	client, _ := fake(t, `{"comments":[{"path":"a.go","line":4,"body":"fix","author":{"login":"rev"}}]}`)
	threads, err := client.Threads("7")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].Path != "a.go" || threads[0].Line != 4 || threads[0].Author != "rev" {
		t.Fatalf("threads = %+v", threads)
	}
}

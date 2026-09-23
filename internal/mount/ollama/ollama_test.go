package ollama

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAllowedAcceptsReadOnlyTools(t *testing.T) {
	if !Allowed([]string{"read", "search"}) {
		t.Fatal("read and search should be allowed against Ollama")
	}
}

func TestAllowedRejectsAWriteTool(t *testing.T) {
	for _, tool := range []string{"write", "edit", "shell"} {
		if Allowed([]string{"read", tool}) {
			t.Fatalf("%s should never be allowed against Ollama", tool)
		}
	}
}

// fakeOllama answers one chat call with the message content a test supplies.
func fakeOllama(t *testing.T, content string, promptTokens, evalTokens int) (*httptest.Server, *request) {
	t.Helper()
	var got request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		reply := reply{
			Message:         message{Role: "assistant", Content: content},
			PromptEvalCount: promptTokens,
			EvalCount:       evalTokens,
		}
		body, _ := json.Marshal(reply)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	return server, &got
}

func TestPostSendsTheModelBriefAndSchema(t *testing.T) {
	server, got := fakeOllama(t, `{"result":"DONE"}`, 12, 34)
	defer server.Close()
	schema := []byte(`{"type":"object"}`)
	result, err := Post(server.URL, "llama3", "review this diff", schema)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "llama3" || len(got.Messages) != 1 || got.Messages[0].Content != "review this diff" {
		t.Fatalf("request = %+v", got)
	}
	if string(got.Format) != string(schema) {
		t.Fatalf("format = %s, want the role's schema", got.Format)
	}
	if result.Value["result"] != "DONE" {
		t.Fatalf("value = %v", result.Value)
	}
	if result.TokensIn != 12 || result.TokensOut != 34 {
		t.Fatalf("tokens = %d/%d", result.TokensIn, result.TokensOut)
	}
}

func TestPostNamesTheURLWhenOllamaIsDown(t *testing.T) {
	_, err := Post("http://127.0.0.1:1", "llama3", "brief", []byte(`{}`))
	if err == nil {
		t.Fatal("want an error when nothing answers")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("error %q does not name the URL", err.Error())
	}
}

func TestPostRejectsAnAnswerThatIsNotJSON(t *testing.T) {
	server, _ := fakeOllama(t, "not json", 1, 1)
	defer server.Close()
	if _, err := Post(server.URL, "llama3", "brief", []byte(`{}`)); err == nil {
		t.Fatal("want an error when the answer does not fit the schema")
	}
}

func TestPostRejectsANonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()
	if _, err := Post(server.URL, "llama3", "brief", []byte(`{}`)); err == nil {
		t.Fatal("want an error on a non-200 status")
	}
}

func TestBaseURLDefaultsToLocalhost(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "")
	if got := BaseURL(); got != "http://localhost:11434" {
		t.Fatalf("base url = %s", got)
	}
}

func TestBaseURLReadsTheEnvironment(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "http://example.local:9999/")
	if got := BaseURL(); got != "http://example.local:9999" {
		t.Fatalf("base url = %s", got)
	}
}

func TestPostSendsAContextSizedToTheBrief(t *testing.T) {
	brief := strings.Repeat("a", 10000)
	server, got := fakeOllama(t, `{"result":"DONE"}`, 12, 34)
	defer server.Close()
	if _, err := Post(server.URL, "llama3", brief, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if want := contextSize(brief); got.Options.NumCtx != want {
		t.Fatalf("num_ctx = %d, want %d", got.Options.NumCtx, want)
	}
	if got.Options.NumCtx <= minContext {
		t.Fatal("a brief longer than the default did not raise the context window")
	}
}

func TestPostFailsWhenThePromptCountShowsTruncation(t *testing.T) {
	brief := "a short brief"
	window := contextSize(brief)
	server, _ := fakeOllama(t, `{"result":"DONE"}`, window, 5)
	defer server.Close()
	_, err := Post(server.URL, "llama3", brief, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("err = %v, want a truncation error", err)
	}
}

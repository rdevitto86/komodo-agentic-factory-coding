package ollama

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// clearEnv unsets everything ModelName and BaseURL might read from the process.
func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv(ModelEnv, "")
	t.Setenv(Env, "")
	t.Setenv(WindowEnv, "")
	t.Setenv("HOME", t.TempDir())
	tagOnce = sync.Once{}
	tagFirst = ""
}

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
	t.Setenv("HOME", t.TempDir())
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

func TestBaseURLReadsTheOverlayUnderTheEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OLLAMA_BASE_URL", "")
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"local_url": "http://10.0.0.5:11434/"}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BaseURL(); got != "http://10.0.0.5:11434" {
		t.Fatalf("base url = %s; the overlay's local_url must apply", got)
	}
	t.Setenv("OLLAMA_BASE_URL", "http://example.local:9999")
	if got := BaseURL(); got != "http://example.local:9999" {
		t.Fatalf("base url = %s; the environment must win over the overlay", got)
	}
}

func TestDialAddressDefaultsThePortByScheme(t *testing.T) {
	cases := map[string]string{
		"http://10.0.0.5":        "10.0.0.5:80",
		"https://ollama.example": "ollama.example:443",
		"http://10.0.0.5:11434/": "10.0.0.5:11434",
	}
	for endpoint, want := range cases {
		if got, ok := dialAddress(endpoint); !ok || got != want {
			t.Errorf("%s: got %q, want %q", endpoint, got, want)
		}
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if !dialable(server.URL) {
		t.Fatalf("%s did not dial", server.URL)
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

// writeOverlay puts a config.json under home so BaseURL, ModelName and Window read it.
func writeOverlay(t *testing.T, home, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".komodo", "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestModelNameDefaultsWhenNothingNamesOne(t *testing.T) {
	clearEnv(t)
	// Point at a closed port so firstTag's Up check fails without reaching a real server.
	t.Setenv(Env, "http://127.0.0.1:1")
	if got := ModelName(); got != DefaultModel {
		t.Fatalf("model = %s, want the default %s", got, DefaultModel)
	}
}

func TestModelNameReadsTheServerListBeforeTheDefault(t *testing.T) {
	clearEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"models":[{"name":"llama4"}]}`))
	}))
	defer server.Close()
	t.Setenv(Env, server.URL)
	if got := ModelName(); got != "llama4" {
		t.Fatalf("model = %s, want the server's first tag", got)
	}
}

func TestModelNameReadsTheOverlayBeforeTheServer(t *testing.T) {
	clearEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeOverlay(t, home, `{"local_model": "overlay-model"}`)
	if got := ModelName(); got != "overlay-model" {
		t.Fatalf("model = %s, want the overlay's local_model", got)
	}
}

func TestModelNameReadsTheEnvironmentBeforeTheOverlay(t *testing.T) {
	clearEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeOverlay(t, home, `{"local_model": "overlay-model"}`)
	t.Setenv(ModelEnv, "env-model")
	if got := ModelName(); got != "env-model" {
		t.Fatalf("model = %s, want the environment's model", got)
	}
}

func TestFitsAndTheWindowOverride(t *testing.T) {
	clearEnv(t)
	if !Fits(100) {
		t.Fatal("a small brief should fit the default window")
	}
	if Fits(DefaultWindow * 10) {
		t.Fatal("a brief far larger than the default window should not fit")
	}
	t.Setenv(WindowEnv, "1500")
	if Fits(2000) {
		t.Fatal("a brief that exceeds the overridden window should not fit")
	}
}

func TestDialAddressWithoutAHost(t *testing.T) {
	got, ok := dialAddress("not-a-url")
	if !ok || got != "not-a-url" {
		t.Fatalf("got = %q, %v; want the raw endpoint through unchanged", got, ok)
	}
}

func TestDialAddressRejectsAnUnparsableEndpoint(t *testing.T) {
	if _, ok := dialAddress("http://%zz"); ok {
		t.Fatal("want dialAddress to reject an endpoint url.Parse cannot read")
	}
}

func TestUpAgainstAClosedListener(t *testing.T) {
	clearEnv(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	t.Setenv(Env, "http://"+address)
	if Up() {
		t.Fatal("Up should be false once the listener is closed")
	}
}

func TestUpAgainstAnOpenListener(t *testing.T) {
	clearEnv(t)
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	t.Setenv(Env, server.URL)
	if !Up() {
		t.Fatal("Up should be true against an open listener")
	}
}

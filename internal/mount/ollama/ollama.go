// Package ollama is the binary's own mount: it posts a brief straight to a local Ollama server.
package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"komodo/internal/mount"
)

// Env names the environment variable that moves the local machine's endpoint.
const Env = "OLLAMA_BASE_URL"

// DefaultURL is where the local machine answers unless the environment says otherwise.
const DefaultURL = "http://localhost:11434"

// DefaultModel is the model the mount falls back to when nothing names one and the server lists none.
const DefaultModel = "llama3.2"

// ModelEnv names the environment variable that picks the local model over the overlay and the server.
const ModelEnv = "OLLAMA_MODEL"

// WindowEnv names the environment variable that caps the local context window, in tokens.
const WindowEnv = "OLLAMA_NUM_CTX"

// DefaultWindow is the largest brief the mount sends a local model without being told otherwise.
const DefaultWindow = 32768

// minContext is Ollama's own default context window, in tokens, and the floor this mount requests.
const minContext = 2048

// init registers this mount and its names, so the doctor catches either outside the mounts.
func init() {
	mount.Register(mount.Host{Name: "ollama", Vendors: []string{"ollama", DefaultModel}})
	mount.RegisterLocal(mount.Local{
		Env: Env, Up: Up, ModelName: ModelName, Fits: Fits, Allowed: Allowed,
		Post: func(model, brief string, schema []byte) (mount.LocalResult, error) {
			result, err := Post(BaseURL(), model, brief, schema)
			return mount.LocalResult{Value: result.Value, TokensIn: result.TokensIn, TokensOut: result.TokensOut}, err
		},
	})
}

// ModelName is the local model every mount uses: the environment, then the overlay, then the
// first model the server lists, then the default, so both hosts always agree.
func ModelName() string {
	if name := os.Getenv(ModelEnv); name != "" {
		return name
	}
	if name := mount.LoadOverlay().LocalModel; name != "" {
		return name
	}
	if name := firstTag(); name != "" {
		return name
	}
	return DefaultModel
}

// Window is the largest context, in tokens, the mount asks a local model for.
func Window() int {
	if raw := os.Getenv(WindowEnv); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	if window := mount.LoadOverlay().LocalWindow; window > 0 {
		return window
	}
	return DefaultWindow
}

// Fits reports whether a brief of this many characters fits the local window with room to answer.
func Fits(chars int) bool {
	return chars/4+1024 <= Window()
}

var (
	tagOnce  sync.Once
	tagFirst string
)

// firstTag asks the server once per process for the first model it holds, or nothing when it cannot say.
func firstTag() string {
	tagOnce.Do(func() {
		if !Up() {
			return
		}
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get(BaseURL() + "/api/tags")
		if err != nil {
			return
		}
		defer response.Body.Close()
		var listed struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.NewDecoder(response.Body).Decode(&listed) == nil && len(listed.Models) > 0 {
			tagFirst = listed.Models[0].Name
		}
	})
	return tagFirst
}

// writeTools are the verbs that make a role's machine call unsafe to run against Ollama.
var writeTools = map[string]bool{"write": true, "edit": true, "shell": true}

// Allowed reports whether a role's tools hold no write or shell, which is what Ollama may carry.
func Allowed(tools []string) bool {
	for _, tool := range tools {
		if writeTools[tool] {
			return false
		}
	}
	return true
}

// BaseURL is where Ollama answers, from the environment or the default.
func BaseURL() string {
	base := os.Getenv(Env)
	if base == "" {
		base = DefaultURL
	}
	return strings.TrimRight(base, "/")
}

// Up reports whether the local machine answers where the environment says it lives.
func Up() bool {
	endpoint := os.Getenv(Env)
	if endpoint == "" {
		endpoint = DefaultURL
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := parsed.Host
	if host == "" {
		host = endpoint
	}
	connection, err := net.DialTimeout("tcp", host, 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = connection.Close()
	return true
}

// contextSize sizes the request window to the brief, at roughly four characters per token,
// with headroom for the reply, floored at Ollama's own default.
func contextSize(brief string) int {
	size := len(brief)/4 + 1024
	if size < minContext {
		return minContext
	}
	return size
}

// message is one turn in the chat the mount sends or reads back.
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// request is one call to the chat endpoint: the model, the brief, and the schema the answer must fit.
type request struct {
	Model    string          `json:"model"`
	Messages []message       `json:"messages"`
	Format   json.RawMessage `json:"format,omitempty"`
	Options  options         `json:"options,omitempty"`
	Stream   bool            `json:"stream"`
}

// options carries the context window size, so a long brief is not silently cut down to the default.
type options struct {
	NumCtx int `json:"num_ctx"`
}

// reply is what the chat endpoint answers, trimmed to what the mount reads.
type reply struct {
	Message         message `json:"message"`
	PromptEvalCount int     `json:"prompt_eval_count"`
	EvalCount       int     `json:"eval_count"`
}

// Result is one call's answer: the parsed JSON the schema described, and the tokens it cost.
type Result struct {
	Value     map[string]any
	TokensIn  int
	TokensOut int
}

// Post sends one brief to the chat endpoint and parses the answer against the schema.
func Post(base, model, brief string, schema []byte) (Result, error) {
	window := contextSize(brief)
	if window > Window() {
		return Result{}, fmt.Errorf("the brief needs a %d-token window and the local machine allows %d; route it to a remote machine or raise %s", window, Window(), WindowEnv)
	}
	body, err := json.Marshal(request{
		Model:    model,
		Messages: []message{{Role: "user", Content: brief}},
		Format:   json.RawMessage(schema),
		Options:  options{NumCtx: window},
	})
	if err != nil {
		return Result{}, err
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	response, err := client.Post(base+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("ollama is not answering at %s: %w", base, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return Result{}, fmt.Errorf("ollama at %s: %w", base, err)
	}
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("ollama at %s answered %d: %s", base, response.StatusCode, strings.TrimSpace(string(data)))
	}
	var parsed reply
	if err := json.Unmarshal(data, &parsed); err != nil {
		return Result{}, fmt.Errorf("ollama at %s did not answer JSON: %w", base, err)
	}
	if parsed.PromptEvalCount >= window {
		return Result{}, fmt.Errorf("ollama at %s truncated the brief: the prompt used %d tokens against a %d context", base, parsed.PromptEvalCount, window)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(parsed.Message.Content), &value); err != nil {
		return Result{}, fmt.Errorf("ollama's answer does not fit the schema: %w", err)
	}
	return Result{Value: value, TokensIn: parsed.PromptEvalCount, TokensOut: parsed.EvalCount}, nil
}

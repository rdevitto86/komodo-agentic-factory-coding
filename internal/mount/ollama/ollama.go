// Package ollama is the binary's own mount: it posts a brief straight to a local Ollama server.
package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"komodo/internal/profile"
)

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
	base := os.Getenv(profile.OllamaEnv)
	if base == "" {
		base = profile.DefaultOllamaURL
	}
	return strings.TrimRight(base, "/")
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
	Stream   bool            `json:"stream"`
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
	body, err := json.Marshal(request{
		Model:    model,
		Messages: []message{{Role: "user", Content: brief}},
		Format:   json.RawMessage(schema),
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
	var value map[string]any
	if err := json.Unmarshal([]byte(parsed.Message.Content), &value); err != nil {
		return Result{}, fmt.Errorf("ollama's answer does not fit the schema: %w", err)
	}
	return Result{Value: value, TokensIn: parsed.PromptEvalCount, TokensOut: parsed.EvalCount}, nil
}

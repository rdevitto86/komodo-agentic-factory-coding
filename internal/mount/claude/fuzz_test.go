package claude

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzGlobalSettings proves merging a user's global settings.json never panics on any input,
// well formed, malformed, or carrying komodo's own hooks under a shape it does not expect.
func FuzzGlobalSettings(f *testing.F) {
	for _, seed := range []string{
		"",
		"{}",
		"not json",
		"[]",
		`{"hooks":{}}`,
		`{"hooks":"oops"}`,
		`{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"/bin/komodo guard"}]}]}}`,
		`{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"command":"other"}]}],"SessionStart":"oops"}}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		path := filepath.Join(t.TempDir(), "settings.json")
		if body != "" {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, _ = globalSettings(path, "komodo")
	})
}

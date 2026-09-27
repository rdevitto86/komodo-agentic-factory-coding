package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goModule writes a one-file module and returns the directory and the file's path.
func goModule(t *testing.T, source string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func TestFormatRewritesTheEditedGoFile(t *testing.T) {
	t.Parallel()
	dir, path := goModule(t, "package main\nfunc main()   {\n}\n")
	got := dispatch(t, dir, "format", map[string]any{
		"session_id": "fmt", "cwd": dir, "tool_name": "Write", "tool_input": map[string]any{"file_path": path},
	})
	if got.code != 0 || got.stdout != "" {
		t.Fatalf("a clean file returned %+v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package main\n\nfunc main() {\n}\n" {
		t.Fatalf("file not formatted: %q", data)
	}
}

func TestFormatReturnsLintAsContextAndNeverRefuses(t *testing.T) {
	t.Parallel()
	dir, path := goModule(t, "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%d\\n\", \"x\")\n}\n")
	got := dispatch(t, dir, "format", map[string]any{
		"session_id": "lint", "cwd": dir, "tool_name": "Write", "tool_input": map[string]any{"file_path": "main.go"},
	})
	if got.code != 0 {
		t.Fatalf("format refused: %+v", got)
	}
	if !strings.Contains(got.stdout, "main.go:") || !strings.Contains(got.stdout, "Printf") {
		t.Fatalf("lint not returned as context: %+v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestFormatSkipsCallsThatEditNoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name   string
		fields map[string]any
	}{
		{"not a write tool", map[string]any{"cwd": dir, "tool_name": "Read", "tool_input": map[string]any{"file_path": "a.go"}}},
		{"missing file", map[string]any{"cwd": dir, "tool_name": "Write", "tool_input": map[string]any{"file_path": "gone.go"}}},
		{"another language", map[string]any{"cwd": dir, "tool_name": "Write", "tool_input": map[string]any{"file_path": "go.mod"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := dispatch(t, dir, "format", tc.fields)
			if got.code != 0 || got.stdout != "" || got.stderr != "" {
				t.Fatalf("got %+v, want a silent allow", got)
			}
		})
	}
}

// nodeRoot writes a worktree whose installed formatter and linter are the given scripts, empty for none.
func nodeRoot(t *testing.T, prettier, eslint string) (string, string) {
	t.Helper()
	root := t.TempDir()
	// Its own .git stops the worktree walk here; a sandbox's temp dir sits inside the real worktree.
	bin := filepath.Join(root, nodeBin)
	for _, dir := range []string{filepath.Join(root, ".git"), bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, script := range map[string]string{"prettier": prettier, "eslint": eslint} {
		if script == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "app.ts")
	if err := os.WriteFile(path, []byte("let a=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func TestFormatRunsTheReposOwnTypeScriptTools(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		prettier  string
		eslint    string
		formatted bool
		lint      string
	}{
		{"formats and returns lint", `echo "let a = 1;" > "$2"`, `echo "no-unused-vars"; exit 1`, true, "no-unused-vars"},
		{"clean lint stays quiet", `echo "let a = 1;" > "$2"`, "exit 0", true, ""},
		{"no linter installed", `echo "let a = 1;" > "$2"`, "", true, ""},
		{"no tools installed", "", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, path := nodeRoot(t, tc.prettier, tc.eslint)
			got := dispatch(t, root, "format", map[string]any{
				"session_id": "ts", "cwd": root, "tool_name": "Write", "tool_input": map[string]any{"file_path": path},
			})
			if got.code != 0 {
				t.Fatalf("format refused: %+v", got)
			}
			if tc.lint == "" && got.stdout != "" || !strings.Contains(got.stdout, tc.lint) {
				t.Fatalf("stdout = %q, want lint %q", got.stdout, tc.lint)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if formatted := string(data) == "let a = 1;\n"; formatted != tc.formatted {
				t.Fatalf("file = %q, want formatted %v", data, tc.formatted)
			}
		})
	}
}

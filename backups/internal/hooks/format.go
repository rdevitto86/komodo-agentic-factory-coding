package hooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/mount"
	"komodo/internal/proc"
)

// nodeBin is where a repo's installed formatter and linter for TypeScript live.
var nodeBin = filepath.Join("node_modules", ".bin")

// typeScriptExts are the extensions the repo's TypeScript formatter and linter take.
var typeScriptExts = map[string]bool{".ts": true, ".tsx": true}

// formatFile formats the file a builder just edited and returns lint on that file alone as context.
func formatFile(ctx context.Context, in Input) (Outcome, error) {
	path := editedPath(in)
	if path == "" {
		return Outcome{Verdict: Allow}, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(in.Cwd, path)
	}
	if _, err := os.Stat(path); err != nil {
		return Outcome{Verdict: Allow}, nil
	}

	var lint string
	switch ext := filepath.Ext(path); {
	case ext == ".go":
		lint = formatGo(ctx, path)
	case typeScriptExts[ext]:
		lint = formatTypeScript(ctx, in.Root, path)
	}
	if lint == "" {
		return Outcome{Verdict: Allow}, nil
	}
	return Outcome{Verdict: Inform, Message: "Lint on " + filepath.Base(path) + ":\n" + lint}, nil
}

// editedPath is the file a registered host's write tool names in this call, or the empty string.
func editedPath(in Input) string {
	for _, tools := range mount.GuardHosts() {
		if !tools.WriteTools[in.ToolName] {
			continue
		}
		for _, field := range tools.PathFields {
			if path, ok := in.ToolInput[field].(string); ok && path != "" {
				return path
			}
		}
	}
	return ""
}

// formatGo runs gofmt on the file, then vets its package and keeps only the lines naming the file.
func formatGo(ctx context.Context, path string) string {
	dir := filepath.Dir(path)
	if ran := proc.Exec(dir, remaining(ctx), "gofmt", "-w", path); !ran.OK() {
		return strings.TrimSpace(ran.Output)
	}
	ran := proc.Exec(dir, remaining(ctx), "go", "vet", ".")
	if ran.OK() {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(ran.Output, "\n") {
		if strings.Contains(line, filepath.Base(path)+":") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return strings.Join(lines, "\n")
}

// formatTypeScript runs the repo's own installed formatter and linter on the file, when it has them.
func formatTypeScript(ctx context.Context, root, path string) string {
	formatter := filepath.Join(root, nodeBin, "prettier")
	if _, err := os.Stat(formatter); err == nil {
		proc.Exec(root, remaining(ctx), formatter, "--write", path)
	}
	linter := filepath.Join(root, nodeBin, "eslint")
	if _, err := os.Stat(linter); err != nil {
		return ""
	}
	ran := proc.Exec(root, remaining(ctx), linter, path)
	if ran.OK() {
		return ""
	}
	return strings.TrimSpace(ran.Output)
}

// remaining is the time left before the hook's deadline, or the default command clock with none.
func remaining(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		return time.Until(deadline)
	}
	return proc.DefaultTimeout
}

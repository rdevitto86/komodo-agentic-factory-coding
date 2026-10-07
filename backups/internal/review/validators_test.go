package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diffAdding wraps added lines in a minimal unified diff for one file.
func diffAdding(file string, lines ...string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n@@ -1,0 +1,%d @@\n", file, file, len(lines))
	for _, line := range lines {
		out.WriteString("+" + line + "\n")
	}
	return out.String()
}

// writeFile writes one file under root, creating its directory.
func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

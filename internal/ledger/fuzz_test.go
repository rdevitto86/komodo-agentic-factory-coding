package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzRead proves a corrupt ledger line is skipped, never a panic, and aggregates cleanly.
func FuzzRead(f *testing.F) {
	f.Add(`{"run":"r","station":"close","seconds":1.5,"outcome":"ok"}` + "\n")
	f.Add("{not json\n\n{\"tokens_in\":-1}\n")
	f.Fuzz(func(t *testing.T, text string) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, RunFile), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		entries, _ := New(dir).Read(RunFile)
		_ = Render(Aggregate(entries))
	})
}

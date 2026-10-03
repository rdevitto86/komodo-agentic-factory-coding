package gate

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// GofmtCheck fails naming each Go file under root that gofmt would rewrite.
func GofmtCheck(root string) Check {
	return Check{Name: "gofmt", Run: func(out io.Writer) error {
		unformatted, err := Unformatted(root)
		if err != nil {
			return err
		}
		for _, path := range unformatted {
			fmt.Fprintln(out, path)
		}
		if len(unformatted) > 0 {
			return fmt.Errorf("%d file(s) need gofmt -w", len(unformatted))
		}
		return nil
	}}
}

// Unformatted lists every Go file under root gofmt would rewrite, skipping the directories the go tool skips.
func Unformatted(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if formatted, err := format.Source(data); err != nil || !bytes.Equal(formatted, data) {
			rel, _ := filepath.Rel(root, path)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out, err
}

package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUnformattedListsOnlyWhatGofmtWouldRewrite(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"ok.go":              "package a\n\nfunc F() {}\n",
		"bad.go":             "package a\nfunc  G( ) {}\n",
		"broken.go":          "package a\nfunc {\n",
		"testdata/skip.go":   "package a\nfunc  H( ) {}\n",
		".komodo/wt/skip.go": "package a\nfunc  I( ) {}\n",
		"_scratch/skip.go":   "package a\nfunc  J( ) {}\n",
		"sub/nested_ok.go":   "package sub\n",
		"sub/nested_bad.go":  "package sub\nvar  X=1\n",
		"notes.txt":          "func  K( ) {}\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Unformatted(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bad.go", "broken.go", "sub/nested_bad.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Unformatted = %v, want %v", got, want)
	}
	if err := GofmtCheck(root).Run(os.Stderr); err == nil {
		t.Fatal("the check passed with unformatted files")
	}
}

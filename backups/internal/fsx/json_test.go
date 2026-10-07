package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadStrictJSON(t *testing.T) {
	type config struct {
		Name string `json:"name"`
	}
	cases := []struct {
		name, body string
		write      bool
		found      bool
		fails      bool
	}{
		{name: "absent"},
		{name: "blank", body: " \n", write: true},
		{name: "valid", body: `{"name":"a"}`, write: true, found: true},
		{name: "bad JSON", body: "{broken", write: true, fails: true},
		{name: "unknown field", body: `{"other":1}`, write: true, fails: true},
		{name: "trailing data", body: `{"name":"a"} junk`, write: true, fails: true},
		{name: "a second value", body: `{"name":"a"}{"name":"b"}`, write: true, fails: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if tc.write {
				if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var got config
			found, err := ReadStrictJSON(path, &got)
			if tc.fails {
				if err == nil || !strings.Contains(err.Error(), path) {
					t.Fatalf("err = %v, want one naming %s", err, path)
				}
				return
			}
			if err != nil || found != tc.found {
				t.Fatalf("found, err = %v, %v; want %v, nil", found, err, tc.found)
			}
		})
	}
}

func TestReadStrictJSONReportsAReadErrorThatIsNotAbsence(t *testing.T) {
	dir := t.TempDir()
	var got struct{}
	if _, err := ReadStrictJSON(dir, &got); err == nil {
		t.Fatal("want an error reading a directory as a file")
	}
}

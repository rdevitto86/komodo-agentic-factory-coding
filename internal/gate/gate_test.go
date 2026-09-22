package gate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStopsAtTheFirstFailure(t *testing.T) {
	ran := []string{}
	checks := []Check{
		{Name: "one", Run: func(_ io.Writer) error { ran = append(ran, "one"); return nil }},
		{Name: "two", Run: func(_ io.Writer) error { ran = append(ran, "two"); return errors.New("boom") }},
		{Name: "three", Run: func(_ io.Writer) error { ran = append(ran, "three"); return nil }},
	}
	var out bytes.Buffer
	err := Run(checks, &out)
	if err == nil || !strings.Contains(err.Error(), "two: boom") {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(ran, ",") != "one,two" {
		t.Fatalf("ran = %v", ran)
	}
}

func TestRunReportsEveryCheckPassing(t *testing.T) {
	var out bytes.Buffer
	checks := []Check{{Name: "only", Run: func(_ io.Writer) error { return nil }}}
	if err := Run(checks, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 check(s) passed") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestReadManifestParsesSumLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestName)
	body := "aaa  komodo-darwin-arm64\nbbb  komodo-linux-amd64\n\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sums, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if sums["komodo-darwin-arm64"] != "aaa" || len(sums) != 2 {
		t.Fatalf("sums = %v", sums)
	}
}

func TestSumIsStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("komodo"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := Sum(path)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Sum(path)
	if first != second || len(first) != 64 {
		t.Fatalf("sum = %q %q", first, second)
	}
}

func TestInstallWritesBothHooks(t *testing.T) {
	dir := t.TempDir()
	written, err := Install(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("written = %v", written)
	}
	for _, path := range written {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s is not executable", path)
		}
		body, _ := os.ReadFile(path)
		if !strings.Contains(string(body), "gate") || !strings.Contains(string(body), "uname") {
			t.Fatalf("%s does not run the gate by platform", path)
		}
	}
}

func TestTargetsCoverThreePlatforms(t *testing.T) {
	if len(Targets) != 3 {
		t.Fatalf("targets = %d", len(Targets))
	}
	seen := map[string]bool{}
	for _, target := range Targets {
		seen[target.GOOS] = true
	}
	for _, want := range []string{"darwin", "windows", "linux"} {
		if !seen[want] {
			t.Errorf("no target for %s", want)
		}
	}
}

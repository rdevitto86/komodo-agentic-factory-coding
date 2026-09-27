package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hookRun is what one runHook call printed and the code it exited with, -1 when it returned.
type hookRun struct {
	stdout string
	stderr string
	code   int
}

// runHookWith runs runHook on stdin in a scratch worktree, capturing both streams and the exit code.
func runHookWith(t *testing.T, stdin string, args ...string) hookRun {
	t.Helper()
	scratch := t.TempDir()
	stdinPath := filepath.Join(scratch, "stdin")
	if err := os.WriteFile(stdinPath, []byte(stdin), 0o644); err != nil {
		t.Fatal(err)
	}
	inFile, err := os.Open(stdinPath)
	if err != nil {
		t.Fatal(err)
	}
	outFile, err := os.Create(filepath.Join(scratch, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	errFile, err := os.Create(filepath.Join(scratch, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut, oldErr, oldExit := os.Stdin, os.Stdout, os.Stderr, exit
	result := hookRun{code: -1}
	func() {
		defer func() {
			os.Stdin, os.Stdout, os.Stderr, exit = oldIn, oldOut, oldErr, oldExit
			if recovered := recover(); recovered != nil && recovered != "exit" {
				panic(recovered)
			}
		}()
		os.Stdin, os.Stdout, os.Stderr = inFile, outFile, errFile
		exit = func(c int) { result.code = c; panic("exit") }
		runHook(scratch, args)
	}()
	for _, file := range []*os.File{inFile, outFile, errFile} {
		file.Close()
	}
	out, _ := os.ReadFile(filepath.Join(scratch, "stdout"))
	errOut, _ := os.ReadFile(filepath.Join(scratch, "stderr"))
	result.stdout, result.stderr = string(out), string(errOut)
	return result
}

func TestRunHookAllowsWithNoHookNamed(t *testing.T) {
	got := runHookWith(t, "")
	if got.code != -1 || !strings.Contains(got.stderr, "name a hook; allowing") {
		t.Fatalf("got %+v, want a logged allow", got)
	}
}

func TestRunHookDispatchesThroughTheTable(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
		args  []string
		logs  bool
	}{
		{"an unknown hook allows", "{}", []string{"nosuch"}, true},
		{"a hook that cannot read its payload allows", "not json", []string{"timewarn", "--turns", "5"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runHookWith(t, tc.stdin, tc.args...)
			if got.code != 0 || got.stdout != "" {
				t.Fatalf("got %+v, want an allow", got)
			}
			if logged := strings.Contains(got.stderr, "allowing"); logged != tc.logs {
				t.Fatalf("stderr = %q, want logged %v", got.stderr, tc.logs)
			}
		})
	}
}

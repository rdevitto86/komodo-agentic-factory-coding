package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const zeroSha = "0000000000000000000000000000000000000000"

// verifyOrder is the shared discovery order; a repo's first present entry is its gate.
var verifyOrder = []struct{ path, kind string }{
	{".claude/verify.py", "python"},
	{"scripts/verify.py", "python"},
	{".claude/verify.sh", "shell"},
	{"Makefile", "make"},
	{"Taskfile.yml", "task"},
	{"Taskfile.yaml", "task"},
	{"justfile", "just"},
}

// hasTarget reports whether a build file declares a target line at the given indent.
func hasTarget(path, target, indent string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, indent+target+":") {
			return true
		}
	}
	return false
}

// resolveVerify returns the repo's verify command, or empty when it declares no gate.
func resolveVerify(root string) string {
	python := "python3"
	if found := findPython(); found != nil {
		python = strings.Join(found, " ")
	}
	if strings.ContainsAny(python, " \t") {
		python = "'" + python + "'"
	}
	for _, entry := range verifyOrder {
		path := filepath.Join(root, entry.path)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		switch entry.kind {
		case "python":
			return python + " " + entry.path
		case "shell":
			return "bash " + entry.path
		case "make":
			if hasTarget(path, "verify", "") {
				return "make verify"
			}
		case "task":
			if hasTarget(path, "verify", "  ") {
				return "task verify"
			}
		case "just":
			if hasTarget(path, "verify", "") {
				return "just verify"
			}
		}
	}
	return ""
}

// isForce reports whether the update is not a fast-forward of the remote ref.
func isForce(root, localSha, remoteSha string) bool {
	if remoteSha == zeroSha || localSha == zeroSha {
		return false
	}
	cmd := exec.Command("git", "merge-base", "--is-ancestor", remoteSha, localSha)
	cmd.Dir = root
	return cmd.Run() != nil
}

// runVerify runs the repo's gate under its timeout and returns the output plus whether it passed.
func runVerify(root, command string) (string, bool) {
	seconds := 600
	if raw := os.Getenv("KOMODO_VERIFY_TIMEOUT"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			seconds = parsed
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/s", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// prePushMain refuses a protected ref, a delete, or a force update, then runs the repo's verify gate.
func prePushMain() {
	root := gitOutput("", "rev-parse", "--show-toplevel")
	if root == "" {
		return
	}
	patterns := protectedPatterns(root)
	var problems []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 4 {
			continue
		}
		localSha, remoteRef, remoteSha := parts[1], parts[2], parts[3]
		name := strings.TrimPrefix(remoteRef, "refs/heads/")
		if isProtected(name, patterns) {
			problems = append(problems, fmt.Sprintf("push to protected ref %s; open a pull request instead", name))
		}
		if localSha == zeroSha {
			problems = append(problems, fmt.Sprintf("deleting remote ref %s from a hook-guarded push", name))
		}
		if isForce(root, localSha, remoteSha) {
			problems = append(problems, fmt.Sprintf("non-fast-forward update of %s; rewriting published history is refused", name))
		}
	}
	if len(problems) > 0 {
		blocked("pre-push", problems)
	}

	command := resolveVerify(root)
	if command == "" {
		return
	}
	if output, ok := runVerify(root, command); !ok {
		fmt.Fprint(os.Stderr, output)
		fmt.Fprintf(os.Stderr, "\npre-push: %s failed; fix it or push with --no-verify if you must\n", command)
		os.Exit(1)
	}
}

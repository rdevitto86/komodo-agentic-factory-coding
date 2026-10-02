package guard

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Case is one row of the table the gate runs: a call, and whether the guard must refuse it.
type Case struct {
	Name   string
	Tool   string
	Input  map[string]any
	Branch string
	Mode   Mode
	// Role sets RoleEnv for this row only, empty meaning the orchestrator, which carries none.
	Role    string
	Deny    bool
	Finding string
}

// bash builds a table row for one shell command, judged under the default mode.
func bash(name, command, branch string, deny bool, finding string) Case {
	return Case{Name: name, Tool: "Bash", Input: map[string]any{"command": command}, Branch: branch, Deny: deny, Finding: finding}
}

// write builds a table row for one file write.
func write(name, path string, deny bool, finding string) Case {
	return Case{Name: name, Tool: "Write", Input: map[string]any{"file_path": path}, Branch: "feat/x", Deny: deny, Finding: finding}
}

// asRole sets a table row's role, empty meaning the orchestrator, which carries none.
func asRole(role string, row Case) Case {
	row.Role = role
	return row
}

// spawn builds a table row for one agent-spawn call, with or without an isolation option.
func spawn(name, isolation string, deny bool, finding string) Case {
	input := map[string]any{}
	if isolation != "" {
		input["isolation"] = isolation
	}
	return Case{Name: name, Tool: "Agent", Input: input, Branch: "feat/x", Deny: deny, Finding: finding}
}

// configCases builds one denied row per path the policy protects, so no host is named here.
func configCases(policy Policy) []Case {
	var out []Case
	for _, pattern := range policy.ConfigPaths {
		path := strings.NewReplacer("/**", "/probe", "**/", "", "/*", "/probe").Replace(pattern)
		out = append(out, write("a write under "+pattern, path, true, "host or toolkit config"))
		out = append(out, bash("redirecting into "+pattern, "echo x > "+path, "feat/x", true, "host or toolkit config"))
	}
	return out
}

// Table is every call the gate checks: denied rows for each rule, and allowed opposites proving no overreach.
func Table(policy Policy) []Case {
	table := append(configCases(policy), []Case{
		// Rule 1: no commit, push, merge, delete, or force on a critical ref.
		bash("commit on main", "git commit -m 'feat: thing'", "main", true, "create a branch first"),
		bash("commit on its own branch", "git commit -m 'feat: thing'", "feat/x", false, ""),
		bash("push to main", "git push origin main", "feat/x", true, "open a pull request"),
		bash("a model session's push to main is refused (REQ-26)", "git push origin main", "feat/x", true, "open a pull request"),
		bash("push its own branch by name", "git push origin feat/x", "feat/x", false, ""),
		bash("merge on main", "git merge feat/x", "main", true, "merge button"),
		bash("merge main into its branch", "git merge main", "feat/x", false, ""),
		bash("gh pr merge", "gh pr merge 12 --squash", "feat/x", true, "merge button"),
		bash("branch -D main", "git branch -D main", "feat/x", true, "never deleted"),
		bash("delete its own branch", "git branch -D feat/old", "feat/x", false, ""),
		asRole("builder", bash("a line session's push to an epic branch is refused (decision 0028)", "git push origin feat/1.0.0-alpha.7", "feat/x", true, "open a pull request")),
		asRole("builder", bash("a line session's merge onto an epic branch is refused (decision 0028)", "git merge feat/x", "feat/1.0.0-alpha.7", true, "merge button")),
		bash("the orchestrator pushes an epic branch; a person still merges it into main", "git push origin feat/1.0.0-alpha.7", "feat/x", false, ""),
		bash("the orchestrator merges onto an epic branch", "git merge feat/x", "feat/1.0.0-alpha.7", false, ""),
		bash("push to a slugged feat branch is not an epic branch", "git push origin feat/versions-go-alpha", "feat/x", false, ""),
		bash("a push with several refspecs is judged on every one, not only the last", "git push origin main feat/x", "feat/x", true, "open a pull request"),
		bash("sudo push to main", "sudo git push origin main", "feat/x", true, "open a pull request"),
		bash("a shell -c push to main", "bash -c 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("an env-prefixed push to main", "FOO=1 env -i git push origin main", "feat/x", true, "open a pull request"),
		bash("an eval of a push to main", `eval "git push origin main"`, "feat/x", true, "open a pull request"),
		bash("a login shell's -lc push to main", "bash -lc 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("a -c whose script follows another option", "bash -c -e 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("a wrapper with a flag argument", "sudo -u root git push origin main", "feat/x", true, "open a pull request"),
		bash("a flag value that names git", "env -u git git push origin main", "feat/x", true, "open a pull request"),
		bash("a wrapper chain five deep", "sudo env A=1 nice -n 5 timeout 60 bash -c 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("a commit inside an if", "if true; then git commit -m x; fi", "main", true, "create a branch first"),
		bash("a compound switch -c then merge is judged on the branch it created (TSK-08.14.2)",
			"git switch -c fix/x && git merge origin/main", "main", false, ""),
		bash("a compound checkout -b then merge is judged on the branch it created (TSK-08.14.2)",
			"git checkout -b fix/y && git merge origin/main", "main", false, ""),
		bash("a switch back onto main still refuses the merge after it (TSK-08.14.2)",
			"git switch main && git merge feat/x", "feat/x", true, "merge button"),
		bash("a push inside a brace group", "{ git push origin main; }", "feat/x", true, "open a pull request"),
		bash("sudo push of its own branch", "sudo git push origin feat/x", "feat/x", false, ""),
		bash("a timed test run", "timeout 60 go test ./...", "feat/x", false, ""),
		bash("xargs searching for git", "git ls-files | xargs grep git", "feat/x", false, ""),
		bash("a shell running a script file", "bash scripts/check.sh", "feat/x", false, ""),

		// Rule 2: pushed history is never rewritten, on any branch.
		bash("force push to own branch", "git push --force origin feat/x", "feat/x", true, "never rewritten"),
		bash("push -u to own branch", "git push -u origin feat/x", "feat/x", false, ""),

		// Rule 3: git hooks are never skipped.
		bash("commit --no-verify skips the gate", "git commit --no-verify -m x", "feat/x", true, "skips the gate"),
		bash("commit runs the gate", "git commit -m 'feat: thing'", "feat/x", false, ""),

		// A trailer is now the commit-msg hook's rule, not the guard's; the tool call itself is allowed.
		bash("a trailer reaches the commit-msg hook, not the guard", "git commit -m 'feat: x\n\nCo-authored-by: A <a@b.c>'", "feat/x", false, ""),

		// Rule 5: a line session never edits or writes outside its worktree.
		asRole("builder", write("edit above the root", "../outside/file.go", true, "outside the worktree")),
		asRole("builder", write("a new source file", "internal/line/new.go", false, "")),
		asRole("builder", bash("redirect above the root", "echo x > ../outside.txt", "feat/x", true, "outside the worktree")),
		asRole("builder", bash("redirect into the worktree", "echo x > out.txt", "feat/x", false, "")),
		asRole("builder", bash("a shell -c redirect above the root", "sh -c 'echo x > ../outside.txt'", "feat/x", true, "outside the worktree")),
		write("the orchestrator writes outside the repo", "../outside/file.go", false, ""),

		// A line spawn never cuts its own worktree; the line already cut it. The orchestrator may.
		asRole("builder", spawn("a line spawn with isolation set is denied", "worktree", true, "a spawn never cuts its own worktree")),
		asRole("builder", spawn("a line spawn without isolation is allowed", "", false, "")),
		spawn("the orchestrator spawns an isolated agent", "worktree", false, ""),

		// Structural, not one of the five: docs/prd.md and eval/** refuse every line role (REQ-41).
		asRole("builder", write("a line role's write to docs/prd.md is refused (REQ-41)", "docs/prd.md", true, "host or toolkit config")),
		asRole("builder", write("a line role's write to eval/** is refused (REQ-41)", "eval/probe", true, "host or toolkit config")),
		asRole("", write("the orchestrator is not a line session and may write docs/prd.md", "docs/prd.md", false, "")),

		// Inside the worktree an agent is free.
		bash("rm -rf a build directory", "rm -rf node_modules", "feat/x", false, ""),
		bash("run the tests", "go test ./...", "feat/x", false, ""),
		bash("open a pull request through komodo", "komodo pr create --title 'fix: x' --body b", "feat/x", false, ""),
		bash("gh pr create skips the title check and the labels", "gh pr create --base main --head feat/x --title t --body b", "feat/x", true, "komodo pr create"),
	}...)
	return table
}

// RunTable checks every row and returns the rows whose outcome was wrong.
func RunTable(root string, policy Policy) []string {
	original, hadRole := os.LookupEnv(RoleEnv)
	defer restoreRole(original, hadRole)
	var wrong []string
	for _, item := range Table(policy) {
		setRole(item.Role)
		request := Request{HookEventName: "PreToolUse", ToolName: item.Tool, Cwd: root, ToolInput: item.Input}
		rowPolicy := policy
		rowPolicy.Mode = item.Mode
		decision := Check(request, rowPolicy, item.Branch)
		switch {
		case decision.Deny != item.Deny:
			verb := "denied"
			if item.Deny {
				verb = "allowed"
			}
			wrong = append(wrong, fmt.Sprintf("%s: %s and should not be", item.Name, verb))
		case item.Deny && item.Finding != "" && !containsAny(decision.Findings, item.Finding):
			wrong = append(wrong, fmt.Sprintf("%s: denied for the wrong reason: %s", item.Name, strings.Join(decision.Findings, "; ")))
		}
	}
	return wrong
}

// Report writes the table's outcome and returns whether every row held.
func Report(root string, policy Policy, out io.Writer) bool {
	table := Table(policy)
	denied := 0
	for _, item := range table {
		if item.Deny {
			denied++
		}
	}
	wrong := RunTable(root, policy)
	for _, line := range wrong {
		fmt.Fprintln(out, line)
	}
	fmt.Fprintf(out, "%d command(s), %d denied, %d allowed, %d wrong\n",
		len(table), denied, len(table)-denied, len(wrong))
	return len(wrong) == 0
}

// setRole makes this process a line role for the rest of the row, or the orchestrator when empty.
func setRole(role string) {
	if role == "" {
		os.Unsetenv(RoleEnv)
		return
	}
	os.Setenv(RoleEnv, role)
}

// restoreRole puts RoleEnv back the way RunTable found it.
func restoreRole(original string, had bool) {
	if had {
		os.Setenv(RoleEnv, original)
		return
	}
	os.Unsetenv(RoleEnv)
}

// containsAny reports whether any finding holds the needle.
func containsAny(findings []string, needle string) bool {
	for _, finding := range findings {
		if strings.Contains(finding, needle) {
			return true
		}
	}
	return false
}

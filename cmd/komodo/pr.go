package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"komodo/internal/git"
	"komodo/internal/line"
	"komodo/internal/pr"
)

// labelFlags collects a repeated --label flag.
type labelFlags []string

// String joins the collected labels with commas.
func (l *labelFlags) String() string { return strings.Join(*l, ",") }

// Set appends one --label value.
func (l *labelFlags) Set(value string) error {
	*l = append(*l, value)
	return nil
}

// prTitle is the pull request title the template asks for: a known type, a colon, then the summary.
var prTitle = regexp.MustCompile(`^(feat|fix|chore|docs|test|refactor|perf|build|ci)(\([a-z0-9/-]+\))?!?: \S.*[^.\s]$`)

// maxTitle is the longest pull request title the template allows.
const maxTitle = 72

// runPR opens a pull request with its title checked and its labels applied, or labels the current branch's.
func runPR(root string, args []string) {
	if len(args) == 0 {
		fail(errors.New("usage: komodo pr create --title t (--body b | --body-file f) [--base b] [--draft] [--label l]... | komodo pr label [--base b] [--label l]..."))
	}
	switch args[0] {
	case "create":
		runPRCreate(root, args[1:])
	case "label":
		runPRLabel(root, args[1:])
	default:
		fail(fmt.Errorf("unknown pr command %q: create or label", args[0]))
	}
}

// runPRCreate checks the title, opens the pull request from the current branch, and labels it.
func runPRCreate(root string, args []string) {
	set := flag.NewFlagSet("pr create", flag.ExitOnError)
	title := set.String("title", "", "<type>: <summary>, at most 72 characters")
	body := set.String("body", "", "the pull request body")
	bodyFile := set.String("body-file", "", "a file holding the pull request body")
	base := set.String("base", "", "the branch to merge into, the remote's default when empty")
	draft := set.Bool("draft", false, "open the pull request as a draft")
	var extra labelFlags
	set.Var(&extra, "label", "a further label, such as breaking; repeatable")
	_ = set.Parse(args)
	if err := checkPRTitle(*title); err != nil {
		fail(err)
	}
	text, err := prBody(*body, *bodyFile)
	if err != nil {
		fail(err)
	}
	head := git.TrackedBranch(root)
	if head == "" {
		fail(errors.New("HEAD tracks no branch; check out one, or cut a detached worktree with komodo worktree add"))
	}
	target := *base
	if target == "" {
		target = line.DefaultBase(root)
	}
	if head == target {
		fail(fmt.Errorf("check out the branch to open a pull request from; HEAD is %s", head))
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "origin/"+head); err != nil {
		fail(fmt.Errorf("origin has no %s; push it first: git push origin HEAD:refs/heads/%s", head, head))
	}
	client := pr.New(root)
	url, err := client.Create(target, head, *title, text, *draft)
	if err != nil {
		fail(err)
	}
	fmt.Println(url)
	labelPull(root, client, url, target, extra)
}

// runPRLabel applies the labels the current branch's open pull request earns.
func runPRLabel(root string, args []string) {
	set := flag.NewFlagSet("pr label", flag.ExitOnError)
	base := set.String("base", "", "the branch the pull request merges into, the remote's default when empty")
	var extra labelFlags
	set.Var(&extra, "label", "a further label, such as breaking; repeatable")
	_ = set.Parse(args)
	target := *base
	if target == "" {
		target = line.DefaultBase(root)
	}
	client := pr.New(root)
	pull, err := client.View("")
	if err != nil {
		fail(err)
	}
	labelPull(root, client, pull.URL, target, extra)
}

// labelPull adds @agent, the scope the branch's files earn, the stage, branch/feature, and any extra labels.
func labelPull(root string, client *pr.Client, url, base string, extra []string) {
	wanted := append([]string{"@agent", line.ScopeLabel(root, changedFiles(root, base))}, extra...)
	kept, warnings := line.ApplyLabelSet(client, url, wanted, line.OptionalLabels(root, base))
	fmt.Printf("labels: %s\n", strings.Join(kept, ", "))
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
}

// checkPRTitle refuses a title the pull request template does not allow.
func checkPRTitle(title string) error {
	if len(title) > maxTitle {
		return fmt.Errorf("the title is %d characters; at most %d", len(title), maxTitle)
	}
	if !prTitle.MatchString(title) {
		return fmt.Errorf("the title %q is not <type>: <summary>, with type one of feat fix chore docs test refactor perf build ci and no trailing period", title)
	}
	return nil
}

// prBody is the body text, read from a file when one is named; exactly one source is required.
func prBody(body, file string) (string, error) {
	if (body == "") == (file == "") {
		return "", errors.New("give the body with exactly one of --body or --body-file")
	}
	if file == "" {
		return body, nil
	}
	data, err := os.ReadFile(file)
	return string(data), err
}

// changedFiles are the files the current branch changes since it left base, by the remote's copy when present.
func changedFiles(root, base string) []string {
	for _, ref := range []string{"origin/" + base, base} {
		if out, err := git.Run(root, "diff", "--name-only", ref+"...HEAD"); err == nil {
			return strings.Fields(out)
		}
	}
	return nil
}

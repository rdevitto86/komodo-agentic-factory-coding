// Package pr wraps the gh commands the line uses to open and answer pull requests.
package pr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"komodo/internal/proc"
)

// Timeout bounds how long one gh command may run before its process group is killed; a test lowers it.
var Timeout = proc.DefaultTimeout

// waitDelay bounds how long a killed gh command's output pipes may stay open after it exits.
const waitDelay = 5 * time.Second

// Thread is one unresolved review comment on a pull request.
type Thread struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Author string `json:"author"`
	Body   string `json:"body"`
}

// Pull is the fields the line reads back about a pull request.
type Pull struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
	Title  string `json:"title"`
	Draft  bool   `json:"isDraft"`
}

// Runner runs one gh invocation, so a test can stand in for the real CLI.
type Runner func(dir string, args ...string) (string, error)

// Run is the default runner: the gh binary on PATH; a hung gh is killed, process group
// included, once Timeout passes.
func Run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	stdout, stderr := proc.NewBoundedWriter(proc.MaxOutput), proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("gh %s: timed out after %s: %s", strings.Join(args, " "), Timeout, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return "", fmt.Errorf("gh %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Client talks to one repository through gh.
type Client struct {
	Dir string
	Run Runner
}

// New returns a client rooted at dir using the gh binary.
func New(dir string) *Client { return &Client{Dir: dir, Run: Run} }

// run invokes the client's runner.
func (c *Client) run(args ...string) (string, error) {
	runner := c.Run
	if runner == nil {
		runner = Run
	}
	return runner(c.Dir, args...)
}

// Create opens a pull request and returns its URL.
func (c *Client) Create(base, head, title, body string, draft bool) (string, error) {
	args := []string{"pr", "create", "--base", base, "--head", head, "--title", title, "--body", body}
	if draft {
		args = append(args, "--draft")
	}
	return c.run(args...)
}

// View returns what gh knows about one pull request, or the current branch's.
func (c *Client) View(number string) (Pull, error) {
	var pull Pull
	args := []string{"pr", "view", "--json", "number,url,state,title,isDraft"}
	if number != "" {
		args = append(args[:2], append([]string{number}, args[2:]...)...)
	}
	out, err := c.run(args...)
	if err != nil {
		return pull, err
	}
	return pull, json.Unmarshal([]byte(out), &pull)
}

// MergedHead reports whether the forge holds a merged pull request whose head is branch.
func (c *Client) MergedHead(branch string) (bool, error) {
	out, err := c.run("pr", "list", "--head", branch, "--state", "merged", "--json", "headRefName", "--limit", "1")
	if err != nil {
		return false, err
	}
	var rows []struct {
		Head string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Head == branch {
			return true, nil
		}
	}
	return false, nil
}

// OpenHead reports whether the forge holds an open pull request from head into base.
func (c *Client) OpenHead(head, base string) (bool, error) {
	out, err := c.run("pr", "list", "--head", head, "--base", base, "--state", "open", "--json", "headRefName", "--limit", "1")
	if err != nil {
		return false, err
	}
	var rows []struct {
		Head string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Head == head {
			return true, nil
		}
	}
	return false, nil
}

// Edit changes a pull request's body, title, or draft state.
func (c *Client) Edit(number string, args ...string) error {
	_, err := c.run(append([]string{"pr", "edit", number}, args...)...)
	return err
}

// Label adds labels to a pull request, ignoring ones the repo does not define.
func (c *Client) Label(number string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	args := []string{"pr", "edit", number}
	for _, label := range labels {
		args = append(args, "--add-label", label)
	}
	_, err := c.run(args...)
	return err
}

// Ready marks a draft pull request ready for review.
func (c *Client) Ready(number string) error {
	_, err := c.run("pr", "ready", number)
	return err
}

// Unlabel removes labels from a pull request.
func (c *Client) Unlabel(number string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	args := []string{"pr", "edit", number}
	for _, label := range labels {
		args = append(args, "--remove-label", label)
	}
	_, err := c.run(args...)
	return err
}

// Labels lists the labels the repository already defines.
func (c *Client) Labels() ([]string, error) {
	out, err := c.run("label", "list", "--json", "name")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names, nil
}

// Merge merges a pull request into its base with a merge commit, never a squash or a rebase,
// so a branch stacked on it keeps the commits its own history was cut from.
func (c *Client) Merge(number string) error {
	_, err := c.run("pr", "merge", number, "--merge")
	return err
}

// threadsQuery fetches a pull request's inline review threads by URL.
const threadsQuery = `query($url: URI!) {
  resource(url: $url) {
    ... on PullRequest {
      reviewThreads(first: 100) {
        nodes {
          id
          isResolved
          path
          line
          comments(first: 10) {
            nodes {
              body
              author { login }
            }
          }
        }
      }
    }
  }
}`

// replyMutation adds a reply comment inside one review thread.
const replyMutation = `mutation($id: ID!, $body: String!) {
  addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $id, body: $body}) {
    comment { id }
  }
}`

// Threads lists the unresolved review threads on a pull request.
func (c *Client) Threads(number string) ([]Thread, error) {
	pull, err := c.View(number)
	if err != nil {
		return nil, err
	}
	out, err := c.run("api", "graphql", "-f", "query="+threadsQuery, "-f", "url="+pull.URL)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Resource struct {
				ReviewThreads struct {
					Nodes []struct {
						ID         string `json:"id"`
						IsResolved bool   `json:"isResolved"`
						Path       string `json:"path"`
						Line       int    `json:"line"`
						Comments   struct {
							Nodes []struct {
								Body   string `json:"body"`
								Author struct {
									Login string `json:"login"`
								} `json:"author"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"resource"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, err
	}
	var threads []Thread
	for _, node := range payload.Data.Resource.ReviewThreads.Nodes {
		if node.IsResolved || len(node.Comments.Nodes) == 0 {
			continue
		}
		opening := node.Comments.Nodes[0]
		threads = append(threads, Thread{
			ID: node.ID, Path: node.Path, Line: node.Line,
			Author: opening.Author.Login, Body: opening.Body,
		})
	}
	return threads, nil
}

// resolveMutation marks one review thread resolved.
const resolveMutation = `mutation($id: ID!) {
  resolveReviewThread(input: {threadId: $id}) {
    thread { id }
  }
}`

// Resolve marks one review thread resolved, so it drops out of the next Threads call.
func (c *Client) Resolve(threadID string) error {
	_, err := c.run("api", "graphql", "-f", "query="+resolveMutation, "-f", "id="+threadID)
	return err
}

// KeepKnown returns the known labels matching a wanted one by its name before the first space,
// so '@agent' finds '@agent 🤖' and 'scope/guard' finds 'scope/guard 🛡️'.
func KeepKnown(wanted, known []string) []string {
	var out []string
	for _, label := range wanted {
		for _, candidate := range known {
			if labelName(candidate) == label {
				out = append(out, candidate)
				break
			}
		}
	}
	return out
}

// labelName is a label's name before its first space, dropping any trailing emoji.
func labelName(label string) string {
	if i := strings.Index(label, " "); i >= 0 {
		return label[:i]
	}
	return label
}

// Package pr wraps the gh commands the line uses to open and answer pull requests.
package pr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

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

// Run is the default runner: the gh binary on PATH.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
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

// Comment posts one comment on a pull request.
func (c *Client) Comment(number, body string) error {
	_, err := c.run("pr", "comment", number, "--body", body)
	return err
}

// Threads lists the unresolved review threads on a pull request.
func (c *Client) Threads(number string) ([]Thread, error) {
	out, err := c.run("pr", "view", number, "--json", "reviews,comments")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Comments []struct {
			Path   string `json:"path"`
			Line   int    `json:"line"`
			Body   string `json:"body"`
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, err
	}
	var threads []Thread
	for index, comment := range payload.Comments {
		threads = append(threads, Thread{
			ID: fmt.Sprint(index), Path: comment.Path, Line: comment.Line,
			Author: comment.Author.Login, Body: comment.Body,
		})
	}
	return threads, nil
}

// Reply answers one review thread.
func (c *Client) Reply(number, body string) error { return c.Comment(number, body) }

// KeepKnown returns only the labels the repository already defines.
func KeepKnown(wanted, known []string) []string {
	var out []string
	for _, label := range wanted {
		for _, candidate := range known {
			if label == candidate {
				out = append(out, label)
				break
			}
		}
	}
	return out
}

package conductor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/mount"
)

// StationEscalate is the ledger station for the orchestrator session that settles one escalation.
const StationEscalate = "escalate"

// The one action an orchestrator returns for an escalation.
const (
	ActionAnswer  = "answer"
	ActionSplit   = "split"
	ActionClarify = "clarify"
	ActionRetry   = "retry"
	ActionStop    = "stop"
)

// answerLead opens the orchestrator's answer, as the builder that stopped on it reads it.
const answerLead = "## The orchestrator's answer\n\n"

// errNoAction refuses an orchestrator result that names none of the allowed actions.
var errNoAction = errors.New("the orchestrator returned no allowed action")

// Escalation is what the orchestrator reads about one stuck group: the state it left and why it stopped.
type Escalation struct {
	Group  string
	Left   GroupState
	Reason string
}

// escalate blocks a group stalled too often, else starts one orchestrator session and applies the one action
// it returns; with no orchestrator wired the group waits at Escalated for a person.
func (d *Driver) escalate(ctx context.Context, s *State, r *round) error {
	r.reason = d.reason(*s, r)
	if stalled(s, r) || d.Orchestrator == nil {
		return nil
	}
	req, err := d.Orchestrator(Escalation{Group: s.Group, Left: s.Left, Reason: r.reason})
	if err != nil {
		return err
	}
	head, err := d.Stations.Head()
	if err != nil {
		return err
	}
	before, err := d.Stations.Diff(head)
	if err != nil {
		return err
	}
	handle, err := d.Host.Start(ctx, req)
	if err != nil {
		return err
	}
	result, err := d.session(ctx, s, StationEscalate, req, handle)
	if err != nil {
		return err
	}
	action, _ := result.Value["action"].(string)
	answer, _ := result.Value["answer"].(string)
	switch action {
	case ActionAnswer:
		r.answer = answer
	case ActionSplit, ActionClarify:
		after, err := d.Stations.Diff(head)
		if err != nil {
			return err
		}
		// A rewrite touching a file outside the group's own backlog file is outside the orchestrator's limits.
		if outside := outsideGroupFile(s.Group, before, after); len(outside) > 0 {
			s.Stop, r.needs = true, fmt.Sprintf(
				"the %s touched %s outside its own backlog file", action, strings.Join(outside, ", "),
			)
			break
		}
		problems, err := d.lint()
		if err != nil {
			return err
		}
		// A rewrite that fails lint is outside the orchestrator's limits, so the group stops instead.
		if len(problems) > 0 {
			s.Stop, r.needs = true, fmt.Sprintf("the %s does not pass lint: %s", action, strings.Join(problems, "; "))
			break
		}
		r.answer = answer
	case ActionRetry:
		if r.heavy || d.Heavy.Model == "" {
			s.Stop, r.needs = true, "the one retry on the heavy tier is spent or has no machine"
			break
		}
		r.heavy, s.Left = true, Building
	case ActionStop:
		s.Stop = true
		r.needs, _ = result.Value["needs"].(string)
	default:
		return fmt.Errorf("%w: %q", errNoAction, action)
	}
	s.Answered = true
	return nil
}

// timeout saves a group whose own budget ran out as an escalation, and asks the orchestrator about it
// on a fresh context, so the spent budget never cuts off the session that decides what happens next.
func (d *Driver) timeout(s State, r *round, cause error) (State, error) {
	s.Left = s.Current
	s.Current = Escalated
	s.SlotFree, s.SessionDone, s.ChecksPassed, s.Conflict, s.ShipDone, s.Edited = false, false, false, false, false, false
	s.Escalate, s.Answered, s.Stop = true, false, false
	r.reason = fmt.Sprintf("%s ran out of its budget after %s", s.Group, s.TimeUsed.Round(time.Second))
	r.keep(&s)
	if err := d.Save(s); err != nil {
		return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
	}
	if err := d.escalate(context.Background(), &s, r); err != nil {
		return s, fmt.Errorf("%s escalated at %s: %w", s.Group, s.Current, err)
	}
	r.keep(&s)
	if err := d.Save(s); err != nil {
		return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
	}
	return s, cause
}

// changedFiles returns every path a unified diff's "+++ " and "--- " lines name, skipping /dev/null.
func changedFiles(diff string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(diff, "\n") {
		var prefix string
		switch {
		case strings.HasPrefix(line, "+++ "):
			prefix = "+++ "
		case strings.HasPrefix(line, "--- "):
			prefix = "--- "
		default:
			continue
		}
		path := strings.TrimPrefix(line, prefix)
		if path == "/dev/null" {
			continue
		}
		path = strings.TrimPrefix(strings.TrimPrefix(path, "a/"), "b/")
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}

// outsideGroupFile returns every path after newly touches outside the group's folder or a new sibling group folder.
func outsideGroupFile(groupID, before, after string) []string {
	had := map[string]bool{}
	for _, path := range changedFiles(before) {
		had[path] = true
	}
	epicDir := backlog.GroupFilesDir + "/" + backlog.EpicDirName(backlog.EpicIDOfGroup(groupID)) + "/"
	own := epicDir + backlog.GroupDirName(groupID) + "/"
	created := newFiles(after)
	var outside []string
	for _, path := range changedFiles(after) {
		if had[path] || strings.HasPrefix(path, own) || (created[path] && siblingGroupFile(path, epicDir)) {
			continue
		}
		outside = append(outside, path)
	}
	return outside
}

// newFiles is every path a diff creates: one whose old side is /dev/null.
func newFiles(diff string) map[string]bool {
	out := map[string]bool{}
	fromNull := false
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "--- "):
			fromNull = strings.TrimPrefix(line, "--- ") == "/dev/null"
		case strings.HasPrefix(line, "+++ ") && fromNull:
			out[strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")] = true
			fromNull = false
		}
	}
	return out
}

// siblingGroupFile reports a path inside a tg- folder directly under epicDir.
func siblingGroupFile(path, epicDir string) bool {
	rest, ok := strings.CutPrefix(path, epicDir)
	return ok && strings.HasPrefix(rest, "tg-") && strings.Contains(rest, "/")
}

// lint returns the lint problems a split or clarify left; with no linter wired nothing proves the rewrite.
func (d *Driver) lint() ([]string, error) {
	if d.Lint == nil {
		return []string{"no lint is wired to check the rewrite"}, nil
	}
	return d.Lint()
}

// reason is why the group escalated: the failure that stopped it, else each blocked task's question, else the
// blocked builder's own question or summary.
func (d *Driver) reason(s State, r *round) string {
	if r.reason != "" {
		return r.reason
	}
	result, err := d.Host.Result(lastSession(s))
	if err != nil {
		return "the session that stopped the group left no result: " + err.Error()
	}
	tasks, _ := result.Value["tasks"].([]any)
	var questions []string
	for _, each := range tasks {
		task, _ := each.(map[string]any)
		id, _ := task["task"].(string)
		question, _ := task["question"].(string)
		if task["result"] == resultBlocked && question != "" {
			questions = append(questions, id+": "+question)
		}
	}
	if len(questions) > 0 {
		return strings.Join(questions, "; ")
	}
	for _, key := range []string{"question", "summary"} {
		if text, ok := result.Value[key].(string); ok && text != "" {
			return text
		}
	}
	return "the builder returned BLOCKED with no question"
}

// builderRequest is the builder's request, on the heavy tier once the orchestrator asked for a retry.
func (d *Driver) builderRequest(r *round) mount.StartRequest {
	req := d.Builder
	if r.heavy {
		req.Model, req.Effort = d.Heavy.Model, d.Heavy.Effort
	}
	return req
}

// startBuilder resumes the blocked builder with the orchestrator's answer when the host can, else starts a
// fresh builder whose brief ends with that answer; with no answer it starts a fresh builder.
func (d *Driver) startBuilder(ctx context.Context, req mount.StartRequest, r *round) (mount.StartRequest, mount.Handle, error) {
	answer := r.answer
	r.answer = ""
	if answer == "" {
		handle, err := d.Host.Start(ctx, req)
		return req, handle, err
	}
	if r.builder != "" && d.Host.Capabilities().Resume {
		if handle, err := d.Host.Resume(ctx, r.builder, answerLead+answer); err == nil {
			return req, handle, nil
		}
	}
	req.Brief += "\n\n" + answerLead + answer
	handle, err := d.Host.Start(ctx, req)
	return req, handle, err
}

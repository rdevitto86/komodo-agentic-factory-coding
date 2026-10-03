package conductor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"komodo/internal/mount"
)

// orchestratorHost is the fake host with a scripted orchestrator: each orchestrator session returns the next action.
type orchestratorHost struct {
	*fakeHost
	actions []map[string]any
	asked   []mount.StartRequest
	// startErr, when set, fails every orchestrator session's start.
	startErr error
}

func (o *orchestratorHost) Start(ctx context.Context, req mount.StartRequest) (mount.Handle, error) {
	if req.Role != "orchestrator" {
		return o.fakeHost.Start(ctx, req)
	}
	if o.startErr != nil {
		return "", o.startErr
	}
	o.asked = append(o.asked, req)
	return o.handle(req.Role, pop(&o.actions, map[string]any{"action": ActionStop, "why": "nothing scripted"})), nil
}

// escalating wires the rig's driver to a scripted orchestrator, a heavy machine and a lint returning problems.
func escalating(r *rig, problems []string, actions ...map[string]any) *orchestratorHost {
	host := &orchestratorHost{fakeHost: r.host, actions: actions}
	r.driver.Host = host
	r.driver.Heavy = mount.Machine{Model: "opus", Effort: "high"}
	r.driver.Lint = func() ([]string, error) { return problems, nil }
	r.driver.Orchestrator = func(e Escalation) (mount.StartRequest, error) {
		return mount.StartRequest{Role: "orchestrator", Brief: string(e.Left) + ": " + e.Reason}, nil
	}
	return host
}

func TestAnEscalationGetsOneOrchestratorSessionAndItsAction(t *testing.T) {
	blocked := map[string]any{"result": "BLOCKED", "question": "which clock does auth.New take?"}
	for _, tc := range []struct {
		name     string
		builds   []map[string]any
		problems []string
		actions  []map[string]any
		want     GroupState
		asked    int
		check    func(t *testing.T, r *rig, host *orchestratorHost, final State)
	}{
		{
			name:    "an answer resumes the blocked builder with it",
			builds:  []map[string]any{blocked},
			actions: []map[string]any{{"action": ActionAnswer, "answer": "inject a clock parameter"}},
			want:    Shipped, asked: 1,
			check: func(t *testing.T, r *rig, host *orchestratorHost, _ State) {
				if !strings.Contains(host.asked[0].Brief, "Building: which clock does auth.New take?") {
					t.Fatalf("orchestrator brief = %q, want the state left and the builder's question", host.asked[0].Brief)
				}
				if len(r.host.inputs) != 1 || !strings.Contains(r.host.inputs[0], "inject a clock parameter") {
					t.Fatalf("builder inputs = %q, want the answer resumed into it", r.host.inputs)
				}
			},
		},
		{
			name:    "a clarify that passes lint returns to the state left",
			builds:  []map[string]any{blocked},
			actions: []map[string]any{{"action": ActionClarify, "answer": "TSK-1 now names a.go"}},
			want:    Shipped, asked: 1,
		},
		{
			name:     "a split that fails lint stops the group",
			builds:   []map[string]any{blocked},
			problems: []string{"TG-1: a task names no files"},
			actions:  []map[string]any{{"action": ActionSplit, "answer": "TG-1 splits in two"}},
			want:     Blocked, asked: 1,
		},
		{
			name:    "a retry rebuilds on the heavy tier, and only once",
			builds:  []map[string]any{blocked, blocked},
			actions: []map[string]any{{"action": ActionRetry}, {"action": ActionRetry}},
			want:    Blocked, asked: 2,
			check: func(t *testing.T, r *rig, _ *orchestratorHost, _ State) {
				var heavy int
				for _, req := range r.host.starts {
					if req.Role == "builder" && req.Model == "opus" && req.Effort == "high" {
						heavy++
					}
				}
				if heavy != 1 {
					t.Fatalf("builder starts = %+v, want exactly one on the heavy machine", r.host.starts)
				}
			},
		},
		{
			name:    "a stop blocks the group",
			builds:  []map[string]any{blocked},
			actions: []map[string]any{{"action": ActionStop, "needs": "a decision on the clock"}},
			want:    Blocked, asked: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			r.host.builds = tc.builds
			host := escalating(r, tc.problems, tc.actions...)
			final, err := r.drive(t)
			if tc.want == Shipped && err != nil {
				t.Fatalf("drive = %v; a settled escalation clears its failure", err)
			}
			if final.Current != tc.want || len(host.asked) != tc.asked {
				t.Fatalf("final = %s after %d orchestrator session(s), want %s after %d",
					final.Current, len(host.asked), tc.want, tc.asked)
			}
			if tc.check != nil {
				tc.check(t, r, host, final)
			}
		})
	}
}

func TestAnOrchestratorResultWithNoAllowedActionLeavesTheGroupEscalated(t *testing.T) {
	r := newRig(t)
	r.host.builds = []map[string]any{{"result": "BLOCKED"}}
	escalating(r, nil, map[string]any{"action": "rewrite the PRD"})
	final, err := r.drive(t)
	if !errors.Is(err, errNoAction) {
		t.Fatalf("drive error = %v, want errNoAction", err)
	}
	if final.Current != Escalated || final.Answered {
		t.Fatalf("final = %s answered %v, want Escalated and unanswered", final.Current, final.Answered)
	}
}

func TestASplitWithNoLintWiredStopsTheGroup(t *testing.T) {
	r := newRig(t)
	r.host.builds = []map[string]any{{"result": "BLOCKED"}}
	escalating(r, nil, map[string]any{"action": ActionSplit})
	r.driver.Lint = nil
	if final, _ := r.drive(t); final.Current != Blocked {
		t.Fatalf("final = %s, want Blocked when nothing can lint the rewrite", final.Current)
	}
}

func TestASavedUnansweredEscalationAsksTheOrchestrator(t *testing.T) {
	r := newRig(t)
	host := escalating(r, nil, map[string]any{"action": ActionAnswer, "answer": "ship it"})
	saved := State{Group: "TG-1", Current: Escalated, Escalate: true, Left: Shipping}
	*r.saved = append(*r.saved, saved)
	final, err := r.driver.Drive(context.Background(), saved)
	if err != nil || final.Current != Shipped || len(host.asked) != 1 {
		t.Fatalf("drive = %s, %v after %d session(s); want Shipped after one", final.Current, err, len(host.asked))
	}
}

func TestAnAnswerStartsAFreshBuilderWhenTheHostCannotResume(t *testing.T) {
	r := newRig(t)
	r.host.resume = false
	r.host.builds = []map[string]any{{"result": "BLOCKED"}}
	escalating(r, nil, map[string]any{"action": ActionAnswer, "answer": "use the fake clock"})
	if final, err := r.drive(t); err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	var builders []mount.StartRequest
	for _, req := range r.host.starts {
		if req.Role == "builder" {
			builders = append(builders, req)
		}
	}
	if len(builders) != 2 || !strings.HasSuffix(builders[1].Brief, answerLead+"use the fake clock") {
		t.Fatalf("builder starts = %+v, want a fresh builder whose brief ends with the answer", builders)
	}
}

func TestAnAnswerToABlockedRepairLeadsItsFixList(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}}
	r.host.repairs = []map[string]any{{"result": "BLOCKED"}}
	escalating(r, nil, map[string]any{"action": ActionAnswer, "answer": "the fix is in a.go"})
	if final, err := r.drive(t); err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	if len(r.host.inputs) != 2 || !strings.HasPrefix(r.host.inputs[1], answerLead+"the fix is in a.go\n\n## Fix list") {
		t.Fatalf("repair inputs = %q, want the answer ahead of the fix list", r.host.inputs)
	}
}

func TestAnOrchestratorThatCannotSettleLeavesTheGroupEscalated(t *testing.T) {
	failing := errors.New("no orchestrator")
	for _, tc := range []struct {
		name string
		wire func(r *rig)
	}{
		{"its request fails", func(r *rig) {
			r.driver.Orchestrator = func(Escalation) (mount.StartRequest, error) { return mount.StartRequest{}, failing }
		}},
		{"its lint fails", func(r *rig) {
			r.driver.Lint = func() ([]string, error) { return nil, failing }
		}},
		{"its session cannot start", func(r *rig) {
			r.driver.Host.(*orchestratorHost).startErr = failing
		}},
		{"its session is cut off", func(r *rig) {
			save := r.driver.Save
			r.driver.Save = func(s State) error {
				if s.Current == Escalated && len(s.Sessions) > 1 {
					return failing
				}
				return save(s)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			r.host.builds = []map[string]any{{"result": "BLOCKED"}}
			escalating(r, nil, map[string]any{"action": ActionClarify})
			tc.wire(r)
			final, err := r.drive(t)
			if !errors.Is(err, failing) || final.Current != Escalated || final.Answered {
				t.Fatalf("drive = %s answered %v, %v; want Escalated and unanswered", final.Current, final.Answered, err)
			}
		})
	}
}

func TestASavedEscalationWhoseOrchestratorFailsReturnsTheFailure(t *testing.T) {
	r := newRig(t)
	escalating(r, nil)
	r.driver.Orchestrator = func(Escalation) (mount.StartRequest, error) {
		return mount.StartRequest{}, errors.New("no orchestrator")
	}
	saved := State{Group: "TG-1", Current: Escalated, Escalate: true, Left: Shipping}
	*r.saved = append(*r.saved, saved)
	final, err := r.driver.Drive(context.Background(), saved)
	if err == nil || final.Current != Escalated {
		t.Fatalf("drive = %s, %v; want the failure with the group still Escalated", final.Current, err)
	}
}

package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"komodo/internal/backlog"
	"komodo/internal/conductor"
	"komodo/internal/doctor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/profile"
	"komodo/internal/review"
)

// Drive cuts one group's worktree when no run is open for it, then drives it through the conductor to Shipped,
// merged into its epic branch when cut from one; it resumes a saved state.json and exits non-zero short of Shipped.
func Drive(options Options) (int, error) {
	markLine()
	root := options.Root
	// A group resumed past Prepare has its tasks DONE already, so its open run's plan keeps closed tasks.
	plan, err := line.PlanForGroup(root, options.Target)
	if err != nil {
		return 1, err
	}
	if plan == nil {
		return 1, errors.New("nothing is ready")
	}
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	runState, err := cutIfNeeded(root, plan, stdout)
	if err != nil {
		return 1, err
	}
	worktree := line.WorktreePath(root, plan.Worktree)
	contract, err := driverContract(root, worktree)
	if err != nil {
		return 1, err
	}
	driver, err := newDriver(root, plan, runState.Run, contract, options.PR)
	if err != nil {
		return 1, err
	}
	budget := options.Budget
	if budget <= 0 {
		budget = GroupBudget
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	// An interrupt cancels the drive, which stops the running session, so no session outlives its conductor.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	statePath := conductor.StatePath(root, plan.Group)
	final, err := driveState(ctx, driver, statePath, plan, worktree)
	if saveErr := conductor.SaveState(statePath, final); saveErr != nil && err == nil {
		err = saveErr
	}
	if err != nil {
		return 1, err
	}
	if final.Current != conductor.Shipped {
		return 1, fmt.Errorf("%s stopped at %s, not Shipped", plan.Group, final.Current)
	}
	if err := pruneLeftovers(root, plan.Base, stdout); err != nil {
		return 1, err
	}
	return 0, nil
}

// driveState resumes a group from its saved state.json, or starts it fresh at Ready with its
// slot already taken, since a targeted run never waits on another group's slot.
func driveState(
	ctx context.Context, driver *conductor.Driver, statePath string, plan *line.Plan, worktree string,
) (conductor.State, error) {
	saved, err := conductor.LoadState(statePath)
	// A blocked group a person edited restarts with its slot taken, its builder reading the edited group first.
	if err == nil && saved.Current == conductor.Blocked && saved.Edited {
		if text, ok := editedGroup(worktree, plan.Group); ok {
			driver.Builder.Brief = "# The group, as a person edited it\n\n" + text + "\n\n" + driver.Builder.Brief
		}
		saved.Current, saved.SlotFree, saved.Edited = conductor.Ready, true, false
		return driver.Drive(ctx, saved)
	}
	if err == nil {
		return driver.Resume(ctx, saved)
	}
	fresh := conductor.State{
		Group: plan.Group, Worktree: worktree, Branch: plan.Branch, Current: conductor.Ready, SlotFree: true,
	}
	return driver.Drive(ctx, fresh)
}

// cutIfNeeded returns the group's own run, clearing leftovers then cutting its branch and worktree when
// none is open yet, and fills the plan's base, branch and worktree from whichever run it finds.
func cutIfNeeded(root string, plan *line.Plan, out io.Writer) (line.RunState, error) {
	if state, err := line.LoadRunFor(root, plan.Group); err == nil && state.Group == plan.Group {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
		return state, nil
	}
	if err := pruneLeftovers(root, plan.Base, out); err != nil {
		return line.RunState{}, err
	}
	return line.Start(root, plan, "", false)
}

// pruneLeftovers removes merged and abandoned groups' worktrees and branches and the oldest run folders,
// printing each removal to out.
func pruneLeftovers(root, base string, out io.Writer) error {
	if base == "" {
		base = line.DefaultBase(root)
	}
	done, err := doctor.Prune(root, base)
	if err != nil {
		return err
	}
	for _, item := range done {
		fmt.Fprintln(out, item)
	}
	return nil
}

// driverContract resolves the profile's host and builds its session driver over worktree.
func driverContract(root, worktree string) (mount.Contract, error) {
	selected := profile.Select(root)
	host, ok := mount.Get(selected.Host)
	if !ok || host.Contract == nil {
		return nil, errors.New("no mount is installed here; run komodo install")
	}
	return host.Contract(root, worktree), nil
}

// newDriver builds the conductor's driver for one group: its host, its requests, its wrapped
// stations, and the shared ledger, saving every move to the group's own state.json.
func newDriver(root string, plan *line.Plan, run string, contract mount.Contract, client *pr.Client) (*conductor.Driver, error) {
	builder, err := BuilderRequest(root, plan)
	if err != nil {
		return nil, err
	}
	worktree := line.WorktreePath(root, plan.Worktree)
	if client == nil {
		client = pr.New(worktree)
	}
	heavy := plan.Profile.Tiers.Heavy
	if machine, ok := plan.Profile.Machine("builder"); ok {
		heavy.Effort = machine.Effort
	}
	stations := &conductor.Line{Root: root, Plan: plan, Client: client}
	return &conductor.Driver{
		Host:     contract,
		Stations: stations,
		Block:    stations.Block,
		Ledger:   line.Book(root),
		Run:      run,
		Builder:  builder,
		Lenses:   review.ForMode(plan.Profile.Mode),
		// A lens's brief carries the diff, so it is built at review, after the build is committed.
		Review: func(lens review.Lens) (mount.StartRequest, error) {
			if err := line.CommitBuild(root, plan); err != nil {
				return mount.StartRequest{}, fmt.Errorf("committing the build for review: %w", err)
			}
			return ReviewerRequest(root, plan, lens)
		},
		// A re-review diffs from the last reviewed HEAD, so the repair is committed first.
		ReReview: func(lens review.Lens, s conductor.State) (string, error) {
			if err := line.CommitBuild(root, plan); err != nil {
				return "", fmt.Errorf("committing the repair for re-review: %w", err)
			}
			return ReReviewInput(root, plan, s, lens)
		},
		SeverityFloor: plan.Profile.SeverityFloor,
		Tasks:         plan.Tasks,
		Save: func(s conductor.State) error {
			return conductor.SaveState(conductor.StatePath(root, plan.Group), s)
		},
		WriteReview: func(group string, result mount.Result) error {
			return writeReview(root, group, result)
		},
		Orchestrator: func(e conductor.Escalation) (mount.StartRequest, error) {
			return orchestratorRequest(root, plan, e)
		},
		Heavy: heavy,
		Lint: func() ([]string, error) {
			return lintBacklog(worktree)
		},
	}, nil
}

// orchestratorRequest fills the headless orchestrator's start request for one escalation: its role's
// template with the group, the state it left, why, and each task's files, on the orchestrator's machine.
func orchestratorRequest(root string, plan *line.Plan, e conductor.Escalation) (mount.StartRequest, error) {
	definition, err := line.LoadRole(root, "orchestrator")
	if err != nil {
		return mount.StartRequest{}, err
	}
	tasks := make([]string, 0, len(plan.Tasks))
	for _, task := range plan.Tasks {
		tasks = append(tasks, fmt.Sprintf("- %s %s: `%s`", task.ID, task.Title, strings.Join(task.Files, "`, `")))
	}
	brief := strings.NewReplacer(
		"{{group}}", e.Group, "{{branch}}", plan.Branch, "{{left}}", string(e.Left),
		"{{reason}}", e.Reason, "{{tasks}}", strings.Join(tasks, "\n"),
	).Replace(definition.Body)
	machine, _ := plan.Profile.Machine("orchestrator")
	return mount.StartRequest{
		Role:   "orchestrator",
		Brief:  brief,
		Tools:  definition.Tools,
		Model:  machine.Model,
		Effort: machine.Effort,
		Schema: []byte(line.SchemaText(root, "orchestrator")),
	}, nil
}

// editedGroup is the group's section of the worktree's backlog, as a person edited it on the group's branch.
func editedGroup(worktree, group string) (string, bool) {
	path, err := backlog.Find(worktree)
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return backlog.GroupText(string(data), group)
}

// lintBacklog returns the worktree backlog's lint problems, as komodo lint reports them: BACKLOG.md's when it
// exists, else each docs/backlog group file's own.
func lintBacklog(worktree string) ([]string, error) {
	if path, err := backlog.Find(worktree); err == nil {
		parsed, err := backlog.Load(path)
		if err != nil {
			return nil, err
		}
		return append(backlog.Lint(parsed), backlog.LintContext(worktree, parsed)...), nil
	}
	files, err := filepath.Glob(filepath.Join(worktree, "docs", "backlog", "*.md"))
	if err != nil {
		return nil, err
	}
	var problems []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		problems = append(problems, backlog.ParseGroupFile(string(data)).Problems...)
	}
	return problems, nil
}

// writeReview saves the reviewer's whole result, findings with their files and titles, where
// ShipGroup reads it, so its findings gate and the findings it files match what the reviewer said.
func writeReview(root, group string, result mount.Result) error {
	data, err := json.Marshal(result.Value)
	if err != nil {
		return err
	}
	path := line.ResultPath(root, group+"-review")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

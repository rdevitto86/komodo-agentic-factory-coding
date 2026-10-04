package conductor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/line"
)

// needsDefault is what a stop needs when neither the orchestrator nor the conductor named it.
const needsDefault = "a person's decision on the question above"

// stop writes down what the orchestrator could not settle: the blocker note naming the state the group left,
// why it stopped, and what it needs, which Block commits and publishes; with no Block wired it waits.
func (d *Driver) stop(ctx context.Context, s *State, r *round) error {
	if d.Block == nil {
		return nil
	}
	needs := r.needs
	if needs == "" {
		needs = needsDefault
	}
	return d.Block(ctx, backlog.BlockerNote{
		At: time.Now(), Run: d.Run, State: string(s.Left), Items: []string{r.reason}, Needs: needs,
	})
}

// Block saves the group's work as a WIP commit, writes its blocker note, and publishes a draft PR
// labelled status: blocked; a note that never publishes fails, one missing only its label warns.
func (l *Line) Block(ctx context.Context, note backlog.BlockerNote) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result, err := line.ShipBlocked(l.Root, l.Plan, note, l.Client)
	if err != nil {
		return err
	}
	if result.URL == "" && len(result.Warnings) > 0 {
		return fmt.Errorf("%s's blocker note stayed local: %s", l.Plan.Group, strings.Join(result.Warnings, "; "))
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "komodo: %s's blocker note: %s\n", l.Plan.Group, warning)
	}
	return nil
}

package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"komodo/internal/line"
)

// hedge matches the phrases that turn a verdict into a guess.
var hedge = regexp.MustCompile(`(?i)\b(might|maybe|perhaps|probably|possibly|hopefully|should work|i think|i believe|it seems)\b`)

// Hedges lists each hedging phrase text uses, in order.
func Hedges(text string) []string {
	return hedge.FindAllString(text, -1)
}

// actsWithoutAsking runs a group whose every task is fully specified and fails on any question asked or any
// hedge in a result: the right move there is to act and state the verdict.
func actsWithoutAsking(ctx context.Context, env Env) error {
	if ran := env.Komodo(ctx, nil, "run", env.Group()); ran.Code != 0 {
		return fmt.Errorf("the run exited %d: %s", ran.Code, ran.Output)
	}
	paths, err := filepath.Glob(filepath.Join(env.Dir(), line.StateDir, "results", "TSK-*.json"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("the run left no task results to score")
	}
	var problems []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var result struct {
			Result   string   `json:"result"`
			Summary  string   `json:"summary"`
			Question string   `json:"question"`
			Notes    []string `json:"notes"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if result.Result == "BLOCKED" || strings.TrimSpace(result.Question) != "" {
			problems = append(problems, fmt.Sprintf("%s asked %q instead of acting", filepath.Base(path), result.Question))
		}
		if found := Hedges(result.Summary + "\n" + strings.Join(result.Notes, "\n")); len(found) > 0 {
			problems = append(problems, fmt.Sprintf("%s hedged: %s", filepath.Base(path), strings.Join(found, ", ")))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d unneeded question(s) or hedge(s); the target is zero:\n%s", len(problems), strings.Join(problems, "\n"))
	}
	return nil
}

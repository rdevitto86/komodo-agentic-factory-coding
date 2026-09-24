package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/profile"
	"komodo/internal/recall"
	"komodo/internal/toolkit"
)

// runMetrics prints what the two ledger files hold.
func runMetrics(root string) {
	entries, err := line.Book(root).All()
	if err != nil {
		fail(err)
	}
	fmt.Print(ledger.Render(ledger.Aggregate(entries)))
}

// runThreads prints the unresolved review threads on a pull request, or resolves one by id.
func runThreads(root string, args []string) {
	set := flag.NewFlagSet("threads", flag.ExitOnError)
	resolve := set.String("resolve", "", "resolve the review thread with this id")
	number, rest := splitPositional(args, "resolve")
	_ = set.Parse(rest)
	client := pr.New(root)
	if *resolve != "" {
		if err := client.Resolve(*resolve); err != nil {
			fail(err)
		}
		fmt.Println("resolved", *resolve)
		return
	}
	threads, err := client.Threads(number)
	if err != nil {
		fail(err)
	}
	if threads == nil {
		threads = []pr.Thread{}
	}
	printJSON(threads)
}

// runMachine posts one task's brief to the local machine, writes the result, and stamps the ledger.
func runMachine(root string, args []string) {
	set := flag.NewFlagSet("machine", flag.ExitOnError)
	role := set.String("role", "builder", "the role the brief was written for")
	taskID, rest := splitTaskArg(args)
	if taskID == "" {
		fail(fmt.Errorf("usage: komodo machine <task> [--role reviewer]"))
	}
	_ = set.Parse(rest)
	definition, err := line.LoadRole(root, *role)
	if err != nil {
		fail(err)
	}
	local := mount.LocalMachine()
	if !local.Allowed(definition.Tools) {
		fail(fmt.Errorf("%s writes, and a write role cannot run on the local machine; step routes it to a remote one instead", *role))
	}
	brief, err := os.ReadFile(filepath.Join(root, line.StateDir, "briefs", taskID+".md"))
	if err != nil {
		fail(err)
	}
	schema, err := fs.ReadFile(toolkit.FS(root), path.Join("roles", definition.Returns))
	if err != nil {
		fail(err)
	}
	tier := definition.Tier
	if *role == "reviewer" {
		tier = "reviewer"
	}
	model, err := localModel(profile.Select(root).Tiers, tier)
	if err != nil {
		fail(err)
	}
	started := time.Now()
	result, err := local.Post(model, string(brief), schema)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(line.ResultPath(root, taskID)), 0o755); err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(result.Value, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(line.ResultPath(root, taskID), data, 0o644); err != nil {
		fail(err)
	}
	entry := ledger.Entry{
		Task: taskID, Station: "machine", Role: *role, Tier: definition.Tier,
		Provider: mount.LocalName, Model: model, Seconds: time.Since(started).Seconds(),
		TokensIn: result.TokensIn, TokensOut: result.TokensOut, Outcome: "done",
	}
	if state, err := line.LoadRun(root); err == nil {
		entry.Run, entry.Group = state.Run, state.Group
	}
	if err := line.Book(root).Stamp(entry); err != nil {
		fail(err)
	}
	fmt.Println("wrote", line.ResultPath(root, taskID))
}

// runRecall scores the local machine's reviewer against the seeded bugs and records the score by model.
func runRecall(root string, args []string) {
	set := flag.NewFlagSet("recall", flag.ExitOnError)
	model := set.String("model", "", "the local model to score, the local machine's own when empty")
	_ = set.Parse(args)
	local := mount.LocalMachine()
	if *model == "" {
		*model = local.ModelName()
	}
	if *model == "" {
		fail(fmt.Errorf("no local model to score; pass --model"))
	}
	score, err := recall.Run(root, *model, local.Post)
	if err != nil {
		fail(err)
	}
	fmt.Printf("%s: caught %d/%d, recall %.2f, %d false findings on the clean cases\n",
		*model, score.Caught, score.Cases, score.Recall, score.FalsePositives)
	if err := recall.Save(recall.Path(), *model, score); err != nil {
		fail(err)
	}
	fmt.Println("wrote", recall.Path())
}

// localModel resolves a role's tier to the model of whichever tier the local machine mounts;
// reviewer resolves through Tiers.Reviewer, which Tiers.Machine does not know.
func localModel(tiers mount.Tiers, tier string) (string, error) {
	machine := tiers.Machine(tier)
	if tier == "reviewer" {
		machine = tiers.Reviewer
	}
	if machine.Local() {
		return machine.Model, nil
	}
	for _, fallback := range []string{"light", "standard", "heavy"} {
		if candidate := tiers.Machine(fallback); candidate.Local() {
			return "", fmt.Errorf("%s tier does not mount the local machine; %s does, and komodo machine does not switch tiers", tier, fallback)
		}
	}
	return "", fmt.Errorf("no tier mounts the local machine")
}

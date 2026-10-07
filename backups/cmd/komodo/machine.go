package main

import (
	"flag"
	"fmt"

	"komodo/internal/harness"
	"komodo/internal/ledger"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/recall"
)

// runMetrics prints what the two ledger files hold.
func runMetrics(root string) {
	entries, err := harness.Book(root).All()
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
	if err := recall.Save(mount.RecallPath(), *model, score); err != nil {
		fail(err)
	}
	fmt.Println("wrote", mount.RecallPath())
}

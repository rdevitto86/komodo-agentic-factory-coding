package main

import (
	"flag"
	"fmt"

	"komodo/internal/backlog"
	"komodo/internal/ingest"
	"komodo/internal/line"
)

// runIngest compiles every READY group into its card under .komodo/queue, or one named group;
// --json prints the compiled cards instead of one summary line per group.
func runIngest(root string, args []string) {
	set := flag.NewFlagSet("ingest", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print the compiled cards")
	needle, rest := splitPositional(args)
	_ = set.Parse(rest)
	parsed, _, err := line.LoadBacklog(root)
	if err != nil {
		fail(err)
	}
	groups := ingest.ReadyGroups(parsed)
	if needle != "" {
		group, ok := parsed.Group(needle)
		if !ok {
			fail(fmt.Errorf("no group matching %q", needle))
		}
		groups = []backlog.Group{group}
	}
	cards := make([]ingest.Card, 0, len(groups))
	for _, group := range groups {
		card, err := ingest.Write(root, parsed, group)
		if err != nil {
			fail(err)
		}
		cards = append(cards, card)
	}
	if *asJSON {
		printJSON(cards)
		return
	}
	for _, card := range cards {
		fmt.Printf("%s %s %d task(s) %d file(s) %s\n",
			card.Group, card.Hash[:12], card.Size.Tasks, card.Size.Files, ingest.Path(card.Group))
	}
	fmt.Printf("%d card(s)\n", len(cards))
}

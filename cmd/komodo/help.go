package main

import (
	"flag"
	"fmt"
	"strings"
)

// runHelp prints the usage, or with --skill the komodo skill generated from it.
func runHelp(args []string) {
	set := flag.NewFlagSet("help", flag.ExitOnError)
	skill := set.Bool("skill", false, "print the komodo skill generated from this help")
	_ = set.Parse(args)
	if *skill {
		fmt.Print(helpSkill())
		return
	}
	fmt.Print(usage)
}

// helpSkill renders the usage as the komodo skill, so an agent's command reference matches this binary.
func helpSkill() string {
	commands := strings.TrimSpace(strings.SplitN(usage, "\n", 2)[1])
	var out strings.Builder
	out.WriteString("---\nname: komodo\ndescription: The komodo command reference, generated from komodo help.\n---\n\n")
	out.WriteString("# komodo\n\n")
	out.WriteString("Every command the binary takes. `komodo help --skill` writes this file; never edit it by hand.\n\n")
	out.WriteString("```\n  " + commands + "\n```\n")
	return out.String()
}

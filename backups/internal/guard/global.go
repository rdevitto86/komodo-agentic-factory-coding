package guard

// Suite is one role's own commands, table rows, and hooks, layered after the global suite that
// always runs first; Hooks names rows of the hooks package's own table, not this package's.
type Suite struct {
	Name     string
	Commands []string
	Rows     []Case
	Hooks    []string
}

// suiteRegistry holds every suite a role file registers, global included.
var suiteRegistry = map[string]Suite{}

// registerSuite adds one suite to the registry; each role file calls this from its own init.
func registerSuite(s Suite) {
	suiteRegistry[s.Name] = s
}

// roleOrder is every role suite's own name, global apart, orchestrator first since it spawns the rest.
var roleOrder = []string{"orchestrator", "builder", "reviewer", "architect", "planner", "tester", "scout", "researcher"}

// For returns the suite role runs under; an empty role and "orchestrator" both name the orchestrator.
func For(role string) (Suite, bool) {
	if role == "" {
		role = "orchestrator"
	}
	suite, ok := suiteRegistry[role]
	return suite, ok
}

// Names lists every role suite's name, in the order roles were added, global apart.
func Names() []string {
	out := make([]string, len(roleOrder))
	copy(out, roleOrder)
	return out
}

// Global returns the suite that runs first for every role: the git and config rules none escapes.
func Global() Suite {
	return suiteRegistry["global"]
}

// init registers the global suite from the default policy; a repo or machine policy still widens
// what Check refuses at runtime, but this snapshot is what Names and tests read.
func init() {
	registerSuite(Suite{
		Name:  "global",
		Hooks: []string{"guard"},
		Rows:  globalRows(DefaultPolicy()),
	})
}

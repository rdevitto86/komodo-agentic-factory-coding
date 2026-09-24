package guard

import (
	"fmt"
	"regexp"
	"strings"

	"komodo/internal/mount"
)

// leakFinding is reported when a commit, pull request, issue, or comment carries private text.
const leakFinding = "a session link or trailer never leaves the machine"

// ghFindings refuses a gh call that writes to the forge, except the two writes the line needs,
// and any call whose outgoing text carries a private pattern or a trailer.
func ghFindings(kept []string, expanding map[string]bool, policy Policy, cwd, stdin string) []string {
	if len(kept) < 2 {
		return nil
	}
	findings := forgeFindings(kept, expanding)
	if text := outgoingText(kept, cwd, stdin); hasPrivate(text) || policy.HasTrailer(normalizeMessage(text)) {
		findings = append(findings, leakFinding)
	}
	return findings
}

// hasPrivate reports whether text holds a pattern a registered mount considers private.
func hasPrivate(text string) bool {
	if text == "" {
		return false
	}
	for _, pattern := range mount.GuardPrivatePatterns() {
		if compiled, err := regexp.Compile(pattern); err == nil && compiled.MatchString(text) {
			return true
		}
	}
	return false
}

// bodyVerbs are the gh pr and issue subcommands whose body text reaches the forge.
var bodyVerbs = map[string]map[string]bool{
	"pr":    {"create": true, "edit": true, "comment": true, "review": true},
	"issue": {"create": true, "comment": true},
}

// outgoingText is what a gh call sends to the forge: a pr or issue body, or an api payload.
func outgoingText(kept []string, cwd, stdin string) string {
	switch kept[1] {
	case "api":
		return apiText(kept[2:], cwd, stdin)
	case "pr", "issue":
		if rest := afterRepoFlags(kept[2:]); len(rest) > 0 && bodyVerbs[kept[1]][rest[0]] {
			return ghBody(rest[1:], cwd, stdin)
		}
	}
	return ""
}

// ghBody composes a pr or issue body from every --body and --body-file, as commitMessage reads -m.
func ghBody(args []string, cwd, stdin string) string {
	var parts []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-b" || arg == "--body":
			if index+1 < len(args) {
				index++
				parts = append(parts, args[index])
			}
		case strings.HasPrefix(arg, "--body="):
			parts = append(parts, strings.TrimPrefix(arg, "--body="))
		case arg == "-F" || arg == "--body-file":
			if index+1 < len(args) {
				index++
				parts = append(parts, readMessageFile(args[index], cwd, stdin))
			}
		case strings.HasPrefix(arg, "--body-file="):
			parts = append(parts, readMessageFile(strings.TrimPrefix(arg, "--body-file="), cwd, stdin))
		case strings.HasPrefix(arg, "-b") && !strings.HasPrefix(arg, "--"):
			parts = append(parts, strings.TrimPrefix(arg[2:], "="))
		case strings.HasPrefix(arg, "-F") && !strings.HasPrefix(arg, "--"):
			parts = append(parts, readMessageFile(strings.TrimPrefix(arg[2:], "="), cwd, stdin))
		}
	}
	return strings.Join(parts, "\n\n")
}

// apiText composes what a gh api call sends: every field's value, an @file field's content, and --input's.
func apiText(args []string, cwd, stdin string) string {
	var parts []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		name, value, hasEq := splitGhFlag(arg)
		switch {
		case name == "-f" || name == "-F" || name == "--field" || name == "--raw-field":
			var field string
			field, index = nextValue(args, index, value, hasEq)
			_, text, _ := strings.Cut(field, "=")
			parts = append(parts, text)
			if (name == "-F" || name == "--field") && strings.HasPrefix(text, "@") {
				parts = append(parts, readMessageFile(text[1:], cwd, stdin))
			}
		case name == "--input":
			var file string
			file, index = nextValue(args, index, value, hasEq)
			parts = append(parts, readMessageFile(file, cwd, stdin))
		case strings.HasPrefix(arg, "-"):
			if apiValueFlags[name] && !hasEq && index+1 < len(args) {
				index++
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// forgeFindings refuses a gh call that writes to the forge, except the writes the line needs.
func forgeFindings(kept []string, expanding map[string]bool) []string {
	switch kept[1] {
	case "pr":
		if rest := afterRepoFlags(kept[2:]); len(rest) > 0 && rest[0] == "merge" {
			return []string{"gh pr merge: landing is the human's merge button"}
		}
	case "api":
		return apiFindings(kept[2:], expanding)
	case "ruleset":
		return rulesetFindings(afterRepoFlags(kept[2:]))
	case "repo":
		return repoFindings(afterRepoFlags(kept[2:]))
	case "secret":
		return secretFindings(afterRepoFlags(kept[2:]))
	}
	return nil
}

// expandingIn reports whether any of the given argument strings came from a word the shell expands.
func expandingIn(expanding map[string]bool, values ...string) bool {
	for _, value := range values {
		if expanding[value] {
			return true
		}
	}
	return false
}

// unresolvedText reports whether text still holds a substitution, backtick, or variable the shell fills in.
func unresolvedText(text string) bool {
	return strings.Contains(text, "$(") || strings.Contains(text, "`") || unresolvedVarRe.MatchString(text)
}

// afterRepoFlags drops the -R or --repo flags gh accepts before a subcommand's verb.
func afterRepoFlags(args []string) []string {
	for len(args) > 0 {
		switch {
		case args[0] == "-R" || args[0] == "--repo":
			if len(args) < 2 {
				return nil
			}
			args = args[2:]
		case strings.HasPrefix(args[0], "--repo=") || (strings.HasPrefix(args[0], "-R") && len(args[0]) > 2):
			args = args[1:]
		default:
			return args
		}
	}
	return args
}

// apiValueFlags are gh api's own options that consume the next word, besides the ones read below.
var apiValueFlags = map[string]bool{
	"-H": true, "--header": true, "--cache": true, "-p": true, "--preview": true,
	"-q": true, "--jq": true, "-t": true, "--template": true,
}

// pullsCreateRe matches the one POST the line needs to open a pull request.
var pullsCreateRe = regexp.MustCompile(`^/?repos/[^/]+/[^/]+/pulls(\?.*)?$`)

// commentsCreateRe matches the POSTs the respond skill needs: an issue, a pull request, or a reply comment.
var commentsCreateRe = regexp.MustCompile(`^/?repos/[^/]+/[^/]+/(issues/[0-9]+/comments|pulls/[0-9]+/comments|pulls/[0-9]+/comments/[0-9]+/replies)(\?.*)?$`)

// sensitiveSegments names the path segments of the forge no gh api call reaches, whatever its method.
var sensitiveSegments = map[string]bool{
	"protection": true, "rulesets": true, "merge": true, "merges": true, "hooks": true,
	"keys": true, "secrets": true, "collaborators": true,
}

// apiFindings refuses a gh api call that writes, reaches a sensitive endpoint, or crosses to another host.
func apiFindings(args []string, expanding map[string]bool) []string {
	var method, hostname, endpoint, graphqlQuery string
	hasFieldFlag, queryFromFile := false, false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		name, value, hasEq := splitGhFlag(arg)
		switch {
		case name == "-X" || name == "--method":
			method, index = nextValue(args, index, value, hasEq)
		case name == "--hostname":
			hostname, index = nextValue(args, index, value, hasEq)
		case name == "-f" || name == "-F" || name == "--field" || name == "--raw-field":
			hasFieldFlag = true
			var field string
			field, index = nextValue(args, index, value, hasEq)
			if key, val, ok := strings.Cut(field, "="); ok && key == "query" {
				// gh sends the last query= it is given, so only the last one counts.
				graphqlQuery = val
				queryFromFile = strings.HasPrefix(val, "@") || (expandingIn(expanding, arg, field) && unresolvedText(val))
			}
		case name == "--input":
			hasFieldFlag = true
			queryFromFile = true
			if !hasEq && index+1 < len(args) {
				index++
			}
		case strings.HasPrefix(arg, "-"):
			if apiValueFlags[name] && !hasEq && index+1 < len(args) {
				index++
			}
		default:
			if endpoint == "" {
				endpoint = arg
			}
		}
	}
	if endpoint == "graphql" {
		if _, saw := mutationFields(graphqlQuery); hostname != "" && saw {
			return []string{fmt.Sprintf("gh api --hostname %s graphql: a forge write goes through the line, never an agent", hostname)}
		}
		if queryFromFile {
			return []string{"gh api graphql: a query read from a file or a substitution is not visible to the guard; pass it literally with -f query="}
		}
		if fields, saw := mutationFields(graphqlQuery); saw && !allAllowed(fields) {
			return []string{"gh api graphql: a forge write goes through the line, never an agent"}
		}
		return nil
	}
	effective := strings.ToUpper(method)
	if effective == "" {
		effective = "GET"
		if hasFieldFlag {
			effective = "POST"
		}
	}
	isWrite := effective != "GET" && effective != "HEAD"
	if hostname != "" && isWrite {
		return []string{fmt.Sprintf("gh api --hostname %s %s %s: a forge write goes through the line, never an agent", hostname, effective, endpoint)}
	}
	if endpointNamesSensitivePart(endpoint) {
		return []string{fmt.Sprintf("gh api %s %s: a forge write goes through the line, never an agent", effective, endpoint)}
	}
	if isWrite && !pullsCreateRe.MatchString(endpoint) && !commentsCreateRe.MatchString(endpoint) {
		return []string{fmt.Sprintf("gh api %s %s: a forge write goes through the line, never an agent", effective, endpoint)}
	}
	return nil
}

// ghGluedFlags are gh api's short options that take a value, which pflag also reads glued, as -XDELETE.
var ghGluedFlags = map[byte]bool{'X': true, 'f': true, 'F': true, 'H': true, 'p': true, 'q': true, 't': true}

// splitGhFlag splits one gh api argument into its flag and value, walking a short cluster the way
// pflag does: -iXPUT is -i then -X PUT, and a bare trailing value flag takes the next word.
func splitGhFlag(arg string) (name, value string, hasValue bool) {
	if strings.HasPrefix(arg, "--") {
		return strings.Cut(arg, "=")
	}
	if len(arg) < 2 || arg[0] != '-' {
		return arg, "", false
	}
	for position := 1; position < len(arg); position++ {
		letter := arg[position]
		if ghGluedFlags[letter] {
			rest := strings.TrimPrefix(arg[position+1:], "=")
			return "-" + string(letter), rest, rest != ""
		}
		if !ghBoolFlags[letter] {
			return arg, "", false
		}
	}
	return arg, "", false
}

// ghBoolFlags are gh api's short options that take no value and may lead a cluster, as -i does.
var ghBoolFlags = map[byte]bool{'i': true}

// nextValue reads a flag's value from its =value form, or from the word that follows it.
func nextValue(args []string, index int, value string, hasEq bool) (string, int) {
	if hasEq {
		return value, index
	}
	if index+1 < len(args) {
		return args[index+1], index + 1
	}
	return "", index
}

// endpointNamesSensitivePart reports whether an endpoint touches a part of the forge no call
// reaches, matching whole path segments after repos/<owner>/<repo>, so a repo's own name never counts.
func endpointNamesSensitivePart(endpoint string) bool {
	path, _, _ := strings.Cut(strings.ToLower(strings.TrimPrefix(endpoint, "/")), "?")
	segments := strings.Split(path, "/")
	if len(segments) >= 3 && segments[0] == "repos" {
		segments = segments[3:]
	}
	for index, segment := range segments {
		if sensitiveSegments[segment] {
			return true
		}
		if segment == "git" && index+1 < len(segments) && segments[index+1] == "refs" {
			return true
		}
	}
	return false
}

// allowedMutations are the writes the line and the respond skill need through gh api graphql.
var allowedMutations = map[string]bool{
	"addComment": true, "addPullRequestReviewComment": true, "addPullRequestReviewThreadReply": true,
	"resolveReviewThread": true, "createPullRequest": true,
}

// allAllowed reports whether a mutation names at least one field and every field is one the line allows.
func allAllowed(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if !allowedMutations[field] {
			return false
		}
	}
	return true
}

// mutationFields returns every mutation operation's top-level fields and whether any operation is a
// mutation, reading the keyword and its selection set at depth zero, past strings and # comments.
func mutationFields(document string) (fields []string, saw bool) {
	depth := 0
	inMutation := false
	for index := 0; index < len(document); index++ {
		char := document[index]
		switch {
		case char == '"':
			index = skipGraphQLString(document, index)
		case char == '#':
			for index+1 < len(document) && document[index+1] != '\n' {
				index++
			}
		case char == '(':
			depth++
		case char == ')':
			depth--
		case char == '{':
			if depth == 0 && inMutation {
				fields = append(fields, selectionNames(document[index+1:])...)
				inMutation, saw = false, true
			}
			depth++
		case char == '}':
			depth--
		case depth == 0 && isNameStart(char):
			end := index
			for end < len(document) && isNameChar(document[end]) {
				end++
			}
			switch name := document[index:end]; name {
			case "mutation":
				inMutation, saw = true, true
			case "query", "subscription", "fragment":
				inMutation = false
			}
			index = end - 1
		}
	}
	return fields, saw
}

// selectionNames reads the top-depth field names of one selection set, skipping strings and # comments.
func selectionNames(body string) []string {
	var names []string
	depth := 0
	for index := 0; index < len(body); index++ {
		char := body[index]
		switch {
		case char == '"':
			index = skipGraphQLString(body, index)
		case char == '#':
			for index+1 < len(body) && body[index+1] != '\n' {
				index++
			}
		case char == '{' || char == '(':
			depth++
		case char == '}' || char == ')':
			if depth == 0 {
				return names
			}
			depth--
		case depth == 0 && isNameStart(char):
			end := index
			for end < len(body) && isNameChar(body[end]) {
				end++
			}
			name := body[index:end]
			next := strings.TrimLeft(body[end:], " \t\r\n,")
			if !strings.HasPrefix(next, ":") {
				names = append(names, name)
			}
			index = end - 1
		}
	}
	return names
}

// skipGraphQLString returns the index of the quote that closes the string opening at index.
func skipGraphQLString(body string, index int) int {
	if strings.HasPrefix(body[index:], `"""`) {
		if end := strings.Index(body[index+3:], `"""`); end >= 0 {
			return index + 3 + end + 2
		}
		return len(body)
	}
	for next := index + 1; next < len(body); next++ {
		switch body[next] {
		case '\\':
			next++
		case '"':
			return next
		}
	}
	return len(body)
}

// isNameStart reports whether a byte can open a GraphQL name.
func isNameStart(char byte) bool {
	return char == '_' || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

// isNameChar reports whether a byte can continue a GraphQL name.
func isNameChar(char byte) bool {
	return isNameStart(char) || (char >= '0' && char <= '9')
}

// rulesetReadVerbs are the gh ruleset subcommands that only read.
var rulesetReadVerbs = map[string]bool{"list": true, "view": true, "check": true}

// rulesetFindings refuses a gh ruleset call that carries a write verb.
func rulesetFindings(args []string) []string {
	if len(args) == 0 || rulesetReadVerbs[args[0]] {
		return nil
	}
	return []string{fmt.Sprintf("gh ruleset %s: a forge write goes through the line, never an agent", args[0])}
}

// repoWriteVerbs are the gh repo subcommands that change the repository itself or its branches.
var repoWriteVerbs = map[string]bool{
	"edit": true, "delete": true, "rename": true, "archive": true, "unarchive": true, "sync": true,
}

// deployKeyWriteVerbs are the gh repo deploy-key subcommands that add or remove a key.
var deployKeyWriteVerbs = map[string]bool{"add": true, "delete": true}

// repoFindings refuses a gh repo call that changes the repository, its branches, or its deploy keys.
func repoFindings(args []string) []string {
	switch {
	case len(args) == 0:
		return nil
	case args[0] == "deploy-key":
		rest := afterRepoFlags(args[1:])
		if len(rest) == 0 || !deployKeyWriteVerbs[rest[0]] {
			return nil
		}
		return []string{fmt.Sprintf("gh repo deploy-key %s: a forge write goes through the line, never an agent", rest[0])}
	case repoWriteVerbs[args[0]]:
		return []string{fmt.Sprintf("gh repo %s: a forge write goes through the line, never an agent", args[0])}
	}
	return nil
}

// secretWriteVerbs are the gh secret subcommands that set or remove a secret.
var secretWriteVerbs = map[string]bool{"set": true, "delete": true, "remove": true}

// secretFindings refuses a gh secret call that sets or deletes a secret.
func secretFindings(args []string) []string {
	if len(args) == 0 || !secretWriteVerbs[args[0]] {
		return nil
	}
	return []string{fmt.Sprintf("gh secret %s: a forge write goes through the line, never an agent", args[0])}
}

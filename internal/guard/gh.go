package guard

import (
	"fmt"
	"regexp"
	"strings"
)

// ghFindings refuses a gh call that writes to the forge, since a forge write goes through the
// line, never an agent; it stays silent on every read and on the two writes the line itself needs.
func ghFindings(kept []string) []string {
	if len(kept) < 2 {
		return nil
	}
	switch kept[1] {
	case "pr":
		if len(kept) > 2 && kept[2] == "merge" {
			return []string{"gh pr merge: landing is the human's merge button"}
		}
	case "api":
		return apiFindings(kept[2:])
	case "ruleset":
		return rulesetFindings(kept[2:])
	case "repo":
		return repoFindings(kept[2:])
	case "secret":
		return secretFindings(kept[2:])
	}
	return nil
}

// apiValueFlags are gh api's own options that consume the next word, besides the ones read below.
var apiValueFlags = map[string]bool{
	"-H": true, "--header": true, "--cache": true, "-p": true, "--preview": true,
	"-q": true, "--jq": true, "-t": true, "--template": true,
}

// pullsCreateRe matches the one POST the line needs to open a pull request.
var pullsCreateRe = regexp.MustCompile(`^/?repos/[^/]+/[^/]+/pulls(\?.*)?$`)

// commentsCreateRe matches the POSTs the respond skill needs: an issue, a pull request, or a reply comment.
var commentsCreateRe = regexp.MustCompile(`^/?repos/[^/]+/[^/]+/(issues/[0-9]+/comments|pulls/[0-9]+/comments|comments/[0-9]+/replies)(\?.*)?$`)

// sensitiveSegments names the path segments of the forge no gh api call reaches, whatever its method.
var sensitiveSegments = map[string]bool{
	"protection": true, "rulesets": true, "merge": true, "merges": true, "hooks": true,
	"keys": true, "secrets": true, "collaborators": true,
}

// apiFindings refuses a gh api call that writes, reaches a sensitive endpoint, or crosses to another host.
func apiFindings(args []string) []string {
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
				graphqlQuery += val
				queryFromFile = queryFromFile || strings.HasPrefix(val, "@")
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
		if queryFromFile {
			return []string{"gh api graphql: a query read from a file is not visible to the guard; pass it with -f query="}
		}
		if strings.Contains(strings.ToLower(graphqlQuery), "mutation") && !onlyAllowedMutations(graphqlQuery) {
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

// splitGhFlag splits one gh api argument into its flag and value, reading the glued short form
// -XDELETE or -fbody=x the way pflag does, and --name=value the usual way.
func splitGhFlag(arg string) (name, value string, hasValue bool) {
	if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' && ghGluedFlags[arg[1]] {
		return arg[:2], strings.TrimPrefix(arg[2:], "="), true
	}
	if strings.HasPrefix(arg, "--") {
		return strings.Cut(arg, "=")
	}
	return arg, "", false
}

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

// mutationCallRe matches a GraphQL field name immediately followed by its argument list.
var mutationCallRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// graphqlKeywords are the GraphQL syntax words a call name is never mistaken for.
var graphqlKeywords = map[string]bool{"mutation": true, "query": true, "subscription": true, "fragment": true, "on": true}

// allowedMutations are the writes the line and the respond skill need through gh api graphql.
var allowedMutations = map[string]bool{
	"addComment": true, "addPullRequestReviewComment": true, "addPullRequestReviewThreadReply": true,
	"resolveReviewThread": true, "createPullRequest": true,
}

// onlyAllowedMutations reports whether every mutation a GraphQL query names is one the line allows.
func onlyAllowedMutations(query string) bool {
	named := false
	for _, match := range mutationCallRe.FindAllStringSubmatch(query, -1) {
		name := match[1]
		if graphqlKeywords[name] {
			continue
		}
		named = true
		if !allowedMutations[name] {
			return false
		}
	}
	return named
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

// repoWriteVerbs are the gh repo subcommands that change or remove the repository itself.
var repoWriteVerbs = map[string]bool{"edit": true, "delete": true, "rename": true}

// repoFindings refuses a gh repo call that edits, deletes, or renames the repository.
func repoFindings(args []string) []string {
	if len(args) == 0 || !repoWriteVerbs[args[0]] {
		return nil
	}
	return []string{fmt.Sprintf("gh repo %s: a forge write goes through the line, never an agent", args[0])}
}

// secretWriteVerbs are the gh secret subcommands that set or remove a secret.
var secretWriteVerbs = map[string]bool{"set": true, "delete": true}

// secretFindings refuses a gh secret call that sets or deletes a secret.
func secretFindings(args []string) []string {
	if len(args) == 0 || !secretWriteVerbs[args[0]] {
		return nil
	}
	return []string{fmt.Sprintf("gh secret %s: a forge write goes through the line, never an agent", args[0])}
}

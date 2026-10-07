// Package eval runs the golden suite: pinned task groups in real repos, judged by hidden tests.
package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SuiteDir is where the real suite lives, relative to the repo root; line sessions may not touch it.
const SuiteDir = "eval"

// SuiteFile is the suite's index inside its directory.
const SuiteFile = "suite.json"

// The golden suite's size, below which it proves nothing: repos, their languages, and groups per repo.
const (
	MinRepos  = 2
	MinGroups = 10
)

// Languages are the languages the golden repos must cover.
var Languages = []string{"Go", "TypeScript"}

// ErrShort is returned when a suite holds fewer repos, languages or groups than the golden suite needs.
var ErrShort = errors.New("the suite is smaller than the golden suite must be")

// pinned matches a full commit hash, so a group names one commit and never a moving ref.
var pinned = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Suite is the golden suite: the repos and the groups pinned in them, read from Dir.
type Suite struct {
	Dir    string  `json:"-"`
	Repos  []Repo  `json:"repos"`
	Groups []Group `json:"groups"`
}

// Repo is one golden repository: its name, where to clone it from, and its language.
type Repo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Language string `json:"language"`
}

// Group is one golden group rebuilt from a merged change: its parent commit, its intent, and its hidden tests.
type Group struct {
	ID     string `json:"id"`
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
	// File is the group's backlog section, relative to the suite's directory.
	File string `json:"file"`
	// Hidden is the directory, relative to the suite's, that holds the hidden tests at their repo paths.
	Hidden      string   `json:"hidden"`
	HiddenTests []string `json:"hidden_tests"`
	// Test is the command that runs the hidden tests in the group's finished worktree.
	Test string `json:"test"`
}

// Load reads the suite in dir and fails on the first group that is not pinned, whole, and inside dir.
func Load(dir string) (Suite, error) {
	data, err := os.ReadFile(filepath.Join(dir, SuiteFile))
	if err != nil {
		return Suite{}, err
	}
	var suite Suite
	if err := json.Unmarshal(data, &suite); err != nil {
		return Suite{}, fmt.Errorf("%s: %w", SuiteFile, err)
	}
	suite.Dir = dir
	repos := make(map[string]bool, len(suite.Repos))
	for _, repo := range suite.Repos {
		if repo.Name == "" || repo.URL == "" || repo.Language == "" {
			return Suite{}, fmt.Errorf("repo %q needs a name, a url and a language", repo.Name)
		}
		if strings.HasPrefix(repo.URL, "-") {
			return Suite{}, fmt.Errorf("repo %q names a url starting with -, which git would read as an option", repo.Name)
		}
		if repos[repo.Name] {
			return Suite{}, fmt.Errorf("repo %q is named twice", repo.Name)
		}
		repos[repo.Name] = true
	}
	seen := make(map[string]bool, len(suite.Groups))
	for _, group := range suite.Groups {
		if err := suite.check(group, repos); err != nil {
			return Suite{}, fmt.Errorf("%s/%s: %w", group.Repo, group.ID, err)
		}
		key := group.Repo + "/" + group.ID
		if seen[key] {
			return Suite{}, fmt.Errorf("%s is named twice", key)
		}
		seen[key] = true
	}
	return suite, nil
}

// check fails on a group naming an unknown repo, a moving ref, or a file missing from the suite.
func (s Suite) check(group Group, repos map[string]bool) error {
	switch {
	case group.ID == "":
		return errors.New("the group has no id")
	case !repos[group.Repo]:
		return fmt.Errorf("names repo %q, which the suite does not hold", group.Repo)
	case !pinned.MatchString(group.Commit):
		return fmt.Errorf("commit %q is not a full commit hash", group.Commit)
	case len(group.HiddenTests) == 0:
		return errors.New("names no hidden tests")
	case strings.TrimSpace(group.Test) == "":
		return errors.New("names no command to run its hidden tests")
	}
	paths := []string{group.File}
	for _, test := range group.HiddenTests {
		if !filepath.IsLocal(test) {
			return fmt.Errorf("hidden test %q is not a repo path", test)
		}
		paths = append(paths, filepath.Join(group.Hidden, test))
	}
	for _, path := range paths {
		if !filepath.IsLocal(path) {
			return fmt.Errorf("%q is outside the suite", path)
		}
		if _, err := os.Stat(filepath.Join(s.Dir, path)); err != nil {
			return err
		}
	}
	return nil
}

// Repo returns the repo a group names.
func (s Suite) Repo(group Group) Repo {
	for _, repo := range s.Repos {
		if repo.Name == group.Repo {
			return repo
		}
	}
	return Repo{}
}

// Short lists how the suite falls below the golden suite's size, or nothing when it is big enough.
func (s Suite) Short() []string {
	var short []string
	if len(s.Repos) < MinRepos {
		short = append(short, fmt.Sprintf("%d repos, want at least %d", len(s.Repos), MinRepos))
	}
	for _, language := range Languages {
		found := false
		for _, repo := range s.Repos {
			found = found || strings.EqualFold(repo.Language, language)
		}
		if !found {
			short = append(short, "no "+language+" repo")
		}
	}
	counts := make(map[string]int, len(s.Repos))
	for _, group := range s.Groups {
		counts[group.Repo]++
	}
	for _, repo := range s.Repos {
		if counts[repo.Name] < MinGroups {
			short = append(short, fmt.Sprintf("%s holds %d groups, want at least %d", repo.Name, counts[repo.Name], MinGroups))
		}
	}
	return short
}

// List prints every repo, then each of its groups with the commit it is pinned to, and returns ErrShort
// when the suite is smaller than the golden suite must be.
func (s Suite) List(w io.Writer) error {
	for _, repo := range s.Repos {
		fmt.Fprintf(w, "%s (%s) %s\n", repo.Name, repo.Language, repo.URL)
		for _, group := range s.Groups {
			if group.Repo == repo.Name {
				fmt.Fprintf(w, "  %s  %s  %s  %d hidden tests\n", group.ID, group.Commit, group.File, len(group.HiddenTests))
			}
		}
	}
	short := s.Short()
	if len(short) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrShort, strings.Join(short, "; "))
}

// Package detect reads a repo's tree once and caches what it finds under .komodo/profile.json.
package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// cacheFile is where a detection's profile lives, relative to the repo root.
const cacheFile = ".komodo/profile.json"

// Profile is what a tree looked like the last time it was read.
type Profile struct {
	Languages []string `json:"languages"`
	Cloud     []string `json:"cloud"`
	Data      []string `json:"data"`
	CI        []string `json:"ci"`
	Verify    string   `json:"verify"`
	Compile   string   `json:"compile"`
}

// cache is what a detection last wrote to disk.
type cache struct {
	Profile Profile `json:"profile"`
}

// languageByExt maps a file extension to the language it names.
var languageByExt = map[string]string{
	".go":   "Go",
	".py":   "Python",
	".ts":   "TypeScript",
	".tsx":  "TypeScript",
	".js":   "JavaScript",
	".jsx":  "JavaScript",
	".rb":   "Ruby",
	".rs":   "Rust",
	".java": "Java",
	".cs":   "C#",
	".php":  "PHP",
}

// skipDir names directories a detection never descends into.
var skipDir = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".komodo":      true,
	"bin":          true,
}

// samTransform matches a SAM transform declaration in a CloudFormation template.
var samTransform = regexp.MustCompile(`(?m)^\s*Transform:\s*.*Serverless`)

// terraformProvider matches a Terraform provider block.
var terraformProvider = regexp.MustCompile(`(?m)^\s*provider\s+"[^"]+"\s*\{`)

// composeServices matches a Docker Compose services block.
var composeServices = regexp.MustCompile(`(?m)^services:`)

// Load walks the tree, returns the fresh profile, and rewrites the cache only when it changed.
func Load(root string) Profile {
	profile, _ := Detect(root)
	path := filepath.Join(root, cacheFile)
	data, err := os.ReadFile(path)
	if err == nil {
		var cached cache
		if json.Unmarshal(data, &cached) == nil && reflect.DeepEqual(cached.Profile, profile) {
			return cached.Profile
		}
	}
	save(root, profile)
	return profile
}

// LoadCached reads the profile straight from the cache file, without walking the tree or saving.
func LoadCached(root string) (Profile, bool) {
	data, err := os.ReadFile(filepath.Join(root, cacheFile))
	if err != nil {
		return Profile{}, false
	}
	var cached cache
	if json.Unmarshal(data, &cached) != nil {
		return Profile{}, false
	}
	return cached.Profile, true
}

// Detect walks the tree once and returns the profile it found, and the manifests it read.
func Detect(root string) (Profile, []string) {
	languages := map[string]bool{}
	var manifests []string
	cloud := map[string]bool{}
	data := map[string]bool{}
	found := map[string]bool{}

	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if entry.IsDir() {
			if rel != "." && (skipDir[entry.Name()] || strings.HasPrefix(entry.Name(), ".")) && entry.Name() != ".github" {
				return filepath.SkipDir
			}
			return nil
		}
		if language, ok := languageByExt[strings.ToLower(filepath.Ext(rel))]; ok {
			languages[language] = true
		}
		name := entry.Name()
		found[name] = true
		switch name {
		case "cdk.json":
			cloud["aws"] = true
			manifests = append(manifests, rel)
		case "cloudbuild.yaml", "app.yaml":
			cloud["gcp"] = true
			manifests = append(manifests, rel)
		case "azure-pipelines.yml":
			cloud["azure"] = true
			manifests = append(manifests, rel)
		case "template.yaml":
			manifests = append(manifests, rel)
			if body, readErr := os.ReadFile(path); readErr == nil && samTransform.Match(body) {
				cloud["aws"] = true
			}
		case "schema.prisma":
			data["prisma"] = true
			manifests = append(manifests, rel)
		case "dbt_project.yml":
			data["dbt"] = true
			manifests = append(manifests, rel)
		case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
			manifests = append(manifests, rel)
			if body, readErr := os.ReadFile(path); readErr == nil && composeServices.Match(body) {
				data["compose"] = true
			}
		case "go.mod", "package.json", "pyproject.toml", "requirements.txt", "Cargo.toml", "Gemfile":
			manifests = append(manifests, rel)
		}
		if strings.HasSuffix(name, ".tf") {
			manifests = append(manifests, rel)
			if body, readErr := os.ReadFile(path); readErr == nil && terraformProvider.Match(body) {
				cloud["terraform"] = true
			}
		}
		if strings.HasSuffix(name, ".sql") && strings.Contains(rel, "migrat") {
			data["sql"] = true
			manifests = append(manifests, rel)
		}
		return nil
	})

	var ci []string
	if entries, err := os.ReadDir(filepath.Join(root, ".github", "workflows")); err == nil && len(entries) > 0 {
		ci = append(ci, "github-actions")
		manifests = append(manifests, filepath.Join(".github", "workflows"))
	}

	verify, compile := commands(found)

	profile := Profile{
		Languages: sorted(languages),
		Cloud:     sorted(cloud),
		Data:      sorted(data),
		CI:        ci,
		Verify:    verify,
		Compile:   compile,
	}
	sort.Strings(manifests)
	return profile, manifests
}

// commands picks the verify and compile commands from the first manifest a fixed discovery order names.
func commands(found map[string]bool) (verify, compile string) {
	switch {
	case found["go.mod"]:
		return "go test ./...", "go build ./..."
	case found["package.json"]:
		return "npm test", "npm run build"
	case found["pyproject.toml"], found["requirements.txt"]:
		return "pytest", ""
	case found["Cargo.toml"]:
		return "cargo test", "cargo build"
	case found["Gemfile"]:
		return "bundle exec rspec", ""
	default:
		return "", ""
	}
}

// sorted returns the keys of a set, alphabetised, or nil for an empty set.
func sorted(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// save writes the profile to the cache file.
func save(root string, profile Profile) {
	path := filepath.Join(root, cacheFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	cached := cache{Profile: profile}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

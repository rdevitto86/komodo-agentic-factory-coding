package glob

import "testing"

func TestMatchTreatsDirDoubleStarAsAPrefix(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"internal/repo/**", "internal/repo/context.go", true},
		{"internal/repo/**", "internal/repo/sub/deep/file.go", true},
		{"internal/repo/**", "internal/repo", true},
		{"internal/repo/**", "internal/other/context.go", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestMatchUsesPathMatchForASingleStar(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"src/*.ts", "src/a.ts", true},
		{"src/*.ts", "src/a.js", false},
		{"docs/*.md", "docs/readme.md", true},
		{"docs/*.md", "other/readme.md", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestMatchHandlesTheShippedDoubleStarShapes(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"**/*.go", "a/b/c.go", true},
		{"**/*.go", "a/b/c.rs", false},
		{"**/Dockerfile", "deploy/Dockerfile", true},
		{"**/Dockerfile", "deploy/Dockerfile.dev", false},
		{"**/migrations/**", "db/migrations/001.sql", true},
		{"**/migrations/**", "db/schema/001.sql", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestMatchOnAPlainPathIsExact(t *testing.T) {
	if !Match("README.md", "README.md") {
		t.Fatal("an identical path must match")
	}
	if Match("README.md", "docs/README.md") {
		t.Fatal("a differing path must not match")
	}
}

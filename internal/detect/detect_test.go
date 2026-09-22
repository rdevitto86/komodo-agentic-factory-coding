package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// write puts one file into a fixture repo.
func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectFindsLanguageAndCommandsFromGoMod(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example\n")
	write(t, root, "main.go", "package main\n")

	profile, manifests := Detect(root)

	if len(profile.Languages) != 1 || profile.Languages[0] != "Go" {
		t.Fatalf("languages = %v", profile.Languages)
	}
	if profile.Verify != "go test ./..." || profile.Compile != "go build ./..." {
		t.Fatalf("commands = %q, %q", profile.Verify, profile.Compile)
	}
	if len(profile.Cloud) != 0 {
		t.Fatalf("cloud = %v, want none", profile.Cloud)
	}
	if len(manifests) != 1 || manifests[0] != "go.mod" {
		t.Fatalf("manifests = %v", manifests)
	}
}

func TestDetectFindsCloudMarkers(t *testing.T) {
	root := t.TempDir()
	write(t, root, "cdk.json", "{}")
	write(t, root, "infra/main.tf", `provider "aws" {
  region = "us-east-1"
}
`)
	write(t, root, "cloudbuild.yaml", "steps: []")
	write(t, root, "azure-pipelines.yml", "trigger: none")
	write(t, root, "template.yaml", "Transform: AWS::Serverless-2016-10-31\n")

	profile, _ := Detect(root)

	want := map[string]bool{"aws": true, "gcp": true, "azure": true, "terraform": true}
	if len(profile.Cloud) != len(want) {
		t.Fatalf("cloud = %v", profile.Cloud)
	}
	for _, name := range profile.Cloud {
		if !want[name] {
			t.Fatalf("unexpected cloud marker %q", name)
		}
	}
}

func TestDetectFindsDataAndCI(t *testing.T) {
	root := t.TempDir()
	write(t, root, "prisma/schema.prisma", "datasource db {}\n")
	write(t, root, "dbt_project.yml", "name: proj\n")
	write(t, root, "docker-compose.yml", "services:\n  web: {}\n")
	write(t, root, "migrations/0001_init.sql", "CREATE TABLE t (id int);\n")
	write(t, root, ".github/workflows/ci.yml", "on: push\n")

	profile, _ := Detect(root)

	wantData := map[string]bool{"prisma": true, "dbt": true, "compose": true, "sql": true}
	if len(profile.Data) != len(wantData) {
		t.Fatalf("data = %v", profile.Data)
	}
	for _, name := range profile.Data {
		if !wantData[name] {
			t.Fatalf("unexpected data source %q", name)
		}
	}
	if len(profile.CI) != 1 || profile.CI[0] != "github-actions" {
		t.Fatalf("ci = %v", profile.CI)
	}
}

func TestDetectAnUnknownTreeIsAnEmptyProfile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# nothing here\n")

	profile, manifests := Detect(root)

	if len(profile.Languages) != 0 || len(profile.Cloud) != 0 || len(profile.Data) != 0 || len(profile.CI) != 0 {
		t.Fatalf("profile = %+v, want empty", profile)
	}
	if profile.Verify != "" || profile.Compile != "" {
		t.Fatalf("commands = %q, %q, want empty", profile.Verify, profile.Compile)
	}
	if len(manifests) != 0 {
		t.Fatalf("manifests = %v, want none", manifests)
	}
}

func TestLoadWritesAndReusesTheCache(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example\n")

	first := Load(root)
	if first.Verify != "go test ./..." {
		t.Fatalf("verify = %q", first.Verify)
	}

	cachePath := filepath.Join(root, cacheFile)
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}

	second := Load(root)
	if second.Verify != first.Verify {
		t.Fatalf("second load = %+v, want %+v", second, first)
	}
	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("cache was rewritten though the manifest did not change")
	}
}

func TestLoadRecomputesWhenAManifestIsAdded(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example\n")
	_ = Load(root)

	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "go.mod"), future, future); err != nil {
		t.Fatal(err)
	}
	write(t, root, "package.json", "{}")

	updated := Load(root)
	if updated.Verify != "go test ./..." {
		t.Fatalf("verify = %q, want go.mod to still win the discovery order", updated.Verify)
	}

	data, err := os.ReadFile(filepath.Join(root, cacheFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("cache file is empty")
	}
}

func TestLoadDoesNotRewriteWhenOnlyAManifestTimestampChanges(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example\n")
	_ = Load(root)

	cachePath := filepath.Join(root, cacheFile)
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	// A touched manifest with no content change leaves the profile identical.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "go.mod"), future, future); err != nil {
		t.Fatal(err)
	}

	updated := Load(root)
	if updated.Verify != "go test ./..." {
		t.Fatalf("verify = %q", updated.Verify)
	}

	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("cache was rewritten though the profile did not change")
	}
}

func TestLoadNoticesAManifestThatDidNotExistAtTheLastWalk(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# nothing here\n")

	empty := Load(root)
	if empty.Verify != "" {
		t.Fatalf("verify = %q, want empty profile", empty.Verify)
	}

	write(t, root, "go.mod", "module example\n")
	updated := Load(root)
	if updated.Verify != "go test ./..." {
		t.Fatalf("verify = %q, want the new go.mod to be detected", updated.Verify)
	}
}

func TestLoadCachedReadsWithoutWalkingOrSaving(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example\n")
	_ = Load(root)

	cachePath := filepath.Join(root, cacheFile)
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	cached, ok := LoadCached(root)
	if !ok {
		t.Fatal("want a cached profile")
	}
	if cached.Verify != "go test ./..." {
		t.Fatalf("verify = %q", cached.Verify)
	}

	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("LoadCached rewrote the cache file")
	}
}

func TestLoadCachedReportsNoCacheWhenNoneWasWritten(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# nothing here\n")

	if _, ok := LoadCached(root); ok {
		t.Fatal("want no cache when detect has never run")
	}
}

func TestLoadNoticesALanguageAddedWithNoManifestOfItsOwn(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example\n")
	write(t, root, "main.go", "package main\n")

	first := Load(root)
	if len(first.Languages) != 1 || first.Languages[0] != "Go" {
		t.Fatalf("languages = %v, want [Go]", first.Languages)
	}

	write(t, root, "script.py", "print('hi')\n")
	updated := Load(root)
	if len(updated.Languages) != 2 || updated.Languages[0] != "Go" || updated.Languages[1] != "Python" {
		t.Fatalf("languages = %v, want [Go Python] once a .py file exists though no manifest changed", updated.Languages)
	}

	cached, err := os.ReadFile(filepath.Join(root, cacheFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cached), "Python") {
		t.Fatalf("cache = %s, want the fresh languages saved", cached)
	}
}

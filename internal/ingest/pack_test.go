package ingest

import (
	"reflect"
	"strings"
	"testing"
)

// packTree is a small module: card files in a/ and web/, their imports, callers, and neighbouring tests.
var packTree = map[string]string{
	"go.mod":       "module example\n\ngo 1.22\n",
	"AGENTS.md":    "# Rules\n\n- Keep it small.\n",
	"docs/spec.md": "# Spec\n\n## Wanted\n\nThe wanted section.\n\n## Other\n\nNot this.\n",
	"a/a.go":       "package a\n\nimport \"example/b\"\n\n// Do runs.\nfunc Do() string { return b.Exported(1) }\n",
	"a/other.go":   "package a\n\nfunc other() string {\n\treturn Do()\n}\n",
	"a/a_test.go":  "package a\n\nimport \"testing\"\n\nfunc TestDo(t *testing.T) { Do() }\n",
	"b/b.go": "package b\n\n// Exported doubles.\nfunc Exported(x int) string { return \"secret body\" }\n\n" +
		"func hidden() {}\n\n// T holds A.\ntype T struct{ A int }\n\nfunc (t T) M() int { return t.A }\n\nconst Limit = 3\n",
	"c/c.go":           "package c\n\nimport alias \"example/a\"\n\nfunc Use() string {\n\treturn alias.Do()\n}\n",
	"d/d.go":           "package d\n\nfunc Do() {}\n",
	"web/card.ts":      "import { helper } from './util';\n\nexport function build(): string {\n  return helper(1);\n}\n",
	"web/util.ts":      "export function helper(x: number): string {\n  return 'secret';\n}\nfunction internal() {}\n",
	"web/use.ts":       "import { build } from './card';\n\nconst value = build();\n",
	"web/unrelated.ts": "const build = 1;\n",
	"web/card.test.ts": "import { build } from './card';\ntest('build', () => build());\n",
}

// packCard is the card for packTree: the Go and TypeScript card files, a cited section and a free-text note.
var packCard = Card{
	Files:   []string{"a/a.go", "web/card.ts", "a/new.go"},
	Context: []string{"docs/spec.md#wanted", "a plain note", "docs/spec.md#missing"},
}

// itemFor returns the pack item of kind from source, failing the test when it is absent.
func itemFor(t *testing.T, pack Pack, kind PackKind, source string) PackItem {
	t.Helper()
	for _, item := range pack.Items {
		if item.Kind == kind && item.Source == source {
			return item
		}
	}
	t.Fatalf("no %s item from %s in %+v", kind, source, pack.Items)
	return PackItem{}
}

// hasItem reports whether the pack holds an item of kind from source.
func hasItem(pack Pack, kind PackKind, source string) bool {
	for _, item := range pack.Items {
		if item.Kind == kind && item.Source == source {
			return true
		}
	}
	return false
}

func TestBuildPackGathersEachKindOfItem(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, packTree)
	pack, err := BuildPack(root, packCard)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		kind    PackKind
		source  string
		want    []string
		notWant []string
	}{
		{"rules", KindRules, "AGENTS.md", []string{"Keep it small."}, nil},
		{"cited section", KindSpec, "docs/spec.md#wanted", []string{"The wanted section."}, []string{"Not this."}},
		{"free-text note", KindSpec, "a plain note", []string{"a plain note"}, nil},
		{"missing section", KindSpec, "docs/spec.md#missing", []string{"has no section"}, nil},
		{"file body", KindFile, "a/a.go", []string{"func Do() string"}, nil},
		{"new file", KindFile, "a/new.go", []string{"[does not exist yet]"}, nil},
		{
			"go signatures", KindSignatures, "example/b",
			[]string{"package b", "func Exported(x int) string", "type T struct", "func (t T) M() int", "const Limit = 3"},
			[]string{"secret body", "hidden", "doubles"},
		},
		{
			"go callers", KindCallers, "a/a.go",
			[]string{"c/c.go:6: return alias.Do()", "a/other.go:4: return Do()"},
			[]string{"a/a_test.go", "d/d.go"},
		},
		{"neighbouring go test", KindTest, "a/a_test.go", []string{"func TestDo"}, nil},
		{
			"ts signatures", KindSignatures, "web/util.ts",
			[]string{"export function helper(x: number): string"},
			[]string{"secret", "internal"},
		},
		{
			"ts callers", KindCallers, "web/card.ts",
			[]string{"web/use.ts:3: const value = build();"},
			[]string{"unrelated", "import"},
		},
		{"neighbouring ts test", KindTest, "web/card.test.ts", []string{"test('build'"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := itemFor(t, pack, tc.kind, tc.source)
			for _, want := range tc.want {
				if !strings.Contains(item.Body, want) {
					t.Errorf("body lacks %q:\n%s", want, item.Body)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(item.Body, notWant) {
					t.Errorf("body holds %q:\n%s", notWant, item.Body)
				}
			}
		})
	}
}

func TestBuildPackLeavesOutWhatTheBuilderAlreadyHas(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, packTree)
	pack, err := BuildPack(root, packCard)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		kind   PackKind
		source string
	}{
		{"own package signatures", KindSignatures, "example/a"},
		{"a card file as a test", KindTest, "a/a.go"},
		{"an unrelated package's test", KindTest, "b/b_test.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if hasItem(pack, tc.kind, tc.source) {
				t.Fatalf("pack holds %s item from %s", tc.kind, tc.source)
			}
		})
	}
}

func TestBuildPackIsTheSameForTheSameCardAndTree(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, packTree)
	first, err := BuildPack(root, packCard)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPack(root, packCard)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("packs differ:\n%+v\n%+v", first, second)
	}
}

func TestBuildPackCapsEachItemThenTheTotal(t *testing.T) {
	root := t.TempDir()
	big := strings.Repeat("x", PackItemCap*2)
	files := map[string]string{}
	var card Card
	for _, name := range []string{"f1.txt", "f2.txt", "f3.txt", "f4.txt", "f5.txt", "f6.txt", "f7.txt"} {
		files[name] = big
		card.Files = append(card.Files, name)
	}
	writeTree(t, root, files)
	pack, err := BuildPack(root, card)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, item := range pack.Items {
		if len(item.Body) > PackItemCap {
			t.Errorf("%s is %d bytes, over the item cap", item.Source, len(item.Body))
		}
		if !strings.Contains(item.Body, "truncated") {
			t.Errorf("%s was cut without a clip marker", item.Source)
		}
		total += len(item.Body)
	}
	if total != pack.Bytes || total > PackTotalCap {
		t.Fatalf("bytes = %d, summed %d, cap %d", pack.Bytes, total, PackTotalCap)
	}
	if len(pack.Omitted) == 0 || pack.Omitted[len(pack.Omitted)-1] != "file: f7.txt" {
		t.Fatalf("omitted = %v, want the last file named", pack.Omitted)
	}
}

func TestBuildPackKeepsASmallItemAfterTheTotalRunsLow(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"big1.txt": strings.Repeat("x", PackItemCap*2),
		"big2.txt": strings.Repeat("x", PackItemCap*2),
		"big3.txt": strings.Repeat("x", PackItemCap*2),
		"big4.txt": strings.Repeat("x", PackItemCap*2),
		"mid.txt":  strings.Repeat("x", PackTotalCap-4*PackItemCap-100),
		"tiny.txt": "small",
	}
	writeTree(t, root, files)
	card := Card{Files: []string{"big1.txt", "big2.txt", "big3.txt", "big4.txt", "mid.txt", "tiny.txt"}}
	pack, err := BuildPack(root, card)
	if err != nil {
		t.Fatal(err)
	}
	if item := itemFor(t, pack, KindFile, "tiny.txt"); item.Body != "small" {
		t.Fatalf("tiny body = %q", item.Body)
	}
	if pack.Bytes > PackTotalCap {
		t.Fatalf("bytes = %d, over %d", pack.Bytes, PackTotalCap)
	}
}

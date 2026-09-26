package ingest

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"komodo/internal/line"
)

// Context pack caps in bytes, starting values for one item and for the whole pack.
const (
	PackItemCap  = 10000
	PackTotalCap = 48000
	// packItemFloor is the least budget a clipped item keeps; below it the item is omitted instead.
	packItemFloor = 2000
)

// PackKind names what one context pack item holds.
type PackKind string

// The kinds of item a context pack carries, in the order the pack adds them.
const (
	KindRules      PackKind = "rules"
	KindSpec       PackKind = "spec"
	KindFile       PackKind = "file"
	KindSignatures PackKind = "signatures"
	KindCallers    PackKind = "callers"
	KindTest       PackKind = "test"
)

// PackItem is one capped piece of a context pack and the path or reference it came from.
type PackItem struct {
	Kind   PackKind `json:"kind"`
	Source string   `json:"source"`
	Body   string   `json:"body"`
}

// Pack is what a builder starts with instead of exploring: capped items, the ones the total dropped, and body bytes.
type Pack struct {
	Items   []PackItem `json:"items"`
	Omitted []string   `json:"omitted,omitempty"`
	Bytes   int        `json:"bytes"`
}

var (
	tsExport = regexp.MustCompile(
		`^export\s+(?:default\s+)?(?:declare\s+)?(?:async\s+)?(?:abstract\s+)?` +
			`(?:function\*?|const|let|var|class|interface|type|enum)\s+([A-Za-z_$][\w$]*)`,
	)
	tsImport = regexp.MustCompile(`^\s*(?:import|export)\b.*\bfrom\s+['"]([^'"]+)['"]`)
)

// BuildPack compiles a card's context pack from the tree at root, capping each item and then the total.
func BuildPack(root string, card Card) (Pack, error) {
	sources, err := scanSources(root)
	if err != nil {
		return Pack{}, err
	}
	skip := map[string]bool{}
	for _, file := range card.Files {
		skip[file] = true
	}
	tests := neighbourTests(sources, card.Files, skip)
	for _, test := range tests {
		skip[test] = true
	}
	module := modulePath(root)

	var items []PackItem
	if data, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err == nil {
		items = append(items, PackItem{Kind: KindRules, Source: "AGENTS.md", Body: string(data)})
	}
	items = append(items, specItems(root, card.Context)...)
	for _, file := range card.Files {
		items = append(items, readItem(root, KindFile, file))
	}
	items = append(items, goSignatureItems(root, module, card.Files)...)
	items = append(items, tsSignatureItems(root, card.Files)...)
	items = append(items, goCallerItems(root, module, sources, card.Files, skip)...)
	items = append(items, tsCallerItems(root, sources, card.Files, skip)...)
	for _, test := range tests {
		items = append(items, readItem(root, KindTest, test))
	}
	return capPack(items), nil
}

// capPack clips each item to its cap and the total's remainder, omitting one that would fall below the floor.
func capPack(items []PackItem) Pack {
	var pack Pack
	remaining := PackTotalCap
	for _, item := range items {
		limit := min(PackItemCap, remaining)
		if len(item.Body) > limit && limit < packItemFloor {
			pack.Omitted = append(pack.Omitted, string(item.Kind)+": "+item.Source)
			continue
		}
		item.Body = line.Clip(item.Body, limit, string(item.Kind))
		remaining -= len(item.Body)
		pack.Bytes += len(item.Body)
		pack.Items = append(pack.Items, item)
	}
	return pack
}

// scanSources lists every Go and TypeScript file under root, relative and sorted.
func scanSources(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if full != root && skipDir[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !isGo(full) && !isTS(full) {
			return nil
		}
		rel, relErr := filepath.Rel(root, full)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}

// neighbourTests are the Go tests beside a card's Go files and the TypeScript tests named after its TS files.
func neighbourTests(sources, files []string, inCard map[string]bool) []string {
	goDirs := map[string]bool{}
	want := map[string]bool{}
	for _, file := range files {
		switch {
		case isGo(file):
			goDirs[path.Dir(file)] = true
		case isTS(file):
			base := strings.TrimSuffix(strings.TrimSuffix(file, ".tsx"), ".ts")
			for _, suffix := range []string{".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx"} {
				want[base+suffix] = true
			}
		}
	}
	var out []string
	for _, source := range sources {
		if inCard[source] {
			continue
		}
		if want[source] || (strings.HasSuffix(source, "_test.go") && goDirs[path.Dir(source)]) {
			out = append(out, source)
		}
	}
	return out
}

// specItems resolves each cited reference to its file or anchored section; free text is carried as written.
func specItems(root string, refs []string) []PackItem {
	var items []PackItem
	for _, ref := range refs {
		file, anchor, _ := strings.Cut(ref, "#")
		data, err := os.ReadFile(filepath.Join(root, file))
		body := string(data)
		switch {
		case err != nil:
			body = ref
		case anchor != "":
			body = line.Section(body, anchor)
			if body == "" {
				body = fmt.Sprintf("[%s has no section %q]", file, anchor)
			}
		}
		items = append(items, PackItem{Kind: KindSpec, Source: ref, Body: body})
	}
	return items
}

// readItem reads one file into an item, naming a missing, prebuilt, or non-text file instead of reading it.
func readItem(root string, kind PackKind, file string) PackItem {
	item := PackItem{Kind: kind, Source: file}
	full := filepath.Join(root, file)
	info, err := os.Stat(full)
	switch {
	case err != nil:
		item.Body = "[does not exist yet]"
	case strings.HasPrefix(file, "bin/"):
		item.Body = fmt.Sprintf("[a prebuilt binary, %d bytes, never read]", info.Size())
	case !line.IsText(full):
		item.Body = fmt.Sprintf("[not text, %d bytes, never read]", info.Size())
	default:
		data, err := os.ReadFile(full)
		if err != nil {
			item.Body = fmt.Sprintf("[unreadable: %v]", err)
			break
		}
		item.Body = string(data)
	}
	return item
}

// modulePath is the module line of root's go.mod, or empty when there is none.
func modulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, text := range strings.Split(string(data), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(text), "module "); ok {
			return strings.Trim(strings.TrimSpace(name), `"`)
		}
	}
	return ""
}

// importPath is the import path of a repo directory under module.
func importPath(module, dir string) string {
	if dir == "." {
		return module
	}
	return module + "/" + dir
}

// goSignatureItems renders the exported declarations of every in-module package a card's Go files import.
func goSignatureItems(root, module string, files []string) []PackItem {
	if module == "" {
		return nil
	}
	own := map[string]bool{}
	imported := map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range files {
		if !isGo(file) {
			continue
		}
		own[path.Dir(file)] = true
		parsed, err := parser.ParseFile(fset, filepath.Join(root, file), nil, parser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, spec := range parsed.Imports {
			imp, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			if imp == module {
				imported["."] = true
			} else if dir, ok := strings.CutPrefix(imp, module+"/"); ok {
				imported[dir] = true
			}
		}
	}
	var items []PackItem
	for _, dir := range sortedSet(imported) {
		if own[dir] {
			continue
		}
		if body := goSignatures(root, dir); body != "" {
			items = append(items, PackItem{Kind: KindSignatures, Source: importPath(module, dir), Body: body})
		}
	}
	return items
}

// goSignatures prints every exported declaration of the package in dir, function bodies dropped.
func goSignatures(root, dir string) string {
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return ""
	}
	fset := token.NewFileSet()
	name := ""
	var parts []string
	for _, entry := range entries {
		file := entry.Name()
		if entry.IsDir() || !isGo(file) || strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, filepath.Join(root, dir, file), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		name = parsed.Name.Name
		for _, decl := range parsed.Decls {
			node := exportedDecl(decl)
			if node == nil {
				continue
			}
			var buffer bytes.Buffer
			if err := format.Node(&buffer, fset, node); err == nil {
				parts = append(parts, buffer.String())
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "package " + name + "\n\n" + strings.Join(parts, "\n")
}

// exportedDecl is a declaration trimmed to its exported part, a function without its body, or nil.
func exportedDecl(decl ast.Decl) ast.Node {
	switch typed := decl.(type) {
	case *ast.FuncDecl:
		if !typed.Name.IsExported() || (typed.Recv != nil && !exportedReceiver(typed.Recv)) {
			return nil
		}
		return &ast.FuncDecl{Recv: typed.Recv, Name: typed.Name, Type: typed.Type}
	case *ast.GenDecl:
		if typed.Tok == token.IMPORT {
			return nil
		}
		var specs []ast.Spec
		for _, spec := range typed.Specs {
			switch value := spec.(type) {
			case *ast.TypeSpec:
				if value.Name.IsExported() {
					specs = append(specs, value)
				}
			case *ast.ValueSpec:
				if anyExported(value.Names) {
					specs = append(specs, value)
				}
			}
		}
		if len(specs) == 0 {
			return nil
		}
		return &ast.GenDecl{TokPos: typed.TokPos, Tok: typed.Tok, Lparen: typed.Lparen, Specs: specs, Rparen: typed.Rparen}
	}
	return nil
}

// exportedReceiver reports whether a method's receiver type is exported, through pointers and type parameters.
func exportedReceiver(recv *ast.FieldList) bool {
	if len(recv.List) == 0 {
		return false
	}
	expr := recv.List[0].Type
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.Ident:
			return typed.IsExported()
		default:
			return false
		}
	}
}

// anyExported reports whether one of names is exported.
func anyExported(names []*ast.Ident) bool {
	for _, name := range names {
		if name.IsExported() {
			return true
		}
	}
	return false
}

// goSymbols are the top-level names one card file declares, split into plain names and method names.
type goSymbols struct {
	file    string
	dir     string
	pkg     string
	plain   map[string]bool
	methods map[string]bool
}

// goCallerItems lists, per card Go file, every line elsewhere in the repo that references a symbol it declares.
func goCallerItems(root, module string, sources, files []string, skip map[string]bool) []PackItem {
	fset := token.NewFileSet()
	var declared []goSymbols
	for _, file := range files {
		if !isGo(file) {
			continue
		}
		parsed, err := parser.ParseFile(fset, filepath.Join(root, file), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		declared = append(declared, collectGoSymbols(file, parsed))
	}
	if len(declared) == 0 {
		return nil
	}
	hits := make([][]string, len(declared))
	for _, source := range sources {
		if skip[source] || !isGo(source) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, source))
		if err != nil {
			continue
		}
		parsed, err := parser.ParseFile(fset, source, data, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for index, symbols := range declared {
			for _, number := range goReferences(fset, parsed, source, module, symbols) {
				hits[index] = append(hits[index], fmt.Sprintf("%s:%d: %s", source, number, strings.TrimSpace(lines[number-1])))
			}
		}
	}
	var items []PackItem
	for index, symbols := range declared {
		if len(hits[index]) > 0 {
			items = append(items, PackItem{Kind: KindCallers, Source: symbols.file, Body: strings.Join(hits[index], "\n")})
		}
	}
	return items
}

// collectGoSymbols gathers the functions, methods, types, constants and variables a parsed file declares.
func collectGoSymbols(file string, parsed *ast.File) goSymbols {
	symbols := goSymbols{
		file: file, dir: path.Dir(file), pkg: parsed.Name.Name,
		plain: map[string]bool{}, methods: map[string]bool{},
	}
	for _, decl := range parsed.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if typed.Recv != nil {
				symbols.methods[typed.Name.Name] = true
			} else {
				symbols.plain[typed.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch value := spec.(type) {
				case *ast.TypeSpec:
					symbols.plain[value.Name.Name] = true
				case *ast.ValueSpec:
					for _, name := range value.Names {
						symbols.plain[name.Name] = true
					}
				}
			}
		}
	}
	return symbols
}

// goReferences are the sorted lines in parsed naming a symbol: any match in its package, else a qualified use.
func goReferences(fset *token.FileSet, parsed *ast.File, source, module string, symbols goSymbols) []int {
	found := map[int]bool{}
	mark := func(node ast.Node) { found[fset.Position(node.Pos()).Line] = true }
	if path.Dir(source) == symbols.dir && parsed.Name.Name == symbols.pkg {
		ast.Inspect(parsed, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok && (symbols.plain[ident.Name] || symbols.methods[ident.Name]) {
				mark(ident)
			}
			return true
		})
	} else if local := importedAs(parsed, module, symbols); local != "" {
		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || !selector.Sel.IsExported() {
				return true
			}
			qualifier, isIdent := selector.X.(*ast.Ident)
			if (isIdent && qualifier.Name == local && symbols.plain[selector.Sel.Name]) || symbols.methods[selector.Sel.Name] {
				mark(selector)
			}
			return true
		})
	}
	numbers := make([]int, 0, len(found))
	for number := range found {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	return numbers
}

// importedAs is the name parsed uses for the package that declares symbols, or empty when it doesn't import it.
func importedAs(parsed *ast.File, module string, symbols goSymbols) string {
	if module == "" {
		return ""
	}
	want := importPath(module, symbols.dir)
	for _, spec := range parsed.Imports {
		imp, err := strconv.Unquote(spec.Path.Value)
		if err != nil || imp != want {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return symbols.pkg
	}
	return ""
}

// tsSignatureItems lists the export lines of every relative module a card's TypeScript files import.
func tsSignatureItems(root string, files []string) []PackItem {
	inCard := map[string]bool{}
	for _, file := range files {
		inCard[file] = true
	}
	imported := map[string]bool{}
	for _, file := range files {
		if !isTS(file) {
			continue
		}
		for target := range tsImports(root, file, readLines(root, file)) {
			if !inCard[target] {
				imported[target] = true
			}
		}
	}
	var items []PackItem
	for _, target := range sortedSet(imported) {
		var exports []string
		for _, text := range readLines(root, target) {
			if strings.HasPrefix(text, "export ") {
				exports = append(exports, strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "{")))
			}
		}
		if len(exports) > 0 {
			items = append(items, PackItem{Kind: KindSignatures, Source: target, Body: strings.Join(exports, "\n")})
		}
	}
	return items
}

// tsCallerItems lists, per card TypeScript file, every line in an importing file that names one of its exports.
func tsCallerItems(root string, sources, files []string, skip map[string]bool) []PackItem {
	var items []PackItem
	for _, file := range files {
		if !isTS(file) {
			continue
		}
		var names []string
		for _, text := range readLines(root, file) {
			if match := tsExport.FindStringSubmatch(text); match != nil {
				names = append(names, regexp.QuoteMeta(match[1]))
			}
		}
		if len(names) == 0 {
			continue
		}
		pattern := regexp.MustCompile(`\b(?:` + strings.Join(names, "|") + `)\b`)
		var hits []string
		for _, source := range sources {
			if skip[source] || !isTS(source) {
				continue
			}
			lines := readLines(root, source)
			if !tsImports(root, source, lines)[file] {
				continue
			}
			for index, text := range lines {
				if !tsImport.MatchString(text) && pattern.MatchString(text) {
					hits = append(hits, fmt.Sprintf("%s:%d: %s", source, index+1, strings.TrimSpace(text)))
				}
			}
		}
		if len(hits) > 0 {
			items = append(items, PackItem{Kind: KindCallers, Source: file, Body: strings.Join(hits, "\n")})
		}
	}
	return items
}

// tsImports resolves each relative import in a TypeScript file's lines to the repo file it names.
func tsImports(root, file string, lines []string) map[string]bool {
	out := map[string]bool{}
	for _, text := range lines {
		match := tsImport.FindStringSubmatch(text)
		if match == nil || !strings.HasPrefix(match[1], ".") {
			continue
		}
		base := path.Join(path.Dir(file), match[1])
		stem := strings.TrimSuffix(base, ".js")
		for _, candidate := range []string{base, stem + ".ts", stem + ".tsx", base + "/index.ts", base + "/index.tsx"} {
			if info, err := os.Stat(filepath.Join(root, candidate)); err == nil && !info.IsDir() {
				out[candidate] = true
				break
			}
		}
	}
	return out
}

// readLines is a file's lines, or none when it can't be read.
func readLines(root, file string) []string {
	data, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return nil
	}
	return strings.Split(string(data), "\n")
}

// sortedSet is the keys of a set in order.
func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// isGo reports whether a path is Go source.
func isGo(file string) bool { return strings.HasSuffix(file, ".go") }

// isTS reports whether a path is TypeScript source.
func isTS(file string) bool { return strings.HasSuffix(file, ".ts") || strings.HasSuffix(file, ".tsx") }

package doctor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// promiseBullet matches a grammar rule bullet naming the snake_case key its accessor is built from.
var promiseBullet = regexp.MustCompile("(?m)^- \\*\\*`([a-z][a-z0-9_]*)`\\*\\*")

// jsonTagKey extracts the snake_case key a struct field's JSON tag names.
var jsonTagKey = regexp.MustCompile(`json:"([a-z][a-z0-9_]*)`)

// promise is one grammar key or config field, and where it is promised.
type promise struct {
	key    string
	symbol string
	where  string
}

// checkPromises reports a grammar key or config field whose accessor exists but nothing outside its own tests calls.
func checkPromises(root string) []Problem {
	var problems []Problem
	files := sources(root)
	index := indexSymbols(files)
	for _, made := range promises(root, files) {
		if !index.declared[made.symbol] || index.called[made.symbol] {
			continue
		}
		problems = append(problems, Problem{"promises", made.where,
			made.key + " promises " + made.symbol + ", but nothing outside its own tests calls it"})
	}
	return problems
}

// promises lists every grammar bullet komodo/rules makes and every JSON-tagged struct field the source declares.
func promises(root string, files []string) []promise {
	var out []promise
	for _, path := range markdown(root) {
		relative := rel(root, path)
		if !strings.HasPrefix(relative, filepath.Join("komodo", "rules")) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, match := range promiseBullet.FindAllStringSubmatch(string(data), -1) {
			key := match[1]
			out = append(out, promise{key, pascal(key), relative})
		}
	}
	return append(out, fieldPromises(root, files)...)
}

// grammarPackages are the packages a repo or a machine's own JSON config populates by field name.
var grammarPackages = []string{filepath.Join("internal", "profile"), filepath.Join("internal", "repo")}

// fieldPromises lists every struct field with a JSON tag in a grammar package, the key it promises to read.
func fieldPromises(root string, files []string) []promise {
	var out []promise
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || !contains(grammarPackages, filepath.Dir(rel(root, path))) {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			field, ok := node.(*ast.Field)
			if !ok || field.Tag == nil {
				return true
			}
			match := jsonTagKey.FindStringSubmatch(field.Tag.Value)
			if match == nil {
				return true
			}
			for _, name := range field.Names {
				position := fset.Position(name.Pos())
				out = append(out, promise{match[1], name.Name, fmt.Sprintf("%s:%d", rel(root, path), position.Line)})
			}
			return true
		})
	}
	return out
}

// pascal turns a snake_case grammar key into the accessor name it promises.
func pascal(key string) string {
	var out strings.Builder
	for _, part := range strings.Split(key, "_") {
		if part == "" {
			continue
		}
		out.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return out.String()
}

// symbolIndex records where a symbol is declared and where a real call site uses it.
type symbolIndex struct {
	declared map[string]bool
	called   map[string]bool
}

// indexSymbols parses every source file once, resolving each symbol's declaration and its real call sites.
func indexSymbols(files []string) symbolIndex {
	index := symbolIndex{declared: map[string]bool{}, called: map[string]bool{}}
	for _, path := range files {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		test := strings.HasSuffix(path, "_test.go")
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				index.declared[n.Name.Name] = true
			case *ast.Field:
				if n.Tag != nil {
					for _, name := range n.Names {
						index.declared[name.Name] = true
					}
				}
			case *ast.CallExpr:
				if !test {
					if ident, ok := n.Fun.(*ast.Ident); ok {
						index.called[ident.Name] = true
					}
				}
			case *ast.SelectorExpr:
				if !test {
					index.called[n.Sel.Name] = true
				}
			}
			return true
		})
	}
	return index
}

package logger

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestSchemaDocumentsEveryKey keeps docs/schema.md and the exported Key*
// constants in sync: every constant must appear in the document, both as its
// key string and as its constant name.
func TestSchemaDocumentsEveryKey(t *testing.T) {
	doc, err := os.ReadFile("docs/schema.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "`"+SchemaVersion+"`") {
		t.Errorf("docs/schema.md does not mention SchemaVersion %s", SchemaVersion)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "schema.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if !strings.HasPrefix(name.Name, "Key") {
					continue
				}
				value, _ := strconv.Unquote(vs.Values[i].(*ast.BasicLit).Value)
				if other, dup := seen[value]; dup {
					t.Errorf("%s and %s share the key %q", name.Name, other, value)
				}
				seen[value] = name.Name
				for _, want := range []string{"`" + value + "`", "`" + name.Name + "`"} {
					if !strings.Contains(string(doc), want) {
						t.Errorf("docs/schema.md does not document %s", want)
					}
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no Key* constants found")
	}
}

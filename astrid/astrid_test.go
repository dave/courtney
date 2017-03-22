package astrid

import (
	"go/build"
	"go/parser"
	"testing"

	"golang.org/x/tools/go/loader"

	"strings"

	"go/ast"

	"go/token"

	"github.com/dave/patsy/builder"
	"github.com/dave/patsy/vos"
	"github.com/pkg/errors"
)

func TestCall(t *testing.T) {
	multi(t, map[string]string{
		"function call": `package a
		
			var f func(int, string) bool
			
			var _ = f(1, "a") // *1
			
			func foo() {
				f(0, "a")
				f(1, "b")
				f(1, "a") // #1
			}
		`,
		"method call": `package a
		
			type T struct{}
			
			func (T) f(int, string) bool {
				return false
			}
			
			var t T
			
			var _ = t.f(1, "a") // *1
			
			func foo() {
				t.f(0, "a")
				t.f(1, "b")
				t.f(1, "a") // #1
			}
			
			func bar() {
				var t T
				// different t
				t.f(1, "a")
			}
		`,
	})
}

func TestSimple(t *testing.T) {
	multi(t, map[string]string{
		"basic lit int": `package a
		
			var _ = 2 // *1
			
			func foo() int {
				i := 1
				i++
				i = 2 // #1
				return i
			}
		`,
		"basic lit bool": `package a
		
			var _ = false // *1
			
			func foo() bool {
				i := true
				if i {
					i = false // #1
				}
				return i
			}
		`,
		"nil": `package a
		
			var _ interface{} = nil // *1
			
			func foo() error {
				var err error
				if err != nil { // #1
					return err
				}
				return nil // #1
			}
		`,
		"selector": `package a
		
			type T struct {
				A string
				B string
			}
			
			var t T
			
			var _ = t.B // *1
			
			func foo() string {
				if true {
					return t.A
				}
				return t.B // #1
			}
			
			func bar() string {
				var t T
				// not the same t, so no match
				return t.B
			}
		`,
	})
}

func multi(t *testing.T, tests map[string]string) {
	for name, source := range tests {
		test(t, name, source)
	}
}

func test(t *testing.T, name, source string) {
	env := vos.Mock()
	b, err := builder.New(env, "ns")
	if err != nil {
		t.Fatalf("Error creating builder in %s: %s", name, err)
	}
	defer b.Cleanup()

	ppath, _, err := b.Package("a", map[string]string{
		"a.go": source,
	})
	if err != nil {
		t.Fatalf("Error creating package in %s: %s", name, err)
	}

	ctxt := build.Default
	ctxt.GOPATH = env.Getenv("GOPATH")
	wd, err := env.Getwd()
	if err != nil {
		t.Fatalf("Error getting working dir in %s: %s", name, err)
	}
	conf := loader.Config{Build: &ctxt, Cwd: wd, ParserMode: parser.ParseComments}

	conf.Import(ppath)
	prog, err := conf.Load()
	if err != nil {
		t.Fatalf("Error loading prog in %s: %s", name, err)
	}

	f := prog.Imported[ppath].Files[0]
	comments := make(map[int]string)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if !strings.HasPrefix(c.Text, "// ") {
				continue
			}
			text := strings.TrimPrefix(c.Text, "// ")
			pos := prog.Fset.Position(c.Pos())
			comments[pos.Line] = text
		}
	}

	annotation := func(node ast.Node) string {
		if node == nil {
			return ""
		}
		text, _ := comments[prog.Fset.Position(node.Pos()).Line]
		return text
	}

	check := func(ann, anh string, annotationMatch func(string) bool, nodeMatch func(ast.Node) bool) error {
		found := make(map[int]bool)
		ast.Inspect(f, func(n ast.Node) bool {
			if nodeMatch(n) {
				pos := prog.Fset.Position(n.Pos())
				found[pos.Line] = true
			}
			return true
		})
		if len(found) <= 1 {
			return errors.Errorf("Found annotated expression %s, but no matching expressions", ann)
		}
		// ensure all matching comments are in expected
		for line, text := range comments {
			if !annotationMatch(text) {
				continue
			}
			if _, ok := found[line]; !ok {
				return errors.Errorf("Expected matching annotation %s at line %d, but no matching expression.", anh, line)
			}
		}
		// ensure all expected are matching comments
		for line := range found {
			if text, ok := comments[line]; !ok {
				return errors.Errorf("Found matching expression at line %d, but no annotation %s.", line, anh)
			} else if !annotationMatch(text) {
				return errors.Errorf("Found matching expression at line %d, but annotation %s did not match %s.", line, text, anh)
			}
		}
		return nil
	}

	matcher := NewMatcher(prog.Imported[ppath].Info)

	ast.Inspect(f, func(node ast.Node) bool {
		ann := annotation(node)
		if !strings.HasPrefix(ann, "*") {
			return true
		}
		anh := "#" + strings.TrimPrefix(ann, "*")

		var expr ast.Expr

		switch n := node.(type) {
		case *ast.AssignStmt:
			if len(n.Rhs) != 1 {
				return true
			}
			expr = n.Rhs[0]
		case *ast.GenDecl:
			if n.Tok == token.IMPORT {
				return true
			}
			if len(n.Specs) != 1 {
				return true
			}
			switch s := n.Specs[0].(type) {
			case *ast.ValueSpec:
				if len(s.Values) != 1 {
					return true
				}
				expr = s.Values[0]
			case *ast.TypeSpec:
				expr = s.Type
			}
		}

		if expr == nil {
			return true
		}

		err := check(
			ann,
			anh,
			func(current string) bool {
				return current == ann || current == anh
			},
			func(current ast.Node) bool {
				if current == node {
					// exclude the node being matched, because it has a *
					// annotation
					return false
				}
				if currentExpr, ok := current.(ast.Expr); ok {
					return matcher.Match(expr, currentExpr)
				}
				return false
			},
		)
		if err != nil {
			t.Errorf("Failed in %s: %s", name, err)
		}

		return true
	})

}

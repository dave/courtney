package brenda_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"

	"github.com/dave/courtney/brenda"

	"fmt"

	"sort"
	"strings"

	"github.com/pkg/errors"

	"regexp"

	"os"
)

func ExampleNewSolver_bool_lit_3() {
	printExample(`
		var a bool
		if a && false {} else {}
	`)
	// Output:
	// if a && false {
	// 	// IMPOSSIBLE
	// } else {
	// 	// a UNKNOWN
	// }
}

func ExampleNewSolver_bool_lit_2() {
	printExample(`
		var a bool
		if a || true {} else {}
	`)
	// Output:
	// if a || true {
	// 	// a UNKNOWN
	// } else {
	// 	// IMPOSSIBLE
	// }
}

func ExampleNewSolver_bool_lit_1() {
	printExample(`
		if false {} else {}
	`)
	// Output:
	// if false {
	// 	// IMPOSSIBLE
	// } else {
	//
	// }
}

func ExampleNewSolver_simple() {
	printExample(`
		var a bool
		if a { }
	`)
	// Output:
	// if a {
	// 	// a TRUE
	// }
}

func ExampleNewSolver_else() {
	printExample(`
		var a bool
		if a {} else {}
	`)
	// Output:
	// if a {
	// 	// a TRUE
	// } else {
	// 	// a FALSE
	// }
}

func ExampleNewSolver_and() {
	printExample(`
		var a, b bool
		if a && b {} else {}
	`)
	// Output:
	// if a && b {
	// 	// a TRUE
	// 	// b TRUE
	// } else {
	// 	// a UNKNOWN
	// 	// b UNKNOWN
	// }
}

func ExampleNewSolver_unknown() {
	printExample(`
		var a, b, c bool
		if a && (b || c) {} else if b {}
	`)
	// Output:
	// if a && (b || c) {
	// 	// a TRUE
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// } else if b {
	// 	// a FALSE
	// 	// b TRUE
	// 	// c UNKNOWN
	// }
}

func ExampleNewSolver_impossible() {
	printExample(`
		var a bool
		if a {} else if !a {} else {}
	`)
	// Output:
	// if a {
	// 	// a TRUE
	// } else if !a {
	// 	// a FALSE
	// } else {
	// 	// IMPOSSIBLE
	// }
}

func ExampleNewSolver_or() {
	printExample(`
		var a, b bool
		if a || b {} else {}
	`)
	// Output:
	// if a || b {
	// 	// a UNKNOWN
	// 	// b UNKNOWN
	// } else {
	// 	// a FALSE
	// 	// b FALSE
	// }
}

func ExampleNewSolver_unary() {
	printExample(`
		var a, b bool
		if !(a || b) {} else {}
	`)
	// Output:
	// if !(a || b) {
	// 	// a FALSE
	// 	// b FALSE
	// } else {
	// 	// a UNKNOWN
	// 	// b UNKNOWN
	// }
}

func ExampleNewSolver_else_if() {
	printExample(`
		var a, b bool
		if a {} else if b {} else {}
	`)
	// Output:
	// if a {
	// 	// a TRUE
	// } else if b {
	// 	// a FALSE
	// 	// b TRUE
	// } else {
	// 	// a FALSE
	// 	// b FALSE
	// }
}

func ExampleNewSolver_invert() {
	printExample(`
		// should correctly detect that b == nil is the inverse of b != nil
		var a, b error
		if a == nil && b == nil {} else if b != nil {} else {}
	`)
	// Output:
	// if a == nil && b == nil {
	// 	// a == nil TRUE
	// 	// b == nil TRUE
	// } else if b != nil {
	// 	// a == nil UNKNOWN
	// 	// b != nil TRUE
	// } else {
	// 	// a == nil FALSE
	// 	// b == nil TRUE
	// }
}

func ExampleNewSolver_invert2() {
	printExample(`
		// should correctly detect that a == nil is the inverse of a != nil
		var a error
		var b bool
		if a == nil && b || a != nil {} else if b {} else {}
	`)
	// Output:
	// if a == nil && b || a != nil {
	// 	// a == nil UNKNOWN
	// 	// b UNKNOWN
	// } else if b {
	// 	// IMPOSSIBLE
	// } else {
	// 	// a == nil TRUE
	// 	// b FALSE
	// }
}

func ExampleNewSolver_invert_gt() {
	printExample(`
		var a int
		if a > 0 {} else if a <= 0 {} else {}
	`)
	// Output:
	// if a > 0 {
	// 	// a > 0 TRUE
	// } else if a <= 0 {
	// 	// a <= 0 TRUE
	// } else {
	// 	// IMPOSSIBLE
	// }
}

func ExampleNewSolver_invert_lt() {
	printExample(`
		var a int
		if a < 0 {} else if a >= 0 {} else {}
	`)
	// Output:
	// if a < 0 {
	// 	// a < 0 TRUE
	// } else if a >= 0 {
	// 	// a >= 0 TRUE
	// } else {
	// 	// IMPOSSIBLE
	// }
}

func ExampleNewSolver_brackets1() {
	printExample(`
		var a, b, c bool
		if a || (b && c) {} else if a {} else {}
	`)
	// Output:
	// if a || (b && c) {
	// 	// a UNKNOWN
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// } else if a {
	// 	// IMPOSSIBLE
	// } else {
	// 	// a FALSE
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// }
}

func ExampleNewSolver_brackets2() {
	printExample(`
		var a, b, c bool
		if a || (b && c) {} else if b {} else {}
	`)
	// Output:
	// if a || (b && c) {
	// 	// a UNKNOWN
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// } else if b {
	// 	// a FALSE
	// 	// b TRUE
	// 	// c FALSE
	// } else {
	// 	// a FALSE
	// 	// b FALSE
	// 	// c UNKNOWN
	// }
}

func ExampleNewSolver_brackets3() {
	printExample(`
		var a, b, c bool
		if a && (b || c) {} else if a {} else {}
	`)
	// Output:
	// if a && (b || c) {
	// 	// a TRUE
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// } else if a {
	// 	// a TRUE
	// 	// b FALSE
	// 	// c FALSE
	// } else {
	// 	// a FALSE
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// }
}

func ExampleNewSolver_brackets4() {
	printExample(`
		var a, b, c bool
		if a && (b || c) {} else if b {} else {}
	`)
	// Output:
	// if a && (b || c) {
	// 	// a TRUE
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// } else if b {
	// 	// a FALSE
	// 	// b TRUE
	// 	// c UNKNOWN
	// } else {
	// 	// a UNKNOWN
	// 	// b FALSE
	// 	// c UNKNOWN
	// }
}

func ExampleNewSolver_errors() {
	printExample(`
		var a error
		if a == nil {} else {}
	`)
	// Output:
	// if a == nil {
	// 	// a == nil TRUE
	// } else {
	// 	// a != nil TRUE
	// }
}

func ExampleNewSolver_mixed() {
	printExample(`
		var a error
		var b, c bool
		var d int
		if a == nil && (b && d > 0) || c {} else if d <= 0 || c {} else if b {}
	`)
	// Output:
	// if a == nil && (b && d > 0) || c {
	// 	// a == nil UNKNOWN
	// 	// b UNKNOWN
	// 	// c UNKNOWN
	// 	// d > 0 UNKNOWN
	// } else if d <= 0 || c {
	// 	// a == nil UNKNOWN
	// 	// b UNKNOWN
	// 	// c FALSE
	// 	// d <= 0 TRUE
	// } else if b {
	// 	// a == nil FALSE
	// 	// b TRUE
	// 	// c FALSE
	// 	// d > 0 TRUE
	// }
}

func printExample(src string) {
	err := printOutput(os.Stdout, src)
	if err != nil {
		fmt.Printf("%+v", err)
	}
}

func printOutput(writer io.Writer, src string) error {
	fpath := "/foo.go"
	ppath := "a.b/c"
	src = fmt.Sprintf("package a\n\nfunc a() {\n%s\n}", src)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fpath, src, parser.ParseComments)
	if err != nil {
		return errors.Wrap(err, "Error parsing file")
	}

	info := types.Info{
		Uses: make(map[*ast.Ident]types.Object),
		Defs: make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{
		Importer: importer.Default(),
	}
	if _, err = conf.Check(ppath, fset, []*ast.File{f}, &info); err != nil {
		return errors.Wrap(err, "Error checking conf")
	}

	var ifs *ast.IfStmt
	ast.Inspect(f, func(node ast.Node) bool {
		if ifs != nil {
			// once if statement is found, skip all further code
			return false
		}
		if n, ok := node.(*ast.IfStmt); ok && n != nil {
			ifs = n
			return false
		}
		return true
	})
	if ifs == nil {
		return errors.New("No *ast.IfStmt found")
	}

	err = printIf(writer, fset, info, ifs)
	if err != nil {
		return err
	}

	return nil
}

func printIf(writer io.Writer, fset *token.FileSet, info types.Info, expr *ast.IfStmt, falseExpr ...ast.Expr) error {

	s := brenda.NewSolver(fset, info.Uses, info.Defs, expr.Cond, falseExpr...)
	err := s.SolveTrue()
	if err != nil {
		return err
	}

	fmt.Fprintln(writer, "if", printNode(fset, expr.Cond), "{")

	printMatches(writer, fset, s)

	switch e := expr.Else.(type) {
	case nil:
		// No else block
		fmt.Fprintln(writer, "}")
	case *ast.BlockStmt:
		// Else block
		s := brenda.NewSolver(fset, info.Uses, info.Defs, expr.Cond, falseExpr...)
		s.SolveFalse()
		fmt.Fprintln(writer, "} else {")
		printMatches(writer, fset, s)
		fmt.Fprintln(writer, "}")
	case *ast.IfStmt:
		// Else if statement
		fmt.Fprint(writer, "} else ")
		falseExpr = append(falseExpr, expr.Cond)
		printIf(writer, fset, info, e, falseExpr...)
	}
	return nil
}

func printMatches(writer io.Writer, fset *token.FileSet, s *brenda.Solver) {
	if s.Impossible {
		fmt.Fprintln(writer, "\t// IMPOSSIBLE")
		return
	}
	var lines []string
	for ex, m := range s.Components {
		switch {
		case m.Match:
			lines = append(lines, fmt.Sprint("\t// ", printNode(fset, ex), " TRUE"))
		case m.Inverse:
			lines = append(lines, fmt.Sprint("\t// ", printNode(fset, ex), " FALSE"))
		default:
			lines = append(lines, fmt.Sprint("\t// ", printNode(fset, ex), " UNKNOWN"))
		}
	}
	sort.Strings(lines)
	fmt.Fprintln(writer, strings.Join(lines, "\n"))
}

func printNode(fset *token.FileSet, node ast.Node) string {
	buf := &bytes.Buffer{}
	err := format.Node(buf, fset, node)
	if err != nil {
		return err.Error()
	}
	return buf.String()
}

func unindent(s string) string {
	// first trim any \n
	s = strings.Trim(s, "\n")

	// then work out how the first line is indented
	loc := regexp.MustCompile("[^\t ]").FindStringIndex(s)
	if loc == nil {
		// string is empty?
		return s
	}

	indent := s[:loc[0]]
	if indent == "" {
		// string is not indented
		return s
	}
	return strings.Replace("\n"+s, "\n"+indent, "\n", -1)[1:]
}

func ExampleNewSolver_usage() {

	// A simple source file
	src := `package foo

	func foo(a, b bool) {
		if a { } else if b { } else { }
	}`

	// We parse the AST
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "foo.go", src, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	// We extract type info
	info := types.Info{
		Uses: make(map[*ast.Ident]types.Object),
		Defs: make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.Default()}
	if _, err = conf.Check("foo", fset, []*ast.File{f}, &info); err != nil {
		fmt.Println(err)
		return
	}

	// Walk the AST until we find the first *ast.IfStmt
	var ifs *ast.IfStmt
	ast.Inspect(f, func(node ast.Node) bool {
		if ifs != nil {
			return false
		}
		if n, ok := node.(*ast.IfStmt); ok && n != nil {
			ifs = n
			return false
		}
		return true
	})
	if ifs == nil {
		fmt.Println("No *ast.IfStmt found")
		return
	}

	var printIf func(*ast.IfStmt, ...ast.Expr) error
	var sprintResults func(*brenda.Solver) string
	var sprintNode func(ast.Node) string

	// This is called recursively for the if and all else-if statements. falseExpr
	// is a slice of all the conditions that came before an else-if statement,
	// which must all be false for the else-if to be reached.
	printIf = func(ifStmt *ast.IfStmt, falseExpr ...ast.Expr) error {

		s := brenda.NewSolver(fset, info.Uses, info.Defs, ifStmt.Cond, falseExpr...)
		err := s.SolveTrue()
		if err != nil {
			return err
		}

		fmt.Printf("if %s {\n%s\n}", sprintNode(ifStmt.Cond), sprintResults(s))

		switch e := ifStmt.Else.(type) {
		case *ast.BlockStmt:

			// Else block
			s := brenda.NewSolver(fset, info.Uses, info.Defs, ifStmt.Cond, falseExpr...)
			s.SolveFalse()

			fmt.Printf(" else {\n%s\n}", sprintResults(s))

		case *ast.IfStmt:

			// Else if statement
			fmt.Print(" else ")

			// Add the condition from the current if statement to the list of
			// false expressions for the else-if solver
			falseExpr = append(falseExpr, ifStmt.Cond)
			printIf(e, falseExpr...)

		}
		return nil
	}

	// Helper function to print results
	sprintResults = func(s *brenda.Solver) string {
		if s.Impossible {
			// If the expression is impossible
			return "\t// IMPOSSIBLE"
		}

		// The results must be sorted to ensure repeatable output
		var lines []string
		for expr, result := range s.Components {
			switch {
			case result.Match:
				lines = append(lines, fmt.Sprint("\t// ", printNode(fset, expr), " TRUE"))
			case result.Inverse:
				lines = append(lines, fmt.Sprint("\t// ", printNode(fset, expr), " FALSE"))
			default:
				lines = append(lines, fmt.Sprint("\t// ", printNode(fset, expr), " UNKNOWN"))
			}
		}
		sort.Strings(lines)
		return strings.Join(lines, "\n")
	}

	// Helper function to print AST nodes
	sprintNode = func(n ast.Node) string {
		buf := &bytes.Buffer{}
		err := format.Node(buf, fset, n)
		if err != nil {
			return err.Error()
		}
		return buf.String()
	}

	if err := printIf(ifs); err != nil {
		fmt.Println(err)
		return
	}

	// Output:
	// if a {
	// 	// a TRUE
	// } else if b {
	// 	// a FALSE
	// 	// b TRUE
	// } else {
	// 	// a FALSE
	// 	// b FALSE
	// }
}

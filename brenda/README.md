[![Build Status](https://travis-ci.org/dave/brenda.svg?branch=master)](https://travis-ci.org/dave/brenda) [![Go Report Card](https://goreportcard.com/badge/github.com/dave/brenda)](https://goreportcard.com/report/github.com/dave/brenda) [![codecov](https://codecov.io/gh/dave/brenda/branch/master/graph/badge.svg)](https://codecov.io/gh/dave/brenda)

# Brenda

Brenda is a boolean expression solver.

Given an AST expression containing an arbitrary combination of `!`, `&&` 
and `||` expressions, it is possible to solve the boolean state of certain 
components. For example:

```go
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
```

Some inputs may be unknown:

```go
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
```

Some branches may be impossible:

```go
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
```

Brenda supports complex components, and can detect the inverse use of `==`, `!=`, 
`<`, `>=`, `>` and `<=`:

```go
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
```

Here's an example of the full usage:

```go
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
info := &types.Info{Uses: make(map[*ast.Ident]types.Object)}
conf := types.Config{Importer: importer.Default()}
if _, err = conf.Check("foo", fset, []*ast.File{f}, info); err != nil {
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

	s := brenda.NewSolver(fset, info.Uses, ifStmt.Cond, falseExpr...)
	err := s.SolveTrue()
	if err != nil {
		return err
	}

	fmt.Printf("if %s {\n%s\n}", sprintNode(ifStmt.Cond), sprintResults(s))

	switch e := ifStmt.Else.(type) {
	case *ast.BlockStmt:

		// Else block
		s := brenda.NewSolver(fset, info.Uses, ifStmt.Cond, falseExpr...)
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
```

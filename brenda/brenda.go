// Package brenda is a boolean expression solver for Go AST
package brenda

//go:generate go get github.com/dave/rebecca/cmd/becca
//go:generate becca -package=github.com/dave/brenda

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"

	"github.com/dave/courtney/astrid"
	"github.com/pkg/errors"
)

// NewSolver returns a new *Solver. fset should be the AST FileSet. uses should
// be the Uses from go/types.Info. expression is the expression to solve.
// falseExpressions is a slice of expressions we know to be false - e.g. all
// previous conditions that came before an else-if statement.
func NewSolver(fset *token.FileSet, uses, defs map[*ast.Ident]types.Object, expression ast.Expr, falseExpressions ...ast.Expr) *Solver {
	return &Solver{
		fset:       fset,
		expr:       expression,
		falseExpr:  falseExpressions,
		itemUses:   make(map[ast.Expr]use),
		Components: make(map[ast.Expr]*Result),
		matcher:    astrid.NewMatcher(uses, defs),
	}
}

// Solver solves boolean expressions given the ast.Expr
type Solver struct {
	fset       *token.FileSet       // The AST FileSet providing position information
	expr       ast.Expr             // The main expression that we're analysing
	full       ast.Expr             // The expression combined with all known false expressions
	falseExpr  []ast.Expr           // Expressions known to be false (in an else-if statement)
	items      []ast.Expr           // The individual components of the full expression
	itemUses   map[ast.Expr]use     // Information about each use of each item in the full expression
	Components map[ast.Expr]*Result // Components is a map of all the individual components of the expression, and the results
	Impossible bool                 // Impossible is true if this expression is impossible
	matcher    *astrid.Matcher
}

type use struct {
	item     ast.Expr // This is an item in the Solver.items map
	inverted bool     // True if this use is the inverse of the item
}

// Result contains information about each result.
type Result struct {
	Match   bool // Match is true if this component must be true.
	Inverse bool // Inverse is true if this component must be false.
}

func (s *Solver) initFull(invert bool) {
	if s.expr == nil {
		// if the input expression is false, just return the false expressions
		if len(s.falseExpr) == 0 {
			return // panic?
		}
		if len(s.falseExpr) == 1 {
			s.full = astrid.Invert(s.falseExpr[0])
			return
		}
		out := astrid.Invert(s.falseExpr[0])
		for i := 1; i < len(s.falseExpr); i++ {
			out = &ast.BinaryExpr{X: out, Y: astrid.Invert(s.falseExpr[i]), Op: token.LAND}
		}
		s.full = out
		return
	}
	out := s.expr
	if invert {
		out = astrid.Invert(s.expr)
	}
	for _, prev := range s.falseExpr {
		out = &ast.BinaryExpr{
			X:  out,
			Op: token.LAND,
			Y:  astrid.Invert(prev),
		}
	}
	// only need to strip position info if we need to pretty-print the node:
	// strip(out)
	s.full = out
}

// SolveTrue solves the expression as true - e.g. for the main block of an if statement
func (s *Solver) SolveTrue() error {
	return s.solve(false)
}

// SolveFalse solves the expression as false - e.g. for the else block of an if statement
func (s *Solver) SolveFalse() error {
	return s.solve(true)
}

func (s *Solver) solve(invert bool) error {

	s.initFull(invert)
	if err := s.initItems(s.full); err != nil {
		return err
	}

	for _, c := range s.items {
		s.Components[c] = &Result{true, true}
	}

	found := false

	length := len(s.items)
	permutations := 1 << uint(length) // 1<<n === 2^n

	for i := 0; i < permutations; i++ {
		m := make(map[ast.Expr]bool)
		for j, c := range s.items {
			/*
				i is the bitmap of the current scenario e.g. 001001000
				j is the bit of the current item e.g. 3
				1<<uint(j) is the bitmap of the current item e.g. 000001000
				i&(1<<uint(j)) is them AND together.
				=> 0 if the current item is false, >0 if true.
			*/
			r := i&(1<<uint(j)) > 0
			m[c] = r
		}
		result := s.execute(s.full, m)
		if result {
			found = true
		}
		for j, c := range s.items {
			r := i&(1<<uint(j)) > 0
			if result {
				if r {
					s.Components[c].Inverse = false
				} else {
					s.Components[c].Match = false
				}
			}
		}
	}

	if !found {
		// if we didn't find any matches, clear all results and set impossible flag
		for _, v := range s.Components {
			v.Match = false
			v.Inverse = false
		}
		s.Impossible = true
	}
	return nil
}

func (s *Solver) initItems(node ast.Node) error {
	switch n := node.(type) {
	case *ast.BinaryExpr:
		switch n.Op {
		case token.LAND, token.LOR:
			if err := s.initItems(n.X); err != nil {
				return err
			}
			if err := s.initItems(n.Y); err != nil {
				return err
			}
		case token.EQL, token.LSS, token.GTR, token.NEQ, token.LEQ, token.GEQ:
			s.registerItem(n)
		default:
			s.registerItem(n)
		}
	case *ast.UnaryExpr:
		if err := s.initItems(n.X); err != nil {
			return err
		}
	case *ast.ParenExpr:
		if err := s.initItems(n.X); err != nil {
			return err
		}
	case ast.Expr:
		s.registerItem(n)
	default:
		return errors.Errorf("Unknown %T %s", node, s.sprintNode(node))
	}
	return nil
}

func (s *Solver) registerItem(e ast.Expr) {
	if boolTrue(e) || boolFalse(e) {
		// no need to register boolean literals
		return
	}
	for _, c := range s.items {
		if s.matcher.Match(c, e) {
			s.itemUses[e] = use{item: c, inverted: false}
			return
		}
		if s.matcher.Match(astrid.Invert(c), e) {
			s.itemUses[e] = use{item: c, inverted: true}
			return
		}
	}
	s.items = append(s.items, e)
	s.itemUses[e] = use{item: e, inverted: false}
}

func (s *Solver) execute(ex ast.Expr, inputs map[ast.Expr]bool) bool {
	switch e := ex.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND:
			return s.execute(e.X, inputs) && s.execute(e.Y, inputs)
		case token.LOR:
			return s.execute(e.X, inputs) || s.execute(e.Y, inputs)
		default:
			return s.evaluate(ex, inputs)
		}
	case *ast.UnaryExpr:
		switch e.Op {
		case token.NOT:
			return !s.execute(e.X, inputs)
		default:
			panic(fmt.Sprintf("unknown unary expression %s", s.sprintNode(ex)))
		}
	case *ast.ParenExpr:
		return s.execute(e.X, inputs)
	default:
		return s.evaluate(ex, inputs)
	}
}

func (s *Solver) evaluate(ex ast.Expr, inputs map[ast.Expr]bool) bool {
	if boolTrue(ex) {
		return true
	}
	if boolFalse(ex) {
		return false
	}
	use, ok := s.itemUses[ex]
	if !ok {
		panic(fmt.Sprintf("unknown item %s", s.sprintNode(ex)))
	}
	if use.inverted {
		return !inputs[use.item]
	}
	return inputs[use.item]
}

func (s *Solver) sprintNode(node ast.Node) string {
	buf := &bytes.Buffer{}
	err := format.Node(buf, s.fset, node)
	if err != nil {
		return err.Error()
	}
	return buf.String()
}

func boolTrue(v ast.Expr) bool {
	if id, ok := v.(*ast.Ident); ok {
		return id.Obj == nil && id.Name == "true"
	}
	return false
}

func boolFalse(v ast.Expr) bool {
	if id, ok := v.(*ast.Ident); ok {
		return id.Obj == nil && id.Name == "false"
	}
	return false
}

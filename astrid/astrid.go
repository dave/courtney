// Package astrid is a collection of AST utilities
package astrid

//go:generate go get github.com/dave/rebecca/cmd/becca
//go:generate becca -package=github.com/dave/astrid

import (
	"go/ast"
	"go/token"
	"go/types"
)

// NewMatcher returns a new *Matcher with the provided Uses and Defs from
// types.Info
func NewMatcher(uses, defs map[*ast.Ident]types.Object) *Matcher {
	return &Matcher{
		uses: uses,
		defs: defs,
	}
}

// Matcher matches ast expressions
type Matcher struct {
	uses map[*ast.Ident]types.Object
	defs map[*ast.Ident]types.Object
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

// Match determines whether two ast.Expr's are equivalent
func (m *Matcher) Match(a, b ast.Expr) bool {
	// are the expressions equal?
	switch at := a.(type) {
	case nil:
		return b == nil
	case *ast.Ident:
		if boolTrue(a) && boolTrue(b) {
			return true
		}
		if boolFalse(a) && boolFalse(b) {
			return true
		}
		if bt, ok := b.(*ast.Ident); ok {
			usea, isusea := m.uses[at]
			useb, isuseb := m.uses[bt]
			defa, isdefa := m.defs[at]
			defb, isdefb := m.defs[bt]
			switch {
			case isusea && isuseb && usea == useb,
				isdefa && isdefb && defa == defb,
				isdefa && isuseb && defa == useb,
				isusea && isdefb && usea == defb:
				return true
			}
			return false
		}
		return false
	case *ast.SelectorExpr:
		if bt, ok := b.(*ast.SelectorExpr); ok {
			return m.Match(at.Sel, bt.Sel) &&
				m.Match(at.X, bt.X)
		}
		return false
	case *ast.CallExpr:
		if bt, ok := b.(*ast.CallExpr); ok {
			return m.Match(at.Fun, bt.Fun) &&
				m.MatchSlice(at.Args, bt.Args) &&
				((at.Ellipsis == token.NoPos) == (bt.Ellipsis == token.NoPos))
		}
		return false
	case *ast.BasicLit:
		if bt, ok := b.(*ast.BasicLit); ok {
			return at.Kind == bt.Kind &&
				at.Value == bt.Value
		}
	case *ast.ParenExpr:
		if bt, ok := b.(*ast.ParenExpr); ok {
			return m.Match(at.X, bt.X)
		}
		return false
	case *ast.IndexExpr:
		if bt, ok := b.(*ast.IndexExpr); ok {
			return m.Match(at.X, bt.X) &&
				m.Match(at.Index, bt.Index)
		}
		return false
	case *ast.SliceExpr:
		if bt, ok := b.(*ast.SliceExpr); ok {
			return m.Match(at.X, bt.X) &&
				m.Match(at.High, bt.High) &&
				m.Match(at.Low, bt.Low) &&
				m.Match(at.Max, bt.Max) &&
				at.Slice3 == bt.Slice3
		}
		return false
	case *ast.TypeAssertExpr:
		if bt, ok := b.(*ast.TypeAssertExpr); ok {
			return m.Match(at.X, bt.X) &&
				m.Match(at.Type, bt.Type)
		}
		return false
	case *ast.StarExpr:
		if bt, ok := b.(*ast.StarExpr); ok {
			return m.Match(at.X, bt.X)
		}
		return false
	case *ast.UnaryExpr:
		if bt, ok := b.(*ast.UnaryExpr); ok {
			return m.Match(at.X, bt.X) &&
				at.Op == bt.Op
		}
		return false
	case *ast.BinaryExpr:
		if bt, ok := b.(*ast.BinaryExpr); ok {
			return m.Match(at.X, bt.X) &&
				m.Match(at.Y, bt.Y) &&
				at.Op == bt.Op
		}
		return false
	case *ast.Ellipsis:
		// notest
		if bt, ok := b.(*ast.Ellipsis); ok {
			return m.Match(at.Elt, bt.Elt)
		}
		return false
	case *ast.CompositeLit:
		if bt, ok := b.(*ast.CompositeLit); ok {
			return m.Match(at.Type, bt.Type) &&
				m.MatchSlice(at.Elts, bt.Elts)
		}
		return false
	case *ast.KeyValueExpr:
		if bt, ok := b.(*ast.KeyValueExpr); ok {
			return m.Match(at.Key, bt.Key) &&
				m.Match(at.Value, bt.Value)
		}
		return false
	case *ast.ArrayType:
		if bt, ok := b.(*ast.ArrayType); ok {
			return m.Match(at.Elt, bt.Elt) &&
				m.Match(at.Len, bt.Len)
		}
		return false
	case *ast.MapType:
		if bt, ok := b.(*ast.MapType); ok {
			return m.Match(at.Key, bt.Key) &&
				m.Match(at.Value, bt.Value)
		}
		return false
	case *ast.ChanType:
		if bt, ok := b.(*ast.ChanType); ok {
			return m.Match(at.Value, bt.Value) &&
				at.Dir == bt.Dir
		}
		return false
	case *ast.BadExpr, *ast.FuncLit, *ast.StructType, *ast.FuncType, *ast.InterfaceType:
		// can't be compared
		// notest
		return false
	}
	return false
}

// MatchSlice determines whether two slices of ast.Expr's are equivalent
func (m *Matcher) MatchSlice(a, b []ast.Expr) bool {
	if len(a) != len(b) {
		return false
	}
	for i, ae := range a {
		be := b[i]
		if !m.Match(ae, be) {
			return false
		}
	}
	return true
}

// Invert returns the inverse of the provided expression.
func Invert(node ast.Expr) ast.Expr {
	if be, ok := node.(*ast.BinaryExpr); ok && (be.Op == token.NEQ || be.Op == token.EQL || be.Op == token.LSS || be.Op == token.GTR || be.Op == token.LEQ || be.Op == token.GEQ) {
		/*
			EQL: ==
			NEQ: !=
			LSS: <
			GTR: >
			LEQ: <=
			GEQ: >=
		*/
		var op token.Token
		switch be.Op {
		case token.NEQ: //    !=
			op = token.EQL // ==
		case token.EQL: //    ==
			op = token.NEQ // !=
		case token.LSS: //    <
			op = token.GEQ // >=
		case token.GTR: //    >
			op = token.LEQ // <=
		case token.LEQ: //    <=
			op = token.GTR // >
		case token.GEQ: //    >=
			op = token.LSS // <
		}
		return &ast.BinaryExpr{
			X:  be.X,
			Op: op,
			Y:  be.Y,
		}
	} else if un, ok := node.(*ast.UnaryExpr); ok && un.Op == token.NOT {
		return un.X
	} else if boolTrue(node) {
		return ast.NewIdent("false")
	} else if boolFalse(node) {
		return ast.NewIdent("true")
	} else if _, ok := node.(*ast.Ident); ok {
		return &ast.UnaryExpr{
			Op: token.NOT,
			X:  node,
		}
	} else if _, ok := node.(*ast.ParenExpr); ok {
		return &ast.UnaryExpr{
			Op: token.NOT,
			X:  node,
		}
	}
	return &ast.UnaryExpr{
		Op: token.NOT,
		X:  &ast.ParenExpr{X: node},
	}
}

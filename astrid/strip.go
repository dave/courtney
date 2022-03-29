package astrid

import (
	"go/ast"
	"go/token"
)

// StripPosition removes all position information from a node and all
// descendants. This is useful before printing a modified node.
func StripPosition(node ast.Node) {
	// notest
	ast.Inspect(node, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		switch n := node.(type) {
		case *ast.Comment:
			n.Slash = token.NoPos
		case *ast.FieldList:
			n.Closing = token.NoPos
			n.Opening = token.NoPos
		case *ast.BadExpr:
			n.From = token.NoPos
			n.To = token.NoPos
		case *ast.Ident:
			n.NamePos = token.NoPos
		case *ast.BasicLit:
			n.ValuePos = token.NoPos
		case *ast.Ellipsis:
			n.Ellipsis = token.NoPos
		case *ast.CompositeLit:
			n.Lbrace = token.NoPos
			n.Rbrace = token.NoPos
		case *ast.ParenExpr:
			n.Lparen = token.NoPos
			n.Rparen = token.NoPos
		case *ast.IndexExpr:
			n.Lbrack = token.NoPos
			n.Rbrack = token.NoPos
		case *ast.SliceExpr:
			n.Rbrack = token.NoPos
			n.Lbrack = token.NoPos
		case *ast.TypeAssertExpr:
			n.Rparen = token.NoPos
			n.Lparen = token.NoPos
		case *ast.CallExpr:
			n.Rparen = token.NoPos
			n.Lparen = token.NoPos
			n.Ellipsis = token.NoPos
		case *ast.StarExpr:
			n.Star = token.NoPos
		case *ast.UnaryExpr:
			n.OpPos = token.NoPos
		case *ast.BinaryExpr:
			n.OpPos = token.NoPos
		case *ast.KeyValueExpr:
			n.Colon = token.NoPos
		case *ast.ArrayType:
			n.Lbrack = token.NoPos
		case *ast.StructType:
			n.Struct = token.NoPos
		case *ast.FuncType:
			n.Func = token.NoPos
		case *ast.InterfaceType:
			n.Interface = token.NoPos
		case *ast.MapType:
			n.Map = token.NoPos
		case *ast.ChanType:
			n.Arrow = token.NoPos
			n.Begin = token.NoPos
		case *ast.BadStmt:
			n.From = token.NoPos
			n.To = token.NoPos
		case *ast.EmptyStmt:
			n.Semicolon = token.NoPos
		case *ast.LabeledStmt:
			n.Colon = token.NoPos
		case *ast.SendStmt:
			n.Arrow = token.NoPos
		case *ast.IncDecStmt:
			n.TokPos = token.NoPos
		case *ast.AssignStmt:
			n.TokPos = token.NoPos
		case *ast.GoStmt:
			n.Go = token.NoPos
		case *ast.DeferStmt:
			n.Defer = token.NoPos
		case *ast.ReturnStmt:
			n.Return = token.NoPos
		case *ast.BranchStmt:
			n.TokPos = token.NoPos
		case *ast.BlockStmt:
			n.Lbrace = token.NoPos
			n.Rbrace = token.NoPos
		case *ast.IfStmt:
			n.If = token.NoPos
		case *ast.CaseClause:
			n.Case = token.NoPos
			n.Colon = token.NoPos
		case *ast.SwitchStmt:
			n.Switch = token.NoPos
		case *ast.TypeSwitchStmt:
			n.Switch = token.NoPos
		case *ast.CommClause:
			n.Colon = token.NoPos
			n.Case = token.NoPos
		case *ast.SelectStmt:
			n.Select = token.NoPos
		case *ast.ForStmt:
			n.For = token.NoPos
		case *ast.RangeStmt:
			n.For = token.NoPos
			n.TokPos = token.NoPos
		case *ast.ImportSpec:
			n.EndPos = token.NoPos
		case *ast.BadDecl:
			n.From = token.NoPos
			n.To = token.NoPos
		case *ast.GenDecl:
			n.TokPos = token.NoPos
			n.Lparen = token.NoPos
			n.Rparen = token.NoPos
		case *ast.File:
			n.Package = token.NoPos
		}
		return true
	})
}

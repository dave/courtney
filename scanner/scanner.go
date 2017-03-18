package scanner

import (
	"go/ast"
	"go/build"
	"go/constant"
	"go/token"

	"go/parser"

	"github.com/dave/brenda"
	"github.com/dave/courtney"
	"github.com/dave/patsy/vos"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

type CodeMap struct {
	env      vos.Env
	prog     *loader.Program
	Excludes map[string]map[int]bool
	paths    *courtney.PathCache
}

type PackageMap struct {
	path string
	name string
	code *CodeMap
	info *loader.PackageInfo
	prog *loader.Program
	fset *token.FileSet
}

type packageId struct {
	path string
	name string
}

func New(env vos.Env, paths *courtney.PathCache) *CodeMap {
	return &CodeMap{
		env:      env,
		Excludes: make(map[string]map[int]bool),
		paths:    paths,
	}
}

func (c *CodeMap) AddExclude(fpath string, line int) {
	if c.Excludes[fpath] == nil {
		c.Excludes[fpath] = make(map[int]bool)
	}
	c.Excludes[fpath][line] = true
}

func (c *CodeMap) LoadProgram(packages []courtney.PackageSpec) error {
	ctxt := build.Default
	ctxt.GOPATH = c.env.Getenv("GOPATH")
	wd, err := c.env.Getwd()
	if err != nil {
		return err
	}
	conf := loader.Config{Build: &ctxt, Cwd: wd, ParserMode: parser.ParseComments}

	for _, p := range packages {
		conf.Import(p.Path)
	}
	prog, err := conf.Load()
	if err != nil {
		return errors.Wrap(err, "Error loading config")
	}
	c.prog = prog
	return nil
}

func (c *CodeMap) ScanPackages() error {
	for _, p := range c.prog.Imported {
		pm := PackageMap{
			path: p.Pkg.Path(),
			name: p.Pkg.Name(),
			code: c,
			info: p,
			prog: c.prog,
			fset: c.prog.Fset,
		}
		if err := pm.FindExcludes(); err != nil {
			return err
		}
	}
	return nil
}

func (p *PackageMap) FindExcludes() error {
	for _, f := range p.info.Files {
		var err error
		ast.Inspect(f, func(node ast.Node) bool {
			if err != nil {
				return false
			}
			if b, inner := p.inspectNode(node); inner != nil {
				err = inner
				return false
			} else {
				return b
			}
		})
		if err != nil {
			return err
		}
		for _, cg := range f.Comments {
			p.inspectComment(f, cg)
		}
	}
	return nil
}

func (p *PackageMap) inspectComment(f *ast.File, cg *ast.CommentGroup) {
	for _, cm := range cg.List {
		if cm.Text == "//notest" || cm.Text == "// notest" {

			inside := func(node, holder ast.Node) bool {
				return node != nil && holder != nil && node.Pos() > holder.Pos() && node.Pos() <= holder.End()
			}
			var scope ast.Node
			ast.Inspect(f, func(node ast.Node) bool {
				if inside(cm, node) {
					scope = node
					return true
				}
				return false
			})

			comment := p.fset.Position(cm.Pos())
			start := p.fset.Position(scope.Pos())
			end := p.fset.Position(scope.End())
			for line := comment.Line; line < end.Line; line++ {
				p.code.AddExclude(start.Filename, line)
			}
		}
	}
}

func (p *PackageMap) inspectNode(node ast.Node) (bool, error) {
	if node == nil {
		return true, nil
	}
	switch n := node.(type) {
	case *ast.CallExpr:
		if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "panic" {
			pos := p.fset.Position(n.Pos())
			p.code.AddExclude(pos.Filename, pos.Line)
		}
	case *ast.IfStmt:
		if err := p.inspectIf(n); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (p *PackageMap) inspectIf(stmt *ast.IfStmt, falseExpr ...ast.Expr) error {

	// main if block
	s := brenda.NewSolver(p.fset, p.info.Uses, stmt.Cond, falseExpr...)
	if err := s.SolveTrue(); err != nil {
		return err
	}
	p.processResults(s, stmt.Body)

	switch e := stmt.Else.(type) {
	case *ast.BlockStmt:

		// else block
		s := brenda.NewSolver(p.fset, p.info.Uses, stmt.Cond, falseExpr...)
		if err := s.SolveFalse(); err != nil {
			return err
		}
		p.processResults(s, e)

	case *ast.IfStmt:

		// else if block
		falseExpr = append(falseExpr, stmt.Cond)
		if err := p.inspectIf(e, falseExpr...); err != nil {
			return err
		}
	}
	return nil
}

func (p *PackageMap) processResults(s *brenda.Solver, block *ast.BlockStmt) {
	for expr, match := range s.Components {
		if !match.Match && !match.Inverse {
			continue
		}
		found, op, expr := p.isErrorComparison(expr)
		if !found {
			continue
		}
		if op == token.NEQ && match.Match || op == token.EQL && match.Inverse {
			ast.Inspect(block, p.inspectNodeForReturn(expr))
			ast.Inspect(block, p.inspectNodeForWrap(block, expr))
		}
	}
}

func (p *PackageMap) isErrorComparison(e ast.Expr) (found bool, sign token.Token, expr ast.Expr) {
	if b, ok := e.(*ast.BinaryExpr); ok {
		if b.Op != token.NEQ && b.Op != token.EQL {
			return
		}
		xErr := p.isError(b.X)
		yNil := p.isNil(b.Y)
		if xErr && yNil {
			return true, b.Op, b.X
		}
		yErr := p.isError(b.Y)
		xNil := p.isNil(b.X)
		if yErr && xNil {
			return true, b.Op, b.Y
		}
	}
	return
}

func (p *PackageMap) inspectNodeForReturn(search ast.Expr) func(node ast.Node) bool {
	return func(node ast.Node) bool {
		if node == nil {
			return true
		}
		switch n := node.(type) {
		case *ast.ReturnStmt:
			if p.isErrorReturn(n, search) {
				pos := p.fset.Position(n.Pos())
				p.code.AddExclude(pos.Filename, pos.Line)
			}
		}
		return true
	}
}

func (p *PackageMap) inspectNodeForWrap(block *ast.BlockStmt, search ast.Expr) func(node ast.Node) bool {
	return func(node ast.Node) bool {
		if node == nil {
			return true
		}
		switch n := node.(type) {
		case *ast.DeclStmt:
			// covers the case:
			// var e = foo()
			// and
			// var e error = foo()
			gd, ok := n.Decl.(*ast.GenDecl)
			if !ok {
				return true
			}
			if gd.Tok != token.VAR {
				return true
			}
			if len(gd.Specs) != 1 {
				return true
			}
			spec, ok := gd.Specs[0].(*ast.ValueSpec)
			if !ok {
				return true
			}
			if len(spec.Names) != 1 || len(spec.Values) != 1 {
				return true
			}
			newSearch := spec.Names[0]

			if p.isErrorCall(spec.Values[0], search) {
				ast.Inspect(block, p.inspectNodeForReturn(newSearch))
			}

		case *ast.AssignStmt:
			if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
				return true
			}
			newSearch := n.Lhs[0]

			if p.isErrorCall(n.Rhs[0], search) {
				ast.Inspect(block, p.inspectNodeForReturn(newSearch))
			}
		}
		return true
	}
}

func (p *PackageMap) isErrorCall(expr, search ast.Expr) bool {
	n, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	if !p.isError(n) {
		return false
	}
	for _, arg := range n.Args {
		if p.matchExpr(arg, search) {
			return true
		}
	}
	return false
}

func (p *PackageMap) isErrorReturn(r *ast.ReturnStmt, search ast.Expr) bool {
	if len(r.Results) == 0 {
		return false
	}

	last := r.Results[len(r.Results)-1]

	// check the last result is an error
	if !p.isError(last) {
		return false
	}

	// check all the other results are nil or zero
	for i, v := range r.Results {
		if i == len(r.Results)-1 {
			// ignore the last item
			break
		}
		if !p.isZero(v) {
			return false
		}
	}

	return p.matchExpr(last, search) || p.isErrorCall(last, search)
}

func (p *PackageMap) matchExpr(a, b ast.Expr) bool {
	// are the expressions equal?
	switch at := a.(type) {
	case nil:
		return b == nil
	case *ast.Ident:
		if bt, ok := b.(*ast.Ident); ok {
			usea, isusea := p.info.Uses[at]
			useb, isuseb := p.info.Uses[bt]
			defa, isdefa := p.info.Defs[at]
			defb, isdefb := p.info.Defs[bt]
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
			return p.matchExpr(at.Sel, bt.Sel) &&
				p.matchExpr(at.X, bt.X)
		}
		return false
	case *ast.CallExpr:
		if bt, ok := b.(*ast.CallExpr); ok {
			return p.matchExpr(at.Fun, bt.Fun) &&
				p.matchExprs(at.Args, bt.Args) &&
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
			return p.matchExpr(at.X, bt.X)
		}
		return false
	case *ast.IndexExpr:
		if bt, ok := b.(*ast.IndexExpr); ok {
			return p.matchExpr(at.X, bt.X) &&
				p.matchExpr(at.Index, bt.Index)
		}
		return false
	case *ast.SliceExpr:
		if bt, ok := b.(*ast.SliceExpr); ok {
			return p.matchExpr(at.X, bt.X) &&
				p.matchExpr(at.High, bt.High) &&
				p.matchExpr(at.Low, bt.Low) &&
				p.matchExpr(at.Max, bt.Max) &&
				at.Slice3 == bt.Slice3
		}
		return false
	case *ast.TypeAssertExpr:
		if bt, ok := b.(*ast.TypeAssertExpr); ok {
			return p.matchExpr(at.X, bt.X) &&
				p.matchExpr(at.Type, bt.Type)
		}
		return false
	case *ast.StarExpr:
		if bt, ok := b.(*ast.StarExpr); ok {
			return p.matchExpr(at.X, bt.X)
		}
		return false
	case *ast.UnaryExpr:
		if bt, ok := b.(*ast.UnaryExpr); ok {
			return p.matchExpr(at.X, bt.X) &&
				at.Op == bt.Op
		}
		return false
	case *ast.BinaryExpr:
		if bt, ok := b.(*ast.BinaryExpr); ok {
			return p.matchExpr(at.X, bt.X) &&
				p.matchExpr(at.Y, bt.Y) &&
				at.Op == bt.Op
		}
		return false
	case *ast.Ellipsis:
		if bt, ok := b.(*ast.Ellipsis); ok {
			return p.matchExpr(at.Elt, bt.Elt)
		}
		return false
	case *ast.CompositeLit:
		if bt, ok := b.(*ast.CompositeLit); ok {
			return p.matchExpr(at.Type, bt.Type) &&
				p.matchExprs(at.Elts, bt.Elts)
		}
		return false
	case *ast.KeyValueExpr:
		if bt, ok := b.(*ast.KeyValueExpr); ok {
			return p.matchExpr(at.Key, bt.Key) &&
				p.matchExpr(at.Value, bt.Value)
		}
		return false
	case *ast.ArrayType:
		if bt, ok := b.(*ast.ArrayType); ok {
			return p.matchExpr(at.Elt, bt.Elt) &&
				p.matchExpr(at.Len, bt.Len)
		}
		return false
	case *ast.MapType:
		if bt, ok := b.(*ast.MapType); ok {
			return p.matchExpr(at.Key, bt.Key) &&
				p.matchExpr(at.Value, bt.Value)
		}
		return false
	case *ast.ChanType:
		if bt, ok := b.(*ast.ChanType); ok {
			return p.matchExpr(at.Value, bt.Value) &&
				at.Dir == bt.Dir
		}
		return false
	case *ast.BadExpr, *ast.FuncLit, *ast.StructType, *ast.FuncType, *ast.InterfaceType:
		// can't be compared
		return false
	}
	return false
}

func (p *PackageMap) matchExprs(a, b []ast.Expr) bool {
	if len(a) != len(b) {
		return false
	}
	for i, ae := range a {
		be := b[i]
		if !p.matchExpr(ae, be) {
			return false
		}
	}
	return true
}

func (p *PackageMap) isError(v ast.Expr) bool {
	t := p.info.Types[v]
	return t.Type.String() == "error" && t.Type.Underlying().String() == "interface{Error() string}"
}

func (p *PackageMap) isNil(v ast.Expr) bool {
	t := p.info.Types[v]
	return t.IsNil()
}

func (p *PackageMap) isZero(v ast.Expr) bool {
	t := p.info.Types[v]
	if t.IsNil() {
		return true
	}
	if t.Value != nil {
		// constant
		switch t.Value.Kind() {
		case constant.Bool:
			return constant.BoolVal(t.Value) == false
		case constant.String:
			return constant.StringVal(t.Value) == ""
		case constant.Int, constant.Float, constant.Complex:
			return constant.Sign(t.Value) == 0
		default:
			return false
		}
	}
	if t.IsValue() {
		if cl, ok := v.(*ast.CompositeLit); ok {
			for _, e := range cl.Elts {
				if kve, ok := e.(*ast.KeyValueExpr); ok {
					e = kve.Value
				}
				if !p.isZero(e) {
					return false
				}
			}
		}
	}
	return true
}

package scanner

import (
	"go/ast"
	"go/build"
	"go/constant"
	"go/token"
	"go/types"

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
		found, op, ident := p.isErrorComparison(expr)
		if !found {
			continue
		}
		o := p.info.Uses[ident]
		if op == token.NEQ && match.Match || op == token.EQL && match.Inverse {
			ast.Inspect(block, p.inspectNodeForReturn(o))
			ast.Inspect(block, p.inspectNodeForWrap(block, o))
		}
	}
}

func (p *PackageMap) isErrorComparison(e ast.Expr) (found bool, sign token.Token, ident *ast.Ident) {
	if b, ok := e.(*ast.BinaryExpr); ok {
		if b.Op != token.NEQ && b.Op != token.EQL {
			return
		}
		_, xId := b.X.(*ast.Ident)
		xErr := xId && p.isError(b.X)
		yNil := p.isNil(b.Y)
		if xErr && yNil {
			return true, b.Op, b.X.(*ast.Ident)
		}
		_, yId := b.Y.(*ast.Ident)
		yErr := yId && p.isError(b.Y)
		xNil := p.isNil(b.X)
		if yErr && xNil {
			return true, b.Op, b.Y.(*ast.Ident)
		}
	}
	return
}

func (p *PackageMap) inspectNodeForReturn(o types.Object) func(node ast.Node) bool {
	return func(node ast.Node) bool {
		if node == nil {
			return true
		}
		switch n := node.(type) {
		case *ast.ReturnStmt:
			if p.isErrorReturn(o, n) {
				pos := p.fset.Position(n.Pos())
				p.code.AddExclude(pos.Filename, pos.Line)
			}
		}
		return true
	}
}

func (p *PackageMap) inspectNodeForWrap(block *ast.BlockStmt, inputErrorObject types.Object) func(node ast.Node) bool {
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
			id := spec.Names[0]
			newErrorObject, ok := p.info.Defs[id]

			if p.isErrorCall(spec.Values[0], inputErrorObject) {
				ast.Inspect(block, p.inspectNodeForReturn(newErrorObject))
			}

		case *ast.AssignStmt:
			if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
				return true
			}
			id, ok := n.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			var newErrorObject types.Object
			switch n.Tok {
			case token.DEFINE:
				// covers the case:
				// e := foo()
				newErrorObject = p.info.Defs[id]
			case token.ASSIGN:
				// covers the case:
				// var e error
				// e = foo()
				newErrorObject = p.info.Uses[id]
			}

			if p.isErrorCall(n.Rhs[0], inputErrorObject) {
				ast.Inspect(block, p.inspectNodeForReturn(newErrorObject))
			}
		}
		return true
	}
}

func (p *PackageMap) isErrorCall(expr ast.Expr, inputErrorObject types.Object) bool {
	n, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	if !p.isError(n) {
		return false
	}
	for _, arg := range n.Args {
		if ident, ok := arg.(*ast.Ident); ok {
			if p.info.Uses[ident] == inputErrorObject {
				return true
			}
		}
	}
	return false
}

func (p *PackageMap) isErrorReturn(o types.Object, r *ast.ReturnStmt) bool {
	if len(r.Results) == 0 {
		return false
	}

	last := r.Results[len(r.Results)-1]

	if !p.isError(last) {
		return false
	}

	switch n := last.(type) {
	case *ast.Ident:
		// covers the case:
		// return err
		if o != p.info.Uses[n] {
			return false
		}
	case *ast.CallExpr:
		// covers the case:
		// var wrap func(error) error
		// return wrap(err)
		if !p.isErrorCall(n, o) {
			return false
		}
	default:
		return false
	}

	for i, v := range r.Results {
		if i == len(r.Results)-1 {
			// ignore the last item
			break
		}
		if !p.isZero(v) {
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

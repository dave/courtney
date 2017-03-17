package scanner

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/brenda"
)

type CodeMap struct {
	fset     *token.FileSet
	packages map[PackageId]*ast.Package
	types    map[PackageId]*types.Package
	info     *types.Info
	excludes map[string]map[int]bool
}

type PackageId struct {
	Path string
	Name string
}

func NewCodeMap() *CodeMap {
	return &CodeMap{
		fset:     token.NewFileSet(),
		packages: make(map[PackageId]*ast.Package),
		types:    make(map[PackageId]*types.Package),
		info: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		},
		excludes: make(map[string]map[int]bool),
	}
}

func (c *CodeMap) Excludes() map[string]map[int]bool {
	return c.excludes
}

func (c *CodeMap) ScanRecursive(ppath, dir string) error {
	return nil
}

func (c *CodeMap) ScanDir(ppath, dir string) error {

	fd, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer fd.Close()

	list, err := fd.Readdir(-1)
	if err != nil {
		return err
	}

	for _, f := range list {
		if strings.HasSuffix(f.Name(), ".go") {
			if err := c.ScanFile(ppath, dir, f.Name(), nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *CodeMap) ScanFile(ppath, dir, fname string, src interface{}) error {
	fpath := filepath.Join(dir, fname)
	f, err := parser.ParseFile(c.fset, fpath, src, parser.ParseComments)
	if err != nil {
		return err
	}

	id := PackageId{Path: ppath, Name: f.Name.Name}
	p, found := c.packages[id]
	if !found {
		p = &ast.Package{
			Name:  id.Name,
			Files: make(map[string]*ast.File),
		}
		c.packages[id] = p
	}
	p.Files[fpath] = f
	return nil
}

func (c *CodeMap) CheckTypes() error {
	for id, pkg := range c.packages {
		var conf types.Config
		var files []*ast.File
		for _, f := range pkg.Files {
			files = append(files, f)
		}
		conf.Importer = importer.Default()
		pkg, err := conf.Check(id.Path, c.fset, files, c.info)
		if err != nil {
			return err
		}
		c.types[id] = pkg
	}
	return nil
}

func (c *CodeMap) FindErrorReturns() error {
	for _, pkg := range c.packages {
		for _, f := range pkg.Files {
			var err error
			ast.Inspect(f, func(node ast.Node) bool {
				if err != nil {
					return false
				}
				if b, inner := c.inspectNodeForIf(node); inner != nil {
					err = inner
					return false
				} else {
					return b
				}
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CodeMap) inspectNodeForIf(node ast.Node) (bool, error) {
	if node == nil {
		return true, nil
	}
	switch n := node.(type) {
	case *ast.IfStmt:
		if err := c.inspectIf(n); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (c *CodeMap) inspectIf(stmt *ast.IfStmt, falseExpr ...ast.Expr) error {

	// main if block
	s := brenda.NewSolver(c.fset, c.info.Uses, stmt.Cond, falseExpr...)
	if err := s.SolveTrue(); err != nil {
		return err
	}
	c.processResults(s, stmt.Body)

	switch e := stmt.Else.(type) {
	case *ast.BlockStmt:

		// else block
		s := brenda.NewSolver(c.fset, c.info.Uses, stmt.Cond, falseExpr...)
		if err := s.SolveFalse(); err != nil {
			return err
		}
		c.processResults(s, e)

	case *ast.IfStmt:

		// else if block
		falseExpr = append(falseExpr, stmt.Cond)
		if err := c.inspectIf(e, falseExpr...); err != nil {
			return err
		}
	}
	return nil
}

func (c *CodeMap) processResults(s *brenda.Solver, block *ast.BlockStmt) {
	for expr, match := range s.Components {
		if !match.Match && !match.Inverse {
			continue
		}
		found, op, ident := c.isErrorComparison(expr)
		if !found {
			continue
		}
		o := c.info.Uses[ident]
		if op == token.NEQ && match.Match || op == token.EQL && match.Inverse {
			ast.Inspect(block, c.inspectNodeForReturn(o))
			ast.Inspect(block, c.inspectNodeForWrap(block, o))
		}
	}
}

func (c *CodeMap) isErrorComparison(e ast.Expr) (found bool, sign token.Token, ident *ast.Ident) {
	if b, ok := e.(*ast.BinaryExpr); ok {
		if b.Op != token.NEQ && b.Op != token.EQL {
			return
		}
		_, xId := b.X.(*ast.Ident)
		xErr := xId && c.isError(b.X)
		yNil := c.isNil(b.Y)
		if xErr && yNil {
			return true, b.Op, b.X.(*ast.Ident)
		}
		_, yId := b.Y.(*ast.Ident)
		yErr := yId && c.isError(b.Y)
		xNil := c.isNil(b.X)
		if yErr && xNil {
			return true, b.Op, b.Y.(*ast.Ident)
		}
	}
	return
}

func (c *CodeMap) inspectNodeForReturn(o types.Object) func(node ast.Node) bool {
	return func(node ast.Node) bool {
		if node == nil {
			return true
		}
		switch n := node.(type) {
		case *ast.ReturnStmt:
			if c.isErrorReturn(o, n) {
				p := c.fset.Position(n.Pos())
				if c.excludes[p.Filename] == nil {
					c.excludes[p.Filename] = make(map[int]bool)
				}
				c.excludes[p.Filename][p.Line] = true
			}
		}
		return true
	}
}

func (c *CodeMap) inspectNodeForWrap(block *ast.BlockStmt, inputErrorObject types.Object) func(node ast.Node) bool {
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
			newErrorObject, ok := c.info.Defs[id]

			if c.isErrorCall(spec.Values[0], inputErrorObject) {
				ast.Inspect(block, c.inspectNodeForReturn(newErrorObject))
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
				newErrorObject = c.info.Defs[id]
			case token.ASSIGN:
				// covers the case:
				// var e error
				// e = foo()
				newErrorObject = c.info.Uses[id]
			}

			if c.isErrorCall(n.Rhs[0], inputErrorObject) {
				ast.Inspect(block, c.inspectNodeForReturn(newErrorObject))
			}
		}
		return true
	}
}

func (c *CodeMap) isErrorCall(expr ast.Expr, inputErrorObject types.Object) bool {
	n, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	if !c.isError(n) {
		return false
	}
	for _, arg := range n.Args {
		if ident, ok := arg.(*ast.Ident); ok {
			if c.info.Uses[ident] == inputErrorObject {
				return true
			}
		}
	}
	return false
}

func (c *CodeMap) isErrorReturn(o types.Object, r *ast.ReturnStmt) bool {
	if len(r.Results) == 0 {
		return false
	}

	last := r.Results[len(r.Results)-1]

	if !c.isError(last) {
		return false
	}

	switch n := last.(type) {
	case *ast.Ident:
		// covers the case:
		// return err
		if o != c.info.Uses[n] {
			return false
		}
	case *ast.CallExpr:
		// covers the case:
		// var wrap func(error) error
		// return wrap(err)
		if !c.isErrorCall(n, o) {
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
		if !c.isZero(v) {
			return false
		}
	}

	return true
}

func (c *CodeMap) isError(v ast.Expr) bool {
	t := c.info.Types[v]
	return t.Type.String() == "error" && t.Type.Underlying().String() == "interface{Error() string}"
}

func (c *CodeMap) isNil(v ast.Expr) bool {
	t := c.info.Types[v]
	return t.IsNil()
}

func (c *CodeMap) isZero(v ast.Expr) bool {
	t := c.info.Types[v]
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
				if !c.isZero(e) {
					return false
				}
			}
		}
	}
	return true
}

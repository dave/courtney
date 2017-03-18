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
	*CodeMap
	info *loader.PackageInfo
	fset *token.FileSet
}

type FileMap struct {
	*PackageMap
	file *ast.File
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
		pm := &PackageMap{
			CodeMap: c,
			info:    p,
			fset:    c.prog.Fset,
		}
		if err := pm.ScanPackage(); err != nil {
			return err
		}
	}
	return nil
}

func (p *PackageMap) ScanPackage() error {
	for _, f := range p.info.Files {
		fm := &FileMap{
			PackageMap: p,
			file:       f,
		}
		if err := fm.FindExcludes(); err != nil {
			return err
		}
	}
	return nil
}

func (fm *FileMap) FindExcludes() error {
	var err error
	ast.Inspect(fm.file, func(node ast.Node) bool {
		if err != nil {
			return false
		}
		if b, inner := fm.inspectNode(node); inner != nil {
			err = inner
			return false
		} else {
			return b
		}
	})
	if err != nil {
		return err
	}
	for _, cg := range fm.file.Comments {
		fm.inspectComment(cg)
	}
	return nil
}

func (f *FileMap) findScope(node ast.Node, filter func(ast.Node) bool) ast.Node {
	inside := func(node, holder ast.Node) bool {
		return node != nil && holder != nil && node.Pos() > holder.Pos() && node.Pos() <= holder.End()
	}
	var scopes []ast.Node
	ast.Inspect(f.file, func(scope ast.Node) bool {
		if inside(node, scope) {
			scopes = append(scopes, scope)
			return true
		}
		return false
	})
	// find the last matching scope
	for i := len(scopes) - 1; i >= 0; i-- {
		if filter == nil || filter(scopes[i]) {
			return scopes[i]
		}
	}
	return nil
}

func (f *FileMap) inspectComment(cg *ast.CommentGroup) {
	for _, cm := range cg.List {
		if cm.Text != "//notest" && cm.Text != "// notest" {
			continue
		}

		// get the parent scope
		scope := f.findScope(cm, nil)

		// scope can be nil if the comment is in an empty file... in that
		// case we don't need any excludes.
		if scope != nil {
			comment := f.fset.Position(cm.Pos())
			start := f.fset.Position(scope.Pos())
			end := f.fset.Position(scope.End())
			for line := comment.Line; line < end.Line; line++ {
				f.AddExclude(start.Filename, line)
			}
		}
	}
}

func (f *FileMap) inspectNode(node ast.Node) (bool, error) {
	if node == nil {
		return true, nil
	}
	switch n := node.(type) {
	case *ast.CallExpr:
		if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "panic" {
			pos := f.fset.Position(n.Pos())
			f.AddExclude(pos.Filename, pos.Line)
		}
	case *ast.IfStmt:
		if err := f.inspectIf(n); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (f *FileMap) inspectIf(stmt *ast.IfStmt, falseExpr ...ast.Expr) error {

	// main if block
	s := brenda.NewSolver(f.fset, f.info.Uses, stmt.Cond, falseExpr...)
	if err := s.SolveTrue(); err != nil {
		return err
	}
	f.processResults(s, stmt.Body)

	switch e := stmt.Else.(type) {
	case *ast.BlockStmt:

		// else block
		s := brenda.NewSolver(f.fset, f.info.Uses, stmt.Cond, falseExpr...)
		if err := s.SolveFalse(); err != nil {
			return err
		}
		f.processResults(s, e)

	case *ast.IfStmt:

		// else if block
		falseExpr = append(falseExpr, stmt.Cond)
		if err := f.inspectIf(e, falseExpr...); err != nil {
			return err
		}
	}
	return nil
}

func (f *FileMap) processResults(s *brenda.Solver, block *ast.BlockStmt) {
	for expr, match := range s.Components {
		if !match.Match && !match.Inverse {
			continue
		}
		found, op, expr := f.isErrorComparison(expr)
		if !found {
			continue
		}
		if op == token.NEQ && match.Match || op == token.EQL && match.Inverse {
			ast.Inspect(block, f.inspectNodeForReturn(expr))
			ast.Inspect(block, f.inspectNodeForWrap(block, expr))
		}
	}
}

func (f *FileMap) isErrorComparison(e ast.Expr) (found bool, sign token.Token, expr ast.Expr) {
	if b, ok := e.(*ast.BinaryExpr); ok {
		if b.Op != token.NEQ && b.Op != token.EQL {
			return
		}
		xErr := f.isError(b.X)
		yNil := f.isNil(b.Y)
		if xErr && yNil {
			return true, b.Op, b.X
		}
		yErr := f.isError(b.Y)
		xNil := f.isNil(b.X)
		if yErr && xNil {
			return true, b.Op, b.Y
		}
	}
	return
}

func (f *FileMap) inspectNodeForReturn(search ast.Expr) func(node ast.Node) bool {
	return func(node ast.Node) bool {
		if node == nil {
			return true
		}
		switch n := node.(type) {
		case *ast.ReturnStmt:
			if f.isErrorReturn(n, search) {
				pos := f.fset.Position(n.Pos())
				f.AddExclude(pos.Filename, pos.Line)
			}
		}
		return true
	}
}

func (f *FileMap) inspectNodeForWrap(block *ast.BlockStmt, search ast.Expr) func(node ast.Node) bool {
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

			if f.isErrorCall(spec.Values[0], search) {
				ast.Inspect(block, f.inspectNodeForReturn(newSearch))
			}

		case *ast.AssignStmt:
			if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
				return true
			}
			newSearch := n.Lhs[0]

			if f.isErrorCall(n.Rhs[0], search) {
				ast.Inspect(block, f.inspectNodeForReturn(newSearch))
			}
		}
		return true
	}
}

func (f *FileMap) isErrorCall(expr, search ast.Expr) bool {
	n, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	if !f.isError(n) {
		return false
	}
	for _, arg := range n.Args {
		if f.matchExpr(arg, search) {
			return true
		}
	}
	return false
}

func (f *FileMap) isErrorReturnNamedResultParameters(r *ast.ReturnStmt, search ast.Expr) bool {
	// covers the syntax:
	// func a() (err error) {
	// 	if err != nil {
	// 		return
	// 	}
	// }
	scope := f.findScope(r, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return true
		}
		return false
	})
	var t *ast.FuncType
	switch s := scope.(type) {
	case *ast.FuncDecl:
		t = s.Type
	case *ast.FuncLit:
		t = s.Type
	}
	last := t.Results.List[len(t.Results.List)-1]
	if last.Names == nil {
		// anonymous
		return false
	}
	id := last.Names[len(last.Names)-1]
	return f.matchExpr(id, search)
}

func (f *FileMap) isErrorReturn(r *ast.ReturnStmt, search ast.Expr) bool {
	if len(r.Results) == 0 {
		return f.isErrorReturnNamedResultParameters(r, search)
	}

	last := r.Results[len(r.Results)-1]

	// check the last result is an error
	if !f.isError(last) {
		return false
	}

	// check all the other results are nil or zero
	for i, v := range r.Results {
		if i == len(r.Results)-1 {
			// ignore the last item
			break
		}
		if !f.isZero(v) {
			return false
		}
	}

	return f.matchExpr(last, search) || f.isErrorCall(last, search)
}

func (f *FileMap) matchExpr(a, b ast.Expr) bool {
	// are the expressions equal?
	switch at := a.(type) {
	case nil:
		return b == nil
	case *ast.Ident:
		if bt, ok := b.(*ast.Ident); ok {
			usea, isusea := f.info.Uses[at]
			useb, isuseb := f.info.Uses[bt]
			defa, isdefa := f.info.Defs[at]
			defb, isdefb := f.info.Defs[bt]
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
			return f.matchExpr(at.Sel, bt.Sel) &&
				f.matchExpr(at.X, bt.X)
		}
		return false
	case *ast.CallExpr:
		if bt, ok := b.(*ast.CallExpr); ok {
			return f.matchExpr(at.Fun, bt.Fun) &&
				f.matchExprs(at.Args, bt.Args) &&
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
			return f.matchExpr(at.X, bt.X)
		}
		return false
	case *ast.IndexExpr:
		if bt, ok := b.(*ast.IndexExpr); ok {
			return f.matchExpr(at.X, bt.X) &&
				f.matchExpr(at.Index, bt.Index)
		}
		return false
	case *ast.SliceExpr:
		if bt, ok := b.(*ast.SliceExpr); ok {
			return f.matchExpr(at.X, bt.X) &&
				f.matchExpr(at.High, bt.High) &&
				f.matchExpr(at.Low, bt.Low) &&
				f.matchExpr(at.Max, bt.Max) &&
				at.Slice3 == bt.Slice3
		}
		return false
	case *ast.TypeAssertExpr:
		if bt, ok := b.(*ast.TypeAssertExpr); ok {
			return f.matchExpr(at.X, bt.X) &&
				f.matchExpr(at.Type, bt.Type)
		}
		return false
	case *ast.StarExpr:
		if bt, ok := b.(*ast.StarExpr); ok {
			return f.matchExpr(at.X, bt.X)
		}
		return false
	case *ast.UnaryExpr:
		if bt, ok := b.(*ast.UnaryExpr); ok {
			return f.matchExpr(at.X, bt.X) &&
				at.Op == bt.Op
		}
		return false
	case *ast.BinaryExpr:
		if bt, ok := b.(*ast.BinaryExpr); ok {
			return f.matchExpr(at.X, bt.X) &&
				f.matchExpr(at.Y, bt.Y) &&
				at.Op == bt.Op
		}
		return false
	case *ast.Ellipsis:
		if bt, ok := b.(*ast.Ellipsis); ok {
			return f.matchExpr(at.Elt, bt.Elt)
		}
		return false
	case *ast.CompositeLit:
		if bt, ok := b.(*ast.CompositeLit); ok {
			return f.matchExpr(at.Type, bt.Type) &&
				f.matchExprs(at.Elts, bt.Elts)
		}
		return false
	case *ast.KeyValueExpr:
		if bt, ok := b.(*ast.KeyValueExpr); ok {
			return f.matchExpr(at.Key, bt.Key) &&
				f.matchExpr(at.Value, bt.Value)
		}
		return false
	case *ast.ArrayType:
		if bt, ok := b.(*ast.ArrayType); ok {
			return f.matchExpr(at.Elt, bt.Elt) &&
				f.matchExpr(at.Len, bt.Len)
		}
		return false
	case *ast.MapType:
		if bt, ok := b.(*ast.MapType); ok {
			return f.matchExpr(at.Key, bt.Key) &&
				f.matchExpr(at.Value, bt.Value)
		}
		return false
	case *ast.ChanType:
		if bt, ok := b.(*ast.ChanType); ok {
			return f.matchExpr(at.Value, bt.Value) &&
				at.Dir == bt.Dir
		}
		return false
	case *ast.BadExpr, *ast.FuncLit, *ast.StructType, *ast.FuncType, *ast.InterfaceType:
		// can't be compared
		return false
	}
	return false
}

func (f *FileMap) matchExprs(a, b []ast.Expr) bool {
	if len(a) != len(b) {
		return false
	}
	for i, ae := range a {
		be := b[i]
		if !f.matchExpr(ae, be) {
			return false
		}
	}
	return true
}

func (f *FileMap) isError(v ast.Expr) bool {
	t := f.info.Types[v]
	return t.Type.String() == "error" && t.Type.Underlying().String() == "interface{Error() string}"
}

func (f *FileMap) isNil(v ast.Expr) bool {
	t := f.info.Types[v]
	return t.IsNil()
}

func (f *FileMap) isZero(v ast.Expr) bool {
	t := f.info.Types[v]
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
				if !f.isZero(e) {
					return false
				}
			}
		}
	}
	return true
}

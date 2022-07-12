package scanner

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"github.com/dave/astrid"
	"github.com/dave/brenda"
	"github.com/dave/courtney/shared"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

// CodeMap scans a number of packages for code to exclude
type CodeMap struct {
	setup    *shared.Setup
	pkgs     []*packages.Package
	Excludes map[string]map[int]bool
}

// PackageMap scans a single package for code to exclude
type PackageMap struct {
	*CodeMap
	pkg  *packages.Package
	fset *token.FileSet
}

// FileMap scans a single file for code to exclude
type FileMap struct {
	*PackageMap
	file    *ast.File
	matcher *astrid.Matcher
}

type packageId struct {
	path string
	name string
}

// New returns a CoseMap with the provided setup
func New(setup *shared.Setup) *CodeMap {
	return &CodeMap{
		setup:    setup,
		Excludes: make(map[string]map[int]bool),
	}
}

func (c *CodeMap) addExclude(fpath string, line int) {
	if c.Excludes[fpath] == nil {
		c.Excludes[fpath] = make(map[int]bool)
	}
	c.Excludes[fpath][line] = true
}

// LoadProgram uses the loader package to load and process the source for a
// number or packages.
func (c *CodeMap) LoadProgram() error {
	var patterns []string
	for _, p := range c.setup.Packages {
		patterns = append(patterns, p.Path)
	}
	wd, err := c.setup.Env.Getwd()
	if err != nil {
		return errors.WithStack(err)
	}

	cfg := &packages.Config{
		Dir: wd,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Env: c.setup.Env.Environ(),
	}

	// add a recover to catch a panic and add some context to the error
	defer func() {
		if panicErr := recover(); panicErr != nil {
			panic(fmt.Sprintf("%+v", errors.Errorf("Panic in packages.Load: %s", panicErr)))
		}
	}()

	pkgs, err := packages.Load(cfg, patterns...)
	/*
		ctxt := build.Default
		ctxt.GOPATH = c.setup.Env.Getenv("GOPATH")

		conf := loader.Config{Build: &ctxt, Cwd: wd, ParserMode: parser.ParseComments}

		for _, p := range c.setup.Packages {
			conf.Import(p.Path)
		}
		prog, err := conf.Load()
	*/
	if err != nil {
		return errors.Wrap(err, "Error loading config")
	}
	c.pkgs = pkgs
	return nil
}

// ScanPackages scans the imported packages
func (c *CodeMap) ScanPackages() error {
	for _, p := range c.pkgs {
		pm := &PackageMap{
			CodeMap: c,
			pkg:     p,
			fset:    p.Fset,
		}
		if err := pm.ScanPackage(); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

// ScanPackage scans a single package
func (p *PackageMap) ScanPackage() error {
	for _, f := range p.pkg.Syntax {

		fm := &FileMap{
			PackageMap: p,
			file:       f,
			matcher:    astrid.NewMatcher(p.pkg.TypesInfo.Uses, p.pkg.TypesInfo.Defs),
		}
		if err := fm.FindExcludes(); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

// FindExcludes scans a single file to find code to exclude from coverage files
func (f *FileMap) FindExcludes() error {
	var err error

	ast.Inspect(f.file, func(node ast.Node) bool {
		if err != nil {
			// notest
			return false
		}
		b, inner := f.inspectNode(node)
		if inner != nil {
			// notest
			err = inner
			return false
		}
		return b
	})
	if err != nil {
		return errors.WithStack(err)
	}
	for _, cg := range f.file.Comments {
		f.inspectComment(cg)
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
	// notest
	return nil
}

func (f *FileMap) inspectComment(cg *ast.CommentGroup) {
	for _, cm := range cg.List {
		if !strings.HasPrefix(cm.Text, "//notest") && !strings.HasPrefix(cm.Text, "// notest") {
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
			endLine := end.Line
			if _, ok := scope.(*ast.CaseClause); ok {
				// case block needs an extra line...
				endLine++
			}
			for line := comment.Line; line < endLine; line++ {
				f.addExclude(start.Filename, line)
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
			f.addExclude(pos.Filename, pos.Line)
		}
	case *ast.IfStmt:
		if err := f.inspectIf(n); err != nil {
			return false, err
		}
	case *ast.SwitchStmt:
		if n.Tag != nil {
			// we are only concerned with switch statements with no tag
			// expression e.g. switch { ... }
			return true, nil
		}
		var falseExpr []ast.Expr
		var defaultClause *ast.CaseClause
		for _, s := range n.Body.List {
			cc := s.(*ast.CaseClause)
			if cc.List == nil {
				// save the default clause until the end
				defaultClause = cc
				continue
			}
			if err := f.inspectCase(cc, falseExpr...); err != nil {
				return false, err
			}
			falseExpr = append(falseExpr, f.boolOr(cc.List))
		}
		if defaultClause != nil {
			if err := f.inspectCase(defaultClause, falseExpr...); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func (f *FileMap) inspectCase(stmt *ast.CaseClause, falseExpr ...ast.Expr) error {
	s := brenda.NewSolver(f.fset, f.pkg.TypesInfo.Uses, f.pkg.TypesInfo.Defs, f.boolOr(stmt.List), falseExpr...)
	if err := s.SolveTrue(); err != nil {
		return errors.WithStack(err)
	}
	f.processResults(s, &ast.BlockStmt{List: stmt.Body})
	return nil
}

func (f *FileMap) boolOr(list []ast.Expr) ast.Expr {
	if len(list) == 0 {
		return nil
	}
	if len(list) == 1 {
		return list[0]
	}
	current := list[0]
	for i := 1; i < len(list); i++ {
		current = &ast.BinaryExpr{X: current, Y: list[i], Op: token.LOR}
	}
	return current
}

func (f *FileMap) inspectIf(stmt *ast.IfStmt, falseExpr ...ast.Expr) error {

	// main if block
	s := brenda.NewSolver(f.fset, f.pkg.TypesInfo.Uses, f.pkg.TypesInfo.Defs, stmt.Cond, falseExpr...)
	if err := s.SolveTrue(); err != nil {
		return errors.WithStack(err)
	}
	f.processResults(s, stmt.Body)

	switch e := stmt.Else.(type) {
	case *ast.BlockStmt:

		// else block
		s := brenda.NewSolver(f.fset, f.pkg.TypesInfo.Uses, f.pkg.TypesInfo.Defs, stmt.Cond, falseExpr...)
		if err := s.SolveFalse(); err != nil {
			return errors.WithStack(err)
		}
		f.processResults(s, e)

	case *ast.IfStmt:

		// else if block
		falseExpr = append(falseExpr, stmt.Cond)
		if err := f.inspectIf(e, falseExpr...); err != nil {
			return errors.WithStack(err)
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
				f.addExclude(pos.Filename, pos.Line)
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
				// notest
				return true
			}
			if gd.Tok != token.VAR {
				// notest
				return true
			}
			if len(gd.Specs) != 1 {
				// notest
				return true
			}
			spec, ok := gd.Specs[0].(*ast.ValueSpec)
			if !ok {
				// notest
				return true
			}
			if len(spec.Names) != 1 || len(spec.Values) != 1 {
				// notest
				return true
			}
			newSearch := spec.Names[0]

			if f.isErrorCall(spec.Values[0], search) {
				ast.Inspect(block, f.inspectNodeForReturn(newSearch))
			}

		case *ast.AssignStmt:
			if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
				// notest
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
		// never gets here, but leave it in for completeness
		// notest
		return false
	}
	for _, arg := range n.Args {
		if f.matcher.Match(arg, search) {
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
	if t.Results == nil {
		return false
	}
	last := t.Results.List[len(t.Results.List)-1]
	if last.Names == nil {
		// anonymous returns - shouldn't be able to get here because a bare
		// return statement with either have zero results or named results.
		// notest
		return false
	}
	id := last.Names[len(last.Names)-1]
	return f.matcher.Match(id, search)
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

	return f.matcher.Match(last, search) || f.isErrorCall(last, search)
}

func (f *FileMap) isError(v ast.Expr) bool {
	if n, ok := f.pkg.TypesInfo.TypeOf(v).(*types.Named); ok {
		o := n.Obj()
		return o != nil && o.Pkg() == nil && o.Name() == "error"
	}
	return false
}

func (f *FileMap) isNil(v ast.Expr) bool {
	t := f.pkg.TypesInfo.Types[v]
	return t.IsNil()
}

func (f *FileMap) isZero(v ast.Expr) bool {
	t := f.pkg.TypesInfo.Types[v]
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
			// notest
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

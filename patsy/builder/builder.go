// package builder can be used in testing to create a temporary go module or gopath, src,
// namespace and package directory, and populate it with source files.
package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dave/courtney/patsy/vos"
	"github.com/pkg/errors"
)

// New creates a new temporary location, either for a go module or for a gopath root.
// See NewGoModule or NewGoRoot for the details.
// Remember to defer the Cleanup() method to delete the temporary files.
func New(env vos.Env, namespace string, gomod bool) (*Builder, error) {
	if gomod {
		return NewGoModule(env, namespace)
	} else {
		return NewGoRoot(env, namespace)
	}
}

// NewGoRoot creates a new gopath in the system temporary location, creates the src
// dir and the namespace dir. The gopath is appended to the beginning of the
// existing gopath, so existing imports will still work.
// Remember to defer the Cleanup() method to delete the temporary files.
func NewGoRoot(env vos.Env, namespace string) (*Builder, error) {
	gopath, err := ioutil.TempDir("", "go")
	if err != nil {
		return nil, errors.Wrap(err, "Error creating temporary gopath root dir")
	}

	// gopath needs to match what `go list` will be returning, so eval symlinks and clean
	gopath, err = filepath.EvalSymlinks(filepath.Clean(gopath))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	root := filepath.Join(gopath, "src", namespace)

	b := &Builder{
		env:       env,
		gopath:    gopath,
		root:      root,
		namespace: namespace,
		gomod:     false,
	}

	if err := os.Mkdir(filepath.Join(gopath, "src"), os.FileMode(0777)); err != nil {
		b.Cleanup()
		return nil, errors.Wrap(err, "Error creating temporary gopath src dir")
	}
	if err := os.MkdirAll(root, os.FileMode(0777)); err != nil {
		b.Cleanup()
		return nil, errors.Wrap(err, "Error creating temporary namespace dir")
	}

	// from go1.16 onwards we need to explicitly turn go modules off in GOROOT mode
	err = b.env.Setenv("GO111MODULE", "off")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = b.env.Setenv("GOPATH", gopath+string(filepath.ListSeparator)+b.env.Getenv("GOPATH"))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// change dir to root to ensure consistent behaviour
	_ = os.Chdir(gopath)
	_ = b.env.Setwd(gopath)

	return b, nil
}

// NewGoModule creates a new go module root in the system temporary location, creates the root dir
// and the go.mod file. Remember to defer the Cleanup() method to delete the temporary files.
func NewGoModule(env vos.Env, namespace string) (*Builder, error) {
	root, err := ioutil.TempDir("", "go")
	if err != nil {
		return nil, errors.Wrap(err, "Error creating temporary gopath root dir")
	}

	// root needs to match what `go list` will be returning, so eval symlinks and clean
	root, err = filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	b := &Builder{
		env:       env,
		gopath:    "",
		root:      root,
		namespace: namespace,
		gomod:     true,
	}

	err = b.env.Setenv("GOPATH", "")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	gomodFile := fmt.Sprintf("module %s", namespace)
	err = ioutil.WriteFile(
		filepath.Join(root, "go.mod"), []byte(gomodFile), os.FileMode(0666))
	if err != nil {
		return nil, errors.Wrap(err, "Error creating temporary go.mod file")
	}

	// change dir to root to ensure consistent behaviour
	_ = os.Chdir(root)
	_ = b.env.Setwd(root)

	return b, nil
}

// Builder can be used in testing to create a temporary go module or gopath, src, namespace
// and package directory, and populate it with source files.
type Builder struct {
	env       vos.Env // mockable environment
	gopath    string  // temporary gopath root dir
	root      string  // temporary root dir for namespace
	namespace string  // temporary namespace
	gomod     bool    // gomodules enabled or not
}

// Root returns the temporary gopath root dir.
func (b *Builder) Root() string {
	return b.root
}

// File creates a new source file in the package.
func (b *Builder) File(packageName, filename, contents string) error {
	dir := filepath.Join(b.root, packageName)
	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		// most editors will indent multi line strings in Go source with
		// tabs, so we convert to spaces for yaml files.
		contents = strings.Replace(contents, "\t", "    ", -1)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, filename), []byte(contents), 0777); err != nil {
		return errors.Wrapf(err, "Error creating temporary source file %s", filename)
	}
	return nil
}

// Package creates a new package and populates with source files.
func (b *Builder) Package(packageName string, files map[string]string) (packagePath string, packageDir string, err error) {
	dir := filepath.Join(b.root, packageName)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return "", "", errors.Wrap(err, "Error creating temporary package dir")
	}

	if files != nil {
		for filename, contents := range files {
			if err := b.File(packageName, filename, contents); err != nil {
				return "", "", err
			}
		}
	}

	return path.Join(b.namespace, packageName), dir, nil
}

// Cleanup deletes all temporary files.
func (b *Builder) Cleanup() {
	_ = os.RemoveAll(b.root)
}

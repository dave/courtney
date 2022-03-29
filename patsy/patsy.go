// package patsy is a package helper utility. It allows the conversion of go
// package paths to filesystem directories and vice versa.
package patsy

//go:generate go get github.com/dave/rebecca/cmd/becca
//go:generate becca -package=github.com/dave/patsy

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dave/courtney/patsy/vos"
	"github.com/pkg/errors"
)

// Name returns the package name for a given path and src dir. Note that
// the src dir (e.g. working dir) is required because multiple vendored
// packages can correspond to the same path when accessed from different dirs.
func Name(env vos.Env, packagePath string, srcDir string) (string, error) {
	c := build.Default
	c.GOPATH = env.Getenv("GOPATH")

	// change dir to the srcDir because go/build uses os.Getwd when go modules is enabled
	err := os.Chdir(srcDir)
	if err != nil {
		return "", errors.WithStack(err)
	}

	// go/build package relies on `os.Getenv` ... and this causes issues in go1.15
	// so we have to set and unset GO111MODULE=off from our vos.Env
	go111Mod := env.Getenv("GO111MODULE")
	originalGo111Mod := os.Getenv("GO111MODULE")
	if go111Mod != originalGo111Mod {
		_ = os.Setenv("GO111MODULE", go111Mod)
		defer func() { _ = os.Setenv("GO111MODULE", originalGo111Mod) }()
	}

	p, err := c.Import(packagePath, srcDir, 0)
	if err != nil {
		return "", errors.Wrapf(err, "importing %s", packagePath)
	}
	return p.Name, nil
}

// Dir returns the filesystem path for the directory corresponding to the go
// package path provided.
func Dir(env vos.Env, packagePath string) (string, error) {
	// use Dirs internally to find the directory
	dirs, err := Dirs(env, packagePath)
	if err == nil {
		dir, ok := dirs[packagePath]
		if ok {
			return dir, nil
		}
	}

	// The go list command will throw an error if the package directory is
	// empty. In this case we need to explore the filesystem to see if there is
	// a directory in <gopath>/src/<package-path>. Remember there can be
	// several gopaths. We return the first matching directory.
	if env.Getenv("GOPATH") != "" {
		for _, gopath := range filepath.SplitList(env.Getenv("GOPATH")) {
			dir := filepath.Join(gopath, "src", packagePath)
			if s, err := os.Stat(dir); err == nil && s.IsDir() {
				return dir, nil
			}
		}
	}

	return "", errors.Errorf("Dir not found for %s", packagePath)
}

// Dirs returns the filesystem path for all packages under the directory corresponding to the go
// package path provided.
func Dirs(env vos.Env, packagePath string) (map[string]string, error) {
	wd, err := env.Getwd()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	exe := exec.Command("go", "list", "-f", "{{.ImportPath}}:{{.Dir}}", packagePath)
	exe.Dir = wd
	exe.Env = env.Environ()
	out, err := exe.CombinedOutput()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	lines := strings.Split(string(out), "\n")

	result := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "go: warning:") {
			continue
		}

		chunks := strings.Split(strings.TrimSpace(line), ":")
		importPath := chunks[0]
		dir := chunks[1]

		result[importPath] = dir
	}

	return result, nil
}

// Path returns the go package path corresponding to the filesystem directory
// provided.
func Path(env vos.Env, packageDir string) (string, error) {
	// packageDir needs to match what `go list` will be returning, so eval symlinks and clean
	packageDir, err := filepath.EvalSymlinks(filepath.Clean(packageDir))
	if err != nil {
		return "", errors.WithStack(err)
	}

	// use Dirs internally
	dirs, err := Dirs(env, packageDir)
	if err == nil {
		for ppath, dir := range dirs {
			if dir == packageDir {
				return ppath, nil
			}
		}
	}

	// The go list command will throw an error if the package directory is
	// empty. In this case we need to explore the filesystem to see if there is
	// a directory in <gopath>/src/<package-path>. Remember there can be
	// several gopaths. We return the first matching directory.
	if env.Getenv("GOPATH") != "" {
		for _, gopath := range filepath.SplitList(env.Getenv("GOPATH")) {
			if strings.HasPrefix(packageDir, gopath) {
				rel, inner := filepath.Rel(filepath.Join(gopath, "src"), packageDir)
				if inner == nil && rel != "" {
					// Remember we're returning a package path, which uses forward
					// slashes even on windows
					return filepath.ToSlash(rel), nil
				}
			}
		}
	}

	return "", errors.Errorf("Package not found for %s", packageDir)
}

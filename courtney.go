package courtney

import (
	"os"
	"strings"

	"path/filepath"

	"path"

	"github.com/dave/patsy"
	"github.com/dave/patsy/vos"
)

type PackageSpec struct {
	Dir  string
	Path string
}

func ParseArgs(env vos.Env, paths *PathCache, args ...string) ([]PackageSpec, error) {
	packages := map[string]string{}
	for _, ppath := range args {
		var dir string
		recursive := false
		if strings.HasSuffix(ppath, "/...") {
			ppath = strings.TrimSuffix(ppath, "/...")
			recursive = true
		}
		if strings.HasSuffix(ppath, "/") {
			ppath = strings.TrimSuffix(ppath, "/")
		}
		if ppath == "." {
			var err error
			dir, err = env.Getwd()
			if err != nil {
				return nil, err
			}
			ppath, err = paths.GetPackageFromDir(dir)
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			dir, err = paths.GetDirFromPackage(ppath)
			if err != nil {
				return nil, err
			}
		}
		if !recursive {
			packages[ppath] = dir
		}
		if recursive {
			dirs := map[string]bool{}
			filepath.Walk(dir, func(fpath string, info os.FileInfo, err error) error {
				if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
					// Scan until we find a Go source file. Record the dir and
					// skip the rest of the dir
					fdir, _ := filepath.Split(fpath)
					dirs[fdir] = true
					return nil
				}
				return nil
			})
			for dir := range dirs {
				ppath, err := paths.GetPackageFromDir(dir)
				if err != nil {
					return nil, err
				}
				packages[ppath] = dir
			}
		}
	}
	var specs []PackageSpec
	for ppath, dir := range packages {
		specs = append(specs, PackageSpec{Path: ppath, Dir: dir})
	}
	return specs, nil
}

func NewPathCache(env vos.Env) *PathCache {
	return &PathCache{
		env:   env,
		dirs:  make(map[string]string),
		paths: make(map[string]string),
	}
}

type PathCache struct {
	env   vos.Env
	dirs  map[string]string
	paths map[string]string
}

// FilenameToGoName converts a full filepath to a package path and filename:
// /Users/dave/go/src/github.com/dave/foo.go -> github.com/dave/foo.go
func (f *PathCache) FilePathToGoName(fpath string) (string, error) {
	fdir, fname := filepath.Split(fpath)
	ppath, err := f.GetPackageFromDir(fdir)
	if err != nil {
		return "", err
	}
	return path.Join(ppath, fname), nil
}

// GetPackageFromDir does the same as patsy.GetPackageFromDir but cached.
func (f *PathCache) GetPackageFromDir(dir string) (string, error) {
	// check the cache first
	if ppath, ok := f.paths[dir]; ok {
		return ppath, nil
	}
	ppath, err := patsy.GetPackageFromDir(f.env, dir)
	if err != nil {
		return "", err
	}
	f.paths[dir] = ppath
	f.dirs[ppath] = dir
	return ppath, nil
}

// GoNameToFilePath converts a package path and filename to a full filepath:
// github.com/dave/foo.go -> /Users/dave/go/src/github.com/dave/foo.go
func (f *PathCache) GoNameToFilePath(gpath string) (string, error) {
	ppath, fname := path.Split(gpath)
	fdir, err := f.GetDirFromPackage(ppath)
	if err != nil {
		return "", err
	}
	return filepath.Join(fdir, fname), nil
}

// GetDirFromPackage does the same as patsy.GetDirFromPackage but cached.
func (f *PathCache) GetDirFromPackage(ppath string) (string, error) {
	// check the cache first
	if dir, ok := f.dirs[ppath]; ok {
		return dir, nil
	}
	dir, err := patsy.GetDirFromPackage(f.env, ppath)
	if err != nil {
		return "", err
	}
	f.paths[dir] = ppath
	f.dirs[ppath] = dir
	return dir, nil
}

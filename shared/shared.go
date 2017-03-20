package shared

import (
	"os"
	"strings"

	"path/filepath"

	"github.com/dave/patsy"
	"github.com/dave/patsy/vos"
)

type PackageSpec struct {
	Dir  string
	Path string
}

func ParseArgs(env vos.Env, paths *patsy.Cache, args ...string) ([]PackageSpec, error) {
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
			ppath, err = paths.Path(dir)
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			dir, err = paths.Dir(ppath)
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
				ppath, err := paths.Path(dir)
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

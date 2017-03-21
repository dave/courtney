package shared

import (
	"os"
	"strings"

	"path/filepath"

	"github.com/dave/patsy"
	"github.com/dave/patsy/vos"
)

type Setup struct {
	Env      vos.Env
	Paths    *patsy.Cache
	Enforce  bool
	Verbose  bool
	Output   string
	TestArgs []string
	Packages []PackageSpec
}

type PackageSpec struct {
	Dir  string
	Path string
}

func (s *Setup) Parse(args []string) error {
	if len(args) == 0 {
		args = []string{"./..."}
	}
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
			dir, err = s.Env.Getwd()
			if err != nil {
				return err
			}
			ppath, err = s.Paths.Path(dir)
			if err != nil {
				return err
			}
		} else {
			var err error
			dir, err = s.Paths.Dir(ppath)
			if err != nil {
				return err
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
				ppath, err := s.Paths.Path(dir)
				if err != nil {
					return err
				}
				packages[ppath] = dir
			}
		}
	}
	for ppath, dir := range packages {
		s.Packages = append(s.Packages, PackageSpec{Path: ppath, Dir: dir})
	}
	return nil
}

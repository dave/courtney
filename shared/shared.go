package shared

import (
	"strings"

	"github.com/dave/patsy"
	"github.com/dave/patsy/vos"
)

// Setup holds globals, environment and command line flags for the courtney
// command
type Setup struct {
	Env                  vos.Env
	Paths                *patsy.Cache
	Enforce              bool
	Verbose              bool
	Short                bool
	Timeout              string
	Load                 string
	Output               string
	TestArgs             []string
	ExcludeGeneratedCode bool
	Packages             []PackageSpec
}

// PackageSpec identifies a package by dir and path
type PackageSpec struct {
	Dir  string
	Path string
}

// Parse parses a slice of strings into the Packages slice
func (s *Setup) Parse(args []string) error {
	if len(args) == 0 {
		args = []string{"./..."}
	}
	packages := map[string]string{}
	for _, ppath := range args {
		ppath = strings.TrimSuffix(ppath, "/")

		paths, err := s.Paths.Dirs(ppath)
		if err != nil {
			return err
		}

		for importPath, dir := range paths {
			packages[importPath] = dir
		}
	}
	for ppath, dir := range packages {
		s.Packages = append(s.Packages, PackageSpec{Path: ppath, Dir: dir})
	}
	return nil
}

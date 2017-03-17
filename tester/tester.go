package tester

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crypto/md5"

	"github.com/dave/patsy"
	"github.com/dave/patsy/vos"
	"github.com/pkg/errors"
	"golang.org/x/tools/cover"
	"github.com/dave/courtney/tester/merge"
)

func New(env vos.Env) *Tester {
	return &Tester{
		env: env,
	}
}

type Tester struct {
	env    vos.Env
	cover  string
	Result []*cover.Profile
}

type spec struct {
	dir       string
	pkg       string
	recursive bool
}

func (t *Tester) Test(packages ...string) error {
	var dirs []spec
	for _, p := range packages {
		s := spec{
			pkg: p,
		}
		if strings.HasSuffix(s.pkg, "/...") {
			s.pkg = strings.TrimSuffix(s.pkg, "/...")
			s.recursive = true
		}
		if strings.HasSuffix(s.pkg, "/") {
			s.pkg = strings.TrimSuffix(s.pkg, "/")
		}
		dir, err := patsy.GetDirFromPackage(t.env, s.pkg)
		if err != nil {
			return err
		}
		s.dir = dir
		dirs = append(dirs, s)
	}

	t.cover = filepath.Join(dirs[0].dir, ".coverage")

	os.RemoveAll(t.cover)
	defer os.RemoveAll(t.cover)

	if _, err := os.Stat(t.cover); os.IsNotExist(err) {
		if err := os.Mkdir(t.cover, 0777); err != nil {
			return errors.Wrapf(err, "Error creating temporary coverage dir %s", t.cover)
		}
	}

	processed := map[string]bool{}

	// walker function for processing any recursive packages
	walker := func(fpath string, file os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, "Error passed in while walking files")
		}
		if strings.HasPrefix(file.Name(), ".") {
			if file.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if file.IsDir() {
			if _, ok := processed[fpath]; ok {
				// if we've already processed this dir, skip it
				return nil
			}
			if err := t.processDir(fpath, dirs); err != nil {
				return err
			}
			processed[fpath] = true
		}
		return nil
	}

	for _, spec := range dirs {
		if spec.recursive {
			if err := filepath.Walk(spec.dir, walker); err != nil {
				return errors.Wrap(err, "Error while walking files")
			}
		} else {
			if _, ok := processed[spec.dir]; ok {
				// if we've already processed this dir, skip it
				continue
			}
			if err := t.processDir(spec.dir, dirs); err != nil {
				return err
			}
			processed[spec.dir] = true
		}
	}

	return nil
}

func (t *Tester) Print() {
	for _, p := range t.Result {
		fmt.Println(p.FileName)
		for _, b := range p.Blocks {
			fmt.Printf("%#v\n", b)
		}
	}
}

func (t *Tester) processDir(dir string, all []spec) error {

	coverageFilename := fmt.Sprintf("%x", md5.Sum([]byte(dir))) + ".out"
	coverageFilepath := filepath.Join(t.cover, coverageFilename)

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "Error reading files from %s", dir)
	}

	foundTest := false
	for _, f := range files {
		if strings.HasSuffix(f.Name(), "_test.go") {
			foundTest = true
		}
	}
	if !foundTest {
		return nil
	}

	// for testing?
	//if _, err := os.Stat(coverageFilepath); err == nil {
	//	return processCoverageFile(coverageFilepath)
	//}

	os.Chdir(dir)

	var allpkgs []string
	for _, s := range all {
		p := s.pkg
		if s.recursive {
			p = fmt.Sprintf("%s/...", s.pkg)
		}
		allpkgs = append(allpkgs, p)
	}
	coverParam := fmt.Sprintf("-coverpkg=%s", strings.Join(allpkgs, ","))
	outParam := fmt.Sprintf("-coverprofile=%s", coverageFilepath)
	exe := exec.Command("go", "test", outParam, coverParam)
	exe.Env = t.env.Environ()
	b, err := exe.CombinedOutput()
	if strings.Contains(string(b), "no buildable Go source files in") {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "Error executing test \nOutput:[\n%s]\n", b)
	}
	return t.processCoverageFile(coverageFilepath)
}

func (t *Tester) processCoverageFile(filename string) error {
	profiles, err := cover.ParseProfiles(filename)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if t.Result, err = merge.AddProfile(t.Result, p); err != nil {
			return err
		}
	}
	return nil
}

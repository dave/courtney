package tester_test

import (
	"testing"

	"fmt"

	"io/ioutil"
	"path"
	"path/filepath"

	"strings"

	"regexp"

	"github.com/dave/courtney/shared"
	"github.com/dave/courtney/tester"
	"github.com/dave/patsy"
	"github.com/dave/patsy/builder"
	"github.com/dave/patsy/vos"
)

var annotatedLine = regexp.MustCompile(`// \d+$`)

func TestNew(t *testing.T) {

	type args []string
	type files map[string]string
	type packages map[string]files
	type test struct {
		args     args
		packages packages
	}

	tests := map[string]test{
		"simple": {
			args: args{"ns/..."},
			packages: packages{
				"a": files{
					"a.go": `package a
						func Foo(i int) int {
							i++ // 0
							return i
						}
					`,
					"a_test.go": `package a`,
				},
			},
		},
		"simple test": {
			args: args{"ns/..."},
			packages: packages{
				"a": files{
					"a.go": `package a
					
						func Foo(i int) int {
							i++ // 1
							return i
						}
						
						func Bar(i int) int {
							i++ // 0
							return i
						}
					`,
					"a_test.go": `package a
					
					import "testing"
					
					func TestFoo(t *testing.T) {
						i := Foo(1)
						if i != 2 {
							t.Fail()
						}
					}
					`,
				},
			},
		},
		"cross package test": {
			args: args{"ns/a", "ns/b"},
			packages: packages{
				"a": files{
					"a.go": `package a
					
						func Foo(i int) int {
							i++ // 1
							return i
						}
						
						func Bar(i int) int {
							i++ // 1
							return i
						}
					`,
					"a_test.go": `package a
					
					import "testing"
					
					func TestFoo(t *testing.T) {
						i := Foo(1)
						if i != 2 {
							t.Fail()
						}
					}
					`,
				},
				"b": files{
					"b_exclude.go": `package b`,
					"b_test.go": `package b
						
						import (
							"testing"
							"ns/a"
						)
						
						func TestBar(t *testing.T) {
							i := a.Bar(1)
							if i != 2 {
								t.Fail()
							}
						}
					`,
				},
			},
		},
	}

	for name, test := range tests {

		func() { // run in a closure to ensure deferred cleanup after every test.

			env := vos.Mock()
			b, err := builder.New(env, "ns")
			if err != nil {
				t.Fatalf("Error creating builder in %s: %s", name, err)
			}
			defer b.Cleanup()

			for pname, files := range test.packages {
				if _, _, err := b.Package(pname, files); err != nil {
					t.Fatalf("Error creating package %s in %s: %s", pname, name, err)
				}
			}

			paths := patsy.NewCache(env)

			setup := &shared.Setup{
				Env:   env,
				Paths: paths,
			}
			if err := setup.Parse(test.args); err != nil {
				t.Fatalf("Error in '%s' parsing args: %s", name, err)
			}

			ts := tester.New(setup)

			if err := ts.Test(); err != nil {
				t.Fatalf("Error in '%s' while running test: %s", name, err)
			}

			filesInOutput := map[string]bool{}
			for _, p := range ts.Results {
				filesInOutput[p.FileName] = true
				pkg, fname := path.Split(p.FileName)
				dir, err := patsy.Dir(env, pkg)
				if err != nil {
					t.Fatalf("Error in '%s' while getting dir from package: %s", name, err)
				}
				src, err := ioutil.ReadFile(filepath.Join(dir, fname))
				lines := strings.Split(string(src), "\n")
				matched := map[int]bool{}
				for _, b := range p.Blocks {
					if !strings.HasSuffix(lines[b.StartLine], fmt.Sprintf("// %d", b.Count)) {
						t.Fatalf("Error in '%s' - incorrect count %d at %s line %d", name, b.Count, p.FileName, b.StartLine)
					}
					matched[b.StartLine] = true
				}
				for i, line := range lines {
					if annotatedLine.MatchString(line) {
						if _, ok := matched[i]; !ok {
							t.Fatalf("Error in '%s' - annotated line doesn't match a coverage block as %s line %d", name, p.FileName, i)
						}
					}
				}
			}
			for pname, files := range test.packages {
				for fname := range files {
					if strings.HasSuffix(fname, "_test.go") {
						continue
					}
					if strings.HasSuffix(fname, "_exclude.go") {
						// so we can have simple source files with no logic
						// blocks
						continue
					}
					fullFilename := path.Join("ns", pname, fname)
					if _, ok := filesInOutput[fullFilename]; !ok {
						t.Fatalf("Error in '%s' - %s does not appear in coverge output", name, fullFilename)
					}
				}
			}
		}()
	}
}

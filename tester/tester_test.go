package tester_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/dave/courtney/shared"
	"github.com/dave/courtney/tester"
	"github.com/dave/courtney/patsy"
	"github.com/dave/courtney/patsy/builder"
	"github.com/dave/courtney/patsy/vos"
	"golang.org/x/tools/cover"
)

func TestTester_ProcessExcludes(t *testing.T) {
	for _, gomod := range []bool{true, false} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			env := vos.Mock()
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatalf("Error creating builder in %s", err)
			}
			defer b.Cleanup()

			_, pdir, err := b.Package("a", map[string]string{
				"a.go": `package a`,
			})
			if err != nil {
				t.Fatalf("Error creating temp package: %s", err)
			}

			setup := &shared.Setup{
				Env:   env,
				Paths: patsy.NewCache(env),
			}
			ts := tester.New(setup)
			ts.Results = []*cover.Profile{
				{
					FileName: "ns/a/a.go",
					Blocks: []cover.ProfileBlock{
						{Count: 1, StartLine: 1, EndLine: 10},
						{Count: 0, StartLine: 11, EndLine: 20},
						{Count: 1, StartLine: 21, EndLine: 30},
						{Count: 0, StartLine: 31, EndLine: 40},
					},
				},
			}
			excludes := map[string]map[int]bool{
				filepath.Join(pdir, "a.go"): {
					25: true,
					35: true,
				},
			}
			expected := []cover.ProfileBlock{
				{Count: 1, StartLine: 1, EndLine: 10},
				{Count: 0, StartLine: 11, EndLine: 20},
				{Count: 1, StartLine: 21, EndLine: 30},
			}
			if err := ts.ProcessExcludes(excludes); err != nil {
				t.Fatalf("Processing excludes: %s", err)
			}
			if !reflect.DeepEqual(ts.Results[0].Blocks, expected) {
				t.Fatalf("Processing excludes - got:\n%#v\nexpected:\n%#v\n", ts.Results[0].Blocks, expected)
			}
		})
	}
}

func TestTester_Enforce(t *testing.T) {
	for _, gomod := range []bool{true, false} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			env := vos.Mock()
			setup := &shared.Setup{
				Env:     env,
				Paths:   patsy.NewCache(env),
				Enforce: true,
			}
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatalf("Error creating builder: %s", err)
			}
			defer b.Cleanup()

			_, _, _ = b.Package("a", map[string]string{
				"a.go": "package a\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20",
			})

			ts := tester.New(setup)
			ts.Results = []*cover.Profile{
				{
					FileName: "ns/a/a.go",
					Mode:     "b",
					Blocks: []cover.ProfileBlock{
						{Count: 1},
					},
				},
			}
			if err := ts.Enforce(); err != nil {
				t.Fatalf("Error enforcing: %s", err)
			}

			ts.Results[0].Blocks = []cover.ProfileBlock{
				{Count: 1, StartLine: 1, EndLine: 2},
				{Count: 0, StartLine: 6, EndLine: 11},
			}
			err = ts.Enforce()
			if err == nil {
				t.Fatal("Error enforcing - should get error, got nil")
			}
			expected := "Error - untested code:\nns/a/a.go:6-11:\n\t5\n\t6\n\t7\n\t8\n\t9\n\t10"
			if err.Error() != expected {
				t.Fatalf("Error enforcing - got \n%s\nexpected:\n%s\n", strconv.Quote(err.Error()), strconv.Quote(expected))
			}

			// check that blocks next to each other are merged
			ts.Results[0].Blocks = []cover.ProfileBlock{
				{Count: 1, StartLine: 1, EndLine: 2},
				{Count: 0, StartLine: 6, EndLine: 11},
				{Count: 0, StartLine: 12, EndLine: 16},
				{Count: 0, StartLine: 18, EndLine: 21},
			}
			err = ts.Enforce()
			if err == nil {
				t.Fatal("Error enforcing - should get error, got nil")
			}
			expected = "Error - untested code:\nns/a/a.go:6-16:\n\t5\n\t6\n\t7\n\t8\n\t9\n\t10\n\t11\n\t12\n\t13\n\t14\n\t15ns/a/a.go:18-21:\n\t17\n\t18\n\t19\n\t20"
			if err.Error() != expected {
				t.Fatalf("Error enforcing - got \n%s\nexpected:\n%s\n", strconv.Quote(err.Error()), strconv.Quote(expected))
			}
		})
	}
}

func TestTester_Save_output(t *testing.T) {
	env := vos.Mock()
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %s", err)
	}
	out := filepath.Join(dir, "foo.bar")
	setup := &shared.Setup{
		Env:    env,
		Paths:  patsy.NewCache(env),
		Output: out,
	}
	ts := tester.New(setup)
	ts.Results = []*cover.Profile{
		{
			FileName: "a",
			Mode:     "b",
			Blocks:   []cover.ProfileBlock{{}},
		},
	}
	if err := ts.Save(); err != nil {
		t.Fatalf("Error saving: %s", err)
	}
	if _, err := ioutil.ReadFile(out); err != nil {
		t.Fatalf("Error loading coverage: %s", err)
	}
}

func TestTester_Save_no_results(t *testing.T) {
	env := vos.Mock()
	sout := &bytes.Buffer{}
	serr := &bytes.Buffer{}
	env.Setstdout(sout)
	env.Setstderr(serr)
	setup := &shared.Setup{
		Env:   env,
		Paths: patsy.NewCache(env),
	}
	ts := tester.New(setup)
	if err := ts.Save(); err != nil {
		t.Fatalf("Error saving: %s", err)
	}
	expected := "No results\n"
	if sout.String() != expected {
		t.Fatalf("Error saving, stdout: got:\n%s\nexpected:\n%s\n", sout.String(), expected)
	}
}

func TestTester_Test(t *testing.T) {

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
		for _, gomod := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s,gomod=%v", name, gomod), func(t *testing.T) {
				env := vos.Mock()
				b, err := builder.New(env, "ns", gomod)
				if err != nil {
					t.Fatalf("Error creating builder in %s: %+v", name, err)
				}
				defer b.Cleanup()

				for pname, files := range test.packages {
					if _, _, err := b.Package(pname, files); err != nil {
						t.Fatalf("Error creating package %s in %s: %+v", pname, name, err)
					}
				}

				paths := patsy.NewCache(env)

				setup := &shared.Setup{
					Env:   env,
					Paths: paths,
				}
				if err := setup.Parse(test.args); err != nil {
					t.Fatalf("Error in '%s' parsing args: %+v", name, err)
				}

				ts := tester.New(setup)

				if err := ts.Test(); err != nil {
					t.Fatalf("Error in '%s' while running test: %+v", name, err)
				}

				fmt.Printf("Results: %#v\n", ts.Results)

				filesInOutput := map[string]bool{}
				for _, p := range ts.Results {

					filesInOutput[p.FileName] = true
					pkg, fname := path.Split(p.FileName)
					pkg = strings.TrimSuffix(pkg, "/")
					dir, err := patsy.Dir(env, pkg)
					if err != nil {
						t.Fatalf("Error in '%s' while getting dir from package: %+v", name, err)
					}
					src, err := ioutil.ReadFile(filepath.Join(dir, fname))
					if err != nil {
						t.Fatalf("Error in '%s' while opening coverage: %+v", name, err)
					}
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
				fmt.Printf("%#v\n", filesInOutput)
				for pname, files := range test.packages {
					for fname := range files {
						if strings.HasSuffix(fname, ".mod") {
							continue
						}
						if strings.HasSuffix(fname, "_test.go") {
							continue
						}
						if strings.HasSuffix(fname, "_exclude.go") {
							// so we can have simple source files with no logic
							// blocks
							continue
						}
						fullFilename := path.Join("ns", pname, fname)
						fmt.Println(fullFilename)
						if _, ok := filesInOutput[fullFilename]; !ok {
							t.Fatalf("Error in '%s' - %s does not appear in coverge output", name, fullFilename)
						}
					}
				}
			})
		}
	}
}

var annotatedLine = regexp.MustCompile(`// \d+$`)

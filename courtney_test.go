package main

import (
	"fmt"
	"testing"

	"io/ioutil"
	"path/filepath"

	"bytes"
	"strings"

	"os"

	"github.com/dave/courtney/shared"
	"github.com/dave/patsy"
	"github.com/dave/patsy/builder"
	"github.com/dave/patsy/vos"
)

func TestRun(t *testing.T) {
	for _, gomod := range []bool{true, false} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			name := "run"
			env := vos.Mock()
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatalf("Error creating builder in %s: %s", name, err)
			}
			defer b.Cleanup()

			_, pdir, err := b.Package("a", map[string]string{
				"a.go": `package a
		
			func Foo(i int) int {
				i++
				return i
			}
			
			func Bar(i int) int {
				i++
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
			})
			if err != nil {
				t.Fatalf("Error creating builder in %s: %s", name, err)
			}

			if err := env.Setwd(pdir); err != nil {
				t.Fatalf("Error in Setwd in %s: %s", name, err)
			}

			sout := &bytes.Buffer{}
			serr := &bytes.Buffer{}
			env.Setstdout(sout)
			env.Setstderr(serr)

			setup := &shared.Setup{
				Env:     env,
				Paths:   patsy.NewCache(env),
				Enforce: true,
				Verbose: true,
			}
			err = Run(setup)
			if err == nil {
				t.Fatalf("Error in %s. Run should error.", name)
			}
			expected := `Error - untested code:
ns/a/a.go:8-11:
	func Bar(i int) int {
		i++
		return i
	}`
			if !strings.Contains(err.Error(), expected) {
				t.Fatalf("Error in %s err. Got: \n%s\nExpected to contain: \n%s\n", name, err.Error(), expected)
			}

			coverage, err := ioutil.ReadFile(filepath.Join(pdir, "coverage.out"))
			if err != nil {
				t.Fatalf("Error reading coverage file in %s: %s", name, err)
			}
			expected = `mode: set
ns/a/a.go:3.24,6.5 2 1
ns/a/a.go:8.24,11.5 2 0
`
			if string(coverage) != expected {
				t.Fatalf("Error in %s coverage. Got: \n%s\nExpected: \n%s\n", name, string(coverage), expected)
			}

			setup = &shared.Setup{
				Env:   env,
				Paths: patsy.NewCache(env),
			}
			if err := Run(setup); err != nil {
				t.Fatalf("Error running program (second try) in %s: %s", name, err)
			}
		})
	}
}

func TestRun_load(t *testing.T) {
	for _, gomod := range []bool{true, false} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			name := "load"
			env := vos.Mock()
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatalf("Error creating builder in %s: %s", name, err)
			}
			defer b.Cleanup()

			_, pdir, err := b.Package("a", map[string]string{
				"a.go": `package a
		
			func Foo(i int) int {
				i++
				return i
			}
			
			func Bar(i int) int {
				i++
				return i
			}
		`,
				"a_test.go": `package a
					
			import "testing"
			
			func TestFoo(t *testing.T) {
				// In "load" mode, this test will not run.
				t.Fail()
			}
		`,
				"a.out": `mode: set
ns/a/a.go:3.24,6.5 2 1
ns/a/a.go:8.24,11.5 2 0
`,
				"b.out": `mode: set
ns/a/a.go:3.24,6.5 2 0
ns/a/a.go:8.24,11.5 2 1
`,
			})
			if err != nil {
				t.Fatalf("Error creating builder in %s: %s", name, err)
			}

			if err := env.Setwd(pdir); err != nil {
				t.Fatalf("Error in Setwd in %s: %s", name, err)
			}

			// annoyingly, filepath.Glob in "Load" method does not respect the mocked
			// vos working directory
			if err := os.Chdir(pdir); err != nil {
				t.Fatalf("Error in os.Chdir in %s: %s", name, err)
			}

			sout := &bytes.Buffer{}
			serr := &bytes.Buffer{}
			env.Setstdout(sout)
			env.Setstderr(serr)

			setup := &shared.Setup{
				Env:   env,
				Paths: patsy.NewCache(env),
				Load:  "*.out",
			}
			if err := Run(setup); err != nil {
				t.Fatalf("Error running program in %s: %s", name, err)
			}

			coverage, err := ioutil.ReadFile(filepath.Join(pdir, "coverage.out"))
			if err != nil {
				t.Fatalf("Error reading coverage file in %s: %s", name, err)
			}
			expected := `mode: set
ns/a/a.go:3.24,6.5 2 1
ns/a/a.go:8.24,11.5 2 1
`
			if string(coverage) != expected {
				t.Fatalf("Error in %s coverage. Got: \n%s\nExpected: \n%s\n", name, string(coverage), expected)
			}
		})
	}
}

package scanner_test

import (
	"strconv"
	"strings"
	"testing"

	"path/filepath"

	"github.com/dave/courtney/scanner"
	"github.com/dave/courtney/shared"
	"github.com/dave/courtney/patsy"
	"github.com/dave/courtney/patsy/builder"
	"github.com/dave/courtney/patsy/vos"
)

func TestSingle(t *testing.T) {
	tests := map[string]string{
		"single": `package a

			func wrap(error) error

			func a() error {
				var a bool
				var err error
				if err != nil {
					if a { // this line will not be excluded!
						return wrap(err) // *
					}
					return wrap(err) // *
				}
				return nil
			}
		`,
	}
	test(t, tests)
}

func TestSwitchCase(t *testing.T) {
	tests := map[string]string{
		"simple switch": `package a

			func a() error {
				var err error
				switch {
				case err != nil:
					return err // *
				}
				return nil
			}
		`,
		"switch multi": `package a

			func a() error {
				var a bool
				var err error
				switch {
				case err == nil, a:
					return err
				default:
					return err // *
				}
				return nil
			}
		`,
		"simple switch ignored": `package a

			func a() error {
				var a bool
				var err error
				switch a {
				case err != nil:
					return err
				}
				return nil
			}
		`,
		"complex switch": `package a

			func foo() error {
				var err error
				var b, c bool
				var d int
				switch {
				case err == nil && (b && d > 0) || c:
					return err
				case d <= 0 || c:
					return err
				case b:
					return err // *
				}
				return err
			}
		`,
	}
	test(t, tests)
}

func TestNamedParameters(t *testing.T) {
	tests := map[string]string{
		"named parameters simple": `package a

			func a() (err error) {
				if err != nil {
					return // *
				}
				return
			}
		`,
		"named parameters ignored": `package a

			func a() {
				var err error
				if err != nil {
					return
				}
				return
			}
		`,
		"named parameters 2": `package a

			func a() (i int, err error) {
				i = 1
				if err != nil {
					return // *
				}
				return
			}
		`,
		"named parameters must be last": `package a

			func a() (err error, i int) {
				i = 1
				if err != nil {
					return
				}
				return
			}
		`,
		"named parameters must be not nil": `package a

			func a() (err error) {
				return
			}
		`,
		"named parameters func lit": `package a

			func a() {
				func () (err error) {
					if err != nil {
						return // *
					}
					return
				}()
			}
		`,
	}
	test(t, tests)
}

func TestBool(t *testing.T) {
	tests := map[string]string{
		"wrap1": `package a

			func a() error {
				var wrap func(error) error
				var err error
				if err != nil {
					return wrap(err) // *
				}
				return nil
			}
			`,
		"wrap ignored": `package a

			func a() int {
				var wrap func(error) int
				var err error
				if err != nil {
					return wrap(err)
				}
				return 0
			}
			`,
		"wrap2": `package a

			func a() error {
				var wrap func(error) error
				var err error
				if err != nil {
					w := wrap(err)
					return w // *
				}
				return nil
			}
			`,
		"wrap3": `package a

			func a() error {
				var wrap func(error) error
				var err error
				var w error
				if err != nil {
					w = wrap(err)
					return w // *
				}
				return nil
			}
			`,
		"wrap4": `package a

			func a() error {
				var wrap func(error) error
				var err error
				if err != nil {
					var w = wrap(err)
					return w // *
				}
				return nil
			}
			`,
		"wrap5": `package a

			func a() error {
				var wrap func(error) error
				var err error
				if err != nil {
					var w error = wrap(err)
					return w // *
				}
				return nil
			}
			`,
		"wrap no tuple": `package a

			func a() (int, error) {
				var wrap func(error) (int, error)
				var err error
				if err != nil {
					return wrap(err)
				}
				return 0, nil
			}
		`,
		"logical and first": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if err != nil && 1 == 1 {
					return err // *
				}
				return nil
			}
			`,
		"logical and second": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if 1 == 1 && err != nil {
					return err // *
				}
				return nil
			}
			`,
		"logical and third": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if 1 == 1 && 2 == 2 && err != nil {
					return err // *
				}
				return nil
			}
			`,
		"logical and brackets": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if 1 == 1 && (2 == 2 && err != nil) {
					return err // *
				}
				return nil
			}
			`,
		"logical or first": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if err == nil || 1 == 1 {
					return err
				} else {
					return err // *
				}
				return nil
			}
			`,
		"logical or second": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if 1 == 1 || err == nil {
					return err
				} else {
					return err // *
				}
				return nil
			}
			`,
		"logical or third": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if 1 == 1 || 2 == 2 || err == nil {
					return err
				} else {
					return err // *
				}
				return nil
			}
			`,
		"logical or brackets": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if 1 == 1 || (2 == 2 || err == nil) {
					return err
				} else {
					return err // *
				}
				return nil
			}
			`,
		"complex": `package a

			func foo() error {
				var err error
				var b, c bool
				var d int
				if err == nil && (b && d > 0) || c {
					return err
				} else if d <= 0 || c {
					return err
				} else if b {
					return err // *
				}
				return err
			}
		`,
	}
	test(t, tests)
}

func TestGeneral(t *testing.T) {
	tests := map[string]string{
		"simple": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if err != nil {
					return err // *
				}
				return nil
			}
			`,
		"wrong way round": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if nil != err {
					return err // *
				}
				return nil
			}
			`,
		"not else block": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if err != nil {
					return err // *
				} else {
					return err
				}
				return nil
			}
			`,
		"any name": `package a

			import "fmt"

			func a() error {
				_, foo := fmt.Println()
				if foo != nil {
					return foo // *
				}
				return nil
			}
			`,
		"don't mark if ==": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if err == nil {
					return err
				}
				return nil
			}
			`,
		"use else block if err == nil": `package a

			import "fmt"

			func a() error {
				_, err := fmt.Println()
				if err == nil {
					return err
				} else {
					return err // *
				}
				return nil
			}
			`,
		"support if with init form": `package a

			import "fmt"

			func a() error {
				if _, err := fmt.Println(); err != nil {
					return err // *
				}
				return nil
			}
			`,
		"only in if block": `package foo

			import "fmt"

			func Baz() error {
				return fmt.Errorf("foo")
			}
			`,
	}
	test(t, tests)
}

func TestZeroValues(t *testing.T) {
	tests := map[string]string{
		"only return if all other return vars are zero": `package a

			import "fmt"

			type iface interface{}

			type strct struct {
				a int
				b string
			}

			func Foo() (iface, bool, int, string, float32, strct, strct, error) {
				if _, err := fmt.Println(); err != nil {
					return 1, false, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, true, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 1, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 0, "a", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 0, "", 1.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 0, "", 0.0, strct{1, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 0, "", 0.0, strct{0, "a"}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 0, "", 0.0, strct{0, ""}, strct{a: 1, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: "a"}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, false, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err // *
				}
				return nil, false, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, nil
			}
			`,
	}
	test(t, tests)
}

func TestSelectorExpressions(t *testing.T) {
	tests := map[string]string{
		"selector expression": `package foo

			func Baz() error {
				type T struct {
					Err error
				}
				var b T
				if b.Err != nil {
					return b.Err // *
				}
				return nil
			}
			`,
	}
	test(t, tests)
}

func TestFunctionExpressions(t *testing.T) {
	tests := map[string]string{
		"function expression": `package foo

			func Baz() error {
				var f func(int) error
				if f(5) != nil {
					return f(5) // *
				}
				return nil
			}
			`,
		"function expression params": `package foo

			func Baz() error {
				var f func(int) error
				if f(4) != nil {
					return f(5)
				}
				return nil
			}
			`,
		"function expression params 2": `package foo

			func Baz() error {
				var f func(...int) error
				if f(4) != nil {
					return f(4, 4)
				}
				return nil
			}
			`,
		"function expression elipsis": `package foo

			func Baz() error {
				var f func(...interface{}) error
				var a []interface{}
				if f(a) != nil {
					return f(a...)
				}
				return nil
			}
			`,
		"function expression elipsis 2": `package foo

			func Baz() error {
				var f func(...interface{}) error
				var a []interface{}
				if f(a) != nil {
					return f(a) // *
				}
				return nil
			}
			`,
	}
	test(t, tests)
}

func TestPanic(t *testing.T) {
	tests := map[string]string{
		"panic": `package foo

			func Baz() error {
				panic("") // *
			}
			`,
	}
	test(t, tests)
}

func TestComments(t *testing.T) {
	tests := map[string]string{
		"scope": `package foo

			func Baz() int {
				i := 1
				if i > 1 {
					return i
				}

				//notest
				             // *
				if i > 2 {   // *
					return i // *
				}            // *
				return 0     // *
			}
			`,
		"scope if": `package foo

			func Baz(i int) int {
				if i > 2 {
					//notest
					return i // *
				}
				return 0
			}
			`,
		"scope file": `package foo

			//notest
			                      // *
			func Baz(i int) int { // *
				if i > 2 {        // *
					return i      // *
				}                 // *
				return 0          // *
			}                     // *
			                      // *
			func Foo(i int) int { // *
				return 0          // *
			}
			`,
		"complex comments": `package foo

			type Logger struct {
				Enabled bool
			}
			func (l Logger) Print(i ...interface{}) {}

			func Foo() {
				var logger Logger
				var tokens []interface{}
				if logger.Enabled {
					// notest
					for i, token := range tokens {        // *
						logger.Print("[", i, "] ", token) // *
					}                                     // *
				}
			}
			`,
		"case block": `package foo

			func Foo() bool {
				switch {
				case true:
					// notest
					if true {       // *
						return true // *
					}               // *
					return false    // *
				}
				return false
			}
			`,
	}
	test(t, tests)
}

func test(t *testing.T, tests map[string]string) {
	for name, source := range tests {
		env := vos.Mock()
		b, err := builder.New(env, "ns", true)
		if err != nil {
			t.Fatalf("Error creating builder in %s: %+v", name, err)
		}
		defer b.Cleanup()

		ppath, pdir, err := b.Package("a", map[string]string{
			"a.go": source,
		})
		if err != nil {
			t.Fatalf("Error creating package in %s: %+v", name, err)
		}

		paths := patsy.NewCache(env)
		setup := &shared.Setup{
			Env:   env,
			Paths: paths,
		}
		if err := setup.Parse([]string{ppath}); err != nil {
			t.Fatalf("Error parsing args in %s: %+v", name, err)
		}

		cm := scanner.New(setup)

		if err := cm.LoadProgram(); err != nil {
			t.Fatalf("Error loading program in %s: %+v", name, err)
		}

		if err := cm.ScanPackages(); err != nil {
			t.Fatalf("Error scanning packages in %s: %+v", name, err)
		}

		result := cm.Excludes[filepath.Join(pdir, "a.go")]

		for i, line := range strings.Split(source, "\n") {
			expected := strings.HasSuffix(line, "// *") ||
				strings.HasSuffix(line, "//notest") ||
				strings.HasSuffix(line, "// notest")
			if result[i+1] != expected {
				t.Fatalf("Unexpected state in %s, line %d: %s\n", name, i, strconv.Quote(strings.Trim(line, "\t")))
			}
		}
	}
}

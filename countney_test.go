package courtney_test

import (
	"testing"

	"strings"

	"fmt"

	"strconv"

	"github.com/dave/courtney"
	"github.com/stretchr/testify/require"
)

func TestSingle(t *testing.T) {
	test(t, "single", `package a
			
			func a() error {
				var wrap func(error) error
				var err error
				if err != nil {
					return wrap(err) // *
				}
				return nil
			}
			`)
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
	}
	for name, in := range tests {
		test(t, name, in)
	}
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
		"only return if all other return vars are zero": `package a
			
			import "fmt"
			
			type iface interface{}
			
			type strct struct {
				a int
				b string
			}
			
			func Foo() (iface, int, string, float32, strct, strct, error) {
				if _, err := fmt.Println(); err != nil {
					return 1, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 1, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 0, "a", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 0, "", 1.0, strct{0, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 0, "", 0.0, strct{1, ""}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 0, "", 0.0, strct{0, "a"}, strct{a: 0, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 0, "", 0.0, strct{0, ""}, strct{a: 1, b: ""}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: "a"}, err
				}
				if _, err := fmt.Println(); err != nil {
					return nil, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, err // *
				}
				return nil, 0, "", 0.0, strct{0, ""}, strct{a: 0, b: ""}, nil
			}
			`,
		"only if if block": `package foo
			
			import "fmt"
			
			func Baz() error {
				return fmt.Errorf("foo")
			}
			`,
	}
	for name, in := range tests {
		test(t, name, in)
	}
}

func test(t *testing.T, name, in string) {
	cm := courtney.NewCodeMap()

	require.NoError(t, cm.ScanFile("foo.bar/baz", "/baz", "main.go", in), "Error in %s", name)
	require.NoError(t, cm.CheckTypes(), "Error in %s", name)
	require.NoError(t, cm.FindErrorReturns(), "Error in %s", name)

	result := cm.Excludes()["/baz/main.go"]

	for i, line := range strings.Split(in, "\n") {
		expected := strings.HasSuffix(line, "// *")
		if result[i+1] != expected {
			fmt.Printf("Unexpected state in %s, line %d: %s\n", name, i, strconv.Quote(strings.Trim(line, "\t")))
			t.Fail()
		}
	}
}

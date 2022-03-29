package patsy_test

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dave/courtney/patsy"
	"github.com/dave/courtney/patsy/builder"
	"github.com/dave/courtney/patsy/vos"
)

func TestName2(t *testing.T) {
	for _, gomod := range []bool{false, true} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			env := vos.Mock()
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Cleanup()

			_, dirA, err := b.Package("a", map[string]string{
				"a.go": "package a",
			})
			if err != nil {
				t.Fatal(err)
			}

			_, dirB, err := b.Package("b", map[string]string{
				"b.go": "package b",
			})
			if err != nil {
				t.Fatal(err)
			}

			packagePathC, _, err := b.Package("c", map[string]string{
				"c.go": "package c",
			})
			if err != nil {
				t.Fatal(err)
			}

			// We add a vendored version of "c" inside "b" that has the name "v"
			_, _, err = b.Package(path.Join("b", "vendor", packagePathC), map[string]string{
				"c.go": "package v",
			})
			if err != nil {
				t.Fatal(err)
			}

			name, err := patsy.Name(env, packagePathC, dirA)
			if err != nil {
				t.Fatal(err)
			}
			expected := "c"
			if name != expected {
				t.Fatalf("Got %s, Expected %s", name, expected)
			}

			name, err = patsy.Name(env, packagePathC, dirB)
			if err != nil {
				t.Fatal(err)
			}

			// without go mod we expect the vendored version of "c"
			// but with go mod the vendoring is ignored, so we still expect "c"
			if gomod {
				expected = "c"
			} else {
				expected = "v"
			}
			if name != expected {
				t.Fatalf("Got %s, Expected %s", name, expected)
			}
		})
	}
}

func TestNameGoPathFromAnywhere(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns", false)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()

	packagePath, _, err := b.Package("a", map[string]string{
		"a.go": "package b",
	})
	if err != nil {
		t.Fatal(err)
	}

	// with absolute packagePage we can get the name from any srcDir
	name, err := patsy.Name(env, packagePath, "/")
	if err != nil {
		t.Fatal(err)
	}
	expected := "b"
	if name != expected {
		t.Fatalf("Got %s, Expected %s", name, expected)
	}
}

func TestPathGoMod(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns", true)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()

	packagePath, packageDir, err := b.Package("a", map[string]string{
		"a.go": "package b",
	})
	if err != nil {
		t.Fatal(err)
	}

	calculatedPath, err := patsy.Path(env, packageDir)
	if err != nil {
		t.Fatal(err)
	}
	if calculatedPath != packagePath {
		t.Fatalf("Got %s, Expected %s", calculatedPath, packagePath)
	}
}

func TestPathGoPath(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns", false)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()

	packagePath, packageDir, err := b.Package("a", map[string]string{
		"a.go": "package b",
	})
	if err != nil {
		t.Fatal(err)
	}

	calculatedPath, err := patsy.Path(env, packageDir)
	if err != nil {
		t.Fatal(err)
	}
	if calculatedPath != packagePath {
		t.Fatalf("Got %s, Expected %s", calculatedPath, packagePath)
	}

	_ = env.Setenv("GOPATH", "/foo/"+string(filepath.ListSeparator)+env.Getenv("GOPATH"))

	calculatedPath, err = patsy.Path(env, packageDir)
	if err != nil {
		t.Fatal(err)
	}
	if calculatedPath != packagePath {
		t.Fatalf("Got %s, Expected %s", calculatedPath, packagePath)
	}

	_ = env.Setenv("GOPATH", "/bar/")
	_, err = patsy.Path(env, packageDir)
	if err == nil {
		t.Fatal("Expected error, got none.")
	} else if !strings.HasPrefix(err.Error(), "Package not found") {
		t.Fatalf("Expected 'Package not found', got '%s'", err.Error())
	}
}

// In gopath mode we use a fallback to find the dir of an empty package.
func TestPathNoSrcGoPath(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns", false)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()

	packagePath, packageDir, err := b.Package("a", nil)
	if err != nil {
		t.Fatal(err)
	}

	calculatedPath, err := patsy.Path(env, packageDir)
	if err != nil {
		t.Fatal(err)
	}
	if calculatedPath != packagePath {
		t.Fatalf("Got %s, expected %s", calculatedPath, packagePath)
	}
}

// In gomod mode we don't support empty packages.
func TestPathNoSrcGoMod(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns", true)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()

	_, packageDir, err := b.Package("a", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = patsy.Dir(env, packageDir)
	if err == nil {
		t.Fatal("Expected error, got none.")
	} else if !strings.HasPrefix(err.Error(), "Dir not found") {
		t.Fatalf("Expected 'Dir not found', got '%s'", err.Error())
	}
}

func TestDir(t *testing.T) {
	for _, gomod := range []bool{false, true} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			env := vos.Mock()
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Cleanup()

			packagePath, packageDir, err := b.Package("a", map[string]string{
				"a.go": "package b",
			})
			if err != nil {
				t.Fatal(err)
			}

			calculatedDir, err := patsy.Dir(env, packagePath)
			if err != nil {
				t.Fatal(err)
			}
			if calculatedDir != packageDir {
				t.Fatalf("Got %s, expected %s", calculatedDir, packageDir)
			}
		})
	}
}

// In gopath mode we use a fallback to find the dir of an empty package.
func TestDirNoSrcGoPath(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns", false)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()

	packagePath, packageDir, err := b.Package("a", nil)
	if err != nil {
		t.Fatal(err)
	}

	calculatedDir, err := patsy.Dir(env, packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if calculatedDir != packageDir {
		t.Fatalf("Got %s, expected %s", calculatedDir, packageDir)
	}
}

// In gomod mode we don't support empty packages.
func TestDirNoSrcGoMod(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns", true)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()

	packagePath, _, err := b.Package("a", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = patsy.Dir(env, packagePath)
	if err == nil {
		t.Fatal("Expected error, got none.")
	} else if !strings.HasPrefix(err.Error(), "Dir not found") {
		t.Fatalf("Expected 'Dir not found', got '%s'", err.Error())
	}
}

func TestDirs(t *testing.T) {
	for _, gomod := range []bool{false, true} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			env := vos.Mock()
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Cleanup()

			packagePath, packageDir, err := b.Package("a", map[string]string{
				"a.go": "package b",
			})
			if err != nil {
				t.Fatal(err)
			}

			calculatedDirs, err := patsy.Dirs(env, packagePath)
			if err != nil {
				t.Fatal(err)
			}
			if len(calculatedDirs) != 1 || calculatedDirs[packagePath] != packageDir {
				t.Fatalf("Got %v, expected {%s: %s}", calculatedDirs, packagePath, packageDir)
			}

			// using . inside a package should give the packageDir as well
			_ = env.Setwd(packageDir)
			calculatedDirs, err = patsy.Dirs(env, ".")
			if err != nil {
				t.Fatal(err)
			}
			if len(calculatedDirs) != 1 || calculatedDirs[packagePath] != packageDir {
				t.Fatalf("Got %v, expected map[%s: %s]", calculatedDirs, packagePath, packageDir)
			}
		})
	}
}

func TestDirsSubpackages(t *testing.T) {
	for _, gomod := range []bool{false, true} {
		t.Run(fmt.Sprintf("gomod=%v", gomod), func(t *testing.T) {
			env := vos.Mock()
			b, err := builder.New(env, "ns", gomod)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Cleanup()

			packagePathA, packageDirA, err := b.Package("a", map[string]string{
				"a.go": "package a",
			})
			if err != nil {
				t.Fatal(err)
			}

			packagePathB, packageDirB, err := b.Package("a/b", map[string]string{
				"b.go": "package b",
			})
			if err != nil {
				t.Fatal(err)
			}

			packagePathC, packageDirC, err := b.Package("a/c", map[string]string{
				"c.go": "package c",
			})
			if err != nil {
				t.Fatal(err)
			}

			calculatedDirs, err := patsy.Dirs(env, "ns/a/...")
			if err != nil {
				t.Fatal(err)
			}
			if len(calculatedDirs) != 3 {
				t.Fatalf("Got %v, expected 3 packages", calculatedDirs)
			}
			if calculatedDirs[packagePathA] != packageDirA {
				t.Fatalf("Got %s, expected %s", calculatedDirs[packagePathA], packageDirA)
			}
			if calculatedDirs[packagePathB] != packageDirB {
				t.Fatalf("Got %s, expected %s", calculatedDirs[packagePathB], packageDirB)
			}
			if calculatedDirs[packagePathC] != packageDirC {
				t.Fatalf("Got %s, expected %s", calculatedDirs[packagePathC], packageDirC)
			}
		})
	}
}

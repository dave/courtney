package shared_test

import (
	"testing"

	"github.com/dave/courtney/shared"
	"github.com/dave/patsy"
	"github.com/dave/patsy/builder"
	"github.com/dave/patsy/vos"
)

func TestParseArgs(t *testing.T) {
	env := vos.Mock()
	b, err := builder.New(env, "ns")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()
	apath, adir, err := b.Package("a", map[string]string{
		"a.go": `package a`,
	})
	if err != nil {
		t.Fatal(err)
	}

	paths := patsy.NewCache(env)

	if err := env.Setwd(adir); err != nil {
		t.Fatal(err)
	}

	setup := shared.Setup{
		Env:   env,
		Paths: paths,
	}

	if err := setup.Parse([]string{"."}); err != nil {
		t.Fatal(err)
	}
	if len(setup.Packages) != 1 {
		t.Fatalf("Error in ParseArgs - wrong number of packages. Expected 1, got %d", len(setup.Packages))
	}
	expected := shared.PackageSpec{
		Dir:  adir,
		Path: apath,
	}
	if setup.Packages[0] != expected {
		t.Fatalf("Error in ParseArgs - wrong package. Expected %#v. Got %#v.", expected, setup.Packages[0])
	}

}

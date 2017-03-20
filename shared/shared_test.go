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

	pcache := patsy.NewCache(env)

	if err := env.Setwd(adir); err != nil {
		t.Fatal(err)
	}

	psa, err := shared.ParseArgs(env, pcache, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(psa) != 1 {
		t.Fatalf("Error in ParseArgs - wrong number of packages. Expected 1, got %d", len(psa))
	}
	expected := shared.PackageSpec{
		Dir:  adir,
		Path: apath,
	}
	if psa[0] != expected {
		t.Fatalf("Error in ParseArgs - wrong package. Expected %#v. Got %#v.", expected, psa[0])
	}

}

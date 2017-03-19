package main

import (
	"flag"
	"log"

	"github.com/dave/courtney/scanner"
	"github.com/dave/courtney/shared"
	"github.com/dave/courtney/tester"
	"github.com/dave/patsy/pathcache"
	"github.com/dave/patsy/vos"
)

func main() {
	//enforceFlag := flag.Bool("enforce", false, "Enforce 100% code coverage")
	flag.Parse()
	//enforce := *enforceFlag
	args := flag.Args()
	env := vos.Os()

	if len(args) == 0 {
		args = []string{"./..."}
	}

	paths := pathcache.New(env)
	packages, err := shared.ParseArgs(env, paths, args...)
	if err != nil {
		log.Fatal(err)
	}

	s := scanner.New(env, paths)
	if err := s.LoadProgram(packages); err != nil {
		log.Fatal(err)
	}
	if err := s.ScanPackages(); err != nil {
		log.Fatal(err)
	}

	t := tester.New(env, paths)
	if err := t.Test(packages); err != nil {
		log.Fatal(err)
	}
	if err := t.ProcessExcludes(s.Excludes); err != nil {
		log.Fatal(err)
	}
	if err := t.Save(); err != nil {
		log.Fatal(err)
	}

}

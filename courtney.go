package main

import (
	"flag"
	"log"

	"strings"

	"github.com/dave/courtney/scanner"
	"github.com/dave/courtney/shared"
	"github.com/dave/courtney/tester"
	"github.com/dave/patsy"
	"github.com/dave/patsy/vos"
)

func main() {

	env := vos.Os()

	enforceFlag := flag.Bool("e", false, "Enforce 100% code coverage")
	verboseFlag := flag.Bool("v", false, "Verbose output")
	outputFlag := flag.String("o", "", "Override coverage file location")
	argsFlag := new(argsValue)
	flag.Var(argsFlag, "t", "Argument to pass to the 'go test' command. Can be used more than once.")

	flag.Parse()

	setup := &shared.Setup{
		Env:      env,
		Paths:    patsy.NewCache(env),
		Enforce:  *enforceFlag,
		Verbose:  *verboseFlag,
		Output:   *outputFlag,
		TestArgs: argsFlag.args,
	}
	if err := setup.Parse(flag.Args()); err != nil {
		log.Fatal(err)
	}

	s := scanner.New(setup)
	if err := s.LoadProgram(); err != nil {
		log.Fatal(err)
	}
	if err := s.ScanPackages(); err != nil {
		log.Fatal(err)
	}

	t := tester.New(setup)
	if err := t.Test(); err != nil {
		log.Fatal(err)
	}
	if err := t.ProcessExcludes(s.Excludes); err != nil {
		log.Fatal(err)
	}
	if err := t.Save(); err != nil {
		log.Fatal(err)
	}
	if err := t.Enforce(); err != nil {
		log.Fatal(err)
	}

}

type argsValue struct {
	args []string
}

var _ flag.Value = (*argsValue)(nil)

func (v *argsValue) String() string {
	if v == nil {
		return ""
	}
	return strings.Join(v.args, " ")
}
func (v *argsValue) Set(s string) error {
	v.args = append(v.args, s)
	return nil
}

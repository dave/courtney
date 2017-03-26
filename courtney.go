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
	// notest
	env := vos.Os()

	enforceFlag := flag.Bool("e", false, "Enforce 100% code coverage")
	verboseFlag := flag.Bool("v", false, "Verbose output")
	outputFlag := flag.String("o", "", "Override coverage file location")
	argsFlag := new(argsValue)
	flag.Var(argsFlag, "t", "Argument to pass to the 'go test' command. Can be used more than once.")
	loadFlag := flag.String("l", "", "Load coverage file(s) instead of running 'go test'")

	flag.Parse()

	setup := &shared.Setup{
		Env:      env,
		Paths:    patsy.NewCache(env),
		Enforce:  *enforceFlag,
		Verbose:  *verboseFlag,
		Output:   *outputFlag,
		TestArgs: argsFlag.args,
		Load:     *loadFlag,
	}
	if err := Run(setup); err != nil {
		log.Fatal(err)
	}
}

// Run initiates the command with the provided setup
func Run(setup *shared.Setup) error {

	if err := setup.Parse(flag.Args()); err != nil {
		return err
	}

	s := scanner.New(setup)
	if err := s.LoadProgram(); err != nil {
		return err
	}
	if err := s.ScanPackages(); err != nil {
		return err
	}

	t := tester.New(setup)
	if setup.Load == "" {
		if err := t.Test(); err != nil {
			return err
		}
	} else {
		if err := t.Load(); err != nil {
			return err
		}
	}
	if err := t.ProcessExcludes(s.Excludes); err != nil {
		return err
	}
	if err := t.Save(); err != nil {
		return err
	}
	if err := t.Enforce(); err != nil {
		return err
	}

	return nil
}

type argsValue struct {
	args []string
}

var _ flag.Value = (*argsValue)(nil)

func (v *argsValue) String() string {
	// notest
	if v == nil {
		return ""
	}
	return strings.Join(v.args, " ")
}
func (v *argsValue) Set(s string) error {
	// notest
	v.args = append(v.args, s)
	return nil
}

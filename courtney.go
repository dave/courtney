package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dave/patsy"
	"github.com/dave/patsy/vos"
	"github.com/triarius/courtney/scanner"
	"github.com/triarius/courtney/shared"
	"github.com/triarius/courtney/tester"
)

func main() {
	// notest
	env := vos.Os()

	enforceFlag := flag.Bool("e", false, "Enforce 100% code coverage")
	verboseFlag := flag.Bool("v", false, "Verbose output")
	outputFlag := flag.String("o", "", "Override coverage file location")
	exludePathsFlag := flag.String("x", "", "Exclude subdirs from tests")
	argsFlag := new(argsValue)
	flag.Var(argsFlag, "t", "Argument to pass to the 'go test' command. Can be used more than once.")
	loadFlag := flag.String("l", "", "Load coverage file(s) instead of running 'go test'")

	flag.Parse()

	setup := &shared.Setup{
		Env:          env,
		Paths:        patsy.NewCache(env),
		Enforce:      *enforceFlag,
		Verbose:      *verboseFlag,
		ExcludePaths: make(map[string]bool),
		Output:       *outputFlag,
		TestArgs:     argsFlag.args,
		Load:         *loadFlag,
	}

	for _, path := range strings.Split(*exludePathsFlag, ",") {
		setup.ExcludePaths[path] = true
	}

	if err := Run(setup); err != nil {
		fmt.Println(err)
		os.Exit(1)
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

	return t.Enforce()
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

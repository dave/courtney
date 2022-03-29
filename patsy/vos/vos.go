// Package vos is a virtual os tool. It allows mocking of the os.Environ,
// os.Getenv and os.Getwd functions.
package vos

import (
	"io"

	"github.com/dave/courtney/patsy/vos/mock"
	"github.com/dave/courtney/patsy/vos/os"
)

// Env provides an interface with methods similar to os.Environ, os.Getenv and
// os.Getwd functions.
type Env interface {
	Environ() []string

	Getenv(key string) string
	Getwd() (string, error)
	Stdout() io.Writer
	Stderr() io.Writer
	Stdin() io.Reader

	Setenv(key, value string) error
	Setwd(dir string) error
	Setstdout(io.Writer)
	Setstderr(io.Writer)
	Setstdin(io.Reader)
}

var _ Env = (*os.Env)(nil)
var _ Env = (*mock.Env)(nil)

// Os returns an Env that provides a direct pass-through to the os package. Use
// this in production.
func Os() Env {
	return os.New()
}

// Mock returns an Env that provides a mock for the os package. Use this in
// testing.
func Mock() Env {
	return mock.New()
}

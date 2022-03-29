package mock

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

func New() *Env {
	return &Env{
		varsm: new(sync.RWMutex),
		vars:  make(map[string]string),
	}
}

type Env struct {
	varsm  *sync.RWMutex
	vars   map[string]string
	wd     string
	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
}

func (e *Env) Stdout() io.Writer {
	if e.stdout != nil {
		return e.stdout
	}
	return os.Stdout
}

func (e *Env) Setstdout(w io.Writer) {
	e.stdout = w
}

func (e *Env) Stderr() io.Writer {
	if e.stderr != nil {
		return e.stderr
	}
	return os.Stderr
}

func (e *Env) Setstderr(w io.Writer) {
	e.stderr = w
}

func (e *Env) Stdin() io.Reader {
	if e.stdin != nil {
		return e.stdin
	}
	return os.Stdin
}

func (e *Env) Setstdin(r io.Reader) {
	e.stdin = r
}

func (e *Env) Getenv(key string) string {
	if e.vars == nil {
		return os.Getenv(key)
	}
	v, ok := e.getVar(key)
	if !ok {
		return os.Getenv(key)
	}
	return v
}

func (e *Env) Setenv(key, value string) error {
	e.setVar(key, value)
	return nil
}

func (e *Env) Getwd() (string, error) {
	if e.wd == "" {
		return os.Getwd()
	}
	return e.wd, nil
}

func (e *Env) Setwd(dir string) error {
	e.wd = dir
	return nil
}

// Environ returns a copy of strings representing the environment, in the form "key=value".
func (e *Env) Environ() []string {
	if len(e.vars) == 0 {
		return os.Environ()
	}
	var out []string
	merged := make(map[string]string)
	for _, e := range os.Environ() {
		// Add the environment variables from the system
		parts := strings.Split(e, "=")
		merged[parts[0]] = parts[1]
	}
	// Overwrite with the mocked environment variables
	e.writeVars(merged)
	for k, v := range merged {
		// Join them back together in Environ syntax
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func (e *Env) getVar(key string) (string, bool) {
	e.varsm.RLock()
	defer e.varsm.RUnlock()
	v, ok := e.vars[key]
	return v, ok
}

func (e *Env) setVar(key, value string) {
	e.varsm.Lock()
	defer e.varsm.Unlock()
	e.vars[key] = value
}

func (e *Env) writeVars(target map[string]string) {
	e.varsm.RLock()
	defer e.varsm.RUnlock()
	for k, v := range e.vars {
		target[k] = v
	}
}

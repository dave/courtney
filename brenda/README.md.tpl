[![Build Status](https://travis-ci.org/dave/brenda.svg?branch=master)](https://travis-ci.org/dave/brenda) [![Go Report Card](https://goreportcard.com/badge/github.com/dave/brenda)](https://goreportcard.com/report/github.com/dave/brenda) [![codecov](https://codecov.io/gh/dave/brenda/branch/master/graph/badge.svg)](https://codecov.io/gh/dave/brenda)

# Brenda

Brenda is a boolean expression solver.

Given an AST expression containing an arbitrary combination of `!`, `&&` 
and `||` expressions, it is possible to solve the boolean state of certain 
components. For example:

{{ "ExampleNewSolver_else" | example }}

Some inputs may be unknown:

{{ "ExampleNewSolver_unknown" | example }}

Some branches may be impossible:

{{ "ExampleNewSolver_impossible" | example }}

Brenda supports complex components, and can detect the inverse use of `==`, `!=`, 
`<`, `>=`, `>` and `<=`:

{{ "ExampleNewSolver_mixed" | example }}

Here's an example of the full usage:

{{ "ExampleNewSolver_usage" | example }}
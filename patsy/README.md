[![Build Status](https://travis-ci.org/dave/patsy.svg?branch=master)](https://travis-ci.org/dave/patsy) [![Go Report Card](https://goreportcard.com/badge/github.com/dave/patsy)](https://goreportcard.com/report/github.com/dave/patsy) [![codecov](https://codecov.io/gh/dave/patsy/branch/master/graph/badge.svg)](https://codecov.io/gh/dave/patsy)

# Patsy

Patsy is a package helper for Go.

# Dir
Dir returns the filesystem path for the directory corresponding to the go
package path provided.

# Path
Path returns the go package path corresponding to the filesystem directory
provided.

# Cache
NewCache returns a new *Cache, allowing cached access to patsy utility
functions.

[![Build Status](https://travis-ci.org/dave/courtney.svg?branch=master)](https://travis-ci.org/dave/courtney) [![Go Report Card](https://goreportcard.com/badge/github.com/dave/courtney)](https://goreportcard.com/report/github.com/dave/courtney) [![codecov](https://codecov.io/gh/dave/courtney/branch/master/graph/badge.svg)](https://codecov.io/gh/dave/courtney)

# Courtney

Courtney makes your code coverage more meaningful, by excluding some of the 
less important parts.

1. Packages are tested with coverage.  
2. Coverage files are merged.  
3. Some code is less important to test. This is excluded from the coverage file.      
4. Optionally we enforce that all remaining code is covered.

# Excludes 
What do we exclude from the coverage report?

### Blocks including a panic 
If you need to test that your code panics correctly, it should probably be an 
error rather than a panic. 

### Notest comments
Blocks or files with a `// notest` comment are excluded.

### Generated code
Generated code files, that contain the respective comment line that is
specified by the [Go Team](https://github.com/golang/go/issues/41196) in
[`go generate`](https://golang.org/s/generatedcode).

This exclude is disabled by default. Use flag `-g` to enable this behavior.

### Blocks returning a error tested to be non-nil
We only exclude blocks where the error being returned has been tested to be 
non-nil, so:

```go
err := foo()
if err != nil {
    return err // excluded 
}
```

... however:

```go
if i == 0 {
    return errors.New("...") // not excluded
}
```

All errors are originally created with code similar to `errors.New`, which is 
not excluded from the coverage report - it's important your tests hit these. 

It's less important your tests cover all the points that an existing non-nil 
error is passed back, so these are excluded. 

A few more rules:
* If multiple return values are returned, error must be the last, and all 
others must be nil or zero values.  
* We also exclude blocks returning an error which is the result of a function 
taking a non-nil error as a parameter, e.g. `errors.Wrap(err, "...")`.  
* We also exclude blocks containing a bare return statement, where the function 
has named result parameters, and the last result is an error that has been 
tested non-nil. Be aware that in this scenario no attempt is made to verify 
that the other result parameters are zero values.  

# Limitations  
* Having test coverage doesn't mean your code is well tested.  
* It's up to you to make sure that your tests explore the appropriate edge 
  cases.  

# Install
```
go get -u github.com/dave/courtney 
```

# Usage
Run the courtney command followed by a list of packages. Use `.` for the 
package in the current directory, and adding `/...` tests all sub-packages 
recursively. If no packages are provided, the default is `./...`.

To test the current package, and all sub-packages recursively: 
```
courtney
```

To test just the current package: 
```
courtney .
```

To test the `a` package, it's sub-packages and the `b` package: 
```
courtney github.com/dave/a/... github.com/dave/b
```

# Options
### Enforce: -e
`Enforce 100% code coverage.`

The command will exit with an error if any code remains uncovered. Combining a 
CI system with a fully tested package and the `-e` flag is extremely useful. It 
ensures any pull request has tests that cover all new code. For example, [here 
is a PR](https://github.com/dave/courtney/pull/5) for this project that lacks 
tests. As you can see the Travis build failed with a descriptive error. 

### Output: -o
`Override coverage file location.`

Provide a custom location for the coverage file. The default is `./coverage.out`.

### Test flags: -t
`Argument to pass to the 'go test' command.`

If you have special arguments to pass to the `go test` command, add them here. 
Add one `-t` flag per argument e.g.
```
courtney -t="-count=2" -t="-parallel=4"
```

### Verbose: -v
`Verbose output`

All the output from the `go test -v` command is shown.

### Exclude generated code: -g
`Exclude generated code from coverage`

All generated code is excluded from coverage.

# Output
Courtney will fail if the tests fail. If the tests succeed, it will create or
overwrite a `coverage.out` file in the current directory.

# Continuous integration
To upload your coverage to [codecov.io](https://codecov.io/) via 
[travis](https://travis-ci.org/), use a `.travis.yml` file something like this:

```yml
language: go
go:
  - 1.x
notificaitons:
  email:
    recipients: <your-email>
    on_failure: always
install:
  - go get -u github.com/dave/courtney
  - go get -t -v ./...
script:
  - courtney
after_success:
  - bash <(curl -s https://codecov.io/bash)
```

For [coveralls.io](https://coveralls.io/), use something like this:

```yml
language: go
go:
    - 1.x
notificaitons:
  email:
    recipients: <your-email>
    on_failure: always
install:
  - go get -u github.com/mattn/goveralls
  - go get -u github.com/dave/courtney
  - go get -t -v ./...
script:
  - courtney
after_success:
  - goveralls -coverprofile=coverage.out -service=travis-ci
```
[![Build Status](https://travis-ci.org/dave/courtney.svg?branch=master)](https://travis-ci.org/dave/courtney) [![Go Report Card](https://goreportcard.com/badge/github.com/dave/courtney)](https://goreportcard.com/report/github.com/dave/courtney) [![codecov](https://codecov.io/gh/dave/courtney/branch/master/graph/badge.svg)](https://codecov.io/gh/dave/courtney)

# Courtney

Courtney is a coverage tool for Go.

Courtney runs your tests, merges and prepares the coverage files.

1. Packages are tested with coverage.  
2. Coverage files are merged.  
3. Some code doesn't need to be tested. This is excluded from the coverage files.      
4. Optionally we enforce that all remaining code is covered.  

# Excludes 
What do we exclude from the coverage report? ([more details](#details))
1. Blocks returning an error that has been tested non-nil.  
2. Blocks or files with a `// notest` comment.  
3. Blocks including a panic.  

# Limitations
* Having test coverage doesn't mean your code is well tested.  
* It's up to you to make sure that your tests explore the appropriate edge 
  cases.  
* However, not having test coverage is a good indicator that something isn't 
  being tested.  

# Benefits
Courtney makes your code coverage more meaningful. If you enforce 100% test 
coverage, then any new functionality will have to be accompanied with tests.

# Details
A few more rules:
* If multiple return values are returned, error must be the last, and all others must be nil or zero values.  
* We also exclude blocks returning an error which is the result of a function taking a non-nil error as a parameter, e.g. `errors.Wrap(err, "")`.  
* We also exclude blocks containing a bare return statement, where the function has named result parameters, and the last result is an error that has been tested to be non-nil. Be aware that in this scenario no attempt is made to verify that the other result parameters are nil or zero.  

# Install
```
go get -u github.com/dave/courtney 
```

# Usage
Run the courtney command followed by a list of packages. You can use `.` for 
the package in the current directory, and adding the `/...` suffix tests all 
sub-packages recursively. If no packages are provided, the default is `./...`.

To test the current package, and all sub-packages recursively: 
```
courtney
```

To test `tester`, it's sub-packages and the `scanner` package: 
```
courtney github.com/dave/courtney/tester/... github.com/dave/courtney/scanner
```

# Output
Courtney will fail if the tests fail. If the tests succeed, it will create or
overwrite a `coverage.out` file in the current directory.

# Coverage
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

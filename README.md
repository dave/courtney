[![Build Status](https://travis-ci.org/dave/courtney.svg?branch=master)](https://travis-ci.org/dave/courtney) [![Go Report Card](https://goreportcard.com/badge/github.com/dave/courtney)](https://goreportcard.com/report/github.com/dave/courtney) [![codecov](https://codecov.io/gh/dave/courtney/branch/master/graph/badge.svg)](https://codecov.io/gh/dave/courtney)

# Courtney

Courtney is a coverage tool for Go.

Courtney runs your tests and prepares your code coverage files.

1. All packages are tested with coverage.    
2. All the coverage files are merged.  
3. Some parts of the code don't need to be tested. We disable these in the coverage files.  
4. You should have 100% coverage! Optionally we enforce this.     

# Excludes 
What doesn't need to be tested?
1. Blocks including a panic.  
2. Blocks returning an error that has been tested to be non-nil. ([more details](#details))
3. Blocks or files with a `// notest` comment.  

# Limitations
What courtney doesn't do:
Courtney doesn't mean your tests are complete. Courtney doesn't mean your tests 
are good. 100% test coverage doesn't mean all the edge cases are being explored.

# Benefits
What courtney does do:
Courtney makes your code coverage more meaningful. If you enforce 100% test 
coverage, then any new functionality will have to be accompanied with tests.

# Details
A few more rules:
* If multiple return values are returned, error must be the last, and all others must be nil or zero values.  
* We also exclude blocks returning an error which is the result of a function taking a non-nil error as a parameter, e.g. `errors.Wrap(err, "")`.  
* We also exclude blocks containing a bare return statement, where the function has named result parameters, and the last result is an error that has been tested to be non-nil. Be aware that in this scenario no attempt is made to verify that the other result parameters are nil or zero.  

# Install
```
go get -u github.com/dave/courtney/... 
```

# Usage
Run the courtney command followed by a list of packages. You can use `.` for 
the package in the current directory, and adding the `/...` suffix tests all 
sub-packages recursively.

To test the current package recursively: 
```
courtney ./...
```

To test `tester` recursively and `scanner`: 
```
courtney github.com/dave/courtney/tester/... github.com/dave/courtney/scanner
```

# Output
Courtney will fail if the tests fail. If the tests succeed, it will create or
overwrite a `coverage.out` file in the current directory.

# Coverage
To upload your coverage to [codecov.io](https://codecov.io/) via 
[travis](https://travis-ci.org/), use the following `.travis.yml` file:

```yml
language: go
go:
    - 1.x
notificaitons:
  email:
    recipients: <your-email>
    on_failure: always
install:
  - go get -u github.com/dave/courtney/...
  - go get -t -v ./...
script:
  - courtney ./...
after_success:
  - bash <(curl -s https://codecov.io/bash)
```

# Courtney is a coverage tool for Go.
Courtney runs your tests and prepares your code coverage files.

1) All packages are tested with coverage.    
2) All the coverage files are merged.  
3) Some parts of the code don't need to be tested. We disable these in the coverage files.  
4) You should have 100% coverage! Optionally we enforce this.     

# What doesn't need to be tested?
1) Blocks including a panic.  
2) Blocks returning an error that has been tested to be non-nil*.
3) Blocks containing the "//notest" comment.  
4) Files containing the "//notest-file" comment.  
5) Packages containing the "//notest-package" comment.  

# What courtney doesn't do
Courtney doesn't mean your tests are complete. Courtney doesn't mean your tests 
are good. 100% test coverage doesn't mean all the edge cases are being explored.

# What courtney does
Courtney makes your code coverage stats much more meaningful. If you enforce 
100% test coverage, then any new functionality will have to be accompanied with 
tests.

* If multiple return values are returned, error must be the last, and all 
  others must be nil or zero values. We also include blocks returning an error 
  which is the result of a function taking a non-nil error as a parameter, e.g.
  github.com/pkg/errors.Wrap(error, string).

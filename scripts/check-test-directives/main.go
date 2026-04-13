// Command check-test-directives verifies that every test file in the repository has
// the correct //go:build directive for the "integration" build tag.
//
// Unit tests and integration tests are separated by build tag so that a plain
// "go test ./..." only runs unit tests. Integration tests opt in with
// "//go:build integration" and are run separately (e.g. with -tags=integration).
//
// To keep things consistent, we enforce a naming convention: files matching
// integration(_...)?_test.go are integration tests, and everything else is a
// unit test. This tool checks that the two stay in sync — integration-named
// files must have "integration" in their build constraint, and all other test
// files must have "!integration".
//
// Usage:
//
//	go run . [directory]
//
// If no directory is given, the current working directory is used.
package main

import (
	"fmt"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

var (
	testFileRx            = regexp.MustCompile(`_test\.go$`)
	integrationTestFileRx = regexp.MustCompile(`integration(_[a-z0-9_]+)?_test\.go$`)
	helpersTestFileRx     = regexp.MustCompile(`helpers(_[a-z0-9_]+)?_test\.go$`)

	ignoreDirectories = map[string]bool{
		".git": true, "scripts": true, ".tmp": true, "magefiles": true, ".cache": true,
	}
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	root, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Checking build directives in test files at %q\n", root)

	var errs []string

	_ = filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && ignoreDirectories[info.Name()] {
			return filepath.SkipDir
		}

		name := info.Name()
		if !testFileRx.MatchString(name) || helpersTestFileRx.MatchString(name) {
			return nil
		}

		isIntegration := integrationTestFileRx.MatchString(name)
		if err := checkFile(path, isIntegration); err != nil {
			rel, _ := filepath.Rel(root, path)
			errs = append(errs, fmt.Sprintf("  %s: %v", rel, err))
		}
		return nil
	})

	if len(errs) > 0 {
		fmt.Println("\n✖ Failed directive expectations:")
		for _, e := range errs {
			fmt.Println(e)
		}
		os.Exit(1)
	}

	fmt.Println("\n✔ All directives match expectations")
}

func checkFile(path string, wantIntegration bool) error {
	expr, err := parseBuildConstraint(path)
	if err != nil {
		return err
	}

	positive, negative := hasTag(expr, "integration")

	if wantIntegration && !positive {
		return fmt.Errorf("integration test missing 'integration' build tag")
	}
	if !wantIntegration && !negative {
		return fmt.Errorf("non-integration test missing '!integration' build tag")
	}

	return nil
}

func parseBuildConstraint(path string) (constraint.Expr, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	for _, group := range f.Comments {
		for _, c := range group.List {
			if constraint.IsGoBuild(c.Text) {
				return constraint.Parse(c.Text)
			}
		}
	}

	return nil, fmt.Errorf("no //go:build directive found")
}

// hasTag walks the constraint expression tree and reports whether the given tag
// appears in a positive or negative (negated) position. This avoids needing to
// evaluate the full expression, so platform tags like "windows" don't interfere.
func hasTag(expr constraint.Expr, tag string) (positive, negative bool) {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		if e.Tag == tag {
			return true, false
		}
	case *constraint.NotExpr:
		p, n := hasTag(e.X, tag)
		return n, p
	case *constraint.AndExpr:
		p1, n1 := hasTag(e.X, tag)
		p2, n2 := hasTag(e.Y, tag)
		return p1 || p2, n1 || n2
	case *constraint.OrExpr:
		p1, n1 := hasTag(e.X, tag)
		p2, n2 := hasTag(e.Y, tag)
		return p1 || p2, n1 || n2
	}
	return false, false
}

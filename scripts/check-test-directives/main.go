package main

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	testFileRx            = regexp.MustCompile("_test.go$")
	integrationTestFileRx = regexp.MustCompile("integration(_[a-z0-9_]+)?_test.go$")
	helpersTestFileRx     = regexp.MustCompile("helpers(_[a-z0-9_]+)?_test.go$")

	workingDirectory string
	errors           []pathError

	integrationTag    = buildTag{name: "integration", value: true}
	nonIntegrationTag = buildTag{name: "non-integration", value: false}

	tagOverrides = tagOverridesFilesMap{
		"executors/custom/terminal_test.go": tagOverridesMap{
			"windows": false,
		},
		"helpers/archives/zip_create_unix_test.go": tagOverridesMap{
			"windows": false,
		},
		"executors/docker/internal/volumes/parser/windows_path_test.go": tagOverridesMap{
			"windows": false,
		},
	}
)

type tagOverridesFilesMap map[string]tagOverridesMap

type tagOverridesMap map[string]bool

type pathError struct {
	path string
	err  error
}

type buildTag struct {
	name  string
	value bool
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("checking working directory: %v", err))
	}

	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	workingDirectory = filepath.Clean(path)
}

func main() {
	fmt.Printf("Analyse build directives in test files at %q\n", workingDirectory)

	walkNonIntegrationTestFiles(workingDirectory, integrationBuildConstraintDoesntExist)
	walkIntegrationTestFiles(workingDirectory, integrationBuildConstraintExists)

	checkErrors()
}

func walkNonIntegrationTestFiles(rootPath string, fn func(path string) error) {
	walkTestFiles("non-integration", rootPath, func(walkPath string, info fs.FileInfo, _ error) error {
		name := info.Name()
		if !integrationTestFileRx.MatchString(name) && !helpersTestFileRx.MatchString(name) {
			return fn(walkPath)
		}

		return nil
	})
}

func walkIntegrationTestFiles(rootPath string, fn func(path string) error) {
	walkTestFiles("integration", rootPath, func(walkPath string, info fs.FileInfo, _ error) error {
		if integrationTestFileRx.MatchString(info.Name()) {
			return fn(walkPath)
		}

		return nil
	})
}

func walkTestFiles(testType string, rootPath string, walkFunc filepath.WalkFunc) {
	fmt.Printf("\nChecking %s test files...\n", testType)

	err := filepath.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		name := info.Name()
		if info.IsDir() {
			if name == ".git" {
				return filepath.SkipDir
			}
		}

		if !testFileRx.MatchString(name) {
			return nil
		}

		recordError(path, walkFunc(path, info, err))

		return nil
	})

	if err != nil {
		panic(fmt.Sprintf("walking files: %v", err))
	}
}

func recordError(path string, err error) {
	if err == nil {
		return
	}

	pe := pathError{path: path, err: err}
	errors = append(errors, pe)
}

func integrationBuildConstraintDoesntExist(path string) error {
	return checkBuildConstraints(path, nonIntegrationTag)
}

func integrationBuildConstraintExists(path string) error {
	return checkBuildConstraints(path, integrationTag)
}

func checkBuildConstraints(path string, integrationTag buildTag) error {
	fmt.Printf(" -> %s...\n", trimWDFromPath(path))

	comments, err := parseFile(path)
	if err != nil {
		return err
	}

	expressions, err := scanAndParseBuildConstraints(comments)
	if err != nil {
		return err
	}

	for _, expr := range expressions {
		if !expr.Eval(integrationEvalFn(path, integrationTag)) {
			return fmt.Errorf(
				"invalid integration build constraint %q evaluation for %s test file",
				expr.String(),
				integrationTag.name,
			)
		}
	}

	return nil
}

func parseFile(path string) ([]*ast.CommentGroup, error) {
	fileSet := token.NewFileSet()
	f, err := parser.ParseFile(fileSet, path, nil, parser.PackageClauseOnly+parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	if len(f.Comments) < 1 {
		return nil, fmt.Errorf("missing top-level comments")
	}

	return f.Comments, nil
}

func scanAndParseBuildConstraints(comments []*ast.CommentGroup) ([]constraint.Expr, error) {
	var expressions []constraint.Expr

	for _, group := range comments {
		for _, line := range group.List {
			text := line.Text
			if constraint.IsGoBuild(text) || constraint.IsPlusBuild(text) {
				expr, err := constraint.Parse(text)
				if err != nil {
					return nil, fmt.Errorf("parsing constraint %q: %w", text, err)
				}
				expressions = append(expressions, expr)
			}
		}
	}

	return expressions, nil
}

func integrationEvalFn(path string, integrationTag buildTag) func(tag string) bool {
	return func(tag string) bool {
		if tag == "integration" {
			return integrationTag.value
		}

		m, ok := tagOverrides[trimWDFromPath(path)]
		if ok {
			v, ok := m[tag]
			if ok {
				return v
			}
		}

		return true
	}
}

func checkErrors() {
	fmt.Println()

	if len(errors) < 1 {
		fmt.Println("✔ All directives match expectations")
		return
	}

	fmt.Println("✖ Failed directives expectations:")

	for _, e := range errors {
		fmt.Printf("%80s: %v\n", trimWDFromPath(e.path), e.err)
	}

	os.Exit(1)
}

func trimWDFromPath(path string) string {
	return strings.TrimPrefix(path, workingDirectory+"/")
}

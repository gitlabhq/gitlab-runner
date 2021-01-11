package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

const coberturaDTDDecl = `<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">`

var byFiles bool

func fatal(format string, a ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}

func main() {
	var ignore Ignore

	flag.BoolVar(&byFiles, "by-files", false, "code coverage by file, not class")
	flag.BoolVar(&ignore.GeneratedFiles, "ignore-gen-files", false, "ignore generated files")
	ignoreDirsRe := flag.String("ignore-dirs", "", "ignore dirs matching this regexp")
	ignoreFilesRe := flag.String("ignore-files", "", "ignore files matching this regexp")

	flag.Parse()

	var err error
	if *ignoreDirsRe != "" {
		ignore.Dirs, err = regexp.Compile(*ignoreDirsRe)
		if err != nil {
			fatal("Bad -ignore-dirs regexp: %s\n", err)
		}
	}

	if *ignoreFilesRe != "" {
		ignore.Files, err = regexp.Compile(*ignoreFilesRe)
		if err != nil {
			fatal("Bad -ignore-files regexp: %s\n", err)
		}
	}

	if err := convert(os.Stdin, os.Stdout, &ignore); err != nil {
		fatal("code coverage conversion failed: %s", err)
	}
}

func convert(in io.Reader, out io.Writer, ignore *Ignore) error {
	profiles, err := ParseProfiles(in, ignore)
	if err != nil {
		return err
	}

	pkgs, err := getPackages(profiles)
	if err != nil {
		return err
	}

	sources := make([]*Source, 0)
	pkgMap := make(map[string]*packages.Package)
	for _, pkg := range pkgs {
		sources = appendIfUnique(sources, pkg.Module.Dir)
		pkgMap[pkg.ID] = pkg
	}

	coverage := Coverage{Sources: sources, Packages: nil, Timestamp: time.Now().UnixNano() / int64(time.Millisecond)}
	if err := coverage.parseProfiles(profiles, pkgMap, ignore); err != nil {
		return err
	}

	_, _ = fmt.Fprint(out, xml.Header)
	_, _ = fmt.Fprintln(out, coberturaDTDDecl)

	encoder := xml.NewEncoder(out)
	encoder.Indent("", "  ")
	if err := encoder.Encode(coverage); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(out)
	return nil
}

func getPackages(profiles []*Profile) ([]*packages.Package, error) {
	var pkgNames []string
	for _, profile := range profiles {
		pkgNames = append(pkgNames, getPackageName(profile.FileName))
	}
	return packages.Load(&packages.Config{Mode: packages.NeedFiles | packages.NeedModule}, pkgNames...)
}

func appendIfUnique(sources []*Source, dir string) []*Source {
	for _, source := range sources {
		if source.Path == dir {
			return sources
		}
	}
	return append(sources, &Source{dir})
}

func getPackageName(filename string) string {
	pkgName, _ := filepath.Split(filename)
	// TODO(boumenot): Windows vs. Linux
	return strings.TrimRight(strings.TrimRight(pkgName, "\\"), "/")
}

func findAbsFilePath(pkg *packages.Package, profileName string) string {
	filename := filepath.Base(profileName)
	for _, fullpath := range pkg.GoFiles {
		if filepath.Base(fullpath) == filename {
			return fullpath
		}
	}
	return ""
}

func (cov *Coverage) parseProfiles(profiles []*Profile, pkgMap map[string]*packages.Package, ignore *Ignore) error {
	cov.Packages = []*Package{}
	for _, profile := range profiles {
		pkgName := getPackageName(profile.FileName)
		pkgPkg := pkgMap[pkgName]
		if err := cov.parseProfile(profile, pkgPkg, ignore); err != nil {
			return err
		}
	}
	cov.LinesValid = cov.NumLines()
	cov.LinesCovered = cov.NumLinesWithHits()
	cov.LineRate = cov.HitRate()
	return nil
}

func (cov *Coverage) parseProfile(profile *Profile, pkgPkg *packages.Package, ignore *Ignore) error {
	if pkgPkg == nil || pkgPkg.Module == nil {
		return fmt.Errorf("package required when using go modules")
	}
	fileName := profile.FileName[len(pkgPkg.Module.Path)+1:]
	absFilePath := findAbsFilePath(pkgPkg, profile.FileName)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, absFilePath, nil, 0)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadFile(absFilePath)
	if err != nil {
		return err
	}

	if ignore.Match(fileName, data) {
		return nil
	}

	pkgPath, _ := filepath.Split(fileName)
	pkgPath = strings.TrimRight(strings.TrimRight(pkgPath, "/"), "\\")
	pkgPath = filepath.Join(pkgPkg.Module.Path, pkgPath)
	// TODO(boumenot): package paths are not file paths, there is a consistent separator
	pkgPath = strings.Replace(pkgPath, "\\", "/", -1)

	var pkg *Package
	for _, p := range cov.Packages {
		if p.Name == pkgPath {
			pkg = p
		}
	}
	if pkg == nil {
		pkg = &Package{Name: pkgPkg.ID, Classes: []*Class{}}
		cov.Packages = append(cov.Packages, pkg)
	}
	visitor := &fileVisitor{
		fset:     fset,
		fileName: fileName,
		fileData: data,
		classes:  make(map[string]*Class),
		pkg:      pkg,
		profile:  profile,
	}
	ast.Walk(visitor, parsed)
	pkg.LineRate = pkg.HitRate()
	return nil
}

type fileVisitor struct {
	fset     *token.FileSet
	fileName string
	fileData []byte
	pkg      *Package
	classes  map[string]*Class
	profile  *Profile
}

func (v *fileVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		class := v.class(n)
		method := v.method(n)
		method.LineRate = method.Lines.HitRate()
		class.Methods = append(class.Methods, method)
		for _, line := range method.Lines {
			class.Lines = append(class.Lines, line)
		}
		class.LineRate = class.Lines.HitRate()
	}
	return v
}

func (v *fileVisitor) method(n *ast.FuncDecl) *Method {
	method := &Method{Name: n.Name.Name}
	method.Lines = []*Line{}

	start := v.fset.Position(n.Pos())
	end := v.fset.Position(n.End())
	startLine := start.Line
	startCol := start.Column
	endLine := end.Line
	endCol := end.Column
	// The blocks are sorted, so we can stop counting as soon as we reach the end of the relevant block.
	for _, b := range v.profile.Blocks {
		if b.StartLine > endLine || (b.StartLine == endLine && b.StartCol >= endCol) {
			// Past the end of the function.
			break
		}
		if b.EndLine < startLine || (b.EndLine == startLine && b.EndCol <= startCol) {
			// Before the beginning of the function
			continue
		}
		for i := b.StartLine; i <= b.EndLine; i++ {
			method.Lines.AddOrUpdateLine(i, int64(b.Count))
		}
	}
	return method
}

func (v *fileVisitor) class(n *ast.FuncDecl) *Class {
	var className string
	if byFiles {
		//className = filepath.Base(v.fileName)
		//
		// NOTE(boumenot): ReportGenerator creates links that collide if names are not distinct.
		// This could be an issue in how I am generating the report, but I have not been able
		// to figure it out.  The work around is to generate a fully qualified name based on
		// the file path.
		//
		// src/lib/util/foo.go -> src.lib.util.foo.go
		className = strings.Replace(v.fileName, "/", ".", -1)
		className = strings.Replace(className, "\\", ".", -1)
	} else {
		className = v.recvName(n)
	}
	class := v.classes[className]
	if class == nil {
		class = &Class{Name: className, Filename: v.fileName, Methods: []*Method{}, Lines: []*Line{}}
		v.classes[className] = class
		v.pkg.Classes = append(v.pkg.Classes, class)
	}
	return class
}

func (v *fileVisitor) recvName(n *ast.FuncDecl) string {
	if n.Recv == nil {
		return "-"
	}
	recv := n.Recv.List[0].Type
	start := v.fset.Position(recv.Pos())
	end := v.fset.Position(recv.End())
	name := string(v.fileData[start.Offset:end.Offset])
	return strings.TrimSpace(strings.TrimLeft(name, "*"))
}

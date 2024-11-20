package docs

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"sort"
	"strings"

	"github.com/samber/lo"
)

var supportedKubeClientNames = []string{
	"kubeClient",
	"KubeClient",
	"client",
}

type simplePosition struct {
	fileName string
	line     int
}

type configFlag struct {
	Name  string
	Value string
}

func (v configFlag) valid() bool {
	return v.Name != "" && v.Value != ""
}

func (v configFlag) String() string {
	return fmt.Sprintf("%s=%s", v.Name, v.Value)
}

type verb struct {
	Verb        string
	ConfigFlags []configFlag
}

func (p verb) String() string {
	if !lo.EveryBy(p.ConfigFlags, func(ff configFlag) bool {
		return ff.valid()
	}) || len(p.ConfigFlags) == 0 {
		return p.Verb
	}

	featureFlagsStrings := lo.Map(p.ConfigFlags, func(ff configFlag, _ int) string {
		return ff.String()
	})
	sort.Strings(featureFlagsStrings)

	return fmt.Sprintf("%s (`%s`)", p.Verb, strings.Join(featureFlagsStrings, ", "))
}

type PermissionsGroup map[string][]verb

// Beware, we currently only support the CoreV1 API. If we add resources that require a different API group,
// for example "rbac.authorization.k8s.io", we will need to update this function to parse the API group too.
func parsePermissions() (PermissionsGroup, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseDir(fset, "executors/kubernetes", filterTestFiles, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	positions := map[simplePosition]token.Pos{}
	permissions := PermissionsGroup{}

	for _, pkg := range f {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(node ast.Node) bool {
				return inspectNode(fset, positions, node)
			})

			processPermissions(fset, f.Comments, positions, permissions)
		}
	}

	var errs []string
	for _, pos := range positions {
		errs = append(errs, fmt.Sprintf("%v Missing Kube API annotations.", fset.Position(pos)))
	}

	if len(errs) == 0 {
		return permissions, nil
	}

	return nil, fmt.Errorf("%s\n\nAnnotations must be written as comments directly above each `kubeClient.CoreV1` call and in the format of // kubeAPI: <Resource>, <Verb>, <FF=VALUE>(optional)", strings.Join(errs, "\n"))
}

func filterTestFiles(fileInfo fs.FileInfo) bool {
	return !strings.HasSuffix(fileInfo.Name(), "_test.go")
}

func inspectNode(fset *token.FileSet, positions map[simplePosition]token.Pos, node ast.Node) bool {
	expr, ok := node.(*ast.CallExpr)
	if !ok {
		return true
	}

	sel, ok := expr.Fun.(*ast.SelectorExpr)
	if !ok {
		return true
	}

	if sel.Sel.Name != "CoreV1" {
		return true
	}

	var name string
	switch selectorIdents := sel.X.(type) {
	case *ast.SelectorExpr:
		name = selectorIdents.Sel.Name
	case *ast.Ident:
		name = selectorIdents.Name
	default:
		return true
	}

	// TODO: Check for the type of the field instead of the Name
	if !slices.Contains(supportedKubeClientNames, name) {
		return true
	}

	callPosition := fset.Position(node.Pos())
	sp := simplePosition{
		fileName: callPosition.Filename,
		line:     callPosition.Line - 1,
	}
	positions[sp] = node.Pos()

	return true
}

func processPermissions(fset *token.FileSet, comments []*ast.CommentGroup, positions map[simplePosition]token.Pos, permissions PermissionsGroup) {
	for _, commentGroup := range comments {
		for _, comment := range commentGroup.List {
			position := fset.Position(comment.Pos())
			sp := simplePosition{
				fileName: position.Filename,
				line:     position.Line,
			}
			if _, ok := positions[sp]; !ok {
				continue
			}
			if !strings.HasPrefix(comment.Text, "// kubeAPI:") && !strings.HasPrefix(comment.Text, "//kubeAPI:") {
				continue
			}

			groupPermissions(comment, permissions)

			// TODO: make these checks more robust based on the called methods instead of the comment
			delete(positions, sp)
		}
	}
}

func groupPermissions(comment *ast.Comment, permissions PermissionsGroup) {
	resource, verbs, featureFlags := parseComment(comment)
	if resource == "ignore" {
		return
	}

	if _, ok := permissions[resource]; !ok {
		permissions[resource] = []verb{}
	}

	// Iterate through all verbs. If a verb is already in
	// the resource list, append the feature flags to the existing
	// list. Otherwise add a new entry.
	for _, v := range verbs {
		_, verbIndex, _ := lo.FindIndexOf(permissions[resource], func(p verb) bool {
			return p.Verb == v
		})

		if verbIndex != -1 {
			permissions[resource][verbIndex].ConfigFlags = append(permissions[resource][verbIndex].ConfigFlags, featureFlags...)
		} else {
			permissions[resource] = append(permissions[resource], verb{
				Verb:        v,
				ConfigFlags: featureFlags,
			})
		}
	}

	slices.SortFunc(permissions[resource], func(a, b verb) int {
		return strings.Compare(a.Verb, b.Verb)
	})
}

func parseComment(comment *ast.Comment) (string, []string, []configFlag) {
	components := lo.Map(strings.Split(comment.Text, ","), func(c string, _ int) string {
		return strings.TrimSpace(c)
	})

	i := strings.Index(comment.Text, "kubeAPI:") + len("kubeAPI:")
	resource := strings.TrimSpace(components[0][i:])
	var verbs []string
	var ffs []string
	for _, c := range components[1:] {
		if strings.Contains(c, "=") {
			ffs = append(ffs, c)
			continue
		}

		verbs = append(verbs, c)
	}

	featureFlags := lo.Map(ffs, func(ff string, _ int) configFlag {
		split := strings.Split(ff, "=")
		return configFlag{
			Name:  strings.TrimSpace(split[0]),
			Value: strings.TrimSpace(split[1]),
		}
	})

	return resource, verbs, featureFlags
}

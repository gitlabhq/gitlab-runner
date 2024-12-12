package docs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/samber/lo"
)

var supportedKubernetesClientTypes = []string{"kubernetes.Interface"}

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
func parsePermissions(path string, filter func(fileInfo fs.FileInfo) bool) (PermissionsGroup, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseDir(fset, path, filter, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	positions := map[simplePosition]token.Pos{}
	permissions := PermissionsGroup{}

	for _, pkg := range f {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(node ast.Node) bool {
				inspectNode(fset, positions, node)
				return true
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

	return permissions, fmt.Errorf("%s\n\nAnnotations must be written as comments directly above each Kubernetes Client usage call and in the format of // kubeAPI: <Resource>, <Verb>, <FF=VALUE>(optional)\n", strings.Join(errs, "\n"))
}

func filterTestFiles(fileInfo fs.FileInfo) bool {
	return !strings.HasSuffix(fileInfo.Name(), "_test.go")
}

func inspectNode(fset *token.FileSet, positions map[simplePosition]token.Pos, node ast.Node) {
	expr, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}

	sel, ok := expr.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	root := getTypeRoot(sel.X)
	if root == nil {
		return
	}

	if root.structType == nil {
		if !slices.Contains(supportedKubernetesClientTypes, root.valueType) {
			return
		}
	} else {
		str := root.structType
		callFieldName := getCallFieldName(sel)

		var found bool
		for _, field := range str.Fields.List {
			var name string
			if len(field.Names) == 0 {
				expr, ok := field.Type.(*ast.SelectorExpr)
				if !ok {
					return
				}
				name = expr.Sel.Name
			} else {
				name = field.Names[0].Name
			}

			var buf bytes.Buffer
			_ = printer.Fprint(&buf, fset, field.Type)
			if slices.Contains(supportedKubernetesClientTypes, buf.String()) && name == callFieldName {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	callPosition := fset.Position(node.Pos())
	sp := simplePosition{
		fileName: callPosition.Filename,
		line:     callPosition.Line - 1,
	}
	positions[sp] = node.Pos()
}

func getCallFieldName(expr *ast.SelectorExpr) string {
	if expr == nil {
		return ""
	}

	if expr, ok := expr.X.(*ast.SelectorExpr); ok {
		return getCallFieldName(expr)
	}

	if ident, ok := expr.X.(*ast.Ident); ok && ident.Obj != nil {
		return expr.Sel.Name
	}

	return getCallFieldName(expr.X.(*ast.SelectorExpr))
}

type typeRoot struct {
	structType *ast.StructType
	valueType  string
}

func getTypeRoot(expr any) *typeRoot {
	if expr == nil || reflect.ValueOf(expr).IsNil() {
		return nil
	}

	switch exp := expr.(type) {
	case *ast.Ident:
		return getTypeRoot(exp.Obj)
	case *ast.SelectorExpr:
		ident, ok := exp.X.(*ast.Ident)
		if !ok || ident.Obj != nil {
			return getTypeRoot(exp.X)
		}

		return &typeRoot{
			valueType: fmt.Sprintf("%s.%s", ident.Name, exp.Sel.Name),
		}
	case *ast.Object:
		return getTypeRoot(exp.Decl)
	case *ast.Field:
		return getTypeRoot(exp.Type)
	case *ast.TypeSpec:
		return getTypeRoot(exp.Type)
	case *ast.StarExpr:
		return getTypeRoot(exp.X)
	case *ast.AssignStmt:
		return getTypeRoot(exp.Rhs[0])
	case *ast.ValueSpec:
		selectorExpr, ok := exp.Type.(*ast.SelectorExpr)
		if !ok {
			return nil
		}

		ident, ok := selectorExpr.X.(*ast.Ident)
		if !ok {
			return nil
		}

		return &typeRoot{
			valueType: fmt.Sprintf("%s.%s", ident.Name, selectorExpr.Sel.Name),
		}
	case *ast.StructType:
		return &typeRoot{
			structType: exp,
		}
	default:
		return nil
	}
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
	// list. Otherwise, add a new entry.
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

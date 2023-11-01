package kubernetes

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"

	"github.com/samber/lo"
)

type simplePosition struct {
	fileName string
	line     int
}

type verbFeatureFlag struct {
	Name  string
	Value string
}

func (v verbFeatureFlag) valid() bool {
	return v.Name != "" && v.Value != ""
}

func (v verbFeatureFlag) String() string {
	return fmt.Sprintf("%s=%s", v.Name, v.Value)
}

type verb struct {
	Verb         string
	FeatureFlags []verbFeatureFlag
}

func (p verb) String() string {
	if !lo.EveryBy(p.FeatureFlags, func(ff verbFeatureFlag) bool {
		return ff.valid()
	}) || len(p.FeatureFlags) == 0 {
		return p.Verb
	}

	featureFlagsStrings := lo.Map(p.FeatureFlags, func(ff verbFeatureFlag, _ int) string {
		return ff.String()
	})
	sort.Strings(featureFlagsStrings)

	return fmt.Sprintf("%s (`%s`)", p.Verb, strings.Join(featureFlagsStrings, ", "))
}

type permissionsGroup map[string][]verb

func parsePermissions() (permissionsGroup, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseDir(fset, "executors/kubernetes", nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	positions := map[simplePosition]token.Pos{}
	permissions := permissionsGroup{}

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

	selectorIdents, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return true
	}

	// TODO: Check for the type of the field instead of the Name
	if selectorIdents.Sel.Name != "kubeClient" && selectorIdents.Sel.Name != "KubeClient" {
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

func processPermissions(fset *token.FileSet, comments []*ast.CommentGroup, positions map[simplePosition]token.Pos, permissions permissionsGroup) {
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

func groupPermissions(comment *ast.Comment, permissions permissionsGroup) {
	resource, verbs, featureFlags := parseComment(comment)
	if resource == "ignore" {
		return
	}

	if _, ok := permissions[resource]; !ok {
		permissions[resource] = []verb{}
	}

	_, verbIndex, _ := lo.FindIndexOf(permissions[resource], func(item verb) bool {
		return lo.Contains(verbs, item.Verb)
	})
	if verbIndex > -1 {
		for _, ff := range featureFlags {
			if !lo.ContainsBy(permissions[resource][verbIndex].FeatureFlags, func(item verbFeatureFlag) bool {
				return item.Name == ff.Name
			}) {
				permissions[resource][verbIndex].FeatureFlags = append(permissions[resource][verbIndex].FeatureFlags, ff)
			}
		}

		return
	}

	for _, v := range verbs {
		permissions[resource] = append(permissions[resource], verb{
			Verb:         v,
			FeatureFlags: featureFlags,
		})
	}
}

func parseComment(comment *ast.Comment) (string, []string, []verbFeatureFlag) {
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

	featureFlags := lo.Map(ffs, func(ff string, _ int) verbFeatureFlag {
		split := strings.Split(ff, "=")
		return verbFeatureFlag{
			Name:  strings.TrimSpace(split[0]),
			Value: strings.TrimSpace(split[1]),
		}
	})

	return resource, verbs, featureFlags
}

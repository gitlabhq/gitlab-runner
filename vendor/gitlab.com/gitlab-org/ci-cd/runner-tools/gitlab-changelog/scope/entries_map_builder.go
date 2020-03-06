package scope

import (
	"fmt"
	"sort"
	"strings"

	"github.com/juliangruber/go-intersect"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/config"
)

type Object interface {
	IncludeAuthor()
	Labels() config.Labels
	Entry() string
}

type EntriesMapBuilder interface {
	Build(objects []Object) (EntriesMap, error)
}

func NewEntriesMapBuilder(configuration config.Configuration) EntriesMapBuilder {
	return NewEntriesMapBuilderWithFactory(configuration, NewEntriesMap)
}

func NewEntriesMapBuilderWithFactory(configuration config.Configuration, factory EntriesMapFactory) EntriesMapBuilder {
	b := &defaultEntriesMapBuilder{
		configuration:     configuration,
		entriesMapFactory: factory,
	}
	b.sortMatchers()

	return b
}

type defaultEntriesMapBuilder struct {
	entriesMap    EntriesMap
	configuration config.Configuration

	entriesMapFactory EntriesMapFactory
}

func (b *defaultEntriesMapBuilder) sortMatchers() {
	sort.Strings(b.configuration.AuthorshipLabels)

	for _, matcher := range b.configuration.LabelMatchers {
		sort.Strings(matcher.Labels)
	}
}

func (b *defaultEntriesMapBuilder) Build(objects []Object) (EntriesMap, error) {
	err := b.prepareMap()
	if err != nil {
		return nil, fmt.Errorf("error while preparing entries map struct: %w", err)
	}

	for _, object := range objects {
		err = b.mapObject(object)
		if err != nil {
			return nil, fmt.Errorf("error while maping object: %w", err)
		}
	}

	return b.entriesMap, nil
}

func (b *defaultEntriesMapBuilder) prepareMap() error {
	entries, err := b.entriesMapFactory(b.configuration.Order, b.configuration.Names)
	if err != nil {
		return err
	}

	b.entriesMap = entries

	return nil
}

func (b *defaultEntriesMapBuilder) mapObject(object Object) error {
	objectLabels := object.Labels()
	sort.Strings(objectLabels)

	b.mapObjectAuthor(object, objectLabels)
	chosenScope := b.mapObjectScope(objectLabels)

	return b.entriesMap.Add(chosenScope, object.Entry())
}

func (b *defaultEntriesMapBuilder) mapObjectAuthor(object Object, objectLabels []string) {
	if len(b.configuration.AuthorshipLabels) < 1 {
		return
	}

	intersection := intersect.Hash(b.configuration.AuthorshipLabels, objectLabels)
	if len(intersection) > 0 {
		object.IncludeAuthor()
	}
}

func (b *defaultEntriesMapBuilder) mapObjectScope(objectLabels []string) config.Scope {
	for _, matcher := range b.configuration.LabelMatchers {
		if isSliceEqual(matcher.Labels, objectLabels) {
			return matcher.Scope
		}
	}

	return b.configuration.DefaultScope
}

func isSliceEqual(compareSource []string, target []string) bool {
	rawIntersection := intersect.Hash(compareSource, target)

	intersection := make([]string, 0)
	for _, raw := range rawIntersection {
		intersection = append(intersection, raw.(string))
	}
	sort.Strings(intersection)

	return strings.Join(compareSource, ":") == strings.Join(intersection, ":")
}

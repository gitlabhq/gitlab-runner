package scope

import (
	"fmt"

	config "gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/config"
)

type ErrUnknownScope struct {
	scope config.Scope
}

func (e *ErrUnknownScope) Error() string {
	return fmt.Sprintf("unknown scope %q", e.scope)
}

func (e *ErrUnknownScope) Is(err error) bool {
	_, ok := err.(*ErrUnknownScope)

	return ok
}

type ErrMissingScopeName struct {
	scope config.Scope
}

func (e *ErrMissingScopeName) Error() string {
	return fmt.Sprintf("missing name for scope %q", e.scope)
}

func (e *ErrMissingScopeName) Is(err error) bool {
	_, ok := err.(*ErrMissingScopeName)

	return ok
}

type EntriesMapFactory func(order config.Order, names config.Names) (EntriesMap, error)

type EntriesMap interface {
	Add(scope config.Scope, entry string) error
	ForEach(fn func(entries Entries) error) error
}

type Entries struct {
	ScopeName string
	Entries   []string
}

func NewEntriesMap(order config.Order, names config.Names) (EntriesMap, error) {
	em := &defaultEntriesMap{
		order: order,
		inner: make(map[config.Scope]Entries),
	}

	for _, scope := range order {
		scopeName, ok := names[scope]
		if !ok {
			return nil, &ErrMissingScopeName{scope: scope}
		}

		em.inner[scope] = Entries{
			ScopeName: scopeName,
			Entries:   make([]string, 0),
		}
	}

	return em, nil
}

type defaultEntriesMap struct {
	order config.Order
	inner map[config.Scope]Entries
}

func (m *defaultEntriesMap) Add(scope config.Scope, entry string) error {
	_, ok := m.inner[scope]
	if !ok {
		return &ErrUnknownScope{scope: scope}
	}

	e := m.inner[scope]
	e.Entries = append(e.Entries, entry)
	m.inner[scope] = e

	return nil
}

func (m *defaultEntriesMap) ForEach(fn func(entries Entries) error) error {
	for _, scope := range m.order {
		entries := m.inner[scope]
		if len(entries.Entries) < 1 {
			continue
		}

		err := fn(entries)
		if err != nil {
			return err
		}
	}

	return nil
}

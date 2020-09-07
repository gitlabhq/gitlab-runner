package registry

import (
	"fmt"
)

type FactoryAlreadyRegisteredError struct {
	factoryType string
	factoryName string
}

func NewFactoryAlreadyRegisteredError(factoryType string, factoryName string) *FactoryAlreadyRegisteredError {
	return &FactoryAlreadyRegisteredError{
		factoryType: factoryType,
		factoryName: factoryName,
	}
}

func (e *FactoryAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("factory for %s %q already registered", e.factoryType, e.factoryName)
}

func (e *FactoryAlreadyRegisteredError) Is(err error) bool {
	eerr, ok := err.(*FactoryAlreadyRegisteredError)
	if !ok {
		return false
	}

	return eerr.factoryName == e.factoryName
}

type FactoryNotRegisteredError struct {
	factoryType string
	factoryName string
}

func NewFactoryNotRegisteredError(factoryType string, factoryName string) *FactoryNotRegisteredError {
	return &FactoryNotRegisteredError{
		factoryType: factoryType,
		factoryName: factoryName,
	}
}

func (e *FactoryNotRegisteredError) Error() string {
	return fmt.Sprintf("factory for %s %q is not registered", e.factoryType, e.factoryName)
}

func (e *FactoryNotRegisteredError) Is(err error) bool {
	eerr, ok := err.(*FactoryNotRegisteredError)
	if !ok {
		return false
	}

	return eerr.factoryName == e.factoryName
}

type Registry interface {
	Register(factoryName string, factory interface{}) error
	Get(factoryName string) (interface{}, error)
}

type factoryRegistry struct {
	factoryType string
	store       map[string]interface{}
}

func (r factoryRegistry) Register(factoryName string, factory interface{}) error {
	_, ok := r.store[factoryName]
	if ok {
		return NewFactoryAlreadyRegisteredError(r.factoryType, factoryName)
	}

	r.store[factoryName] = factory

	return nil
}

func (r factoryRegistry) Get(factoryName string) (interface{}, error) {
	factory, ok := r.store[factoryName]
	if !ok {
		return nil, NewFactoryNotRegisteredError(r.factoryType, factoryName)
	}

	return factory, nil
}

func New(factoryType string) Registry {
	return &factoryRegistry{
		factoryType: factoryType,
		store:       make(map[string]interface{}),
	}
}

package auth_methods

import (
	"fmt"
)

type MissingRequiredConfigurationKeyError struct {
	key string
}

func NewMissingRequiredConfigurationKeyError(key string) *MissingRequiredConfigurationKeyError {
	return &MissingRequiredConfigurationKeyError{
		key: key,
	}
}

func (e *MissingRequiredConfigurationKeyError) Error() string {
	return fmt.Sprintf("missing required auth method configuration key %q", e.key)
}

func (e *MissingRequiredConfigurationKeyError) Is(err error) bool {
	eerr, ok := err.(*MissingRequiredConfigurationKeyError)
	if !ok {
		return false
	}

	return eerr.key == e.key
}

type Data map[string]interface{}

func (d Data) Filter(requiredFields []string, allowedFields []string) (Data, error) {
	for _, required := range requiredFields {
		_, ok := d[required]
		if !ok {
			return nil, NewMissingRequiredConfigurationKeyError(required)
		}
	}

	newData := make(Data)
	for _, allowed := range allowedFields {
		value, ok := d[allowed]
		if !ok {
			continue
		}
		newData[allowed] = value
	}

	return newData, nil
}

package autoscaler

import (
	"fmt"
	"net/url"
)

const acqKeyField = "acquisition-key"

type envKeyFields struct {
	acqKey string
}

func (a envKeyFields) toFields() url.Values {
	return url.Values{acqKeyField: []string{a.acqKey}}
}

// parseEnvKeyFields returns the autoscaler-owned fields
// and a fresh copy of the executor-owned remainder.
func parseEnvKeyFields(fields url.Values) (envKeyFields, url.Values, error) {
	ak := fields.Get(acqKeyField)
	if ak == "" {
		return envKeyFields{}, nil, fmt.Errorf("%s is required", acqKeyField)
	}
	executor := url.Values{}
	for k, v := range fields {
		if k == acqKeyField {
			continue
		}
		executor[k] = v
	}
	return envKeyFields{acqKey: ak}, executor, nil
}

package docker

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	envKeyBuildContainerIDField = "build-container-id"
	envKeyHelperIDField         = "helper-id"
	envKeyServiceIDsField       = "service-ids"
)

type envKeyFields struct {
	buildContainerID    string
	helperContainerID   string
	serviceContainerIDs []string
}

func (k envKeyFields) toValues() url.Values {
	v := url.Values{
		envKeyBuildContainerIDField: []string{k.buildContainerID},
		envKeyHelperIDField:         []string{k.helperContainerID},
	}
	if len(k.serviceContainerIDs) > 0 {
		v[envKeyServiceIDsField] = []string{strings.Join(k.serviceContainerIDs, ",")}
	}
	return v
}

func parseEnvKeyFields(fields url.Values) (envKeyFields, error) {
	k := envKeyFields{
		buildContainerID:  fields.Get(envKeyBuildContainerIDField),
		helperContainerID: fields.Get(envKeyHelperIDField),
	}
	if k.buildContainerID == "" {
		return envKeyFields{}, fmt.Errorf("%s is required", envKeyBuildContainerIDField)
	}
	if k.helperContainerID == "" {
		return envKeyFields{}, fmt.Errorf("%s is required", envKeyHelperIDField)
	}
	if raw := fields.Get(envKeyServiceIDsField); raw != "" {
		k.serviceContainerIDs = strings.Split(raw, ",")
		for _, id := range k.serviceContainerIDs {
			if id == "" {
				return envKeyFields{}, fmt.Errorf("%s contains an empty ID", envKeyServiceIDsField)
			}
		}
	}
	return k, nil
}

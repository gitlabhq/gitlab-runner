package configfile

import (
	"bytes"
	"encoding/json"

	jsonschema_generator "github.com/invopop/jsonschema"
	jsonschema_validator "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var configSchema *jsonschema_validator.Schema

func init() {
	defer func() {
		if r := recover(); r != nil {
			// Config validation is best-effort
			logrus.Warningf("Something went wrong creating config schema: %v", r)
		}
	}()

	r := &jsonschema_generator.Reflector{
		RequiredFromJSONSchemaTags: true,
		DoNotReference:             true,
	}
	schema, err := json.Marshal(r.Reflect(&common.Config{}))
	if err != nil {
		panic(err)
	}
	doc, err := jsonschema_validator.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		panic(err)
	}
	c := jsonschema_validator.NewCompiler()
	if err := c.AddResource("config_schema.json", doc); err != nil {
		panic(err)
	}
	configSchema = c.MustCompile("config_schema.json")
}

func validate(config *common.Config) error {
	defer func() {
		if r := recover(); r != nil {
			// Config validation is best-effort
			logrus.Warningf("Something went wrong validating config: %v", r)
		}
	}()

	// Validation must be done on generic types so we re-unmarshal the config into a JSON value
	configString, err := json.Marshal(config)
	if err != nil {
		panic(err)
	}
	jsonValue, err := jsonschema_validator.UnmarshalJSON(bytes.NewReader(configString))
	if err != nil {
		panic(err)
	}

	return configSchema.Validate(jsonValue)
}

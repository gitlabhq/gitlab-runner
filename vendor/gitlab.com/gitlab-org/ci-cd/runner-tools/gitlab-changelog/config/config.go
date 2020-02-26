package config

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type YamlErrorWrapper struct {
	inner error
}

func (e *YamlErrorWrapper) Error() string {
	return e.inner.Error()
}

func (e *YamlErrorWrapper) Is(err error) bool {
	_, ok := err.(*YamlErrorWrapper)

	return ok
}

type Names map[Scope]string

type Order []Scope

type LabelMatchers []LabelMatcher

type LabelMatcher struct {
	Labels Labels `yaml:"labels"`
	Scope  Scope  `yaml:"scope"`
}

type Labels = []string

type Scope string

type Configuration struct {
	DefaultScope     Scope         `yaml:"default_scope"`
	Names            Names         `yaml:"names"`
	Order            Order         `yaml:"order"`
	LabelMatchers    LabelMatchers `yaml:"label_matchers"`
	AuthorshipLabels Labels        `yaml:"authorship_labels"`
}

func LoadConfig(configFile string) (Configuration, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return Configuration{}, fmt.Errorf("error while reading configuration file %q: %w", configFile, err)
	}

	var config Configuration
	decoder := yaml.NewDecoder(bytes.NewBuffer(data))
	err = decoder.Decode(&config)
	if err != nil {
		return Configuration{}, fmt.Errorf("error while decoding configuration file %q: %w", configFile, &YamlErrorWrapper{inner: err})
	}

	return config, nil
}

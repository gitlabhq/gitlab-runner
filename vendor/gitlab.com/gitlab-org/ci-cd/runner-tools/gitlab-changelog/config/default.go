package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v2"
)

const (
	defaultNewFeature             Scope = "new-feature"
	defaultFix                    Scope = "fix"
	defaultSecurityFix            Scope = "security-fix"
	defaultTechnicalDeptReduction Scope = "technical-dept-reduction"
	defaultDocumentation          Scope = "documentation"
	defaultOther                  Scope = "other"
)

var (
	defaultNames = Names{
		defaultNewFeature:             "New features",
		defaultFix:                    "Bug fixes",
		defaultSecurityFix:            "Security fixes",
		defaultTechnicalDeptReduction: "Technical dept reduction",
		defaultDocumentation:          "Documentation changes",
		defaultOther:                  "Other changes",
	}

	defaultOrder = Order{
		defaultNewFeature,
		defaultFix,
		defaultSecurityFix,
		defaultTechnicalDeptReduction,
		defaultDocumentation,
		defaultOther,
	}

	defaultLabelMatchers = LabelMatchers{
		{
			Labels: Labels{"feature"},
			Scope:  defaultNewFeature,
		},
		{
			Labels: Labels{"security"},
			Scope:  defaultSecurityFix,
		},
		{
			Labels: Labels{"bug"},
			Scope:  defaultFix,
		},
		{
			Labels: Labels{"backstage"},
			Scope:  defaultTechnicalDeptReduction,
		},
		{
			Labels: Labels{"documentation"},
			Scope:  defaultDocumentation,
		},
	}

	defaultAuthorshipLabels = Labels{"Community contribution"}

	defaultConfig = Configuration{
		DefaultScope:     defaultOther,
		Names:            defaultNames,
		Order:            defaultOrder,
		LabelMatchers:    defaultLabelMatchers,
		AuthorshipLabels: defaultAuthorshipLabels,
	}
)

func DefaultConfig() Configuration {
	return defaultConfig
}

func DumpDefaultConfig() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := yaml.NewEncoder(buf)

	err := encoder.Encode(DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("error while encoding default configuration to YAML: %w", err)
	}

	return buf.Bytes(), nil
}

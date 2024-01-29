package build

import (
	"encoding/json"
	"strconv"
)

const (
	TypeDockerImage        = "Docker image"
	TypeFile               = "File"
	TypeDockerImageArchive = "Docker image archive"
	TypeOSBinary           = "OS binary"
	TypeMacOSPackage       = "macOS package"
)

type Component interface {
	json.Marshaler
	json.Unmarshaler

	Value() string
	Type() string
	Description() string
	Required() bool

	WithDescription(string) Component
	WithRequired() Component
}

type component struct {
	value       string
	typ         string
	description string
	required    bool
}

func (c *component) Value() string {
	return c.value
}

func (c *component) Type() string {
	return c.typ
}

func (c *component) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"Value": c.value,
		"Type":  c.typ,
	})
}

func (c *component) UnmarshalJSON(b []byte) error {
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	c.value = m["Value"]
	c.typ = m["Type"]
	c.description = m["Description"]
	c.required, _ = strconv.ParseBool(m["Required"])

	return nil
}

func (c *component) Description() string {
	return c.description
}

func (c *component) Required() bool {
	return c.required
}

func (c *component) WithDescription(description string) Component {
	c.description = description
	return c
}

func (c *component) WithRequired() Component {
	c.required = true
	return c
}

func NewComponent(value, typ, description string, required bool) Component {
	var comp Component
	switch typ {
	case TypeDockerImageArchive:
		comp = NewDockerImageArchive(value)
	case TypeDockerImage:
		comp = NewDockerImage(value)
	case TypeFile:
		comp = NewFile(value)
	case TypeOSBinary:
		comp = NewOSBinary(value)
	case TypeMacOSPackage:
		comp = NewMacOSPackage(value)
	default:
		panic("Invalid component type " + typ)
	}

	comp = comp.WithDescription(description)
	if required {
		comp = comp.WithRequired()
	}

	return comp
}

func NewDockerImage(value string) Component {
	return &component{
		value: value,
		typ:   TypeDockerImage,
	}
}

func NewDockerImageArchive(value string) Component {
	return &component{
		value: value,
		typ:   TypeDockerImageArchive,
	}
}

func NewFile(value string) Component {
	return &component{
		value: value,
		typ:   TypeFile,
	}
}

func NewOSBinary(value string) Component {
	return &component{
		value: value,
		typ:   TypeOSBinary,
	}
}

func NewMacOSPackage(value string) Component {
	return &component{
		value: value,
		typ:   TypeMacOSPackage,
	}
}

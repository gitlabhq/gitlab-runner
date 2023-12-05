package build

import (
	"encoding/json"
)

const (
	typeDockerImage        = "Docker image"
	typeFile               = "File"
	typeDockerImageArchive = "Docker image archive"
)

type Component interface {
	json.Marshaler
	json.Unmarshaler

	Value() string
	Type() string
}

type component struct {
	value string
	typ   string
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

	return nil
}

func NewComponent(value, typ string) Component {
	switch typ {
	case typeDockerImageArchive:
		return NewDockerImageArchive(value)
	case typeDockerImage:
		return NewDockerImage(value)
	case typeFile:
		return NewFile(value)
	}

	panic("Invalid component type " + typ)
}

func NewDockerImage(value string) Component {
	return &component{
		value: value,
		typ:   typeDockerImage,
	}
}

func NewDockerImageArchive(value string) Component {
	return &component{
		value: value,
		typ:   typeDockerImageArchive,
	}
}

func NewFile(value string) Component {
	return &component{
		value: value,
		typ:   typeFile,
	}
}

package helpers

import (
	"bufio"
	"bytes"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v2"
)

func ToYAML(src interface{}) string {
	data, err := yaml.Marshal(src)
	if err == nil {
		return string(data)
	}
	return ""
}

func ToTOML(src interface{}) string {
	var data bytes.Buffer
	buffer := bufio.NewWriter(&data)

	if err := toml.NewEncoder(buffer).Encode(src); err != nil {
		return ""
	}

	if err := buffer.Flush(); err != nil {
		return ""
	}

	return data.String()
}

func ToConfigMap(list interface{}) (map[string]interface{}, bool) {
	x, ok := list.(map[string]interface{})
	if ok {
		return x, ok
	}

	y, ok := list.(map[interface{}]interface{})
	if !ok {
		return nil, false
	}

	result := make(map[string]interface{})
	for k, v := range y {
		result[k.(string)] = v
	}

	return result, true
}

func GetMapKey(value map[string]interface{}, keys ...string) (result interface{}, ok bool) {
	result = value

	for _, key := range keys {
		switch t := result.(type) {
		case map[string]interface{}:
			if result, ok = t[key]; ok {
				continue
			}
		case map[interface{}]interface{}:
			if result, ok = t[key]; ok {
				continue
			}
		}
		return nil, false
	}

	return result, true
}

package vault

type SecretEngine interface {
	EngineName() string
	Get(path string) (map[string]interface{}, error)
	Put(path string, data map[string]interface{}) error
	Delete(path string) error
}

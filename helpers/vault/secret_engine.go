package vault

type SecretEngine interface {
	EngineName() string
	Get(path string) (map[string]any, error)
	Put(path string, data map[string]any) error
	Delete(path string) error
}

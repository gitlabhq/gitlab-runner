package vault

type AuthMethod interface {
	Name() string
	Authenticate(client Client) error
	Token() string
}

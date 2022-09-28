package vault

//go:generate mockery --name=AuthMethod --inpackage
type AuthMethod interface {
	Name() string
	Authenticate(client Client) error
	Token() string
}

package ssh

//nolint:lll
type Config struct {
	User                         string `toml:"user,omitempty" json:"user" long:"user" env:"SSH_USER" description:"User name"`
	Password                     string `toml:"password,omitempty" json:"password" long:"password" env:"SSH_PASSWORD" description:"User password"`
	Host                         string `toml:"host,omitempty" json:"host" long:"host" env:"SSH_HOST" description:"Remote host"`
	Port                         string `toml:"port,omitempty" json:"port" long:"port" env:"SSH_PORT" description:"Remote host port"`
	IdentityFile                 string `toml:"identity_file,omitempty" json:"identity_file" long:"identity-file" env:"SSH_IDENTITY_FILE" description:"Identity file to be used"`
	DisableStrictHostKeyChecking *bool  `toml:"disable_strict_host_key_checking,omitempty" json:"disable_strict_host_key_checking" long:"disable-strict-host-key-checking" env:"DISABLE_STRICT_HOST_KEY_CHECKING" description:"Disable SSH strict host key checking"`
	KnownHostsFile               string `toml:"known_hosts_file,omitempty" json:"known_hosts_file" long:"known-hosts-file" env:"KNOWN_HOSTS_FILE" description:"Location of known_hosts file. Defaults to ~/.ssh/known_hosts"`
}

func (c *Config) ShouldDisableStrictHostKeyChecking() bool {
	return c.DisableStrictHostKeyChecking != nil && *c.DisableStrictHostKeyChecking
}

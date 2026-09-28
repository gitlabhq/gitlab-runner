package docker

import (
	"os"
	"strconv"
)

type Credentials struct {
	Host      string `toml:"host,omitempty" json:"host" long:"host" env:"DOCKER_HOST" description:"Docker daemon address"`
	CertPath  string `toml:"tls_cert_path,omitempty" json:"tls_cert_path" long:"cert-path" env:"DOCKER_CERT_PATH" description:"Certificate path"`
	TLSVerify bool   `toml:"tls_verify,omitzero" json:"tls_verify" long:"tlsverify" env:"DOCKER_TLS_VERIFY" description:"Use TLS and verify the remote"`

	// TLSServerName is the name the server certificate is verified against
	// when it is not the address in Host. Set for docker+machine machines
	// whose certificate is issued for the machine name.
	TLSServerName string `toml:"-" json:"-"`
}

func credentialsFromEnv() Credentials {
	tlsVerify, _ := strconv.ParseBool(os.Getenv("DOCKER_TLS_VERIFY"))
	return Credentials{
		Host:      os.Getenv("DOCKER_HOST"),
		CertPath:  os.Getenv("DOCKER_CERT_PATH"),
		TLSVerify: tlsVerify,
	}
}

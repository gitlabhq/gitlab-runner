package docker_helpers

import (
	"os"
	"strconv"
)

type DockerCredentials struct {
	Host      string `toml:"host,omitempty" json:"host" long:"host" env:"DOCKER_HOST" description:"Docker daemon address"`
	CertPath  string `toml:"tls_cert_path,omitempty" json:"tls_cert_path" long:"cert-path" env:"DOCKER_CERT_PATH" description:"Certificate path"`
	TLSVerify bool   `toml:"tls_verify,omitzero" json:"tls_verify" long:"tlsverify" env:"DOCKER_TLS_VERIFY" description:"Use TLS and verify the remote"`
}

func credentialsFromEnv() DockerCredentials {
	tlsVerify, _ := strconv.ParseBool(os.Getenv("DOCKER_TLS_VERIFY"))
	return DockerCredentials{
		Host:      os.Getenv("DOCKER_HOST"),
		CertPath:  os.Getenv("DOCKER_CERT_PATH"),
		TLSVerify: tlsVerify,
	}
}

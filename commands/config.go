package commands

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func GetDefaultConfigFile() string {
	return filepath.Join(getDefaultConfigDirectory(), "config.toml")
}

func GetDefaultCertificateDirectory() string {
	return filepath.Join(getDefaultConfigDirectory(), "certs")
}

func init() {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		err := os.Setenv("CONFIG_FILE", GetDefaultConfigFile())
		if err != nil {
			logrus.WithError(err).Fatal("Couldn't set CONFIG_FILE environment variable")
		}
	}
}

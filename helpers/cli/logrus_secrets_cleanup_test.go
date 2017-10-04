package cli_helpers

import (
	"bytes"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func runOnHijackedLogrusOutput(t *testing.T, handler func(t *testing.T, output *bytes.Buffer)) {
	oldOutput := logrus.StandardLogger().Out
	defer func() { logrus.StandardLogger().Out = oldOutput }()

	buf := bytes.NewBuffer([]byte{})
	logrus.StandardLogger().Out = buf

	oldHooks := logrus.LevelHooks{}
	for level, hooks := range logrus.StandardLogger().Hooks {
		oldHooks[level] = hooks
	}
	defer func() { logrus.StandardLogger().Hooks = oldHooks }()

	AddSecretsCleanupLogHook()

	handler(t, buf)
}

func TestSecretsCleanupHookWithoutSecrets(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		logrus.Errorln("Fatal: Get http://localhost/?id=123")
		assert.Contains(t, output.String(), `Get http://localhost/?id=123`)
	})
}

func TestSecretsCleanupHookWithSecrets(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		logrus.Errorln("Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234")
		assert.Contains(t, output.String(), `Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]`)
	})
}

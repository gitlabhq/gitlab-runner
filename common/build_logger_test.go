package common_test

import (
	"bytes"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	test "gitlab.com/gitlab-org/gitlab-runner/common/test"
)

func newBuildLogger(testName string, jt common.JobTrace) common.BuildLogger {
	return common.NewBuildLogger(jt, logrus.WithField("test", testName))
}

func runOnHijackedLogrusOutput(t *testing.T, handler func(t *testing.T, output *bytes.Buffer)) {
	oldOutput := logrus.StandardLogger().Out
	defer func() { logrus.StandardLogger().Out = oldOutput }()

	buf := bytes.NewBuffer([]byte{})
	logrus.StandardLogger().Out = buf

	handler(t, buf)
}

func TestLogLineWithoutSecret(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		jt := test.NewStubJobTrace()
		l := newBuildLogger("log-line-without-secret", jt)

		l.Errorln("Fatal: Get http://localhost/?id=123")
		assert.Contains(t, jt.Read(), `Get http://localhost/?id=123`)
		assert.Contains(t, output.String(), `Get http://localhost/?id=123`)
	})
}

func TestLogLineWithSecret(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		jt := test.NewStubJobTrace()
		l := newBuildLogger("log-line-with-secret", jt)

		l.Errorln("Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234")
		assert.Contains(t, jt.Read(), `Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]`)
		assert.Contains(t, output.String(), `Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234`)
	})
}

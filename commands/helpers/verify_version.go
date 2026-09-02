package helpers

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

// VerifyVersionCommand checks that this binary's version matches an expected
// one. The boot-verify canary runs it in the helper environment with the
// manager's version, catching helper images pinned to a different release.
// The comparison happens in the binary so it works identically on every OS
// and shell.
type VerifyVersionCommand struct {
	Expected string `long:"expected" description:"Version this binary must match"`
}

func NewVerifyVersionCommand() cli.Command {
	return common.NewCommand("verify-version", "check that this binary matches an expected version (internal)", &VerifyVersionCommand{})
}

func (c *VerifyVersionCommand) verify() error {
	if c.Expected == "" {
		return errors.New("missing --expected")
	}

	if common.AppVersion.Version != c.Expected {
		return fmt.Errorf("helper version %q (revision %s) does not match runner version %q",
			common.AppVersion.Version, common.AppVersion.Revision, c.Expected)
	}

	fmt.Printf("helper version %s (revision %s) matches the runner\n",
		common.AppVersion.Version, common.AppVersion.Revision)
	return nil
}

func (c *VerifyVersionCommand) Execute(*cli.Context) {
	log.SetRunnerFormatter()

	if err := c.verify(); err != nil {
		logrus.Fatalln(err)
	}
}

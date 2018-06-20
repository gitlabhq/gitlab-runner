package cli_helpers

import (
	"strings"

	"github.com/sirupsen/logrus"
)

// WarnOnBool logs warning if args contains true or false
// github.com/urfave/cli breaks badly if boolean are set using --flag true instead of --flag=true or just --flag
// this is a simple check that warn the user about this if detects "true" or "false" alone in the arguments
func WarnOnBool(args []string) {
	// we skip the first element because it contains the program name
	for idx, a := range args[1:] {
		arg := strings.ToLower(a)
		if arg == "true" || arg == "false" {
			supposedFlag := "--key"
			if idx > 0 {
				supposedFlag = args[idx]
			}

			logrus.Warningf("boolean parameters must be passed in the command line with %s=%s", supposedFlag, arg)
			logrus.Warningln("parameters after this may be ignored")
			break
		}
	}
}

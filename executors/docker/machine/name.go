package machine

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
)

func machineFormat(runner string, template string) string {
	if runner != "" {
		return "runner-" + strings.ToLower(runner) + "-" + template
	}
	return template
}

func machineFilter(config *common.RunnerConfig) string {
	return machineFormat(dns.MakeRFC1123Compatible(config.ShortDescription()), config.Machine.MachineName)
}

func matchesMachineFilter(name, filter string) bool {
	var query string
	if n, _ := fmt.Sscanf(name, filter, &query); n == 1 {
		return true
	}
	return false
}

func filterMachineList(machines []string, filter string) (newMachines []string) {
	newMachines = make([]string, 0, len(machines))
	for _, machine := range machines {
		if matchesMachineFilter(machine, filter) {
			newMachines = append(newMachines, machine)
		}
	}
	return
}

func newMachineName(config *common.RunnerConfig) string {
	r := make([]byte, 4)
	_, _ = rand.Read(r)
	t := time.Now().Unix()
	return fmt.Sprintf(machineFilter(config), fmt.Sprintf("%d-%x", t, r))
}

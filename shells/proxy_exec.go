package shells

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func WrapShell(shell common.Shell) common.Shell {
	return &ProxyExecShell{shell}
}

type ProxyExecShell struct {
	common.Shell
}

func (s *ProxyExecShell) GetEntrypointCommand(info common.ShellScriptInfo, probeFile string) []string {
	entrypoint := s.Shell.GetEntrypointCommand(info, probeFile)
	if len(entrypoint) == 0 || info.Build == nil || !info.Build.Runner.IsProxyExec() {
		return entrypoint
	}

	return append([]string{info.Build.TmpProjectDir() + "/gitlab-runner-helper", "proxy-exec"}, entrypoint...)
}

func (s *ProxyExecShell) GetConfiguration(info common.ShellScriptInfo) (*common.ShellConfiguration, error) {
	base, err := s.Shell.GetConfiguration(info)
	if err != nil || info.Build == nil || !info.Build.Runner.IsProxyExec() {
		return base, err
	}

	tempDir := fmt.Sprintf("%q", info.Build.TmpProjectDir())

	return &common.ShellConfiguration{
		Command:       info.RunnerCommand,
		Arguments:     append([]string{"proxy-exec", "--temp-dir", info.Build.TmpProjectDir(), base.Command}, base.Arguments...),
		CmdLine:       info.RunnerCommand + " proxy-exec --temp-dir " + tempDir + " " + base.CmdLine,
		DockerCommand: append([]string{info.Build.TmpProjectDir() + "/gitlab-runner-helper", "proxy-exec"}, base.DockerCommand...),
		PassFile:      base.PassFile,
		Extension:     base.Extension,
	}, nil
}

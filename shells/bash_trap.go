package shells

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

type BashTrapShell struct {
	*BashShell
}

func (b *BashTrapShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	w := &BashWriter{
		TemporaryPath:      info.Build.TmpProjectDir(),
		Shell:              b.Shell,
		checkForErrors:     info.Build.IsFeatureFlagOn(featureflags.EnableBashExitCodeCheck),
		useNewEval:         info.Build.IsFeatureFlagOn(featureflags.UseNewEvalStrategy),
		useNewEscape:       info.Build.IsFeatureFlagOn(featureflags.UseNewShellEscape),
		usePosixEscape:     info.Build.IsFeatureFlagOn(featureflags.PosixlyCorrectEscapes),
		useJSONTermination: true,
	}

	return b.generateScript(w, buildStage, info)
}

package common

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type ShellConfiguration struct {
	Command   string
	Arguments []string

	CmdLine string // combination of shell escaped command + args

	DockerCommand []string
	PassFile      bool
	Extension     string
}

type ShellType int

const (
	NormalShell ShellType = iota
	LoginShell
)

func (s *ShellConfiguration) String() string {
	return helpers.ToYAML(s)
}

type ShellScriptInfo struct {
	Shell                string
	Build                *Build
	Type                 ShellType
	User                 string
	RunnerCommand        string
	PreGetSourcesScript  string
	PostGetSourcesScript string
	PreBuildScript       string
	PostBuildScript      string
}

//go:generate mockery --name=Shell --inpackage
type Shell interface {
	GetName() string
	GetFeatures(features *FeaturesInfo)
	IsDefault() bool

	GetConfiguration(info ShellScriptInfo) (*ShellConfiguration, error)
	GenerateScript(buildStage BuildStage, info ShellScriptInfo) (string, error)
	GenerateSaveScript(info ShellScriptInfo, scriptPath, script string) (string, error)
}

var shells map[string]Shell

func RegisterShell(shell Shell) {
	logrus.Debugln("Registering", shell.GetName(), "shell...")

	if shells == nil {
		shells = make(map[string]Shell)
	}
	if shells[shell.GetName()] != nil {
		panic("Shell already exist: " + shell.GetName())
	}
	shells[shell.GetName()] = shell
}

func GetShell(shell string) Shell {
	if shells == nil {
		return nil
	}

	return shells[shell]
}

func GetShellConfiguration(info ShellScriptInfo) (*ShellConfiguration, error) {
	shell := GetShell(info.Shell)
	if shell == nil {
		return nil, fmt.Errorf("shell %s not found", info.Shell)
	}

	return shell.GetConfiguration(info)
}

func GenerateShellScript(buildStage BuildStage, info ShellScriptInfo) (string, error) {
	shell := GetShell(info.Shell)
	if shell == nil {
		return "", fmt.Errorf("shell %s not found", info.Shell)
	}

	return shell.GenerateScript(buildStage, info)
}

func GetDefaultShell() string {
	if shells == nil {
		panic("no shells defined")
	}

	for _, shell := range shells {
		if shell.IsDefault() {
			return shell.GetName()
		}
	}
	panic("no default shell defined")
}

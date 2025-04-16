package shells

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ShellWriter interface {
	EnvVariableKey(name string) string
	Variable(variable common.JobVariable)
	// Save variables in key=value format to a temporary file
	DotEnvVariables(baseFilename string, variables map[string]string) string
	SourceEnv(pathname string)
	Command(command string, arguments ...string)
	CommandWithStdin(stdin, command string, arguments ...string)
	CommandArgExpand(command string, arguments ...string)
	Line(text string)
	CheckForErrors()

	// SetupGitCredHelper sets up a credential helper in the confFile.
	// This helper pulls out the job token from the environment.
	// It disables any other helper, by replacing all with "", thus global or system helpers won't run.
	SetupGitCredHelper(confFile, section, user string)

	IfDirectory(path string)
	IfFile(file string)
	IfCmd(cmd string, arguments ...string)
	IfCmdWithOutput(cmd string, arguments ...string)
	IfGitVersionIsAtLeast(version string)
	Else()
	EndIf()

	Cd(path string)
	MkDir(path string)
	RmDir(path string)
	RmFile(path string)

	// RmFilesRecursive deletes all files in path with a basename of name
	// It does not delete directories with a basename of name
	RmFilesRecursive(path string, name string)
	// RmDirsRecursive deletes all directories and their content in path with a basename of name
	RmDirsRecursive(path string, name string)

	Absolute(path string) string
	Join(elem ...string) string
	TmpFile(name string) string

	MkTmpDir(name string) string

	Printf(fmt string, arguments ...interface{})
	Noticef(fmt string, arguments ...interface{})
	Warningf(fmt string, arguments ...interface{})
	Errorf(fmt string, arguments ...interface{})
	EmptyLine()

	SectionStart(id, command string, options []string)
	SectionEnd(id string)

	Finish(trace bool) string
}

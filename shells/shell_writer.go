package shells

import "gitlab.com/gitlab-org/gitlab-runner/common"

type ShellWriter interface {
	EnvVariableKey(name string) string
	Variable(variable common.JobVariable)
	Command(command string, arguments ...string)
	Line(text string)
	CheckForErrors()

	IfDirectory(path string)
	IfFile(file string)
	IfCmd(cmd string, arguments ...string)
	IfCmdWithOutput(cmd string, arguments ...string)
	Else()
	EndIf()

	Cd(path string)
	MkDir(path string)
	RmDir(path string)
	RmFile(path string)
	Absolute(path string) string
	TmpFile(name string) string

	MkTmpDir(name string) string

	Print(fmt string, arguments ...interface{})
	Notice(fmt string, arguments ...interface{})
	Warning(fmt string, arguments ...interface{})
	Error(fmt string, arguments ...interface{})
	EmptyLine()

	Finish(trace bool) string
}

package shells

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//go:generate mockery --name=ShellWriter --inpackage
type ShellWriter interface {
	EnvVariableKey(name string) string
	Variable(variable common.JobVariable)
	Command(command string, arguments ...string)
	CommandArgExpand(command string, arguments ...string)
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
	RmFilesRecursive(path string, name string)
	Absolute(path string) string
	Join(elem ...string) string
	TmpFile(name string) string

	MkTmpDir(name string) string

	Printf(fmt string, arguments ...interface{})
	Noticef(fmt string, arguments ...interface{})
	Warningf(fmt string, arguments ...interface{})
	Errorf(fmt string, arguments ...interface{})
	EmptyLine()

	SectionStart(id, command string)
	SectionEnd(id string)

	Finish(trace bool) string
}

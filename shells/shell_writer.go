package shells

import "gitlab.com/gitlab-org/gitlab-runner/common/spec"

type ShellWriter interface {
	EnvVariableKey(name string) string
	Variable(variable spec.Variable)
	// Save variables in key=value format to a temporary file
	DotEnvVariables(baseFilename string, variables map[string]string) string
	SourceEnv(pathname string)
	Command(command string, arguments ...string)
	// CommandWithStdin runs command with arguments and feeds stdin to its standard input.
	// A trailing newline is appended to stdin.
	// The child's exit code must propagate as the script's exit code on failure. When
	// bestEffort is set, neither a non-zero exit code nor a failure to even start the command
	// may fail the script, and no error check may be emitted after it.
	CommandWithStdin(bestEffort bool, stdin string, command string, arguments ...string)
	CommandArgExpand(command string, arguments ...string)
	Line(text string)
	CheckForErrors()

	// SetupGitCredHelper sets up a credential helper in the confFile.
	// This helper pulls out the job token from the environment.
	// It disables any other helper, by replacing all with "", thus global or system helpers won't run.
	SetupGitCredHelper(confFile, section, user string)

	// ExportRaw exports a variable name and sets the value to value. It does NOT escape the value, so it will be
	// expanded. The caller is responsible to ensure the value is safe.
	ExportRaw(name, value string)

	IfDirectory(path string)
	IfFile(file string)
	IfFileReadable(file string)
	IfCmd(cmd string, arguments ...string)
	IfCmdWithOutput(cmd string, arguments ...string)
	IfCmdWithOutputArgExpand(cmd string, arguments ...string)
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

	Printf(fmt string, arguments ...any)
	Noticef(fmt string, arguments ...any)
	Warningf(fmt string, arguments ...any)
	Errorf(fmt string, arguments ...any)
	EmptyLine()

	SectionStart(id, command string, options []string)
	SectionEnd(id string)

	Finish(trace bool) string
}

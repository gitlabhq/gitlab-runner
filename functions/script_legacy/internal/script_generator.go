package internal

import (
	"fmt"
	"strings"
)

// ScriptGeneratorConfig holds configuration options for script generation.
type ScriptGeneratorConfig struct {
	DebugTrace     bool
	CheckForErrors bool
	PosixEscape    bool
	TraceSections  bool
	ShellPath      string
	// GitLabEnvFile is the path to the GITLAB_ENV file. When set, the
	// generated script exports GITLAB_ENV and sources the file as a
	// preamble so that dynamic variables written by previous stages are
	// available in this stage.
	GitLabEnvFile string
}

// ScriptGenerator generates bash scripts from command arrays.
type ScriptGenerator struct {
	header        *ScriptHeader
	processor     *CommandProcessor
	gitLabEnvFile string
}

// NewScriptGenerator creates a new script generator with the given configuration.
func NewScriptGenerator(config ScriptGeneratorConfig) *ScriptGenerator {
	return &ScriptGenerator{
		header:        NewScriptHeader(config.ShellPath, config.DebugTrace),
		processor:     NewCommandProcessor(config),
		gitLabEnvFile: config.GitLabEnvFile,
	}
}

// GenerateScript creates a complete shell script with all commands.
// Commands are executed in a single shell session, preserving state.
// The shebang uses the detected shell path for deterministic execution.
func (g *ScriptGenerator) GenerateScript(commands []string) string {
	var buf strings.Builder

	buf.WriteString(g.header.Generate())

	if g.gitLabEnvFile != "" {
		// Export GITLAB_ENV so user commands can append KEY=VALUE pairs to it,
		// then source any variables written by previous stages. This mirrors
		// what AbstractShell.writeExports does for the legacy shell path.
		fmt.Fprintf(&buf, "export GITLAB_ENV=%q\n", g.gitLabEnvFile)
		fmt.Fprintf(
			&buf,
			"if [ -f %q ]; then while read -r line; do export \"$line\" || true; done < %q; fi\n\n",
			g.gitLabEnvFile,
			g.gitLabEnvFile,
		)
	}

	for i, cmd := range commands {
		g.processor.ProcessCommand(&buf, i, cmd)
	}

	return buf.String()
}

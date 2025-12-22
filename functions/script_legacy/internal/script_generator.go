package internal

import "strings"

// ScriptGeneratorConfig holds configuration options for script generation.
type ScriptGeneratorConfig struct {
	DebugTrace     bool
	CheckForErrors bool
	PosixEscape    bool
	TraceSections  bool
	ShellPath      string
}

// ScriptGenerator generates bash scripts from command arrays.
type ScriptGenerator struct {
	header    *ScriptHeader
	processor *CommandProcessor
}

// NewScriptGenerator creates a new script generator with the given configuration.
func NewScriptGenerator(config ScriptGeneratorConfig) *ScriptGenerator {
	return &ScriptGenerator{
		header:    NewScriptHeader(config.ShellPath, config.DebugTrace),
		processor: NewCommandProcessor(config),
	}
}

// GenerateScript creates a complete shell script with all commands.
// Commands are executed in a single shell session, preserving state.
// The shebang uses the detected shell path for deterministic execution.
func (g *ScriptGenerator) GenerateScript(commands []string) string {
	var buf strings.Builder

	buf.WriteString(g.header.Generate())

	for i, cmd := range commands {
		g.processor.ProcessCommand(&buf, i, cmd)
	}

	return buf.String()
}

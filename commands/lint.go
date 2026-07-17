package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// LintCommand validates a config.toml file for syntax errors, semantic
// (jsonschema) errors, and unknown/misspelled keys.
type LintCommand struct {
	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
}

// NewLintCommand creates the cli.Command for linting a config.toml file.
func NewLintCommand() cli.Command {
	return common.NewCommand("lint", "validate syntax and keys of a config.toml file", &LintCommand{
		ConfigFile: GetDefaultConfigFile(),
	})
}

// Execute runs the lint command.
func (c *LintCommand) Execute(_ *cli.Context) {
	if err := LintConfigFile(c.ConfigFile); err != nil {
		logrus.Fatalln(err)
	}
	logrus.Println("Configuration file is valid.")
}

// LintConfigFile validates the given config file by:
//  1. requiring the file to exist (lint deviates from Config.LoadConfig here,
//     which treats a missing file as a soft error - inappropriate for a
//     validator invoked explicitly by the user);
//  2. decoding and built-in struct validation via Config.LoadConfig, which
//     handles TOML syntax errors and structural Validate() checks without the
//     system-ID side-effect that configfile.Load() carries (configfile.Load()
//     calls newSystemIDState() and will create a .runner_system_id file next
//     to the config, which is inappropriate for a read-only validator);
//  3. jsonschema validation via configfile.Validate(), surfaced as a hard error
//     (configfile.Load() only logs this as a warning and exits 0);
//  4. re-decoding metadata only to surface undecoded (unknown / misspelled)
//     keys. Step 4 duplicates only the metadata extraction because
//     BurntSushi/toml exposes Undecoded() solely through the return value of
//     DecodeFile, which LoadConfig discards. Keeping the extra pass local to
//     lint is a smaller, safer change than threading metadata through the
//     shared loader.
func LintConfigFile(configFile string) error {
	if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("config file not found: %s", configFile)
	} else if err != nil {
		return fmt.Errorf("accessing config file: %w", err)
	}

	// Primary validation: TOML decode + built-in struct Validate().
	// LoadConfig treats a missing file as a soft error; we guard against that
	// above, so by this point the file is known to exist.
	cfg := common.NewConfig()
	if err := cfg.LoadConfig(configFile); err != nil {
		return fmt.Errorf("decoding config: %w", err)
	}

	// Jsonschema validation is best-effort in configfile.Load() (it only logs).
	// For lint we treat any schema violation as a hard error.
	if err := configfile.Validate(cfg); err != nil {
		return fmt.Errorf("config schema validation failed: %w", err)
	}

	// Diagnostic pass: capture undecoded (unknown / misspelled) keys.
	var rawCfg common.Config
	meta, err := toml.DecodeFile(configFile, &rawCfg)
	if err != nil {
		// The loader above already succeeded, so a syntax error here would be
		// unexpected. Surface it rather than silently swallow.
		return fmt.Errorf("decoding config: %w", err)
	}

	undecoded := meta.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}

	msgs := make([]string, 0, len(undecoded))
	for _, key := range undecoded {
		msgs = append(msgs, fmt.Sprintf("  unknown key: %s", key))
	}

	return fmt.Errorf(
		"%d unknown key(s) found in %s:\n%s",
		len(undecoded),
		configFile,
		strings.Join(msgs, "\n"),
	)
}

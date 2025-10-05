package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd sets up the base "confb" command tree.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confb",
  	Short: "confb (config blender) — stitch multiple config fragments into deterministic outputs",
  	Long: `confb watches or builds configuration outputs from one or more source files.

Supported formats:
  - KDL: merge selected sections, key policy (first_wins|last_wins|append)
  - YAML/JSON/TOML: maps (deep|replace), arrays (append|unique_append|replace)
  - INI: repeated_keys (append|last_wins)
  - RAW: newline-normalized concatenation

Typical workflow:
  1) put your rules in ~/.config/confb/confb.yaml
  2) confb build -c ~/.config/confb/confb.yaml
  3) confb run   -c ~/.config/confb/confb.yaml  (watch & rebuild)`,		
	}

	cmd.DisableAutoGenTag = true

	// global persistent flags (available to all subcommands)
	cmd.PersistentFlags().StringP("config", "c", "confb.yaml",
		"path to confb configuration file")
	cmd.PersistentFlags().StringP("chdir", "C", "",
		"change working directory before reading the config")

	// attach subcommands
	cmd.AddCommand(newBuildCmd())
  cmd.AddCommand(newRunCmd()) // register daemon
  cmd.AddCommand(newValidateCmd())
  cmd.AddCommand(generateManCmd(cmd))

	return cmd
}


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const defaultRelConfig = ".config/confb/confb.yaml"

// defaultConfigPath returns "$HOME/.config/confb/confb.yaml", or "confb.yaml" if $HOME is unknown.
func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "confb.yaml"
	}
	return filepath.Join(home, defaultRelConfig)
}

// expandPath expands "~" and environment variables in a path.
func expandPath(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return os.ExpandEnv(p)
}

// resolveConfig applies precedence: flag > CONFB_CONFIG > defaultConfigPath.
func resolveConfig(cmd *cobra.Command) (string, error) {
	if f := cmd.Flags().Lookup("config"); f != nil && f.Changed {
		cp, _ := cmd.Flags().GetString("config")
		return expandPath(cp), nil
	}
	if v := os.Getenv("CONFB_CONFIG"); v != "" {
		return expandPath(v), nil
	}
	return defaultConfigPath(), nil
}

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
  2) confb build
  3) confb run      (watch & rebuild)`,
		Version:           version,
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
	}

	cmd.SetVersionTemplate("confb version {{.Version}}\n")

	cmd.PersistentFlags().StringP("config", "c", defaultConfigPath(), "path to confb configuration file (env CONFB_CONFIG)")
	cmd.PersistentFlags().StringP("chdir", "C", "", "change working directory before reading config")

	// Honor --chdir early; also fold env into the flag if user didn't pass -c.
	cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		if cd, _ := c.Flags().GetString("chdir"); cd != "" {
			if err := os.Chdir(cd); err != nil {
				return fmt.Errorf("unable to chdir: %w", err)
			}
		}
		if f := c.Flags().Lookup("config"); f != nil && !f.Changed {
			if v := os.Getenv("CONFB_CONFIG"); v != "" {
				_ = c.Flags().Set("config", expandPath(v))
			}
		}
		return nil
	}

	// Optional: "version" alias so both "--version" and "version" work
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("confb version %s\n", version)
		},
	})

	// attach subcommands
	cmd.AddCommand(
		newBuildCmd(),
		newRunCmd(),
		newValidateCmd(),
		generateManCmd(cmd),
		newCompletionCmd(cmd),
		newReloadCmd(),
	)

	// default action with no subcommand: show help
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Help() }

	return cmd
}

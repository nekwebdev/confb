package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd sets up the base "confb" command tree.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confb",
		Short: "confb (config blender) — stitch multiple config fragments into deterministic outputs",
		Long: `confb reads a confb.yaml file and builds final config files
from an ordered set of fragments. Future versions can run as a daemon.`,
		SilenceUsage:  true,  // don’t show usage on user errors
		SilenceErrors: true,  // handle error messages ourselves
		Version:       version,
	}

	// global persistent flags (available to all subcommands)
	cmd.PersistentFlags().StringP("config", "c", "confb.yaml",
		"path to confb configuration file")
	cmd.PersistentFlags().StringP("chdir", "C", "",
		"change working directory before reading the config")

	// attach subcommands
	cmd.AddCommand(newBuildCmd())
  cmd.AddCommand(newRunCmd()) // register daemon

	return cmd
}


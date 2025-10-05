package cli

import "github.com/spf13/cobra"

// NewRootCmdForTest builds the root command for tests using the same
// unexported subcommand constructors used in production.
func NewRootCmdForTest() *cobra.Command {
	root := &cobra.Command{
		Use:   "confb",
		Short: "config blender",
	}
	// mirror root flags
	root.PersistentFlags().StringP("config", "c", "confb.yaml", "path to confb.yaml")
	root.PersistentFlags().String("chdir", "", "chdir before running command")

	// subcommands
	root.AddCommand(
		newBuildCmd(),
		newRunCmd(),
		newValidateCmd(),
	)
	return root
}

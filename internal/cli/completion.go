package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for Bash, Zsh, Fish, and PowerShell.

Examples:
  # bash (writes to stdout)
  confb completion bash > ~/.local/share/bash-completion/completions/confb

  # zsh
  confb completion zsh > ~/.local/share/zsh/site-functions/_confb

  # fish
  confb completion fish > ~/.config/fish/completions/confb.fish
`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Bash completion",
		RunE: func(c *cobra.Command, _ []string) error { return root.GenBashCompletionV2(os.Stdout, true) },
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Zsh completion",
		RunE: func(c *cobra.Command, _ []string) error { return root.GenZshCompletion(os.Stdout) },
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Fish completion",
		RunE: func(c *cobra.Command, _ []string) error { return root.GenFishCompletion(os.Stdout, true) },
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "powershell",
		Short: "PowerShell completion",
		RunE: func(c *cobra.Command, _ []string) error { return root.GenPowerShellCompletionWithDesc(os.Stdout) },
	})
	return cmd
}

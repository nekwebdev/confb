package cli

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// generateManCmd creates "confb man" command to generate manual pages
func generateManCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "man",
		Short: "Generate man pages for confb",
		Long: `Generate UNIX manual pages for confb and its subcommands.
By default, outputs to ./man1. Use --output to specify another directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" {
				outputDir = "./man1"
			}
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return err
			}
			header := &doc.GenManHeader{
				Title:   "CONFB",
				Section: "1",
				Source:  "confb",
				Manual:  "confb manual",
			}
			cmd.DisableAutoGenTag = true
			if err := doc.GenManTree(root, header, outputDir); err != nil {
				return err
			}
			log.Printf("Man pages written to %s\n", outputDir)
			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "./man1", "output directory for generated man pages")
	return cmd
}

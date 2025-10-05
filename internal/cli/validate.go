package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nekwebdev/confb/internal/config"
)

func newValidateCmd() *cobra.Command {
	var trace bool
	var list bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate confb.yaml (no build, no writes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Root().Flags().GetString("config")
			chdir, _ := cmd.Root().Flags().GetString("chdir")

			if chdir != "" {
				if err := os.Chdir(chdir); err != nil {
					return fmt.Errorf("failed to chdir to %q: %w", chdir, err)
				}
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if trace {
				base, err := cfg.BaseDir()
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "confb: baseDir = %s\n", base)
				absCfg, _ := filepath.Abs(cfgPath)
				fmt.Fprintf(os.Stderr, "confb: config = %s\n", absCfg)
			}

			if list {
				for _, t := range cfg.Targets {
					fmt.Fprintf(os.Stderr, "target: %s (format=%s, output=%s)\n", t.Name, t.Format, t.Output)
				}
			}

			// success if we got here
			fmt.Fprintln(os.Stderr, "confb: validation OK")
			return nil
		},
	}

	cmd.Flags().BoolVar(&trace, "trace", false, "print resolved baseDir and config path")
	cmd.Flags().BoolVar(&list, "list", false, "list targets after validation")
	return cmd
}

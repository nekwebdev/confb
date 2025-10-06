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
		Short: "Validate the confb.yaml without writing outputs",
		Long:  "Validate parses and checks confb.yaml (globs, rules, and options) and prints any errors.",
		Example: `  confb validate
  confb validate -c ./confb.yaml
  CONFB_CONFIG=./alt.yaml confb validate`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath, err := resolveConfig(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config invalid: %w", err)
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

			fmt.Fprintln(os.Stderr, "confb: validation OK")
			return nil
		},
	}

	cmd.Flags().BoolVar(&trace, "trace", false, "print resolved baseDir and config path")
	cmd.Flags().BoolVar(&list, "list", false, "list targets after validation")
	return cmd
}

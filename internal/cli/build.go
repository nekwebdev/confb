package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nekwebdev/confb/internal/config"
	executor "github.com/nekwebdev/confb/internal/exec"
	"github.com/nekwebdev/confb/internal/plan"
)

// parseOverrides turns ["name=/tmp/out", "other=..."] into a map.
func parseOverrides(pairs []string) (map[string]string, error) {
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		if !strings.Contains(p, "=") {
			return nil, fmt.Errorf("invalid --output-override %q (expected TARGET=PATH)", p)
		}
		kv := strings.SplitN(p, "=", 2)
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		if k == "" || v == "" {
			return nil, fmt.Errorf("invalid --output-override %q (empty key or value)", p)
		}
		out[k] = v
	}
	return out, nil
}

func newBuildCmd() *cobra.Command {
	var trace bool
	var dryRun bool
	var traceChecksums bool
	var overridesFlag []string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build all targets defined in confb.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Root().Flags().GetString("config")
			chdir, _ := cmd.Root().Flags().GetString("chdir")

			// optional chdir for reproducibility
			if chdir != "" {
				if err := os.Chdir(chdir); err != nil {
					return fmt.Errorf("failed to chdir to %q: %w", chdir, err)
				}
			}

			// load and validate
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			overrides, err := parseOverrides(overridesFlag)
			if err != nil {
				return err
			}

			// trace header
			if trace {
				base, err := cfg.BaseDir()
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "confb: baseDir = %s\n", base)
				absCfg, _ := filepath.Abs(cfgPath)
				fmt.Fprintf(os.Stderr, "confb: config = %s\n", absCfg)
			}

			if len(cfg.Targets) == 0 {
				return errors.New("no targets defined (validation should have caught this)")
			}

			// per-target planning + optional write
			for _, t := range cfg.Targets {
				override := overrides[t.Name]
				rt, err := plan.PlanTarget(cfg, t, override)
				if err != nil {
					return err
				}

				// show resolved files and any deduped ones
				fmt.Fprintf(os.Stderr, "\nTARGET %q\n", rt.Name)
				fmt.Fprintf(os.Stderr, "  output: %s\n", rt.Output)
				fmt.Fprintf(os.Stderr, "  files (%d):\n", len(rt.Files))
				for i, f := range rt.Files {
					fmt.Fprintf(os.Stderr, "    %2d. %s\n", i+1, f)
				}
				if len(rt.Deduped) > 0 {
					fmt.Fprintf(os.Stderr, "  deduped (%d):\n", len(rt.Deduped))
					for _, d := range rt.Deduped {
						fmt.Fprintf(os.Stderr, "    - %s\n", d)
					}
				}

				// optional checksum trace
				if traceChecksums {
					sum, err := executor.SHA256OfFiles(rt.Files)
					if err != nil {
						return fmt.Errorf("%s: checksum: %w", rt.Name, err)
					}
					fmt.Fprintf(os.Stderr, "  sha256(blended inputs): %s\n", sum)
				}

				if dryRun {
					fmt.Fprintf(os.Stderr, "  action: dry-run (no write)\n")
					continue
				}

				// do the write atomically
				if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "  action: wrote %s\n", rt.Output)
			}

			return nil
		},
	}

	// flags for build
	cmd.Flags().BoolVar(&trace, "trace", false, "print resolved baseDir and config details")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and plan only; do not write outputs")
	cmd.Flags().BoolVar(&traceChecksums, "trace-checksums", false, "compute and print SHA-256 of blended inputs")
	cmd.Flags().StringArrayVar(&overridesFlag, "output-override", nil, "override TARGET=PATH (repeatable)")

	return cmd
}

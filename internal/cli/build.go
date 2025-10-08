package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nekwebdev/confb/internal/blend"
	"github.com/nekwebdev/confb/internal/config"
	executor "github.com/nekwebdev/confb/internal/exec"
	"github.com/nekwebdev/confb/internal/plan"
)

// commentPrefixFor returns the single-line comment prefix for a given format,
// and whether comments are supported for that format.
func commentPrefixFor(format string) (string, bool) {
	switch strings.ToLower(format) {
	case "kdl":
		return "// ", true
	case "toml", "yaml", "yml":
		return "# ", true
	case "ini":
		return "; ", true
	default: // json, raw, unknown
		return "", false
	}
}

// headerForTarget builds the annotation header to prepend to an output file.
// It enumerates sources and merge rules, and includes version/time.
// Returns nil if the format doesn't support comments.
func headerForTarget(cmd *cobra.Command, t config.Target, rt *plan.ResolvedTarget) []byte {
	prefix, ok := commentPrefixFor(t.Format)
	if !ok {
		return nil
	}

	var lines []string
	lines = append(lines, "confb build")
	if v := cmd.Root().Version; v != "" {
		lines = append(lines, "version: "+v)
	}
	lines = append(lines,
		"fmt: "+strings.ToLower(t.Format),
		"target: "+t.Name,
		"output: "+rt.Output,
		"time: "+time.Now().Format(time.RFC3339),
	)

	// merge rule summary (format-aware)
	if t.Merge != nil && t.Merge.Rules != nil {
		r := t.Merge.Rules
		switch strings.ToLower(t.Format) {
		case "kdl":
			var parts []string
			if r.KDLKeys != "" {
				parts = append(parts, "keys="+strings.ToLower(r.KDLKeys))
			}
			if len(r.KDLSectionKeys) > 0 {
				parts = append(parts, "section_keys=["+strings.Join(r.KDLSectionKeys, ",")+"]")
			}
			if len(parts) > 0 {
				lines = append(lines, "merge.rules: "+strings.Join(parts, " "))
			}
		case "ini":
			if r.INIRepeatedKeys != "" {
				lines = append(lines, "merge.rules: repeated_keys="+strings.ToLower(r.INIRepeatedKeys))
			}
		default:
			var parts []string
			if r.Maps != "" {
				parts = append(parts, "maps="+strings.ToLower(r.Maps))
			}
			if r.Arrays != "" {
				parts = append(parts, "arrays="+strings.ToLower(r.Arrays))
			}
			if len(parts) > 0 {
				lines = append(lines, "merge.rules: "+strings.Join(parts, " "))
			}
		}
	}

	lines = append(lines, fmt.Sprintf("sources[%d]:", len(rt.Files)))
	for i, p := range rt.Files {
		sha := ""
		if b, err := os.ReadFile(p); err == nil {
			sum := sha256.Sum256(b)
			sha = hex.EncodeToString(sum[:])
		}
		lines = append(lines, fmt.Sprintf("  %d) %s sha256=%s", i+1, p, sha))
	}

	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteString(prefix)
		buf.WriteString(l)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n') // blank line after header
	return buf.Bytes()
}

// parseOverrides parses --output-override TARGET=PATH flags into a map.
func parseOverrides(list []string) (map[string]string, error) {
	out := make(map[string]string, len(list))
	for _, p := range list {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --output-override %q (expected TARGET=PATH)", p)
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
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
	var overridesFlag []string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build all targets once (no watch)",
		Long: `Build reads the configuration file, resolves sources (globs, tilde, dedupe),
merges or concatenates them per target, and writes outputs atomically.

notes:
  • loads default config from ~/.config/confb/confb.yaml unless -c is used or CONFB_CONFIG is set
	• use --trace to print resolved baseDir, config path, the target plan and merge rules
  • use --output-override TARGET=PATH to redirect a single target output
  • if the target format supports comments (kdl/toml/yaml/ini), the output is annotated
    with a header listing sources and (if present) merge rules. json/raw are never annotated.
  • no file watching here; see 'confb run' for the daemon (watch & rebuild).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// honor --chdir early
			if chdir, _ := cmd.Root().Flags().GetString("chdir"); chdir != "" {
				if err := os.Chdir(chdir); err != nil {
					return fmt.Errorf("chdir %q: %w", chdir, err)
				}
			}

			cfgPath, _ := cmd.Root().Flags().GetString("config")
			if cfgPath == "" {
				return errors.New("no config path (use -c/--config)")
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
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

			// per-target planning + write
			for _, t := range cfg.Targets {
				override := overrides[t.Name]
				rt, err := plan.PlanTarget(cfg, t, override)
				if err != nil {
					return err
				}

				if trace {
					fmt.Fprintf(os.Stderr, "target: %s (format=%s)\n", t.Name, strings.ToLower(t.Format))
					fmt.Fprintf(os.Stderr, "  output: %s\n", rt.Output)
					if len(rt.Files) > 0 {
						fmt.Fprintln(os.Stderr, "  files:")
						for _, f := range rt.Files {
							fmt.Fprintf(os.Stderr, "    - %s\n", f)
						}
					}
					if t.Merge != nil && t.Merge.Rules != nil {
						format := strings.ToLower(t.Format)
						r := t.Merge.Rules
						fmt.Fprintf(os.Stderr, "  merge.rules: ")
						switch format {
						case "kdl":
							fmt.Fprintf(os.Stderr, "keys=%s section_keys=%v\n", strings.ToLower(r.KDLKeys), r.KDLSectionKeys)
						case "ini":
							fmt.Fprintf(os.Stderr, "repeated_keys=%s\n", strings.ToLower(r.INIRepeatedKeys))
						default:
							fmt.Fprintf(os.Stderr, "maps=%s arrays=%s\n", strings.ToLower(r.Maps), strings.ToLower(r.Arrays))
						}
					}
				}

				if dryRun {
					fmt.Fprintf(os.Stderr, "confb: %s -> %s (dry-run)\n", t.Name, rt.Output)
					continue
				}

				// merged vs concat path
				if t.Merge != nil {
					format := strings.ToLower(t.Format)
					var content string
					switch format {
					case "yaml", "yml", "json", "toml":
						content, err = blend.BlendStructured(format, t.Merge.Rules, rt.Files)
					case "kdl":
						content, err = blend.BlendKDL(t.Merge.Rules, rt.Files)
					case "ini":
						content, err = blend.BlendINI(t.Merge.Rules, rt.Files)
					case "raw":
						err = fmt.Errorf("merge not supported for format %q", t.Format)
					default:
						err = fmt.Errorf("unknown format %q", t.Format)
					}
					if err != nil {
						return fmt.Errorf("%s: merge: %w", rt.Name, err)
					}

					// prepend header if supported
					header := headerForTarget(cmd, t, rt)
					if header != nil {
						var buf bytes.Buffer
						buf.Write(header)
						buf.WriteString(content)
						if err := executor.WriteAtomic(rt.Output, buf.String()); err != nil {
							return err
						}
					} else {
						if err := executor.WriteAtomic(rt.Output, content); err != nil {
							return err
						}
					}
					fmt.Fprintf(os.Stderr, "  action: merged (%s) -> wrote %s\n", format, rt.Output)
				} else {
					// concat; if header supported, we need to inject it by doing the concat here
					header := headerForTarget(cmd, t, rt)
					if header == nil {
						if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
							return err
						}
						fmt.Fprintf(os.Stderr, "  action: wrote %s\n", rt.Output)
						continue
					}
					// concat with normalization: CRLF->LF, ensure LF final newline per file
					var out bytes.Buffer
					out.Write(header)
					for _, f := range rt.Files {
						b, err := os.ReadFile(f)
						if err != nil {
							return err
						}
						s := string(b)
						s = strings.ReplaceAll(s, "\r\n", "\n")
						s = strings.ReplaceAll(s, "\r", "\n")
						if !strings.HasSuffix(s, "\n") {
							s += "\n"
						}
						out.WriteString(s)
					}
					if err := executor.WriteAtomic(rt.Output, out.String()); err != nil {
						return err
					}
					fmt.Fprintf(os.Stderr, "  action: wrote %s\n", rt.Output)
				}
			}
			return nil
		},
	}

	// flags for build
	cmd.Flags().BoolVar(&trace, "trace", false, "print resolved baseDir, config path, and per-target plan")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and plan only; do not write outputs")
	cmd.Flags().StringArrayVar(&overridesFlag, "output-override", nil, "override TARGET=PATH (repeatable)")

	return cmd
}

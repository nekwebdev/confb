package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

func newReloadCmd() *cobra.Command {
	var pidFileFlag string
	var unitFlag string
	var userUnit bool
	var method string
	var trace bool

	cmd := &cobra.Command{
		Use:   "reload",
		Short: "Signal the running confb daemon to reload configuration (SIGHUP)",
		Long: `Reload sends SIGHUP to the running confb daemon.

Methods:
  - pid:     read a PID file and send SIGHUP
  - systemd: use 'systemctl kill -s HUP <unit>' (system or --user)
  - auto:    try pid first (if provided/found), then systemd

Options:
  --pid-file: explicit pidfile path (expands ~)
  --unit:     systemd unit name (default: "confb.service")
  --user:     target the user systemd instance instead of system
  --method:   auto|pid|systemd (default: auto)

Search order for pid method (first match wins if --pid-file not set):
  1) ~/.cache/confb/confb.pid
  2) /run/user/<uid>/confb/confb.pid
  3) /var/run/confb.pid`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch method {
			case "", "auto", "pid", "systemd":
			default:
				return fmt.Errorf("invalid --method %q (expected auto|pid|systemd)", method)
			}
			if unitFlag == "" {
				unitFlag = "confb.service"
			}
			if method == "" {
				method = "auto"
			}

			// try pidfile first if method=auto or pid
			if method == "pid" || method == "auto" {
				if pidPath, err := resolvePIDPath(pidFileFlag); err == nil {
					if trace {
						fmt.Fprintf(os.Stderr, "confb: pidfile = %s\n", pidPath)
					}
					pid, err := readPID(pidPath)
					if err != nil {
						return err
					}
					if trace {
						fmt.Fprintf(os.Stderr, "confb: pid = %d\n", pid)
					}
					// verify running
					if err := syscall.Kill(pid, 0); err != nil {
						return fmt.Errorf("process %d not running (from %s): %w", pid, pidPath, err)
					}
					// send SIGHUP
					if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
						return fmt.Errorf("failed to send SIGHUP to pid %d: %w", pid, err)
					}
					fmt.Println("confb: reload signal sent (pid)")
					return nil
				} else if method == "pid" {
					// forced pid method and we couldn't find a pidfile
					if trace {
						fmt.Fprintf(os.Stderr, "confb: pid method failed: %v\n", err)
					}
					return err
				}
				// auto: fall through to systemd silently unless --trace
				if trace {
					fmt.Fprintln(os.Stderr, "confb: pidfile not found, trying systemd…")
				}
			}

			// systemd path (system first, then --user if auto and not explicitly --user)
			if method == "systemd" || method == "auto" {
				if err := trySystemdKill(unitFlag, userUnit, trace); err == nil {
					if userUnit {
						fmt.Println("confb: reload signal sent (systemd --user)")
					} else {
						fmt.Println("confb: reload signal sent (systemd)")
					}
					return nil
				} else if method == "auto" && !userUnit {
					if trace {
						fmt.Fprintln(os.Stderr, "confb: systemd (system) failed, trying --user…")
					}
					if err2 := trySystemdKill(unitFlag, true, trace); err2 == nil {
						fmt.Println("confb: reload signal sent (systemd --user)")
						return nil
					} else if trace {
						fmt.Fprintf(os.Stderr, "confb: systemd attempts failed: %v / %v\n", err, err2)
					}
				} else if method == "systemd" {
					if trace {
						fmt.Fprintf(os.Stderr, "confb: systemd failed\n")
					}
					return fmt.Errorf("systemd method failed for unit %q", unitFlag)
				}
			}

			return errors.New("could not reload daemon (no pidfile found and systemd attempts failed)")
		},
	}

	cmd.Flags().StringVar(&pidFileFlag, "pid-file", "", "override PID file path")
	cmd.Flags().StringVar(&unitFlag, "unit", "confb.service", "systemd unit name (e.g., confb.service)")
	cmd.Flags().BoolVar(&userUnit, "user", false, "use systemd --user instead of system instance")
	cmd.Flags().StringVar(&method, "method", "auto", "reload method: auto|pid|systemd")
	cmd.Flags().BoolVar(&trace, "trace", false, "verbose output")
	return cmd
}

// trySystemdKill executes `systemctl kill -s HUP <unit>`.
// It suppresses stdout/stderr unless trace=true.
func trySystemdKill(unit string, userInstance bool, trace bool) error {
	if runtime.GOOS != "linux" {
		return errors.New("systemd unavailable on this OS")
	}

	args := []string{}
	if userInstance {
		args = append(args, "--user")
	}

	// optional probe (quiet unless trace)
	if trace {
		fmt.Fprintf(os.Stderr, "confb: exec: systemctl %s is-active %s\n", strings.Join(args, " "), unit)
	}
	probe := exec.Command("systemctl", append(args, "is-active", unit)...)
	if !trace {
		probe.Stdout = nil
		probe.Stderr = nil
	}
	_ = probe.Run() // probe result not critical

	killArgs := append(args, "kill", "-s", "HUP", unit)
	if trace {
		fmt.Fprintf(os.Stderr, "confb: exec: systemctl %s\n", strings.Join(killArgs, " "))
	}
	cmd := exec.Command("systemctl", killArgs...)
	if !trace {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	return cmd.Run()
}

func resolvePIDPath(override string) (string, error) {
	if override != "" {
		p := expandHome(override)
		if fileExists(p) {
			return p, nil
		}
		return "", fmt.Errorf("specified --pid-file not found: %s", p)
	}

	// default search order
	candidates := []string{
		"~/.cache/confb/confb.pid",
		userRuntimePID(),
		"/var/run/confb.pid",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		p := expandHome(c)
		if fileExists(p) {
			return p, nil
		}
	}
	return "", errors.New("pidfile not found in default locations")
}

func userRuntimePID() string {
	u, err := user.Current()
	if err != nil || u.Uid == "" {
		return ""
	}
	return filepath.Join("/run/user", u.Uid, "confb", "confb.pid")
}

func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func readPID(p string) (int, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, fmt.Errorf("read pid file: %w", err)
	}
	s := strings.TrimSpace(string(b))
	pid, err := strconv.Atoi(s)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid in %s: %q", p, s)
	}
	return pid, nil
}

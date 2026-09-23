// cmd/harnessd/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/marceloamoreno87/workflow-dev-template/internal/daemon"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "harnessd:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("harnessd", flag.ContinueOnError)
	configPath := fs.String("config", ".harness/daemon.yaml", "daemon config file (relative paths resolve against the process working directory)")
	check := fs.Bool("check-config", false, "validate config and exit")
	install := fs.Bool("install-service", false, "write the systemd user unit and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *install {
		return installService(*configPath)
	}
	cfg, err := daemon.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if *check {
		fmt.Println("ok")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return daemon.Run(ctx, cfg, daemon.EmptySource())
}

func installService(configPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	unit, err := daemon.SystemdUnit(execPath, absConfig)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	destDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, "harnessd.service")
	if err := os.WriteFile(dest, []byte(unit), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\nrun: systemctl --user daemon-reload && systemctl --user enable --now harnessd\n", dest)
	return nil
}

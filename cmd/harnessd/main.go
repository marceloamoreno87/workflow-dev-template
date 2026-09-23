// cmd/harnessd/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
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
	if err := fs.Parse(args); err != nil {
		return err
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

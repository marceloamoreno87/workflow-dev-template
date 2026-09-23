// cmd/harness-mcp/main.go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/marceloamoreno87/workflow-dev-template/internal/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "harness-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("harness-mcp", flag.ContinueOnError)
	workItem := fs.String("work-item", "", "bound work item id")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *workItem == "" {
		return fmt.Errorf("work item required")
	}
	token := os.Getenv("HARNESS_MCP_TOKEN")
	if token == "" {
		return fmt.Errorf("HARNESS_MCP_TOKEN required")
	}
	return mcp.NewServer(token, *workItem, mcp.DiagBackend()).Serve(os.Stdin, os.Stdout)
}

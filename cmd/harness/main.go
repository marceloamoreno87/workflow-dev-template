// cmd/harness/main.go
package main

import (
	"os"

	"github.com/marceloamoreno87/workflow-dev-template/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

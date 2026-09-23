// internal/daemon/systemd.go
package daemon

import (
	"fmt"
	"strings"
)

// SystemdUnit renders the user-service unit for harnessd. Paths must be absolute;
// main resolves them before calling.
func SystemdUnit(execPath, configPath string) (string, error) {
	if strings.TrimSpace(execPath) == "" || strings.TrimSpace(configPath) == "" {
		return "", fmt.Errorf("%w: exec and config paths required", ErrDaemon)
	}
	return `[Unit]
Description=Harness daemon
After=network-online.target

[Service]
Type=simple
ExecStart=` + execPath + ` --config ` + configPath + `
Restart=on-failure
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=default.target
`, nil
}

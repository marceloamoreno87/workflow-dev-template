// internal/daemon/run.go
package daemon

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"time"
)

func DBPath(cfg Config) string {
	return filepath.Join(harnessDir(cfg.WorkspaceRoot), "harness.db")
}

func Run(ctx context.Context, cfg Config, source IntakeSource) error {
	return Serve(ctx, cfg, source, nil)
}

func Serve(ctx context.Context, cfg Config, source IntakeSource, ready chan<- string) error {
	d, err := Open(cfg, source)
	if err != nil {
		return err
	}
	defer d.Close()

	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		return err
	}
	if ready != nil {
		ready <- listener.Addr().String()
	}
	server := &http.Server{Handler: d.Dashboard().Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()

	if _, err := d.Tick(time.Now()); err != nil {
		_ = server.Close()
		return err
	}
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
			return nil
		case now := <-ticker.C:
			_, _ = d.Tick(now)
		}
	}
}

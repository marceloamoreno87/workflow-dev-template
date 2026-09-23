// internal/daemon/config.go
package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrDaemon = errors.New("invalid daemon configuration")

type Config struct {
	WorkspaceRoot string
	PollInterval  time.Duration
	BindAddr      string
	Token         string
}

func (c Config) Validate() error {
	if !filepath.IsAbs(c.WorkspaceRoot) {
		return fmt.Errorf("%w: workspace must be absolute", ErrDaemon)
	}
	if c.PollInterval < 5*time.Second || c.PollInterval > time.Hour {
		return fmt.Errorf("%w: poll interval outside 5s..1h", ErrDaemon)
	}
	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil || host == "" {
		return fmt.Errorf("%w: bind %q", ErrDaemon, c.BindAddr)
	}
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("%w: bind %q is not loopback", ErrDaemon, c.BindAddr)
		}
	}
	if len([]byte(c.Token)) < 16 {
		return fmt.Errorf("%w: operator token too short", ErrDaemon)
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("%w: unreadable config", ErrDaemon)
	}
	if len(raw) > 64*1024 {
		return Config{}, fmt.Errorf("%w: config too large", ErrDaemon)
	}
	var doc struct {
		SchemaVersion string `yaml:"schemaVersion"`
		Workspace     string `yaml:"workspace"`
		PollInterval  string `yaml:"pollInterval"`
		Bind          string `yaml:"bind"`
		TokenFile     string `yaml:"tokenFile"`
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return Config{}, fmt.Errorf("%w: malformed config", ErrDaemon)
	}
	if doc.SchemaVersion != "harness.daemon/v1" {
		return Config{}, fmt.Errorf("%w: schema %q", ErrDaemon, doc.SchemaVersion)
	}
	interval, err := time.ParseDuration(doc.PollInterval)
	if err != nil {
		return Config{}, fmt.Errorf("%w: poll interval", ErrDaemon)
	}
	tokenPath := doc.TokenFile
	if tokenPath == "" {
		return Config{}, fmt.Errorf("%w: token file required", ErrDaemon)
	}
	if !filepath.IsAbs(tokenPath) {
		tokenPath = filepath.Join(filepath.Dir(path), tokenPath)
	}
	tokenRaw, err := os.ReadFile(tokenPath)
	if err != nil {
		return Config{}, fmt.Errorf("%w: unreadable token file", ErrDaemon)
	}
	cfg := Config{
		WorkspaceRoot: doc.Workspace,
		PollInterval:  interval,
		BindAddr:      doc.Bind,
		Token:         strings.TrimSpace(string(tokenRaw)),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

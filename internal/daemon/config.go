// internal/daemon/config.go
package daemon

import (
	"errors"
	"fmt"
	"net"
	"net/url"
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
	RequiredRoles []string
	BudgetUSD     float64
	MaxDuration   time.Duration
	Telegram      *TelegramConfig
}

type TelegramConfig struct {
	BaseURL         string
	AllowedUsers    []int64
	AllowedChats    []int64
	BotToken        string
	ChallengeSecret []byte
}

var defaultRoles = []string{"product", "implementer", "reviewer"}

const defaultBudgetUSD = 10

const defaultMaxDuration = 2 * time.Hour

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
	if len(c.RequiredRoles) == 0 {
		return fmt.Errorf("%w: at least one required role", ErrDaemon)
	}
	if !(c.BudgetUSD > 0) {
		return fmt.Errorf("%w: budget must be positive", ErrDaemon)
	}
	if c.MaxDuration <= 0 {
		return fmt.Errorf("%w: max duration must be positive", ErrDaemon)
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
		SchemaVersion string   `yaml:"schemaVersion"`
		Workspace     string   `yaml:"workspace"`
		PollInterval  string   `yaml:"pollInterval"`
		Bind          string   `yaml:"bind"`
		TokenFile     string   `yaml:"tokenFile"`
		Roles         []string `yaml:"roles"`
		BudgetUSD     *float64 `yaml:"budgetUSD"`
		MaxDuration   string   `yaml:"maxDuration"`
		Telegram      *struct {
			BaseURL             string  `yaml:"baseURL"`
			AllowedUsers        []int64 `yaml:"allowedUsers"`
			AllowedChats        []int64 `yaml:"allowedChats"`
			BotTokenFile        string  `yaml:"botTokenFile"`
			ChallengeSecretFile string  `yaml:"challengeSecretFile"`
		} `yaml:"telegram"`
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
		BudgetUSD:     defaultBudgetUSD,
	}
	if doc.Roles == nil {
		cfg.RequiredRoles = append([]string{}, defaultRoles...)
	} else {
		cfg.RequiredRoles = append([]string{}, doc.Roles...)
	}
	if doc.BudgetUSD != nil {
		cfg.BudgetUSD = *doc.BudgetUSD
	}
	if doc.MaxDuration == "" {
		cfg.MaxDuration = defaultMaxDuration
	} else {
		maxDuration, err := time.ParseDuration(doc.MaxDuration)
		if err != nil {
			return Config{}, fmt.Errorf("%w: max duration", ErrDaemon)
		}
		cfg.MaxDuration = maxDuration
	}
	if doc.Telegram != nil {
		telegram, err := loadTelegramConfig(filepath.Dir(path), doc.Telegram)
		if err != nil {
			return Config{}, err
		}
		cfg.Telegram = telegram
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func readSecretFile(dir, name, what string) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: %s file required", ErrDaemon, what)
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(dir, name)
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%w: unreadable %s file", ErrDaemon, what)
	}
	return []byte(strings.TrimSpace(string(raw))), nil
}

func loadTelegramConfig(dir string, doc *struct {
	BaseURL             string  `yaml:"baseURL"`
	AllowedUsers        []int64 `yaml:"allowedUsers"`
	AllowedChats        []int64 `yaml:"allowedChats"`
	BotTokenFile        string  `yaml:"botTokenFile"`
	ChallengeSecretFile string  `yaml:"challengeSecretFile"`
},
) (*TelegramConfig, error) {
	u, err := url.Parse(doc.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: telegram base url needs http(s) with host", ErrDaemon)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: credentials do not belong in urls", ErrDaemon)
	}
	if len(doc.AllowedUsers) == 0 || len(doc.AllowedChats) == 0 {
		return nil, fmt.Errorf("%w: telegram needs allowed users and chats", ErrDaemon)
	}
	botToken, err := readSecretFile(dir, doc.BotTokenFile, "bot token")
	if err != nil {
		return nil, err
	}
	if len(botToken) == 0 {
		return nil, fmt.Errorf("%w: bot token required", ErrDaemon)
	}
	secret, err := readSecretFile(dir, doc.ChallengeSecretFile, "challenge secret")
	if err != nil {
		return nil, err
	}
	if len(secret) < 16 {
		return nil, fmt.Errorf("%w: challenge secret too short", ErrDaemon)
	}
	return &TelegramConfig{
		BaseURL:         strings.TrimSuffix(u.Scheme+"://"+u.Host+u.Path, "/"),
		AllowedUsers:    doc.AllowedUsers,
		AllowedChats:    doc.AllowedChats,
		BotToken:        string(botToken),
		ChallengeSecret: secret,
	}, nil
}

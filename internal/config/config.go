package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const AppName = "elsabo"

// Config is persisted user settings.
type Config struct {
	SSHHost         string `yaml:"ssh_host"`
	SSHUser         string `yaml:"ssh_user"`
	SSHPort         int    `yaml:"ssh_port"`
	SSHStrictHost   bool   `yaml:"ssh_strict_host"`
	SSHIdentityFile string `yaml:"ssh_identity_file"`
	SSHKnownHosts   string `yaml:"ssh_known_hosts"`
	// SSHPassword is stored as plain text in YAML; prefer ELSABO_SSH_PASSWORD env for shared machines.
	SSHPassword  string              `yaml:"ssh_password,omitempty"`
	SSHTargets   []SSHTarget         `yaml:"ssh_targets,omitempty"`
	MogwaiCmd    string              `yaml:"mogwai_cmd"`
	DefaultEntry string              `yaml:"default_entry"`
	VaultDir     string              `yaml:"vault_dir"`
	DomainTags   map[string][]string `yaml:"domain_tags"`
	Verify       VerifyConfig        `yaml:"verify"`
}

// VerifyConfig tunes HTTP checks after deploy.
type VerifyConfig struct {
	Timeout           time.Duration `yaml:"timeout"`
	UserAgent         string        `yaml:"user_agent"`
	MinBodyBytes      int           `yaml:"min_body_bytes"`
	MaxRedirects      int           `yaml:"max_redirects"`
	ExtraStubSnippets []string      `yaml:"extra_stub_snippets"`
	TryWWW            bool          `yaml:"try_www"`
}

type rawConfig struct {
	SSHHost         string              `yaml:"ssh_host"`
	SSHUser         string              `yaml:"ssh_user"`
	SSHPort         int                 `yaml:"ssh_port"`
	SSHStrictHost   *bool               `yaml:"ssh_strict_host"`
	SSHIdentityFile string              `yaml:"ssh_identity_file"`
	SSHKnownHosts   string              `yaml:"ssh_known_hosts"`
	SSHPassword     string              `yaml:"ssh_password"`
	SSHTargets      []SSHTarget         `yaml:"ssh_targets"`
	MogwaiCmd       string              `yaml:"mogwai_cmd"`
	DefaultEntry    string              `yaml:"default_entry"`
	VaultDir        string              `yaml:"vault_dir"`
	DomainTags      map[string][]string `yaml:"domain_tags"`
	Verify          *rawVerify          `yaml:"verify"`
}

type rawVerify struct {
	Timeout           *time.Duration `yaml:"timeout"`
	UserAgent         *string        `yaml:"user_agent"`
	MinBodyBytes      *int           `yaml:"min_body_bytes"`
	MaxRedirects      *int           `yaml:"max_redirects"`
	ExtraStubSnippets []string       `yaml:"extra_stub_snippets"`
	TryWWW            *bool          `yaml:"try_www"`
}

// Defaults returns baseline options.
func Defaults() Config {
	home, _ := os.UserHomeDir()
	vault := filepath.Join(home, ".config", AppName, "vault")
	return Config{
		SSHPort:       22,
		SSHStrictHost: false,
		MogwaiCmd:     "mogwai sites list",
		DefaultEntry:  "index.php",
		VaultDir:      vault,
		DomainTags:    map[string][]string{},
		Verify: VerifyConfig{
			Timeout:      25 * time.Second,
			UserAgent:    "Mozilla/5.0 (compatible; Elsabo/1.0; +https://github.com/MidasWR/Elasbo)",
			MinBodyBytes: 80,
			MaxRedirects: 5,
			TryWWW:       true,
		},
	}
}

// Load reads YAML from path.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawConfig
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	c := Defaults()
	if raw.SSHHost != "" {
		c.SSHHost = raw.SSHHost
	}
	if raw.SSHUser != "" {
		c.SSHUser = raw.SSHUser
	}
	if raw.SSHPort != 0 {
		c.SSHPort = raw.SSHPort
	}
	if raw.SSHStrictHost != nil {
		c.SSHStrictHost = *raw.SSHStrictHost
	}
	if raw.SSHIdentityFile != "" {
		c.SSHIdentityFile = raw.SSHIdentityFile
	}
	if raw.SSHKnownHosts != "" {
		c.SSHKnownHosts = raw.SSHKnownHosts
	}
	if raw.SSHPassword != "" {
		c.SSHPassword = raw.SSHPassword
	}
	if len(raw.SSHTargets) > 0 {
		c.SSHTargets = append([]SSHTarget(nil), raw.SSHTargets...)
	}
	if raw.MogwaiCmd != "" {
		c.MogwaiCmd = raw.MogwaiCmd
	}
	if raw.DefaultEntry != "" {
		c.DefaultEntry = raw.DefaultEntry
	}
	if raw.VaultDir != "" {
		c.VaultDir = raw.VaultDir
	}
	if raw.DomainTags != nil {
		c.DomainTags = map[string][]string{}
		for k, v := range raw.DomainTags {
			c.DomainTags[normalizeDomain(k)] = append([]string(nil), v...)
		}
	}
	if raw.Verify != nil {
		rv := raw.Verify
		if rv.Timeout != nil {
			c.Verify.Timeout = *rv.Timeout
		}
		if rv.UserAgent != nil {
			c.Verify.UserAgent = *rv.UserAgent
		}
		if rv.MinBodyBytes != nil {
			c.Verify.MinBodyBytes = *rv.MinBodyBytes
		}
		if rv.MaxRedirects != nil {
			c.Verify.MaxRedirects = *rv.MaxRedirects
		}
		if len(rv.ExtraStubSnippets) > 0 {
			c.Verify.ExtraStubSnippets = append([]string(nil), rv.ExtraStubSnippets...)
		}
		if rv.TryWWW != nil {
			c.Verify.TryWWW = *rv.TryWWW
		}
	}
	if v := strings.TrimSpace(os.Getenv("ELSABO_SSH_PASSWORD")); v != "" {
		c.SSHPassword = v
	}
	return &c, nil
}

// Save writes config to path (creates parent dirs).
func Save(path string, c *Config) error {
	if c == nil {
		return errors.New("nil config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// DefaultPath returns ~/.config/elsabo/config.yaml
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "config.yaml")
	}
	return filepath.Join(home, ".config", AppName, "config.yaml")
}

func normalizeDomain(d string) string {
	return strings.ToLower(strings.TrimSpace(d))
}

// TagsFor returns tags for a server name (normalized key).
func (c *Config) TagsFor(serverName string) []string {
	if c == nil || c.DomainTags == nil {
		return nil
	}
	return append([]string(nil), c.DomainTags[normalizeDomain(serverName)]...)
}

// SetTags replaces tags for a domain.
func (c *Config) SetTags(serverName string, tags []string) {
	if c.DomainTags == nil {
		c.DomainTags = map[string][]string{}
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	c.DomainTags[normalizeDomain(serverName)] = out
}

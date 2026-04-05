package config

import (
	"fmt"
	"strconv"
	"strings"
)

// SSHTarget is one panel / server to connect to.
type SSHTarget struct {
	Name     string `yaml:"name,omitempty"`
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Port     int    `yaml:"port,omitempty"`
	Password string `yaml:"password,omitempty"` // optional; bulk: tab-separated after user@host
}

var errLineEmpty = fmt.Errorf("skip empty line")

// EffectiveTargets returns dial targets: ssh_targets if set, else a single entry from ssh_host + ssh_user.
func EffectiveTargets(c *Config) []SSHTarget {
	if c == nil {
		return nil
	}
	if len(c.SSHTargets) > 0 {
		out := make([]SSHTarget, 0, len(c.SSHTargets))
		for _, t := range c.SSHTargets {
			host := strings.TrimSpace(t.Host)
			if host == "" {
				continue
			}
			user := strings.TrimSpace(t.User)
			if user == "" {
				user = strings.TrimSpace(c.SSHUser)
			}
			if user == "" {
				continue
			}
			port := t.Port
			if port == 0 {
				port = c.SSHPort
			}
			if port == 0 {
				port = 22
			}
			name := strings.TrimSpace(t.Name)
			out = append(out, SSHTarget{
				Name: name, Host: host, User: user, Port: port,
				Password: strings.TrimSpace(t.Password),
			})
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	host := strings.TrimSpace(c.SSHHost)
	user := strings.TrimSpace(c.SSHUser)
	if host == "" || user == "" {
		return nil
	}
	port := c.SSHPort
	if port == 0 {
		port = 22
	}
	return []SSHTarget{{Host: host, User: user, Port: port}}
}

// ParseSSHLine parses one non-empty line: user@host[:port] optional \t password
// Lines starting with # are skipped (returns errLineEmpty).
func ParseSSHLine(line string) (SSHTarget, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return SSHTarget{}, errLineEmpty
	}
	pass := ""
	if tab := strings.Index(line, "\t"); tab >= 0 {
		pass = line[tab+1:]
		line = strings.TrimSpace(line[:tab])
	}
	at := strings.LastIndex(line, "@")
	if at <= 0 || at >= len(line)-1 {
		return SSHTarget{}, fmt.Errorf("need user@host: %q", line)
	}
	user := strings.TrimSpace(line[:at])
	rest := strings.TrimSpace(line[at+1:])
	if user == "" || rest == "" {
		return SSHTarget{}, fmt.Errorf("need user@host: %q", line)
	}
	host := rest
	port := 0
	// user@host:2222 — take numeric suffix as port
	if i := strings.LastIndex(rest, ":"); i > 0 && i < len(rest)-1 {
		tail := rest[i+1:]
		if p, err := strconv.Atoi(tail); err == nil && p > 0 && p < 65536 {
			host = strings.TrimSpace(rest[:i])
			port = p
		}
	}
	if host == "" {
		return SSHTarget{}, fmt.Errorf("empty host: %q", line)
	}
	return SSHTarget{Host: host, User: user, Port: port, Password: pass}, nil
}

// ParseSSHBulk parses newline-separated user@host[:port] entries.
func ParseSSHBulk(text string) ([]SSHTarget, []error) {
	var targets []SSHTarget
	var errs []error
	for _, line := range strings.Split(text, "\n") {
		t, err := ParseSSHLine(line)
		if err != nil {
			if err == errLineEmpty {
				continue
			}
			errs = append(errs, fmt.Errorf("%q: %w", strings.TrimSpace(line), err))
			continue
		}
		if t.Port == 0 {
			t.Port = 0 // filled from config default when dialing
		}
		targets = append(targets, t)
	}
	return targets, errs
}

// FormatSSHBulk renders targets as lines for the bulk editor (without names).
func FormatSSHBulk(targets []SSHTarget) string {
	if len(targets) == 0 {
		return ""
	}
	var b strings.Builder
	for i, t := range targets {
		if i > 0 {
			b.WriteByte('\n')
		}
		var line string
		if t.Port > 0 {
			line = fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port)
		} else {
			line = fmt.Sprintf("%s@%s", t.User, t.Host)
		}
		b.WriteString(line)
		if strings.TrimSpace(t.Password) != "" {
			b.WriteByte('\t')
			b.WriteString(t.Password)
		}
	}
	return b.String()
}

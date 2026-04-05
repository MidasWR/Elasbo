package sshutil

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client wraps ssh.Session and SFTP factory per connection.
type Client struct {
	cfg    Config
	client *ssh.Client
}

// Config for dialing.
type Config struct {
	Host         string
	User         string
	Port         int
	KnownHosts   string // optional path for StrictHost / knownhosts
	IdentityFile string // optional explicit private key (tried before agent keys)
	Password     string // optional; tried after public-key methods (plaintext in config — avoid if possible)
	StrictHost   bool
	timeout      time.Duration
}

func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return p
}

// Dial opens one SSH connection (password not supported — use keys / ssh-agent).
func Dial(ctx context.Context, cfg Config) (*Client, error) {
	cfg.IdentityFile = expandPath(cfg.IdentityFile)
	cfg.KnownHosts = expandPath(cfg.KnownHosts)
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.timeout == 0 {
		cfg.timeout = 30 * time.Second
	}
	authMethods, err := authMethodsForDial(cfg)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := hostKeyCallback(cfg)
	if err != nil {
		return nil, err
	}
	cc := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.timeout,
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	cch, chans, reqs, err := ssh.NewClientConn(raw, addr, cc)
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return &Client{cfg: cfg, client: ssh.NewClient(cch, chans, reqs)}, nil
}

func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Run executes a shell command on the remote (non-interactive).
func (c *Client) Run(ctx context.Context, cmdline string) (stdout, stderr string, exit int, err error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer sess.Close()
	var outBuf, errBuf bytes.Buffer
	sess.Stdout = &outBuf
	sess.Stderr = &errBuf
	done := make(chan error, 1)
	go func() {
		done <- sess.Run(cmdline)
	}()
	select {
	case <-ctx.Done():
		_ = sess.Close()
		return outBuf.String(), errBuf.String(), -1, ctx.Err()
	case err := <-done:
		if err != nil {
			if ee, ok := err.(*ssh.ExitError); ok {
				return outBuf.String(), errBuf.String(), ee.ExitStatus(), nil
			}
			return outBuf.String(), errBuf.String(), -1, err
		}
		return outBuf.String(), errBuf.String(), 0, nil
	}
}

// ClientForSFTP returns the underlying ssh client for github.com/pkg/sftp — we use embedded sftp in x/crypto.
func (c *Client) Raw() *ssh.Client {
	return c.client
}

func authMethodsForDial(cfg Config) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	pw := strings.TrimSpace(cfg.Password)

	if idPath := strings.TrimSpace(cfg.IdentityFile); idPath != "" {
		b, err := os.ReadFile(idPath)
		if err != nil {
			if os.IsNotExist(err) && pw != "" {
				// password-only / wrong path: continue without this key
			} else if os.IsNotExist(err) {
				return nil, fmt.Errorf("identity file not found %q: clear SSH key field or set ssh_password", idPath)
			} else {
				return nil, fmt.Errorf("ssh identity %s: %w", idPath, err)
			}
		} else {
			key, err := ssh.ParsePrivateKey(b)
			if err != nil {
				if pw == "" {
					return nil, fmt.Errorf("ssh identity %s: %w", idPath, err)
				}
			} else {
				methods = append(methods, ssh.PublicKeys(key))
			}
		}
	}

	rest, errKeys := authFromAgentAndKeys()
	if errKeys == nil {
		methods = append(methods, rest...)
	}
	if pw != "" {
		methods = append(methods, ssh.Password(pw))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no ssh auth: set ssh_password, ssh_identity_file, SSH_AUTH_SOCK, or a key in ~/.ssh")
	}
	return methods, nil
}

func authFromAgentAndKeys() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			ag := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(ag.Signers))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{"id_ed25519", "id_rsa"} {
			p := filepath.Join(home, ".ssh", name)
			if b, err := os.ReadFile(p); err == nil {
				key, err := ssh.ParsePrivateKey(b)
				if err == nil {
					methods = append(methods, ssh.PublicKeys(key))
				}
			}
		}
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no ssh auth: set SSH_AUTH_SOCK or place a key in ~/.ssh")
	}
	return methods, nil
}

func hostKeyCallback(cfg Config) (ssh.HostKeyCallback, error) {
	if !cfg.StrictHost {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	kh := strings.TrimSpace(cfg.KnownHosts)
	var paths []string
	if kh != "" {
		paths = []string{kh}
	} else {
		if home, err := os.UserHomeDir(); err == nil {
			paths = []string{filepath.Join(home, ".ssh", "known_hosts")}
		}
	}
	var cb ssh.HostKeyCallback
	var err error
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, statErr := os.Stat(p); statErr != nil {
			continue
		}
		cb, err = knownhosts.New(p)
		if err != nil {
			return nil, err
		}
		break
	}
	if cb == nil {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	return cb, nil
}

// ExpandRemoteHome is unused — kept for future.
func ExpandRemoteHome(p string) string {
	return strings.TrimSpace(p)
}

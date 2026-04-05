package sshutil

import (
	"context"
	"io"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTP wraps a client for file operations on the remote.
type SFTP struct {
	cli *sftp.Client
}

// NewSFTP opens an SFTP session over an existing SSH client.
func NewSFTP(c *ssh.Client) (*SFTP, error) {
	cli, err := sftp.NewClient(c)
	if err != nil {
		return nil, err
	}
	return &SFTP{cli: cli}, nil
}

func (s *SFTP) Close() error {
	if s.cli == nil {
		return nil
	}
	return s.cli.Close()
}

// ReadFile reads full remote path.
func (s *SFTP) ReadFile(ctx context.Context, path string) ([]byte, error) {
	f, err := s.cli.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readAllCtx(ctx, f)
}

// WriteFile writes data to path (creates or truncates).
// Remove deletes a remote file.
func (s *SFTP) Remove(ctx context.Context, path string) error {
	errCh := make(chan error, 1)
	go func() { errCh <- s.cli.Remove(path) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *SFTP) WriteFile(ctx context.Context, path string, data []byte, perm os.FileMode) error {
	f, err := s.cli.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer f.Close()
	if perm != 0 {
		_ = f.Chmod(perm)
	}
	n, err := f.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}

func readAllCtx(ctx context.Context, r io.Reader) ([]byte, error) {
	ch := make(chan struct{})
	var buf []byte
	var err error
	go func() {
		buf, err = io.ReadAll(r)
		close(ch)
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ch:
		return buf, err
	}
}

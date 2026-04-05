package fastpanel

import (
	"context"
	"fmt"

	"github.com/midaswr/elsabo/internal/sshutil"
)

// FetchSites runs mogwai on the remote via SSH and parses the result.
func FetchSites(ctx context.Context, ssh *sshutil.Client, mogwaiCmd string) ([]Site, error) {
	if mogwaiCmd == "" {
		mogwaiCmd = "mogwai sites list"
	}
	out, stderr, exit, err := ssh.Run(ctx, mogwaiCmd)
	if err != nil {
		return nil, fmt.Errorf("ssh run: %w", err)
	}
	if exit != 0 {
		return nil, fmt.Errorf("mogwai exit %d: %s", exit, stderr)
	}
	if stderr != "" && len(stderr) > 4 {
		// warnings only — still try parse stdout
		_ = stderr
	}
	return ParseSitesList(out)
}

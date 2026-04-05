package replace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"

	"github.com/midaswr/elsabo/internal/config"
	"github.com/midaswr/elsabo/internal/fastpanel"
	"github.com/midaswr/elsabo/internal/verify"
)

// RemoteFiles abstracts SFTP operations (implemented by sshutil.SFTP, or fakes in tests).
type RemoteFiles interface {
	ReadFile(ctx context.Context, remotePath string) ([]byte, error)
	WriteFile(ctx context.Context, remotePath string, data []byte, perm os.FileMode) error
	Remove(ctx context.Context, remotePath string) error
}

// Runner performs replace + verify + rollback.
type Runner struct {
	Config *config.Config
	Remote RemoteFiles
}

// SiteResult is the outcome for one domain.
type SiteResult struct {
	Site       fastpanel.Site
	OK         bool
	RollbackOK bool
	Message    string
	Verify     verify.Result
}

// ReplaceSite swaps entry file and rolls back if verify fails.
func (r *Runner) ReplaceSite(ctx context.Context, site fastpanel.Site, entryName string, newContent []byte) SiteResult {
	res := SiteResult{Site: site}
	if entryName == "" {
		res.Message = "empty_entry_name"
		return res
	}
	if r.Remote == nil {
		res.Message = "nil_remote"
		return res
	}
	if r.Config == nil {
		res.Message = "nil_config"
		return res
	}

	remotePath := path.Join(site.DocumentRoot, entryName)

	prev, err := r.Remote.ReadFile(ctx, remotePath)
	hadOriginal := true
	if err != nil {
		if isRemoteNotExist(err) {
			hadOriginal = false
			prev = nil
		} else {
			res.Message = fmt.Sprintf("read_original: %v", err)
			return res
		}
	}

	if err := r.Remote.WriteFile(ctx, remotePath, newContent, 0o644); err != nil {
		res.Message = fmt.Sprintf("write_new: %v", err)
		return res
	}

	vctx, cancel := verify.WithTimeout(ctx, r.Config.Verify)
	defer cancel()
	vres := verify.CheckSite(vctx, r.Config.Verify, site.ServerName)
	res.Verify = vres
	if vres.OK {
		res.OK = true
		res.Message = "replaced_and_verified"
		return res
	}

	if hadOriginal {
		if err := r.Remote.WriteFile(ctx, remotePath, prev, 0o644); err != nil {
			res.RollbackOK = false
			res.Message = fmt.Sprintf("verify_failed:%s; rollback_write_failed:%v", vres.Reason, err)
			return res
		}
		res.RollbackOK = true
		res.Message = fmt.Sprintf("verify_failed:%s; rolled_back", vres.Reason)
		return res
	}

	if err := r.Remote.Remove(ctx, remotePath); err != nil {
		res.RollbackOK = false
		res.Message = fmt.Sprintf("verify_failed:%s; rollback_remove_failed:%v", vres.Reason, err)
		return res
	}
	res.RollbackOK = true
	res.Message = fmt.Sprintf("verify_failed:%s; removed_new_file", vres.Reason)
	return res
}

func isRemoteNotExist(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	var se *sftp.StatusError
	if errors.As(err, &se) && se.FxCode() == sftp.ErrSSHFxNoSuchFile {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such file") || strings.Contains(s, "not exist")
}

package verify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/midaswr/elsabo/internal/config"
)

// Result of an HTTP check.
type Result struct {
	OK         bool
	Reason     string
	StatusCode int
	URL        string
	FinalURL   string
	BodySample string
}

var defaultStubSnippets = []string{
	"welcome to nginx",
	"nginx docker",
	"successfuly installed nginx",
	"it works!",
	"apache2 ubuntu default page",
	"apache http server test page",
	"centos-webPanel test page",
	"powered by almalinux",
	"proudly powered by litespeed web server",
	"checking your browser",
}

// CheckSite tries https://host/ first, then https://www.host/ only when the first attempt
// fails for transport/DNS reasons — not when the server responds but the page looks wrong
// (HTTP errors, short body, stubs). Otherwise a “good” www response could mask a broken apex
// after deploy and skip rollback.
func CheckSite(ctx context.Context, cfg config.VerifyConfig, host string) Result {
	host = strings.TrimSpace(host)
	if host == "" {
		return Result{OK: false, Reason: "empty_host"}
	}
	apex := buildURL(host, false)
	last := checkURL(ctx, cfg, cacheBustURL(apex))
	if last.OK {
		return last
	}
	if cfg.TryWWW && shouldTryWWWAfterFailure(last) && !strings.HasPrefix(strings.ToLower(host), "www.") {
		alt := buildURL(host, true)
		return checkURL(ctx, cfg, cacheBustURL(alt))
	}
	return last
}

func shouldTryWWWAfterFailure(r Result) bool {
	if r.OK {
		return false
	}
	s := r.Reason
	switch {
	case strings.HasPrefix(s, "http_"):
		return false
	case strings.HasPrefix(s, "body_too_small"):
		return false
	case strings.HasPrefix(s, "detected_server_stub"):
		return false
	case strings.HasPrefix(s, "read_body"):
		return false
	case s == "empty_host", s == "bad_request":
		return false
	default:
		// connection_failed, TLS/DNS errors, etc.
		return true
	}
}

func cacheBustURL(raw string) string {
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + "elsabo_cb=" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func buildURL(host string, www bool) string {
	h := host
	if www && !strings.HasPrefix(strings.ToLower(h), "www.") {
		h = "www." + h
	}
	return "https://" + h + "/"
}

func checkURL(ctx context.Context, cfg config.VerifyConfig, rawURL string) Result {
	client := &http.Client{
		Timeout: cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{OK: false, Reason: "bad_request", URL: rawURL}
	}
	if cfg.UserAgent != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return Result{
			OK:     false,
			Reason: fmt.Sprintf("connection_failed: %v", err),
			URL:    rawURL,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{
			OK:         false,
			Reason:     fmt.Sprintf("read_body: %v", err),
			StatusCode: resp.StatusCode,
			URL:        rawURL,
			FinalURL:   resp.Request.URL.String(),
		}
	}

	lower := strings.ToLower(string(body))
	rc := resp.StatusCode
	if rc == 429 {
		return Result{
			OK:         false,
			Reason:     "http_429",
			StatusCode: rc,
			URL:        rawURL,
			FinalURL:   resp.Request.URL.String(),
			BodySample: sample(string(body)),
		}
	}
	if rc < 200 || rc >= 300 {
		return Result{
			OK:         false,
			Reason:     fmt.Sprintf("http_%d", rc),
			StatusCode: rc,
			URL:        rawURL,
			FinalURL:   resp.Request.URL.String(),
			BodySample: sample(string(body)),
		}
	}

	minB := cfg.MinBodyBytes
	if minB <= 0 {
		minB = 80
	}
	if len(body) < minB {
		return Result{
			OK:         false,
			Reason:     fmt.Sprintf("body_too_small:%d<%d", len(body), minB),
			StatusCode: rc,
			URL:        rawURL,
			FinalURL:   resp.Request.URL.String(),
			BodySample: sample(string(body)),
		}
	}

	for _, snip := range append(defaultStubSnippets, cfg.ExtraStubSnippets...) {
		s := strings.ToLower(strings.TrimSpace(snip))
		if s != "" && strings.Contains(lower, s) {
			return Result{
				OK:         false,
				Reason:     "detected_server_stub:" + s,
				StatusCode: rc,
				URL:        rawURL,
				FinalURL:   resp.Request.URL.String(),
				BodySample: sample(string(body)),
			}
		}
	}

	return Result{
		OK:         true,
		Reason:     "ok",
		StatusCode: rc,
		URL:        rawURL,
		FinalURL:   resp.Request.URL.String(),
	}
}

func sample(s string) string {
	const max = 120
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// WithTimeout wraps context with cfg timeout if parent has none.
func WithTimeout(parent context.Context, cfg config.VerifyConfig) (context.Context, context.CancelFunc) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 25 * time.Second
	}
	return context.WithTimeout(parent, cfg.Timeout+5*time.Second)
}

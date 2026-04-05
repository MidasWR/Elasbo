package verify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MidasWR/Elasbo/internal/config"
)

func TestCheckURL_stubNginx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>Welcome to nginx!</body></html>"))
	}))
	defer srv.Close()
	cfg := config.VerifyConfig{
		Timeout:      5 * time.Second,
		MinBodyBytes: 20,
		MaxRedirects: 3,
	}
	r := checkURL(context.Background(), cfg, srv.URL+"/")
	if r.OK {
		t.Fatalf("expected stub failure, got %+v", r)
	}
	if r.Reason == "ok" {
		t.Fatal(r)
	}
}

func TestCheckURL_ok(t *testing.T) {
	body := make([]byte, 200)
	for i := range body {
		body[i] = 'x'
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	cfg := config.VerifyConfig{
		Timeout:      5 * time.Second,
		MinBodyBytes: 80,
		MaxRedirects: 3,
	}
	r := checkURL(context.Background(), cfg, srv.URL+"/")
	if !r.OK {
		t.Fatalf("expected ok, got %+v", r)
	}
}

func TestShouldTryWWWAfterFailure(t *testing.T) {
	if shouldTryWWWAfterFailure(Result{OK: false, Reason: "http_500"}) {
		t.Fatal("http error must not fall through to www")
	}
	if shouldTryWWWAfterFailure(Result{OK: false, Reason: "body_too_small:1<80"}) {
		t.Fatal("content probe must not fall through to www")
	}
	if shouldTryWWWAfterFailure(Result{OK: false, Reason: "detected_server_stub:nginx"}) {
		t.Fatal("stub detect must not fall through to www")
	}
	if !shouldTryWWWAfterFailure(Result{OK: false, Reason: "connection_failed: dial tcp: missing address"}) {
		t.Fatal("transport errors should allow www retry")
	}
}

func TestCheckURL_smallBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()
	cfg := config.VerifyConfig{
		Timeout:      5 * time.Second,
		MinBodyBytes: 80,
		MaxRedirects: 3,
	}
	r := checkURL(context.Background(), cfg, srv.URL+"/")
	if r.OK {
		t.Fatal(r)
	}
}

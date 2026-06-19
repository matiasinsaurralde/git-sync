package syncer

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-git/go-git/v6/plumbing/transport"
	transporthttp "github.com/go-git/go-git/v6/plumbing/transport/http"

	"entire.io/entire/git-sync/internal/auth"
	"entire.io/entire/git-sync/internal/gitproto"
)

func TestResolveAuthMethodPrefersExplicitToken(t *testing.T) {
	ep, err := transport.ParseURL("https://github.com/entireio/cli.git")
	if err != nil {
		t.Fatalf("new endpoint: %v", err)
	}

	originalCred := auth.GitCredentialCommand
	t.Cleanup(func() { auth.GitCredentialCommand = originalCred })
	auth.GitCredentialCommand = func(_ context.Context, op auth.CredentialOp, input string) ([]byte, error) {
		t.Fatalf("unexpected git credential %s call with input %q", op, input)
		return nil, nil
	}

	resolved := auth.Resolve(auth.Endpoint{
		Username: "git",
		Token:    "explicit-token",
	}, ep)

	basic, ok := resolved.(*transporthttp.BasicAuth)
	if !ok {
		t.Fatalf("expected basic auth, got %T", resolved)
	}
	if basic.Username != "git" || basic.Password != "explicit-token" {
		t.Fatalf("unexpected auth: %+v", basic)
	}
}

func TestNewHTTPConnSkipTLSVerify(t *testing.T) {
	stats := newStats(false)
	conn, err := newConn(Endpoint{
		URL:           "https://example.com/repo.git",
		SkipTLSVerify: true,
	}, "source", stats, nil)
	if err != nil {
		t.Fatalf("new conn: %v", err)
	}
	httpConn, ok := conn.(*gitproto.HTTPConn)
	if !ok {
		t.Fatalf("expected *gitproto.HTTPConn, got %T", conn)
	}
	rt, ok := httpConn.HTTP.Transport.(*countingRoundTripper)
	if !ok {
		t.Fatalf("expected countingRoundTripper, got %T", httpConn.HTTP.Transport)
	}
	base, ok := rt.base.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport base, got %T", rt.base)
	}
	if base.TLSClientConfig == nil || !base.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected InsecureSkipVerify transport")
	}
}

func TestNewHTTPConnUsesProvidedHTTPClient(t *testing.T) {
	stats := newStats(false)
	baseTransport := http.DefaultTransport
	baseClient := &http.Client{Transport: baseTransport}

	conn, err := newConn(Endpoint{URL: "https://example.com/repo.git"}, "source", stats, baseClient)
	if err != nil {
		t.Fatalf("new conn: %v", err)
	}
	httpConn, ok := conn.(*gitproto.HTTPConn)
	if !ok {
		t.Fatalf("expected *gitproto.HTTPConn, got %T", conn)
	}
	if httpConn.HTTP == baseClient {
		t.Fatalf("expected cloned HTTP client, got original pointer")
	}
	rt, ok := httpConn.HTTP.Transport.(*countingRoundTripper)
	if !ok {
		t.Fatalf("expected countingRoundTripper, got %T", httpConn.HTTP.Transport)
	}
	if rt.base != baseTransport {
		t.Fatalf("wrapped base transport = %T, want %T", rt.base, baseTransport)
	}
}

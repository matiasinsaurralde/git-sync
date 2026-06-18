package auth

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-git/v6/plumbing/transport"
	transporthttp "github.com/go-git/go-git/v6/plumbing/transport/http"
)

func TestCredentialInput_FillQueryWithEmbeddedUser(t *testing.T) {
	ep := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/owner/repo.git",
		User:   url.User("myuser"),
	}

	got := credentialInput(ep, "", "")
	want := "protocol=https\nhost=github.com\npath=owner/repo.git\nusername=myuser\n\n"
	if got != want {
		t.Errorf("credentialInput returned:\n%q\nwant:\n%q", got, want)
	}
}

func TestCredentialInput_NilEndpoint(t *testing.T) {
	got := credentialInput(nil, "", "")
	if got != "" {
		t.Errorf("expected empty string for nil endpoint, got %q", got)
	}
}

func TestCredentialInput_EmptyHost(t *testing.T) {
	ep := &url.URL{Scheme: "https"}
	got := credentialInput(ep, "", "")
	if got != "" {
		t.Errorf("expected empty string for empty host, got %q", got)
	}
}

func TestCredentialInput_FillQueryNoUser(t *testing.T) {
	ep := &url.URL{
		Scheme: "https",
		Host:   "example.com",
		Path:   "/repo.git",
	}
	got := credentialInput(ep, "", "")
	want := "protocol=https\nhost=example.com\npath=repo.git\n\n"
	if got != want {
		t.Errorf("credentialInput returned:\n%q\nwant:\n%q", got, want)
	}
}

// A non-default port must appear in the host field (gitcredentials(7) host is
// "host[:port]"); otherwise credentials are stored/looked up under the wrong
// (default-port) entry.
func TestCredentialInput_IncludesPort(t *testing.T) {
	ep := &url.URL{
		Scheme: "https",
		Host:   "example.com:8443",
		Path:   "/repo.git",
	}
	got := credentialInput(ep, "", "")
	want := "protocol=https\nhost=example.com:8443\npath=repo.git\n\n"
	if got != want {
		t.Errorf("credentialInput returned:\n%q\nwant:\n%q", got, want)
	}
}

func TestCredentialInput_ApproveRejectFormatIncludesUserAndPassword(t *testing.T) {
	ep := &url.URL{
		Scheme: "https",
		Host:   "example.com",
		Path:   "/owner/repo.git",
	}
	got := credentialInput(ep, "alice", "s3cret")
	want := "protocol=https\nhost=example.com\npath=owner/repo.git\nusername=alice\npassword=s3cret\n\n"
	if got != want {
		t.Errorf("credentialInput returned:\n%q\nwant:\n%q", got, want)
	}
}

func TestCredentialInput_ExplicitUserOverridesURLUser(t *testing.T) {
	ep := &url.URL{
		Scheme: "https",
		Host:   "example.com",
		User:   url.User("from-url"),
	}
	got := credentialInput(ep, "explicit", "")
	if !strings.Contains(got, "username=explicit\n") {
		t.Errorf("expected explicit username to win, got:\n%q", got)
	}
	if strings.Contains(got, "username=from-url") {
		t.Errorf("URL-embedded username should be overridden, got:\n%q", got)
	}
}

func TestParseCredentialOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantUser string
		wantPass string
		wantLen  int
	}{
		{
			name:     "standard username and password",
			output:   "username=foo\npassword=bar\n",
			wantUser: "foo",
			wantPass: "bar",
			wantLen:  2,
		},
		{
			name:     "with extra fields",
			output:   "protocol=https\nhost=example.com\nusername=alice\npassword=secret\n",
			wantUser: "alice",
			wantPass: "secret",
			wantLen:  4,
		},
		{
			name:    "empty input",
			output:  "",
			wantLen: 0,
		},
		{
			name:    "blank lines only",
			output:  "\n\n\n",
			wantLen: 0,
		},
		{
			name:     "value with equals sign",
			output:   "password=tok=en\n",
			wantPass: "tok=en",
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCredentialOutput([]byte(tt.output))
			if len(got) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d; result = %v", len(got), tt.wantLen, got)
			}
			if tt.wantUser != "" {
				if got["username"] != tt.wantUser {
					t.Errorf("username = %q, want %q", got["username"], tt.wantUser)
				}
			}
			if tt.wantPass != "" {
				if got["password"] != tt.wantPass {
					t.Errorf("password = %q, want %q", got["password"], tt.wantPass)
				}
			}
		})
	}
}

func TestResolve(t *testing.T) {
	ep, err := transport.ParseURL("https://example.com/repo.git")
	if err != nil {
		t.Fatal(err)
	}

	sshEP := &url.URL{Scheme: "ssh", Host: "example.com", Path: "/repo.git"}

	tests := []struct {
		name     string
		raw      Endpoint
		ep       *url.URL
		wantType string // "token", "basic", "nil"
		wantUser string
		wantPass string
	}{
		{
			name:     "bearer token set returns TokenAuth",
			raw:      Endpoint{BearerToken: "my-bearer"},
			ep:       ep,
			wantType: "token",
			wantPass: "my-bearer",
		},
		{
			name:     "token with username returns BasicAuth",
			raw:      Endpoint{Token: "my-token", Username: "alice"},
			ep:       ep,
			wantType: "basic",
			wantUser: "alice",
			wantPass: "my-token",
		},
		{
			name:     "token without username returns BasicAuth with git",
			raw:      Endpoint{Token: "my-token"},
			ep:       ep,
			wantType: "basic",
			wantUser: "git",
			wantPass: "my-token",
		},
		{
			name:     "nothing set non-HTTP endpoint returns nil",
			raw:      Endpoint{},
			ep:       sshEP,
			wantType: "nil",
		},
		{
			name:     "nothing set HTTP endpoint returns nil (defer to helper on 401)",
			raw:      Endpoint{},
			ep:       ep,
			wantType: "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Resolve must never consult the git credential helper —
			// that lookup is deferred to a 401 response. If anything
			// here invokes the helper, fail loudly.
			origCmd := GitCredentialCommand
			defer func() { GitCredentialCommand = origCmd }()
			GitCredentialCommand = func(_ context.Context, op CredentialOp, input string) ([]byte, error) {
				t.Fatalf("unexpected GitCredentialCommand(%q, %q) call during Resolve", op, input)
				return nil, nil
			}

			got := Resolve(tt.raw, tt.ep)

			switch tt.wantType {
			case "nil":
				if got != nil {
					t.Errorf("expected nil auth, got %T", got)
				}
			case "token":
				ta, ok := got.(*transporthttp.TokenAuth)
				if !ok {
					t.Fatalf("expected *TokenAuth, got %T", got)
				}
				if ta.Token != tt.wantPass {
					t.Errorf("token = %q, want %q", ta.Token, tt.wantPass)
				}
			case "basic":
				ba, ok := got.(*transporthttp.BasicAuth)
				if !ok {
					t.Fatalf("expected *BasicAuth, got %T", got)
				}
				if ba.Username != tt.wantUser {
					t.Errorf("username = %q, want %q", ba.Username, tt.wantUser)
				}
				if ba.Password != tt.wantPass {
					t.Errorf("password = %q, want %q", ba.Password, tt.wantPass)
				}
			}
		})
	}
}

func TestExplicitAuth(t *testing.T) {
	tests := []struct {
		name     string
		raw      Endpoint
		wantType string // "token", "basic", "nil"
		wantUser string
		wantPass string
	}{
		{
			name:     "bearer token returns TokenAuth",
			raw:      Endpoint{BearerToken: "bearer-abc"},
			wantType: "token",
			wantPass: "bearer-abc",
		},
		{
			name:     "token with username returns BasicAuth",
			raw:      Endpoint{Token: "tok", Username: "bob"},
			wantType: "basic",
			wantUser: "bob",
			wantPass: "tok",
		},
		{
			name:     "token without username returns BasicAuth with git",
			raw:      Endpoint{Token: "tok"},
			wantType: "basic",
			wantUser: "git",
			wantPass: "tok",
		},
		{
			name:     "nothing set returns nil",
			raw:      Endpoint{},
			wantType: "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explicitAuth(tt.raw)

			switch tt.wantType {
			case "nil":
				if got != nil {
					t.Errorf("expected nil, got %T", got)
				}
			case "token":
				ta, ok := got.(*transporthttp.TokenAuth)
				if !ok {
					t.Fatalf("expected *TokenAuth, got %T", got)
				}
				if ta.Token != tt.wantPass {
					t.Errorf("token = %q, want %q", ta.Token, tt.wantPass)
				}
			case "basic":
				ba, ok := got.(*transporthttp.BasicAuth)
				if !ok {
					t.Fatalf("expected *BasicAuth, got %T", got)
				}
				if ba.Username != tt.wantUser {
					t.Errorf("username = %q, want %q", ba.Username, tt.wantUser)
				}
				if ba.Password != tt.wantPass {
					t.Errorf("password = %q, want %q", ba.Password, tt.wantPass)
				}
			}
		})
	}
}

type recordedCredCall struct {
	op    CredentialOp
	input string
}

func withRecordingHelper(t *testing.T, calls *[]recordedCredCall, handler func(op CredentialOp, input string) ([]byte, error)) {
	t.Helper()
	orig := GitCredentialCommand
	t.Cleanup(func() { GitCredentialCommand = orig })
	GitCredentialCommand = func(_ context.Context, op CredentialOp, input string) ([]byte, error) {
		*calls = append(*calls, recordedCredCall{op: op, input: input})
		if handler == nil {
			return nil, nil
		}
		return handler(op, input)
	}
}

func TestGitCredentialHelper_Lookup_ReturnsCredentials(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com", Path: "/owner/repo.git"}
	var calls []recordedCredCall
	withRecordingHelper(t, &calls, func(op CredentialOp, _ string) ([]byte, error) {
		if op != CredentialOpFill {
			t.Fatalf("expected fill, got %q", op)
		}
		return []byte("username=alice\npassword=s3cret\n"), nil
	})

	user, pass, ok, err := GitCredentialHelper{}.Lookup(context.Background(), ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if user != "alice" || pass != "s3cret" {
		t.Errorf("got user=%q pass=%q, want alice/s3cret", user, pass)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 helper call, got %d", len(calls))
	}
	if !strings.Contains(calls[0].input, "protocol=https\nhost=example.com\n") {
		t.Errorf("fill input missing host/protocol:\n%q", calls[0].input)
	}
}

func TestGitCredentialHelper_Lookup_HelperFailsReturnsNotFound(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com"}
	withRecordingHelper(t, new([]recordedCredCall), func(_ CredentialOp, _ string) ([]byte, error) {
		return nil, errors.New("no helper")
	})

	_, _, ok, err := GitCredentialHelper{}.Lookup(context.Background(), ep)
	if err != nil {
		t.Errorf("expected no error when helper has no credentials, got %v", err)
	}
	if ok {
		t.Error("expected ok=false when helper fails")
	}
}

// TestGitCredentialHelper_Lookup_ContextCanceledSurfacesError ensures a
// cancelled context isn't masked as "no credentials available": when the
// `git credential fill` subprocess dies because the context is gone, Lookup
// must return the context error so callers report it instead of falling back
// to the original HTTP 401.
func TestGitCredentialHelper_Lookup_ContextCanceledSurfacesError(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	withRecordingHelper(t, new([]recordedCredCall), func(_ CredentialOp, _ string) ([]byte, error) {
		// exec.CommandContext kills the subprocess once the context is done,
		// surfacing as a command error.
		return nil, errors.New("signal: killed")
	})

	_, _, ok, err := GitCredentialHelper{}.Lookup(ctx, ep)
	if ok {
		t.Error("expected ok=false on a cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestGitCredentialHelper_Lookup_EmptyPasswordReturnsNotFound(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com"}
	withRecordingHelper(t, new([]recordedCredCall), func(_ CredentialOp, _ string) ([]byte, error) {
		return []byte("username=alice\n"), nil
	})

	_, _, ok, err := GitCredentialHelper{}.Lookup(context.Background(), ep)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when password is empty")
	}
}

func TestGitCredentialHelper_Lookup_UsernameFallsBackToGit(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com"}
	withRecordingHelper(t, new([]recordedCredCall), func(_ CredentialOp, _ string) ([]byte, error) {
		return []byte("password=tok\n"), nil
	})

	user, _, ok, err := GitCredentialHelper{}.Lookup(context.Background(), ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if user != "git" {
		t.Errorf("expected username fallback to 'git', got %q", user)
	}
}

func TestGitCredentialHelper_Lookup_NonHTTPEndpointReturnsNotFound(t *testing.T) {
	ep := &url.URL{Scheme: "ssh", Host: "example.com"}
	calls := new([]recordedCredCall)
	withRecordingHelper(t, calls, func(_ CredentialOp, _ string) ([]byte, error) {
		return []byte("password=tok\n"), nil
	})

	_, _, ok, err := GitCredentialHelper{}.Lookup(context.Background(), ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for SSH endpoint — credential helper protocol is HTTP-only")
	}
	if len(*calls) != 0 {
		t.Errorf("expected no helper call for SSH endpoint, got %d", len(*calls))
	}
}

func TestGitCredentialHelper_Approve_SendsCredentialsToHelper(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com", Path: "/owner/repo.git"}
	var calls []recordedCredCall
	withRecordingHelper(t, &calls, nil)

	GitCredentialHelper{}.Approve(context.Background(), ep, "alice", "s3cret")

	if len(calls) != 1 || calls[0].op != CredentialOpApprove {
		t.Fatalf("expected one 'approve' call, got %+v", calls)
	}
	want := "protocol=https\nhost=example.com\npath=owner/repo.git\nusername=alice\npassword=s3cret\n\n"
	if calls[0].input != want {
		t.Errorf("approve input:\n%q\nwant:\n%q", calls[0].input, want)
	}
}

func TestGitCredentialHelper_Reject_SendsCredentialsToHelper(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com"}
	var calls []recordedCredCall
	withRecordingHelper(t, &calls, nil)

	GitCredentialHelper{}.Reject(context.Background(), ep, "alice", "bad")

	if len(calls) != 1 || calls[0].op != CredentialOpReject {
		t.Fatalf("expected one 'reject' call, got %+v", calls)
	}
	if !strings.Contains(calls[0].input, "username=alice\npassword=bad\n") {
		t.Errorf("reject input missing creds:\n%q", calls[0].input)
	}
}

func TestGitCredentialHelper_ApproveRejectSwallowHelperErrors(t *testing.T) {
	ep := &url.URL{Scheme: "https", Host: "example.com"}
	withRecordingHelper(t, new([]recordedCredCall), func(_ CredentialOp, _ string) ([]byte, error) {
		return nil, errors.New("helper unavailable")
	})

	// Approve/Reject must not panic when the helper is broken.
	GitCredentialHelper{}.Approve(context.Background(), ep, "u", "p")
	GitCredentialHelper{}.Reject(context.Background(), ep, "u", "p")
}

// TestGitCredentialCmdInheritsEnvWithoutOverridingTerminalPrompt locks in
// the corrected behaviour from issue #63: the proactive-Lookup path is what
// caused the original spurious prompt on a public repo (already fixed by
// deferring Lookup to a real 401), and we deliberately do NOT also force
// GIT_TERMINAL_PROMPT=0. Forcing it would block legitimate first-time
// authentication to a new host. Non-interactive callers (CI, daemons) set
// the env var in their own environment, and we inherit it as-is.
func TestGitCredentialCmdInheritsEnvWithoutOverridingTerminalPrompt(t *testing.T) {
	// When the parent process has no GIT_TERMINAL_PROMPT set, the
	// subprocess must not have one either — letting git's default
	// (prompting allowed) take effect.
	t.Setenv("GIT_TERMINAL_PROMPT", "")
	os.Unsetenv("GIT_TERMINAL_PROMPT")
	cmd := newGitCredentialCmd(context.Background(), CredentialOpFill, "protocol=https\nhost=example.com\n\n")
	// cmd.Env == nil means "inherit from parent" — equivalent to no override.
	// If the implementation sets cmd.Env explicitly we still want no
	// GIT_TERMINAL_PROMPT entry.
	for _, kv := range cmd.Env {
		if strings.HasPrefix(kv, "GIT_TERMINAL_PROMPT=") {
			t.Errorf("subprocess must not force GIT_TERMINAL_PROMPT; got %q", kv)
		}
	}

	// When the parent sets GIT_TERMINAL_PROMPT=0 (non-interactive callers),
	// the subprocess sees the same value — we pass it through, we don't
	// override or strip it.
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	cmd = newGitCredentialCmd(context.Background(), CredentialOpFill, "protocol=https\nhost=example.com\n\n")
	// cmd.Env == nil also satisfies this case (subprocess inherits the
	// parent env including the value we just Setenv'd). If cmd.Env is
	// populated, it must contain exactly the parent value.
	if cmd.Env != nil {
		var found, count int
		for _, kv := range cmd.Env {
			if strings.HasPrefix(kv, "GIT_TERMINAL_PROMPT=") {
				count++
				if kv == "GIT_TERMINAL_PROMPT=0" {
					found++
				}
			}
		}
		if count != 1 || found != 1 {
			t.Errorf("expected exactly one GIT_TERMINAL_PROMPT=0 entry passed through, got %d total / %d matching: %v", count, found, cmd.Env)
		}
	}
}

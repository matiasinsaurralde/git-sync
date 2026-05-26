package main

import (
	"strings"
	"testing"

	"entire.io/entire/git-sync/cmd/git-sync/internal/sha256convert"
)

func TestResolveConvertSHA256Args(t *testing.T) {
	const url = "http://example.invalid/repo.git"
	const dir = "/tmp/out"

	tests := []struct {
		name    string
		req     sha256convert.Request
		args    []string
		wantURL string
		wantDir string
		wantErr string
	}{
		{
			name:    "both positionals",
			args:    []string{url, dir},
			wantURL: url,
			wantDir: dir,
		},
		{
			name:    "url flag plus positional dir — the reported bug",
			req:     sha256convert.Request{SourceURL: url},
			args:    []string{dir},
			wantURL: url,
			wantDir: dir,
		},
		{
			name:    "dir flag plus positional url",
			req:     sha256convert.Request{TargetDir: dir},
			args:    []string{url},
			wantURL: url,
			wantDir: dir,
		},
		{
			name:    "both flags, no positionals",
			req:     sha256convert.Request{SourceURL: url, TargetDir: dir},
			args:    nil,
			wantURL: url,
			wantDir: dir,
		},
		{
			name:    "missing dir",
			req:     sha256convert.Request{SourceURL: url},
			args:    nil,
			wantErr: "requires a source URL and a target directory",
		},
		{
			name:    "missing both",
			args:    nil,
			wantErr: "requires a source URL and a target directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req
			err := resolveConvertSHA256Args(&req, tt.args)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
			if tt.wantErr != "" {
				return
			}
			if req.SourceURL != tt.wantURL {
				t.Errorf("SourceURL: got %q, want %q", req.SourceURL, tt.wantURL)
			}
			if req.TargetDir != tt.wantDir {
				t.Errorf("TargetDir: got %q, want %q", req.TargetDir, tt.wantDir)
			}
		})
	}
}

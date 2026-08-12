package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// CLI smoke tests exercise main()'s flag wiring via a subprocess binary.
// Unit tests already cover validateRemote / validateLocalPort; these catch
// regressions where main stops calling them or flag defaults change.

var (
	cliBinOnce sync.Once
	cliBinPath string
	cliBinErr  error
)

func cliBinary(t *testing.T) string {
	t.Helper()
	cliBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "untls-cli-bin-")
		if err != nil {
			cliBinErr = err
			return
		}
		cliBinPath = filepath.Join(dir, "untls")
		cmd := exec.Command("go", "build", "-o", cliBinPath, ".")
		out, err := cmd.CombinedOutput()
		if err != nil {
			cliBinErr = err
			cliBinPath = string(out)
			return
		}
	})
	if cliBinErr != nil {
		t.Fatalf("go build test binary: %v\n%s", cliBinErr, cliBinPath)
	}
	return cliBinPath
}

func TestCLI_FlagValidation(t *testing.T) {
	bin := cliBinary(t)

	tests := []struct {
		name       string
		args       []string
		wantSubstr string
	}{
		{
			name:       "missing -t",
			args:       nil,
			wantSubstr: "missing tcp socket",
		},
		{
			name:       "invalid -t port",
			args:       []string{"-t", "example.com:99999"},
			wantSubstr: "port must be 1-65535",
		},
		{
			name:       "invalid -l",
			args:       []string{"-t", "127.0.0.1:443", "-l", "-1"},
			wantSubstr: "invalid -l port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected non-zero exit, output=%q", out)
			}
			if !strings.Contains(string(out), tt.wantSubstr) {
				t.Fatalf("output %q does not contain %q", out, tt.wantSubstr)
			}
		})
	}
}

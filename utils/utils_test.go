// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	// Redirect HOME so the test never touches the real config directory.
	home := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("USERPROFILE", home)
	default:
		t.Setenv("HOME", home)
	}

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error = %v", err)
	}

	want := home
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		want = filepath.Join(home, ".config", "sgh")
	}
	if dir != want {
		t.Errorf("ConfigDir() = %q, want %q", dir, want)
	}

	// The directory must exist afterwards — callers write into it immediately.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config dir was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%s exists but is not a directory", dir)
	}
}

func TestConfigDirIsIdempotent(t *testing.T) {
	home := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("USERPROFILE", home)
	default:
		t.Setenv("HOME", home)
	}

	first, err := ConfigDir()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := ConfigDir()
	if err != nil {
		t.Fatalf("second call on an existing directory should succeed: %v", err)
	}
	if first != second {
		t.Errorf("ConfigDir() not stable: %q then %q", first, second)
	}
}

func TestValidateGitHubToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr string
	}{
		{"valid length", strings.Repeat("a", 20), ""},
		{"comfortably long", "ghp_" + strings.Repeat("a", 36), ""},

		{"empty", "", "cannot be empty"},
		{"one char short", strings.Repeat("a", 19), "too short"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGitHubToken(tt.token)

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// This helper is deliberately weaker than validation.ValidateGitHubToken: it
// checks length only and accepts any prefix. Callers needing prefix enforcement
// must use pkg/validation instead, so pin the difference here.
func TestValidateGitHubTokenDoesNotCheckPrefix(t *testing.T) {
	if err := ValidateGitHubToken(strings.Repeat("z", 40)); err != nil {
		t.Errorf("expected prefix-less token to pass this weaker check, got %v", err)
	}
}

func TestConstants(t *testing.T) {
	if CONFIG_FILE_NAME != "sgh.json" {
		t.Errorf("CONFIG_FILE_NAME = %q, want sgh.json", CONFIG_FILE_NAME)
	}
	if LOG_FILE_NAME != "sgh.log" {
		t.Errorf("LOG_FILE_NAME = %q, want sgh.log", LOG_FILE_NAME)
	}
	if EXCLUDE_REPOSITORY_FLAG != "exclude-repository" {
		t.Errorf("EXCLUDE_REPOSITORY_FLAG = %q, want exclude-repository", EXCLUDE_REPOSITORY_FLAG)
	}
}

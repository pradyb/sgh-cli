// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the configuration directory path for the current OS
func ConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	var configDir string
	switch runtime.GOOS {
	case "darwin", "linux":
		configDir = filepath.Join(homeDir, ".config", "sgh")
	case "windows":
		configDir = homeDir
	default:
		configDir = homeDir
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	return configDir, nil
}

// Constants for flags and configuration
const (
	EXCLUDE_REPOSITORY_FLAG = "exclude-repository"
	CONFIG_FILE_NAME        = "sgh.json"
	LOG_FILE_NAME           = "sgh.log"
)

// ValidateGitHubToken validates the GitHub token format
func ValidateGitHubToken(token string) error {
	if token == "" {
		return fmt.Errorf("GitHub token cannot be empty")
	}
	if len(token) < 20 {
		return fmt.Errorf("GitHub token appears to be invalid (too short)")
	}
	return nil
}

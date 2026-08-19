// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"
)

// Data goes to stdout; commentary goes to stderr. That split is what keeps
// `sgh repo list -J | jq` working while warnings remain visible on a terminal,
// so it is an interface guarantee rather than an implementation detail.
func TestCommentaryGoesToStderrNotStdout(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{
			"selected repos",
			func() { PrintSelectedRepos("create branch", "my-org", []string{"api-gateway"}) },
		},
		{
			"fuzzy match warning",
			func() {
				PrintFuzzyMatchWarning("api", []string{"api-gateway", "api-legacy"}, "api-gateway")
			},
		},
		{
			"no fuzzy match warning",
			func() { PrintNoFuzzyMatchWarning("nonexistent") },
		},
		{
			"cli error",
			func() { PrintCLIError("something failed", "try --verbose") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureBoth(t, tt.fn)

			if strings.TrimSpace(stdout) != "" {
				t.Errorf("wrote to stdout, which would corrupt piped output:\n%s", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Error("wrote nothing to stderr; the user would see no warning at all")
			}
		})
	}
}

// PrintJSON is the opposite case: it is data, so it must reach stdout and must
// not be polluted by anything on the same stream.
func TestPrintJSONGoesToStdout(t *testing.T) {
	stdout, stderr := captureBoth(t, func() {
		PrintJSON(map[string]any{"name": "api-gateway"})
	})

	if !strings.Contains(stdout, "api-gateway") {
		t.Errorf("PrintJSON must write to stdout, got:\n%s", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Errorf("PrintJSON wrote to stderr:\n%s", stderr)
	}
}

func TestPrintCLIErrorIncludesHints(t *testing.T) {
	out := captureStderr(t, func() {
		PrintCLIError("token is invalid", "run sgh whoami", "check SGH_TOKEN")
	})

	for _, want := range []string{"token is invalid", "run sgh whoami", "check SGH_TOKEN"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

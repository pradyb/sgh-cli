// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package service

// SetGitHubBaseURLForTesting overrides the GitHub REST API base URL and
// returns a function that restores the original value.
//
// It exists here, rather than alongside the other test helpers in
// internal/service/servicetest, only because githubBaseURL is unexported.
// Deliberately keep this file free of any "testing" or "net/http/httptest"
// import: internal/service is reachable from every command package, so
// anything it imports is linked into the shipped binary.
//
// Prefer servicetest.SetGitHubBaseURL, which wraps this.
func SetGitHubBaseURLForTesting(url string) (restore func()) {
	original := githubBaseURL
	githubBaseURL = url
	return func() { githubBaseURL = original }
}

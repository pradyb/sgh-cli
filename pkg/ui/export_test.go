// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package ui

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// capture redirects one of the standard streams to a pipe while fn runs and
// returns everything written to it. The Print* functions in this package write
// directly to os.Stdout / os.Stderr, so this is the only way to assert on what
// a user actually sees.
func capture(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	original := *stream
	*stream = w
	t.Cleanup(func() { *stream = original })

	// Drain concurrently so a write larger than the pipe buffer cannot deadlock.
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	func() {
		defer func() {
			_ = w.Close()
			*stream = original
		}()
		fn()
	}()

	out := <-done
	_ = r.Close()
	return out
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return capture(t, &os.Stdout, fn)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return capture(t, &os.Stderr, fn)
}

// captureBoth returns stdout and stderr separately for the same call, which is
// how the tests verify that data goes to stdout and commentary goes to stderr.
func captureBoth(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, fn)
	})
	return stdout, stderr
}

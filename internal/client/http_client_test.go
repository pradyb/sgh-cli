// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package client

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

const testToken = "ghp_1234567890abcdef1234567890abcdef123456"

// transportOf digs the real *http.Transport out from behind the interceptor.
func transportOf(t *testing.T, c *HttpClient) *http.Transport {
	t.Helper()

	interceptor, ok := c.Client.Transport.(*Interceptor)
	if !ok {
		t.Fatalf("expected transport to be *Interceptor, got %T", c.Client.Transport)
	}

	transport, ok := interceptor.OriginalTransport.(*http.Transport)
	if !ok {
		t.Fatalf("expected original transport to be *http.Transport, got %T", interceptor.OriginalTransport)
	}

	return transport
}

// A custom http.Transport inherits nothing from http.DefaultTransport, so Proxy
// has to be wired up explicitly or HTTP_PROXY/HTTPS_PROXY/NO_PROXY are silently
// ignored and every request behind a corporate proxy fails. This guards against
// the field being dropped again.
//
// We assert on the function identity rather than on a resolved proxy URL:
// http.ProxyFromEnvironment caches the environment on first use process-wide,
// so any resolution-based assertion would depend on test ordering.
func TestNewHttpClientUsesProxyFromEnvironment(t *testing.T) {
	transport := transportOf(t, NewHttpClient(30*time.Second, testToken))

	if transport.Proxy == nil {
		t.Fatal("transport.Proxy is nil: HTTP_PROXY, HTTPS_PROXY and NO_PROXY would be ignored")
	}

	want := reflect.ValueOf(http.ProxyFromEnvironment).Pointer()
	if got := reflect.ValueOf(transport.Proxy).Pointer(); got != want {
		t.Error("transport.Proxy is not http.ProxyFromEnvironment")
	}
}

func TestNewHttpClientTransportSettings(t *testing.T) {
	transport := transportOf(t, NewHttpClient(30*time.Second, testToken))

	if !transport.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 = false, want true")
	}
	if transport.MaxIdleConnsPerHost == 0 {
		t.Error("MaxIdleConnsPerHost = 0, want a bounded pool")
	}
	if transport.TLSHandshakeTimeout == 0 {
		t.Error("TLSHandshakeTimeout = 0, want a bounded handshake")
	}
}

func TestNewHttpClientTimeout(t *testing.T) {
	const timeout = 45 * time.Second

	c := NewHttpClient(timeout, testToken)
	if c.Client.Timeout != timeout {
		t.Errorf("Client.Timeout = %v, want %v", c.Client.Timeout, timeout)
	}
	if c.Token != testToken {
		t.Errorf("Token = %q, want %q", c.Token, testToken)
	}
}

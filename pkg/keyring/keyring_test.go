// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package keyring

import "testing"

// TestOSKeyring_RoundTrip exercises the real OS keyring. CI's Linux runner has no
// Secret Service running, so this skips rather than fails when the backend truly isn't
// there — the same "no keyring available" case the rest of this feature degrades for.
func TestOSKeyring_RoundTrip(t *testing.T) {
	k := New()
	const owner = "sgh-keyring-test-owner"
	t.Cleanup(func() { _ = k.Delete(owner) })

	if err := k.Set(owner, "ghp_roundtrip_test"); err != nil {
		t.Skipf("no OS keyring backend available in this environment: %v", err)
	}

	tok, ok, err := k.Get(owner)
	if err != nil || !ok || tok != "ghp_roundtrip_test" {
		t.Fatalf("Get() = (%q, %v, %v), want (ghp_roundtrip_test, true, nil)", tok, ok, err)
	}

	if err := k.Delete(owner); err != nil {
		t.Fatalf("Delete(): %v", err)
	}
	if _, ok, _ := k.Get(owner); ok {
		t.Error("expected no entry after Delete()")
	}
}

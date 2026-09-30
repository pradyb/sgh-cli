// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package keyring

import (
	"errors"
	"testing"
)

func TestFake_SetGetDelete(t *testing.T) {
	f := NewFake()

	if _, ok, err := f.Get("acme"); ok || err != nil {
		t.Fatalf("Get() on empty fake = (_, %v, %v), want (_, false, nil)", ok, err)
	}

	if err := f.Set("acme", "ghp_secret"); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	tok, ok, err := f.Get("acme")
	if err != nil || !ok || tok != "ghp_secret" {
		t.Fatalf("Get() = (%q, %v, %v), want (ghp_secret, true, nil)", tok, ok, err)
	}

	// Overwrite.
	if err := f.Set("acme", "ghp_updated"); err != nil {
		t.Fatalf("Set() overwrite: %v", err)
	}
	if tok, _, _ := f.Get("acme"); tok != "ghp_updated" {
		t.Errorf("Get() after overwrite = %q, want ghp_updated", tok)
	}

	if err := f.Delete("acme"); err != nil {
		t.Fatalf("Delete(): %v", err)
	}
	if _, ok, _ := f.Get("acme"); ok {
		t.Error("expected no entry after Delete()")
	}

	// Deleting an already-absent entry is not an error (matches the real keyring's
	// ErrNotFound-is-not-a-failure behaviour).
	if err := f.Delete("acme"); err != nil {
		t.Errorf("Delete() on absent entry: %v, want nil", err)
	}
}

func TestFake_MultipleOwnersAreIndependent(t *testing.T) {
	f := NewFake()
	f.Set("acme", "ghp_acme")
	f.Set("other", "ghp_other")

	if tok, _, _ := f.Get("acme"); tok != "ghp_acme" {
		t.Errorf("acme = %q, want ghp_acme", tok)
	}
	if tok, _, _ := f.Get("other"); tok != "ghp_other" {
		t.Errorf("other = %q, want ghp_other", tok)
	}

	f.Delete("acme")
	if tok, _, _ := f.Get("other"); tok != "ghp_other" {
		t.Errorf("deleting acme must not affect other, got %q", tok)
	}
}

func TestFake_Unavailable(t *testing.T) {
	f := NewFake()
	f.Unavailable = errors.New("no keyring backend")
	f.data["acme"] = "ghp_preexisting" // bypass Set to seed data despite Unavailable

	if err := f.Set("acme", "x"); !errors.Is(err, f.Unavailable) {
		t.Errorf("Set() = %v, want Unavailable", err)
	}
	if _, ok, err := f.Get("acme"); ok || !errors.Is(err, f.Unavailable) {
		t.Errorf("Get() = (_, %v, %v), want (_, false, Unavailable)", ok, err)
	}
	if err := f.Delete("acme"); !errors.Is(err, f.Unavailable) {
		t.Errorf("Delete() = %v, want Unavailable", err)
	}
}

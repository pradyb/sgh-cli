// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/pradyb/sgh-cli/pkg/keyring"
)

// withFakeKeyring swaps in a fresh Fake for the duration of one test and restores the
// package-level TestMain fake afterward, so tests can run independently of each other.
func withFakeKeyring(t *testing.T) *keyring.Fake {
	t.Helper()
	orig := TokenKeyring
	fake := keyring.NewFake()
	TokenKeyring = fake
	t.Cleanup(func() { TokenKeyring = orig })
	return fake
}

func TestSetToken_UsesKeyringWhenAvailable(t *testing.T) {
	withFakeKeyring(t)
	cfg := &Config{}
	cfg.AddOrganization("acme")

	usedKeyring := cfg.SetToken("acme", "ghp_secret")

	if !usedKeyring {
		t.Error("expected usedKeyring = true")
	}
	if got := cfg.TokenForOwner("acme"); got != "ghp_secret" {
		t.Errorf("TokenForOwner() = %q, want ghp_secret", got)
	}
	if got := cfg.TokenSourceForOwner("acme"); got != "keyring" {
		t.Errorf("TokenSourceForOwner() = %q, want keyring", got)
	}
	// The plaintext field itself must be empty — the whole point of the feature.
	if org := cfg.orgData["acme"]; org.Token != "" {
		t.Errorf("plaintext Token field = %q, want empty when stored in keyring", org.Token)
	}
}

func TestSetToken_FallsBackToPlaintextWhenKeyringUnavailable(t *testing.T) {
	fake := withFakeKeyring(t)
	fake.Unavailable = errors.New("no keyring backend")
	cfg := &Config{}
	cfg.AddOrganization("acme")

	usedKeyring := cfg.SetToken("acme", "ghp_secret")

	if usedKeyring {
		t.Error("expected usedKeyring = false")
	}
	if got := cfg.TokenForOwner("acme"); got != "ghp_secret" {
		t.Errorf("TokenForOwner() = %q, want ghp_secret (plaintext fallback)", got)
	}
	if got := cfg.TokenSourceForOwner("acme"); got != "plaintext" {
		t.Errorf("TokenSourceForOwner() = %q, want plaintext", got)
	}
}

func TestTokenForOwner_KeyringEntryGoneDespiteSource(t *testing.T) {
	withFakeKeyring(t)
	cfg := &Config{}
	cfg.AddOrganization("acme")
	cfg.SetToken("acme", "ghp_secret")

	// Simulate the keyring entry disappearing after it was set (e.g. the user cleared
	// it directly in Keychain Access) without sgh knowing.
	TokenKeyring.Delete("acme")

	if got := cfg.TokenForOwner("acme"); got != "" {
		t.Errorf("TokenForOwner() = %q, want empty when the keyring entry is gone", got)
	}
	// HasToken must agree, not just TokenForOwner.
	if cfg.HasToken("acme") {
		t.Error("HasToken() = true, want false when the keyring entry is gone")
	}
}

func TestHasToken(t *testing.T) {
	withFakeKeyring(t)
	cfg := &Config{}
	cfg.AddOrganization("acme")

	if cfg.HasToken("acme") {
		t.Error("expected no token yet")
	}
	cfg.SetToken("acme", "ghp_secret")
	if !cfg.HasToken("acme") {
		t.Error("expected a token after SetToken")
	}
	if cfg.HasToken(nonExistentOrg) {
		t.Error("expected false for a non-existent org")
	}
}

func TestTokenSourceForOwner(t *testing.T) {
	withFakeKeyring(t)
	// "plain" is built directly (not via AddOrganization+SetToken) to simulate a legacy
	// entry loaded from disk with a plaintext token, bypassing the keyring entirely.
	cfg := &Config{Organizations: []Organization{
		{Name: "plain", Token: "ghp_x"},
		{Name: "none"},
	}}
	cfg.rebuildOrgData()
	cfg.AddOrganization("keyed")
	cfg.SetToken("keyed", "ghp_y")

	cases := map[string]string{"plain": "plaintext", "keyed": "keyring", "none": "", nonExistentOrg: ""}
	for org, want := range cases {
		if got := cfg.TokenSourceForOwner(org); got != want {
			t.Errorf("TokenSourceForOwner(%q) = %q, want %q", org, got, want)
		}
	}
}

func TestRemoveToken(t *testing.T) {
	withFakeKeyring(t)
	cfg := &Config{}
	cfg.AddOrganization("acme")
	cfg.SetToken("acme", "ghp_secret")

	removed, err := cfg.RemoveToken("acme")
	if err != nil {
		t.Fatalf("RemoveToken(): %v", err)
	}
	if !removed {
		t.Error("expected removed = true")
	}
	if cfg.HasToken("acme") {
		t.Error("expected no token after RemoveToken")
	}
	if _, ok, _ := TokenKeyring.Get("acme"); ok {
		t.Error("expected the keyring entry itself to be gone")
	}

	// Removing an org with no token, or a non-existent org, is a no-op, not an error —
	// and removed must say so, so the CLI doesn't print a false "removed" message.
	if removed, err := cfg.RemoveToken("acme"); err != nil || removed {
		t.Errorf("RemoveToken() on an already-tokenless org = (%v, %v), want (false, nil)", removed, err)
	}
	if removed, err := cfg.RemoveToken(nonExistentOrg); err != nil || removed {
		t.Errorf("RemoveToken() on a non-existent org = (%v, %v), want (false, nil)", removed, err)
	}
}

func TestRemoveToken_KeyringDeleteFailurePropagates(t *testing.T) {
	fake := withFakeKeyring(t)
	cfg := &Config{}
	cfg.AddOrganization("acme")
	cfg.SetToken("acme", "ghp_secret")

	fake.Unavailable = errors.New("keyring locked")
	_, err := cfg.RemoveToken("acme")

	if err == nil {
		t.Fatal("expected an error when the keyring delete fails")
	}
	// A failed removal must not corrupt the config's own bookkeeping. Checking via
	// HasToken/TokenForOwner here would conflate two different failures (the keyring is
	// down for *every* call right now, not just Delete) — check the field directly, then
	// confirm the entry is genuinely untouched by restoring the keyring and reading it back.
	org := cfg.orgData["acme"]
	if org.TokenSource != TokenSourceKeyring {
		t.Errorf("TokenSource = %q, want unchanged %q after a failed removal", org.TokenSource, TokenSourceKeyring)
	}
	fake.Unavailable = nil
	if got := cfg.TokenForOwner("acme"); got != "ghp_secret" {
		t.Errorf("TokenForOwner() = %q, want ghp_secret — the keyring entry itself must be untouched", got)
	}
}

func TestRemoveToken_PlaintextOrgNeedsNoKeyringCall(t *testing.T) {
	fake := withFakeKeyring(t)
	// Built directly (not via AddOrganization+SetToken) to simulate a legacy plaintext
	// entry loaded from disk, bypassing the keyring entirely.
	cfg := &Config{Organizations: []Organization{{Name: "acme", Token: "ghp_plain"}}}
	cfg.rebuildOrgData()

	fake.Unavailable = errors.New("keyring locked") // must not matter: nothing to delete there

	removed, err := cfg.RemoveToken("acme")
	if err != nil {
		t.Fatalf("RemoveToken() on a plaintext-only org: %v, want nil", err)
	}
	if !removed {
		t.Error("expected removed = true")
	}
	if cfg.HasToken("acme") {
		t.Error("expected no token after RemoveToken")
	}
}

// --- migration (Init) ---------------------------------------------------

func TestInit_MigratesPlaintextTokenToKeyring(t *testing.T) {
	withIsolatedHome(t)
	withFakeKeyring(t)

	onDisk := &Config{Organizations: []Organization{{Name: "acme", Token: "ghp_legacy"}}}
	data, err := json.MarshalIndent(onDisk, "", "    ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(configFile(), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Init()
	if err != nil {
		t.Fatalf("Init(): %v", err)
	}

	if got := cfg.TokenForOwner("acme"); got != "ghp_legacy" {
		t.Errorf("TokenForOwner() = %q, want ghp_legacy (via keyring, post-migration)", got)
	}
	if got := cfg.TokenSourceForOwner("acme"); got != "keyring" {
		t.Errorf("TokenSourceForOwner() = %q, want keyring after migration", got)
	}
	if len(cfg.MigratedTokens) != 1 || cfg.MigratedTokens[0] != "acme" {
		t.Errorf("MigratedTokens = %v, want [acme]", cfg.MigratedTokens)
	}

	// Migration must persist: the file on disk no longer holds the plaintext token.
	reloaded := &Config{}
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Organizations[0].Token != "" {
		t.Errorf("reloaded plaintext Token = %q, want empty (migration must be saved)", reloaded.Organizations[0].Token)
	}
	if reloaded.Organizations[0].TokenSource != TokenSourceKeyring {
		t.Errorf("reloaded TokenSource = %q, want %q", reloaded.Organizations[0].TokenSource, TokenSourceKeyring)
	}
}

func TestInit_MigrationSkippedWhenKeyringUnavailable(t *testing.T) {
	withIsolatedHome(t)
	fake := withFakeKeyring(t)
	fake.Unavailable = errors.New("no keyring backend")

	onDisk := &Config{Organizations: []Organization{{Name: "acme", Token: "ghp_legacy"}}}
	data, _ := json.MarshalIndent(onDisk, "", "    ")
	if err := os.WriteFile(configFile(), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Init()
	if err != nil {
		t.Fatalf("Init(): %v", err)
	}

	if len(cfg.MigratedTokens) != 0 {
		t.Errorf("MigratedTokens = %v, want none when the keyring is unavailable", cfg.MigratedTokens)
	}
	if got := cfg.TokenForOwner("acme"); got != "ghp_legacy" {
		t.Errorf("TokenForOwner() = %q, want the token to still work via the untouched plaintext field", got)
	}
	if got := cfg.TokenSourceForOwner("acme"); got != "plaintext" {
		t.Errorf("TokenSourceForOwner() = %q, want plaintext (unmigrated)", got)
	}
}

func TestInit_AlreadyMigratedOrgIsNotReMigrated(t *testing.T) {
	withIsolatedHome(t)
	withFakeKeyring(t)

	onDisk := &Config{Organizations: []Organization{{Name: "acme", TokenSource: TokenSourceKeyring}}}
	data, _ := json.MarshalIndent(onDisk, "", "    ")
	if err := os.WriteFile(configFile(), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Init()
	if err != nil {
		t.Fatalf("Init(): %v", err)
	}

	if len(cfg.MigratedTokens) != 0 {
		t.Errorf("MigratedTokens = %v, want none — nothing to migrate", cfg.MigratedTokens)
	}
}

func TestInit_NoTokensNoMigration(t *testing.T) {
	withIsolatedHome(t)
	withFakeKeyring(t)

	onDisk := &Config{Organizations: []Organization{{Name: "acme"}}}
	data, _ := json.MarshalIndent(onDisk, "", "    ")
	if err := os.WriteFile(configFile(), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Init()
	if err != nil {
		t.Fatalf("Init(): %v", err)
	}
	if len(cfg.MigratedTokens) != 0 {
		t.Errorf("MigratedTokens = %v, want none", cfg.MigratedTokens)
	}
}

// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

// Package keyring stores and retrieves per-owner GitHub tokens in the OS keyring
// (macOS Keychain, Windows Credential Manager, Linux Secret Service), so callers never
// need to hold or persist the token themselves beyond this package's methods.
package keyring

import (
	"errors"

	zkeyring "github.com/zalando/go-keyring"
)

// service is the keyring service name every entry is stored under; owner (the org/user
// name) is the per-entry account name within that service.
const service = "sgh"

// Keyring stores and retrieves per-owner secrets. Get's ok return distinguishes "no
// entry for this owner" (ok=false, err=nil) from a genuine access failure (err!=nil,
// e.g. no keyring backend on headless Linux) — callers use err to decide whether to
// fall back to another storage method, not to distinguish "not found".
type Keyring interface {
	Set(owner, token string) error
	Get(owner string) (token string, ok bool, err error)
	Delete(owner string) error
}

// osKeyring is the real implementation, backed by the OS keyring via zalando/go-keyring.
type osKeyring struct{}

// New returns a Keyring backed by the current OS's native keyring.
func New() Keyring { return osKeyring{} }

func (osKeyring) Set(owner, token string) error {
	return zkeyring.Set(service, owner, token)
}

func (osKeyring) Get(owner string) (string, bool, error) {
	token, err := zkeyring.Get(service, owner)
	if errors.Is(err, zkeyring.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return token, true, nil
}

func (osKeyring) Delete(owner string) error {
	err := zkeyring.Delete(service, owner)
	if errors.Is(err, zkeyring.ErrNotFound) {
		return nil // already gone is not a failure
	}
	return err
}

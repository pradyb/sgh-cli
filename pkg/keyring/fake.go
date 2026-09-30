// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package keyring

import "sync"

// Fake is an in-memory Keyring for tests. Set Unavailable to simulate a keyring that
// can't be reached at all (e.g. headless Linux with no Secret Service) — every method
// then returns Unavailable as its error, regardless of what's already stored.
type Fake struct {
	mu          sync.Mutex
	data        map[string]string
	Unavailable error
}

// NewFake returns an empty Fake, ready to use.
func NewFake() *Fake {
	return &Fake{data: make(map[string]string)}
}

func (f *Fake) Set(owner, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Unavailable != nil {
		return f.Unavailable
	}
	f.data[owner] = token
	return nil
}

func (f *Fake) Get(owner string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Unavailable != nil {
		return "", false, f.Unavailable
	}
	token, ok := f.data[owner]
	return token, ok, nil
}

func (f *Fake) Delete(owner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Unavailable != nil {
		return f.Unavailable
	}
	delete(f.data, owner)
	return nil
}

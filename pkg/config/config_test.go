// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	internalconfig "github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// isolateHome points the OS home directory lookup at a fresh temp dir so
// Save() never touches the real user's sgh config.
func isolateHome(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tempDir)
	} else {
		t.Setenv("HOME", tempDir)
	}
}

func newTestContext() *context.Context {
	return &context.Context{Config: &internalconfig.Config{}}
}

func TestAddOrganization(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()

	alreadyPresent := AddOrganization(ctx, "acme")
	if alreadyPresent {
		t.Fatal("expected first add to report not-already-present")
	}
	if !ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected organization to be added")
	}

	if data, err := os.ReadFile(ConfigFilePath()); err != nil || len(data) == 0 {
		t.Errorf("expected config to be saved to disk, err=%v", err)
	}

	alreadyPresent = AddOrganization(ctx, "acme")
	if !alreadyPresent {
		t.Error("expected second add of the same org to report already-present")
	}
}

func TestAddRepository(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	if AddRepository(ctx, "acme", "widgets") {
		t.Fatal("expected first add to report not-already-present")
	}
	if !ctx.Config.IsRepositoryPresent("acme", "widgets") {
		t.Error("expected repository to be added")
	}
	if !AddRepository(ctx, "acme", "widgets") {
		t.Error("expected second add of the same repo to report already-present")
	}
}

func TestAddPullRequestAssignee(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	if AddPullRequestAssignee(ctx, "acme", "jane-doe") {
		t.Fatal("expected first add to report not-already-present")
	}
	if !ctx.Config.IsPullRequestAssigneePresent("acme", "jane-doe") {
		t.Error("expected assignee to be added")
	}
	if !AddPullRequestAssignee(ctx, "acme", "jane-doe") {
		t.Error("expected second add of the same assignee to report already-present")
	}
}

func TestAddRepositoryPattern(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	if AddRepositoryPattern(ctx, "acme", true, false, "^widget-.*") {
		t.Fatal("expected first add to report not-already-present")
	}
	if !ctx.Config.IsRepositoryPatternPresent("acme", "^widget-.*", true) {
		t.Error("expected include pattern to be added")
	}
	if !AddRepositoryPattern(ctx, "acme", true, false, "^widget-.*") {
		t.Error("expected second add of the same pattern to report already-present")
	}
}

func TestSaveRepositoryNamesForFuzzySearch(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()

	SaveRepositoryNamesForFuzzySearch(ctx, "acme", []string{"widgets", "gadgets"})

	if !ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected organization to be added")
	}
	if !ctx.Config.IsRepositoryPresent("acme", "widgets") || !ctx.Config.IsRepositoryPresent("acme", "gadgets") {
		t.Error("expected both repositories to be added")
	}
}

func TestSetToken(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	SetToken(ctx, "acme", "ghp_secret")

	if got := ctx.Config.TokenForOwner("acme"); got != "ghp_secret" {
		t.Errorf("TokenForOwner() = %q, want %q", got, "ghp_secret")
	}
}

func TestSetOwnerType(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	SetOwnerType(ctx, "acme", "user")

	if got := ctx.Config.OwnerTypeFor("acme"); got != "user" {
		t.Errorf("OwnerTypeFor() = %q, want %q", got, "user")
	}
}

func TestSetTaggerName(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	SetTaggerName(ctx, "acme", "Jane Doe")

	if got := ctx.Config.TaggerName("acme"); got != "Jane Doe" {
		t.Errorf("TaggerName() = %q, want %q", got, "Jane Doe")
	}
}

func TestSetTaggerEmail(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	SetTaggerEmail(ctx, "acme", "jane@example.com")

	if got := ctx.Config.TaggerEmail("acme"); got != "jane@example.com" {
		t.Errorf("TaggerEmail() = %q, want %q", got, "jane@example.com")
	}
}

func TestRemoveOrganization(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")

	RemoveOrganization(ctx, "acme")
	if ctx.Config.IsOrganizationPresent("acme") {
		t.Error("expected organization to be removed")
	}

	// Removing again should be a safe no-op (not-found branch).
	RemoveOrganization(ctx, "acme")
}

func TestRemoveRepository(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddRepository("acme", "widgets")

	RemoveRepository(ctx, "acme", "widgets")
	if ctx.Config.IsRepositoryPresent("acme", "widgets") {
		t.Error("expected repository to be removed")
	}

	// Not-found branch.
	RemoveRepository(ctx, "acme", "widgets")
}

func TestRemovePullRequestAssignee(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddPullRequestAssignee("acme", "jane-doe")

	RemovePullRequestAssignee(ctx, "acme", "jane-doe")
	if ctx.Config.IsPullRequestAssigneePresent("acme", "jane-doe") {
		t.Error("expected assignee to be removed")
	}

	// Not-found branch.
	RemovePullRequestAssignee(ctx, "acme", "jane-doe")
}

func TestRemoveRepositoryPattern(t *testing.T) {
	isolateHome(t)
	ctx := newTestContext()
	ctx.Config.AddOrganization("acme")
	ctx.Config.AddRepositoryPattern("acme", true, false, "^widget-.*")
	ctx.Config.AddRepositoryPattern("acme", false, true, "^legacy-.*")

	RemoveRepositoryPattern(ctx, "acme", true, "^widget-.*")
	if ctx.Config.IsRepositoryPatternPresent("acme", "^widget-.*", true) {
		t.Error("expected include pattern to be removed")
	}

	RemoveRepositoryPattern(ctx, "acme", false, "^legacy-.*")
	if ctx.Config.IsRepositoryPatternPresent("acme", "^legacy-.*", false) {
		t.Error("expected exclude pattern to be removed")
	}

	// Not-found branches for both kinds.
	RemoveRepositoryPattern(ctx, "acme", true, "^widget-.*")
	RemoveRepositoryPattern(ctx, "acme", false, "^legacy-.*")
}

func TestConfigFilePath(t *testing.T) {
	isolateHome(t)

	got := ConfigFilePath()
	if filepath.Base(got) != internalconfig.DefaultFilename {
		t.Errorf("ConfigFilePath() = %q, want base name %q", got, internalconfig.DefaultFilename)
	}
}

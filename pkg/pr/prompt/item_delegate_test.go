// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package prompt

import (
	"net/http"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func testDelegatePR() model.PullRequestResponse {
	return model.PullRequestResponse{
		PRNumber:  42,
		TitleName: "Fix bug",
		State:     "OPEN",
		HTMLUrl:   "https://github.com/testorg/test-repo/pull/42",
		Base:      model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
		Head:      model.PRBranch{Ref: "feature", Sha: "abcdef1"},
	}
}

func TestNewDelegateKeyMap(t *testing.T) {
	keys := newDelegateKeyMap()

	tests := []struct {
		name    string
		binding []string
		want    string
	}{
		{"status", keys.status.Keys(), "s"},
		{"approve", keys.approve.Keys(), "A"},
		{"merge", keys.merge.Keys(), "m"},
		{"approveMerge", keys.approveMerge.Keys(), "M"},
		{"closePR", keys.closePR.Keys(), "c"},
		{"openBrowser", keys.openBrowser.Keys(), "o"},
		{"diff", keys.diff.Keys(), "d"},
	}
	for _, tt := range tests {
		if len(tt.binding) != 1 || tt.binding[0] != tt.want {
			t.Errorf("%s keys = %v, want [%s]", tt.name, tt.binding, tt.want)
		}
	}
}

func TestNewItemDelegate_HelpFuncs(t *testing.T) {
	keys := newDelegateKeyMap()
	d := newItemDelegate(nil, "testorg", keys)

	short := d.ShortHelpFunc()
	if len(short) != 7 {
		t.Fatalf("ShortHelpFunc() len = %d, want 7", len(short))
	}

	full := d.FullHelpFunc()
	if len(full) != 1 || len(full[0]) != 7 {
		t.Fatalf("FullHelpFunc() = %+v, want a single group of 7", full)
	}

	if !d.Styles.SelectedTitle.GetBold() {
		t.Error("expected SelectedTitle style to be bold")
	}
	if !d.Styles.SelectedDesc.GetItalic() {
		t.Error("expected SelectedDesc style to be italic")
	}
}

func TestItemDelegate_UpdateFunc_NoSelection(t *testing.T) {
	keys := newDelegateKeyMap()
	d := newItemDelegate(nil, "testorg", keys)
	l := list.New([]list.Item{}, d, 80, 24)

	cmd := d.UpdateFunc(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}, &l)
	if cmd != nil {
		t.Error("expected nil cmd when no item is selected")
	}
}

func TestItemDelegate_UpdateFunc_NonKeyMsg(t *testing.T) {
	keys := newDelegateKeyMap()
	d := newItemDelegate(nil, "testorg", keys)
	l := list.New([]list.Item{testDelegatePR()}, d, 80, 24)

	cmd := d.UpdateFunc(tea.WindowSizeMsg{Width: 80, Height: 24}, &l)
	if cmd != nil {
		t.Error("expected nil cmd for a non-KeyMsg message")
	}
}

func TestItemDelegate_UpdateFunc_UnmatchedKey(t *testing.T) {
	keys := newDelegateKeyMap()
	d := newItemDelegate(nil, "testorg", keys)
	l := list.New([]list.Item{testDelegatePR()}, d, 80, 24)

	cmd := d.UpdateFunc(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")}, &l)
	if cmd != nil {
		t.Error("expected nil cmd for an unmatched key")
	}
}

func TestItemDelegate_UpdateFunc_Status(t *testing.T) {
	keys := newDelegateKeyMap()
	d := newItemDelegate(nil, "testorg", keys)
	l := list.New([]list.Item{testDelegatePR()}, d, 80, 24)

	cmd := d.UpdateFunc(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}, &l)
	if cmd == nil {
		t.Fatal("expected a non-nil cmd")
	}
	msg, ok := cmd().(eventMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want eventMsg", msg)
	}
	if msg.eventType != "STATUS" || msg.orgName != "testorg" || msg.repoName != "test-repo" {
		t.Errorf("unexpected eventMsg: %+v", msg)
	}
}

func TestItemDelegate_UpdateFunc_ConfirmActions(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantEvent  string
		wantPrefix string
	}{
		{"approve", "A", "APPROVE", "Approve PR"},
		{"merge", "m", "MERGE", "Merge PR"},
		{"approveMerge", "M", "APPROVE_MERGE", "Approve and Merge PR"},
		{"close", "c", "CLOSE", "Close PR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := newDelegateKeyMap()
			d := newItemDelegate(nil, "testorg", keys)
			l := list.New([]list.Item{testDelegatePR()}, d, 80, 24)

			cmd := d.UpdateFunc(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}, &l)
			if cmd == nil {
				t.Fatal("expected a non-nil cmd")
			}
			msg, ok := cmd().(confirmMsg)
			if !ok {
				t.Fatalf("cmd() = %T, want confirmMsg", msg)
			}
			if msg.eventType != tt.wantEvent {
				t.Errorf("eventType = %q, want %q", msg.eventType, tt.wantEvent)
			}
			if msg.orgName != "testorg" || msg.repoName != "test-repo" {
				t.Errorf("unexpected org/repo: %+v", msg)
			}
			if len(msg.prompt) == 0 || msg.prompt[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Errorf("prompt = %q, want prefix %q", msg.prompt, tt.wantPrefix)
			}
		})
	}
}

func TestItemDelegate_UpdateFunc_OpenBrowser(t *testing.T) {
	keys := newDelegateKeyMap()
	d := newItemDelegate(nil, "testorg", keys)
	l := list.New([]list.Item{testDelegatePR()}, d, 80, 24)

	cmd := d.UpdateFunc(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")}, &l)
	if cmd == nil {
		t.Fatal("expected a non-nil cmd")
	}
	msg, ok := cmd().(browserOpenMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want browserOpenMsg", msg)
	}
	if msg.url != "https://github.com/testorg/test-repo/pull/42" {
		t.Errorf("url = %q", msg.url)
	}
}

func TestItemDelegate_UpdateFunc_Diff(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/42/files", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: []map[string]interface{}{
			{"filename": "main.go", "additions": 1, "deletions": 0, "status": "modified", "patch": "@@ -1,2 +1,3 @@\n+line"},
		},
	})
	ctx := service.NewMockContext(t, mockServer)

	keys := newDelegateKeyMap()
	d := newItemDelegate(ctx, "testorg", keys)
	l := list.New([]list.Item{testDelegatePR()}, d, 80, 24)

	cmd := d.UpdateFunc(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}, &l)
	if cmd == nil {
		t.Fatal("expected a non-nil cmd")
	}
	msg, ok := cmd().(diffLoadedMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want diffLoadedMsg", msg)
	}
	if msg.title == "" {
		t.Error("expected a non-empty diff title")
	}
	if len(msg.lines) == 0 {
		t.Error("expected non-empty diff lines")
	}
}

// Note: openURL wraps ui.OpenURL, which spawns a real OS command to launch
// the default browser. It's intentionally not exercised here to avoid
// popping a browser window during `go test`.

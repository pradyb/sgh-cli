// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package prompt

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/internal/testutils"
	"github.com/pradyb/sgh-cli/pkg/pr"
)

// --- test helpers -----------------------------------------------------

// newRawModel builds a prModel directly, without going through newModel (and
// therefore without any network access), so pure View/Update logic can be
// exercised in isolation.
func newRawModel(items []list.Item) prModel {
	delegateKeys := newDelegateKeyMap()
	listKeys := newListKeyMap()
	delegate := newItemDelegate(nil, "testorg", delegateKeys)
	l := list.New(items, delegate, 80, 24)
	l.Title = "Interactive Pull Request Options"
	l.SetShowHelp(false)

	s := spinner.New()
	s.Spinner = spinner.Points

	return prModel{
		list:         l,
		keys:         listKeys,
		delegateKeys: delegateKeys,
		spinner:      s,
		prRequest:    pr.PRRequest{OrgName: "testorg"},
		termWidth:    80,
		termHeight:   24,
	}
}

func samplePR() model.PullRequestResponse {
	return model.PullRequestResponse{
		PRNumber:  100,
		TitleName: "Add feature",
		State:     "OPEN",
		Mergeable: "MERGEABLE",
		HTMLUrl:   "https://github.com/testorg/test-repo/pull/100",
		Base:      model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
		Head:      model.PRBranch{Ref: "feature", Sha: "abcdef1234567"},
		Author:    model.User{Login: "author1", Name: "Author One"},
	}
}

// firstOfType pulls a message of type T out of a tea.Cmd's result, unwrapping
// a tea.BatchMsg if necessary (Update often batches the list/spinner cmds
// alongside the one we care about).
func firstOfType[T any](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a non-nil cmd")
	}
	return extractType[T](t, cmd())
}

func extractType[T any](t *testing.T, msg tea.Msg) T {
	t.Helper()
	if v, ok := msg.(T); ok {
		return v
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if v, ok := c().(T); ok {
				return v
			}
		}
	}
	var zero T
	t.Fatalf("could not find message of type %T in %#v", zero, msg)
	return zero
}

// prDetailGraphQLBody builds a minimal but well-formed GetPRDetailsGraphQL
// mock response body.
func prDetailGraphQLBody(number int, state, mergeable, mergeStateStatus string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"organization": map[string]interface{}{
				"repository": map[string]interface{}{
					"pullRequest": map[string]interface{}{
						"number":           number,
						"title":            "Add feature",
						"body":             "description",
						"url":              fmt.Sprintf("https://github.com/testorg/test-repo/pull/%d", number),
						"baseRef":          map[string]interface{}{"name": "main", "repository": map[string]interface{}{"name": "test-repo"}},
						"headRef":          map[string]interface{}{"name": "feature", "repository": map[string]interface{}{"name": "test-repo"}},
						"headRefOid":       "abc123",
						"reviewDecision":   "",
						"state":            state,
						"mergeable":        mergeable,
						"mergeStateStatus": mergeStateStatus,
						"createdAt":        "2024-01-01T00:00:00Z",
						"updatedAt":        "2024-01-02T00:00:00Z",
						"mergedAt":         "",
						"mergedBy":         map[string]interface{}{"login": "", "name": ""},
						"author":           map[string]interface{}{"login": "author1", "name": "Author One"},
						"reviewRequests":   map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
						"assignees":        map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
						"labels":           map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
						"comments":         map[string]interface{}{"totalCount": 0},
						"commits":          map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
						"additions":        1,
						"deletions":        1,
						"changedFiles":     1,
						"files":            map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
						"reviews":          map[string]interface{}{"totalCount": 0, "edges": []map[string]interface{}{}},
					},
				},
			},
		},
	}
}

func emptySearchGraphQLBody() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"issueCount": 0,
				"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
				"edges":      []map[string]interface{}{},
			},
		},
	}
}

// --- newListKeyMap / newModel / resizeList -----------------------------

func TestNewListKeyMap(t *testing.T) {
	keys := newListKeyMap()
	if got := keys.refresh.Keys(); len(got) != 1 || got[0] != "r" {
		t.Errorf("refresh keys = %v, want [r]", got)
	}
	if keys.refresh.Help().Key != "r" || keys.refresh.Help().Desc != "refresh pr list" {
		t.Errorf("unexpected help: %+v", keys.refresh.Help())
	}
}

func TestNewModel_Empty(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody(),
	})
	ctx := service.NewMockContext(t, mockServer)
	req := pr.PRRequest{OrgName: "testorg", LastCount: 10}

	m := newModel(ctx, req)

	if len(m.list.Items()) != 0 {
		t.Errorf("expected no items, got %d", len(m.list.Items()))
	}
	if m.keys == nil || m.delegateKeys == nil {
		t.Error("expected key maps to be initialized")
	}
	if m.showEventPanel || m.showSpinner {
		t.Error("expected showEventPanel/showSpinner to start false")
	}
	if m.ctx != ctx {
		t.Error("expected ctx to be stored on the model")
	}
	if m.prRequest.OrgName != "testorg" {
		t.Errorf("unexpected prRequest: %+v", m.prRequest)
	}
	if m.termWidth == 0 {
		t.Error("expected a non-zero termWidth")
	}
	if m.list.Title != "Interactive Pull Request Options" {
		t.Errorf("Title = %q", m.list.Title)
	}
}

func TestNewModel_WithItems(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body: map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"issueCount": 1,
					"pageInfo":   map[string]interface{}{"endCursor": "", "hasNextPage": false},
					"edges": []map[string]interface{}{
						{
							"node": map[string]interface{}{
								"number":           42,
								"title":            "GraphQL PR",
								"url":              "https://github.com/testorg/test-repo/pull/42",
								"baseRef":          map[string]interface{}{"name": "main", "repository": map[string]interface{}{"name": "test-repo"}},
								"headRef":          map[string]interface{}{"name": "feature", "repository": map[string]interface{}{"name": "test-repo"}},
								"state":            "OPEN",
								"mergeStateStatus": "CLEAN",
								"author":           map[string]interface{}{"login": "jdoe", "name": "J Doe"},
							},
						},
					},
				},
			},
		},
	})
	ctx := service.NewMockContext(t, mockServer)

	m := newModel(ctx, pr.PRRequest{OrgName: "testorg", LastCount: 10})

	if len(m.list.Items()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(m.list.Items()))
	}
}

func TestResizeList(t *testing.T) {
	m := newRawModel(nil)

	m.termWidth = 0
	m.resizeList()
	// No panic and dimensions unchanged is the contract for the zero-width guard.

	m.termWidth = 100
	m.termHeight = 40
	m.showEventPanel = false
	m.resizeList()
	fullWidth := m.list.Width()

	m.showEventPanel = true
	m.resizeList()
	panelWidth := m.list.Width()

	if panelWidth >= fullWidth {
		t.Errorf("expected list width to shrink when the event panel is shown: full=%d panel=%d", fullWidth, panelWidth)
	}
}

// --- Init / helpView / View ---------------------------------------------

func TestPRModel_Init(t *testing.T) {
	m := newRawModel(nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected a non-nil cmd from Init")
	}
	if _, ok := cmd().(spinner.TickMsg); !ok {
		t.Errorf("Init() cmd produced %T, want spinner.TickMsg", cmd())
	}
}

func TestPRModel_HelpView(t *testing.T) {
	m := newRawModel(nil)
	out := m.helpView()
	if out == "" {
		t.Fatal("expected a non-empty help view")
	}
}

func TestPRModel_View_List(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	out := m.View()
	if out == "" {
		t.Fatal("expected a non-empty view")
	}
}

func TestPRModel_View_ConfirmPending(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	m.confirmPending = &confirmMsg{prompt: "Approve PR Add feature? (y/n)"}
	out := m.View()
	if !strings.Contains(out, "Approve PR Add feature? (y/n)") {
		t.Errorf("expected confirm prompt in view, got: %s", out)
	}
}

func TestPRModel_View_EventPanelSpinner(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	m.showEventPanel = true
	m.showSpinner = true
	out := m.View()
	if !strings.Contains(out, "Processing") {
		t.Errorf("expected a processing indicator, got: %s", out)
	}
}

func TestPRModel_View_EventPanelSections(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	m.showEventPanel = true
	m.showSpinner = false
	m.sections = []string{"section-one", "section-two"}
	out := m.View()
	if !strings.Contains(out, "section-one") || !strings.Contains(out, "section-two") {
		t.Errorf("expected sections in view, got: %s", out)
	}
}

func TestPRModel_View_NarrowTerminal(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	m.termWidth = 40
	m.termHeight = 20
	m.showEventPanel = true
	out := m.View()
	if out == "" {
		t.Fatal("expected a non-empty view even at narrow width (min-width clamps)")
	}
}

func TestPRModel_View_Diff(t *testing.T) {
	m := newRawModel(nil)
	m.showDiff = true
	m.diffTitle = "PR #1 · title"
	m.diffLines = []string{"@@ -1,2 +1,3 @@", "+added", "-removed", "context"}
	out := m.View()
	if !strings.Contains(out, "Diff: PR #1") {
		t.Errorf("expected diff overlay in view, got: %s", out)
	}
}

// --- Update: diff overlay ------------------------------------------------

func diffModel(scroll int, lineCount, termHeight int) prModel {
	m := newRawModel(nil)
	m.showDiff = true
	m.diffTitle = "diff"
	lines := make([]string, lineCount)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	m.diffLines = lines
	m.diffScroll = scroll
	m.termHeight = termHeight
	return m
}

func TestUpdate_DiffOverlay_CloseKeys(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		m := diffModel(5, 30, 30) // visible = 20
		var keyMsg tea.KeyMsg
		if k == "esc" {
			keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
		} else {
			keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, cmd := m.Update(keyMsg)
		nm := next.(prModel)
		if nm.showDiff {
			t.Errorf("key %q: expected showDiff to become false", k)
		}
		if nm.diffLines != nil {
			t.Errorf("key %q: expected diffLines to be cleared", k)
		}
		if cmd != nil {
			t.Errorf("key %q: expected nil cmd", k)
		}
	}
}

func TestUpdate_DiffOverlay_ScrollDown(t *testing.T) {
	m := diffModel(0, 30, 30) // visible = 20, max scroll = 10
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	nm := next.(prModel)
	if nm.diffScroll != 1 {
		t.Errorf("diffScroll = %d, want 1", nm.diffScroll)
	}

	m2 := diffModel(10, 30, 30) // already at max
	next2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	nm2 := next2.(prModel)
	if nm2.diffScroll != 10 {
		t.Errorf("diffScroll = %d, want unchanged 10", nm2.diffScroll)
	}
}

func TestUpdate_DiffOverlay_ScrollUp(t *testing.T) {
	m := diffModel(5, 30, 30)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	nm := next.(prModel)
	if nm.diffScroll != 4 {
		t.Errorf("diffScroll = %d, want 4", nm.diffScroll)
	}

	m2 := diffModel(0, 30, 30)
	next2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	nm2 := next2.(prModel)
	if nm2.diffScroll != 0 {
		t.Errorf("diffScroll = %d, want unchanged 0", nm2.diffScroll)
	}
}

func TestUpdate_DiffOverlay_PageDown(t *testing.T) {
	m := diffModel(0, 30, 30) // visible=20, max scroll=10
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	nm := next.(prModel)
	if nm.diffScroll != 10 {
		t.Errorf("diffScroll = %d, want clamped to 10", nm.diffScroll)
	}

	m2 := diffModel(0, 5, 30) // visible=20 > len=5, negative clamp to 0
	next2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	nm2 := next2.(prModel)
	if nm2.diffScroll != 0 {
		t.Errorf("diffScroll = %d, want clamped to 0", nm2.diffScroll)
	}
}

func TestUpdate_DiffOverlay_PageUp(t *testing.T) {
	m := diffModel(15, 30, 30)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	nm := next.(prModel)
	if nm.diffScroll != 0 {
		t.Errorf("diffScroll = %d, want clamped to 0 (15-20)", nm.diffScroll)
	}

	m2 := diffModel(25, 30, 30)
	next2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	nm2 := next2.(prModel)
	if nm2.diffScroll != 5 {
		t.Errorf("diffScroll = %d, want 5 (25-20)", nm2.diffScroll)
	}
}

func TestUpdate_DiffOverlay_GoToTopBottom(t *testing.T) {
	m := diffModel(5, 30, 30)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if next.(prModel).diffScroll != 0 {
		t.Errorf("expected scroll reset to 0 on 'g'")
	}

	m2 := diffModel(0, 30, 30)
	next2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if next2.(prModel).diffScroll != 10 {
		t.Errorf("diffScroll = %d, want 10 on 'G'", next2.(prModel).diffScroll)
	}

	m3 := diffModel(0, 5, 30) // len < visible, clamp negative to 0
	next3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if next3.(prModel).diffScroll != 0 {
		t.Errorf("diffScroll = %d, want 0 when content shorter than viewport", next3.(prModel).diffScroll)
	}
}

func TestUpdate_DiffOverlay_UnknownKey(t *testing.T) {
	m := diffModel(5, 30, 30)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	if next.(prModel).diffScroll != 5 {
		t.Error("expected scroll to remain unchanged for an unrecognized key")
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdate_DiffOverlay_WindowSize(t *testing.T) {
	m := diffModel(5, 30, 30)
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	nm := next.(prModel)
	if nm.termWidth != 120 || nm.termHeight != 50 {
		t.Errorf("unexpected term size: %d x %d", nm.termWidth, nm.termHeight)
	}
	if !nm.showDiff {
		t.Error("expected showDiff to remain true")
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

// --- Update: confirmation prompt -----------------------------------------

func TestUpdate_Confirm_Yes(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	pending := &confirmMsg{
		eventType:  "MERGE",
		orgName:    "testorg",
		repoName:   "test-repo",
		selectedPR: samplePR(),
		prompt:     "Merge PR Add feature? (y/n)",
	}
	m.confirmPending = pending

	for _, k := range []string{"y", "Y"} {
		mm := m
		mm.confirmPending = pending
		next, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		nm := next.(prModel)
		if nm.confirmPending != nil {
			t.Errorf("key %q: expected confirmPending to be cleared", k)
		}
		if !nm.showEventPanel || !nm.showSpinner {
			t.Errorf("key %q: expected event panel + spinner to be shown", k)
		}
		em := firstOfType[eventMsg](t, cmd)
		if em.eventType != "MERGE" || em.orgName != "testorg" || em.repoName != "test-repo" {
			t.Errorf("key %q: unexpected eventMsg: %+v", k, em)
		}
	}
}

func TestUpdate_Confirm_No(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	m.confirmPending = &confirmMsg{eventType: "CLOSE", prompt: "Close PR?"}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	nm := next.(prModel)
	if nm.confirmPending != nil {
		t.Error("expected confirmPending to be cleared on any non-y key")
	}
}

// --- Update: top-level message types --------------------------------------

func TestUpdate_DiffLoadedMsg(t *testing.T) {
	m := newRawModel(nil)
	next, cmd := m.Update(diffLoadedMsg{title: "t", lines: []string{"a", "b"}})
	nm := next.(prModel)
	if !nm.showDiff || nm.diffTitle != "t" || len(nm.diffLines) != 2 {
		t.Errorf("unexpected model after diffLoadedMsg: %+v", nm)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdate_ConfirmMsg(t *testing.T) {
	m := newRawModel(nil)
	next, cmd := m.Update(confirmMsg{eventType: "APPROVE", prompt: "Approve?"})
	nm := next.(prModel)
	if nm.confirmPending == nil || nm.confirmPending.eventType != "APPROVE" {
		t.Errorf("unexpected confirmPending: %+v", nm.confirmPending)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdate_BrowserOpenMsg_EmptyURL(t *testing.T) {
	// Deliberately using an empty URL so the openURL()/exec.Command branch is
	// never invoked — see the note in item_delegate_test.go about avoiding a
	// real browser launch during tests.
	m := newRawModel(nil)
	next, cmd := m.Update(browserOpenMsg{url: ""})
	if _, ok := next.(prModel); !ok {
		t.Fatalf("expected prModel, got %T", next)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdate_SectionEvent(t *testing.T) {
	m := newRawModel(nil)
	m.showSpinner = true
	next, _ := m.Update(sectionEvent([]string{"a", "b"}))
	nm := next.(prModel)
	if len(nm.sections) != 2 || nm.showSpinner {
		t.Errorf("unexpected model after sectionEvent: %+v", nm)
	}
}

func TestUpdate_RefreshEvent(t *testing.T) {
	m := newRawModel(nil)
	m.showEventPanel = true
	m.showSpinner = true
	next, _ := m.Update(refreshEvent([]model.PullRequestResponse{samplePR()}))
	nm := next.(prModel)
	if len(nm.list.Items()) != 1 {
		t.Errorf("expected 1 item after refresh, got %d", len(nm.list.Items()))
	}
	if nm.showEventPanel || nm.showSpinner {
		t.Error("expected event panel + spinner to be hidden after refresh")
	}
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 45})
	nm := next.(prModel)
	if nm.termWidth != 150 || nm.termHeight != 45 {
		t.Errorf("unexpected term size: %d x %d", nm.termWidth, nm.termHeight)
	}
}

func TestUpdate_UnhandledMsg(t *testing.T) {
	type unknownMsg struct{}
	m := newRawModel([]list.Item{samplePR()})
	next, _ := m.Update(unknownMsg{})
	if _, ok := next.(prModel); !ok {
		t.Fatalf("expected prModel, got %T", next)
	}
}

func TestUpdate_KeyMsg_CursorResetsEventPanel(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	m.showEventPanel = true
	m.sections = []string{"section"}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	nm := next.(prModel)
	if nm.showEventPanel {
		t.Error("expected showEventPanel to be reset on cursor movement")
	}
	if nm.sections != nil {
		t.Error("expected sections to be cleared on cursor movement")
	}
}

func TestUpdate_KeyMsg_RefreshTriggersNetwork(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       emptySearchGraphQLBody(),
	})
	ctx := service.NewMockContext(t, mockServer)

	m := newModel(ctx, pr.PRRequest{OrgName: "testorg"})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	nm := next.(prModel)
	if !nm.showEventPanel || !nm.showSpinner {
		t.Error("expected refresh to show the event panel + spinner")
	}
	// The mock server returns an empty PR list; firstOfType still confirms
	// the refresh cmd was invoked and produced a refreshEvent.
	_ = firstOfType[refreshEvent](t, cmd)
}

func TestUpdate_KeyMsg_EventMsgTriggersNetwork(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	ctx := service.NewMockContext(t, mockServer)

	m := newRawModel([]list.Item{samplePR()})
	next, cmd := m.Update(eventMsg{eventType: "STATUS", ctx: ctx, orgName: "testorg", repoName: "test-repo", selectedPR: samplePR()})
	nm := next.(prModel)
	if !nm.showEventPanel || !nm.showSpinner {
		t.Error("expected event panel + spinner to be shown")
	}
	se := firstOfType[sectionEvent](t, cmd)
	if len(se) == 0 {
		t.Error("expected non-empty sections from the event")
	}
}

func TestUpdate_FilterState_BlocksOtherBindings(t *testing.T) {
	m := newRawModel([]list.Item{samplePR()})
	// Enter filtering mode first.
	listModel, _ := m.list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m.list = listModel
	if m.list.FilterState() != list.Filtering {
		t.Fatal("expected the list to be in filtering state")
	}

	// While filtering, "r" should not trigger a refresh — it should be
	// treated as filter text input instead.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	nm := next.(prModel)
	if nm.showEventPanel {
		t.Error("expected refresh keybinding to be suppressed while filtering")
	}
}

// --- approvePR / closePR / mergePR ---------------------------------------

func TestApprovePR_NotOpen(t *testing.T) {
	msg, ok := approvePR(nil, "testorg", "test-repo", 7, model.PullRequestResponse{State: "CLOSED"})
	if ok {
		t.Error("expected ok=false for a non-open PR")
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApprovePR_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 1, "state": "APPROVED", "user": map[string]interface{}{"login": "reviewer1"}},
	})
	ctx := service.NewMockContext(t, mockServer)

	msg, ok := approvePR(ctx, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN"})
	if !ok {
		t.Fatalf("expected success, got message: %s", msg)
	}
}

func TestApprovePR_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/reviews", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})
	ctx := service.NewMockContext(t, mockServer)

	msg, ok := approvePR(ctx, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN"})
	if ok {
		t.Error("expected failure")
	}
	if msg == "" {
		t.Error("expected an error message")
	}
}

func TestClosePR_NotOpen(t *testing.T) {
	msg, ok := closePR(nil, "testorg", "test-repo", 7, model.PullRequestResponse{State: "MERGED"})
	if ok {
		t.Error("expected ok=false for a non-open PR")
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestClosePR_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 7, "state": "closed"},
	})
	ctx := service.NewMockContext(t, mockServer)

	msg, ok := closePR(ctx, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN"})
	if !ok {
		t.Fatalf("expected success, got message: %s", msg)
	}
}

func TestClosePR_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "not allowed"},
	})
	ctx := service.NewMockContext(t, mockServer)

	msg, ok := closePR(ctx, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN"})
	if ok {
		t.Error("expected failure")
	}
	if msg == "" {
		t.Error("expected an error message")
	}
}

func TestMergePR_NotOpen(t *testing.T) {
	msg, ok := mergePR(nil, "testorg", "test-repo", 7, model.PullRequestResponse{State: "CLOSED"})
	if ok {
		t.Error("expected ok=false for a non-open PR")
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestMergePR_NotMergeable(t *testing.T) {
	msg, ok := mergePR(nil, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN", Mergeable: "CONFLICTING"})
	if ok {
		t.Error("expected ok=false for a conflicting PR")
	}
	if !strings.Contains(msg, "CONFLICTING") {
		t.Errorf("expected message to mention the mergeable state, got %q", msg)
	}
}

func TestMergePR_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/merge", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"merged": true, "message": "merged", "sha": "abc123"},
	})
	ctx := service.NewMockContext(t, mockServer)

	msg, ok := mergePR(ctx, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN", Mergeable: "MERGEABLE"})
	if !ok {
		t.Fatalf("expected success, got message: %s", msg)
	}
}

func TestMergePR_EmptyMergeableStillAllowed(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/merge", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"merged": true, "message": "merged", "sha": "abc123"},
	})
	ctx := service.NewMockContext(t, mockServer)

	// Mergeable == "" (unknown/not yet computed by GitHub) should still be allowed through.
	_, ok := mergePR(ctx, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN", Mergeable: ""})
	if !ok {
		t.Fatal("expected success when Mergeable is empty")
	}
}

func TestMergePR_Error(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/7/merge", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := service.NewMockContext(t, mockServer)

	msg, ok := mergePR(ctx, "testorg", "test-repo", 7, model.PullRequestResponse{State: "OPEN", Mergeable: "MERGEABLE"})
	if ok {
		t.Error("expected failure")
	}
	if msg == "" {
		t.Error("expected an error message")
	}
}

// --- truncateBody ----------------------------------------------------------

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		max  int
		want string
	}{
		{"empty", "", 10, ""},
		{"whitespace only", "   \n  ", 10, ""},
		{"short", "hello", 200, "hello"},
		{"exactly at limit", strings.Repeat("a", 10), 10, strings.Repeat("a", 10)},
		{"over limit", strings.Repeat("a", 15), 10, strings.Repeat("a", 10) + "..."},
		{"crlf normalized", "line1\r\nline2", 200, "line1\nline2"},
		{"trims surrounding whitespace", "  hi  ", 200, "hi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBody(tt.body, tt.max)
			if got != tt.want {
				t.Errorf("truncateBody(%q, %d) = %q, want %q", tt.body, tt.max, got, tt.want)
			}
		})
	}
}

// --- processEventMsg / processEventAndGetSectionRenders --------------------

func TestProcessEventMsg_GraphQLError(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusForbidden,
		Body:       map[string]interface{}{"message": "forbidden"},
	})
	ctx := service.NewMockContext(t, mockServer)

	status := processEventMsg(ctx, "testorg", "test-repo", 100, "sha", "STATUS")
	if status.actionSuccess {
		t.Error("expected actionSuccess=false on a GraphQL error")
	}
	if status.pullRequestResponse.ErrorMessage == "" {
		t.Error("expected ErrorMessage to be set")
	}
}

func TestProcessEventMsg_Status(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	ctx := service.NewMockContext(t, mockServer)

	status := processEventMsg(ctx, "testorg", "test-repo", 100, "sha", "STATUS")
	if status.actionMessage != "" {
		t.Errorf("expected no action message for STATUS, got %q", status.actionMessage)
	}
	if !status.actionSuccess {
		t.Error("expected actionSuccess=true for STATUS")
	}
}

func TestProcessEventMsg_Approve(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/100/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 1, "state": "APPROVED"},
	})
	ctx := service.NewMockContext(t, mockServer)

	status := processEventMsg(ctx, "testorg", "test-repo", 100, "sha", "APPROVE")
	if !status.actionSuccess {
		t.Fatalf("expected success, got message: %s", status.actionMessage)
	}
}

func TestProcessEventMsg_Merge(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/100/merge", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"merged": true, "sha": "abc123"},
	})
	ctx := service.NewMockContext(t, mockServer)

	status := processEventMsg(ctx, "testorg", "test-repo", 100, "sha", "MERGE")
	if !status.actionSuccess {
		t.Fatalf("expected success, got message: %s", status.actionMessage)
	}
}

func TestProcessEventMsg_ApproveMerge_Success(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/100/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 1, "state": "APPROVED"},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/100/merge", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"merged": true, "sha": "abc123"},
	})
	ctx := service.NewMockContext(t, mockServer)

	status := processEventMsg(ctx, "testorg", "test-repo", 100, "sha", "APPROVE_MERGE")
	if !status.actionSuccess {
		t.Fatalf("expected success, got message: %s", status.actionMessage)
	}
}

func TestProcessEventMsg_ApproveMerge_MergeFails(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/100/reviews", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"id": 1, "state": "APPROVED"},
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/100/merge", testutils.MockResponse{
		StatusCode: http.StatusNotFound,
		Body:       map[string]interface{}{"message": "Not Found"},
	})
	ctx := service.NewMockContext(t, mockServer)

	status := processEventMsg(ctx, "testorg", "test-repo", 100, "sha", "APPROVE_MERGE")
	if status.actionSuccess {
		t.Error("expected failure when merge fails after a successful approve")
	}
	if !strings.HasPrefix(status.actionMessage, "Approved but merge failed:") {
		t.Errorf("unexpected message: %q", status.actionMessage)
	}
}

func TestProcessEventMsg_Close(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	mockServer.SetResponse("/repos/testorg/test-repo/pulls/100", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       map[string]interface{}{"number": 100, "state": "closed"},
	})
	ctx := service.NewMockContext(t, mockServer)

	status := processEventMsg(ctx, "testorg", "test-repo", 100, "sha", "CLOSE")
	if !status.actionSuccess {
		t.Fatalf("expected success, got message: %s", status.actionMessage)
	}
}

func TestProcessEventAndGetSectionRenders(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	mockServer.SetResponse("/graphql", testutils.MockResponse{
		StatusCode: http.StatusOK,
		Body:       prDetailGraphQLBody(100, "OPEN", "MERGEABLE", "CLEAN"),
	})
	ctx := service.NewMockContext(t, mockServer)

	sections := <-processEventAndGetSectionRenders(ctx, "testorg", "test-repo", 100, "sha", "STATUS")
	if len(sections) == 0 {
		t.Fatal("expected non-empty sections")
	}
}

// --- getSectionsRenders / table builders ------------------------------------

func TestGetSectionsRenders_Error(t *testing.T) {
	sections := getSectionsRenders(eventStatusResponse{
		pullRequestResponse: model.PullRequestResponse{ErrorMessage: "boom"},
	})
	if len(sections) != 1 || !strings.Contains(sections[0], "boom") {
		t.Errorf("unexpected sections: %+v", sections)
	}
}

func TestGetSectionsRenders_Full(t *testing.T) {
	status := eventStatusResponse{
		eventType: "APPROVE",
		pullRequestResponse: model.PullRequestResponse{
			PRNumber:  1,
			TitleName: "Add feature",
			Body:      "some body text",
			State:     "OPEN",
			Base:      model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
			Head:      model.PRBranch{Ref: "feature", Sha: "abc1234"},
		},
		checkRunResponse: model.CheckRunResponse{
			OverallConclusion: "SUCCESS",
			CheckRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "success"},
			},
		},
		prReviews: []model.ReviewPullRequestResponse{
			{User: model.User{Login: "reviewer1"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
		},
		actionMessage: "PR Approved successfully",
		actionSuccess: true,
	}

	sections := getSectionsRenders(status)
	joined := strings.Join(sections, "\n")
	if !strings.Contains(joined, "Add feature") {
		t.Error("expected title in sections")
	}
	if !strings.Contains(joined, "some body text") {
		t.Error("expected body preview in sections")
	}
	if !strings.Contains(joined, "Check Runs") {
		t.Error("expected check runs section")
	}
	if !strings.Contains(joined, "Reviews Status") {
		t.Error("expected reviews section")
	}
	if !strings.Contains(joined, "PR Approved successfully") {
		t.Error("expected the action status message")
	}
}

func TestGetSectionsRenders_ActionFailure(t *testing.T) {
	status := eventStatusResponse{
		eventType: "CLOSE",
		pullRequestResponse: model.PullRequestResponse{
			PRNumber:  1,
			TitleName: "Add feature",
			Base:      model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
		},
		actionMessage: "PR is not open — cannot close",
		actionSuccess: false,
	}
	sections := getSectionsRenders(status)
	joined := strings.Join(sections, "\n")
	if !strings.Contains(joined, "PR is not open") {
		t.Error("expected the failure message in sections")
	}
}

func TestGetPRResponseTable_ManyFiles(t *testing.T) {
	files := make([]model.PullRequestFile, 0, 7)
	for i := 0; i < 7; i++ {
		files = append(files, model.PullRequestFile{Filename: fmt.Sprintf("file%d.go", i), Additions: 1, Deletions: 1, ChangeType: "MODIFIED"})
	}
	prResp := model.PullRequestResponse{
		PRNumber:         1,
		State:            "OPEN",
		Mergeable:        "MERGEABLE",
		MergeStateStatus: "CLEAN",
		Base:             model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
		Head:             model.PRBranch{Ref: "feature", Sha: "abcdef1234567"},
		Author:           model.User{Login: "author1"},
		HTMLUrl:          "https://github.com/testorg/test-repo/pull/1",
	}
	filesResp := model.PullRequestFilesResponse{Files: files}

	out := getPRResponseTable(prResp, filesResp)
	if !strings.Contains(out, "and 2 more") {
		t.Errorf("expected overflow indicator for >5 files, got: %s", out)
	}
	if !strings.Contains(out, "file0.go") {
		t.Error("expected first file to be listed")
	}
}

func TestGetPRResponseTable_ChangeTypes(t *testing.T) {
	files := []model.PullRequestFile{
		{Filename: "added.go", ChangeType: "ADDED"},
		{Filename: "deleted.go", ChangeType: "REMOVED"},
		{Filename: "renamed.go", ChangeType: "RENAMED"},
		{Filename: "copied.go", ChangeType: "COPIED"},
	}
	prResp := model.PullRequestResponse{
		PRNumber: 1,
		Base:     model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
	}
	out := getPRResponseTable(prResp, model.PullRequestFilesResponse{Files: files})
	for _, f := range files {
		if !strings.Contains(out, f.Filename) {
			t.Errorf("expected %s in output", f.Filename)
		}
	}
}

func TestGetPRResponseTable_Defaults(t *testing.T) {
	prResp := model.PullRequestResponse{
		PRNumber: 1,
		Base:     model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
		// MergeAt, MergedBy, Labels, ReviewDecision all left zero-value.
	}
	out := getPRResponseTable(prResp, model.PullRequestFilesResponse{})
	if !strings.Contains(out, "test-repo") {
		t.Errorf("expected repository name in table, got: %s", out)
	}
	if !strings.Contains(out, "main") {
		t.Errorf("expected base branch name in table, got: %s", out)
	}
}

func TestGetPRResponseTable_MergedByFallsBackToLogin(t *testing.T) {
	prResp := model.PullRequestResponse{
		PRNumber: 1,
		Base:     model.PRBranch{Ref: "main", Repo: model.Repository{Name: "test-repo"}},
		MergedBy: model.User{Login: "merger-login"},
	}
	out := getPRResponseTable(prResp, model.PullRequestFilesResponse{})
	if !strings.Contains(out, "merger-login") {
		t.Error("expected MergedBy to fall back to Login when Name is empty")
	}
}

func TestGetPRTableStyle_OutOfRange(t *testing.T) {
	rows := [][]string{{"State", "OPEN"}}
	style := getPRTableStyle(0, rows, -1)
	if style.GetForeground() != CellStyle.GetForeground() {
		t.Error("expected the base CellStyle to be returned for an out-of-range row")
	}
	style2 := getPRTableStyle(0, rows, 5)
	if style2.GetForeground() != CellStyle.GetForeground() {
		t.Error("expected the base CellStyle to be returned for an out-of-range row")
	}
}

func TestGetPRTableStyle_LabelValueCombos(t *testing.T) {
	tests := []struct {
		label string
		value string
	}{
		{"State", "OPEN"},
		{"Review Decision", "APPROVED"},
		{"Review Decision", "CHANGES_REQUESTED"},
		{"Review Decision", "REVIEW_REQUIRED"},
		{"Review Decision", "SOMETHING_ELSE"},
		{mergeableTitle, "MERGEABLE"},
		{mergeableTitle, "CONFLICTING"},
		{mergeableTitle, "UNKNOWN"},
		{mergeableTitle, "OTHER"},
		{mergeableStateTitle, "CLEAN"},
		{"Labels", "bug, priority"},
		{"Author", "someone"},
	}
	for _, tt := range tests {
		rows := [][]string{{tt.label, tt.value}}
		// col 0 => dimmed + right aligned regardless of label/value.
		s0 := getPRTableStyle(0, rows, 0)
		if s0.GetAlignHorizontal() != lipgloss.Right {
			t.Errorf("%s/%s: expected column 0 to be right-aligned", tt.label, tt.value)
		}
		// col 1 should not panic and should return some style.
		s1 := getPRTableStyle(1, rows, 0)
		_ = s1
	}
}

func TestGetCheckRunsTable(t *testing.T) {
	resp := model.CheckRunResponse{
		CheckRuns: []model.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "success", StartedAt: "2024-01-01T00:00:00Z", CompletedAt: "2024-01-01T00:05:00Z"},
			{Name: "lint", Status: "completed", Conclusion: "failure", StartedAt: "2024-01-01T00:00:00Z", CompletedAt: "2024-01-01T00:02:00Z"},
		},
	}
	out := getCheckRunsTable(resp)
	if !strings.Contains(out, "build") || !strings.Contains(out, "lint") {
		t.Errorf("expected check run names in output: %s", out)
	}
	if !strings.Contains(out, "Name") || !strings.Contains(out, "Conclusion") {
		t.Errorf("expected headers in output: %s", out)
	}
}

func TestGetCheckRunsTable_Empty(t *testing.T) {
	out := getCheckRunsTable(model.CheckRunResponse{})
	if out == "" {
		t.Error("expected a non-empty (headers-only) table even with no check runs")
	}
}

func TestGetReviewTable_SortsBySubmittedAtDescending(t *testing.T) {
	reviews := []model.ReviewPullRequestResponse{
		{User: model.User{Login: "older"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
		{User: model.User{Login: "newer"}, State: "APPROVED", SubmittedAt: "2024-02-01T00:00:00Z"},
	}
	out := getReviewTable(reviews)
	iOld := strings.Index(out, "older")
	iNew := strings.Index(out, "newer")
	if iOld == -1 || iNew == -1 {
		t.Fatalf("expected both reviewers in output: %s", out)
	}
	if iNew > iOld {
		t.Errorf("expected the newer review to be listed first, got order: %s", out)
	}
}

func TestGetReviewTable_ReviewerFallsBackToLogin(t *testing.T) {
	reviews := []model.ReviewPullRequestResponse{
		{User: model.User{Login: "login-only"}, State: "COMMENTED", SubmittedAt: "2024-01-01T00:00:00Z"},
	}
	out := getReviewTable(reviews)
	if !strings.Contains(out, "login-only") {
		t.Error("expected reviewer login as fallback when Name is empty")
	}
}

// --- renderDiffOverlay -------------------------------------------------------

func TestRenderDiffOverlay_LinePrefixes(t *testing.T) {
	m := newRawModel(nil)
	m.showDiff = true
	m.diffTitle = "short title"
	m.diffLines = []string{"@@ -1,2 +1,3 @@", "+added line", "-removed line", "── file.go ──", "context line"}
	m.termWidth = 100
	m.termHeight = 30

	out := m.renderDiffOverlay()
	for _, want := range []string{"added line", "removed line", "context line", "file.go", "short title"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected overlay to contain %q", want)
		}
	}
}

func TestRenderDiffOverlay_ScrollFooter(t *testing.T) {
	m := newRawModel(nil)
	m.showDiff = true
	m.diffTitle = "title"
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	m.diffLines = lines
	m.termWidth = 100
	m.termHeight = 30 // visible = 20 < 40 total lines

	out := m.renderDiffOverlay()
	if !strings.Contains(out, "of 40 lines") {
		t.Errorf("expected a scroll footer, got: %s", out)
	}
}

func TestRenderDiffOverlay_NarrowClampsAndTitleTruncation(t *testing.T) {
	m := newRawModel(nil)
	m.showDiff = true
	m.diffTitle = strings.Repeat("x", 100)
	m.diffLines = []string{"a"}
	m.termWidth = 20 // overlayWidth would be negative -> clamps to 60
	m.termHeight = 5 // visible would be negative -> clamps to 5

	out := m.renderDiffOverlay()
	if !strings.Contains(out, "…") {
		t.Errorf("expected the long title to be truncated with an ellipsis, got: %s", out)
	}
}

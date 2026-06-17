package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/node"
	"github.com/BX-Team/Nexon/internal/store"
)

func testService(t *testing.T) *core.Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "tui.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.SeedDefaults(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return core.New(st, node.StubFactory)
}

// TestModelRendersAllTabs drives the model headlessly: size it, load every
// panel's data, then cycle through the tabs rendering each. It must never panic
// and every tab must produce non-empty output.
func TestModelRendersAllTabs(t *testing.T) {
	svc := testService(t)
	if _, err := svc.AddUser(core.CreateUserParams{Username: "alice"}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var tm tea.Model = newModel(svc, "http://localhost:8080")
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// Execute each panel's loader and feed the result back in.
	for _, p := range tm.(model).panels {
		if cmd := p.load(); cmd != nil {
			tm, _ = tm.Update(cmd())
		}
	}

	n := len(tm.(model).panels)
	for i := 0; i < n; i++ {
		out := tm.View()
		if strings.TrimSpace(out) == "" {
			t.Fatalf("tab %d rendered empty", i)
		}
		if !strings.Contains(out, "Nexon") {
			t.Fatalf("tab %d missing header: %q", i, out)
		}
		tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyTab})
	}

	// Users tab should list the seeded user once loaded.
	tm.(model).panels[1].load()() // sanity: loader runs without error
}

func TestQuitKey(t *testing.T) {
	var tm tea.Model = newModel(testService(t), "http://localhost:8080")
	_, cmd := tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command on 'q'")
	}
}

// key sends a single rune key to a panel.
func typeStr(p panel, s string) {
	for _, r := range s {
		p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// TestUsersCreateFlow drives the users panel like a user would: open the create
// form, type a name, submit, and confirm the service actually created it.
func TestUsersCreateFlow(t *testing.T) {
	svc := testService(t)
	p := newUsersPanel(svc, "http://localhost:8080")

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}) // open create form
	if !p.capturing() {
		t.Fatal("panel should capture input while the form is open")
	}
	typeStr(p, "bob") // username field is focused first
	p.update(tea.KeyMsg{Type: tea.KeyEnter})

	if p.capturing() {
		t.Fatal("form should close after submit")
	}
	if _, err := svc.GetUser("bob"); err != nil {
		t.Fatalf("user not created: %v", err)
	}
}

// TestClientHeaderFlow creates a client app with one custom header via the form,
// exercising the PasarGuard-style header editor (ctrl+n add, tab between fields).
func TestClientHeaderFlow(t *testing.T) {
	svc := testService(t)
	p := newClientsPanel(svc)

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}) // open create form
	typeStr(p, "Probe")
	p.update(tea.KeyMsg{Type: tea.KeyTab}) // → UA pattern
	typeStr(p, "^Probe")
	p.update(tea.KeyMsg{Type: tea.KeyCtrlN}) // add a header row, focus its key
	typeStr(p, "X-Test")
	p.update(tea.KeyMsg{Type: tea.KeyTab}) // → header value
	typeStr(p, "yes")
	p.update(tea.KeyMsg{Type: tea.KeyEnter}) // submit

	apps, err := svc.ListClientApps()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found *headerCheck
	for _, a := range apps {
		if a.Name == "Probe" {
			found = &headerCheck{ua: a.UAPattern, hv: a.Headers["X-Test"]}
		}
	}
	if found == nil {
		t.Fatal("client 'Probe' was not created")
	}
	if found.ua != "^Probe" {
		t.Fatalf("ua pattern = %q", found.ua)
	}
	if found.hv != "yes" {
		t.Fatalf("custom header X-Test = %q, want \"yes\"", found.hv)
	}
}

type headerCheck struct{ ua, hv string }

// TestUserEditPreservesSize opens the edit form on a user with a data limit and
// submits unchanged; the pre-filled size must round-trip (regression: humanBytes
// "200.0GB" wasn't parseSize-compatible).
func TestUserEditPreservesSize(t *testing.T) {
	svc := testService(t)
	const gb100 = int64(100) << 30
	if _, err := svc.AddUser(core.CreateUserParams{Username: "carol", DataLimit: gb100}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	p := newUsersPanel(svc, "http://localhost:8080")
	p.update(usersMsg{users: mustUsers(t, svc)}) // populate table/cursor
	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	p.update(tea.KeyMsg{Type: tea.KeyEnter}) // submit unchanged

	u, err := svc.GetUser("carol")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if u.DataLimit != gb100 {
		t.Fatalf("data limit corrupted on edit: got %d want %d", u.DataLimit, gb100)
	}
}

func mustUsers(t *testing.T, svc *core.Service) []*store.User {
	t.Helper()
	u, err := svc.ListUsers("")
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	return u
}

// TestUserDetailView opens the per-user detail (enter), loads its devices, and
// checks the sub link + sections render, then esc returns to the list.
func TestUserDetailView(t *testing.T) {
	svc := testService(t)
	u, err := svc.AddUser(core.CreateUserParams{Username: "dave"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	p := newUsersPanel(svc, "http://localhost:8080")
	p.update(usersMsg{users: mustUsers(t, svc)})

	cmd := p.update(tea.KeyMsg{Type: tea.KeyEnter}) // open detail
	if !p.capturing() {
		t.Fatal("detail view should capture input")
	}
	if cmd != nil {
		p.update(cmd()) // run the device loader and feed the result back
	}

	out := p.view()
	if !strings.Contains(out, "/sub/"+u.SubToken) {
		t.Fatalf("detail missing sub link for token %s", u.SubToken)
	}
	if !strings.Contains(out, "Devices") {
		t.Fatal("detail missing devices section")
	}

	p.update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.capturing() {
		t.Fatal("esc should return to the list")
	}
}

// TestGroupCreateFlow creates a node group through the Groups panel form.
func TestGroupCreateFlow(t *testing.T) {
	svc := testService(t)
	p := newGroupsPanel(svc)

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}) // open create form
	if !p.capturing() {
		t.Fatal("group form should capture input")
	}
	typeStr(p, "friends")
	p.update(tea.KeyMsg{Type: tea.KeyEnter})

	groups, err := svc.ListNodeGroups()
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	found := false
	for _, g := range groups {
		if g.Name == "friends" {
			found = true
		}
	}
	if !found {
		t.Fatal("group 'friends' was not created")
	}
}

// TestUserGroupCycle cycles a user through groups with the 'g' key.
func TestUserGroupCycle(t *testing.T) {
	svc := testService(t)
	if _, err := svc.AddUser(core.CreateUserParams{Username: "erin"}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := svc.CreateNodeGroup("friends"); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	p := newUsersPanel(svc, "http://localhost:8080")
	if cmd := p.load(); cmd != nil {
		p.update(cmd()) // populate users + groups
	}

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}) // cycle group

	u, _ := svc.GetUser("erin")
	if u.GroupID == nil || *u.GroupID == store.DefaultGroupID {
		t.Fatalf("group should have moved off default, got %v", u.GroupID)
	}
}

// TestDashboardTick verifies the dashboard schedules a refresh tick and that a
// tick triggers another fetch + reschedule (live refresh chain).
func TestDashboardTick(t *testing.T) {
	p := newDashboardPanel(testService(t))
	if cmd := p.load(); cmd == nil {
		t.Fatal("dashboard load should start a tick + fetch")
	}
	if cmd := p.update(dashTickMsg(time.Now())); cmd == nil {
		t.Fatal("a tick should refetch and reschedule")
	}
}

package main

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// fakeAPI is a scriptable dockerAPI. Tests inject it so no Docker daemon is
// needed anywhere in the suite.
type fakeAPI struct {
	items     map[kind][]item
	listErr   error
	removeErr error
	removed   []string // "kind:id", in call order
}

func (f *fakeAPI) List(k kind) ([]item, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.items[k], nil
}

func (f *fakeAPI) Remove(k kind, id string, force bool) error {
	f.removed = append(f.removed, fmt.Sprintf("%s:%s", k, id))
	return f.removeErr
}

// sampleModel builds a model with three containers, already loaded.
func sampleModel() model {
	now := time.Now().Unix()
	f := &fakeAPI{items: map[kind][]item{
		kindContainers: {
			{id: "c1", name: "web", image: "nginx:latest", status: "Up 2 hours", created: now - 2*3600, running: true},
			{id: "c2", name: "db", image: "postgres:16", status: "Exited (0) 3 days ago", created: now - 72*3600},
			{id: "c3", name: "cache", image: "redis:7", status: "Up 5 minutes", created: now - 5*60, running: true},
		},
	}}
	m := newModel(f)
	m.items[kindContainers] = f.items[kindContainers]
	m.loaded[kindContainers] = true
	m.vp.SetContent(m.listContent())
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestSelectionHelpers(t *testing.T) {
	m := sampleModel()
	if got := m.countSelected(); got != 0 {
		t.Fatalf("fresh countSelected=%d want 0", got)
	}
	m.selected[kindContainers][1] = true
	if got := m.countSelected(); got != 1 {
		t.Fatalf("countSelected=%d want 1", got)
	}
	sel := m.selectedItems()
	if len(sel) != 1 || sel[0].id != "c2" {
		t.Fatalf("selectedItems=%v want [c2]", sel)
	}
}

func TestVisibleIndexesFilter(t *testing.T) {
	m := sampleModel()
	cases := []struct {
		filter string
		want   []int
	}{
		{"", []int{0, 1, 2}},
		{"WEb", []int{0}},    // by name, case-insensitive
		{"redis", []int{2}},  // by image
		{"Exited", []int{1}}, // by status
		{"c3", []int{2}},     // by id
		{"nope", nil},        // no match
	}
	for _, c := range cases {
		m.filter = c.filter
		got := m.visibleIndexes()
		if len(got) != len(c.want) {
			t.Fatalf("filter %q visible=%v want %v", c.filter, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("filter %q visible=%v want %v", c.filter, got, c.want)
			}
		}
	}
}

// Filtering must not misroute selection: the cursor is a visible position, not
// a storage index. Guards the delete-the-wrong-resource bug.
func TestFilterSelectionMapsToCorrectItem(t *testing.T) {
	m := sampleModel()
	m.filter = "cache" // only storage index 2 visible, at cursor position 0
	next, _ := m.updateSelect(key(" "))
	m = next.(model)

	sel := m.selectedItems()
	if len(sel) != 1 || sel[0].id != "c3" {
		t.Fatalf("selectedItems=%v want [c3] — WRONG RESOURCE SELECTED", sel)
	}
	if m.selected[kindContainers][0] {
		t.Fatal("selected[0] set — cursor position leaked as storage index")
	}
	if !m.selected[kindContainers][2] {
		t.Fatal("selected[2] not set — real index resolution failed")
	}
}

func TestFilterCursorClampsWhenListShrinks(t *testing.T) {
	m := sampleModel()
	m.cursor = 2
	m.filter = "db" // 1 match
	m.clampCursor()
	if m.cursor != 0 {
		t.Fatalf("cursor=%d want 0 after filter shrank list", m.cursor)
	}
}

func TestSelectAllNoneOperateOnVisibleSet(t *testing.T) {
	m := sampleModel()
	m.filter = "redis" // visible [2]
	next, _ := m.updateSelect(key("a"))
	m = next.(model)
	if !m.selected[kindContainers][2] || m.selected[kindContainers][0] {
		t.Fatalf("select-all under filter set %v want only 2", m.selected[kindContainers])
	}
	m.selected[kindContainers][0] = true // pre-select a hidden row
	next, _ = m.updateSelect(key("n"))
	m = next.(model)
	if m.selected[kindContainers][2] {
		t.Fatal("select-none left visible row selected")
	}
	if !m.selected[kindContainers][0] {
		t.Fatal("select-none cleared a NON-visible selection (should persist)")
	}
}

func TestSwitchKindResetsAndLoads(t *testing.T) {
	m := sampleModel()
	m.filter = "web"
	m.cursor = 2

	next, cmd := m.switchKind(kindImages)
	m = next.(model)
	if cmd == nil {
		t.Fatal("expected load command for a not-yet-loaded tab")
	}
	if m.kind != kindImages {
		t.Fatalf("kind=%v want images", m.kind)
	}
	if m.filter != "" || m.cursor != 0 || m.filtering {
		t.Fatalf("filter=%q cursor=%d must reset on tab switch", m.filter, m.cursor)
	}
	// Driving the load command populates the tab and clears the loading state.
	msg := cmd()
	next, _ = m.Update(msg)
	m = next.(model)
	if !m.loaded[kindImages] {
		t.Fatal("images not marked loaded after load msg")
	}
}

func TestSwitchKindBackIsNoCmdWhenLoaded(t *testing.T) {
	m := sampleModel()
	next, cmd := m.switchKind(kindImages) // trigger load
	m = next.(model)
	msg := cmd()
	next, _ = m.Update(msg)
	m = next.(model)

	next, cmd = m.switchKind(kindContainers) // containers already loaded
	m = next.(model)
	if cmd != nil {
		t.Fatal("switching back to a loaded tab must not reload")
	}
}

// Selection is per-tab and must survive tab switches.
func TestSelectionPersistsAcrossTabs(t *testing.T) {
	m := sampleModel()
	m.selected[kindContainers][1] = true

	next, cmd := m.switchKind(kindVolumes)
	if cmd != nil {
		m = next.(model)
	} else {
		m = next.(model)
	}
	if got := m.countSelected(); got != 0 {
		t.Fatalf("countSelected on empty tab=%d want 0", got)
	}
	next, _ = m.switchKind(kindContainers)
	m = next.(model)
	if got := m.countSelected(); got != 1 {
		t.Fatalf("countSelected back on containers=%d want 1", got)
	}
}

func TestCursorItem(t *testing.T) {
	m := sampleModel()
	if it, ok := m.cursorItem(); !ok || it.id != "c1" {
		t.Fatalf("cursorItem=%v ok=%v want c1", it, ok)
	}
	m.filter = "zzz"
	m.clampCursor()
	if _, ok := m.cursorItem(); ok {
		t.Fatal("cursorItem must report no item when list is empty")
	}
}

func TestWarning(t *testing.T) {
	if w := (item{}).warning(); w != "" {
		t.Errorf("plain item warning=%q want empty", w)
	}
	if w := (item{running: true}).warning(); w == "" {
		t.Error("running item must warn")
	}
	if w := (item{inUse: true, usedBy: []string{"web", "db"}}).warning(); w == "" {
		t.Error("in-use item must warn")
	}
}

func TestDeleteNextPops(t *testing.T) {
	m := sampleModel()
	m.delKind = kindContainers
	m.queue = []item{{id: "c1", name: "web"}, {id: "c2", name: "db"}}

	cmd := m.deleteNext()
	if cmd == nil || len(m.queue) != 1 {
		t.Fatalf("deleteNext cmd=%v queue=%d want 1 left", cmd, len(m.queue))
	}
	msg, ok := cmd().(deleteDoneMsg)
	if !ok || msg.id != "c1" || msg.title != "web" {
		t.Fatalf("cmd yielded %#v want deleteDoneMsg c1/web", msg)
	}
	m.deleteNext()
	if len(m.queue) != 0 {
		t.Fatalf("queue=%d want 0", len(m.queue))
	}
}

func TestFanOutRespectsCap(t *testing.T) {
	m := sampleModel()
	big := maxConcurrent + 5
	m.items[kindContainers] = make([]item, big)
	m.selected[kindContainers] = map[int]bool{}
	for i := range m.items[kindContainers] {
		m.items[kindContainers][i] = item{id: fmt.Sprintf("c%d", i), name: fmt.Sprintf("n%d", i)}
		m.selected[kindContainers][i] = true
	}
	m.state = stateConfirm

	next, cmd := m.updateConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = next.(model)

	if m.total != big || len(m.targets) != big {
		t.Fatalf("total=%d targets=%d want %d", m.total, len(m.targets), big)
	}
	if want := big - maxConcurrent; len(m.queue) != want {
		t.Fatalf("queue=%d want %d (cap not respected)", len(m.queue), want)
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", cmd())
	}
	if len(batch) != maxConcurrent+1 {
		t.Fatalf("batch=%d want %d (=cap+spinner)", len(batch), maxConcurrent+1)
	}
}

func TestConfirmToDoneFlow(t *testing.T) {
	m := sampleModel()
	var deleted []string
	m.remove = func(k kind, id string) error {
		deleted = append(deleted, id)
		if id == "c3" {
			return fmt.Errorf("boom")
		}
		return nil
	}
	m.selected[kindContainers][0] = true // c1 ok
	m.selected[kindContainers][2] = true // c3 fails

	m.state = stateConfirm
	next, cmd := m.updateConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = next.(model)
	if m.state != stateDeleting {
		t.Fatalf("state=%d want stateDeleting", m.state)
	}
	m = pump(t, m, cmd)

	if m.state != stateDone {
		t.Fatalf("final state=%d want stateDone", m.state)
	}
	if len(deleted) != 2 {
		t.Fatalf("remover called %d times want 2 (%v)", len(deleted), deleted)
	}
	if fail := reportFailures(m.results); fail != 1 {
		t.Fatalf("reportFailures=%d want 1", fail)
	}
	if m.delKind != kindContainers {
		t.Fatalf("delKind=%v want containers", m.delKind)
	}
}

func TestConfirmCancel(t *testing.T) {
	m := sampleModel()
	m.selected[kindContainers][0] = true
	m.state = stateConfirm
	next, _ := m.updateConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = next.(model)
	if m.state != stateSelect {
		t.Fatalf("state=%d want stateSelect after cancel", m.state)
	}
}

func TestLoadedMsgRecordsError(t *testing.T) {
	m := newModel(&fakeAPI{listErr: fmt.Errorf("daemon down")})
	next, _ := m.Update(loadedMsg{k: kindContainers, err: fmt.Errorf("daemon down")})
	m = next.(model)
	if m.lerr[kindContainers] == nil {
		t.Fatal("load error not recorded")
	}
	if !m.loaded[kindContainers] {
		t.Fatal("tab should be marked loaded after error so it stops spinning")
	}
}

// Parallel end-to-end: every selected resource is removed exactly once and the
// run converges to stateDone. Run with -race to exercise the goroutines.
func TestParallelDeleteAllComplete(t *testing.T) {
	m := sampleModel()
	var (
		mu      sync.Mutex
		removed = map[string]int{}
	)
	m.remove = func(k kind, id string) error {
		time.Sleep(2 * time.Millisecond)
		mu.Lock()
		removed[id]++
		mu.Unlock()
		return nil
	}
	for i := range m.items[kindContainers] {
		m.selected[kindContainers][i] = true
	}
	total := len(m.items[kindContainers])

	fm := runProgram(t, m)

	if fm.state != stateDone {
		t.Fatalf("final state=%d want stateDone", fm.state)
	}
	if fm.done != total || len(fm.results) != total {
		t.Fatalf("done=%d results=%d want %d", fm.done, len(fm.results), total)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(removed) != total {
		t.Fatalf("distinct removed=%d want %d (%v)", len(removed), total, removed)
	}
	for id, n := range removed {
		if n != 1 {
			t.Errorf("%s removed %d times want 1", id, n)
		}
	}
}

func runProgram(t *testing.T, m model) model {
	t.Helper()
	m.state = stateConfirm
	p := tea.NewProgram(m, tea.WithoutRenderer(), tea.WithInput(bytes.NewReader(nil)))
	go func() {
		time.Sleep(5 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	}()
	done := make(chan model, 1)
	go func() {
		fm, err := p.Run()
		if err != nil {
			t.Errorf("program run: %v", err)
		}
		done <- fm.(model)
	}()
	select {
	case fm := <-done:
		return fm
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("program did not converge to stateDone")
		return model{}
	}
}

// pump repeatedly executes a command and feeds messages back into Update until
// the model reaches stateDone, dropping spinner ticks so it terminates.
func pump(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; len(queue) > 0; i++ {
		if i > 1000 {
			t.Fatal("pump did not converge")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		switch msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, msg.(tea.BatchMsg)...)
			continue
		case spinner.TickMsg:
			continue
		}
		next, nc := m.Update(msg)
		m = next.(model)
		if nc != nil {
			queue = append(queue, nc)
		}
		if m.state == stateDone {
			return m
		}
	}
	return m
}

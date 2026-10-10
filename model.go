package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// doneLinger is how long the final summary stays on screen before the program
// exits, so the result is actually visible.
const doneLinger = 1200 * time.Millisecond

// Default viewport geometry so the list renders on the very first frame.
const (
	defWidth  = 80
	defHeight = 24
)

// maxConcurrent caps how many removes run at once. The Docker daemon serializes
// much of this work anyway; the cap keeps a bulk delete from stampeding it.
const maxConcurrent = 8

// kind is the resource type shown in one tab.
type kind int

const (
	kindContainers kind = iota
	kindImages
	kindVolumes
	numKinds
)

func (k kind) String() string {
	switch k {
	case kindContainers:
		return "containers"
	case kindImages:
		return "images"
	case kindVolumes:
		return "volumes"
	default:
		return "unknown"
	}
}

// item is one resource row, normalized across containers/images/volumes so the
// list, filter, and selection logic stay kind-agnostic. Populated by docker.go.
type item struct {
	id      string
	name    string
	image   string   // container: image ref · image: repo:tag · volume: driver
	status  string   // container: state/text · image: size · volume: usage
	created int64    // unix seconds
	size    int64    // bytes; 0 when unknown
	running bool     // container only
	inUse   bool     // image/volume referenced by a container
	usedBy  []string // names of containers holding this resource
}

// warning returns the conflict/dependency note shown before deletion, or "".
func (it item) warning() string {
	switch {
	case it.running:
		return "running — will be force-stopped"
	case it.inUse && len(it.usedBy) > 0:
		who := strings.Join(it.usedBy, ", ")
		if len(who) > 40 {
			who = truncate(who, 40)
		}
		return fmt.Sprintf("in use by %d container(s): %s", len(it.usedBy), who)
	case it.inUse:
		return "in use"
	}
	return ""
}

// dockerAPI is the data seam. docker.go provides the real implementation; tests
// inject a fake so no Docker daemon is needed.
type dockerAPI interface {
	List(k kind) ([]item, error)
	Remove(k kind, id string, force bool) error
}

// state is the top-level screen the TUI is showing.
type state int

const (
	stateSelect   state = iota // browsing/choosing resources
	stateConfirm               // y/N gate before deleting
	stateDeleting              // removes running, results streaming in
	stateDone                  // all removes finished; app about to quit
)

// result is the outcome of one remove, shown live in the deleting view.
type result struct {
	id    string
	title string
	err   error
}

type deleteDoneMsg result

type quitMsg struct{}

// loadedMsg carries the result of a background List call for one tab.
type loadedMsg struct {
	k     kind
	items []item
	err   error
}

type model struct {
	api  dockerAPI
	kind kind

	items  [numKinds][]item
	loaded [numKinds]bool
	lerr   [numKinds]error

	cursor   int
	selected [numKinds]map[int]bool

	filter    string
	filtering bool

	state state

	targets []item // immutable snapshot of the full selection (for rendering)
	queue   []item // pending removes (not yet started)
	delKind kind   // kind being deleted
	total   int    // total removes requested this run
	done    int    // removes finished (results arrive out of order)
	results []result

	vp      viewport.Model
	vpReady bool
	spinner spinner.Model
	w, h    int

	// remove performs a single remove for a kind. Defaults to api.Remove(.., true);
	// swapped for a fake in tests.
	remove func(k kind, id string) error
}

func newModel(api dockerAPI) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accent)

	m := model{
		api:      api,
		kind:     kindContainers,
		selected: [numKinds]map[int]bool{},
		state:    stateSelect,
		spinner:  sp,
		w:        defWidth,
		h:        defHeight,
	}
	for i := range m.selected {
		m.selected[i] = map[int]bool{}
	}
	m.remove = func(k kind, id string) error { return api.Remove(k, id, true) }
	m.vp = viewport.New(defWidth-4, defHeight-8)
	m.vpReady = true
	m.vp.SetContent(m.listContent())
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadCmd(m.kind))
}

// loadCmd fetches one tab's items off the UI goroutine.
func (m model) loadCmd(k kind) tea.Cmd {
	api := m.api
	return func() tea.Msg {
		items, err := api.List(k)
		return loadedMsg{k: k, items: items, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		height := msg.Height - 10 // title, details, help, borders
		if height < 3 {
			height = 3
		}
		m.vp.Width = msg.Width - 4
		m.vp.Height = height
		m.vp.SetContent(m.listContent())
		return m, nil

	case loadedMsg:
		m.items[msg.k] = msg.items
		m.loaded[msg.k] = true
		m.lerr[msg.k] = msg.err
		if msg.k == m.kind {
			m.clampCursor()
			m.vp.SetContent(m.listContent())
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateSelect:
			return m.updateSelect(msg)
		case stateConfirm:
			return m.updateConfirm(msg)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case deleteDoneMsg:
		m.results = append(m.results, result(msg))
		m.done++
		if m.done >= m.total {
			m.state = stateDone
			return m, tea.Tick(doneLinger, func(time.Time) tea.Msg { return quitMsg{} })
		}
		if len(m.queue) > 0 {
			return m, m.deleteNext()
		}
		return m, nil

	case quitMsg:
		return m, tea.Quit
	}
	return m, nil
}

// visibleIndexes returns indices into items[kind] matching the filter
// (case-insensitive substring over name, image, status, id). With no filter
// every index is visible, in order. Single source of truth for cursor→item.
func (m model) visibleIndexes() []int {
	all := m.items[m.kind]
	if m.filter == "" {
		idx := make([]int, len(all))
		for i := range all {
			idx[i] = i
		}
		return idx
	}
	q := strings.ToLower(m.filter)
	var out []int
	for i, it := range all {
		if strings.Contains(strings.ToLower(it.name), q) ||
			strings.Contains(strings.ToLower(it.image), q) ||
			strings.Contains(strings.ToLower(it.status), q) ||
			strings.Contains(strings.ToLower(it.id), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m *model) clampCursor() {
	n := len(m.visibleIndexes())
	switch {
	case n == 0:
		m.cursor = 0
	case m.cursor > n-1:
		m.cursor = n - 1
	case m.cursor < 0:
		m.cursor = 0
	}
}

// switchKind moves to another tab, resetting the per-tab cursor and filter and
// lazily loading the tab's items the first time it is shown.
func (m model) switchKind(k kind) (tea.Model, tea.Cmd) {
	if k < 0 || k >= numKinds || k == m.kind {
		return m, nil
	}
	m.kind = k
	m.cursor = 0
	m.filter = ""
	m.filtering = false
	m.clampCursor()
	m.vp.SetContent(m.listContent())
	if !m.loaded[m.kind] {
		return m, m.loadCmd(m.kind)
	}
	return m, nil
}

func (m model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			m.filtering = false
		case tea.KeyEsc:
			m.filter = ""
			m.filtering = false
		case tea.KeyBackspace:
			if r := []rune(m.filter); len(r) > 0 {
				m.filter = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			m.filter += string(msg.Runes)
		}
		m.clampCursor()
		m.vp.SetContent(m.listContent())
		m.ensureCursorVisible()
		return m, nil
	}

	vis := m.visibleIndexes()
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "1":
		return m.switchKind(kindContainers)
	case "2":
		return m.switchKind(kindImages)
	case "3":
		return m.switchKind(kindVolumes)
	case "tab", "right", "l":
		return m.switchKind((m.kind + 1) % numKinds)
	case "shift+tab", "left", "h":
		return m.switchKind((m.kind - 1 + numKinds) % numKinds)
	case "/":
		m.filtering = true
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(vis)-1 {
			m.cursor++
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if len(vis) > 0 {
			m.cursor = len(vis) - 1
		}
	case " ":
		if m.cursor < len(vis) {
			real := vis[m.cursor]
			m.selected[m.kind][real] = !m.selected[m.kind][real]
		}
	case "a":
		for _, real := range vis {
			m.selected[m.kind][real] = true
		}
	case "n":
		for _, real := range vis {
			delete(m.selected[m.kind], real)
		}
	case "enter":
		if m.countSelected() > 0 {
			m.state = stateConfirm
		}
		return m, nil
	}
	m.vp.SetContent(m.listContent())
	m.ensureCursorVisible()
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.targets = m.selectedItems()
		m.queue = m.targets
		m.delKind = m.kind
		m.total = len(m.targets)
		m.done = 0
		m.state = stateDeleting
		cmds := []tea.Cmd{m.spinner.Tick}
		for i := 0; i < maxConcurrent && len(m.queue) > 0; i++ {
			cmds = append(cmds, m.deleteNext())
		}
		return m, tea.Batch(cmds...)
	case "n", "N", "esc", "ctrl+c":
		m.state = stateSelect
	}
	return m, nil
}

// deleteNext pops one item off the queue and returns a command that removes it.
func (m *model) deleteNext() tea.Cmd {
	it := m.queue[0]
	m.queue = m.queue[1:]
	k := m.delKind
	remove := m.remove
	return func() tea.Msg {
		return deleteDoneMsg{id: it.id, title: it.name, err: remove(k, it.id)}
	}
}

func (m model) countSelected() int {
	n := 0
	for _, v := range m.selected[m.kind] {
		if v {
			n++
		}
	}
	return n
}

// selectedItems returns the selected items of the current tab, in list order.
func (m model) selectedItems() []item {
	var out []item
	for i, it := range m.items[m.kind] {
		if m.selected[m.kind][i] {
			out = append(out, it)
		}
	}
	return out
}

// cursorItem returns the item under the cursor, if any.
func (m model) cursorItem() (item, bool) {
	vis := m.visibleIndexes()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return item{}, false
	}
	return m.items[m.kind][vis[m.cursor]], true
}

func (m *model) ensureCursorVisible() {
	if !m.vpReady {
		return
	}
	top := m.vp.YOffset
	bottom := top + m.vp.Height - 1
	if m.cursor < top {
		m.vp.SetYOffset(m.cursor)
	} else if m.cursor > bottom {
		m.vp.SetYOffset(m.cursor - m.vp.Height + 1)
	}
}

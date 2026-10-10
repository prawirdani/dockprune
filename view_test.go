package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestListContentRendersAllNames(t *testing.T) {
	m := sampleModel()
	out := m.listContent()
	for _, want := range []string{"web", "db", "cache"} {
		if !strings.Contains(out, want) {
			t.Errorf("listContent missing %q\n---\n%s", want, out)
		}
	}
}

func TestCursorMarkerRendered(t *testing.T) {
	m := sampleModel()
	if !strings.Contains(m.listContent(), ">") {
		t.Error("cursor row marker '>' not rendered")
	}
}

func TestListContentStates(t *testing.T) {
	// Loading: not yet loaded.
	m := newModel(&fakeAPI{})
	if out := m.listContent(); !strings.Contains(out, "loading containers") {
		t.Errorf("loading state missing: %q", out)
	}
	// Error.
	m.loaded[kindContainers] = true
	m.lerr[kindContainers] = fmt.Errorf("daemon down")
	if out := m.listContent(); !strings.Contains(out, "error:") {
		t.Errorf("error state missing: %q", out)
	}
	// Empty (loaded, no items).
	m.lerr[kindContainers] = nil
	if out := m.listContent(); !strings.Contains(out, "no containers found") {
		t.Errorf("empty state missing: %q", out)
	}
	// Empty filter match.
	m.items[kindContainers] = []item{{id: "x", name: "alpha"}}
	m.filter = "zzz"
	if out := m.listContent(); !strings.Contains(out, "no containers match") {
		t.Errorf("no-match state missing: %q", out)
	}
}

func TestTabsViewHighlightsActive(t *testing.T) {
	m := sampleModel()
	out := m.tabsView()
	if !strings.Contains(out, "1 containers") || !strings.Contains(out, "2 images") || !strings.Contains(out, "3 volumes") {
		t.Errorf("tabs missing labels:\n%s", out)
	}
	// Container count is known and shown.
	if !strings.Contains(out, "(3)") {
		t.Errorf("active tab should show item count:\n%s", out)
	}
}

func TestViewSelectRendersChrome(t *testing.T) {
	m := sampleModel()
	m.selected[kindContainers][0] = true
	out := m.View()
	for _, want := range []string{"dockprune", "1 containers", "web", "1 selected", "enter delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("viewSelect missing %q\n---\n%s", want, out)
		}
	}
}

func TestViewConfirmShowsWarnings(t *testing.T) {
	m := sampleModel()
	m.selected[kindContainers][0] = true // web: running -> warning
	m.selected[kindContainers][1] = true // db: clean
	m.state = stateConfirm
	out := m.View()
	if !strings.Contains(out, "Delete 2 containers") {
		t.Errorf("confirm header wrong:\n%s", out)
	}
	if !strings.Contains(out, "force-removed") {
		t.Errorf("confirm must surface the dependency warning:\n%s", out)
	}
	if !strings.Contains(out, "[y]") || !strings.Contains(out, "[N]") {
		t.Errorf("confirm prompts missing:\n%s", out)
	}
}

func TestViewDeletingShowsAllTargets(t *testing.T) {
	m := sampleModel()
	m.targets = m.items[kindContainers]
	m.total = 3
	m.state = stateDeleting
	m.results = []result{
		{id: "c1", title: "web"},
		{id: "c2", title: "db", err: fmt.Errorf("in use")},
	}
	m.done = 2

	out := m.viewDeleting()
	for _, want := range []string{"web", "db", "cache", "2 / 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("viewDeleting missing %q\n---\n%s", want, out)
		}
	}
	if !strings.Contains(out, "✓ web") {
		t.Error("completed row missing ✓")
	}
	if !strings.Contains(out, "✗ db") {
		t.Error("failed row missing ✗")
	}
}

func TestViewDoneSummary(t *testing.T) {
	m := sampleModel()
	m.targets = m.items[kindContainers][:2]
	m.total = 2
	m.state = stateDone
	m.results = []result{{id: "c1", title: "web"}, {id: "c2", title: "db", err: fmt.Errorf("x")}}
	out := m.View()
	if !strings.Contains(out, "Deleted 1") || !strings.Contains(out, "Failed 1") {
		t.Errorf("done summary wrong:\n%s", out)
	}
}

func TestListLayoutClamps(t *testing.T) {
	nameW, imageW, statusW, ageW := listLayout(120)
	if nameW < 8 || imageW < 8 || statusW != 12 || ageW != 10 {
		t.Errorf("listLayout(120)=(%d,%d,%d,%d) unexpected", nameW, imageW, statusW, ageW)
	}
	nameW, imageW, _, _ = listLayout(0)
	if nameW < 8 || imageW < 8 {
		t.Errorf("listLayout(0) must clamp to minimums, got (%d,%d)", nameW, imageW)
	}
}

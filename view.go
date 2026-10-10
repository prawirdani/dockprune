package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// listLayout computes the column widths (name, image, status) for a given
// overall terminal width. Age is fixed. Kept separate so the geometry is
// testable and tunable in one place.
func listLayout(width int) (nameW, imageW, statusW, ageW int) {
	total := width - 6
	if total < 30 {
		total = 30
	}
	ageW = 10
	statusW = 12
	imageW = total / 3
	nameW = total - imageW - statusW - ageW - 9
	if nameW < 8 {
		nameW = 8
	}
	if imageW < 8 {
		imageW = 8
	}
	return nameW, imageW, statusW, ageW
}

// listContent renders the scrollable rows (the viewport body). It also renders
// the loading, error, and empty states so the box is never blank.
func (m model) listContent() string {
	if !m.loaded[m.kind] {
		return dimStyle.Render("  loading " + m.kind.String() + "…")
	}
	if m.lerr[m.kind] != nil {
		return errStyle.Render("  error: " + oneLine(m.lerr[m.kind].Error()))
	}
	vis := m.visibleIndexes()
	if len(vis) == 0 {
		if m.filter != "" {
			return dimStyle.Render(fmt.Sprintf("  no %s match %q", m.kind, m.filter))
		}
		return dimStyle.Render("  no " + m.kind.String() + " found")
	}

	nameW, imageW, statusW, ageW := listLayout(m.w)
	var b []byte
	for pos, real := range vis {
		it := m.items[m.kind][real]
		cursor := "  "
		if pos == m.cursor {
			cursor = "> "
		}
		check := "[ ]"
		if m.selected[m.kind][real] {
			check = checkStyle.Render("[✓]")
		}
		name := truncate(it.name, nameW)
		if it.running {
			name = "● " + name // filled dot marks a live container
		}
		nameR := normalRowStyle.Render(fmt.Sprintf("%-*s", nameW+2, name))
		if pos == m.cursor {
			nameR = cursorRowStyle.Render(fmt.Sprintf("%-*s", nameW+2, name))
		}
		line := fmt.Sprintf("%s%s %s %s  %s  %s",
			cursor, check, nameR,
			dimStyle.Render(fmt.Sprintf("%-*s", imageW, truncate(it.image, imageW))),
			dimStyle.Render(fmt.Sprintf("%-*s", statusW, truncate(it.status, statusW))),
			dimStyle.Render(fmt.Sprintf("%*s", ageW, relTime(it.created))),
		)
		b = append(b, line...)
		b = append(b, '\n')
	}
	return string(b)
}

// tabsView renders the tab bar, highlighting the active tab and showing each
// tab's item count.
func (m model) tabsView() string {
	var parts []string
	for k := kind(0); k < numKinds; k++ {
		label := fmt.Sprintf("%d %s", int(k)+1, k)
		if m.loaded[k] {
			label += fmt.Sprintf(" (%d)", len(m.items[k]))
		}
		if k == m.kind {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabStyle.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// detailsView renders the focused item's details and any dependency warning.
func (m model) detailsView() string {
	it, ok := m.cursorItem()
	if !ok {
		return dimStyle.Render("  (no selection)")
	}
	var b strings.Builder
	row := func(label, val string) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %-7s", label)))
		b.WriteString(" " + truncate(val, m.w-12) + "\n")
	}
	row("id", it.id)
	row("image", it.image)
	row("status", it.status)
	detail := "created " + relTime(it.created)
	if it.size > 0 {
		detail += "   size " + humanSize(it.size)
	}
	b.WriteString(dimStyle.Render("  ") + dimStyle.Render(detail))
	if w := it.warning(); w != "" {
		b.WriteString("\n  " + warnStyle.Render("⚠ "+w))
	}
	return b.String()
}

// View dispatches to the per-state renderer.
func (m model) View() string {
	switch m.state {
	case stateConfirm:
		return m.viewConfirm()
	case stateDeleting, stateDone:
		return m.viewDeleting()
	default:
		return m.viewSelect()
	}
}

func (m model) viewSelect() string {
	header := lipgloss.JoinHorizontal(lipgloss.Center,
		titleStyle.Render(" dockprune "), " ", m.tabsView())

	info := dimStyle.Render(fmt.Sprintf("%d %s", len(m.items[m.kind]), m.kind))
	if m.filtering || m.filter != "" {
		matched := len(m.visibleIndexes())
		query := m.filter
		if m.filtering {
			query += "█"
		}
		info = fmt.Sprintf("%s  %s",
			countStyle.Render("/"+query),
			dimStyle.Render(fmt.Sprintf("%d/%d matched", matched, len(m.items[m.kind]))))
	}
	info = lipgloss.JoinHorizontal(lipgloss.Center, info, "  ",
		countStyle.Render(fmt.Sprintf("%d selected", m.countSelected())))

	body := boxStyle.Width(m.w - 2).Render(m.vp.View())
	help := helpStyle.Render(
		"↑/↓ move · space select · a all · n none · / filter · ←/→ tabs · enter delete · q quit")

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", header, info, body, m.detailsView(), help)
}

func (m model) viewConfirm() string {
	sel := m.selectedItems()
	var lines strings.Builder
	warns := 0
	for _, it := range sel {
		mark := okStyle.Render("✓")
		if w := it.warning(); w != "" {
			warns++
			mark = warnStyle.Render("⚠")
			lines.WriteString("  " + mark + " " +
				normalRowStyle.Render(truncate(it.name, m.w-40)) +
				dimStyle.Render("  "+truncate(w, 32)) + "\n")
			continue
		}
		lines.WriteString("  " + mark + " " +
			normalRowStyle.Render(truncate(it.name, m.w-8)) + "\n")
	}

	warn := warnStyle.Render(fmt.Sprintf(
		"⚠  Delete %d %s? This is irreversible.", len(sel), m.kind))
	if warns > 0 {
		warn += "\n" + warnStyle.Render(fmt.Sprintf(
			"   %d item(s) are running or in use and will be force-removed.", warns))
	}
	prompt := fmt.Sprintf("%s  %s", countStyle.Render("[y]"), dimStyle.Render("yes"))
	prompt2 := fmt.Sprintf("%s  %s", countStyle.Render("[N]"), dimStyle.Render("no (default)"))

	box := boxStyle.BorderForeground(yellow).Width(m.w - 2).Render(
		warn + "\n\n" + lines.String() + "\n" + prompt + "    " + prompt2)
	return box
}

func (m model) viewDeleting() string {
	lines := m.renderTargets()

	if m.state == stateDone {
		ok, fail := 0, 0
		for _, r := range m.results {
			if r.err != nil {
				fail++
			} else {
				ok++
			}
		}
		summary := okStyle.Render(fmt.Sprintf("Deleted %d", ok))
		if fail > 0 {
			summary += "  " + errStyle.Render(fmt.Sprintf("Failed %d", fail))
		}
		return boxStyle.Width(m.w - 2).Render(lines + "\n" + summary)
	}

	head := fmt.Sprintf("%s Deleting %s…", m.spinner.View(), m.kind)
	progress := dimStyle.Render(fmt.Sprintf("%d / %d", m.done, m.total))
	return boxStyle.Width(m.w - 2).Render(head + "  " + progress + "\n\n" + lines)
}

// renderTargets renders every target once, matching finished results by id.
func (m model) renderTargets() string {
	var b []byte
	for _, it := range m.targets {
		res, done := m.resultFor(it.id)
		switch {
		case done && res.err != nil:
			b = append(b, errStyle.Render("✗ "+truncate(it.name, m.w-24))...)
			b = append(b, dimStyle.Render(" "+truncate(oneLine(res.err.Error()), 18))...)
		case done:
			b = append(b, okStyle.Render("✓ "+truncate(it.name, m.w-6))...)
		default:
			b = append(b, dimStyle.Render("… "+truncate(it.name, m.w-6))...)
		}
		b = append(b, '\n')
	}
	return string(b)
}

func (m model) resultFor(id string) (result, bool) {
	for _, r := range m.results {
		if r.id == id {
			return r, true
		}
	}
	return result{}, false
}

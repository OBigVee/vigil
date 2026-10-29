package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	ink           = lipgloss.Color("#DCE5F4")
	muted         = lipgloss.Color("#8897AF")
	cyan          = lipgloss.Color("#5DE4C7")
	violet        = lipgloss.Color("#B2A4FF")
	amber         = lipgloss.Color("#F5C777")
	danger        = lipgloss.Color("#F08091")
	base          = lipgloss.Color("#101620")
	edge          = lipgloss.NewStyle().Foreground(lipgloss.Color("#344359"))
	textStyle     = lipgloss.NewStyle().Foreground(ink)
	dim           = lipgloss.NewStyle().Foreground(muted)
	accent        = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("#24374B")).Foreground(ink).Bold(true)
)

func historyAppend(xs []float64, v float64) []float64 {
	next := append(append([]float64(nil), xs...), v)
	if len(next) > 40 {
		next = next[len(next)-40:]
	}
	return next
}
func oneLine(s string) string { return strings.ReplaceAll(clean(s), "\n", " ") }
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
func apart(left, right string, w int) string {
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return fit(left, w)
	}
	return left + strings.Repeat(" ", gap) + right
}

// All dimensions include borders. Content is clipped by terminal cell width.
func panel(title string, lines []string, w, h int) string {
	if w < 6 || h < 3 {
		return ""
	}
	inner := w - 4
	title = ansi.Truncate(title, w-6, "…")
	top := edge.Render("╭─ ") + accent.Render(title) + edge.Render(" "+strings.Repeat("─", max(0, w-5-ansi.StringWidth(title)))+"╮")
	out := []string{top}
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, edge.Render("│")+" "+fit(line, inner)+" "+edge.Render("│"))
	}
	out = append(out, edge.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(out, "\n")
}
func usageColor(v float64) lipgloss.Color {
	if v >= 85 {
		return danger
	}
	if v >= 65 {
		return amber
	}
	return cyan
}
func meter(v float64, w int) string {
	v = math.Max(0, math.Min(100, v))
	filled := int(math.Round(v / 100 * float64(w)))
	return lipgloss.NewStyle().Foreground(usageColor(v)).Render(strings.Repeat("━", filled)) + edge.Render(strings.Repeat("─", w-filled))
}
func spark(xs []float64, w int) string {
	if len(xs) == 0 {
		return dim.Render("waiting for samples")
	}
	if len(xs) > w {
		xs = xs[len(xs)-w:]
	}
	r := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, x := range xs {
		i := int(math.Round(math.Max(0, math.Min(100, x)) * 7 / 100))
		b.WriteRune(r[i])
	}
	return b.String()
}
func (m model) metrics(w int) string {
	if w < 76 {
		return panel("RESOURCES", []string{apart(fmt.Sprintf("CPU  %5.1f%%", m.state.CPU), fmt.Sprintf("RAM  %5.1f%%", m.state.RAM), w-4), apart(fmt.Sprintf("%d processes", len(m.state.Processes)), bytesLabel(m.state.Used)+" / "+bytesLabel(m.state.Total), w-4)}, w, 4)
	}
	card := (w - 2) / 3
	last := w - 2 - 2*card
	cpu := panel("CPU", []string{apart(lipgloss.NewStyle().Foreground(usageColor(m.state.CPU)).Bold(true).Render(fmt.Sprintf("%.1f%%", m.state.CPU)), dim.Render(spark(m.cpuHistory, max(1, card-14))), card-4), meter(m.state.CPU, card-4)}, card, 4)
	ram := panel("MEMORY", []string{apart(fmt.Sprintf("%.1f%%", m.state.RAM), dim.Render(bytesLabel(m.state.Used)+" / "+bytesLabel(m.state.Total)), card-4), meter(m.state.RAM, card-4)}, card, 4)
	ai := "AI offline · local inspect ready"
	if m.ai.ready() {
		ai = "Vigil Intelligence · ready"
	}
	if m.busy {
		ai = "Analyzing · Esc cancels"
		if m.collecting {
			ai = "Collecting evidence · Esc cancels"
		}
	}
	processes := panel("ACTIVITY", []string{apart(fmt.Sprintf("%d processes", len(m.state.Processes)), lipgloss.NewStyle().Foreground(violet).Render(spark(m.ramHistory, max(1, last-20))), last-4), dim.Render(ai)}, last, 4)
	return lipgloss.JoinHorizontal(lipgloss.Top, cpu, " ", ram, " ", processes)
}
func (m model) processPanel(w, h int) string {
	ps := m.rows()
	inner := w - 4
	nameW := max(4, inner-29)
	label := func(s string) string {
		if m.sort == s {
			if s == "name" {
				return " ↑"
			}
			return " ↓"
		}
		return ""
	}
	header := dim.Render(fmt.Sprintf("  %-*s %7s %7s %7s", nameW, "PROCESS"+label("name"), "PID", "CPU%"+label("cpu"), "MEM%"+label("memory")))
	lines := []string{header, edge.Render(strings.Repeat("─", inner))}
	count := max(1, h-4)
	start := max(0, min(m.cursor-count/2, len(ps)-count))
	end := min(len(ps), start+count)
	if len(ps) == 0 {
		lines = append(lines, dim.Render("No matching process name or PID."))
	}
	for i := start; i < end; i++ {
		p := ps[i]
		mark := " "
		if i == m.cursor {
			mark = "▸"
		}
		name := ansi.Truncate(oneLine(p.Name), nameW, "…")
		name = fit(name, nameW)
		row := fmt.Sprintf("%s %s %7d %7.1f %7.2f", mark, name, p.PID, p.CPU, p.Memory)
		if i == m.cursor {
			row = selectedStyle.Render(fit(row, inner))
		} else {
			row = textStyle.Render(fit(row, inner))
		}
		lines = append(lines, row)
	}
	title := fmt.Sprintf("PROCESSES  %d/%d", len(ps), len(m.state.Processes))
	return panel(title, lines, w, h)
}
func (m model) selectionPanel(w, h int) string {
	ps := m.rows()
	lines := []string{}
	if len(ps) > 0 {
		p := ps[min(m.cursor, len(ps)-1)]
		lines = []string{accent.Render(oneLine(p.Name)), dim.Render(fmt.Sprintf("PID %d", p.PID)), "", apart("CPU", fmt.Sprintf("%.1f%%", p.CPU), w-4), apart("Memory", fmt.Sprintf("%.2f%%", p.Memory), w-4), "", dim.Render("PROCESS INVESTIGATION"), "Executable & package", "Parent, children & siblings", "Files & working directory", "Sockets & local peers", "", accent.Render("[i] Collect local evidence"), dim.Render("Review first, then ask AI.")}
	}
	return panel("SELECTED PROCESS", lines, w, h)
}
func evidenceSummary(raw string) string {
	var d investigation
	if json.Unmarshal([]byte(raw), &d) != nil {
		return raw
	}
	lines := []string{"PROCESS IDENTITY", fmt.Sprintf("%s  ·  PID %d  ·  parent %d", d.Process.Name, d.Process.PID, d.Process.ParentPID), "Executable: " + d.Process.Executable, "Owner: " + d.Owner, "Package: " + d.Package, "Working directory: " + d.WorkingDirectory}
	if d.Started != nil {
		lines = append(lines, "Started: "+d.Started.Format("2006-01-02 15:04:05 MST"))
	}
	lines = append(lines, fmt.Sprintf("CPU: %.1f%% over %.2fs  ·  Resident memory: %s", d.CPU, d.SampleSeconds, bytesLabel(d.ResidentBytes)), "", "PROCESS FAMILY")
	if h := d.History; h != nil && len(h.Samples) >= 2 {
		cpuValues, memoryValues := []float64{}, []float64{}
		for _, sample := range h.Samples {
			cpuValues = append(cpuValues, sample.CPU)
			memoryValues = append(memoryValues, sample.Memory)
		}
		// Keep each graph within a conventional terminal width; the outer view wraps text.
		lines = append(lines[:len(lines)-1], "SESSION TRENDS", fmt.Sprintf("%d observations · %s to %s", len(h.Samples), h.Samples[0].Time.Format("15:04:05"), h.Samples[len(h.Samples)-1].Time.Format("15:04:05")), "CPU    "+trendSpark(cpuValues), fmt.Sprintf("CPU average %.1f%% · peak %.1f%% (100%% = one logical CPU)", h.AverageCPU, h.PeakCPU), "Memory "+trendSpark(memoryValues), fmt.Sprintf("Memory change %+.2f percentage points of system RAM", h.MemoryChange), "Graphs use independent relative scales. Gaps may include pauses or unavailable metrics.", "", "PROCESS FAMILY")
	} else {
		lines = append(lines[:len(lines)-1], "Session trends: gathering observations; allow several refreshes before inspecting.", "", "PROCESS FAMILY")
	}
	for _, group := range []struct {
		name string
		ps   []relatedProcess
	}{{"Ancestor", d.Ancestors}, {"Child", d.Children}, {"Sibling", d.Siblings}} {
		for _, p := range group.ps {
			lines = append(lines, fmt.Sprintf("%s · %s [%d] · %s", group.name, p.Name, p.PID, p.Executable))
		}
	}
	for _, group := range []struct {
		name  string
		items []string
	}{{"OPEN FILES", d.OpenFiles}, {"MAPPED FILES", d.Libraries}, {"SERVICE / CONTAINER CGROUPS", d.Cgroups}} {
		lines = append(lines, "", group.name)
		if len(group.items) == 0 {
			lines = append(lines, "No entries returned; check collection notes.")
		}
		lines = append(lines, group.items...)
	}
	lines = append(lines, "", "SOCKETS & CONNECTIONS")
	for _, c := range d.Connections {
		line := fmt.Sprintf("%s  %s:%d → %s:%d  %s", c.Protocol, c.Local.IP, c.Local.Port, c.Remote.IP, c.Remote.Port, c.State)
		if c.LocalPeer != nil {
			line += fmt.Sprintf(" · local peer %s [%d]", c.LocalPeer.Name, c.LocalPeer.PID)
		}
		lines = append(lines, line)
	}
	if len(d.Connections) == 0 {
		lines = append(lines, "No entries returned; check collection notes.")
	}
	lines = append(lines, "", "COLLECTION NOTES")
	lines = append(lines, d.Notes...)
	lines = append(lines, "", "MACHINE", fmt.Sprintf("%s %s · %s · %d logical CPUs", d.Machine.Platform, d.Machine.Version, d.Machine.Arch, d.Machine.LogicalCPUs), "Captured: "+d.Captured.Format("2006-01-02 15:04:05 MST"))
	return strings.Join(lines, "\n")
}
func documentPanel(title, body string, offset, w, h int, notice string, raw bool) string {
	inner := w - 4
	lines := []string{}
	if notice != "" {
		noticeLines := strings.Split(ansi.Hardwrap(notice, inner, true), "\n")
		lines = append(lines, noticeLines[:min(len(noticeLines), max(1, h-6))]...)
		lines = append(lines, "")
	}
	fixed := len(lines) + 1
	available := max(1, h-2-fixed)
	wrapped := strings.Split(ansi.Hardwrap(clean(body), inner, true), "\n")
	start := min(offset, max(0, len(wrapped)-available))
	end := min(len(wrapped), start+available)
	for _, line := range wrapped[start:end] {
		rendered := textStyle.Render(line)
		if !raw {
			trim := strings.TrimSpace(line)
			if trim != "" && len(trim) < 65 && strings.ToUpper(trim) == trim {
				rendered = accent.Render(line)
			}
			if strings.HasPrefix(trim, "#") {
				rendered = accent.Render(strings.TrimLeft(trim, "# "))
			}
		}
		lines = append(lines, rendered)
	}
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	lines = append(lines, dim.Render(apart("↑↓ scroll · PgUp/PgDn", fmt.Sprintf("%d–%d / %d", start+1, end, len(wrapped)), inner)))
	return panel(title, lines, w, h)
}
func (m model) View() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 90
	}
	if h <= 0 {
		h = 28
	}
	if w < 48 || h < 16 {
		return fit("VIGIL · enlarge terminal to 48 × 16 · q quits", w)
	}
	status := accent.Render("● LIVE")
	if m.paused {
		status = lipgloss.NewStyle().Foreground(amber).Render("Ⅱ PAUSED")
	}
	if m.state.Time.IsZero() {
		status = dim.Render("○ SAMPLING")
	}
	clock := ""
	if !m.state.Time.IsZero() {
		clock = m.state.Time.Format("15:04:05")
	}
	header := apart(accent.Render("◈ VIGIL")+dim.Render("  /  SYSTEM INTELLIGENCE"), status+"  "+dim.Render(clock), w)
	filter := "All processes"
	if m.filter != "" {
		filter = "Name / PID: " + oneLine(m.filter)
	}
	if m.filtering {
		filter = "Search name / PID › " + oneLine(m.filter) + "▏"
	}
	subtitle := apart(dim.Render(filter), dim.Render("sort: "+m.sort), w)
	top := header + "\n" + subtitle + "\n" + m.metrics(w) + "\n"
	bodyHeight := h - lipgloss.Height(top) - 3
	mapOn := w >= 100 && bodyHeight >= 10 && !m.hideTree
	mapWidth := max(32, min(44, w/3))
	contentWidth := w
	if mapOn {
		contentWidth = w - mapWidth - 1
	}
	var body, help string
	if m.busy {
		body = m.workPanel(contentWidth, bodyHeight)
		help = "Esc cancel  ·  t toggle tree  ·  [/] scroll tree  ·  q quit"
	} else if m.pending != "" {
		title := "REVIEW SNAPSHOT"
		content := m.pending
		raw := true
		if m.localEvidence {
			title = "PROCESS INVESTIGATION · OVERVIEW"
			if m.rawEvidence {
				title = "PROCESS INVESTIGATION · EXACT JSON"
			} else {
				content = evidenceSummary(m.pending)
				raw = false
			}
		}
		body = documentPanel(title, content, m.offset, contentWidth, bodyHeight, "EVIDENCE READY · Review → Enter for AI interpretation. Paths and endpoints may be private; API charges apply.", raw)
		help = "Enter explain  ·  Tab overview/JSON  ·  Esc back"
	} else if m.answer != "" {
		title := "INTELLIGENCE · FINDINGS"
		notice := "AI interpretation · verify conclusions against the collected evidence."
		if m.answerError {
			title = "REQUEST FAILED · DETAILS"
			notice = "The operation did not complete. Esc returns to the monitor."
		}
		body = documentPanel(title, m.answer, m.offset, contentWidth, bodyHeight, notice, false)
		help = "↑↓ scroll  ·  PgUp/PgDn page  ·  Esc back  ·  q quit"
	} else {
		body = m.processPanel(contentWidth, bodyHeight)
		help = "↑↓ select  c/m/n sort  / search  Space pause  i inspect  q quit"
	}
	if mapOn {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.treePanel(mapWidth, bodyHeight), " ", body)
	}
	if w < 100 && m.treeOnly && !m.busy {
		body = m.treePanel(w, bodyHeight)
	}
	if m.pending == "" && m.answer == "" && !m.busy {
		help = "↑↓ select  i inspect  t tree  [/] scroll tree  / search  q quit"
	}
	if m.filtering {
		help = "Type name or PID · Enter apply · Esc clear · Ctrl+U erase"
	}
	activity := "Ready · e inspect / explain  ·  d diagnose  ·  a triage"
	if m.cancelling {
		activity = "Cancelling the current operation…"
	} else if m.collecting {
		activity = "Collecting process evidence…  Esc cancels"
	} else if m.busy {
		activity = "Vigil Intelligence is analyzing…  Esc cancels"
	} else if m.notice != "" {
		activity = m.notice
	} else if m.state.Warning != "" {
		activity = oneLine(m.state.Warning)
	} else if m.pending != "" {
		activity = "Nothing is sent until confirmed. Tab: exact payload · t: tree · [/]: scroll tree."
	}
	output := top + "\n" + body + "\n" + dim.Render(fit(activity, w)) + "\n" + fit(help, w)
	rows := strings.Split(output, "\n")
	for i, line := range rows {
		rows[i] = lipgloss.NewStyle().Background(base).Foreground(ink).Render(fit(line, w))
	}
	return strings.Join(rows, "\n")
}

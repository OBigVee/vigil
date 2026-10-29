package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/joho/godotenv"
)

type sampleMsg Snapshot
type tickMsg struct{}
type answerMsg struct {
	text string
	err  error
}
type model struct {
	collector                     *collector
	ai                            aiClient
	state                         Snapshot
	width, height, cursor, offset int
	sort, filter                  string
	filtering, paused, busy       bool
	pending, task, answer         string
	cancel                        context.CancelFunc
}

func (m model) sample() tea.Cmd { return func() tea.Msg { return sampleMsg(m.collector.collect()) } }
func tick() tea.Cmd             { return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }
func (m model) Init() tea.Cmd   { return m.sample() }
func (m model) rows() []Process {
	ps := []Process{}
	for _, p := range m.state.Processes {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(m.filter)) {
			ps = append(ps, p)
		}
	}
	sortProcesses(ps, m.sort)
	return ps
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
	case sampleMsg:
		var pid int32
		old := m.rows()
		if m.cursor < len(old) {
			pid = old[m.cursor].PID
		}
		m.state = Snapshot(v)
		for i, p := range m.rows() {
			if p.PID == pid {
				m.cursor = i
				break
			}
		}
		return m, tick()
	case tickMsg:
		if m.paused {
			return m, tick()
		}
		return m, m.sample()
	case answerMsg:
		m.busy = false
		m.cancel = nil
		if v.err != nil {
			m.answer = v.err.Error()
		} else {
			m.answer = clean(v.text)
		}
		m.offset = 0
	case tea.KeyMsg:
		key := v.String()
		if key == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.pending != "" {
			switch key {
			case "esc", "n":
				m.pending = ""
			case "down", "j":
				m.offset++
			case "up", "k":
				m.offset = max(0, m.offset-1)
			case "enter", "y":
				payload, task := m.pending, m.task
				m.pending = ""
				m.busy = true
				ctx, cancel := context.WithCancel(context.Background())
				m.cancel = cancel
				return m, func() tea.Msg { s, e := m.ai.ask(ctx, task, payload); return answerMsg{s, e} }
			}
			return m, nil
		}
		if m.filtering {
			switch key {
			case "esc", "enter":
				m.filtering = false
			case "backspace":
				r := []rune(m.filter)
				if len(r) > 0 {
					m.filter = string(r[:len(r)-1])
				}
			default:
				if len(v.Runes) > 0 {
					m.filter += string(v.Runes)
				}
			}
			m.cursor = 0
			return m, nil
		}
		if m.busy && key == "esc" {
			m.cancel()
			return m, nil
		}
		if m.answer != "" {
			switch key {
			case "esc":
				m.answer = ""
			case "j", "down":
				m.offset++
			case "k", "up":
				m.offset = max(0, m.offset-1)
			case "q":
				return m, tea.Quit
			}
			return m, nil
		}
		switch key {
		case "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case " ":
			m.paused = !m.paused
		case "/":
			m.filtering = true
		case "esc":
			m.filter = ""
			m.cursor = 0
		case "j", "down":
			m.cursor = min(m.cursor+1, max(0, len(m.rows())-1))
		case "k", "up":
			m.cursor = max(0, m.cursor-1)
		case "c":
			m.sort = "cpu"
			m.cursor = 0
		case "m":
			m.sort = "memory"
			m.cursor = 0
		case "n":
			m.sort = "name"
			m.cursor = 0
		case "e", "d", "a":
			if m.busy {
				return m, nil
			}
			if m.ai.key == "" {
				m.answer = "Set DEEPSEEK_API_KEY in your environment, then restart Vigil."
				return m, nil
			}
			var payload any
			if key == "e" {
				ps := m.rows()
				if len(ps) == 0 {
					return m, nil
				}
				m.cursor = min(m.cursor, len(ps)-1)
				payload = ps[m.cursor]
				m.task = "Explain this process name, its usual purpose, and how to verify its identity. A name alone is not proof of identity."
			} else {
				s := m.state
				s.Processes = append([]Process(nil), s.Processes...)
				sortProcesses(s.Processes, "cpu")
				if len(s.Processes) > 30 {
					s.Processes = s.Processes[:30]
				}
				payload = s
				m.task = "Assess this snapshot for resource pressure and possible causes of slowness. Explain missing evidence."
				if key == "a" {
					m.task = "Triage unusual resource usage and potentially suspicious process names. No trusted baseline is available. Do not infer malware from names alone. Suggest evidence to collect."
				}
			}
			b, _ := json.MarshalIndent(payload, "", "  ")
			m.pending = string(b)
			m.offset = 0
		}
	}
	m.cursor = min(m.cursor, max(0, len(m.rows())-1))
	return m, nil
}
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if unicode.IsControl(r) || r == '\u202e' || r == '\u202d' {
			return -1
		}
		return r
	}, s)
}

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)

func (m model) View() string {
	w, h := m.width, m.height
	if w == 0 {
		w = 90
	}
	if h == 0 {
		h = 28
	}
	if w < 48 || h < 12 {
		return "Vigil: enlarge terminal to at least 48 × 12. q to quit."
	}
	var b strings.Builder
	b.WriteString(accent.Render("VIGIL  /  SYSTEM INTELLIGENCE") + "\n")
	status := "LIVE"
	if m.paused {
		status = "PAUSED"
	}
	if m.state.Time.IsZero() {
		status = "SAMPLING"
	}
	fmt.Fprintf(&b, "%s  CPU %5.1f%%  RAM %5.1f%%  %s / %s\n", status, m.state.CPU, m.state.RAM, bytesLabel(m.state.Used), bytesLabel(m.state.Total))
	fmt.Fprintf(&b, "%d processes • sort: %s • filter: %s\n", len(m.state.Processes), m.sort, clean(m.filter))
	if m.pending != "" {
		b.WriteString(accent.Render("REVIEW DATA → DEEPSEEK (API charges may apply)") + "\n")
		b.WriteString("Only the JSON below is sent with diagnostic instructions.\n")
		lines := strings.Split(m.pending, "\n")
		start := min(m.offset, max(0, len(lines)-(h-8)))
		for _, l := range lines[start:min(len(lines), start+h-8)] {
			b.WriteString(ansi.Truncate(clean(l), w, "") + "\n")
		}
		if len(lines) > h-8 {
			b.WriteString("↑/↓ scroll to review the full payload.\n")
		}
		b.WriteString("Enter/y send • Esc/n cancel • ↑/↓ scroll")
	} else if m.answer != "" {
		b.WriteString(accent.Render("DIAGNOSTIC • AI output requires verification") + "\n")
		lines := strings.Split(ansi.Wrap(m.answer, w, ""), "\n")
		start := min(m.offset, max(0, len(lines)-(h-7)))
		for _, l := range lines[start:min(len(lines), start+h-7)] {
			b.WriteString(l + "\n")
		}
		b.WriteString("↑/↓ scroll • Esc back • q quit")
	} else {
		b.WriteString(accent.Render(fmt.Sprintf("  %-8s %7s %7s  %s", "PID", "CPU%", "MEM%", "PROCESS")) + "\n")
		ps := m.rows()
		count := h - 8
		start := max(0, m.cursor-count+1)
		for i := start; i < min(len(ps), start+count); i++ {
			p := ps[i]
			mark := " "
			if i == m.cursor {
				mark = "›"
			}
			line := fmt.Sprintf("%s %-8d %7.1f %7.2f  %s", mark, p.PID, p.CPU, p.Memory, clean(p.Name))
			line = ansi.Truncate(line, w, "")
			if i == m.cursor {
				line = accent.Render(line)
			}
			b.WriteString(line + "\n")
		}
		if m.state.Warning != "" {
			b.WriteString(ansi.Truncate(clean(m.state.Warning), w, "") + "\n")
		}
		if m.busy {
			b.WriteString("DeepSeek is analyzing… Esc cancels\n")
		}
		b.WriteString("↑↓ select • c/m/n sort • / filter • Space pause\n")
		b.WriteString("e explain • d diagnose • a triage • q quit")
	}
	return b.String()
}
func main() {
	snapshot := flag.Bool("snapshot", false, "print a JSON snapshot and exit")
	flag.Parse()
	c := &collector{}
	if *snapshot {
		c.collect()
		time.Sleep(time.Second)
		s := c.collect()
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if e := enc.Encode(s); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Vigil: could not load .env; check its permissions and KEY=value syntax")
		os.Exit(1)
	}
	m := model{collector: c, ai: newAIClient(), sort: "cpu"}
	if _, e := tea.NewProgram(m, tea.WithAltScreen()).Run(); e != nil {
		fmt.Fprintln(os.Stderr, "Vigil:", e)
		os.Exit(1)
	}
}

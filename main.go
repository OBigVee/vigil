package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

type sampleMsg Snapshot
type tickMsg struct{}
type workTickMsg struct{ id uint64 }
type investigationMsg struct {
	data investigation
	err  error
}
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
	collecting, localEvidence     bool
	rawEvidence                   bool
	hideTree, treeOnly            bool
	treeOffset                    int
	evidence                      *investigation
	cpuHistory, ramHistory        []float64
	pending, task, answer         string
	cancel                        context.CancelFunc
	workID                        uint64
	workFrame, workStage          int
	workStarted                   time.Time
	workProcess                   Process
	workProgress                  <-chan int
	cancelling, answerError       bool
	notice                        string
}

func (m model) sample() tea.Cmd { return func() tea.Msg { return sampleMsg(m.collector.collect()) } }
func tick() tea.Cmd             { return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }
func workTick(id uint64) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return workTickMsg{id} })
}
func (m model) Init() tea.Cmd { return m.sample() }

// Names match case-insensitively; PID lookup matches the complete numeric ID.
func matchesProcess(p Process, query string) bool {
	query = strings.TrimSpace(query)
	return query == "" || query == strconv.FormatInt(int64(p.PID), 10) || strings.Contains(strings.ToLower(p.Name), strings.ToLower(query))
}
func (m model) rows() []Process {
	ps := []Process{}
	for _, p := range m.state.Processes {
		if matchesProcess(p, m.filter) {
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
		m.cpuHistory = historyAppend(m.cpuHistory, m.state.CPU)
		m.ramHistory = historyAppend(m.ramHistory, m.state.RAM)
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
	case workTickMsg:
		if !m.busy || v.id != m.workID {
			return m, nil
		}
		m.workFrame++
		if m.workProgress != nil {
			for {
				select {
				case stage := <-m.workProgress:
					m.workStage = stage
				default:
					return m, workTick(m.workID)
				}
			}
		}
		return m, workTick(m.workID)
	case investigationMsg:
		m.workProgress = nil
		m.busy = false
		m.collecting = false
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.offset = 0
		if v.err != nil || m.cancelling {
			if errors.Is(v.err, context.Canceled) || m.cancelling {
				m.cancelling = false
				m.notice = "Investigation cancelled. No data was sent."
				return m, nil
			}
			m.answerError = true
			m.answer = "Investigation: " + v.err.Error()
			return m, nil
		}
		b, err := json.MarshalIndent(v.data, "", "  ")
		if err != nil {
			m.answerError = true
			m.answer = "Could not format investigation."
			return m, nil
		}
		m.answerError = false
		m.cancelling = false
		m.pending = string(b)
		m.evidence = &v.data
		m.treeOffset = 0
		m.localEvidence = true
		m.rawEvidence = false
		m.task = explanationTask
		return m, nil
	case answerMsg:
		if m.cancel != nil {
			m.cancel()
		}
		m.busy = false
		m.cancel = nil
		if v.err != nil || m.cancelling {
			if errors.Is(v.err, context.Canceled) || m.cancelling {
				m.cancelling = false
				m.notice = "Interpretation cancelled. The confirmed evidence may already have been submitted."
				return m, nil
			}
			m.answerError = true
			m.answer = v.err.Error()
		} else {
			m.answerError = false
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
		if !m.filtering {
			switch key {
			case "t":
				if m.width >= 100 {
					m.hideTree = !m.hideTree
				} else {
					m.treeOnly = !m.treeOnly
				}
				return m, nil
			case "[":
				m.treeOffset = max(0, m.treeOffset-1)
				return m, nil
			case "]":
				m.treeOffset++
				return m, nil
			}
		}
		if m.busy {
			switch key {
			case "esc":
				if m.cancel != nil {
					m.cancel()
				}
				m.cancelling = true
			case "q":
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
			return m, nil
		}
		if m.pending != "" {
			switch key {
			case "esc", "n":
				m.pending = ""
			case "tab":
				m.rawEvidence = !m.rawEvidence
				m.offset = 0
			case "pgdown", "ctrl+d":
				m.offset += max(1, m.height-15)
			case "pgup", "ctrl+u":
				m.offset = max(0, m.offset-max(1, m.height-15))
			case "down", "j":
				m.offset++
			case "up", "k":
				m.offset = max(0, m.offset-1)
			case "enter", "y":
				if m.ai.key == "" {
					m.answerError = true
					m.answer = "Configure intelligence access in .env, then restart Vigil. Local inspection works without a key."
					m.pending = ""
					return m, nil
				}
				payload, task := m.pending, m.task
				m.pending = ""
				m.busy = true
				m.workID++
				m.workFrame = 0
				m.workStarted = time.Now()
				m.cancelling = false
				m.notice = ""
				ctx, cancel := context.WithCancel(context.Background())
				m.cancel = cancel
				return m, tea.Batch(func() tea.Msg { s, e := m.ai.ask(ctx, task, payload); return answerMsg{s, e} }, workTick(m.workID))
			}
			return m, nil
		}
		if m.filtering {
			switch key {
			case "esc":
				m.filter = ""
				m.filtering = false
			case "enter":
				m.filtering = false
			case "ctrl+u":
				m.filter = ""
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
			case "pgdown", "ctrl+d":
				m.offset += max(1, m.height-15)
			case "pgup", "ctrl+u":
				m.offset = max(0, m.offset-max(1, m.height-15))
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
		case "i", "e":
			if m.busy {
				return m, nil
			}
			ps := m.rows()
			if len(ps) == 0 {
				return m, nil
			}
			selected := ps[min(m.cursor, len(ps)-1)]
			m.localEvidence = false
			m.treeOffset = 0
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			m.cancel = cancel
			m.busy = true
			m.collecting = true
			m.workID++
			m.workFrame = 0
			m.workStage = 0
			m.workStarted = time.Now()
			m.workProcess = selected
			m.cancelling = false
			m.notice = ""
			m.answerError = false
			m.treeOnly = false
			progress := make(chan int, 16)
			m.workProgress = progress
			return m, tea.Batch(func() tea.Msg {
				d, err := investigate(ctx, selected.PID, func(stage int) {
					select {
					case progress <- stage:
					default:
					}
				})
				return investigationMsg{d, err}
			}, workTick(m.workID))
		case "d", "a":
			if m.busy {
				return m, nil
			}
			if m.ai.key == "" {
				m.answerError = true
				m.answer = "Configure intelligence access in .env, then restart Vigil."
				return m, nil
			}
			var payload any
			s := m.state
			s.Processes = append([]Process(nil), s.Processes...)
			sortProcesses(s.Processes, "cpu")
			if len(s.Processes) > 30 {
				s.Processes = s.Processes[:30]
			}
			payload = s
			m.localEvidence = false
			m.task = "Assess this snapshot for resource pressure and possible causes of slowness. Explain missing evidence."
			if key == "a" {
				m.task = "Triage unusual resource usage and potentially suspicious process names. No trusted baseline is available. Do not infer malware from names alone. Suggest evidence to collect."
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

func main() {
	snapshot := flag.Bool("snapshot", false, "print a JSON snapshot and exit")
	search := flag.String("search", "", "filter processes by name or exact PID")
	inspectPID := flag.Int("inspect", 0, "collect local evidence for a PID as JSON, without an AI request")
	flag.Parse()
	if *inspectPID != 0 {
		if *inspectPID < 1 || int64(*inspectPID) > 2147483647 {
			fmt.Fprintln(os.Stderr, "Vigil: invalid PID")
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		d, err := investigate(ctx, int32(*inspectPID))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Vigil:", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(d); err != nil {
			fmt.Fprintln(os.Stderr, "Vigil:", err)
			os.Exit(1)
		}
		return
	}
	c := &collector{}
	if *snapshot {
		c.collect()
		time.Sleep(time.Second)
		s := c.collect()
		if strings.TrimSpace(*search) != "" {
			s.Processes = (model{state: s, filter: *search, sort: "cpu"}).rows()
		}
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
	m := model{collector: c, ai: newAIClient(), sort: "cpu", filter: strings.TrimSpace(*search)}
	if _, e := tea.NewProgram(m, tea.WithAltScreen()).Run(); e != nil {
		fmt.Fprintln(os.Stderr, "Vigil:", e)
		os.Exit(1)
	}
}

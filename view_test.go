package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestResponsiveViews(t *testing.T) {
	b, _ := json.MarshalIndent(investigation{Process: relatedProcess{PID: 42, Name: "worker", Executable: "/opt/" + strings.Repeat("long-path/", 30)}, Notes: []string{"example note"}}, "", "  ")
	for _, size := range [][2]int{{48, 16}, {60, 20}, {90, 28}, {120, 40}, {180, 50}} {
		for _, mode := range []string{"table", "overview", "json", "answer", "busy", "filter", "tree", "interpreting", "cancelling"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], mode), func(t *testing.T) {
				m := model{width: size[0], height: size[1], sort: "cpu", state: Snapshot{Time: time.Now(), CPU: 42, RAM: 55, Total: 16 << 30, Used: 8 << 30}}
				for i := 0; i < 80; i++ {
					m.state.Processes = append(m.state.Processes, Process{PID: int32(i + 1), Name: strings.Repeat("worker", 20), CPU: float64(i), Memory: 1})
				}
				m.cursor = 40
				switch mode {
				case "overview", "json":
					m.pending = string(b)
					m.localEvidence = true
					m.rawEvidence = mode == "json"
				case "answer":
					m.answer = strings.Repeat("A long explanation with /paths/to/files. ", 100)
					m.offset = 10000
				case "interpreting":
					m.busy = true
				case "cancelling":
					m.busy = true
					m.cancelling = true
				case "busy":
					m.busy = true
					m.collecting = true
				case "tree":
					m.treeOnly = true
					m.treeOffset = 10000
				case "filter":
					m.filtering = true
					m.filter = "no-match"
				}
				view := m.View()
				if lipgloss.Height(view) > size[1] {
					t.Fatalf("view exceeds terminal height: %d", lipgloss.Height(view))
				}
				for _, line := range strings.Split(view, "\n") {
					if ansi.StringWidth(line) > size[0] {
						t.Fatalf("line exceeds terminal width: %d", ansi.StringWidth(line))
					}
				}
				if (mode == "overview" || mode == "json") && !strings.Contains(ansi.Strip(view), "Enter explain") {
					t.Fatal("confirmation controls must stay visible")
				}
			})
		}
	}
}
func TestEvidenceOverviewKeepsPaths(t *testing.T) {
	path := "/home/example/project/data.db"
	b, _ := json.Marshal(investigation{OpenFiles: []string{path}})
	if !strings.Contains(evidenceSummary(string(b)), path) {
		t.Fatal("overview lost file path")
	}
}

package main

import (
	"fmt"
	"strings"
	"time"
)

var collectionStages = []string{
	"Identify process and machine",
	"Sample CPU and memory",
	"Trace parents, children and siblings",
	"Read open file paths",
	"Inspect sockets and local peers",
	"Look up installed package",
	"Read mapped files and service groups",
	"Verify process identity",
}
var workFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m model) workPanel(w, h int) string {
	frame := workFrames[m.workFrame%len(workFrames)]
	elapsed := time.Duration(0)
	if !m.workStarted.IsZero() {
		elapsed = time.Since(m.workStarted).Truncate(time.Second)
	}
	title := "INVESTIGATION · COLLECTING"
	target := nodeLabel(m.workProcess.Name, m.workProcess.PID)
	headline := "Collecting evidence from your machine"
	note := "Local collection only. No evidence has been sent for interpretation."
	active := collectionStages[min(m.workStage, len(collectionStages)-1)]
	if !m.collecting {
		title = "INTELLIGENCE · INTERPRETING"
		target = "System snapshot"
		if m.localEvidence && m.evidence != nil {
			target = nodeLabel(m.evidence.Process.Name, m.evidence.Process.PID)
		}
		headline = "Vigil Intelligence is interpreting your evidence"
		active = "Preparing insights for your machine"
		note = "Vigil Intelligence is working. Monitoring continues."
	}
	if m.cancelling {
		headline = "Cancelling request…"
		active = "Waiting for the current operation to stop"
	}
	lines := []string{accent.Render(frame + " " + headline), dim.Render(oneLine(target)), "", textStyle.Render(active), dim.Render(fmt.Sprintf("Elapsed %s · Esc cancels · q quits", elapsed))}
	if h < 12 {
		lines = []string{accent.Render(frame + " " + headline), dim.Render(oneLine(target) + " · " + elapsed.String()), textStyle.Render(active)}
	}
	if h >= 20 {
		lines = append(lines, "", dim.Render("COLLECTION → REVIEW → INTERPRETATION"), "")
		if m.collecting {
			for i, stage := range collectionStages {
				mark := "○"
				style := dim
				if i < m.workStage {
					mark = "✓"
					style = textStyle
				}
				if i == m.workStage {
					mark = frame
					style = accent
				}
				lines = append(lines, style.Render(mark+" "+stage))
			}
		} else {
			lines = append(lines, textStyle.Render("✓ Evidence collected and reviewed"), accent.Render(frame+" Awaiting Vigil Intelligence insights"), dim.Render("○ Findings ready to read"), "", dim.Render("Your insights will appear here when ready."))
		}
	}
	// Keep the current operation and cancellation controls visible on compact screens.
	innerHeight := h - 2
	if len(lines) >= innerHeight {
		lines = lines[:max(1, innerHeight-1)]
	}
	for len(lines) < innerHeight-1 {
		lines = append(lines, "")
	}
	lines = append(lines, dim.Render(strings.TrimSpace(note)))
	return panel(title, lines, w, h)
}

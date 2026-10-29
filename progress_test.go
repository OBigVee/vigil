package main

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestWorkTicksIgnoreOldRequests(t *testing.T) {
	events := make(chan int, 8)
	events <- 2
	events <- 4
	m := model{busy: true, collecting: true, workID: 7, workProgress: events}
	next, cmd := m.Update(workTickMsg{id: 6})
	m = next.(model)
	if cmd != nil || m.workStage != 0 {
		t.Fatal("stale animation changed current request")
	}
	next, cmd = m.Update(workTickMsg{id: 7})
	m = next.(model)
	if cmd == nil || m.workStage != 4 || m.workFrame != 1 {
		t.Fatal("progress tick did not consume actual collector events")
	}
	m.busy = false
	_, cmd = m.Update(workTickMsg{id: 7})
	if cmd != nil {
		t.Fatal("finished work must stop animation")
	}
}
func TestWorkPanelStates(t *testing.T) {
	m := model{width: 90, height: 28, busy: true, collecting: true, workStarted: time.Now(), workProcess: Process{PID: 42, Name: "agent"}, workStage: 4}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "COLLECTING") || !strings.Contains(view, "Inspect sockets and local peers") || !strings.Contains(view, "agent [42]") {
		t.Fatal("missing collection state")
	}
	m.collecting = false
	view = ansi.Strip(m.View())
	if !strings.Contains(view, "INTERPRETING") || !strings.Contains(view, "Preparing insights for your machine") {
		t.Fatal("missing interpretation state")
	}
	m.cancelling = true
	if !strings.Contains(ansi.Strip(m.View()), "Cancelling request") {
		t.Fatal("missing cancellation feedback")
	}
}
func TestCancellationDoesNotOpenStaleEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := model{busy: true, collecting: true, cancel: cancel}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if ctx.Err() != context.Canceled || !m.cancelling {
		t.Fatal("escape must cancel and show feedback")
	}
	next, _ = m.Update(investigationMsg{data: investigation{}})
	m = next.(model)
	if m.pending != "" || m.busy || m.cancelling || !strings.Contains(m.notice, "cancelled") {
		t.Fatal("late success after cancellation must not open a review")
	}
}

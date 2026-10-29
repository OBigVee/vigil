package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestProcessSearch(t *testing.T) {
	p := Process{PID: 1234, Name: "Firefox Web Content"}
	for _, tc := range []struct {
		query string
		want  bool
	}{{"", true}, {"fox", true}, {"FIREFOX", true}, {" 1234 ", true}, {"123", false}, {"12345", false}, {"chrome", false}} {
		if got := matchesProcess(p, tc.query); got != tc.want {
			t.Errorf("query %q: got %v", tc.query, got)
		}
	}
	if !matchesProcess(Process{PID: 9, Name: "worker123"}, "123") {
		t.Fatal("numeric names must remain searchable")
	}
}
func TestSearchInput(t *testing.T) {
	m := model{state: Snapshot{Processes: []Process{{PID: 42, Name: "agent"}, {PID: 142, Name: "other"}}}}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("42")})
	m = next.(model)
	if !m.filtering || len(m.rows()) != 1 || m.rows()[0].PID != 42 {
		t.Fatal("PID search did not filter immediately")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.filtering || m.filter != "42" {
		t.Fatal("Enter must preserve applied search")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if m.filter != "" || len(m.rows()) != 2 {
		t.Fatal("Esc must restore all processes")
	}
	m.filtering = true
	m.filter = "agent"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(model)
	if m.filter != "" || !m.filtering {
		t.Fatal("Ctrl+U must clear input without closing it")
	}
	m.filter = "agent"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if m.filter != "" || m.filtering {
		t.Fatal("Esc while editing must clear and close search")
	}
}

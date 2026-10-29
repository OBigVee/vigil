package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAIRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authorization")
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "test-model" || len(body.Messages) != 2 || !strings.Contains(body.Messages[1].Content, "sample") {
			t.Error("invalid request")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Explanation"}}]}`))
	}))
	defer server.Close()
	c := aiClient{"test-key", "test-model", server.URL, server.Client()}
	result, err := c.ask(context.Background(), "explain", "sample")
	if err != nil || result != "Explanation" {
		t.Fatalf("%q %v", result, err)
	}
}
func TestAIFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"unauthorized", 401, "secret detail"}, {"empty", 200, `{"choices":[]}`}, {"malformed", 200, "garbage"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer s.Close()
			c := aiClient{"key", "model", s.URL, s.Client()}
			_, err := c.ask(context.Background(), "", "")
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), "secret detail") {
				t.Fatal("raw server detail leaked")
			}
		})
	}
}
func TestReviewBeforeSend(t *testing.T) {
	m := model{sort: "cpu", state: Snapshot{Processes: []Process{{PID: 1, Name: "example"}}}}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = next.(model)
	if cmd == nil || !m.collecting || m.pending != "" {
		t.Fatal("inspection must run asynchronously without an API key")
	}
	defer m.cancel()
	next, cmd = m.Update(investigationMsg{data: investigation{Process: relatedProcess{PID: 1, Name: "example"}}})
	m = next.(model)
	if cmd != nil || m.pending == "" || m.busy {
		t.Fatal("evidence must wait for review")
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || next.(model).pending != "" {
		t.Fatal("cancel must dismiss review")
	}
}
func TestSortAndFilter(t *testing.T) {
	m := model{sort: "memory", filter: "WORK", state: Snapshot{Processes: []Process{{PID: 1, Name: "worker", Memory: 1}, {PID: 2, Name: "other", Memory: 9}, {PID: 3, Name: "worker2", Memory: 4}}}}
	ps := m.rows()
	if len(ps) != 2 || ps[0].PID != 3 {
		t.Fatal(ps)
	}
	if strings.Contains(clean("bad\x1b\x00name"), "\x1b") {
		t.Fatal("control character survived")
	}
}

func TestCollector(t *testing.T) {
	c := &collector{}
	first := c.collect()
	second := c.collect()
	if first.Time.IsZero() || second.Time.Before(first.Time) {
		t.Fatal("invalid sample timestamps")
	}
	if second.Total == 0 {
		t.Skip("memory metrics unavailable in this environment")
	}
	if second.Used > second.Total {
		t.Fatal("used memory exceeds total")
	}
	if second.CPU < 0 || second.CPU > 100 {
		t.Fatalf("invalid aggregate CPU: %v", second.CPU)
	}
}

func TestMachineEvidenceForwardedAfterReview(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Messages) != 2 || !strings.Contains(body.Messages[1].Content, "/opt/example/agent") || !strings.Contains(body.Messages[1].Content, "particular process instance") {
			t.Error("machine evidence or explanation task missing")
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"This instance runs /opt/example/agent."}}]}`))
	}))
	defer server.Close()
	m := model{ai: aiClient{"test", "test-model", server.URL, server.Client()}}
	next, cmd := m.Update(investigationMsg{data: investigation{Process: relatedProcess{PID: 42, Name: "agent", Executable: "/opt/example/agent"}}})
	m = next.(model)
	if cmd != nil || calls != 0 {
		t.Fatal("review must not send a request")
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if cmd == nil || !m.busy {
		t.Fatal("confirmation should start request")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatal("expected API request and animation tick")
	}
	result := batch[0]()
	next, _ = m.Update(result)
	if calls != 1 || !strings.Contains(next.(model).answer, "/opt/example/agent") {
		t.Fatal("expected the reviewed evidence explanation")
	}
}

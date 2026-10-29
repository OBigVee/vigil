package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProcessHistoryIdentityBoundsAndEvidence(t *testing.T) {
	var h processHistories
	start := time.UnixMilli(1000)
	p := Process{PID: 42, StartedMS: 1000, MetricsValid: true, CPU: 200, Memory: 2}
	for i := 0; i < 70; i++ {
		p.Memory = float32(i)
		h.record(Snapshot{Time: start.Add(time.Duration(i) * time.Second), Processes: []Process{p}})
	}
	d := investigation{Process: relatedProcess{PID: 42}, Started: &start}
	trend := h.forInvestigation(d)
	if trend == nil || len(trend.Samples) != 60 || trend.AverageCPU != 200 || trend.PeakCPU != 200 || trend.MemoryChange != 59 {
		t.Fatalf("unexpected trend: %+v", trend)
	}
	p.Memory = 100
	h.record(Snapshot{Time: start.Add(71 * time.Second), Processes: []Process{p}})
	if trend.Samples[59].Memory != 69 {
		t.Fatal("review evidence mutated")
	}
	p.StartedMS = 2000
	h.record(Snapshot{Time: start.Add(72 * time.Second), Processes: []Process{p}})
	if h.forInvestigation(d) != nil || len(h[42].Samples) != 0 {
		t.Fatal("PID reuse inherited history")
	}
	h.record(Snapshot{})
	if len(h) != 0 {
		t.Fatal("exited process retained")
	}
}
func TestProcessHistoryRejectsUnavailableAndDuplicateSamples(t *testing.T) {
	var h processHistories
	p := Process{PID: 1, StartedMS: 1, MetricsValid: true}
	now := time.Now()
	h.record(Snapshot{Time: now, Processes: []Process{p}})
	now = now.Add(time.Second)
	h.record(Snapshot{Time: now, Processes: []Process{p}})
	h.record(Snapshot{Time: now, Processes: []Process{p}})
	p.MetricsValid = false
	h.record(Snapshot{Time: now.Add(time.Second), Processes: []Process{p}})
	if len(h[1].Samples) != 1 {
		t.Fatal("invalid samples retained")
	}
}

func TestHistoryAppearsInEvidenceReview(t *testing.T) {
	d := investigation{History: &processTrend{Samples: []processSample{{Time: time.Unix(1, 0), CPU: 1, Memory: 2}, {Time: time.Unix(2, 0), CPU: 3, Memory: 4}}, AverageCPU: 2, PeakCPU: 3, MemoryChange: 2}}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	summary := evidenceSummary(string(raw))
	for _, want := range []string{"SESSION TRENDS", "2 observations", "CPU average 2.0%", "peak 3.0%", "+2.00 percentage points"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in review", want)
		}
	}
}

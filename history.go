package main

import "time"

const processHistoryLimit = 60

type processSample struct {
	Time   time.Time `json:"time"`
	CPU    float64   `json:"cpu_percent"`
	Memory float64   `json:"memory_percent"`
}
type processTrend struct {
	StartedMS    int64           `json:"process_started_at_unix_ms"`
	Samples      []processSample `json:"samples"`
	AverageCPU   float64         `json:"average_cpu_percent"`
	PeakCPU      float64         `json:"peak_cpu_percent"`
	MemoryChange float64         `json:"memory_change_percentage_points"`
}
type processHistories map[int32]processTrend

// Retain only currently visible process identities, with a fixed sample cap.
// The first observation establishes identity; its CPU delta is not yet reliable.
func (h *processHistories) record(s Snapshot) {
	if *h == nil {
		*h = make(processHistories)
	}
	seen := make(map[int32]bool, len(s.Processes))
	for _, p := range s.Processes {
		if p.StartedMS <= 0 {
			continue
		}
		seen[p.PID] = true
		t, exists := (*h)[p.PID]
		if !exists || t.StartedMS != p.StartedMS {
			(*h)[p.PID] = processTrend{StartedMS: p.StartedMS}
			continue
		}
		if !p.MetricsValid {
			continue
		}
		if n := len(t.Samples); n > 0 && !s.Time.After(t.Samples[n-1].Time) {
			continue
		}
		t.Samples = append(t.Samples, processSample{s.Time, p.CPU, float64(p.Memory)})
		if len(t.Samples) > processHistoryLimit {
			t.Samples = append([]processSample(nil), t.Samples[len(t.Samples)-processHistoryLimit:]...)
		}
		(*h)[p.PID] = t
	}
	for pid := range *h {
		if !seen[pid] {
			delete(*h, pid)
		}
	}
}
func (h processHistories) forInvestigation(d investigation) *processTrend {
	if d.Started == nil {
		return nil
	}
	t, ok := h[d.Process.PID]
	if !ok || t.StartedMS != d.Started.UnixMilli() || len(t.Samples) < 2 {
		return nil
	}
	// Copy the review payload so live collection cannot change confirmed evidence.
	t.Samples = append([]processSample(nil), t.Samples...)
	t.AverageCPU = 0
	t.PeakCPU = 0
	for _, s := range t.Samples {
		t.AverageCPU += s.CPU
		t.PeakCPU = max(t.PeakCPU, s.CPU)
	}
	t.AverageCPU /= float64(len(t.Samples))
	t.MemoryChange = t.Samples[len(t.Samples)-1].Memory - t.Samples[0].Memory
	return &t
}

// Scale each trend independently so small memory changes remain visible.
func trendSpark(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	low, high := values[0], values[0]
	for _, v := range values {
		low = min(low, v)
		high = max(high, v)
	}
	scaled := make([]float64, len(values))
	for i, v := range values {
		if high > low {
			scaled[i] = (v - low) * 100 / (high - low)
		}
	}
	return spark(scaled, processHistoryLimit)
}

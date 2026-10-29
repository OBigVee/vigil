package main

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type Process struct {
	PID       int32   `json:"pid"`
	ParentPID int32   `json:"parent_pid"`
	Name      string  `json:"name"`
	CPU       float64 `json:"cpu_percent"`
	Memory    float32 `json:"memory_percent"`
}
type Snapshot struct {
	Time      time.Time `json:"time"`
	OS        string    `json:"os"`
	CPU       float64   `json:"cpu_percent"`
	RAM       float64   `json:"memory_percent"`
	Used      uint64    `json:"used_memory_bytes"`
	Total     uint64    `json:"total_memory_bytes"`
	Processes []Process `json:"processes"`
	Warning   string    `json:"collection_warning,omitempty"`
}

// A collector owns previous samples. Only one collection runs at a time.
type collector struct{ previous map[int32]*process.Process }

func (c *collector) collect() Snapshot {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s := Snapshot{Time: time.Now(), OS: runtime.GOOS}
	var wg sync.WaitGroup
	var cpuErr, memErr, procErr error
	wg.Add(3)
	go func() {
		defer wg.Done()
		v, e := cpu.PercentWithContext(ctx, 0, false)
		cpuErr = e
		if len(v) > 0 {
			s.CPU = v[0]
		}
	}()
	go func() {
		defer wg.Done()
		v, e := mem.VirtualMemoryWithContext(ctx)
		memErr = e
		if e == nil {
			s.RAM = v.UsedPercent
			s.Used = v.Used
			s.Total = v.Total
		}
	}()
	go func() {
		defer wg.Done()
		ps, e := process.ProcessesWithContext(ctx)
		procErr = e
		if e != nil {
			return
		}
		next := make(map[int32]*process.Process, len(ps))
		// Limit concurrent OS queries even on systems with thousands of processes.
		jobs := make(chan *process.Process)
		results := make(chan Process, len(ps))
		var workers sync.WaitGroup
		for i := 0; i < 8; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for p := range jobs {
					name, e := p.NameWithContext(ctx)
					if e != nil {
						continue
					}
					cp, _ := p.PercentWithContext(ctx, 0)
					mp, _ := p.MemoryPercentWithContext(ctx)
					parent, _ := p.PpidWithContext(ctx)
					results <- Process{PID: p.Pid, ParentPID: parent, Name: name, CPU: cp, Memory: mp}
				}
			}()
		}
		for _, p := range ps {
			if old, ok := c.previous[p.Pid]; ok {
				p = old
			}
			next[p.Pid] = p
			jobs <- p
		}
		close(jobs)
		workers.Wait()
		close(results)
		for p := range results {
			s.Processes = append(s.Processes, p)
		}
		c.previous = next
	}()
	wg.Wait()
	for _, e := range []error{cpuErr, memErr, procErr} {
		if e != nil {
			s.Warning += "Some metrics unavailable: " + e.Error() + " "
		}
	}
	sortProcesses(s.Processes, "cpu")
	return s
}
func sortProcesses(ps []Process, by string) {
	sort.Slice(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		switch by {
		case "memory":
			if a.Memory != b.Memory {
				return a.Memory > b.Memory
			}
		case "name":
			if a.Name != b.Name {
				return a.Name < b.Name
			}
		default:
			if a.CPU != b.CPU {
				return a.CPU > b.CPU
			}
		}
		return a.PID < b.PID
	})
}
func bytesLabel(b uint64) string { return fmt.Sprintf("%.1f GiB", float64(b)/(1<<30)) }

package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type Process struct {
		PID int32
		Name string
		MemoryPercent float32
	}
	
type SystemState struct {
	CPUUsage float64
	UsedRAM uint64
	TotalRAM uint64
	RAMPercent float64
	Processes []Process
}


type model struct {
	state SystemState
}

// runs once when the app starts
func (m model) Init() tea.Cmd{
	return nil
}


func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd){
	switch msg := msg.(type){
		case tea.KeyMsg:
			switch msg.String(){
			case "q", "ctrl+c":
				return m, tea.Quit
			}
	}
	return m, nil
}

// view draws state to the terminal
func (m model) View() string {
	s := "==== Vigil: Terminal UI ====\n\n"
	s += fmt.Sprintf("CPU Usage: %.2f%%\n", m.state.CPUUsage)
	s += fmt.Sprintf("RAM Usage: %v MB / %v MB (%.2f%%)\n\n", m.state.UsedRAM, m.state.TotalRAM, m.state.RAMPercent)

	s += "--- Top 10 processes by Memory ---\n"
	for _, p := range m.state.Processes {
		s += fmt.Sprintf("PID: %-8d | Mem: %5.2f%% | Name: %s\n", p.PID, p.MemoryPercent, p.Name)
	}

	s += "\n[Press 'q' or 'ctrl+c' to quit]\n"
	return s
}




func main(){
	fmt.Println("==== Vigil: initiated ===\n")

	// Fetch CPU Usage
	// pass 1-sec interval to cal the % used during that time
	// "false" means we want total CPU usage, not per-core usage

	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil{
		log.Fatalf("Error fetching CPU: %v", err)
	}
	fmt.Printf("CPU Usage: %.2f%%\n", cpuPercent[0])
	
	// Fetch Memory (RAM) Usage
	vMem, err := mem.VirtualMemory()
	if err != nil {
		log.Fatalf("Error fetching Memory: %v", err)
	}
	// convert bytes to megabytes for readability
	totalRAM := vMem.Total / 1024 / 1024
	usedRAM := vMem.Used / 1024/ 1024
	fmt.Printf("RAM Usage: %v MB / %v MB (%.2f%%)\n\n", usedRAM, totalRAM, vMem.UsedPercent)

	// Fetch Top 10 Processes by Memory
	fmt.Println("--- Top 10 processes by Memory ---")

	processes, err := process.Processes()
	if err != nil { 
		log.Fatalf("Error fetching processes: %v", err)
	}

	// ephemeral storage struct to hold data
	type ProcInfo struct {
		PID int32
		Name string
		MemoryPercent float32
	}

	var procList []ProcInfo

	for _, p := range processes{
		
		name, _ := p.Name()
		memPct, _ := p.MemoryPercent()

		procList = append(procList, ProcInfo{
			PID: p.Pid,
			Name: name,
			MemoryPercent: memPct,
		})
	}
	
	// sort list by memory percent in desc order
	sort.Slice(procList, func (i, j int) bool {
		return procList[i].MemoryPercent > procList[j].MemoryPercent
		})
		
	// print top 10 (or few , if < 10 are running)
	
	for i := 0; i < 200 && i < len(procList); i++ {
		p := procList[i]
		fmt.Printf("PID: %-8d | Mem: %5.2f%% | Name: %s\n", p.PID, p.MemoryPercent, p.Name)
	}
	
}

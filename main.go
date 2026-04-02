package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

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
	
	for i := 0; i < 10 && i < len(procList); i++ {
		p := procList[i]
		fmt.Printf("PID: %-8d | Mem: %5.2f%% | Name: %s\n", p.PID, p.MemoryPercent, p.Name)
	}
	
}

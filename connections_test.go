package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func TestFamilyNetworkKeepsRelativeAttribution(t *testing.T) {
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p, e := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if e != nil {
		t.Fatal(e)
	}
	identity := processIdentity(ctx, p)
	d := investigation{Process: relatedProcess{PID: -1}, Children: []relatedProcess{identity}}
	collectFamilyNetwork(ctx, &d)
	if len(d.FamilyNetwork) != 1 {
		t.Fatalf("relative missing: %v", d.Notes)
	}
	entry := d.FamilyNetwork[0]
	if entry.Process.PID != p.Pid || entry.Relationship != "child" || len(d.Connections) != 0 {
		t.Fatal("relative activity attributed to selected process")
	}
	found := false
	port := uint32(listener.Addr().(*net.TCPAddr).Port)
	for _, c := range entry.Connections {
		if c.Local.Port == port {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture listener not collected")
	}
	raw, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	summary := evidenceSummary(string(raw))
	if !strings.Contains(summary, "RELATIVES' NETWORK ACTIVITY") || !strings.Contains(summary, "not proof of communication") {
		t.Fatal("review lost attribution")
	}
}

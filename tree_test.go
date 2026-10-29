package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	gnet "github.com/shirou/gopsutil/v4/net"
)

func TestLiveProcessTree(t *testing.T) {
	m := model{sort: "cpu", state: Snapshot{Processes: []Process{{PID: 1, Name: "init"}, {PID: 2, ParentPID: 1, Name: "desktop"}, {PID: 3, ParentPID: 2, Name: "selected", CPU: 90}, {PID: 4, ParentPID: 3, Name: "child"}, {PID: 5, ParentPID: 2, Name: "sibling"}}}}
	lines := ansi.Strip(strings.Join(m.treeLines(), "\n"))
	for _, want := range []string{"init [1]", "desktop [2]", "● selected [3]", "└─↓ child [4]", "SAME PARENT", "sibling [5]", "Press i"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Index(lines, "init [1]") > strings.Index(lines, "desktop [2]") {
		t.Fatal("ancestors must render root first")
	}
}
func TestEvidenceTreePinsInvestigatedProcess(t *testing.T) {
	m := model{localEvidence: true, pending: "review", state: Snapshot{Processes: []Process{{PID: 99, Name: "other"}}}, evidence: &investigation{Captured: time.Now(), Process: relatedProcess{PID: 3, Name: "agent"}, Connections: []socketEvidence{{Protocol: "tcp", Remote: gnet.Addr{IP: "127.0.0.1", Port: 80}, LocalPeer: &relatedProcess{PID: 8, Name: "server"}}, {Protocol: "tcp", Remote: gnet.Addr{IP: "198.51.100.1", Port: 443}}}}}
	lines := ansi.Strip(strings.Join(m.treeLines(), "\n"))
	if !strings.Contains(lines, "agent [3]") || !strings.Contains(lines, "↔ server [8]") || !strings.Contains(lines, "1 other sockets") {
		t.Fatal("missing evidence relationships")
	}
	if strings.Contains(lines, "other [99]") {
		t.Fatal("tree must stay pinned to investigated process during review")
	}
}
func TestTreeCycleAndMissingParent(t *testing.T) {
	m := model{state: Snapshot{Processes: []Process{{PID: 1, ParentPID: 2, Name: "first"}, {PID: 2, ParentPID: 1, Name: "second"}}}}
	if len(m.treeLines()) > 30 {
		t.Fatal("cyclic ancestry did not terminate")
	}
	m.state.Processes = []Process{{PID: 1, ParentPID: 999, Name: "first"}}
	if !strings.Contains(strings.Join(m.treeLines(), "\n"), "parent unavailable [999]") {
		t.Fatal("missing parent should be explicit")
	}
}

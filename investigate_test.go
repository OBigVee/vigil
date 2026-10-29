package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

func TestInvestigateCurrentProcess(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "vigil-evidence-")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stages := []int{}
	d, err := investigate(ctx, int32(os.Getpid()), func(stage int) { stages = append(stages, stage) })
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != len(collectionStages) {
		t.Fatalf("missing collection progress events: %v", stages)
	}
	for i, stage := range stages {
		if i != stage {
			t.Fatal("collection stages must follow actual work order")
		}
	}
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	if d.Process.PID != int32(os.Getpid()) || d.Process.Name == "" || d.Started == nil {
		t.Fatal("missing process identity")
	}
	if d.Process.Executable == "" {
		t.Fatal("own executable path must be available")
	}
	if !sameFilePath(d.Process.Executable, exe) {
		t.Fatal("wrong executable path")
	}
	if hasCollectionNote(d.Notes, "working_directory:") {
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			t.Fatal("working directory unexpectedly unavailable")
		}
		t.Log("working directory reported unavailable on this platform")
	} else if !sameFilePath(d.WorkingDirectory, cwd) {
		t.Fatal("wrong or silently missing working directory")
	}

	found := false
	for _, path := range d.OpenFiles {
		if sameFilePath(path, f.Name()) {
			found = true
		}
	}
	if !found {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && hasCollectionNote(d.Notes, "open_files:") {
			t.Log("open files reported unavailable on this platform")
		} else {
			t.Fatal("open fixture file not observed or reported unavailable")
		}
	}
	port := uint32(listener.Addr().(*net.TCPAddr).Port)
	found = false
	for _, socket := range d.Connections {
		if socket.Protocol == "tcp" && socket.Local.Port == port && socket.State == "LISTEN" {
			found = true
		}
	}
	if !found {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && hasCollectionNote(d.Notes, "connections:") {
			t.Log("socket ownership reported unavailable on this platform")
		} else {
			t.Fatal("fixture listener not observed or reported unavailable")
		}
	}
	if len(d.Ancestors) == 0 {
		t.Fatal("expected launcher ancestry")
	}
	if d.SampleSeconds <= 0 || d.ResidentBytes == 0 {
		t.Fatal("missing resource evidence")
	}
}
func TestInvestigationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := investigate(ctx, int32(os.Getpid()))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
func TestUniqueEvidenceLimits(t *testing.T) {
	notes := []string{}
	got := uniqueLimit([]string{"/b", "/a", "/b", "/c"}, 2, "files", &notes)
	if strings.Join(got, ",") != "/a,/b" || len(notes) != 1 {
		t.Fatal("deduplication/limit failed")
	}
}
func TestReverseSocketPeer(t *testing.T) {
	a := gnet.ConnectionStat{Pid: 11, Family: 2, Type: 1, Laddr: gnet.Addr{IP: "127.0.0.1", Port: 1234}, Raddr: gnet.Addr{IP: "127.0.0.1", Port: 5678}}
	listener := gnet.ConnectionStat{Pid: 99, Family: 2, Type: 1, Laddr: a.Raddr}
	peer := gnet.ConnectionStat{Pid: 22, Family: 2, Type: 1, Laddr: a.Raddr, Raddr: a.Laddr}
	if reversePeer(a, []gnet.ConnectionStat{listener}) != 0 {
		t.Fatal("listener is not evidence of a connected peer")
	}
	if reversePeer(a, []gnet.ConnectionStat{listener, peer}) != 22 {
		t.Fatal("missing reverse connection peer")
	}
	peer.Type = 2
	if reversePeer(a, []gnet.ConnectionStat{peer}) != 0 {
		t.Fatal("UDP must not match TCP")
	}
}

func TestLimitedRead(t *testing.T) {
	path := t.TempDir() + "/fixture"
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLimited(path, 4); err == nil {
		t.Fatal("oversized read should be reported, not silently truncated")
	}
	if b, err := readLimited(path, 5); err != nil || string(b) != "12345" {
		t.Fatal("exact-limit read failed")
	}
}

func sameFilePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if path, err := filepath.EvalSymlinks(a); err == nil {
		a = path
	}
	if path, err := filepath.EvalSymlinks(b); err == nil {
		b = path
	}
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
func hasCollectionNote(notes []string, prefix string) bool {
	for _, note := range notes {
		if strings.HasPrefix(note, prefix) {
			return true
		}
	}
	return false
}

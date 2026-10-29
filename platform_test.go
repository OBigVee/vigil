package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

func TestPlatformCPUCapability(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	values, err := cpu.PercentWithContext(ctx, 150*time.Millisecond, false)
	knownMissing := runtime.GOOS == "darwin" && !platformTestCGO
	if knownMissing {
		if err == nil {
			t.Fatal("macOS no-CGO capability changed: update the recorded release limitation")
		}
		sample := (&collector{}).collect()
		if sample.Warning == "" {
			t.Fatal("unsupported aggregate CPU must be reported, not silently shown as zero")
		}
		t.Log("Known v0.1.0 macOS no-CGO gap: aggregate CPU unavailable; native CGO lane must collect it")
		return
	}
	if err != nil || len(values) != 1 {
		t.Fatalf("native aggregate CPU collection failed: %v", err)
	}
	if math.IsNaN(values[0]) || math.IsInf(values[0], 0) || values[0] < 0 || values[0] > 100 {
		t.Fatal("invalid aggregate CPU percentage")
	}
}
func TestPlatformCollectorSeesOwnProcess(t *testing.T) {
	sample := (&collector{}).collect()
	if sample.Total == 0 || sample.Used > sample.Total {
		t.Fatal("system memory is missing or invalid")
	}
	for _, p := range sample.Processes {
		if p.PID == int32(os.Getpid()) {
			if p.Name == "" {
				t.Fatal("own process has no name")
			}
			return
		}
	}
	t.Fatal("collector did not enumerate its own process")
}

// A Go helper avoids shell-specific commands and never needs elevated privileges.
func TestPlatformChildFixture(t *testing.T) {
	if os.Getenv("VIGIL_PLATFORM_FIXTURE") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "ready")
	io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}
func TestPlatformChildRelationship(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPlatformChildFixture$")
	child.Env = append(os.Environ(), "VIGIL_PLATFORM_FIXTURE=1")
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); child.Wait() }()
	ready := make(chan bool, 1)
	go func() { scanner := bufio.NewScanner(output); ready <- scanner.Scan() && scanner.Text() == "ready" }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("fixture did not start")
		}
	case <-ctx.Done():
		t.Fatal("fixture startup timed out")
	}
	data, err := investigate(ctx, int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range data.Children {
		if p.PID == int32(child.Process.Pid) {
			if p.ParentPID != int32(os.Getpid()) {
				t.Fatal("incorrect parent relationship")
			}
			return
		}
	}
	t.Fatal("investigation did not identify the controlled child process")
}

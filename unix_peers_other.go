//go:build !linux

package main

import (
	"context"
	"github.com/shirou/gopsutil/v4/process"
)

func collectUnixPeers(_ context.Context, d *investigation, _ []*process.Process) {
	d.Notes = append(d.Notes, "unix_socket_peers: Linux-only collection; unavailable on this platform")
}

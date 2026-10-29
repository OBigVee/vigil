package main

import (
	"context"
	"encoding/binary"
	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/unix"
	"os"
	"testing"
	"time"
)

func TestUnixDiagnosticParser(t *testing.T) {
	data := make([]byte, 24)
	data[0] = 1
	binary.NativeEndian.PutUint32(data[4:], 123)
	binary.NativeEndian.PutUint16(data[16:], 8)
	binary.NativeEndian.PutUint16(data[18:], 2)
	binary.NativeEndian.PutUint32(data[20:], 456)
	d, e := parseUnixDiagnostic(data)
	if e != nil || d.inode != 123 || d.peer != 456 {
		t.Fatalf("bad diagnostic %+v %v", d, e)
	}
	for _, bad := range [][]byte{data[:10], data[:23], append(data, 1)} {
		if _, e := parseUnixDiagnostic(bad); e == nil {
			t.Fatal("malformed diagnostic accepted")
		}
	}
}
func TestUnixConnectedPeerOwnership(t *testing.T) {
	fds, e := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer unix.Close(fds[0])
	defer unix.Close(fds[1])
	var left, right unix.Stat_t
	if e = unix.Fstat(fds[0], &left); e != nil {
		t.Fatal(e)
	}
	if e = unix.Fstat(fds[1], &right); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	diagnostics, e := unixDiagnostics(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if diagnostics[uint32(left.Ino)].peer != uint32(right.Ino) || diagnostics[uint32(right.Ino)].peer != uint32(left.Ino) {
		t.Fatal("kernel peer mismatch")
	}
	p, e := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if e != nil {
		t.Fatal(e)
	}
	d := investigation{Process: relatedProcess{PID: p.Pid}}
	collectUnixPeers(ctx, &d, []*process.Process{p})
	found := false
	for _, peer := range d.UnixPeers {
		if peer.LocalInode == uint32(left.Ino) && peer.PeerInode == uint32(right.Ino) {
			for _, owner := range peer.Owners {
				if owner.PID == p.Pid {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("fixture owner missing; notes %v", d.Notes)
	}
}

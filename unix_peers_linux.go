package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/unix"
)

type unixDiagnostic struct {
	inode, peer uint32
	path        string
}

// Linux SOCK_DIAG reports the actual connected peer inode, unlike /proc/net/unix.
func unixDiagnostics(ctx context.Context) (map[uint32]unixDiagnostic, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	if err = unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	if err = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Usec: 100000}); err != nil {
		return nil, err
	}
	req := make([]byte, 40)
	order := binary.NativeEndian
	order.PutUint32(req, 40)
	order.PutUint16(req[4:], unix.SOCK_DIAG_BY_FAMILY)
	order.PutUint16(req[6:], unix.NLM_F_REQUEST|unix.NLM_F_DUMP)
	order.PutUint32(req[8:], 1)
	req[16] = unix.AF_UNIX
	order.PutUint32(req[20:], 0xffffffff)
	order.PutUint32(req[28:], 5)
	order.PutUint32(req[32:], 0xffffffff)
	order.PutUint32(req[36:], 0xffffffff)
	if err = unix.Sendto(fd, req, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	result := map[uint32]unixDiagnostic{}
	buffer := make([]byte, 65536)
	for total := 0; total < 8<<20; {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		n, _, flags, from, e := unix.Recvmsg(fd, buffer, nil, 0)
		if errors.Is(e, unix.EAGAIN) || errors.Is(e, unix.EINTR) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if flags&unix.MSG_TRUNC != 0 {
			return nil, errors.New("socket diagnostics truncated")
		}
		sender, ok := from.(*unix.SockaddrNetlink)
		if !ok || sender.Pid != 0 {
			return nil, errors.New("unexpected diagnostic sender")
		}
		total += n
		messages, e := syscall.ParseNetlinkMessage(buffer[:n])
		if e != nil {
			return nil, e
		}
		for _, m := range messages {
			if m.Header.Seq != 1 {
				continue
			}
			if m.Header.Flags&unix.NLM_F_DUMP_INTR != 0 {
				return nil, errors.New("socket diagnostic dump interrupted")
			}
			if m.Header.Type == unix.NLMSG_DONE {
				return result, nil
			}
			if m.Header.Type == unix.NLMSG_ERROR {
				return nil, errors.New("socket diagnostic request rejected")
			}
			if m.Header.Type != unix.SOCK_DIAG_BY_FAMILY {
				continue
			}
			d, e := parseUnixDiagnostic(m.Data)
			if e != nil {
				return nil, e
			}
			result[d.inode] = d
		}
	}
	return nil, errors.New("socket diagnostic size limit reached")
}
func parseUnixDiagnostic(data []byte) (unixDiagnostic, error) {
	var d unixDiagnostic
	if len(data) < 16 || data[0] != unix.AF_UNIX {
		return d, errors.New("invalid Unix diagnostic")
	}
	d.inode = binary.NativeEndian.Uint32(data[4:])
	for attrs := data[16:]; len(attrs) > 0; {
		if len(attrs) < 4 {
			return d, errors.New("short Unix attribute")
		}
		n := int(binary.NativeEndian.Uint16(attrs))
		kind := binary.NativeEndian.Uint16(attrs[2:]) & 0x3fff
		if n < 4 || n > len(attrs) {
			return d, errors.New("invalid Unix attribute size")
		}
		switch kind {
		case 0:
			d.path = strings.TrimRight(string(attrs[4:n]), "\x00")
			if strings.HasPrefix(d.path, "\x00") {
				d.path = "@" + d.path[1:]
			}
		case 2:
			if n != 8 {
				return d, errors.New("invalid peer inode")
			}
			d.peer = binary.NativeEndian.Uint32(attrs[4:n])
		}
		step := (n + 3) &^ 3
		if step > len(attrs) {
			return d, errors.New("short Unix attribute padding")
		}
		attrs = attrs[step:]
	}
	return d, nil
}
func socketInodes(ctx context.Context, pid int32) (map[uint32]bool, bool, error) {
	dir, err := os.Open(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return nil, false, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(2049)
	if err != nil && len(names) == 0 {
		return nil, false, err
	}
	capped := len(names) > 2048
	if capped {
		names = names[:2048]
	}
	result := map[uint32]bool{}
	for _, name := range names {
		if ctx.Err() != nil {
			return nil, capped, ctx.Err()
		}
		link, e := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, name))
		if e != nil {
			continue
		}
		if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
			n, e := strconv.ParseUint(link[8:len(link)-1], 10, 32)
			if e == nil {
				result[uint32(n)] = true
			}
		}
	}
	return result, capped, nil
}
func collectUnixPeers(ctx context.Context, d *investigation, ps []*process.Process) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	diagnostics, err := unixDiagnostics(ctx)
	if err != nil {
		d.Notes = append(d.Notes, "unix_socket_peers: unavailable (socket diagnostics, permissions, timeout, or platform restrictions)")
		return
	}
	owned, capped, err := socketInodes(ctx, d.Process.PID)
	if err != nil {
		d.Notes = append(d.Notes, "unix_socket_peers: selected process descriptors unavailable")
		return
	}
	if capped {
		d.Notes = append(d.Notes, "unix_socket_peers: selected descriptors truncated at 2048")
	}
	wanted := map[uint32]bool{}
	for inode := range owned {
		if peer := diagnostics[inode].peer; peer > 0 {
			wanted[peer] = true
		}
	}
	owners := map[uint32][]relatedProcess{}
	if len(wanted) > 0 {
		for i, p := range ps {
			if i >= 4096 || ctx.Err() != nil {
				d.Notes = appendOnce(d.Notes, "unix_socket_peers: owner scan incomplete (4096 processes or time limit)")
				break
			}
			birth, e := p.CreateTimeWithContext(ctx)
			if e != nil {
				continue
			}
			inodes, truncated, e := socketInodes(ctx, p.Pid)
			if e != nil {
				d.Notes = appendOnce(d.Notes, "unix_socket_peers: some process descriptors inaccessible")
				continue
			}
			if truncated {
				d.Notes = appendOnce(d.Notes, "unix_socket_peers: some descriptors truncated at 2048 per process")
			}
			matched := false
			for inode := range inodes {
				if wanted[inode] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			identity := processIdentity(ctx, p)
			fresh, e := process.NewProcessWithContext(ctx, p.Pid)
			if e != nil {
				continue
			}
			after, e := fresh.CreateTimeWithContext(ctx)
			if e != nil || after != birth {
				continue
			}
			for inode := range inodes {
				if wanted[inode] && len(owners[inode]) < 16 {
					owners[inode] = append(owners[inode], identity)
				}
			}
		}
	}
	inodes := make([]uint32, 0, len(owned))
	for inode := range owned {
		inodes = append(inodes, inode)
	}
	sort.Slice(inodes, func(i, j int) bool { return inodes[i] < inodes[j] })
	for _, inode := range inodes {
		record, ok := diagnostics[inode]
		if !ok || record.peer == 0 {
			continue
		}
		if len(d.UnixPeers) >= 64 {
			d.Notes = appendOnce(d.Notes, "unix_socket_peers: truncated at 64 sockets")
			break
		}
		d.UnixPeers = append(d.UnixPeers, unixPeerEvidence{LocalInode: inode, PeerInode: record.peer, LocalPath: record.path, PeerPath: diagnostics[record.peer].path, Owners: owners[record.peer]})
	}
	d.Notes = append(d.Notes, "Unix peer inodes come from Linux SOCK_DIAG; owners hold the peer descriptor, which may be shared or inherited. Owners are capped at 16 per socket. Missing owners do not imply no peer. Observations are not atomic.")
}

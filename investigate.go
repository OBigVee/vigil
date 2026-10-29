package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

const explanationTask = `Explain this particular process instance on this machine using the collected evidence, not a generic process-name description. Start with a concrete summary of what this instance is doing. Then explain its observed software identity, launcher/ancestors, children and siblings, working directory, open files, mapped files, service cgroup, and sockets where evidence exists. Cite JSON field names and actual paths/PIDs/endpoints for important claims. Explain how these observations connect; distinguish observed relationships from hypotheses. An ancestor is a launcher, a sibling only shares a parent, a mapped file may be code or data, and only an observed_local_peer identifies a local peer process; other remote socket addresses do not identify software. Do not invent file contents, paired devices, network peers, or causality. Empty, unavailable, and truncated evidence are different. Explain relevant gaps briefly. Avoid generic verification checklists when the data already answers the question. End with at most two specific read-only next steps if needed.`

type relatedProcess struct {
	PID        int32  `json:"pid"`
	ParentPID  int32  `json:"parent_pid"`
	Name       string `json:"name"`
	Executable string `json:"executable,omitempty"`
}
type socketEvidence struct {
	Protocol  string          `json:"protocol"`
	Local     gnet.Addr       `json:"local"`
	Remote    gnet.Addr       `json:"remote"`
	State     string          `json:"state"`
	LocalPeer *relatedProcess `json:"observed_local_peer,omitempty"`
}
type machineEvidence struct {
	OS          string `json:"os"`
	Arch        string `json:"architecture"`
	Platform    string `json:"platform"`
	Version     string `json:"platform_version"`
	LogicalCPUs int    `json:"logical_cpus"`
}
type investigation struct {
	Captured         time.Time        `json:"captured_at"`
	Machine          machineEvidence  `json:"machine"`
	Process          relatedProcess   `json:"process"`
	Owner            string           `json:"owner,omitempty"`
	Started          *time.Time       `json:"started_at,omitempty"`
	WorkingDirectory string           `json:"working_directory,omitempty"`
	Package          string           `json:"installed_package,omitempty"`
	CPU              float64          `json:"cpu_percent_over_sample"`
	SampleSeconds    float64          `json:"sample_seconds"`
	ResidentBytes    uint64           `json:"resident_memory_bytes"`
	Ancestors        []relatedProcess `json:"ancestors"`
	Children         []relatedProcess `json:"children"`
	Siblings         []relatedProcess `json:"siblings"`
	OpenFiles        []string         `json:"open_files"`
	Libraries        []string         `json:"mapped_files"`
	Cgroups          []string         `json:"linux_cgroups"`
	Connections      []socketEvidence `json:"connections"`
	Notes            []string         `json:"collection_notes"`
}

func processIdentity(ctx context.Context, p *process.Process) relatedProcess {
	n, _ := p.NameWithContext(ctx)
	exe, _ := p.ExeWithContext(ctx)
	ppid, _ := p.PpidWithContext(ctx)
	return relatedProcess{p.Pid, ppid, n, exe}
}
func investigate(ctx context.Context, pid int32, progress ...func(int)) (investigation, error) {
	report := func(stage int) {
		for _, fn := range progress {
			fn(stage)
		}
	}
	report(0)
	if err := ctx.Err(); err != nil {
		return investigation{}, err
	}
	d := investigation{Captured: time.Now(), Machine: machineEvidence{OS: runtime.GOOS, Arch: runtime.GOARCH, LogicalCPUs: runtime.NumCPU()}, Ancestors: []relatedProcess{}, Children: []relatedProcess{}, Siblings: []relatedProcess{}, OpenFiles: []string{}, Libraries: []string{}, Cgroups: []string{}, Connections: []socketEvidence{}, Notes: []string{}}
	p, e := process.NewProcessWithContext(ctx, pid)
	if e != nil {
		return d, fmt.Errorf("process %d is no longer accessible; select it again", pid)
	}
	birth, e := p.CreateTimeWithContext(ctx)
	if e != nil {
		return d, fmt.Errorf("could not establish identity for PID %d; it may have exited or access was denied", pid)
	}
	started := time.UnixMilli(birth)
	d.Started = &started
	d.Process = processIdentity(ctx, p)
	if d.Process.Name == "" {
		return d, fmt.Errorf("process %d exited during investigation", pid)
	}
	note := func(field string, e error) {
		if e != nil {
			d.Notes = append(d.Notes, field+": unavailable (permissions, platform support, or process lifetime)")
		}
	}
	platform, _, version, err := host.PlatformInformationWithContext(ctx)
	note("platform", err)
	d.Machine.Platform = platform
	d.Machine.Version = version
	d.Owner, err = p.UsernameWithContext(ctx)
	note("owner", err)
	d.WorkingDirectory, err = p.CwdWithContext(ctx)
	note("working_directory", err)
	if d.Process.Executable == "" {
		d.Notes = append(d.Notes, "executable: unavailable; kernel threads may not have an executable")
	}
	if mi, e := p.MemoryInfoWithContext(ctx); e == nil {
		d.ResidentBytes = mi.RSS
	} else {
		note("resident_memory", e)
	}
	report(1)
	// Independent process object: sampling must not disturb the live collector's baseline.
	_, err = p.PercentWithContext(ctx, 0)
	note("cpu_baseline", err)
	sampleStart := time.Now()
	timer := time.NewTimer(400 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return d, ctx.Err()
	case <-timer.C:
	}
	d.SampleSeconds = time.Since(sampleStart).Seconds()
	d.CPU, err = p.PercentWithContext(ctx, 0)
	note("cpu_sample", err)
	report(2)
	seen := map[int32]bool{pid: true}
	parent := d.Process.ParentPID
	for len(d.Ancestors) < 8 && parent > 0 && !seen[parent] {
		seen[parent] = true
		ancestor, e := process.NewProcessWithContext(ctx, parent)
		if e != nil {
			note("ancestor", e)
			break
		}
		r := processIdentity(ctx, ancestor)
		d.Ancestors = append(d.Ancestors, r)
		parent = r.ParentPID
	}
	if parent > 0 && !seen[parent] {
		d.Notes = append(d.Notes, "ancestors: truncated at 8 entries")
	}
	ps, err := process.ProcessesWithContext(ctx)
	note("process_relationships", err)
	for _, other := range ps {
		if ctx.Err() != nil {
			return d, ctx.Err()
		}
		if other.Pid == pid {
			continue
		}
		pp, e := other.PpidWithContext(ctx)
		if e != nil {
			continue
		}
		if pp == pid {
			if len(d.Children) < 32 {
				d.Children = append(d.Children, processIdentity(ctx, other))
			} else {
				d.Notes = appendOnce(d.Notes, "children: truncated at 32 entries")
			}
		}
		if pp == d.Process.ParentPID && pp > 0 {
			if len(d.Siblings) < 16 {
				d.Siblings = append(d.Siblings, processIdentity(ctx, other))
			} else {
				d.Notes = appendOnce(d.Notes, "siblings: truncated at 16 entries")
			}
		}
	}
	report(3)
	files, err := p.OpenFilesWithContext(ctx)
	note("open_files", err)
	for _, f := range files {
		d.OpenFiles = append(d.OpenFiles, f.Path)
	}
	d.OpenFiles = uniqueLimit(d.OpenFiles, 64, "open_files", &d.Notes)
	report(4)
	sockets, err := p.ConnectionsWithContext(ctx)
	note("connections", err)
	sort.Slice(sockets, func(i, j int) bool {
		a, b := sockets[i], sockets[j]
		return fmt.Sprint(a.Family, a.Type, a.Laddr, a.Raddr, a.Status) < fmt.Sprint(b.Family, b.Type, b.Laddr, b.Raddr, b.Status)
	})
	if len(sockets) > 64 {
		d.Notes = append(d.Notes, "connections: truncated at 64 entries")
		sockets = sockets[:64]
	}
	for _, s := range sockets {
		protocol := "unknown"
		switch s.Family {
		case 1:
			protocol = "unix"
		case 2, 10:
			switch s.Type {
			case 1:
				protocol = "tcp"
			case 2:
				protocol = "udp"
			}
		}
		d.Connections = append(d.Connections, socketEvidence{Protocol: protocol, Local: s.Laddr, Remote: s.Raddr, State: s.Status})
	}
	if needsLocalPeers(sockets) {
		all, peerErr := gnet.ConnectionsWithContext(ctx, "inet")
		note("local_socket_peers", peerErr)
		if peerErr == nil {
			attachLocalPeers(ctx, d.Connections, sockets, all)
		}
		d.Notes = append(d.Notes, "Local TCP peers require a reverse socket tuple and visible process ownership; unmatched endpoints do not prove no peer exists. Connections can change during collection.")
	}
	report(5)
	d.Package, err = packageOwner(ctx, d.Process.Executable)
	note("installed_package", err)
	report(6)
	if runtime.GOOS == "linux" {
		root := filepath.Join("/proc", strconv.Itoa(int(pid)))
		data, e := readLimited(filepath.Join(root, "cgroup"), 64<<10)
		note("linux_cgroups", e)
		if e == nil {
			d.Cgroups = strings.Fields(string(data))
		}
		data, e = readLimited(filepath.Join(root, "maps"), 2<<20)
		note("mapped_files", e)
		if e == nil {
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 6 && strings.HasPrefix(fields[5], "/") {
					d.Libraries = append(d.Libraries, strings.Join(fields[5:], " "))
				}
			}
			d.Libraries = uniqueLimit(d.Libraries, 32, "mapped_files", &d.Notes)
		}
	} else {
		d.Notes = append(d.Notes, "linux_cgroups and mapped_files: Linux-only collection")
	}
	report(7)
	// Re-open instead of using gopsutil's cached creation time to detect exit/PID reuse.
	current, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return d, fmt.Errorf("process %d exited during investigation; discard this sample", pid)
	}
	currentBirth, err := current.CreateTimeWithContext(ctx)
	if err != nil || currentBirth != birth {
		return d, fmt.Errorf("process %d changed identity during investigation; select it again", pid)
	}
	if ctx.Err() != nil {
		return d, ctx.Err()
	}
	d.Notes = append(d.Notes, "Live observations span the collection interval; files and sockets may change. Socket endpoints do not identify remote applications. File paths do not reveal file contents. Command arguments, environment, and file contents were not collected.")
	return d, nil
}
func appendOnce(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}
func uniqueLimit(xs []string, limit int, label string, notes *[]string) []string {
	sort.Strings(xs)
	result := []string{}
	for _, x := range xs {
		if x != "" && (len(result) == 0 || result[len(result)-1] != x) {
			result = append(result, x)
		}
	}
	if len(result) > limit {
		*notes = append(*notes, fmt.Sprintf("%s: truncated at %d entries", label, limit))
		result = result[:limit]
	}
	return result
}
func readLimited(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("collection size limit exceeded")
	}
	return b, nil
}

// Query package metadata only. Never execute the selected process or invoke a shell.
func packageOwner(ctx context.Context, exe string) (string, error) {
	if exe == "" {
		return "", fmt.Errorf("no executable path")
	}
	for _, tool := range []struct {
		name string
		args []string
	}{{"dpkg-query", []string{"-S", exe}}, {"rpm", []string{"-qf", "--", exe}}, {"pacman", []string{"-Qo", "--", exe}}} {
		path, e := exec.LookPath(tool.name)
		if e != nil {
			continue
		}
		queryCtx, cancel := context.WithTimeout(ctx, time.Second)
		cmd := exec.CommandContext(queryCtx, path, tool.args...)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		output := &limitedOutput{remaining: 8192}
		cmd.Stdout = output
		cmd.Stderr = io.Discard
		err := cmd.Run()
		cancel()
		if err == nil {
			return strings.TrimSpace(output.String()), nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", fmt.Errorf("no package ownership found or package manager unavailable")
}

type limitedOutput struct {
	strings.Builder
	remaining int
}

func (w *limitedOutput) Write(b []byte) (int, error) {
	n := len(b)
	if len(b) > w.remaining {
		b = b[:w.remaining]
	}
	w.Builder.Write(b)
	w.remaining -= len(b)
	return n, nil
}

func needsLocalPeers(sockets []gnet.ConnectionStat) bool {
	for _, s := range sockets {
		if s.Type == 1 && s.Raddr.Port != 0 && (net.ParseIP(s.Raddr.IP).IsLoopback() || s.Raddr.IP == s.Laddr.IP) {
			return true
		}
	}
	return false
}
func reversePeer(selected gnet.ConnectionStat, all []gnet.ConnectionStat) int32 {
	if selected.Type != 1 || selected.Raddr.Port == 0 {
		return 0
	}
	for _, candidate := range all {
		if candidate.Pid > 0 && candidate.Type == selected.Type && candidate.Family == selected.Family && candidate.Laddr == selected.Raddr && candidate.Raddr == selected.Laddr {
			return candidate.Pid
		}
	}
	return 0
}
func attachLocalPeers(ctx context.Context, evidence []socketEvidence, sockets, all []gnet.ConnectionStat) {
	for i, s := range sockets {
		if ctx.Err() != nil {
			return
		}
		pid := reversePeer(s, all)
		if pid == 0 {
			continue
		}
		if p, err := process.NewProcessWithContext(ctx, pid); err == nil {
			r := processIdentity(ctx, p)
			evidence[i].LocalPeer = &r
		}
	}
}

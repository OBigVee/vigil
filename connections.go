package main

import (
	"context"
	"sort"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type unixPeerEvidence struct {
	LocalInode uint32           `json:"local_socket_inode"`
	PeerInode  uint32           `json:"peer_socket_inode"`
	LocalPath  string           `json:"local_socket_path,omitempty"`
	PeerPath   string           `json:"peer_socket_path,omitempty"`
	Owners     []relatedProcess `json:"observed_peer_descriptor_owners"`
}
type familyNetworkEvidence struct {
	Process      relatedProcess   `json:"process"`
	Relationship string           `json:"relationship_to_selected"`
	StartedMS    int64            `json:"started_at_unix_ms"`
	Connections  []socketEvidence `json:"connections"`
}

func socketRecord(s gnet.ConnectionStat) socketEvidence {
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
	return socketEvidence{Protocol: protocol, Local: s.Laddr, Remote: s.Raddr, State: s.Status}
}
func collectFamilyNetwork(ctx context.Context, d *investigation) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	type candidate struct {
		p        relatedProcess
		relation string
	}
	candidates := []candidate{}
	if len(d.Ancestors) > 0 && d.Ancestors[0].PID > 1 {
		candidates = append(candidates, candidate{d.Ancestors[0], "parent"})
	}
	// Children precede siblings; neither relationship proves direct communication.
	for _, group := range []struct {
		ps       []relatedProcess
		relation string
	}{{d.Children, "child"}, {d.Siblings, "sibling"}} {
		ps := append([]relatedProcess(nil), group.ps...)
		sort.Slice(ps, func(i, j int) bool { return ps[i].PID < ps[j].PID })
		for _, p := range ps {
			candidates = append(candidates, candidate{p, group.relation})
		}
	}
	if len(candidates) > 12 {
		candidates = candidates[:12]
		d.Notes = append(d.Notes, "family_network: truncated to 12 relatives (parent, children, then siblings)")
	}
	for _, c := range candidates {
		if ctx.Err() != nil {
			d.Notes = appendOnce(d.Notes, "family_network: collection time limit reached")
			break
		}
		p, e := process.NewProcessWithContext(ctx, c.p.PID)
		if e != nil {
			continue
		}
		birth, e := p.CreateTimeWithContext(ctx)
		if e != nil {
			continue
		}
		identity := processIdentity(ctx, p)
		// Reject relatives that changed parent or executable since enumeration.
		if identity.ParentPID != c.p.ParentPID || identity.Name != c.p.Name || identity.Executable != c.p.Executable {
			continue
		}
		sockets, e := p.ConnectionsWithContext(ctx)
		if e != nil {
			d.Notes = appendOnce(d.Notes, "family_network: some relatives' connections unavailable")
			continue
		}
		fresh, e := process.NewProcessWithContext(ctx, p.Pid)
		if e != nil {
			continue
		}
		after, e := fresh.CreateTimeWithContext(ctx)
		if e != nil || after != birth {
			continue
		}
		sort.Slice(sockets, func(i, j int) bool {
			a, b := sockets[i], sockets[j]
			if a.Laddr.IP != b.Laddr.IP {
				return a.Laddr.IP < b.Laddr.IP
			}
			if a.Laddr.Port != b.Laddr.Port {
				return a.Laddr.Port < b.Laddr.Port
			}
			if a.Raddr.IP != b.Raddr.IP {
				return a.Raddr.IP < b.Raddr.IP
			}
			return a.Raddr.Port < b.Raddr.Port
		})
		entry := familyNetworkEvidence{Process: identity, Relationship: c.relation, StartedMS: birth, Connections: []socketEvidence{}}
		for _, s := range sockets {
			if s.Family != 2 && s.Family != 10 {
				continue
			}
			if len(entry.Connections) >= 16 {
				d.Notes = appendOnce(d.Notes, "family_network: truncated at 16 internet sockets per relative")
				break
			}
			entry.Connections = append(entry.Connections, socketRecord(s))
		}
		d.FamilyNetwork = append(d.FamilyNetwork, entry)
	}
	d.Notes = append(d.Notes, "Family network activity belongs to the labeled relative, not necessarily the selected process or the same application. Parentage does not prove communication; endpoints do not identify remote software.")
}

package main

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func nodeLabel(name string, pid int32) string { return fmt.Sprintf("%s [%d]", oneLine(name), pid) }
func (m model) treeLines() []string {
	ps := m.rows()
	if len(ps) == 0 {
		return []string{"No process selected."}
	}
	selected := ps[min(m.cursor, len(ps)-1)]
	if m.collecting {
		selected = m.workProcess
	}
	// In review/findings, show the process that was actually investigated.
	useEvidence := !m.collecting && m.evidence != nil && (m.localEvidence && (m.pending != "" || m.answer != "" || m.busy) || m.evidence.Process.PID == selected.PID)
	byPID := map[int32]Process{}
	for _, p := range m.state.Processes {
		byPID[p.PID] = p
	}
	ancestors := []relatedProcess{}
	children := []relatedProcess{}
	siblings := []relatedProcess{}
	lines := []string{dim.Render("↓ parent → child"), dim.Render("↔ observed socket peer / owner"), ""}
	if useEvidence {
		d := m.evidence
		selected = Process{PID: d.Process.PID, ParentPID: d.Process.ParentPID, Name: d.Process.Name}
		ancestors = append([]relatedProcess(nil), d.Ancestors...)
		children = append([]relatedProcess(nil), d.Children...)
		siblings = append([]relatedProcess(nil), d.Siblings...)
		lines = append(lines, dim.Render("Evidence at "+d.Captured.Format("15:04:05")))
	} else {
		lines = append(lines, dim.Render("Live process family"))
		seen := map[int32]bool{selected.PID: true}
		parent := selected.ParentPID
		for parent > 0 && !seen[parent] && len(ancestors) < 8 {
			seen[parent] = true
			p, ok := byPID[parent]
			if !ok {
				ancestors = append(ancestors, relatedProcess{PID: parent, Name: "parent unavailable"})
				break
			}
			ancestors = append(ancestors, relatedProcess{PID: p.PID, ParentPID: p.ParentPID, Name: p.Name})
			parent = p.ParentPID
		}
		for _, p := range m.state.Processes {
			if p.PID == selected.PID {
				continue
			}
			r := relatedProcess{PID: p.PID, ParentPID: p.ParentPID, Name: p.Name}
			if p.ParentPID == selected.PID {
				children = append(children, r)
			} else if selected.ParentPID > 0 && p.ParentPID == selected.ParentPID {
				siblings = append(siblings, r)
			}
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].PID < children[j].PID })
	sort.Slice(siblings, func(i, j int) bool { return siblings[i].PID < siblings[j].PID })
	depth := 0
	for i := len(ancestors) - 1; i >= 0; i-- {
		prefix := ""
		if depth > 0 {
			prefix = strings.Repeat(" ", min(depth-1, 5)*2) + "└─↓ "
		}
		lines = append(lines, dim.Render(prefix+nodeLabel(ancestors[i].Name, ancestors[i].PID)))
		depth++
	}
	prefix := ""
	if depth > 0 {
		prefix = strings.Repeat(" ", min(depth-1, 5)*2) + "└─↓ "
	}
	lines = append(lines, accent.Render(prefix+"● "+nodeLabel(selected.Name, selected.PID)))
	childPrefix := strings.Repeat(" ", min(depth, 5)*2)
	for i, p := range children {
		branch := "├─↓ "
		if i == len(children)-1 {
			branch = "└─↓ "
		}
		lines = append(lines, textStyle.Render(childPrefix+branch+nodeLabel(p.Name, p.PID)))
	}
	if len(children) == 0 {
		lines = append(lines, dim.Render(childPrefix+"└─ no visible children"))
	}
	if len(siblings) > 0 {
		lines = append(lines, "", dim.Render("SAME PARENT · siblings"))
		for _, p := range siblings {
			lines = append(lines, dim.Render("├─ "+nodeLabel(p.Name, p.PID)))
		}
	}
	lines = append(lines, "", dim.Render("NETWORK RELATIONSHIPS"))
	if useEvidence {
		peers := 0
		other := 0
		seen := map[int32]bool{}
		for _, c := range m.evidence.Connections {
			if c.LocalPeer == nil {
				other++
				continue
			}
			p := c.LocalPeer
			if seen[p.PID] {
				continue
			}
			seen[p.PID] = true
			peers++
			lines = append(lines, textStyle.Render("├─↔ "+nodeLabel(p.Name, p.PID)), dim.Render("│   "+net.JoinHostPort(c.Remote.IP, strconv.Itoa(int(c.Remote.Port)))))
		}
		if peers == 0 {
			lines = append(lines, dim.Render("No local peer identified."))
		}
		if other > 0 {
			lines = append(lines, dim.Render(fmt.Sprintf("%d other sockets in evidence", other)))
		}
		lines = append(lines, "", dim.Render("UNIX PEER DESCRIPTOR OWNERS"))
		for _, peer := range m.evidence.UnixPeers {
			lines = append(lines, dim.Render(fmt.Sprintf("inode %d ↔ %d", peer.LocalInode, peer.PeerInode)))
			for _, owner := range peer.Owners {
				lines = append(lines, textStyle.Render("├─↔ held by "+nodeLabel(owner.Name, owner.PID)))
			}
			if len(peer.Owners) == 0 {
				lines = append(lines, dim.Render("└─ owner unavailable"))
			}
		}
		lines = append(lines, "", dim.Render("RELATIVES' INTERNET SOCKETS"), dim.Render("Parentage ≠ communication"))
		for _, entry := range m.evidence.FamilyNetwork {
			lines = append(lines, textStyle.Render("├─ "+entry.Relationship+": "+nodeLabel(entry.Process.Name, entry.Process.PID)))
			for _, c := range entry.Connections {
				lines = append(lines, dim.Render("│  "+c.Protocol+" → "+net.JoinHostPort(c.Remote.IP, strconv.Itoa(int(c.Remote.Port)))+" "+c.State))
			}
			if len(entry.Connections) == 0 {
				lines = append(lines, dim.Render("│  no internet sockets returned"))
			}
		}
		lines = append(lines, "", dim.Render("Collected snapshot; may change."), dim.Render("See overview for missing data."))
	} else {
		lines = append(lines, dim.Render("Press i to collect socket peers."))
	}
	return lines
}
func (m model) treePanel(w, h int) string {
	lines := m.treeLines()
	visible := max(1, h-3)
	start := min(m.treeOffset, max(0, len(lines)-visible))
	end := min(len(lines), start+visible)
	content := append([]string(nil), lines[start:end]...)
	for len(content) < h-3 {
		content = append(content, "")
	}
	content = append(content, dim.Render(ansi.Truncate(fmt.Sprintf("[ / ] scroll · %d–%d/%d", start+1, end, len(lines)), w-4, "…")))
	return panel("PROCESS MAP", content, w, h)
}

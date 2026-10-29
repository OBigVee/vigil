# Vigil

A concurrent Go terminal system monitor with optional DeepSeek diagnostics.

The terminal dashboard includes resource bars, recent CPU/memory sparklines, a highlighted process table, and a process relationship map on the left of wide screens. It adapts to terminals at least 48 columns by 16 rows.

## Install from source

Linux is the initial development target. Install Git and Go 1.24 or later, then:

```sh
git clone https://github.com/OBigVee/vigil.git
cd vigil
go build -o vigil .
./vigil
```

Local monitoring, search, and process inspection work without an API key. To enable Vigil Intelligence, create your own local `.env` using `.env.example` as a reference (see setup below). Each user supplies their own provider key.

This repository currently documents building from source. Prebuilt downloads can be distributed separately through GitHub Releases; platform-specific functionality still needs testing before claiming Windows or macOS support.

## Run

Requires Go 1.24 or later and an interactive terminal.

```sh
go run .
# Or build a binary:
go build -o vigil .
./vigil
# Machine-readable snapshot (no AI request):
./vigil --snapshot
# Local process evidence, without an AI request:
./vigil --inspect 1234
```

Vigil samples CPU, memory, and running processes continuously. OS queries run outside the UI event loop, using eight process workers. Refreshes begin one second after each completed sample; slow collections never overlap. CPU percentages need a second sample to warm up. Per-process CPU can exceed 100% across multiple cores. Permissions and OS support may limit metrics; individual inaccessible or exited processes are skipped.

## Controls

| Key | Action |
| --- | --- |
| ↑/↓ or k/j | Select a process |
| c / m / n | Sort by CPU / memory / name |
| / | Search by process name or exact PID; Enter applies, Esc clears |
| Esc | Clear filter, dismiss results, cancel review or request |
| Tab | Toggle investigation overview / exact JSON |
| t | Toggle the process map (full-width on narrow terminals) |
| [ / ] | Scroll the process map up / down |
| PgUp / PgDn | Scroll investigation evidence or findings by a page |
| Space | Pause/resume collection |
| i / e | Collect and review evidence for the selected process; Enter requests an explanation |
| d | Diagnose resource pressure |
| a | Triage unusual activity |
| q / Ctrl+C | Quit |

## Find a process

Press `/` and type a name (such as `firefox`) or a full PID (such as `1234`). Results update while typing. Names match case-insensitively, including partial names; PIDs match exactly. Press Enter to select from results with ↑/↓, then `i` to inspect. Esc clears the search, and Ctrl+U erases the current search input. A numeric query can also match a numeric part of a process name.

Start with a search from the command line:

```sh
./vigil --search firefox
./vigil --search 1234
./vigil --snapshot --search firefox
```

Search covers the processes visible to your user account and current OS permissions.

## Process relationship map

The left pane shows the selected process's visible parent chain, direct children, and siblings. The process table remains on the right. It appears automatically at 100 columns or wider when there is enough vertical space; `t` toggles it. On smaller terminals, `t` switches between the map and the main content. `[` and `]` scroll longer maps.

`↓` marks an observed parent/child relationship; `↔` marks an identified local TCP peer. Siblings are grouped separately because they share a parent rather than a connection. Parentage does not prove an application dependency or identify the original launcher after reparenting.

The normal map uses live process-family data. Press `i` to collect network evidence. After inspection, the map uses that collected snapshot and displays its timestamp; it stays pinned to the investigated PID while reviewing evidence or reading its explanation. Unidentified socket endpoints remain in the detailed evidence without invented software names. Permissions and collection limits can hide relationships; review collection notes for gaps.

## DeepSeek setup

Create a `.env` file in the directory where you launch Vigil (the project root when using `go run .`):

```dotenv
DEEPSEEK_API_KEY=your-key-here
DEEPSEEK_MODEL=deepseek-flash
```

Vigil automatically loads this file at startup. Existing environment variables take priority. `.env` is ignored by Git; `.env.example` contains placeholders only. Never commit a real key or paste it into chat. A missing file is fine; malformed or unreadable files produce an error without displaying their contents. Snapshot mode does not load `.env`.

```sh
go run .
```

Default model: `deepseek-flash`. Override with `DEEPSEEK_MODEL` if needed. Requests use the official HTTPS chat-completions endpoint and a 60-second timeout. Monitoring works without a key.

## Process investigations

Select a process and press `i` or `e`. Vigil collects evidence in the background while monitoring continues, then opens a readable, scrollable local overview. Press Tab to inspect the exact JSON payload. This works without an API key. Enter/y sends the reviewed evidence to DeepSeek for an explanation; Esc/n returns without sending. Long lines wrap so complete paths and endpoints can be reviewed. Esc also cancels collection or a pending API request.

While collecting, a dedicated panel animates and shows the selected PID, elapsed time, and current OS collection stage. Its checklist follows actual collector events rather than a simulated percentage. Monitoring continues in the background. After you confirm the reviewed payload, a separate interpretation panel shows that Vigil is waiting for DeepSeek. Esc shows cancellation feedback, and failures display an error state rather than appearing as AI findings.

Evidence includes:

- OS/distribution, architecture, and logical CPU count.
- PID, executable path, owner, start time, working directory, sampled CPU usage and resident memory.
- Up to eight ancestors, 32 direct children, and 16 siblings, with names and executable paths when accessible.
- Up to 64 distinct open file paths and 32 mapped file paths; no file contents.
- Up to 64 sockets, including local/remote addresses, ports, protocol, and state. Visible local TCP peer processes are matched using reverse socket endpoints; a remote address alone cannot identify software or a paired device.
- Linux cgroup membership (service/container clues) and executable package ownership from installed `dpkg-query`, `rpm`, or `pacman` metadata. Package ownership is not an integrity check. Executables installed outside these package managers may have no package match.

Collection has a ten-second context deadline. Process identity is rechecked before accepting evidence to detect exit or PID reuse. Observations span an interval rather than an atomic snapshot. Permission errors, unsupported fields, and truncated lists are marked in `collection_notes`. Most useful detail is currently available on Linux. Monitoring never automatically elevates privileges.

The explanation prompt requires claims tied to the actual paths, PIDs, relationships, and endpoints in the evidence, with observations separated from hypotheses. Sibling processes only share a parent; open or mapped paths do not establish file contents, and sockets do not reveal remote application identities.

Paths, usernames, and network endpoints may be private. Review them before sending. Command arguments, environment variables, file contents, and hostname are not collected. No request is made until you confirm; requests incur your provider's normal API charges. No process is terminated automatically. `--inspect PID` exports the same local evidence without loading `.env` or making an AI request.

Whole-machine diagnostics (`d`) and triage (`a`) still send a lighter snapshot: OS, timestamp, aggregate CPU/RAM, and up to 30 processes sorted by CPU. They do not yet investigate every process individually.

AI results are hypotheses. Process names can be spoofed, and one snapshot cannot prove malware, overheating, or a root cause. Current triage has no persistent trusted baseline. Temperature sensors, disk/network monitoring, baseline learning, and cross-platform validation remain future work. Linux is the initial development target; gopsutil provides platform-dependent support elsewhere.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
```

Tests use a local HTTP server and controlled local file/socket fixtures; no real key or paid request is needed.

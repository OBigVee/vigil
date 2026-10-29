# Vigil

A concurrent Go terminal system monitor with optional DeepSeek diagnostics.

## Run

Requires Go 1.24 or later and an interactive terminal.

```sh
go run .
# Or build a binary:
go build -o vigil .
./vigil
# Machine-readable snapshot (no AI request):
./vigil --snapshot
```

Vigil samples CPU, memory, and running processes continuously. OS queries run outside the UI event loop, using eight process workers. Refreshes begin one second after each completed sample; slow collections never overlap. CPU percentages need a second sample to warm up. Per-process CPU can exceed 100% across multiple cores. Permissions and OS support may limit metrics; individual inaccessible or exited processes are skipped.

## Controls

| Key | Action |
| --- | --- |
| ↑/↓ or k/j | Select a process |
| c / m / n | Sort by CPU / memory / name |
| / | Edit a process-name filter; Enter finishes |
| Esc | Clear filter, dismiss results, cancel review or request |
| Space | Pause/resume collection |
| e | Explain selected process |
| d | Diagnose resource pressure |
| a | Triage unusual activity |
| q / Ctrl+C | Quit |

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

Every AI action opens a scrollable JSON review. Enter/y sends it; Esc/n cancels. Process explanations send only the selected PID, name, CPU, and memory percentages. Diagnostics and triage send OS, timestamp, aggregate CPU/RAM, and up to 30 processes sorted by CPU. No command lines, environment variables, usernames, paths, or hostname are collected. Diagnostic instructions accompany the reviewed JSON. Requests incur your provider's normal API charges. No automatic requests or process termination occur.

AI results are hypotheses. Process names can be spoofed, and one snapshot cannot prove malware, overheating, or a root cause. Current triage has no persistent trusted baseline. Temperature sensors, disk/network monitoring, baseline learning, and cross-platform validation remain future work. Linux is the initial development target; gopsutil provides platform-dependent support elsewhere.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
```

Tests use a local HTTP server; no real key or paid request is needed.

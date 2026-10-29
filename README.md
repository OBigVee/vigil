# Vigil

A concurrent Go terminal system monitor with optional Vigil Intelligence diagnostics.

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

## Prebuilt downloads

Download an archive from [GitHub Releases](https://github.com/OBigVee/vigil/releases). Choose `linux`, `darwin` (macOS), or `windows`, and `amd64` (Intel/AMD) or `arm64` (ARM / Apple Silicon). Extract it, open a terminal in that folder, and run `./vigil` or `.\vigil.exe` in Windows PowerShell. No Go installation is needed. Check the release's `SHA256SUMS` against your archive before running it.

Linux amd64 is the runtime-tested development platform. Linux arm64 and macOS/Windows downloads are experimental cross-compiled builds and need runtime validation; their availability does not imply feature parity. Binaries are unsigned and macOS builds are not notarized.

Run `./vigil --version` to identify the build. For a private user configuration that works regardless of the launch directory, run `./vigil --init-config`, then edit the file path printed by that command.

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

## Connect an intelligence provider

Vigil supports DeepSeek, OpenAI (Responses API), Gemini (generateContent), and services exposing an OpenAI-compatible Chat Completions API. Each user supplies their own API key; no shared key is bundled. API billing and model access belong to the user's provider account. A chat subscription is not itself an API key. Local monitoring and evidence inspection work without an external service.

Create your private configuration:

```sh
./vigil --init-config
```

Open the printed file in your preferred text editor. Choose one provider and set its key:

```dotenv
VIGIL_PROVIDER=openai
VIGIL_API_KEY=your-own-api-key
```

| Provider value | Default model | Alternative key variable |
| --- | --- | --- |
| `deepseek` | `deepseek-flash` | `DEEPSEEK_API_KEY` |
| `openai` | `gpt-5.6-terra` | `OPENAI_API_KEY` |
| `gemini` | `gemini-3.8-flash` | `GEMINI_API_KEY` |
| `compatible` | Set `VIGIL_MODEL` explicitly | `VIGIL_API_KEY` |

`VIGIL_API_KEY` takes priority over the selected provider's key variable. `VIGIL_MODEL` overrides the default; `DEEPSEEK_MODEL`, `OPENAI_MODEL`, and `GEMINI_MODEL` also work for their respective providers. Models and account availability change; choose a model your account can access. Default provider is `deepseek` for compatibility with existing `.env` files. Set `VIGIL_PROVIDER` explicitly to switch services. Unknown providers and incomplete configurations produce an actionable setup error when interpretation is requested.

For another hosted service with an OpenAI-compatible API:

```dotenv
VIGIL_PROVIDER=compatible
VIGIL_BASE_URL=https://your-provider.example/v1
VIGIL_MODEL=your-model-id
VIGIL_API_KEY=your-own-api-key
```

The base URL must end at the API prefix, without `/chat/completions`. Remote endpoints require HTTPS. HTTP is allowed only on loopback addresses for local servers; keyless loopback services are supported. Responses must match non-streaming Chat Completions text output. Vendor-specific APIs, Azure-specific authentication, tool-only responses, and every provider's custom options are not universally supported.

Configuration locations use Go's per-user config directory: typically `~/.config/vigil/config.env` on Linux (or `$XDG_CONFIG_HOME`), `~/Library/Application Support/vigil/config.env` on macOS, and `%AppData%\vigil\config.env` on Windows. `--init-config` creates a blank-key file with mode 0600 on Unix and never overwrites an existing file. On Windows, access follows the user's directory ACLs.

Configuration precedence: existing environment variables, then `.env` in the current working directory, then the per-user config file. `--config /path/to/settings.env` loads only the explicit file beneath existing environment variables. Restart Vigil after editing. Missing default files are fine; malformed/unreadable files produce an error without showing their contents. Snapshot/inspection exports and `--version` do not load keys. Existing `DEEPSEEK_API_KEY` / `DEEPSEEK_MODEL` setups continue to work.

Never commit real keys or share your config file. `.env` is Git-ignored; `.env.example` contains blank placeholders. Keys are sent only to the configured endpoint for confirmed interpretations. Requests have a 90-second timeout and do not follow redirects. OpenAI Responses requests set `store=false`. No paid live API calls are required by the test suite.

Protocol references: [OpenAI Responses](https://developers.openai.com/api/reference/python/resources/responses/methods/create), [OpenAI default model](https://developers.openai.com/api/docs/models/gpt-5.6-terra), [Gemini generateContent](https://ai.google.dev/api/generate-content), [DeepSeek Chat Completions](https://api-docs.deepseek.com/api/create-chat-completion/).

## Process investigations

Select a process and press `i` or `e`. Vigil collects evidence in the background while monitoring continues, then opens a readable, scrollable local overview. Press Tab to inspect the exact JSON payload. This works without an API key. Enter/y sends the reviewed evidence to the configured intelligence service for an explanation; Esc/n returns without sending. Long lines wrap so complete paths and endpoints can be reviewed. Esc also cancels collection or a pending API request.

While collecting, a dedicated panel animates and shows the selected PID, elapsed time, and current OS collection stage. Its checklist follows actual collector events rather than a simulated percentage. Monitoring continues in the background. After you confirm the reviewed payload, a separate interpretation panel shows that Vigil is waiting for the configured intelligence service. Esc shows cancellation feedback, and failures display an error state rather than appearing as AI findings.

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

## Building a release

```sh
python3 scripts/build_release.py v0.1.0
```

This cross-compiles six binaries with CGO disabled and creates allowlisted archives plus SHA-256 checksums under `dist/v0.1.0/`. No workspace directories, credentials, or personal config files are archived. Run tests first, commit the sources, create and push the matching version tag, then use `scripts/publish_release.py` with `GH_TOKEN` supplied through the environment to upload a draft and publish it as a prerelease. Published releases are never overwritten by that script.

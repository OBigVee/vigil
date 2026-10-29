# Vigil v0.1.0 — preview

Vigil is a Go terminal system monitor with optional machine-specific intelligence.

## Downloads

Choose your operating system and CPU architecture:

- **Linux amd64**: most Intel/AMD PCs. Primary development and runtime test platform.
- **Linux arm64**: ARM64 machines. Cross-compiled; needs hardware validation.
- **macOS (darwin) amd64 / arm64**: Intel / Apple Silicon. Experimental cross-compiled builds; not runtime-tested. Cgroups and mapped-file collection are Linux-only.
- **Windows amd64 / arm64**: experimental cross-compiled builds; not runtime-tested. Some process evidence may be unavailable. Use Windows Terminal or another modern terminal.

Archives contain only the executable, README, blank configuration example, and these release notes. No keys or personal configuration are included. `SHA256SUMS` allows you to check download integrity. These binaries are not code-signed or notarized.

Extract the archive, open a terminal in its folder, and run `./vigil` (Linux/macOS) or `.\vigil.exe` (Windows PowerShell). No Go installation is needed. Local monitoring and inspection work without an API key.

## Connect your own intelligence provider

Run `./vigil --init-config` (Windows: `.\vigil.exe --init-config`). It prints the location of your new configuration file. Open that file in a text editor and set:

```dotenv
VIGIL_PROVIDER=openai
VIGIL_API_KEY=your-own-api-key
```

Choose `deepseek`, `openai`, `gemini`, or `compatible`. Override the model with `VIGIL_MODEL`. Compatible services also require `VIGIL_BASE_URL` pointing to their API base URL. Keyless loopback endpoints are supported for local servers. Arbitrary vendor-specific APIs are not automatically supported.

Restart Vigil after saving. Press `/` to find a name or PID, `i` to collect process evidence, and Enter to request an interpretation after reviewing the payload. External requests use your provider account and its billing; access and model availability depend on that account. Provider integrations have mock-server tests; paid live calls have not been tested for this release.

## Included

- Live CPU, memory, process selection, sorting, pause, and name/PID search.
- Process tree with parent/child relationships and identified local TCP peers.
- Executable/package identity, files, working directory, sockets and Linux service clues.
- Animated collection stages, evidence review, and optional intelligence interpretation.
- DeepSeek, OpenAI Responses, Gemini generateContent, and OpenAI-compatible chat endpoints.
- Private per-user configuration, environment variables, and legacy `.env` support.
- Offline JSON exports through `--snapshot` and `--inspect PID`.

## Limits

This is an early preview, not a fully validated cross-platform stable release. Process permissions and platform support affect evidence availability. Snapshot-based triage has no trusted baseline and does not prove malware or root cause. Windows/macOS runtime behavior, temperature sensors, long-term process histories, and disk/network throughput monitoring remain future work.

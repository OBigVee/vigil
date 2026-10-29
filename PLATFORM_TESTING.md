# Platform validation

The `Platform validation` GitHub Actions workflow builds and runs Vigil on native OS runners. It runs on feature/fix branch pushes, pull requests, main, and manual dispatch. No LLM keys, private machine data, or elevated process privileges are required. The workflow uses read-only repository permissions and pinned action commits.

## Automated matrix

| Runner | Architecture | Go | CGO | Purpose |
| --- | --- | --- | --- | --- |
| Ubuntu 24.04 | amd64 | current stable | enabled | Linux regression coverage |
| Ubuntu 24.04 | amd64 | 1.24.x | enabled | Minimum supported Go version |
| macOS 14 | arm64 | current stable | enabled | Apple Silicon native behavior |
| macOS 15 Intel | amd64 | current stable | disabled | Intel Mac shipped no-CGO build mode |
| Windows 2025 | amd64 | current stable | enabled | Windows native behavior |
| macOS 14 | arm64 | current stable | disabled | Validate the shipped no-CGO build mode |

Race detection runs in CGO-enabled jobs; the no-CGO job runs ordinary tests. These jobs do not validate Linux arm64 or Windows arm64 runtime behavior yet. Compiling a release for those architectures is not proof that it runs correctly.

Each job verifies dependency checksums, runs vet and tests, builds an executable, and exercises its version, config setup, snapshot output and search behavior. OS integration tests use the test process itself, a temporary file, a loopback listener, and a controlled child Go process. They check process identity, resource collection, ancestry and child relationships. Paths are compared after resolving symlinks, with Windows case normalization.

Linux fixture files, sockets and working directories must be found. On other platforms, unavailable fields must be explicitly reported in collection notes; successful calls with silently missing fixture data fail validation. Aggregate CPU must work in every lane, including macOS without CGO. A collection warning fails the executable smoke test. These allowances describe capabilities, not full feature parity.

The executable smoke test emits only an OS/architecture/capability summary. It does not print other process names, files, endpoints, configuration contents or environment variables. It never requests an AI interpretation. Full integration test logs may contain test fixture paths in failure messages; do not submit private investigation reports as CI artifacts.

## Run locally

```sh
go test -race -count=1 -timeout=4m ./...
go vet ./...
go build -ldflags '-X main.version=platform-validation' -o dist/platform-validation/vigil .
python3 scripts/validate_platform.py --binary dist/platform-validation/vigil
```

On Windows, add `.exe` to the binary name and use an installed Python interpreter. Race tests require a supported C compiler. For a deliberate no-CGO macOS check, set `CGO_ENABLED=0` and omit `-race`; the same strict smoke checks apply.

## Known gaps and next collector work

v0.1.0 used gopsutil v3 and was cross-built with CGO disabled. That combination lacked aggregate CPU and current working directory support on macOS, returned an incorrect executable path in the fixture, and did not implement macOS open-file collection. Keep the original v0.1.0 binaries experimental.

The macOS fix migrates to gopsutil v4.26.7 (retaining Go 1.24 compatibility) for CPU, executable and working directory collection without CGO. A Darwin-only, PID-scoped `/usr/sbin/lsof` invocation supplies regular-file and directory descriptor paths. Its NUL-delimited output is bounded to 8 MiB and a three-second child context; errors and timeouts become explicit collection notes. No shell, arguments, environment values or file contents are collected. macOS fixtures now require working directories, file paths and listener evidence, rather than accepting unavailable notes. Linux-only mapped-file and cgroup collection remain unavailable on macOS.

After examining actual native job results, use focused follow-up branches:

- `fix/macos-collector`: validate the no-CGO CPU/identity migration and open-file collector on Intel and Apple Silicon.
- `fix/windows-collector`: address observed permission/path/socket/service issues.
- A future release pipeline should run each shipped binary's checks on its target OS before publication. Publish fixes under a new version rather than replacing v0.1.0 assets.

## Manual desktop checks

CI does not exercise a user's interactive desktop session. On each target machine:

1. Start Vigil in a normal modern terminal without administrator/root privileges.
2. Check CPU and RAM against Activity Monitor, Task Manager, or a Linux monitor over similar sample intervals. Account for multicore per-process CPU normalization.
3. Search by name and exact PID; inspect a process owned by the current user.
4. Confirm its executable, working directory, process family, known open file and local listener against independently observable fixtures.
5. Try a restricted process; unavailable data must be labeled rather than fabricated.
6. Resize the terminal; test tree toggling and scrolling, evidence overview/JSON, inspection progress, cancellation and exit.
7. Confirm local monitoring works without an API key. Optional real API testing requires the tester's own key and explicit evidence confirmation.

Record OS version, CPU architecture, terminal, Vigil version, the expected observation, actual result and sanitized collection notes. Never include API keys, full environment dumps, or private raw process payloads in issue reports.

## macOS fix verification

The macOS fix application code at `1525424` passed native Apple Silicon tests with CGO both enabled and disabled. The identical application code at `cc63b11` passed Intel macOS tests in release no-CGO mode; only CI/documentation changed between those commits. Linux minimum/current Go regression jobs passed. The Windows open-file fixture still fails and is separate work. Run links are recorded in RELEASE_NOTES.md.

Mac-only patch archives can be built with `python3 scripts/build_release.py v0.1.1 --target darwin/amd64 --target darwin/arm64`. Pass the same target flags to the publisher so the checksummed manifest must match the intended platforms. Published v0.1.0 assets are not overwritten.

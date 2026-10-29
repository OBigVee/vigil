# Vigil v0.1.1 — macOS collector fix (preview)

This patch provides corrected macOS downloads for Intel and Apple Silicon. It fixes the CPU and process-evidence limitations in v0.1.0 without requiring users to install Go or enable CGO.

## Download and run

- `vigil_v0.1.1_darwin_arm64.tar.gz`: Apple Silicon (M1/M2/M3 and newer ARM64 Macs).
- `vigil_v0.1.1_darwin_amd64.tar.gz`: Intel Macs.
- `SHA256SUMS`: SHA-256 checksums for the two archives.

Extract the matching archive, open Terminal in the extracted directory, and run `./vigil`. Check `./vigil --version` reports v0.1.1. Archives contain the executable, README, blank configuration example and these notes. No credentials are included. Developer ID signing and notarization are not provided.

Existing users should replace the old binary and keep their private configuration file. The provider settings and evidence-review controls have not changed. New users can run `./vigil --init-config`, open the printed file and set their provider and API key. Local inspection works without a key.

## Fixed on macOS

- Aggregate CPU collection works in the CGO-disabled mode used by the downloads.
- Executable paths and working directories use the maintained native macOS implementation, replacing the incorrect no-CGO executable-path fallback.
- Open regular-file and directory descriptor paths are collected through PID-scoped `/usr/sbin/lsof` metadata. Collection is time- and size-bounded and does not read file contents, process arguments or environment values.
- The dependency upgrade retains Go 1.24 support for source builds.

## Verification

Native Intel and Apple Silicon checks cover CPU, memory, process identity and family, working directory, an open fixture file and a loopback listener. Both release architectures passed without CGO. Linux regression tests passed on minimum and current Go versions. These are automated OS and executable checks; manual desktop testing and live paid LLM calls have not been performed for this patch.

Validation runs: [Apple Silicon and CGO-enabled native checks](https://github.com/OBigVee/vigil/actions/runs/40911107843), [Intel release-mode checks](https://github.com/OBigVee/vigil/actions/runs/40911662586). The application source and dependency versions are identical between these two runs; the second adjusts the Intel CI build mode.

## Remaining limits

- Permissions can restrict evidence from protected processes. Inaccessible data is reported in collection notes.
- Cgroups and mapped-file collection remain Linux-only. No temperature/history/baseline features were added.
- This is still a preview and does not establish complete platform parity.
- This patch publishes macOS archives only. Linux and Windows downloads remain available in [v0.1.0](https://github.com/OBigVee/vigil/releases/tag/v0.1.0). Windows still fails the open-file fixture and remains experimental; it is not claimed fixed by this patch.

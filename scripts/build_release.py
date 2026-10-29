#!/usr/bin/env python3
"""Build explicit release archives; never package the working directory or secrets."""
import argparse
import concurrent.futures
import hashlib
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
TARGETS = [(system, arch) for system in ('linux', 'darwin', 'windows') for arch in ('amd64', 'arm64')]


def build(version, target, output):
    system, arch = target
    name = f'vigil_{version}_{system}_{arch}'
    with tempfile.TemporaryDirectory(prefix='vigil-build-') as temp:
        folder = Path(temp) / name
        folder.mkdir()
        binary = folder / ('vigil.exe' if system == 'windows' else 'vigil')
        env = os.environ.copy()
        env.update(GOOS=system, GOARCH=arch, CGO_ENABLED='0')
        subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags',
                        f'-s -w -X main.version={version}', '-o', str(binary), '.'],
                       cwd=ROOT, env=env, check=True)
        # This allowlist is intentional. Never use a recursive workspace archive.
        for source in ('README.md', '.env.example', 'RELEASE_NOTES.md'):
            (folder / source).write_bytes((ROOT / source).read_bytes())
        if system == 'windows':
            archive = output / f'{name}.zip'
            with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as stream:
                for file in sorted(folder.iterdir()):
                    stream.write(file, f'{name}/{file.name}')
        else:
            archive = output / f'{name}.tar.gz'
            with tarfile.open(archive, 'w:gz') as stream:
                for file in sorted(folder.iterdir()):
                    stream.add(file, arcname=f'{name}/{file.name}', recursive=False)
    print(f'Built {archive.name}', flush=True)
    return archive


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('version')
    parser.add_argument('--target', action='append', choices=[f'{s}/{a}' for s,a in TARGETS],
                        help='build only this target; repeat for multiple targets')
    args = parser.parse_args()
    if not re.fullmatch(r'v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?', args.version):
        parser.error('version must look like v0.1.0')
    output = ROOT / 'dist' / args.version
    output.mkdir(parents=True, exist_ok=True)
    targets = [tuple(t.split("/")) for t in dict.fromkeys(args.target)] if args.target else TARGETS
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        archives = list(executor.map(lambda target: build(args.version, target, output), targets))
    checksums = []
    for archive in sorted(archives):
        checksums.append(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n')
    (output / 'SHA256SUMS').write_text(''.join(checksums))
    print(f'Release assets ready in {output}', flush=True)


if __name__ == '__main__':
    main()

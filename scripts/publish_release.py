#!/usr/bin/env python3
"""Upload verified build assets to a draft, then publish the prerelease."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
REPO = 'OBigVee/vigil'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('version')
    args = parser.parse_args()
    if not re.fullmatch(r'v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?', args.version):
        parser.error('invalid version')
    token = os.environ.get('GH_TOKEN') or os.environ.get('GITHUB_TOKEN')
    if not token:
        parser.error('set GH_TOKEN or GITHUB_TOKEN; never save a token in this script')

    def request(method, url, body=None, binary=False):
        data = body if binary else json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(url, data=data, method=method, headers={
            'Authorization': f'Bearer {token}', 'Accept': 'application/vnd.github+json',
            'User-Agent': 'vigil-release', 'X-GitHub-Api-Version': '2022-11-28',
            'Content-Type': 'application/octet-stream' if binary else 'application/json'})
        try:
            with urllib.request.urlopen(req, timeout=180) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            # Do not log server bodies, credentials, or request headers.
            raise RuntimeError(f'GitHub API returned HTTP {error.code}') from None

    folder = ROOT / 'dist' / args.version
    checksum_file = folder / 'SHA256SUMS'
    checksums = checksum_file.read_text().splitlines()
    files = []
    for line in checksums:
        digest, name = line.split('  ', 1)
        if Path(name).name != name:
            raise RuntimeError('invalid asset filename')
        path = folder / name
        if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise RuntimeError(f'checksum failed for {name}')
        files.append(path)
    if len(files) != 6:
        raise RuntimeError('expected six platform archives')
    files.append(checksum_file)
    # Ensure the tag exists on GitHub before creating a release.
    api = f'https://api.github.com/repos/{REPO}'
    request('GET', f'{api}/git/ref/tags/{args.version}')
    releases = request('GET', f'{api}/releases?per_page=100')
    existing = next((release for release in releases if release['tag_name'] == args.version), None)
    if existing and not existing['draft']:
        raise RuntimeError('release is already published; do not overwrite published assets')
    release = existing or request('POST', f'{api}/releases', {
        'tag_name': args.version, 'name': f'Vigil {args.version} — first preview',
        'body': (ROOT / 'RELEASE_NOTES.md').read_text(), 'draft': True, 'prerelease': True})
    existing_assets = {asset['name'] for asset in release['assets']}
    for path in files:
        if path.name in existing_assets:
            raise RuntimeError('draft already has assets; inspect it before retrying uploads')
        upload = release['upload_url'].split('{', 1)[0] + '?name=' + urllib.parse.quote(path.name)
        request('POST', upload, path.read_bytes(), binary=True)
        print(f'Uploaded {path.name}', flush=True)
    published = request('PATCH', f"{api}/releases/{release['id']}", {'draft': False, 'prerelease': True})
    print(f"Published {published['html_url']}", flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(f'Release failed: {error}', file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
"""Run a built executable on its native OS. Never load keys or call an LLM."""
import argparse
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--expected-version', default='platform-validation')
    parser.add_argument('--allow-missing-cpu', action='store_true')
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())

    def run(*arguments):
        return subprocess.check_output([binary, *arguments], text=True, timeout=60)

    assert run('--version').strip() == f'Vigil {args.expected_version}', 'wrong binary version'
    with tempfile.TemporaryDirectory(prefix='vigil-platform-') as temp:
        config = Path(temp) / 'config.env'
        run('--init-config', '--config', str(config))
        assert 'VIGIL_API_KEY=\n' in config.read_text(), 'setup template must not include a key'
        if os.name != 'nt':
            assert config.stat().st_mode & 0o777 == 0o600, 'configuration is not private'
        repeat = subprocess.run([binary, '--init-config', '--config', str(config)],
                                capture_output=True, timeout=20)
        assert repeat.returncode != 0, 'setup must not overwrite existing configuration'
    snapshot = json.loads(run('--snapshot'))
    assert snapshot['total_memory_bytes'] > 0, 'missing memory collection'
    assert snapshot['processes'], 'no processes enumerated'
    assert all(p['pid'] >= 0 and p['name'] for p in snapshot['processes']), 'invalid process records'
    warning = snapshot.get('collection_warning', '')
    if not args.allow_missing_cpu:
        assert not warning, 'native collection returned warnings: ' + warning
    else:
        assert platform.system() == 'Darwin' and warning, 'expected the known macOS no-CGO gap'
    missing = json.loads(run('--snapshot', '--search', '__vigil_nonexistent_fixture_process__'))
    assert not missing['processes'], 'name search failed to exclude unrelated processes'
    # Summary intentionally excludes other processes' names, paths, and endpoints.
    print(json.dumps({'os': platform.system(), 'architecture': platform.machine(),
                      'version': args.expected_version, 'process_count': len(snapshot['processes']),
                      'memory_available': True, 'collection_warning': warning,
                      'config_setup': 'passed', 'search': 'passed'}, indent=2))


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Collect license texts from the exact Go packages linked into odds (offline)."""
import argparse
import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / 'source/internal/thirdparty/licenses.json'


def go(*args):
    return subprocess.check_output(['go', *args], cwd=ROOT / 'source', text=True)


def objects(stream):
    decoder = json.JSONDecoder()
    while stream.strip():
        value, end = decoder.raw_decode(stream.lstrip())
        yield value
        stream = stream.lstrip()[end:]


def collect():
    modules = {}
    for package in objects(go('list', '-deps', '-json', '.')):
        module = package.get('Module')
        if not module or module.get('Main'):
            continue
        if module.get('Replace'):
            raise ValueError('Generate notices from locked upstream modules, without local replacements.')
        path, directory = module['Path'], Path(module['Dir'])
        entry = modules.setdefault(path, {'path': path, 'version': module['Version'], 'texts': {}})
        current = Path(package['Dir'])
        while current == directory or directory in current.parents:
            for file in sorted(current.iterdir()):
                if file.is_file() and re.match(r'^(LICENSE|COPYING|NOTICE|OFL)([.\-_].*|$)', file.name, re.I):
                    entry['texts'][str(file.relative_to(directory))] = file.read_text()
            if current == directory:
                break
            current = current.parent
    goroot = Path(go('env', 'GOROOT').strip())
    entries = [{'path': 'Go', 'version': go('env', 'GOVERSION').strip(),
                'texts': {'LICENSE': (goroot / 'LICENSE').read_text()}}]
    entries += [modules[path] for path in sorted(modules)]
    for entry in entries:
        if not entry['texts']:
            raise ValueError('Missing license source: ' + entry['path'])
        entry['texts'] = [{'name': name, 'text': text} for name, text in sorted(entry['texts'].items())]
    return (json.dumps(entries, ensure_ascii=False, indent=2) + '\n').encode()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    data = collect()
    if args.check:
        if not OUTPUT.is_file() or OUTPUT.read_bytes() != data:
            raise ValueError('Dependency notices are stale; run make licenses and review the changes.')
    else:
        OUTPUT.parent.mkdir(parents=True, exist_ok=True)
        OUTPUT.write_bytes(data)
    print('Dependency notices verified.' if args.check else 'Dependency notices generated.')


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f'ERROR: {error}', file=sys.stderr)
        sys.exit(1)

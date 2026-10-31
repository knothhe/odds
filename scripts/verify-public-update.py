#!/usr/bin/env python3
"""Check a real public upgrade in a private bundle, without installing it."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
from release import ReleaseError, parse, precedence

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--current',required=True)
    parser.add_argument('--expected',required=True)
    args=parser.parse_args()
    parse(args.current)
    _, pre=parse(args.expected)
    if pre is not None or precedence(args.expected)<=precedence(args.current):
        raise ValueError('Expected version must be a newer stable release.')
    if sys.platform!='darwin' or os.uname().machine!='arm64':
        raise ValueError('Run on the supported macOS arm64 target.')
    config=json.loads((ROOT/'config/updates.json').read_text())
    if config['github']!='knothhe/odds' or not config['publicKey']:
        raise ValueError('Missing fixed project update configuration.')
    key=config['publicKey']
    destination=ROOT/'dist/verification/public-update.json'
    destination.unlink(missing_ok=True)
    feed='https://github.com/knothhe/odds/releases/latest/download/update-darwin-arm64.json'
    with tempfile.TemporaryDirectory(prefix='odds-public-check-') as temporary:
        directory=Path(temporary)
        executable=directory/'odds.app/Contents/MacOS/odds'
        executable.parent.mkdir(parents=True)
        flags='-X github.com/egoist/mygo.production=1 -X github.com/egoist/mygo.packageUpdateFeed='+feed+' -X github.com/egoist/mygo.packageUpdateKey='+key
        subprocess.run([sys.executable,str(ROOT/'scripts/go-patched.py'),'build','-ldflags',flags,
                        '-o',str(executable),'./cmd/check-public-update'],cwd=ROOT/'source',check=True)
        result=subprocess.run([str(executable),'--current',args.current,'--expected',args.expected,'--public-key',key],
                              cwd=directory,capture_output=True,text=True,timeout=330)
        if result.returncode:
            raise ValueError('Public update verification failed: '+result.stderr.strip())
        report=json.loads(result.stdout)
        destination.parent.mkdir(parents=True,exist_ok=True)
        destination.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
        print('Public native Check, authenticated DMG and isolated staging passed. No app replaced or launched. '+str(destination))


if __name__=='__main__':
    try:
        main()
    except (OSError,ValueError,ReleaseError,subprocess.SubprocessError) as error:
        print(f'ERROR: {error}',file=sys.stderr)
        sys.exit(1)

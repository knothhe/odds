#!/usr/bin/env python3
"""Project setup that never prints private key material."""
import json
import os
from pathlib import Path
import subprocess
import sys
sys.dont_write_bytecode = True
from macos import ROOT, BuildError, no_symlink, require


def keygen():
    directory = no_symlink(Path.home() / 'Library/Application Support/odds-signing')
    repository = ROOT.resolve()
    require(directory != repository and repository not in directory.parents, 'Keep update signing material outside source.')
    secret, public = directory / 'mygo-update.key', directory / 'mygo-update.pub'
    if not secret.exists() and not public.exists():
        subprocess.run(['go','tool','mygo','keygen','-o',str(directory)],cwd=ROOT,check=True,stdout=subprocess.DEVNULL)
    require(secret.is_file() and public.is_file() and not secret.is_symlink() and not public.is_symlink(),
            'Partial or symlinked signing key pair; restore the original pair, never replace a published key.')
    key = public.read_text().strip()
    result = subprocess.run(['go','run','./source/cmd/signing-key'],cwd=ROOT,input=secret.read_bytes(),capture_output=True)
    require(result.returncode == 0 and result.stdout.decode().strip() == key, 'Signing key pair does not match.')
    config = no_symlink(ROOT / 'config/updates.json')
    data = json.loads(config.read_text())
    require(not data['publicKey'] or data['publicKey'] == key, 'A different public key is already pinned; refusing key rotation.')
    if not data['publicKey']:
        data['publicKey'] = key
        config.write_text(json.dumps(data,indent=2)+'\n')
    directory.chmod(0o700)
    secret.chmod(0o600)
    print(f'Update public key pinned. Private key kept at {secret}; keep an offline backup. No secrets uploaded.')

if __name__ == '__main__':
    try:
        require(sys.argv[1:] == ['update-keygen'], 'Usage: project.py update-keygen')
        keygen()
    except (BuildError,OSError,ValueError,subprocess.CalledProcessError) as error:
        print(f'ERROR: {error}',file=sys.stderr)
        sys.exit(1)

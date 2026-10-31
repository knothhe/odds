"""Select an existing keychain identity and export only its certificate/key pair."""
from __future__ import annotations
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import shutil
import tempfile

from macos import ROOT, BuildError, require


def list_identities(run):
    """Return paired Code Signing identities without choosing or exporting any."""
    configured = os.environ.get('ODDS_SIGNING_KEYCHAIN', '').strip()
    keychains = ([configured] if configured else
                 shlex.split(run('security', 'list-keychains', '-d', 'user').decode()))
    identities = []
    seen = set()
    for keychain in keychains:
        output = run('security', 'find-identity', '-p', 'codesigning', keychain).decode()
        for identity, label in re.findall(r'\)\s+([A-Fa-f0-9]{40})\s+"([^\n"]+)"', output):
            identity = identity.upper()
            if identity not in seen:
                identities.append({'identity': identity, 'label': label, 'keychain': keychain})
                seen.add(identity)
    return identities


def _private_write(path, data):
    with path.open('xb') as stream:
        path.chmod(0o600)
        stream.write(data)


def export_identity(identity, keychain, run):
    """Export exactly the selected SecIdentity, including its paired private key."""
    identity = identity.upper()
    require(re.fullmatch('[A-F0-9]{40}', identity) is not None,
            'Choose an exact Code Signing identity SHA1 before exporting.')
    directory = Path(tempfile.mkdtemp(prefix='odds-signing-export-'))
    try:
        directory.chmod(0o700)
        archive = directory / 'selected.p12'
        _private_write(archive, b'')
        password = secrets.token_urlsafe(32)
        status = json.loads(run('swift', ROOT / 'scripts/export-identity.swift', keychain, identity,
                                archive, input=password.encode()))
        require(status == {'exported': True, 'certificateSHA1': identity},
                'The native export did not confirm the selected identity.')
        data = archive.read_bytes()
        require(bool(data), 'Selected identity export produced an empty P12.')
        return data, password
    finally:
        try:
            shutil.rmtree(directory)
        except OSError as error:
            raise BuildError('Private keychain export cleanup failed; no secrets uploaded.') from error

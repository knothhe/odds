#!/usr/bin/env python3
"""Build, verify and install odds bundles. No release or keychain mutation in build."""
from __future__ import annotations
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time

sys.dont_write_bytecode = True
from release import parse, ReleaseError
from mygo_patch import PatchError, patched_environment

ROOT = Path(__file__).resolve().parents[1]
LOCAL = ROOT / '.odds-local.json'
TARGET = 'darwin-arm64'

class BuildError(Exception):
    pass

def require(ok, message):
    if not ok:
        raise BuildError(message)

def run(*args, capture=False, env=None, cwd=ROOT, input=None):
    result = subprocess.run([str(a) for a in args], cwd=cwd, env=env, text=True, input=input,
                            stdout=subprocess.PIPE if capture else None, stderr=subprocess.PIPE if capture else None)
    if result.returncode:
        # Never print argv: CI imports pass secret values to security subprocesses.
        raise BuildError(f'{Path(str(args[0])).name} failed (exit {result.returncode})' +
                         (f': {result.stderr.strip()}' if capture else ''))
    return result.stdout.strip() if capture else ''

def version():
    value = json.loads((ROOT / 'mygo.json').read_text())['version']
    parse(value)
    return value

def identity_for(channel):
    return ('odds', 'com.knothhe.odds') if channel == 'release' else ('odds Dev', 'com.knothhe.odds.dev')

def no_symlink(path):
    path = Path(os.path.abspath(path))
    for part in [path, *path.parents]:
        require(not part.is_symlink(), f'Refusing symbolic-link path: {part}')
    return path

def read_pin(path):
    require(path.is_file() and not path.is_symlink(), f'Missing signing pin: {path}. See docs/build-release.md.')
    value = json.loads(path.read_text())
    require(set(value) == {'schema', 'certificateSHA256'} and value['schema'] == 1 and
            re.fullmatch('[a-f0-9]{64}', value['certificateSHA256'] or ''), 'Invalid certificate pin schema.')
    return value['certificateSHA256']

def available_identities():
    keychain = os.environ.get('ODDS_SIGNING_KEYCHAIN', '')
    args = [keychain] if keychain else []
    output = run('security', 'find-identity', '-p', 'codesigning', *args, capture=True)
    return {h.upper(): name for h, name in re.findall(r'\)\s+([0-9A-Fa-f]{40})\s+"([^"]+)"', output)}

def certificate(identity):
    """Find the exact identity's public certificate; validate EKU and expiration."""
    keychain = os.environ.get('ODDS_SIGNING_KEYCHAIN', '')
    pem = run('security', 'find-certificate', '-a', '-p', *([keychain] if keychain else []), capture=True)
    for block in re.findall(r'-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----', pem, re.S):
        der = base64.b64decode(''.join(block.splitlines()[1:-1]))
        if hashlib.sha1(der).hexdigest().upper() != identity:
            continue
        with tempfile.TemporaryDirectory(prefix='odds-public-cert-') as tmp:
            path = Path(tmp) / 'certificate.pem'
            path.write_text(block)
            run('openssl', 'x509', '-in', path, '-noout', '-checkend', '0', capture=True)
            description = run('openssl', 'x509', '-in', path, '-noout', '-text', capture=True)
            require('Code Signing' in description, 'Certificate has no Code Signing extended key usage.')
            dates = run('openssl', 'x509', '-in', path, '-noout', '-dates', capture=True)
        return hashlib.sha256(der).hexdigest(), dates
    raise BuildError('Signing identity has no matching public certificate.')

def signing(channel):
    if channel == 'preview':
        return '-', ''
    explicit = os.environ.get('ODDS_SIGNING_IDENTITY', '').strip().upper()
    require(not LOCAL.is_symlink(), 'Local signing configuration cannot be a symbolic link.')
    local = json.loads(LOCAL.read_text()) if LOCAL.is_file() else {}
    identity = explicit or (local.get('identity', '') if channel == 'dev' else '')
    require(re.fullmatch('[A-F0-9]{40}', identity or '') is not None,
            'A fixed signing identity is required. Run make signing-list, then make signing-config IDENTITY=<SHA1>. '
            'For release, set ODDS_SIGNING_IDENTITY. Explicit non-installable QA: make build CHANNEL=preview.')
    require(identity in available_identities(), 'Configured identity is unavailable; run make signing-list and reselect it.')
    digest, _ = certificate(identity)
    expected = read_pin(ROOT / 'config/macos-signing.json') if channel == 'release' else (digest if explicit else local.get('certificateSHA256'))
    require(digest == expected, 'Signing certificate differs from its stored SHA-256 pin.')
    return identity, digest

def update_config():
    data = json.loads((ROOT / 'config/updates.json').read_text())
    require(set(data) == {'publicKey', 'github', 'tagPrefix', 'deltas'}, 'Unexpected update configuration fields.')
    key = data['publicKey'].strip()
    try:
        decoded = base64.b64decode(key, validate=True)
    except ValueError as exc:
        raise BuildError('Invalid update public key.') from exc
    require(len(decoded) == 32, 'Commit the public key from mygo keygen to config/updates.json before a release build.')
    repo = os.environ.get('GITHUB_REPOSITORY', '') or data['github']
    require(re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo or '') is not None,
            'Set the verified public GitHub repository in config/updates.json (or GITHUB_REPOSITORY in CI).')
    require(not data['github'] or data['github'] == repo, 'CI repository differs from committed update repository.')
    require(data['tagPrefix'] == 'v' and data['deltas'] == 0, 'Only full updates with v tags are configured.')
    require(os.environ.get('MYGO_UPDATER_PRIVATE_KEY', '').strip(), 'MYGO_UPDATER_PRIVATE_KEY is required for release; unsigned updates are never published.')
    return dict(data, publicKey=key, github=repo)

def source_commit():
    try:
        commit = run('git', 'rev-parse', 'HEAD', capture=True)
    except BuildError:
        return 'unknown'
    dirty = run('git', 'status', '--porcelain', '--untracked-files=normal', capture=True)
    return commit + ('-dirty' if dirty else '')

def metadata(channel):
    name, bundle = identity_for(channel)
    return {'schema': 1, 'name': name, 'bundleID': bundle, 'version': version(), 'channel': channel,
            'executor': 'ci' if os.environ.get('GITHUB_ACTIONS') == 'true' else 'local',
            'commit': source_commit(), 'target': TARGET, 'source': 'source-build'}

def stable_requirement(output, bundle_id, identity):
    lines = [line.strip() for line in output.splitlines() if line.strip().startswith('designated => ')]
    if len(lines) != 1:
        return False
    expression = lines[0].removeprefix('designated => ')
    identifier = r'identifier (?:"' + re.escape(bundle_id) + r'"|' + re.escape(bundle_id) + r')'
    fixed = identifier + r' and certificate (?:leaf|root) = H"' + re.escape(identity) + r'"'
    if re.fullmatch(fixed, expression, re.I):
        return True
    # Apple's documented Developer ID requirement. Verify the exact conjunction
    # shape, and separately pin the leaf certificate bytes in verify_bundle.
    developer_id = (identifier + r' and anchor apple generic and certificate 1\[field\.1\.2\.840\.113635\.100\.6\.2\.6\] /\* exists \*/'
                    r' and certificate leaf\[field\.1\.2\.840\.113635\.100\.6\.1\.13\] /\* exists \*/'
                    r' and certificate leaf\[subject\.OU\] = "[A-Z0-9]{10}"')
    return re.fullmatch(developer_id, expression) is not None

def verify_bundle(app, expected, certificate_sha='', identity=''):
    require(app.is_dir() and not app.is_symlink(), f'Bundle missing: {app}')
    info = plistlib.loads((app / 'Contents/Info.plist').read_bytes())
    require(info.get('CFBundleIdentifier') == expected['bundleID'], 'Bundle identifier mismatch.')
    require(info.get('CFBundleShortVersionString') == expected['version'], 'Bundle version mismatch.')
    recorded = json.loads((app / 'Contents/Resources/build-info.json').read_text())
    require(recorded == expected, 'Build metadata mismatch.')
    exe = app / 'Contents/MacOS' / info['CFBundleExecutable']
    require(run('lipo', '-archs', exe, capture=True) == 'arm64', 'Unexpected executable architecture.')
    run('codesign', '--verify', '--deep', '--strict', app, capture=True)
    if certificate_sha:
        for target in [app, exe]:
            with tempfile.TemporaryDirectory(prefix='odds-verify-') as temp:
                prefix = str(Path(temp) / 'cert')
                run('codesign', '--display', '--extract-certificates=' + prefix, target, capture=True)
                require(hashlib.sha256(Path(prefix + '0').read_bytes()).hexdigest() == certificate_sha,
                        'Final code was signed by an unexpected certificate.')
            # Evaluate an explicit certificate + identifier requirement. This accepts
            # both Developer ID and local fixed identities, never an ad-hoc cdhash.
            requirement = f'identifier "{expected["bundleID"]}" and certificate leaf = H"{identity}"'
            run('codesign', '--verify', '--strict', '-R=' + requirement, target, capture=True)
            result = subprocess.run(['codesign', '--display', '--requirements', '-', str(target)], text=True, capture_output=True)
            require(result.returncode == 0 and stable_requirement(result.stdout + '\n' + result.stderr, expected['bundleID'], identity),
                    'Unexpected designated requirement; expected a fixed certificate identity or Developer ID conjunction.')

def release_names(ver):
    return [f'odds-{ver}-arm64.dmg', 'update-darwin-arm64.json']

def verify_release_artifacts(directory, meta, cert, identity):
    require(meta['executor'] == 'ci', 'Formal releases are built by the selected GitHub Actions workflow.')
    names = release_names(meta['version'])
    for name in names:
        file = directory / name
        require(file.is_file() and not file.is_symlink() and file.stat().st_size > 0,
                f'Missing or invalid release artifact: {name}')
    run('go', 'run', './source/cmd/verify-update', '--dir', directory, '--version', meta['version'],
        '--commit', meta['commit'], '--repo', 'knothhe/odds')
    with tempfile.TemporaryDirectory(prefix='odds-dmg-verify-') as temporary:
        mount = Path(temporary) / 'mount'
        run('hdiutil', 'attach', '-readonly', '-nobrowse', '-mountpoint', mount, directory / names[0], capture=True)
        try:
            verify_bundle(mount / 'odds.app', meta, cert, identity)
        finally:
            run('hdiutil', 'detach', mount, capture=True)
    checksum = ''.join(hashlib.sha256((directory / name).read_bytes()).hexdigest() + '  ' + name + '\n' for name in names)
    (directory / 'SHA256SUMS').write_text(checksum)

def build(channel, package=False):
    require(sys.platform == 'darwin', 'Build macOS bundles on macOS.')
    require(run('uname', '-m', capture=True) == 'arm64', 'This release currently supports verified macOS arm64 only.')
    run(sys.executable, ROOT / 'scripts/licenses.py', '--check')
    sign, cert = signing(channel)
    updates = update_config() if channel == 'release' else None
    meta = metadata(channel)
    if channel == 'release':
        require(meta['commit'] != 'unknown' and not meta['commit'].endswith('-dirty'), 'Release builds require a clean committed source tree.')
    no_symlink(ROOT / 'dist')
    (ROOT / 'dist').mkdir(exist_ok=True)
    cfg = json.loads((ROOT / 'mygo.json').read_text())
    cfg.update(name=meta['name'], identifier=meta['bundleID'], version=meta['version'], main=str(ROOT / 'source'))
    cfg['slug'] = 'odds' if channel == 'release' else 'odds-dev'
    if channel != 'release':
        icon_dir = ROOT / 'dist/icons-dev'
        run(ROOT / '.venv-build/bin/python3' if (ROOT / '.venv-build/bin/python3').exists() else sys.executable, ROOT / 'scripts/dev_icon.py', '--input', ROOT / 'assets/icon.png',
            '--output-dir', icon_dir, '--name', 'odds-dev', '--formats', 'png,icns', '--force')
        cfg['icon'], cfg['macos']['icon'] = str(icon_dir / 'odds-dev.png'), str(icon_dir / 'odds-dev.icns')
    else:
        cfg['icon'] = str(ROOT / 'assets/icon.png')
    cfg['macos']['signingIdentity'] = sign
    cfg['macos']['minimumSystemVersion'] = '13.0'
    cfg['macos']['infoPlist'] = {**cfg['macos'].get('infoPlist', {}), 'CFBundleDevelopmentRegion': 'en', 'CFBundleLocalizations': ['en', 'zh-Hans'],
                               'OddsChannel': channel, 'OddsCommit': meta['commit'], 'OddsTarget': TARGET}
    if updates:
        cfg['updates'] = updates
    else:
        cfg.pop('updates', None)
    if channel == 'release' and os.environ.get('ODDS_NOTARY_PROFILE'):
        cfg['macos']['notarize'] = {'keychainProfile': os.environ['ODDS_NOTARY_PROFILE']}
        if os.environ.get('ODDS_SIGNING_KEYCHAIN'):
            cfg['macos']['notarize']['keychain'] = os.environ['ODDS_SIGNING_KEYCHAIN']
    with tempfile.TemporaryDirectory(prefix='.build-', dir=ROOT / 'dist') as tmp:
        temporary = Path(tmp)
        cfg['out'] = str(temporary / 'output')
        record = temporary / 'build-info.json'
        record.write_text(json.dumps(meta, indent=2) + '\n')
        cfg['resources'] = [str(record), cfg['icon'], str(ROOT / 'source/internal/thirdparty/licenses.json')]
        if updates:
            notes = run(sys.executable, ROOT / 'scripts/release-notes.py', updates['github'], 'v' + meta['version'], capture=True)
            changelog = temporary / 'CHANGELOG.md'
            changelog.write_text('## ' + meta['version'] + '\n\n' + notes + '\n')
            cfg['updates']['changelog'] = str(changelog)
        (temporary / 'mygo.json').write_text(json.dumps(cfg, indent=2) + '\n')
        inject = {'Channel': channel, 'Executor': meta['executor'], 'Commit': meta['commit'],
                  'Repository': updates['github'] if updates else '', 'PublicKey': updates['publicKey'] if updates else ''}
        env = dict(os.environ)
        env['MYGO_LDFLAGS'] = ' '.join('-X github.com/knothhe/odds/source/internal/buildinfo.' + k + '=' + v for k, v in inject.items())
        # Debug builds avoid production-only features/timestamp requirements for local self-signed Dev certificates.
        args = ['go', 'tool', 'mygo', 'build', '-platform', 'darwin/arm64']
        if channel != 'release':
            args.append('-debug')
        if channel != 'release':
            args.append('-skip-dmg')
        try:
            with patched_environment(env) as build_env:
                run(*args, temporary, env=build_env)
        except PatchError as error:
            raise BuildError(str(error)) from error
        if channel == 'release':
            require(source_commit() == meta['commit'], 'Build changed the source checkout; refusing to publish those artifacts.')
        output = temporary / 'output' / TARGET
        app = output / (meta['name'] + '.app')
        verify_bundle(app, meta, cert, sign)
        if channel == 'release':
            (output / 'install.sh').unlink(missing_ok=True)
            verify_release_artifacts(output, meta, cert, sign)
        destination = ROOT / 'dist' / channel / TARGET
        destination.parent.mkdir(parents=True, exist_ok=True)
        no_symlink(destination)
        backup = destination.with_name(destination.name + '.previous')
        require(not backup.exists(), f'An earlier recovery backup exists: {backup}')
        if destination.exists():
            destination.rename(backup)
        try:
            output.rename(destination)
        except BaseException:
            if backup.exists():
                backup.rename(destination)
            raise
        if backup.exists():
            shutil.rmtree(backup)
    print(f'Built {channel}: {destination / (meta["name"] + ".app")}')
    if channel == 'preview':
        print('Preview is ad-hoc signed for local QA; installation and release updates are disabled.')
    return destination, meta, cert, sign

RECEIPT = ROOT / '.odds-install.json'
REGISTER = Path('/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister')

def checked_destination(destination):
    dest = no_symlink(Path(destination).expanduser())
    require(dest.name == 'odds Dev.app', 'Only the exact Dev bundle odds Dev.app may be managed here.')
    for name in ('odds', 'odds Dev'):
        data = (Path.home() / 'Library/Application Support' / name).resolve()
        require(dest != data and dest not in data.parents and data not in dest.parents,
                'Installation must not overlap application data.')
    return dest

def installed_identity(dest):
    require(dest.is_dir() and not dest.is_symlink(), 'Installed Dev bundle is missing or symlinked.')
    record = json.loads((dest / 'Contents/Resources/build-info.json').read_text())
    require(record.get('schema') == 1 and record.get('channel') == 'dev' and
            record.get('bundleID') == 'com.knothhe.odds.dev' and record.get('name') == 'odds Dev',
            'Refusing an unrelated bundle, release build, or installation without Dev identity.')
    return record

def target_processes(dest):
    info = plistlib.loads((dest / 'Contents/Info.plist').read_bytes())
    executable = str(dest / 'Contents/MacOS' / info['CFBundleExecutable'])
    lines = run('ps', '-axo', 'pid=,args=', capture=True).splitlines()
    pids = []
    for line in lines:
        fields = line.strip().split(None, 1)
        if len(fields) == 2 and (fields[1] == executable or fields[1].startswith(executable + ' ')):
            pids.append(int(fields[0]))
    return pids

def prepare_removal(dest, cleanup=True):
    # Build/staging validation occurs before asking the old app to quit.
    if target_processes(dest):
        run('swift', ROOT / 'scripts/stop-app.swift', dest)
        deadline = time.monotonic() + 15
        while target_processes(dest):
            require(time.monotonic() < deadline,
                    'The exact target is still running; resolve unsaved settings and retry. No process was killed.')
            time.sleep(.1)
    if not cleanup:
        return
    info = plistlib.loads((dest / 'Contents/Info.plist').read_bytes())
    # The installed executable checks its own compiled Dev identity and actual
    # path. It runs once without acquiring another installation's instance lock.
    command = [str(dest / 'Contents/MacOS' / info['CFBundleExecutable']), '--cleanup-install', str(dest)]
    child = subprocess.Popen(command, cwd=ROOT)
    try:
        status = child.wait(timeout=15)
    except subprocess.TimeoutExpired:
        raise BuildError('Installed cleanup did not finish; the bundle was preserved. No process was killed.') from None
    require(status == 0, 'Installed Dev login-item cleanup failed; bundle and receipt preserved.')

def write_receipt(data):
    require(not RECEIPT.is_symlink(), 'Installation receipt cannot be symlinked.')
    with tempfile.NamedTemporaryFile(mode='w', dir=ROOT, prefix='.odds-receipt-', delete=False) as f:
        temporary = Path(f.name)
        json.dump(data, f, indent=2)
        f.write('\n')
    try:
        temporary.chmod(0o600)
        temporary.replace(RECEIPT)
    finally:
        temporary.unlink(missing_ok=True)

def save_receipt(dest, meta, cert, identity, prepared=False):
    write_receipt({'schema':1,'destination':str(dest),'build':meta,'certificateSHA256':cert,
                   'identity':identity,'prepared':prepared,'unregistered':False,'removed':False})

def mark_receipt(receipt, **state):
    updated = dict(receipt, **state)
    write_receipt(updated)
    return updated

def has_registration(dest):
    result = json.loads(run('swift', ROOT / 'scripts/registered-app.swift', 'com.knothhe.odds.dev', dest, capture=True))
    require(isinstance(result, dict) and set(result) == {'registered'} and type(result['registered']) is bool,
            'LaunchServices registration query returned an invalid result; receipt retained.')
    return result['registered']

def install_recovery(temp, backup, dest):
    commands = []
    if backup.exists():
        if dest.exists():
            commands.append(f'mv {shlex.quote(str(dest))} {shlex.quote(str(temp / "failed.app"))}')
        commands.append(f'mv {shlex.quote(str(backup))} {shlex.quote(str(dest))}')
    if dest.exists() or backup.exists():
        commands.append(f'{shlex.quote(str(REGISTER))} -f {shlex.quote(str(dest))}')
    return '\n'.join(commands)

def install(destination):
    dest = checked_destination(destination)
    if RECEIPT.exists():
        require(not RECEIPT.is_symlink() and json.loads(RECEIPT.read_text()).get('destination') == str(dest),
                'This checkout owns another installation; uninstall that exact target before changing paths.')
    old = installed_identity(dest) if dest.exists() else None
    old_inode = dest.stat().st_ino if old else None
    output, meta, cert, identity = build('dev')
    source = output / 'odds Dev.app'
    require(dest != source and source not in dest.parents and dest not in source.parents,
            'Installation and build output must be separate.')
    dest.parent.mkdir(parents=True, exist_ok=True)
    no_symlink(dest.parent)
    # A TemporaryDirectory would erase the only old copy when rollback fails.
    temp = Path(tempfile.mkdtemp(prefix='.odds-install-', dir=dest.parent))
    backup = temp / 'previous.app'
    installed = False
    completed = False
    keep_recovery = False
    try:
        stage = temp / dest.name
        run('ditto', source, stage)
        verify_bundle(stage, meta, cert, identity)
        checked_destination(dest)
        require(dest.exists() == (old is not None), 'Installation target changed during build; existing files were preserved.')
        if old is not None:
            require(dest.stat().st_ino == old_inode and installed_identity(dest) == old, 'Installation target was replaced during build; retry after reviewing it.')
        if dest.exists():
            prepare_removal(dest, cleanup=False)
        try:
            if dest.exists():
                dest.rename(backup)
            stage.rename(dest)
            installed = True
            run(REGISTER, '-f', dest)
            verify_bundle(dest, meta, cert, identity)
            save_receipt(dest, meta, cert, identity)
            completed = True
        except BaseException as failure:
            try:
                if installed and dest.exists():
                    shutil.rmtree(dest)
                if backup.exists():
                    backup.rename(dest)
                    run(REGISTER, '-f', dest)
            except BaseException as rollback:
                keep_recovery = True
                raise BuildError(f'Installation failed: {failure}\nRollback failed: {rollback}\n'
                                 f'Recovery files retained at: {temp}\n'
                                 f'After resolving the filesystem or registration error, restore with:\n'
                                 f'{install_recovery(temp, backup, dest)}') from rollback
            raise
    finally:
        if not keep_recovery and (completed or not backup.exists()):
            try:
                shutil.rmtree(temp)
            except OSError as cleanup:
                raise BuildError(f'Installation staging cleanup failed; files retained at: {temp}\n{cleanup}') from cleanup
        elif not keep_recovery:
            # Also protect a backup if an interruption escaped rollback itself.
            print(f'Recovery files retained at: {temp}\n'
                  f'After resolving the interruption, restore with:\n'
                  f'{install_recovery(temp, backup, dest)}', file=sys.stderr)
    print(f'Installed Dev source build: {dest}\nData retained: ~/Library/Application Support/odds Dev')

def uninstall(destination):
    dest = checked_destination(destination)
    require(not RECEIPT.is_symlink(), 'Installation receipt cannot be symlinked.')
    if not RECEIPT.exists():
        require(not dest.exists(), 'No receipt for this installation. Verify its Dev identity and run make install at this path first.')
        print('Dev application is not installed; user data is retained.')
        return
    receipt = json.loads(RECEIPT.read_text())
    require(receipt.get('schema') == 1 and receipt.get('destination') == str(dest) and
            receipt.get('build',{}).get('channel') == 'dev', 'Receipt does not own this exact Dev installation.')
    if dest.exists():
        require(receipt.get('removed') is not True, 'A new target appeared after removal; receipt retained for review.')
        original = dest.stat()
        previous = receipt.get('removalIdentity')
        if previous is not None:
            require(receipt.get('prepared') is True and receipt.get('unregistered') is True and dest.is_dir() and
                    previous == {'device': original.st_dev, 'inode': original.st_ino},
                    'The partially removed target was replaced; receipt retained for review.')
        else:
            meta = installed_identity(dest)
            require(meta == receipt['build'], 'Installed build differs from its receipt; reinstall to establish ownership before uninstalling.')
            verify_bundle(dest, meta, receipt['certificateSHA256'], receipt['identity'])
            prepare_removal(dest)
            # The intact app may have been relaunched since a previous attempt.
            receipt = mark_receipt(receipt, prepared=True, unregistered=False)
        if receipt.get('unregistered') is not True:
            run(REGISTER, '-u', dest)
            receipt = mark_receipt(receipt, unregistered=True)
        current = dest.stat()
        require((current.st_dev, current.st_ino) == (original.st_dev, original.st_ino),
                'The target changed during cleanup; receipt retained for review.')
        receipt = mark_receipt(receipt, removalIdentity={'device': current.st_dev, 'inode': current.st_ino})
        shutil.rmtree(dest)
    else:
        require(receipt.get('prepared') is True, 'Application is missing but login-item cleanup was not confirmed; receipt retained.')
        if receipt.get('unregistered') is not True:
            require(not has_registration(dest),
                    'The missing application still has a LaunchServices registration; receipt retained.\n'
                    'Restore this exact Dev installation, then retry cleanup:\n'
                    f'make install INSTALL_DIR={shlex.quote(str(dest))}\n'
                    f'make uninstall INSTALL_DIR={shlex.quote(str(dest))}')
            receipt = mark_receipt(receipt, unregistered=True)
    mark_receipt(receipt, removed=True)
    RECEIPT.unlink()
    print(f'Uninstalled Dev application: {dest}\nUser data and credentials retained.')

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['build', 'package', 'install', 'uninstall', 'signing-list', 'signing-config', 'verify-tag'])
    p.add_argument('--channel', choices=['dev', 'preview', 'release'], default=os.environ.get('CHANNEL', 'dev'))
    p.add_argument('--identity')
    p.add_argument('--destination', default=str(Path.home() / 'Applications/odds Dev.app'))
    p.add_argument('--tag')
    args = p.parse_args()
    if args.action == 'signing-list':
        identities = available_identities()
        if not identities:
            print('No paired Code Signing identity found. See docs/build-release.md for Keychain Access setup.')
        for identity, label in identities.items():
            digest, dates = certificate(identity)
            print(f'{identity}  {label}\n  SHA256 {digest}\n  {dates}')
    elif args.action == 'signing-config':
        identity = (args.identity or '').upper()
        require(identity in available_identities(), 'Choose an exact SHA1 from make signing-list; no identity is chosen automatically.')
        digest, _ = certificate(identity)
        require(not LOCAL.is_symlink(), 'Local signing configuration cannot be a symbolic link.')
        LOCAL.write_text(json.dumps({'identity': identity, 'certificateSHA256': digest}, indent=2) + '\n')
        LOCAL.chmod(0o600)
        print('Saved this repository’s local identity reference. No private key was exported.')
    elif args.action == 'verify-tag':
        require(args.tag == 'v' + version(), 'Tag must exactly match v + mygo.json version.')
        require(run('git', 'cat-file', '-t', 'refs/tags/' + args.tag, capture=True) == 'tag', 'Release tags must be annotated.')
        require(run('git', 'rev-parse', 'HEAD', capture=True) == run('git', 'rev-parse', '--verify', f'refs/tags/{args.tag}^{{commit}}', capture=True), 'Checkout does not match requested tag.')
        require(not run('git', 'status', '--porcelain', capture=True), 'Tag checkout must be clean.')
    elif args.action == 'uninstall':
        require(args.channel == 'dev', 'Only the Dev installation is managed by make uninstall.')
        uninstall(Path(args.destination).expanduser())
    elif args.action == 'install':
        require(args.channel == 'dev', 'make install only installs an isolated Dev source build; use the verified Release package for production.')
        install(Path(args.destination).expanduser())
    else:
        build(args.channel, args.action == 'package')

if __name__ == '__main__':
    try:
        main()
    except (BuildError, ReleaseError, OSError, ValueError, KeyError) as error:
        print(f'ERROR: {error}', file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
"""Publish a complete, verified GitHub release. Re-runs never overwrite same-name assets."""
import hashlib
import json
import os
import re
import subprocess
import sys
import urllib.request
import tempfile

sys.dont_write_bytecode = True
from macos import ROOT, BuildError, require, run, version


def gh(*args):
    return run('gh', *args, capture=True)


def exact_release(repo, tag):
    # Successful pagination distinguishes absence from permission/network failure.
    pages = json.loads(gh('api', '--paginate', '--slurp', f'repos/{repo}/releases?per_page=100'))
    matches = [release for page in pages for release in page if release['tag_name'] == tag]
    require(len(matches) <= 1, 'More than one release matches the tag.')
    return matches[0] if matches else None


def verify_remote_assets(repo, release, expected):
    pages = json.loads(gh('api', '--paginate', '--slurp', f'repos/{repo}/releases/{release["id"]}/assets?per_page=100'))
    actual = {asset['name']: asset for page in pages for asset in page}
    require(set(actual) == set(expected), 'Remote release asset set differs from the release artifact contract.')
    for name, path in expected.items():
        require(name in actual, f'Remote asset missing: {name}')
        asset = actual[name]
        require(asset['size'] == path.stat().st_size, f'Remote asset size mismatch: {name}')
        # Read authenticated bytes for draft verification; do not trust matching names.
        result = subprocess.run(['gh','api','-H','Accept: application/octet-stream',f'repos/{repo}/releases/assets/{asset["id"]}'], capture_output=True)
        require(result.returncode == 0, f'Cannot verify remote asset: {name}')
        require(hashlib.sha256(result.stdout).digest() == hashlib.sha256(path.read_bytes()).digest(), f'Remote asset content mismatch: {name}; no automatic replacement.')
    return actual


def verify_checksums(directory, names):
    entries = (directory / 'SHA256SUMS').read_text().splitlines()
    expected = {}
    for entry in entries:
        digest, separator, name = entry.partition('  ')
        require(separator and re.fullmatch('[a-f0-9]{64}', digest) and name in names and name not in expected,
                'Invalid or duplicated release checksum entry.')
        expected[name] = digest
    require(set(expected) == set(names), 'Checksum file does not match the complete artifact set.')
    for name, digest in expected.items():
        path = directory / name
        require(path.is_file() and not path.is_symlink() and hashlib.sha256(path.read_bytes()).hexdigest() == digest,
                f'Release artifact checksum mismatch: {name}')

def main():
    require(os.environ.get('GITHUB_ACTIONS') == 'true', 'Publishing is owned by the tag workflow.')
    repo = os.environ.get('GITHUB_REPOSITORY', '')
    require(re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+',repo) is not None, 'Invalid repository.')
    tag = os.environ.get('RELEASE_TAG', '')
    ver = version()
    require(tag == 'v' + ver, 'Release tag mismatch.')
    run(sys.executable, ROOT / 'scripts/macos.py', 'verify-tag', '--tag', tag)
    commit = run('git','rev-parse','HEAD',capture=True)
    directory = ROOT / 'dist/release/darwin-arm64'
    run('go','run','./source/cmd/verify-update','--dir',directory,'--version',ver,'--repo',repo,'--commit',commit,'--signature-only')
    names = [f'odds-{ver}-arm64.dmg', 'update-darwin-arm64.json', 'SHA256SUMS']
    expected = {name:directory/name for name in names}
    verify_checksums(directory, names[:-1])
    require(all(path.is_file() and not path.is_symlink() and path.stat().st_size for path in expected.values()), 'Incomplete release artifacts.')
    release = exact_release(repo,tag)
    if release is None:
        # Keep gh release create --verify-tag's remote-tag precondition; the
        # release API would otherwise create a missing tag automatically.
        gh('api',f'repos/{repo}/git/ref/tags/{tag}')
        notes = run(sys.executable,ROOT/'scripts/release-notes.py',repo,tag,capture=True)
        # Use the mutation response: the release list can still reflect the state
        # before creation or publication because GitHub allows it to be cached.
        payload = {'tag_name':tag, 'target_commitish':commit, 'name':tag, 'body':notes,
                   'draft':True, 'prerelease':'-' in ver.split('+')[0]}
        result = subprocess.run(['gh','api','--method','POST',f'repos/{repo}/releases','--input','-'],
                                input=json.dumps(payload),text=True,capture_output=True)
        require(result.returncode == 0, 'Could not create the release draft.')
        release = json.loads(result.stdout)
        require(release['tag_name'] == tag and release['draft'], 'Release creation returned an unexpected release.')
    assets = {asset['name']:asset for asset in release['assets']}
    # Public releases are immutable here; avoid exposing a partially repaired feed.
    if not release['draft']:
        verify_remote_assets(repo,release,expected)
        print(f'Already published and verified: {release["html_url"]}')
        return
    for name,path in expected.items():
        if name not in assets:
            gh('release','upload',tag,str(path),'--repo',repo)
    verify_remote_assets(repo,release,expected)
    public = json.loads(gh('api','--method','PATCH',f'repos/{repo}/releases/{release["id"]}',
                          '-F','draft=false'))
    require(public['id'] == release['id'] and public['tag_name'] == tag and not public['draft'],
            'Release remains unpublished or returned an unexpected release.')
    verify_remote_assets(repo,public,expected)
    for name,path in expected.items():
        url = f'https://github.com/{repo}/releases/download/{tag}/{name}'
        with urllib.request.urlopen(url,timeout=60) as response:
            digest=hashlib.sha256(response.read()).digest()
        require(digest == hashlib.sha256(path.read_bytes()).digest(), f'Public download verification failed: {name}')
    print(f'Published and verified: {public["html_url"]}')

if __name__ == '__main__':
    try:
        main()
    except (BuildError,OSError,ValueError,KeyError) as error:
        print(f'ERROR: {error}',file=sys.stderr)
        sys.exit(1)

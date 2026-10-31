#!/usr/bin/env python3
"""Generate/sync odds cask from one public stable Release; never build or publish."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import urllib.request
from urllib.parse import quote

sys.dont_write_bytecode = True
from release import ReleaseError, parse, precedence

REPO = 'knothhe/odds'
TAP = 'knothhe/homebrew-tap'
ROOT = Path(__file__).resolve().parents[1]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def run(*args, cwd=None):
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True, stdin=subprocess.DEVNULL)
    require(result.returncode == 0, f'{args[0]} failed: {result.stderr.strip()}')
    return result.stdout.strip()


def download(url, limit):
    request = urllib.request.Request(url, headers={'User-Agent':'odds-homebrew'})
    with urllib.request.urlopen(request, timeout=60) as response:
        require(response.url.startswith('https://'), 'Download redirect must remain HTTPS.')
        data = response.read(limit+1)
    require(0 < len(data) <= limit, 'Empty or oversized release asset.')
    return data


def candidate(release, tag, fetch=download):
    version = tag.removeprefix('v')
    _, pre = parse(version)
    require(tag == 'v'+version and pre is None, 'Cask requires an exact stable vSemVer tag.')
    require(release['tag_name'] == tag and release['draft'] is False and release['prerelease'] is False,
            'Release must be public, stable and match the requested tag.')
    name = f'odds-{version}-arm64.dmg'
    assets = {}
    for filename in (name, 'SHA256SUMS'):
        matches = [asset for asset in release['assets'] if asset['name'] == filename]
        require(len(matches) == 1, 'Missing or duplicate release asset: '+filename)
        asset = matches[0]
        expected = f'https://github.com/{REPO}/releases/download/{tag}/{filename}'
        require(asset['state'] == 'uploaded' and type(asset['size']) is int and asset['size'] > 0 and
                asset['browser_download_url'] == expected, 'Untrusted or incomplete release asset: '+filename)
        limit = (1 << 30) if filename == name else (1 << 20)
        require(asset['size'] <= limit, 'Release asset exceeds size limit.')
        data = fetch(expected, asset['size'])
        require(len(data) == asset['size'], 'Release asset size mismatch.')
        digest = hashlib.sha256(data).hexdigest()
        if asset.get('digest') is not None:
            require(asset['digest'] == 'sha256:'+digest, 'GitHub asset digest mismatch.')
        assets[filename] = (data, digest)
    sums = {}
    for line in assets['SHA256SUMS'][0].decode().splitlines():
        digest, separator, filename = line.partition('  ')
        require(separator and re.fullmatch('[0-9a-f]{64}',digest) and filename not in sums,
                'Invalid or duplicate checksum entry.')
        sums[filename] = digest
    require(sums.get(name) == assets[name][1], 'DMG checksum does not match the public release.')
    digest = assets[name][1]
    return f'''cask "odds" do
  version "{version}"
  sha256 "{digest}"

  url "https://github.com/{REPO}/releases/download/v#{{version}}/odds-#{{version}}-arm64.dmg"
  name "odds"
  desc "Small menu bar utilities for macOS"
  homepage "https://github.com/{REPO}"

  livecheck do
    url :url
    strategy :github_latest
  end

  auto_updates true
  depends_on arch: :arm64
  depends_on macos: :ventura

  app "odds.app"
  uninstall quit: "com.knothhe.odds"
end
'''


def replace_cask(path, text):
    require(not path.is_symlink() and not path.parent.is_symlink(), 'Refusing symlinked cask path.')
    if path.exists():
        old = path.read_text()
        if old == text:
            return False
        old_version = re.search(r'^  version "([^"\n]+)"$', old, re.M)
        new_version = re.search(r'^  version "([^"\n]+)"$', text, re.M)
        require(old_version and new_version, 'Cannot determine existing cask version.')
        require(precedence(new_version[1]) > precedence(old_version[1]),
                'Refusing same-version content changes or a cask downgrade.')
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode='w', dir=path.parent, prefix='.odds-', delete=False) as file:
            temporary = Path(file.name)
            file.write(text)
        temporary.chmod(0o644)
        os.replace(temporary,path)
    finally:
        if temporary:
            temporary.unlink(missing_ok=True)
    return True


def sync(tap, text, push=False):
    require(tap.is_dir() and not tap.is_symlink(), 'Pass an existing clean tap checkout.')
    require(Path(run('git','rev-parse','--show-toplevel',cwd=tap)).resolve() == tap.resolve(), 'Expected tap repository root.')
    origin = run('git','remote','get-url','origin',cwd=tap)
    require(origin in (f'https://github.com/{TAP}',f'https://github.com/{TAP}.git',f'git@github.com:{TAP}.git'), 'Unexpected tap origin.')
    require(run('git','branch','--show-current',cwd=tap) == 'main', 'Tap must be on main.')
    require(not run('git','status','--porcelain',cwd=tap), 'Tap checkout must be clean.')
    run('git','fetch','origin','main',cwd=tap)
    require(not run('git','rev-list','origin/main..HEAD',cwd=tap), 'Tap has unrelated unpushed commits.')
    run('git','merge','--ff-only','origin/main',cwd=tap)
    changed = replace_cask(tap / 'Casks/odds.rb',text)
    if push and changed:
        run('git','add','--','Casks/odds.rb',cwd=tap)
        run('git','-c','user.name=github-actions[bot]','-c','user.email=41898282+github-actions[bot]@users.noreply.github.com',
            'commit','-m','Update odds cask', '--','Casks/odds.rb',cwd=tap)
        run('git','push','origin','HEAD:main',cwd=tap) # normal failure on concurrent updates; never force
    return changed


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tag',required=True)
    parser.add_argument('--tap',default='')
    parser.add_argument('--push',action='store_true',help='Explicitly commit/push the verified cask in the tap checkout')
    args=parser.parse_args()
    require(not args.push or args.tap, '--push requires --tap.')
    # Validate tag before constructing a URL or making a request.
    require(args.tag.startswith('v'), 'Expected a vSemVer tag.')
    _,pre=parse(args.tag[1:]);require(pre is None,'Prereleases do not enter the stable tap.')
    release=json.loads(run('gh','api',f'repos/{REPO}/releases/tags/{quote(args.tag,safe="")}'))
    text=candidate(release,args.tag)
    if args.tap:
        changed=sync(Path(args.tap).absolute(),text,args.push)
        print('Cask updated.' if changed else 'Cask already matches; no commit needed.')
    else:
        output=ROOT/'dist/homebrew/Casks/odds.rb'
        replace_cask(output,text)
        print(f'Generated {output}; tap unchanged.')


if __name__ == '__main__':
    try:
        main()
    except (OSError,ValueError,KeyError,ReleaseError) as error:
        print(f'ERROR: {error}',file=sys.stderr)
        sys.exit(1)

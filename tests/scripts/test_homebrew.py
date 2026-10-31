import copy
import hashlib
from pathlib import Path
import subprocess
import shutil
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'scripts'))
import homebrew


def fixture(version='1.2.3'):
    tag='v'+version
    name=f'odds-{version}-arm64.dmg'
    data={name:b'fixture dmg'}
    data['SHA256SUMS']=(hashlib.sha256(data[name]).hexdigest()+'  '+name+'\n').encode()
    release={'tag_name':tag,'draft':False,'prerelease':False,'assets':[
        {'name':key,'state':'uploaded','size':len(value),
         'browser_download_url':f'https://github.com/knothhe/odds/releases/download/{tag}/{key}'} for key,value in data.items()]}
    return release,lambda url,limit:data[url.rsplit('/',1)[1]]


class HomebrewTests(unittest.TestCase):
    def test_public_candidate_has_exact_platform_hash_and_update_contract(self):
        release,fetch=fixture()
        text=homebrew.candidate(release,'v1.2.3',fetch)
        self.assertIn(hashlib.sha256(b'fixture dmg').hexdigest(),text)
        self.assertIn('depends_on arch: :arm64',text)
        self.assertIn('depends_on macos: :ventura',text)
        self.assertIn('auto_updates true',text)
        self.assertIn('uninstall quit: "com.knothhe.odds"',text)
        self.assertNotIn('zap',text)
        with tempfile.TemporaryDirectory() as temporary:
            file=Path(temporary)/'odds.rb';file.write_text(text)
            if shutil.which('ruby'):
                subprocess.run(['ruby','-c',str(file)],check=True,capture_output=True)

    def test_invalid_release_or_asset_is_rejected_before_replacing_cask(self):
        release,fetch=fixture()
        cases=[]
        for field,value in [('draft',True),('prerelease',True),('tag_name','v9.9.9')]:
            candidate=copy.deepcopy(release);candidate[field]=value;cases.append(candidate)
        candidate=copy.deepcopy(release);candidate['assets']=[];cases.append(candidate)
        candidate=copy.deepcopy(release);candidate['assets'].append(candidate['assets'][0]);cases.append(candidate)
        for field,value in [('state','new'),('size',0),('size',999),('browser_download_url','https://example.com/app.dmg'),('digest','sha256:'+'0'*64)]:
            candidate=copy.deepcopy(release);candidate['assets'][0][field]=value;cases.append(candidate)
        for candidate in cases:
            with self.subTest(candidate=candidate),self.assertRaises(ValueError):
                homebrew.candidate(candidate,'v1.2.3',fetch)
        with self.assertRaises(ValueError):
            homebrew.candidate(release,'v1.2.3',lambda url,limit:b'x'*limit)
        release,fetch=fixture('1.2.3-rc.1')
        with self.assertRaises(ValueError):homebrew.candidate(release,'v1.2.3-rc.1',fetch)

    def test_replace_is_idempotent_and_refuses_downgrades_or_same_version_changes(self):
        release,fetch=fixture()
        text=homebrew.candidate(release,'v1.2.3',fetch)
        with tempfile.TemporaryDirectory() as temporary:
            file=Path(temporary)/'Casks/odds.rb'
            self.assertTrue(homebrew.replace_cask(file,text))
            self.assertFalse(homebrew.replace_cask(file,text))
            for changed in [text.replace('1.2.3','1.2.2'),text.replace('fixture','other')+'\n']:
                with self.assertRaises(ValueError):homebrew.replace_cask(file,changed)
                self.assertEqual(file.read_text(),text)
            self.assertEqual(list(file.parent.glob('.odds-*')),[])

    def test_sync_only_commits_its_cask_and_rejects_dirty_or_unpushed_work(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);remote=root/'remote.git';tap=root/'tap'
            def git(*args,cwd=None):
                return subprocess.check_output(['git',*args],cwd=cwd,text=True,stderr=subprocess.DEVNULL).strip()
            git('init','--bare','--initial-branch=main',str(remote))
            git('clone',str(remote),str(tap))
            git('config','user.email','fixture@example.com',cwd=tap);git('config','user.name','fixture',cwd=tap)
            (tap/'README.md').write_text('unchanged')
            git('add','README.md',cwd=tap);git('commit','-m','Initial',cwd=tap);git('push','origin','main',cwd=tap)
            original=homebrew.run
            def local(*args,**kwargs):
                if args==('git','remote','get-url','origin'):return 'git@github.com:knothhe/homebrew-tap.git'
                return original(*args,**kwargs)
            release,fetch=fixture();text=homebrew.candidate(release,'v1.2.3',fetch)
            with patch.object(homebrew,'run',side_effect=local):
                self.assertTrue(homebrew.sync(tap,text,push=True))
                self.assertEqual(git('show','--pretty=','--name-only','HEAD',cwd=tap),'Casks/odds.rb')
                head=git('rev-parse','HEAD',cwd=tap)
                self.assertFalse(homebrew.sync(tap,text,push=True))
                self.assertEqual(git('rev-parse','HEAD',cwd=tap),head)
                (tap/'unrelated').write_text('keep')
                with self.assertRaises(ValueError):homebrew.sync(tap,text,push=True)
                git('add','unrelated',cwd=tap);git('commit','-m','Unrelated',cwd=tap)
                with self.assertRaises(ValueError):homebrew.sync(tap,text,push=True)
                self.assertEqual(git('rev-parse','origin/main',cwd=tap),head)
                # A remote writer racing the final push must win normally;
                # sync must not force-push over its unrelated change.
                git('reset','--hard','origin/main',cwd=tap) # private fixture only
                competitor=root/'competitor'
                git('clone',str(remote),str(competitor))
                git('config','user.email','fixture@example.com',cwd=competitor)
                git('config','user.name','competitor',cwd=competitor)
                (competitor/'concurrent.txt').write_text('keep concurrent change')
                git('add','concurrent.txt',cwd=competitor);git('commit','-m','Concurrent',cwd=competitor)
                def racing(*args,**kwargs):
                    if args==('git','push','origin','HEAD:main'):
                        git('push','origin','main',cwd=competitor)
                    return local(*args,**kwargs)
                with patch.object(homebrew,'run',side_effect=racing),self.assertRaises(ValueError):
                    homebrew.sync(tap,text.replace('1.2.3','1.2.4'),push=True)
                self.assertEqual(git('--git-dir',str(remote),'show','main:concurrent.txt'),'keep concurrent change')

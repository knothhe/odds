"""Exercise the adapted version source and release engine in disposable Git repos."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[2] / 'scripts'
sys.path.insert(0,str(SCRIPTS))
import release

class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='odds-release-test-')
        self.root = Path(self.temporary.name)
        self.cwd = Path.cwd()
        os.chdir(self.root)
        self.git('init','-b','main')
        self.git('config','user.name','Release Test')
        self.git('config','user.email','test@example.invalid')
        self.git('config','commit.gpgsign','false')
        self.git('config','tag.gpgSign','false')
        self.git('config','core.hooksPath',os.devnull)
        (self.root/'mygo.json').write_text(json.dumps({'name':'odds','version':'0.1.0','macos':{'infoPlist':{'LSUIElement':True}}},indent=2)+'\n')
        (self.root/'.release.json').write_text(json.dumps({'version_file':'mygo.json','branch':'main','remote':'origin','release_executor':'github-actions','github_release':False,'checks':[],'build':[],'assets':[]}))
        self.git('add','mygo.json','.release.json')
        self.git('commit','-m','feat: add app')
    def tearDown(self):
        os.chdir(self.cwd)
        self.temporary.cleanup()
    def git(self,*args):
        return subprocess.check_output(['git',*args],stderr=subprocess.DEVNULL,text=True).strip()
    def run_release(self,*args):
        return subprocess.run([sys.executable,str(SCRIPTS/'release.py'),*args],text=True,capture_output=True)
    def test_upgrade_updates_only_mygo_version(self):
        result=self.run_release('patch','--apply','--no-push')
        self.assertEqual(result.returncode,0,result.stderr)
        data=json.loads(Path('mygo.json').read_text())
        self.assertEqual(data['version'],'0.1.1')
        self.assertTrue(data['macos']['infoPlist']['LSUIElement'])
        self.assertEqual(self.git('cat-file','-t','v0.1.1'),'tag')
        self.assertEqual(self.git('status','--porcelain'),'')
    def test_initial_preserves_version_and_head(self):
        before=self.git('rev-parse','HEAD')
        result=self.run_release('initial','--apply','--no-push')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(self.git('rev-parse','HEAD'),before)
        self.assertEqual(json.loads(Path('mygo.json').read_text())['version'],'0.1.0')
        self.assertIn('Release-Type: initial',self.git('cat-file','-p','v0.1.0'))
    def test_preview_does_not_build_or_write(self):
        before=Path('mygo.json').read_bytes()
        result=self.run_release('patch','--no-push')
        self.assertEqual(result.returncode,0,result.stderr)
        plan=json.loads(result.stdout)
        self.assertEqual(plan['release_executor'],'github-actions')
        self.assertEqual(plan['build'],[])
        self.assertEqual(Path('mygo.json').read_bytes(),before)
        self.assertEqual(self.git('tag','--list'),'')
    def test_failed_check_cannot_bump_or_tag(self):
        cfg=json.loads(Path('.release.json').read_text())
        cfg['checks']=[[sys.executable,'-c','raise SystemExit(2)']]
        Path('.release.json').write_text(json.dumps(cfg))
        self.git('add','.release.json');self.git('commit','-m','test: fail check')
        before=self.git('rev-parse','HEAD')
        result=self.run_release('patch','--apply','--no-push')
        self.assertNotEqual(result.returncode,0)
        self.assertEqual(self.git('rev-parse','HEAD'),before)
        self.assertEqual(json.loads(Path('mygo.json').read_text())['version'],'0.1.0')
        self.assertEqual(self.git('tag','--list'),'')
    def test_resume_keeps_initial_tag_and_version(self):
        first=self.run_release('initial','--apply','--no-push')
        self.assertEqual(first.returncode,0,first.stderr)
        tag=self.git('rev-parse','v0.1.0')
        resumed=self.run_release('0.1.0','--resume','--apply','--no-push')
        self.assertEqual(resumed.returncode,0,resumed.stderr)
        self.assertEqual(self.git('rev-parse','v0.1.0'),tag)
    def test_rejects_invalid_or_downgrade_version(self):
        for value in ('0.0.9','0.1.0','01.2.3'):
            result=self.run_release(value,'--apply','--no-push')
            self.assertNotEqual(result.returncode,0)
        self.assertEqual(self.git('status','--porcelain'),'')

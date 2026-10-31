import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'scripts'))
spec=importlib.util.spec_from_file_location('odds_publish',ROOT/'scripts/publish-release.py')
publish=importlib.util.module_from_spec(spec);spec.loader.exec_module(publish)

class PublishTests(unittest.TestCase):
    def test_checksum_contract_rejects_tampering_and_missing_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary)
            names=['app.dmg','archive.tar.gz','update.json']
            for name in names:(root/name).write_bytes(name.encode())
            (root/'SHA256SUMS').write_text(''.join(hashlib.sha256((root/name).read_bytes()).hexdigest()+'  '+name+'\n' for name in names))
            publish.verify_checksums(root,names)
            (root/'app.dmg').write_bytes(b'tampered')
            with self.assertRaises(publish.BuildError):publish.verify_checksums(root,names)
    def test_existing_asset_must_match_bytes_not_just_name_and_size(self):
        with tempfile.TemporaryDirectory() as temporary:
            file=Path(temporary)/'app.dmg';file.write_bytes(b'original')
            payload=[[{'id':7,'name':'app.dmg','size':8}]]
            response=subprocess.CompletedProcess([],0,stdout=b'modified')
            with patch.object(publish,'gh',return_value=json.dumps(payload)),patch.object(publish.subprocess,'run',return_value=response):
                with self.assertRaises(publish.BuildError):publish.verify_remote_assets('knothhe/odds',{'id':1},{'app.dmg':file})

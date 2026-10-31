import importlib.util
import json
import os
from pathlib import Path
import tempfile
import tarfile
import io
import shutil
import hashlib
import re
import shlex
import subprocess
import unittest
from unittest.mock import patch
import sys

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'scripts'))
import macos
import signing_keychain
import project

class InstallTests(unittest.TestCase):
    def setUp(self):
        self.temporary=tempfile.TemporaryDirectory(prefix='odds-install-test-')
        self.root=Path(self.temporary.name).resolve()
        self.dest=self.root/'odds Dev.app'
        self.receipt=self.root/'receipt.json'
        self.patch=patch.object(macos,'RECEIPT',self.receipt)
        self.patch.start()
    def tearDown(self):
        self.patch.stop();self.temporary.cleanup()
    def bundle(self):
        path=self.dest/'Contents/Resources'
        path.mkdir(parents=True)
        record={'schema':1,'channel':'dev','bundleID':'com.knothhe.odds.dev','name':'odds Dev','commit':'test'}
        (path/'build-info.json').write_text(json.dumps(record))
        return record
    def test_symlink_or_release_destination_is_refused(self):
        unrelated=self.root/'unrelated';unrelated.mkdir()
        self.dest.symlink_to(unrelated,target_is_directory=True)
        with self.assertRaises(macos.BuildError):macos.uninstall(self.dest)
        self.assertTrue(unrelated.is_dir())
        with self.assertRaises(macos.BuildError):macos.checked_destination(self.root/'odds.app')
    def test_missing_receipt_cannot_delete_an_existing_bundle(self):
        self.bundle()
        with self.assertRaises(macos.BuildError):macos.uninstall(self.dest)
        self.assertTrue(self.dest.exists())
    def test_wrong_receipt_target_is_refused(self):
        record=self.bundle()
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.root/'other.app'),'build':record}))
        with self.assertRaises(macos.BuildError):macos.uninstall(self.dest)
        self.assertTrue(self.dest.exists())
    def test_cleanup_failure_preserves_bundle_and_receipt(self):
        record=self.bundle()
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':record,'certificateSHA256':'a'*64,'identity':'A'*40}))
        before=self.receipt.read_bytes()
        with patch.object(macos,'verify_bundle'),patch.object(macos,'prepare_removal',side_effect=macos.BuildError('login removal failed')):
            with self.assertRaises(macos.BuildError):macos.uninstall(self.dest)
        self.assertTrue(self.dest.exists());self.assertEqual(self.receipt.read_bytes(),before)
    def test_uninstall_retains_data_and_is_repeatable(self):
        record=self.bundle()
        data=self.root/'config.json';data.write_text('user preferences')
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':record,'certificateSHA256':'a'*64,'identity':'A'*40}))
        with patch.object(macos,'ROOT',self.root),patch.object(macos,'verify_bundle'),patch.object(macos,'prepare_removal'),patch.object(macos,'run'):
            macos.uninstall(self.dest)
            macos.uninstall(self.dest)
        self.assertFalse(self.dest.exists());self.assertFalse(self.receipt.exists())
        self.assertEqual(data.read_text(),'user preferences')
    def test_missing_bundle_without_confirmed_cleanup_keeps_receipt(self):
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':{'channel':'dev'},'prepared':False}))
        with self.assertRaises(macos.BuildError):macos.uninstall(self.dest)
        self.assertTrue(self.receipt.exists())
    def test_failed_install_rollback_retains_old_bundle_outside_automatic_cleanup(self):
        record=self.bundle()
        (self.dest/'marker').write_text('old application')
        output=self.root/'build';source=output/'odds Dev.app'
        source.mkdir(parents=True)
        (source/'marker').write_text('new application')
        original_rename=Path.rename
        def rename(path,target):
            if path.name=='previous.app':
                raise OSError('restore is blocked')
            return original_rename(path,target)
        def command(*args,**kwargs):
            if args[0]=='ditto':
                shutil.copytree(args[1],args[2]);return ''
            if args[0]==macos.REGISTER:
                raise macos.BuildError('registration failed')
            raise AssertionError(args)
        with patch.object(macos,'ROOT',self.root),patch.object(macos,'build',return_value=(output,record,'a'*64,'A'*40)),patch.object(macos,'verify_bundle'),patch.object(macos,'prepare_removal'),patch.object(macos,'run',side_effect=command),patch.object(Path,'rename',rename):
            with self.assertRaises((macos.BuildError,OSError)):
                macos.install(self.dest)
        backups=list(self.root.glob('.odds-install-*/previous.app/marker'))
        self.assertEqual(len(backups),1,'rollback failure deleted the only old application backup')
        self.assertEqual(backups[0].read_text(),'old application')
    def test_missing_bundle_with_completed_unregister_can_finish_without_rescanning_it(self):
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':{'channel':'dev'},'prepared':True,'unregistered':True}))
        with patch.object(macos,'run',side_effect=macos.BuildError('cannot scan a missing bundle')) as command:
            macos.uninstall(self.dest)
        command.assert_not_called()
        self.assertFalse(self.receipt.exists())
    def test_interrupt_after_moving_old_bundle_restores_it(self):
        record=self.bundle()
        (self.dest/'marker').write_text('old application')
        output=self.root/'build';source=output/'odds Dev.app'
        source.mkdir(parents=True)
        (source/'marker').write_text('new application')
        rename=Path.rename
        def interrupt(path,target):
            result=rename(path,target)
            if path==self.dest:
                raise KeyboardInterrupt('interrupted after the old bundle moved')
            return result
        def command(*args,**kwargs):
            if args[0]=='ditto':
                shutil.copytree(args[1],args[2]);return ''
            if args[0]==macos.REGISTER:
                return ''
            raise AssertionError(args)
        with patch.object(macos,'ROOT',self.root),patch.object(macos,'build',return_value=(output,record,'a'*64,'A'*40)),patch.object(macos,'verify_bundle'),patch.object(macos,'prepare_removal'),patch.object(macos,'run',side_effect=command),patch.object(Path,'rename',interrupt):
            with self.assertRaises(KeyboardInterrupt):
                macos.install(self.dest)
        self.assertTrue((self.dest/'marker').exists(),'interrupt deleted the moved old application')
        self.assertEqual((self.dest/'marker').read_text(),'old application')
        self.assertFalse(list(self.root.glob('.odds-install-*')))
    def test_successful_install_and_rollback_clean_staging(self):
        record=self.bundle()
        (self.dest/'marker').write_text('old application')
        output=self.root/'build';source=output/'odds Dev.app'
        source.mkdir(parents=True)
        (source/'marker').write_text('new application')
        registrations=[]
        def command(*args,**kwargs):
            if args[0]=='ditto':
                shutil.copytree(args[1],args[2]);return ''
            if args[0]==macos.REGISTER:
                registrations.append(args)
                if len(registrations)==1:
                    raise macos.BuildError('first registration failed')
                return ''
            raise AssertionError(args)
        with patch.object(macos,'ROOT',self.root),patch.object(macos,'build',return_value=(output,record,'a'*64,'A'*40)),patch.object(macos,'verify_bundle'),patch.object(macos,'prepare_removal'),patch.object(macos,'run',side_effect=command):
            with self.assertRaisesRegex(macos.BuildError,'first registration failed'):
                macos.install(self.dest)
            self.assertEqual((self.dest/'marker').read_text(),'old application')
            self.assertFalse(list(self.root.glob('.odds-install-*')))
            macos.install(self.dest)
        self.assertEqual((self.dest/'marker').read_text(),'new application')
        self.assertFalse(list(self.root.glob('.odds-install-*')))
    def test_partial_bundle_deletion_retries_only_the_original_directory(self):
        record=self.bundle()
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':record,'certificateSHA256':'a'*64,'identity':'A'*40}))
        remove=shutil.rmtree
        def interrupt(path):
            if path==self.dest:
                (self.dest/'Contents/Resources/build-info.json').unlink()
                raise OSError('deletion interrupted')
            return remove(path)
        with patch.object(macos,'ROOT',self.root),patch.object(macos,'verify_bundle') as verify,patch.object(macos,'prepare_removal') as prepare,patch.object(macos,'run') as command:
            with patch.object(macos.shutil,'rmtree',side_effect=interrupt):
                with self.assertRaisesRegex(OSError,'deletion interrupted'):
                    macos.uninstall(self.dest)
            pending=json.loads(self.receipt.read_text())
            self.assertTrue(pending['prepared']);self.assertTrue(pending['unregistered'])
            self.assertFalse(pending.get('removed',False))
            self.assertEqual(pending['removalIdentity']['inode'],self.dest.stat().st_ino)
            macos.uninstall(self.dest)
        verify.assert_called_once();prepare.assert_called_once()
        command.assert_called_once_with(macos.REGISTER,'-u',self.dest)
        self.assertFalse(self.dest.exists());self.assertFalse(self.receipt.exists())
    def test_partial_deletion_cannot_remove_a_replacement_directory(self):
        record=self.bundle()
        original=self.dest.stat()
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':record,'prepared':True,'unregistered':True,'removalIdentity':{'device':original.st_dev,'inode':original.st_ino}}))
        self.dest.rename(self.root/'old.app')
        self.bundle()
        with self.assertRaisesRegex(macos.BuildError,'partially removed target was replaced'):
            macos.uninstall(self.dest)
        self.assertTrue(self.dest.exists());self.assertTrue(self.receipt.exists())
    def test_receipt_unlink_failure_can_finish_after_app_was_removed(self):
        record=self.bundle()
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':record,'certificateSHA256':'a'*64,'identity':'A'*40}))
        unlink=Path.unlink
        def deny_receipt(path,*args,**kwargs):
            if path==self.receipt:
                raise OSError('receipt is locked')
            return unlink(path,*args,**kwargs)
        with patch.object(macos,'ROOT',self.root),patch.object(macos,'verify_bundle'),patch.object(macos,'prepare_removal'),patch.object(macos,'run') as command:
            with patch.object(Path,'unlink',deny_receipt):
                with self.assertRaisesRegex(OSError,'receipt is locked'):
                    macos.uninstall(self.dest)
            self.assertFalse(self.dest.exists())
            self.assertTrue(json.loads(self.receipt.read_text())['removed'])
            command.reset_mock()
            macos.uninstall(self.dest)
            command.assert_not_called()
        self.assertFalse(self.receipt.exists())
    def test_legacy_prepared_receipt_finishes_when_exact_registration_is_absent(self):
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':{'channel':'dev'},'prepared':True}))
        with patch.object(macos,'ROOT',self.root),patch.object(macos,'has_registration',return_value=False) as query,patch.object(macos,'run') as command:
            macos.uninstall(self.dest)
        query.assert_called_once_with(self.dest);command.assert_not_called()
        self.assertFalse(self.receipt.exists())
    def test_legacy_stale_registration_retains_receipt_and_prints_recovery(self):
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':{'channel':'dev'},'prepared':True}))
        before=self.receipt.read_bytes()
        with patch.object(macos,'has_registration',return_value=True),patch.object(macos,'run') as command:
            with self.assertRaisesRegex(macos.BuildError,'make install INSTALL_DIR=') as raised:
                macos.uninstall(self.dest)
        self.assertIn('make uninstall INSTALL_DIR=',str(raised.exception))
        self.assertEqual(self.receipt.read_bytes(),before);command.assert_not_called()
    def test_registration_query_failure_does_not_complete_legacy_cleanup(self):
        self.receipt.write_text(json.dumps({'schema':1,'destination':str(self.dest),'build':{'channel':'dev'},'prepared':True}))
        before=self.receipt.read_bytes()
        with patch.object(macos,'has_registration',side_effect=macos.BuildError('query failed')):
            with self.assertRaisesRegex(macos.BuildError,'query failed'):
                macos.uninstall(self.dest)
        self.assertEqual(self.receipt.read_bytes(),before)
    def test_registration_query_requires_a_boolean_result_for_the_exact_target(self):
        with patch.object(macos,'run',return_value='{"registered":false}') as command:
            self.assertFalse(macos.has_registration(self.dest))
        command.assert_called_once_with('swift',macos.ROOT/'scripts/registered-app.swift','com.knothhe.odds.dev',self.dest,capture=True)
        with patch.object(macos,'run',return_value='{"registered":0}'):
            with self.assertRaises(macos.BuildError):macos.has_registration(self.dest)

class SigningTests(unittest.TestCase):
    @unittest.skipUnless(sys.platform=='darwin','The selected-identity exporter uses macOS Security.framework.')
    def test_native_export_from_two_fixture_identities_contains_only_the_selected_key(self):
        def command(*args,input=None):
            result=subprocess.run([str(arg) for arg in args],input=input,capture_output=True)
            if result.returncode:
                raise AssertionError(f'Fixture command {Path(str(args[0])).name} failed ({result.returncode}); output omitted.')
            return result.stdout
        original=shlex.split(command('security','list-keychains','-d','user').decode())
        with tempfile.TemporaryDirectory(prefix='odds-fixture-signing-') as temporary:
            directory=Path(temporary)
            keychain=directory/'fixture.keychain-db'
            created=False
            try:
                command('security','create-keychain','-p','fixture-keychain-password',keychain)
                created=True
                command('security','unlock-keychain','-p','fixture-keychain-password',keychain)
                identities=[]
                for index in (1,2):
                    private,certificate,archive=[directory/f'fixture-{index}.{suffix}' for suffix in ('key','pem','p12')]
                    command('openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',private,'-out',certificate,
                            '-days','2','-subj',f'/CN=odds isolated fixture {index}')
                    digest=hashlib.sha1(command('openssl','x509','-in',certificate,'-outform','DER')).hexdigest().upper()
                    identities.append(digest)
                    command('openssl','pkcs12','-export','-inkey',private,'-in',certificate,'-out',archive,
                            '-passout','pass:fixture-p12-password','-keypbe','PBE-SHA1-3DES','-certpbe','PBE-SHA1-3DES','-macalg','sha1')
                    command('security','import',archive,'-k',keychain,'-f','pkcs12','-P','fixture-p12-password','-A')
                exported,password=signing_keychain.export_identity(identities[0],str(keychain),command)
                archive=directory/'selected.p12';archive.write_bytes(exported)
                parse_args=('openssl','pkcs12','-in',archive,'-nodes','-passin','stdin')
                try:
                    contents=command(*parse_args,input=password.encode()+b'\n')
                except AssertionError:
                    if not signing_keychain.supports_legacy_provider(command('openssl','version')):raise
                    contents=command(*parse_args,'-legacy',input=password.encode()+b'\n')
                self.assertEqual(len(re.findall(rb'-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----',contents)),1)
                certificates=re.findall(rb'-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----',contents,re.S)
                digests=[hashlib.sha1(command('openssl','x509','-outform','DER',input=certificate)).hexdigest().upper() for certificate in certificates]
                self.assertIn(identities[0],digests)
                self.assertNotIn(identities[1],digests)
            finally:
                try:
                    command('security','list-keychains','-d','user','-s',*original)
                finally:
                    if created or keychain.exists():command('security','delete-keychain',keychain)

    def test_identity_export_never_exports_the_other_keychain_identities(self):
        calls=[]
        identity='A'*40
        def command(*args,input=None):
            calls.append(args)
            if args[0]=='security' and args[1]=='export':
                raise AssertionError('exported every identity in the keychain')
            self.assertEqual(args[:2],('swift',macos.ROOT/'scripts/export-identity.swift'))
            self.assertEqual(args[2:4],('fixture.keychain-db',identity))
            self.assertTrue(input)
            Path(args[4]).write_bytes(b'only selected encrypted P12 fixture')
            return json.dumps({'exported':True,'certificateSHA1':identity}).encode()
        data,password=signing_keychain.export_identity(identity,'fixture.keychain-db',command)
        self.assertEqual(data,b'only selected encrypted P12 fixture')
        self.assertTrue(password)
        self.assertEqual(len(calls),1)
        self.assertFalse(Path(calls[0][4]).exists())

    def test_ci_cleanup_continues_after_keychain_restore_spawn_failure(self):
        spec=importlib.util.spec_from_file_location('odds_ci_cleanup_failure',ROOT/'scripts/ci-sign.py')
        ci=importlib.util.module_from_spec(spec);spec.loader.exec_module(ci)
        with tempfile.TemporaryDirectory() as temporary:
            calls=[]
            def command(*args):
                calls.append(args)
                if args[:2]==('security','list-keychains'):
                    if len(args)==4:return '"fixture-login.keychain-db"'
                    if args[5:]==('fixture-login.keychain-db',):raise OSError('restore could not spawn')
                if args[:2]==('security','create-keychain'):
                    Path(args[-1]).write_bytes(b'fixture private keychain')
                if args[:2]==('security','find-identity'):
                    return '1) '+'A'*40+' "Fixture Code Signing"'
                return ''
            env={'GITHUB_ACTIONS':'true','RUNNER_TEMP':temporary,'MACOS_SIGNING_CERTIFICATE':'Zml4dHVyZQ==',
                 'MACOS_SIGNING_CERTIFICATE_PASSWORD':'fixture','MYGO_UPDATER_PRIVATE_KEY':'fixture',
                 'APPLE_ID':'','APPLE_TEAM_ID':'','APPLE_APP_PASSWORD':''}
            with patch.dict(os.environ,env),patch.object(ci,'read_pin'),patch.object(ci,'secret_run',side_effect=command),patch.object(ci.subprocess,'run',return_value=subprocess.CompletedProcess([],0)):
                with self.assertRaisesRegex((macos.BuildError,OSError),'restore'):
                    ci.main()
            self.assertTrue(any(call[:2]==('security','delete-keychain') for call in calls),'restore failure skipped private keychain deletion')
            self.assertEqual(list(Path(temporary).iterdir()),[],'restore failure retained private CI files')

    def test_keygen_rejects_parent_symlink_into_the_repository_before_reading_keys(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()
            source=root/'source';source.mkdir()
            home=root/'home';(home/'Library/Application Support').mkdir(parents=True)
            (home/'Library/Application Support/odds-signing').symlink_to(source,target_is_directory=True)
            with patch.object(project,'ROOT',source),patch.object(Path,'home',return_value=home),patch.object(project.subprocess,'run') as command:
                with self.assertRaises(macos.BuildError):project.keygen()
                command.assert_not_called()

    def test_release_extraction_accepts_canonical_temp_alias_but_not_archive_links(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()
            actual=root/'actual';actual.mkdir()
            alias=root/'alias';alias.symlink_to(actual,target_is_directory=True)
            archive=root/'app.tar.gz'
            with tarfile.open(archive,'w:gz') as writer:
                entry=tarfile.TarInfo('odds.app/Contents/Info.plist')
                entry.size=4
                writer.addfile(entry,io.BytesIO(b'test'))
            macos.extract_release_archive(archive,alias)
            self.assertEqual((actual/'odds.app/Contents/Info.plist').read_bytes(),b'test')
            with tarfile.open(archive,'w:gz') as writer:
                entry=tarfile.TarInfo('odds.app/escape');entry.type=tarfile.SYMTYPE;entry.linkname='/tmp/outside'
                writer.addfile(entry)
            with self.assertRaises(macos.BuildError):macos.extract_release_archive(archive,alias)
    def test_requirement_cannot_bypass_the_pinned_certificate(self):
        expected='designated => identifier "com.knothhe.odds.dev" and certificate leaf = H"'+'A'*40+'"'
        self.assertTrue(macos.stable_requirement(expected,'com.knothhe.odds.dev','A'*40))
        self.assertFalse(macos.stable_requirement(expected+' or true','com.knothhe.odds.dev','A'*40))
        self.assertFalse(macos.stable_requirement('designated => cdhash H"1234"','com.knothhe.odds.dev','A'*40))
    def test_pin_rejects_unknown_fields(self):
        with tempfile.TemporaryDirectory() as temporary:
            p=Path(temporary)/'pin.json';p.write_text(json.dumps({'schema':1,'certificateSHA256':'a'*64,'extra':True}))
            with self.assertRaises(macos.BuildError):macos.read_pin(p)
    def test_ci_bad_secret_still_restores_keychain_state(self):
        spec=importlib.util.spec_from_file_location('odds_ci_sign',ROOT/'scripts/ci-sign.py')
        ci=importlib.util.module_from_spec(spec);spec.loader.exec_module(ci)
        with tempfile.TemporaryDirectory() as temporary:
            env={'GITHUB_ACTIONS':'true','RUNNER_TEMP':temporary,'MACOS_SIGNING_CERTIFICATE':'not-base64!',
                 'MACOS_SIGNING_CERTIFICATE_PASSWORD':'fixture','MYGO_UPDATER_PRIVATE_KEY':'fixture'}
            with patch.dict(os.environ,env),patch.object(ci,'read_pin'),patch.object(ci,'secret_run',return_value='"login.keychain-db"') as commands:
                with self.assertRaises(ValueError):ci.main()
                self.assertTrue(any(call.args[:5]==('security','list-keychains','-d','user','-s') for call in commands.call_args_list))
            self.assertEqual(list(Path(temporary).iterdir()),[])

#!/usr/bin/env python3
"""Run isolated native UI verification; never install or modify login items."""
import base64
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]


def main():
    if sys.platform != 'darwin' or os.uname().machine != 'arm64':
        raise RuntimeError('Native verification requires macOS arm64 with a GUI session.')
    output = ROOT / 'dist/verification'
    output.mkdir(parents=True, exist_ok=True)
    (output/'native-result.json').unlink(missing_ok=True)
    (output/'native-failure.json').unlink(missing_ok=True)
    with tempfile.TemporaryDirectory(prefix='odds-qa-', dir='/tmp') as temporary:
        directory = Path(temporary)
        executable, address = directory / 'odds-qa', directory / 'ui.sock'
        token = uuid.uuid4().hex[:12]
        subprocess.run([sys.executable, str(ROOT / 'scripts/go-patched.py'), 'build', '-tags', 'e2e',
                        '-ldflags', '-X github.com/knothhe/odds/source/internal/app.previewInstance=' + token,
                        '-o', str(executable), '.'], cwd=ROOT / 'source', check=True)
        env = dict(os.environ, MYGO_AUTOMATION=str(address), TMPDIR=str(directory))
        # Exclude user overrides that could replace the private socket or UI locale.
        env.pop('MYGO_LOCALE', None)
        with (output / 'native.log').open('w') as log:
            child = subprocess.Popen([str(executable), '--preview'], env=env, stdout=log, stderr=log)
            stream = None
            def request(method, **params):
                stream.write((json.dumps({'id': 1, 'method': method, 'params': params})+'\n').encode())
                stream.flush()
                data = json.loads(stream.readline())
                if 'error' in data:
                    raise RuntimeError(str(data['error']))
                return data['result']
            def wait(check, description):
                deadline = time.monotonic()+15
                while time.monotonic()<deadline:
                    if child.poll() is not None:
                        raise RuntimeError('Preview exited; see dist/verification/native.log')
                    result = check()
                    if result:
                        return result
                    time.sleep(.05)
                if stream:
                    (output/'native-failure.json').write_text(json.dumps({'step':description,'windows':request('windows'), 'tree':request('tree')},ensure_ascii=False,indent=2)+'\n')
                raise RuntimeError('Timed out: '+description)
            def nodes():
                return request('tree')['nodes']
            def act(label, action='press'):
                matches = [n for n in nodes() if n.get('label') == label and action in n.get('actions', [])]
                if len(matches) != 1:
                    raise RuntimeError('Missing or ambiguous native control: '+label)
                return request('act', id=matches[0]['id'], action=action)
            try:
                wait(lambda: address.exists(), 'automation socket')
                with socket.socket(socket.AF_UNIX) as connection:
                    connection.settimeout(20)
                    connection.connect(str(address))
                    stream = connection.makefile('rwb')
                    wait(lambda: request('windows')['windows'], 'settings window')
                    wait(lambda: any(n.get('label') == '复制版本' for n in nodes()), 'About controls')
                    request('idle', timeout=5000)
                    screenshot = request('capture')
                    (output / 'native-general.png').write_bytes(base64.b64decode(screenshot['png']))
                    act('依赖与许可证')
                    wait(lambda: any('github.com/egoist/mygo' in n.get('label','') for n in nodes()), 'offline dependencies')
                    # Framework role labels must follow the application's Chinese locale.
                    request('menu', path=['编辑','全选'])
                    request('input', events=[{'type':'key','key':'Cmd+W'}])
                    wait(lambda: all(not w['visible'] for w in request('windows')['windows']), 'close hides settings')
                    second_env=dict(env)
                    second_env.pop('MYGO_AUTOMATION',None)
                    subprocess.run([str(executable),'--preview'], env=second_env, check=True, timeout=15)
                    wait(lambda: any(w['visible'] and w['focused'] for w in request('windows')['windows']), 'second instance restores focus')
                    act('鼠标居中')
                    wait(lambda: any(n.get('label') == '移到当前屏幕中央的快捷键' for n in nodes()), 'shortcut field')
                    act('移到当前屏幕中央的快捷键')
                    wait(lambda: any(n.get('label') == '正在录制快捷键' for n in nodes()), 'recording status')
                    request('idle', timeout=5000)
                    request('input', events=[{'type':'key','key':'Escape'}])
                    wait(lambda: all(n.get('label') != '正在录制快捷键' for n in nodes()), 'Escape cancels recording')
                    request('menu', path=['odds Dev Preview · '+token, '退出 odds Dev'])
                    child.wait(timeout=10)
                    if child.returncode != 0:
                        raise RuntimeError('Preview did not exit normally.')
                    (output / 'native-result.json').write_text(json.dumps({
                        'os': subprocess.check_output(['sw_vers','-productVersion'],text=True).strip(),
                        'arch': os.uname().machine,
                        'passed': ['native-about','offline-dependencies','framework-menu-language','close-hides',
                                   'second-instance-focus','shortcut-recording-cancel','normal-quit'],
                        'notCovered': ['system-hotkeys','login-items','display-removal','minimum-macos-13','published-update']
                    }, ensure_ascii=False, indent=2)+'\n')
                    print('Native verification passed; evidence: dist/verification/')
            finally:
                if stream:
                    stream.close()
                if child.poll() is None:
                    # This is solely the private QA child, never an installed app.
                    child.terminate()
                    try:
                        child.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        child.kill(); child.wait()


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(f'ERROR: {error}', file=sys.stderr)
        sys.exit(1)

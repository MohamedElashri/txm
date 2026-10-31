#!/usr/bin/env python3
"""Version/help/completion smoke only; Windows backend runtime is not certified."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile
import zipfile

from archive_checks import EXPECTED


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive',required=True,type=Path)
    parser.add_argument('--expected-version',required=True)
    parser.add_argument('--report',required=True,type=Path)
    args=parser.parse_args()
    if os.name!='nt':parser.error('requires an actual Windows runner')
    with tempfile.TemporaryDirectory(prefix='txm-windows-package-') as tmp, zipfile.ZipFile(args.archive) as payload:
        root=Path(tmp);expected=(EXPECTED-{'txm'})|{'txm.exe'}
        assert len(payload.namelist())==len(expected) and set(payload.namelist())==expected
        member=payload.getinfo('txm.exe');assert 0<member.file_size<=100<<20
        binary=root/'txm.exe';binary.write_bytes(payload.read(member));home=root/'home';home.mkdir()
        env={k:v for k,v in os.environ.items() if k.upper() in {'SYSTEMROOT','WINDIR','PATH','PATHEXT','COMSPEC'}}
        env.update(HOME=str(home),USERPROFILE=str(home),TEMP=tmp,TMP=tmp,XDG_STATE_HOME=str(home/'state'))
        def run(args,code=0):
            p=subprocess.run([str(binary),*args],cwd=tmp,env=env,capture_output=True,text=True,timeout=20)
            assert p.returncode==code,(args,p.returncode,p.stdout,p.stderr)
            return p.stdout
        assert run(['version']).strip()=='txm version '+args.expected_version
        assert 'Usage:' in run(['--help'])
        for shell in ['bash','zsh','fish','powershell']:assert run(['completion',shell]).strip()
        run(['--backend','native','agent','start','unsupported','--','cmd.exe'],4)
        run(['dashboard'],4)
        assert not (home/'state/txm').exists()
    result={'passed':True,'version':args.expected_version,'archive':str(args.archive),
            'sha256':hashlib.sha256(args.archive.read_bytes()).hexdigest(),'machine':platform.machine(),
            'scope':'Windows executable version/help/completions and explicit unsupported refusals only; no external-backend/install claim'}
    with args.report.open('x') as f:f.write(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result))


if __name__=='__main__':main()

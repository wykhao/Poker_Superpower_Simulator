"""Build explicit, data-free release archives. Python 3.9+, Go 1.25+."""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent.parent
PLATFORMS = ['windows-amd64', 'windows-arm64', 'darwin-amd64', 'darwin-arm64', 'linux-amd64', 'linux-arm64']
ROOT_FILES = ['go.mod','go.sum','README.md','README.en.md','LICENSE','THIRD_PARTY_NOTICES.md','CONTRIBUTING.md','SECURITY.md','CHANGELOG.md','.gitignore','.gitattributes','Start-Poker.vbs']
SOURCE_DIRS = ['cmd','internal','poker','protocol','sdk','scripts','tests','.github','licenses','vendor','docs']

def add_file(z, file, name, executable=False):
    entry = zipfile.ZipInfo(name.replace('\\', '/'))
    entry.compress_type = zipfile.ZIP_DEFLATED
    entry.external_attr = (0o100755 if executable else 0o100644) << 16
    z.writestr(entry, file.read_bytes())

def main():
    p = argparse.ArgumentParser()
    p.add_argument('--go', default='go')
    p.add_argument('--version', default='0.1.0')
    p.add_argument('--platform', action='append', choices=PLATFORMS)
    args = p.parse_args()
    if not re.fullmatch(r'\d+\.\d+\.\d+(?:[-.][A-Za-z0-9.]+)?', args.version):
        p.error('version must be a semantic version, without the v prefix')
    dist = ROOT / 'dist'
    dist.mkdir(exist_ok=True)
    go_version = subprocess.check_output([args.go, 'version'], text=True).strip()
    goroot = Path(subprocess.check_output([args.go, 'env', 'GOROOT'], text=True).strip())
    license_files = list((ROOT/'licenses').rglob('*'))
    dependency_licenses = [f for f in (ROOT/'vendor').rglob('*') if f.is_file() and f.name.upper().startswith(('LICENSE','COPYING','NOTICE'))]

    def build(target):
        goos, goarch = target.split('-')
        folder = dist / f'poker-human-simulator-{args.version}-{target}'
        folder.mkdir(exist_ok=True)
        executable = folder / ('poker-simulator.exe' if goos=='windows' else 'poker-simulator')
        env = dict(os.environ, GOOS=goos, GOARCH=goarch, CGO_ENABLED='0')
        ldflags = f'-s -w -X poker-human-simulator/internal/app.Version={args.version}'
        if goos=='windows': ldflags += ' -H=windowsgui'
        subprocess.run([args.go,'build','-trimpath','-buildvcs=false','-ldflags',ldflags,'-o',str(executable),'./cmd/poker-simulator'], cwd=ROOT, env=env, check=True)
        archive = dist / (folder.name + '.zip')
        # Whitelist contents: never include .local-data, .test-data, test results,
        # an existing history.json, credentials, or unrelated workspace files.
        with zipfile.ZipFile(archive,'w') as z:
            prefix = folder.name + '/'
            add_file(z, executable, prefix+executable.name, True)
            for name in ['README.md','README.en.md','LICENSE','THIRD_PARTY_NOTICES.md','CHANGELOG.md']:
                add_file(z, ROOT/name, prefix+name)
            if (ROOT/'docs'/'screenshot.png').is_file():
                add_file(z,ROOT/'docs'/'screenshot.png',prefix+'docs/screenshot.png')
            for f in license_files:
                if f.is_file(): add_file(z,f,prefix+f.relative_to(ROOT).as_posix())
            for f in dependency_licenses:
                add_file(z,f,prefix+'licenses/dependencies/'+f.relative_to(ROOT/'vendor').as_posix())
            add_file(z,goroot/'LICENSE',prefix+'licenses/Go-BSD.txt')
        return archive

    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
        archives = list(pool.map(build, args.platform or PLATFORMS))
    source = dist/f'poker-human-simulator-{args.version}-source.zip'
    prefix = f'poker-human-simulator-{args.version}/'
    with zipfile.ZipFile(source,'w') as z:
        for name in ROOT_FILES:
            if (ROOT/name).is_file(): add_file(z,ROOT/name,prefix+name)
        for name in SOURCE_DIRS:
            for f in sorted((ROOT/name).rglob('*')):
                if f.is_file() and '__pycache__' not in f.parts:
                    add_file(z,f,prefix+f.relative_to(ROOT).as_posix(),f.suffix=='.sh')
    archives.append(source)
    manifest = {'version':args.version,'go':go_version,'artifacts':[]}
    checksums = []
    for f in sorted(archives):
        digest=hashlib.sha256(f.read_bytes()).hexdigest()
        manifest['artifacts'].append({'file':f.name,'bytes':f.stat().st_size,'sha256':digest})
        checksums.append(f'{digest}  {f.name}')
        print(f'{f.name}: {f.stat().st_size:,} bytes', flush=True)
    (dist/'SHA256SUMS.txt').write_text('\n'.join(checksums)+'\n',encoding='utf-8')
    (dist/'release-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n',encoding='utf-8')

if __name__=='__main__': main()

import collections, hashlib, json, os, pathlib, platform, shutil, subprocess, sys
repo=pathlib.Path(__file__).resolve().parents[1];out=repo/'native-evidence';out.mkdir(exist_ok=True)
arch={'x86_64':'amd64','aarch64':'arm64'}[platform.machine()];assert platform.system()=='Linux' and arch==os.environ['EXPECTED_ARCH']
go=shutil.which('go');assert subprocess.check_output([go,'env','GOARCH'],text=True).strip()==arch
base=os.environ['STORAGE_BASE'];env=dict(os.environ,GOMAXPROCS='2',GOMEMLIMIT='2GiB',GOFLAGS='-p=2')
(out/'environment.json').write_text(json.dumps({'system':platform.system(),'machine':platform.machine(),'arch':arch,'kernel':platform.release(),'go':subprocess.check_output([go,'version'],text=True).strip(),'upstream_base':base,'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'execution':'native hosted Linux; no cross-arch runtime or QEMU command','graph_driver':'overlay','scope':'temporary synthetic stores; no existing host images or volumes'},indent=2)+'\n')
sudo=['sudo','env','PATH='+os.environ['PATH'],'GOMAXPROCS=2','GOMEMLIMIT=2GiB','GOFLAGS=-p=2']
summary=[]
def run(label,cwd,args,before=False):
 r=subprocess.run(sudo+[go,*args],cwd=cwd,env=env,capture_output=True,text=True)
 (out/(label+'.jsonl')).write_text(r.stdout);(out/(label+'.stderr')).write_text(r.stderr)
 rows=[]
 for line in r.stdout.splitlines():
  try:rows.append(json.loads(line))
  except ValueError:pass
 failures=[x.get('Test') for x in rows if x.get('Action')=='fail' and x.get('Test')];passed=[x.get('Test') for x in rows if x.get('Action')=='pass' and x.get('Test')];skips=[x.get('Test') for x in rows if x.get('Action')=='skip' and x.get('Test')]
 if before:
  required=['existing_parent','new_parent','nested_existing_parent','nested_new_parent','whiteout_changes_parent']
  assert r.returncode!=0 and all('TestImplicitDirectoryImportTimes/'+n in failures for n in required),(label,r.returncode,failures,r.stderr)
  assert 'TestImplicitDirectorySize' in failures and 'TestCheckImplicitOverlay' in failures
 else:assert r.returncode==0 and not failures and not skips,(label,r.returncode,failures,r.stderr)
 summary.append({'label':label,'exit_code':r.returncode,'passed':len(passed),'failed_tests_including_parents':len(failures),'skipped':len(skips),'expected_before_failure':before});(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
args=['test','-tags','exclude_graphdriver_btrfs','-json','-count=1','-race','-timeout','5m','-run','Test(Check|Implicit|Explicit|Root)','.']
current=repo/'storage';fixed=(current/'check.go').read_bytes();stock=subprocess.check_output(['git','show',base+':storage/check.go'],cwd=repo)
(current/'check.go').write_bytes(stock)
try:run('upstream-before',current,args,True)
finally:(current/'check.go').write_bytes(fixed)
run('upstream-after',current,args)
subprocess.run(sudo+[go,'vet','-tags','exclude_graphdriver_btrfs','.'],cwd=current,check=True)
subprocess.run(sudo+[go,'build','-tags','exclude_graphdriver_btrfs','-o',str(out/'containers-storage'),'./cmd/containers-storage'],cwd=current,check=True)
(out/'native-elf.txt').write_text(subprocess.check_output(['file',str(out/'containers-storage')],text=True))
# Verify the deployed dependency line separately from latest upstream.
download=json.loads(subprocess.check_output([go,'mod','download','-json','go.podman.io/storage@v1.62.0'],cwd='/tmp',env=dict(env,GOWORK='off'),text=True));old=repo/'compat-v162';shutil.copytree(download['Dir'],old);old.chmod(0o755)
for p in old.rglob('*'):
 if p.is_dir():p.chmod(0o755)
 elif p.is_file():p.chmod(0o644)
for name in ['check_implicit_directory_test.go','check_implicit_overlay_test.go']:shutil.copy2(current/name,old/name)
oldenv=env;env=dict(env,GOWORK='off');sudo+=['GOWORK=off']
run('v162-before',old,args,True)
(old/'check.go').write_bytes((repo/'validation/check-v162.go.txt').read_bytes())
run('v162-after',old,args)
subprocess.run(sudo+[go,'vet','-tags','exclude_graphdriver_btrfs','.'],cwd=old,check=True)
(out/'source-hashes.json').write_text(json.dumps({'upstream_fixed_check_go':hashlib.sha256(fixed).hexdigest(),'v162_fixed_check_go':hashlib.sha256((old/'check.go').read_bytes()).hexdigest(),'tests':{name:hashlib.sha256((current/name).read_bytes()).hexdigest() for name in ['check_implicit_directory_test.go','check_implicit_overlay_test.go']}},indent=2)+'\n')
print(json.dumps(summary,indent=2))

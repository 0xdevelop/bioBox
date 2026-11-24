from pathlib import Path
import os,re,subprocess,tempfile
repo=Path(__file__).resolve().parents[2]
with tempfile.TemporaryDirectory(dir=repo/"tmp",prefix="protected-test-") as d:
 root=Path(d); log=root/"calls"; log.write_text(""); bin=root/"bin";bin.mkdir()
 for name in ["git","gh"]:
  p=bin/name;p.write_text('#!/bin/bash\nprintf "%s\\n" "$*" >> "$CALL_LOG"\nexit 91\n');p.chmod(0o755)
 env={"PATH":str(bin)+os.pathsep+os.environ["PATH"],"CALL_LOG":str(log),"APP_OLD_VERSION":"v0.0.2","GITHUB_REPOSITORY":"0xdevelop/bioBox"}
 src=(repo/"scripts/git_tag.sh").read_text()
 local=re.search(r'    if \[\[ "v\$\{pre_del_version_no\}" == v0\.0\.2 \]\]; then.*?    fi',src,re.S).group(0)
 wf=(repo/".github/workflows/release.yml").read_text()
 remote=re.search(r'          if \[\[ "\$APP_OLD_VERSION" == v0\.0\.2 \]\]; then.*?          fi',wf,re.S).group(0)
 for block in [local,remote]:
  p=subprocess.run(["/bin/bash","-c",'pre_del_version_no=0.0.2; '+block],cwd=repo,env=env,capture_output=True,text=True)
  assert p.returncode==0 and not log.read_text(),(p.returncode,p.stderr,log.read_text())
 assert "candidate-" not in wf and "stage_candidate_assets" not in wf and "refs/heads/main" not in wf
 assert "branches:" not in wf and "path: tmp/arch-assets/*" in wf
 print("v0.0.2 local/remote protected guard PASS; template Release assets path restored")

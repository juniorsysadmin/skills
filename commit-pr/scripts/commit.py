#!/usr/bin/env python3
# commit-pr commit gate (see ../SKILL.md [COMMIT]): commit.py [--amend] <path>...
# Reads the message from the MSG path preflight.py printed, then in one process:
#   index guard  -> the staged paths must equal <path>... (root-relative); else FAIL index\t<staged paths>, exit 1
#   message gate -> check_msg rules; FAIL <rule>\t<line>\t<detail> lines, exit 1
#   OK, git commit -F <MSG> (git's own output follows); a refused commit prints FAIL commit\t<exit code>, exit 1
#   post check   -> the stored message again, for text a hook appended: POST OK, or POST FAIL … lines and exit 1
# Nothing is committed before OK. The MSG file is deleted once the commit exists.
# --amend rewrites the last commit's message; <path>... may then be empty (a message-only fix).
import os, pathlib, subprocess, sys

sys.dont_write_bytecode = True  # no __pycache__ beside the skill: its directory may be tracked or read-only
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import check_msg, scopes

args = sys.argv[1:]
amend = args[:1] == ["--amend"]
want = sorted({os.path.normpath(p) for p in args[amend:]})
if not want and not amend:
    sys.exit("usage: commit.py [--amend] <path>...")

root, _ = scopes.enter_root()
msg = scopes.scratch(root, "msg")
if not os.path.isfile(msg):
    print("FAIL msg\t" + msg + " is missing: write the message there first")
    sys.exit(1)

have = sorted(scopes.zlist("-c", "core.quotepath=off", "diff", "--cached", "--name-only", "-z"))
if want != have:
    print("FAIL index\t" + " ".join(have))
    sys.exit(1)

fails = check_msg.check(open(msg).read())
if fails:
    print("\n".join(fails))
    sys.exit(1)

print("OK", flush=True)
rc = subprocess.run(["git", "commit", "-F", msg] + (["--amend"] if amend else [])).returncode
if rc:
    print(f"FAIL commit\t{rc}")
    sys.exit(1)
os.remove(msg)

fails = check_msg.check(scopes.git("log", "-1", "--format=%B").stdout)
print("\n".join("POST " + f for f in fails) if fails else "POST OK")
sys.exit(1 if fails else 0)

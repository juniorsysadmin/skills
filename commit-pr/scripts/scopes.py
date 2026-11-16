#!/usr/bin/env python3
# commit-pr scope tool (see ../SKILL.md [SCOPES], ../SCOPES.md); runs from any cwd inside the repo.
# --gate          -> FRESH|STALE, then HINT line if the stamp is not gitignored
# --stamp         -> writes today's UTC date to .code-scopes-validated
# [paths...]      -> DEAD GLOB\t<glob> lines, then <scope>\t<path> per path (NO MATCH\t<path> on a miss)
import datetime, json, os, re, subprocess, sys

root = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()
os.chdir(root)
args = sys.argv[1:]
today = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d")

if args == ["--stamp"]:
    open(".code-scopes-validated", "w").write(today + "\n")
    print("STAMPED", today)
    sys.exit(0)

if args == ["--gate"]:
    try:
        fresh = json.load(open(".code-scopes")).get("version") == 1 and open(".code-scopes-validated").read().strip() == today
    except (OSError, ValueError):
        fresh = False
    print("FRESH" if fresh else "STALE")
    if subprocess.run(["git", "check-ignore", "-q", ".code-scopes-validated"]).returncode:
        print("HINT\tadd .code-scopes-validated to .gitignore")
    sys.exit(0)

doc = json.load(open(".code-scopes"))
files = subprocess.run(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], capture_output=True, text=True, check=True).stdout.split("\0")
files = [f for f in files if f]
cache = {}


def rx(glob):
    if glob not in cache:
        # "**/" = zero or more directories, "**" = anything, "*" = within one segment
        r = re.escape(glob).replace(r"\*\*/", "\1").replace(r"\*\*", "\0").replace(r"\*", "[^/]*").replace("\0", ".*").replace("\1", "(?:.*/)?")
        cache[glob] = re.compile(r)
    return cache[glob]


for glob in (g for s in doc["scopes"] for g in s["paths"]):
    if not any(rx(glob).fullmatch(f) for f in files):
        print("DEAD GLOB\t" + glob)

for path in args:
    hit = next((s["name"] for s in doc["scopes"] if any(rx(g).fullmatch(path) for g in s["paths"])), "NO MATCH")
    print(hit + "\t" + path)

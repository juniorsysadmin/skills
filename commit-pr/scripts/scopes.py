#!/usr/bin/env python3
# commit-pr scope tool (see ../SKILL.md [SCOPES], ../SCOPES.md); runs from any cwd inside the repo or a linked worktree.
# The repository is always the one the shell's cwd is in, never the one this file lives in.
# Outside a repository: prints ERROR\tnot a git repository, exits 2.
# --gate          -> FRESH|STALE, then LOCAL if .code-scopes exists untracked (copied from the main checkout when a
#                    linked worktree lacks it), HINT if it is missing, HINT if the old root stamp file is still there
# --stamp         -> writes today's UTC date and the sha256 of .code-scopes to <git-common-dir>/commit-pr/scopes-validated,
#                    one stamp for the main checkout and every linked worktree
# --normalise     -> checks the .code-scopes schema, rewrites it with indent=2; OK, or FAIL schema\t<detail> and exit 1
# --staged        -> as [paths...], over the staged paths
# --inventory     -> as [paths...], over every tracked or new, not ignored path
# [paths...]      -> DEAD GLOB\t<glob> lines, then <scope>\t<path> per path (NO MATCH\t<path> on a miss)
import datetime, hashlib, json, os, re, shutil, subprocess, sys, tempfile

OLD_STAMP = ".code-scopes-validated"
NAME = re.compile(r"[a-z0-9][a-z0-9._-]*")


def git(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True)


def zlist(*args):
    return [f for f in git(*args).stdout.split("\0") if f]


def enter_root():
    """chdir to the root of the work tree the shell is in; return (root, the cwd it left)."""
    top = git("rev-parse", "--show-toplevel")
    if top.returncode or not top.stdout.strip():
        print("ERROR\tnot a git repository")
        sys.exit(2)
    start = os.path.realpath(os.getcwd())
    os.chdir(top.stdout.strip())
    return os.path.realpath(os.getcwd()), start


def git_dir(kind):
    # kind: --git-dir (this work tree's own) or --git-common-dir (shared by every work tree); call after enter_root
    return os.path.realpath(git("rev-parse", kind).stdout.strip())


def main_checkout():
    out = git("worktree", "list", "--porcelain").stdout.split("\n")
    first = next((l[9:] for l in out if l.startswith("worktree ")), "")
    return os.path.realpath(first) if first else ""


def scratch(root, name):
    # Per-work-tree scratch file outside the repository and outside .git, so a file-write tool can reach it unprompted.
    d = os.path.join(tempfile.gettempdir(), f"commit-pr-{os.getuid()}", hashlib.sha256(root.encode()).hexdigest()[:16])
    os.makedirs(d, mode=0o700, exist_ok=True)
    return os.path.join(d, name)


def stamp_path():
    return os.path.join(git_dir("--git-common-dir"), "commit-pr", "scopes-validated")


def stamp_value():
    today = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d")
    return today + " " + hashlib.sha256(open(".code-scopes", "rb").read()).hexdigest()


def gate(root):
    lines = []
    copied = ""
    if not os.path.exists(".code-scopes"):
        main = main_checkout()
        src = os.path.join(main, ".code-scopes")
        if main and main != root and os.path.isfile(src) and git("-C", main, "ls-files", "--error-unmatch", ".code-scopes").returncode:
            shutil.copyfile(src, ".code-scopes")
            copied = main
    try:
        fresh = json.load(open(".code-scopes")).get("version") == 1 and open(stamp_path()).read().strip() == stamp_value()
    except (OSError, ValueError, AttributeError):
        fresh = False
    lines.append("FRESH" if fresh else "STALE")
    if not os.path.exists(".code-scopes"):
        lines.append("HINT\t.code-scopes is new and committed by default; the local-scopes argument keeps it uncommitted (later: git rm --cached .code-scopes)")
    elif copied:
        lines.append(f"LOCAL\t.code-scopes is untracked, copied from {copied}: no chore commit")
    elif git("ls-files", "--error-unmatch", ".code-scopes").returncode:
        lines.append("LOCAL\t.code-scopes is untracked: no chore commit")
    if os.path.exists(OLD_STAMP) and git("ls-files", "--error-unmatch", OLD_STAMP).returncode:
        lines.append(f"HINT\t{OLD_STAMP} is obsolete, the stamp now lives in the git directory: delete the file")
    return lines


def normalise():
    try:
        doc = json.load(open(".code-scopes"))
    except (OSError, ValueError) as e:
        return [f"FAIL schema\t{e}"]
    if not isinstance(doc, dict) or doc.get("version") != 1 or not isinstance(doc.get("scopes"), list) or not doc["scopes"]:
        return ['FAIL schema\twant {"version": 1, "scopes": [...]} with at least one scope']
    fails = []
    for i, s in enumerate(doc["scopes"]):
        if not isinstance(s, dict) or set(s) != {"name", "paths", "description"}:
            fails.append(f"FAIL schema\tscopes[{i}]: want exactly name, paths, description")
            continue
        if not isinstance(s["name"], str) or not NAME.fullmatch(s["name"]):
            fails.append(f"FAIL schema\tscopes[{i}].name: lowercase, no spaces, no '/' | got {s['name']!r}")
        if not isinstance(s["paths"], list) or not s["paths"] or not all(isinstance(p, str) and p for p in s["paths"]):
            fails.append(f"FAIL schema\tscopes[{i}].paths: want at least one glob string")
        if not isinstance(s["description"], str) or not s["description"].strip():
            fails.append(f"FAIL schema\tscopes[{i}].description: want one sentence")
    names = [s.get("name") for s in doc["scopes"] if isinstance(s, dict)]
    fails += [f"FAIL schema\tduplicate scope name {n!r}" for n in sorted({n for n in names if isinstance(n, str) and names.count(n) > 1})]
    if not fails:
        open(".code-scopes", "w").write(json.dumps({"version": 1, "scopes": doc["scopes"]}, indent=2, ensure_ascii=False) + "\n")
    return fails


cache = {}


def rx(glob):
    if glob not in cache:
        # "**/" = zero or more directories, "**" = anything, "*" = within one segment
        r = re.escape(glob).replace(r"\*\*/", "\1").replace(r"\*\*", "\0").replace(r"\*", "[^/]*").replace("\0", ".*").replace("\1", "(?:.*/)?")
        cache[glob] = re.compile(r)
    return cache[glob]


def resolve(paths):
    doc = json.load(open(".code-scopes"))
    files = zlist("ls-files", "--cached", "--others", "--exclude-standard", "-z")
    for glob in (g for s in doc["scopes"] for g in s["paths"]):
        if not any(rx(glob).fullmatch(f) for f in files):
            print("DEAD GLOB\t" + glob)
    for path in paths:
        hit = next((s["name"] for s in doc["scopes"] if any(rx(g).fullmatch(path) for g in s["paths"])), "NO MATCH")
        print(hit + "\t" + path)


def main(args):
    root, _ = enter_root()
    if args == ["--stamp"]:
        os.makedirs(os.path.dirname(stamp_path()), exist_ok=True)
        open(stamp_path(), "w").write(stamp_value() + "\n")
        print("STAMPED", stamp_value().split()[0])
    elif args == ["--gate"]:
        print("\n".join(gate(root)))
    elif args == ["--normalise"]:
        fails = normalise()
        print("\n".join(fails) if fails else "OK")
        sys.exit(1 if fails else 0)
    elif args == ["--staged"]:
        resolve(zlist("-c", "core.quotepath=off", "diff", "--cached", "--name-only", "-z"))
    elif args == ["--inventory"]:
        resolve([f for f in zlist("ls-files", "--cached", "--others", "--exclude-standard", "-z") if f != OLD_STAMP])
    else:
        resolve(args)


if __name__ == "__main__":
    main(sys.argv[1:])

#!/usr/bin/env python3
# commit-pr preflight (see ../SKILL.md [PREFLIGHT]): one call, from any cwd inside the repo or a linked worktree.
# Prints tab-separated facts, then the scopes.py --gate lines, then three headed blocks:
#   ROOT\t<path>                      the work tree the shell is in; every staged path is relative to it
#   CWD\t<path>                       only when the shell is not at ROOT
#   WORKTREE\t<branch>\t<main path>   only in a linked worktree
#   BRANCH\t<name>                    "(detached)" on a detached HEAD
#   DEFAULT\t<name>                   the default branch
#   MSG\t<path>  PATCH\t<path>        scratch files for [COMMIT] and the [STAGE] hunk split
#   FRESH|STALE, LOCAL\t…, HINT\t…    the gate
#   STATUS / STAGED / UNSTAGED        git status --porcelain, git diff --cached --stat, git diff --stat
# Outside a repository: prints ERROR\tnot a git repository, exits 2.
import os, pathlib, sys

sys.dont_write_bytecode = True  # no __pycache__ beside the skill: its directory may be tracked or read-only
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import scopes
from scopes import git


def default_branch():
    head = git("symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD").stdout.strip()
    if head:
        return head.split("/", 1)[1]
    names = [git("config", "init.defaultBranch").stdout.strip(), "main", "master"]
    return next((b for b in names if b and not git("show-ref", "--verify", "--quiet", "refs/heads/" + b).returncode), names[0] or "main")


root, start = scopes.enter_root()
branch = git("branch", "--show-current").stdout.strip() or "(detached)"
print("ROOT\t" + root)
if start != root:
    print("CWD\t" + start)
if scopes.git_dir("--git-dir") != scopes.git_dir("--git-common-dir"):
    print(f"WORKTREE\t{branch}\t{scopes.main_checkout()}")
print("BRANCH\t" + branch)
print("DEFAULT\t" + default_branch())
print("MSG\t" + scopes.scratch(root, "msg"))
print("PATCH\t" + scopes.scratch(root, "patch"))
print("\n".join(scopes.gate(root)))
for head, args in (("STATUS", ["status", "--porcelain"]), ("STAGED", ["diff", "--cached", "--stat"]), ("UNSTAGED", ["diff", "--stat"])):
    print(head)
    print(git(*args).stdout, end="")

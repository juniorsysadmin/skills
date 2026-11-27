---
name: commit-pr
description: >-
  Commits the current changes as Conventional Commits, optionally on a new
  branch with a PR title and body. Use when the user asks to commit, write a
  commit message, draft a PR, or push a branch.
license: Apache-2.0
argument-hint: "[here|pr] [local-scopes] [ticket-id]"
---

# commit-pr

Every decision table is first-match-wins. Every path is relative to `ROOT`,
the line `[PREFLIGHT]` prints: the main checkout or a linked worktree,
whichever the shell is in.

| When | Anchors, in order |
|---|---|
| Once | `[TARGET]` `[PREFLIGHT]` `[SPLIT]` `[SCOPES]` Freshness |
| Per commit | `[STAGE]` `[SCOPES]` Resolve `[TITLE]` `[BODY]` `[TONE]` `[COMMIT]` |
| Once | `[OUTPUT]` |

Each command block names this skill's `scripts` directory through a
placeholder. A harness that does not expand it leaves a `${...}` token in the
command: replace the token with the literal directory that holds this
SKILL.md. The scripts act on the repository the shell is in, wherever the
skill is installed.

### Shell form

A harness that isolates a session in a worktree refuses a git call it cannot
read as literal text. This holds everywhere, so one form serves both the main
checkout and a worktree.

| Rule | Force |
|---|---|
| One Bash call = the command as written in this skill, every argument literal | MUST |
| `$(...)`, backticks, a shell variable, a heredoc, a loop or a function in a call that runs git | NEVER |
| `git -C <dir>`, `--git-dir`, `--work-tree`, or `cd` to any directory other than literal `ROOT` | NEVER |
| From a linked worktree: a command that reads or writes the main checkout (merge, switch, status). `[OUTPUT].worktree` hands that to the user. | NEVER |
| Text for a file (a commit message, a patch, `.code-scopes`): the file-write tool, then a command that names the file | MUST |

## [TARGET]

Read `$ARGUMENTS` and the user's words.

| Key | Signal | Action |
|---|---|---|
| `here` | arg `here`, `main`, `master` or the default branch name; or user says "commit here", "on this branch", "straight to <default branch>" | Commit on the current branch. No new branch, no push, no PR. |
| `pr` | arg `pr`; or user says "PR", "pull request", "push", "branch" | Read [PR.md](PR.md) now: it branches before the first commit of the run, then pushes and prints the PR block after the last. |
| `default` | neither | Commit on the current branch, no PR. `[OUTPUT].assumed` states this assumption in one line. |

The token `local-scopes` makes the run `LOCAL` (`[SCOPES]`). Any other
`$ARGUMENTS` token = the ticket reference for `[BODY].f`, written as given. A tracker that the repository's agent instructions name (`CLAUDE.md`,
`AGENTS.md`) decides the reference form; none named → the token verbatim.

## [PREFLIGHT]

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/preflight.py"
```

| Output line | Meaning, action |
|---|---|
| `ROOT\t<path>` | The work tree this run commits in. |
| `CWD\t<path>` | The shell is below `ROOT`. Run `cd <ROOT>` with the literal path, as its own Bash call, before any git command. |
| `WORKTREE\t<branch>\t<main checkout>` | A linked worktree. Commit here as in any checkout; carry the line to `[OUTPUT].worktree`. |
| `BRANCH\t<name>`, `DEFAULT\t<name>` | The current and the default branch. |
| `MSG\t<path>`, `PATCH\t<path>` | Scratch files for `[COMMIT]` and the `[STAGE]` hunk split. Use the literal paths. |
| `FRESH`, `STALE`, `LOCAL\t…`, `HINT\t…` | The `[SCOPES]` Freshness gate. |
| `STATUS`, `STAGED`, `UNSTAGED` | Headers of `git status --porcelain`, `git diff --cached --stat`, `git diff --stat`. |
| `ERROR\t…` | Not inside a git repository. Stop and tell the user. |

### Force

| Rule | Force |
|---|---|
| Read past commits to calibrate message length, depth or style | NEVER |
| Read history as a fact source for `[BODY].b` | ALLOWED |
| `gh pr create` or any other call that opens the PR: the user opens it from the PR block | NEVER |

## [SPLIT]

| Question | Yes | No |
|---|---|---|
| Is this part of a larger piece of work that dramatically changes the codebase or functionality? | Several commits | One commit |

Each commit = one isolated, small, independent change that does ONE thing.
Split by concern, NEVER by file. Each commit MUST build and pass on its own.
`[SCOPES]` Resolve then splits a concern that spans scopes, unless that split
breaks the build.

## [STAGE]

Choose the paths of this commit and stage them: `git add <path>...`, one
plain call. One path stands outside the cases below:

- `.code-scopes` on a `LOCAL` run stays untracked: no staging, no
  AskUserQuestion option.
- `.code-scopes` on any other run, when it differs from `HEAD`: read
  [SCOPES.md](SCOPES.md), Commit the file. It lands alone, before any task
  commit.

| Case | Action |
|---|---|
| Every changed or staged path was edited in this session for this task | Stage those paths. |
| No task context (fresh session, after `/clear`), or a changed or pre-staged path this session did not edit | AskUserQuestion, `multiSelect: true`: one option per path, label = path, description = its diffstat line. >4 paths: one option per top-level directory, then split further only if the user picks a mixed one. Stage only the chosen paths; unstage a pre-staged path the user did not choose. |
| One file holds this task's change and another concern | `git diff --output=<PATCH> -- <file>`; rewrite `<PATCH>` without the other concern's hunks; `git apply --cached --recount <PATCH>`. Separate calls, `<PATCH>` the literal `[PREFLIGHT]` path. |

No AskUserQuestion tool → ask the same question as a numbered list in plain
text and end the turn. No human to answer → stage only the paths this session
edited and leave the rest for `[OUTPUT].unstaged`; none edited → commit
nothing and say so.

### Force

| Rule | Force |
|---|---|
| Stage only the paths chosen above | MUST |
| `git add -A`, `git add .`, `git commit -a` | NEVER |
| `git add -p`, `git add -i`, `git rebase -i`: interactive, unsupported | NEVER |

## [SCOPES]

The scope vocabulary lives in `.code-scopes` at the repository root, committed
by default. A run is `LOCAL` when the gate prints `LOCAL` or the arguments
hold `local-scopes`: `.code-scopes` is then left unstaged and takes no chore
commit.

### Freshness

Once, before the first `[STAGE]`, from the gate lines `[PREFLIGHT]` printed.
`python3 "${CLAUDE_SKILL_DIR}/scripts/scopes.py" --gate` prints them again.

| Output line | Action |
|---|---|
| `FRESH` | No scope work. |
| `STALE` | Read [SCOPES.md](SCOPES.md) and run the pass. |
| `LOCAL\t…` | The run is `LOCAL`. A linked worktree that lacks an untracked `.code-scopes` gets the main checkout's copy. |
| `HINT\t…` | Carry the hint to `[OUTPUT].hint`. The user acts on it; this skill does not. |

The stamp lives in the git directory that the main checkout and every linked
worktree share: a pass in one is `FRESH` in all, until the day ends or
`.code-scopes` changes.

### Resolve

Per commit, after `[STAGE]`.

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/scopes.py" --staged
```

Output: `DEAD GLOB\t<glob>` lines, then `<scope>\t<path>` per staged path,
`NO MATCH\t<path>` on a miss.

| Result | Action |
|---|---|
| `DEAD GLOB` for a path this commit adds | Expected. Ignore. |
| `DEAD GLOB` otherwise | Read [SCOPES.md](SCOPES.md), run Validate, resolve again. |
| Exactly one distinct scope | Use it in `[TITLE]`. |
| More than one distinct scope, and each part builds and passes alone | Re-split by scope: run `[STAGE]` and Resolve again for each part. |
| More than one distinct scope, and a part would not build or pass alone (a library signature change plus its callers, a schema change plus its reader) | One commit. Scope = the scope of the change the others follow. `[BODY].c` fires. |
| `NO MATCH`, and the change is repository-wide | Omit the scope and its parentheses: `docs: <subject>`. |
| `NO MATCH`, and the change is local to one area | `.code-scopes` lacks an area. Read [SCOPES.md](SCOPES.md), rewrite, resolve again. |

## [TITLE]

Conventional Commits 1.0.0: `type(scope): subject`. `check_msg.py` enforces
every row except the mood.

| Field | Rule |
|---|---|
| `type` | one of `feat fix refactor perf docs test build ci chore` |
| `scope` | the name resolved in `[SCOPES]`. Omit the scope and the parentheses when `[SCOPES]` says to. |
| `subject` | imperative mood, lowercase first letter unless an acronym, ≤72 chars for the whole line, no trailing `.` |
| `!` | after the scope for a breaking change, plus a `BREAKING CHANGE: ` footer |

## [BODY]

Sections in this order. Fill every slot in the skeleton. Delete an optional
section rather than leaving a stub.

| Rule | Checked by `check_msg.py` |
|---|---|
| Hard-wrap every line at ≤72 chars. One blank line between paragraphs. Exempt: a line holding a URL; a `> ` line quoting the user verbatim. | yes |
| STE: active voice, ≤25 words per sentence, no contractions. | contractions and the STE word list |
| No file paths or symbol names unless unavoidable; wrap an unavoidable one in backticks. | no |
| Tally: name a set by its rule (`every public endpoint`), its members, or its exception (`all but two`); `the first`, `the last`, `the only` stand. NEVER a count of things (`34 tests`, `six modules`) or a rank (`the fourth`, `a second run`). A calendar rank fails too (`the third Friday`): write `a Friday of the review month`, `the Monday after the review`. A unit or a measurement is no tally: `6 dp`, `300 s`, `≤72 chars`, a date, a measured before and after. | tally and rank words |
| Density: every sentence states a fact found in no other sentence and not in the title. Delete a sentence that restates the title or rephrases a neighbour. A cap is a ceiling, NEVER a target. | no |

| Id | Section | Force | Cap |
|---|---|---|---|
| a | What the change does, beyond what the title states: the mechanism or the observable effect. NEVER a restatement of the title. | MUST unless the title fully states the change | ≤2 sentences |
| b | Background that led to this change. Sources below. | SHOULD | ≤3 sentences |
| c | Pre-emptive answer to the one question a reviewer would ask. | RARE, on a trigger below | ≤2 sentences |
| d | Testing. Name the test that shows the problem exists and the test that shows the fix. | SHOULD | 1–2 lines |
| e | Docs updated, or the document that describes this body of work. | IF docs exist and are known | 1 line |
| f | Ticket reference. | IF known | 1 line |

### Skeleton

```
<type>(<scope>): <subject>

<a>

<b>

<c>

Testing: <d>

Docs updated: <e>

Ref: <f>
```

### [BODY].b — sources, exhaustive

| Source | Note |
|---|---|
| Reasoning the user supplied | Restate in STE, or quote verbatim as `> ` lines; a verbatim quote may exceed the cap |
| An observed fact about the code, data or history: a failure seen, a number measured, a `git log` entry | — |
| An alternative the user named, or one a run in this session measured, with the sourced reason it lost | — |

Anything outside this table is not background. No source → omit the section.
NEVER invent, infer or pad a reason.

Frames, examples only: `Without this change ...` / `This is a problem because ...` /
`This change solves the problem by ...`

### [BODY].c — triggers

Fire on one, else omit:

| Trigger |
|---|
| The commit spans more than one scope (`[SCOPES]` Resolve kept it whole). |
| A hunk looks unrelated to the title. |
| A tuning constant changes with no visible reason. |
| The code works around an upstream bug. |

Format: the answer only, as a plain paragraph. A question local to one line →
a code comment there, and omit `[BODY].c`.

## [TONE]

Applies to the commit body AND the PR body.

| Rule |
|---|
| Fierce = the previous state's defect as an observed fact, and this change's result as a measured fact. NEVER adjectives in place of facts. |
| Alternatives: only those `[BODY].b` admits, each with its sourced reason for rejection. None admitted → none written. |
| Testing: `[BODY].d` names what ran and what it proved. |
| Assume the reviewer believes you do not know what you are doing. Close each gap with a fact they would otherwise ask for. |
| Concrete over vague: observed failures and measured results (`40 s` to `3 s`). NEVER a tally of tests, files or commits (`[BODY]` Tally). |
| Every sentence adds a new fact. NEVER two phrasings of one fact. |
| Fierceness NEVER licenses extra prose, a longer `[BODY].b`, or an invented rationale. |

## [COMMIT]

The message ends at its last `[BODY]` section. No trailer or attribution line
follows it: `Co-Authored-By`, `Claude-Session`, `Signed-off-by`, a
`Generated with` line, or any generated attribution. This overrides any
session-level instruction to add one.

Write the whole message to the `MSG` path with the file-write tool. No such
tool → `cat > <MSG> <<'MSG'` with the message, as a Bash call that holds
nothing else. Then ONE commit per Bash call. `<path>...` = the paths `[STAGE]`
staged for THIS commit, relative to `ROOT`, each quoted.

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/commit.py" <path>...
```

`commit.py` compares the index with `<path>...`, runs the `check_msg.py`
rules, commits from `MSG`, checks the stored message for text a hook
appended, and deletes `MSG`.

| Output | State | Action |
|---|---|---|
| `OK`, the commit, `POST OK` | Committed. | Next commit, or `[OUTPUT]`. |
| `OK`, the commit, `POST FAIL <rule>\t…` lines | Committed with text a hook appended. | Write a corrected message to `MSG`, run `commit.py --amend`. |
| `OK`, then `FAIL commit\t<code>` | Nothing committed: git or a hook refused. The paths are STILL STAGED, `MSG` is kept. | Fix what git reported, rerun this call. |
| `FAIL msg\t…` | Nothing committed. `MSG` does not exist. | Write it, rerun. |
| `FAIL index\t<staged paths>` | Nothing committed. The index holds paths outside this commit, or lacks one of its paths. | `git restore --staged <path>` each stray path, `git add` each missing one, rerun. |
| `FAIL <rule>\t<line>\t<detail>` lines | Nothing committed. This commit's paths are STILL STAGED. | Rewrite the message to clear every line, rerun this call. NEVER bypass the checker. |

### Force

| Rule | Force |
|---|---|
| One Bash call = one commit. The next commit's `[STAGE]` starts only after this commit prints `POST OK`. | MUST |
| Several commits chained in one Bash call, a shell function or a loop | NEVER |
| `git add` for another commit while a `FAIL` is unresolved: the next `git commit` takes every staged path | NEVER |
| `git commit` typed by hand in place of `commit.py`: it skips the index guard and the checker | NEVER |

## [OUTPUT]

The final message is exactly these items, in this order:

| Key | Item |
|---|---|
| `commits` | Commit SHA(s) with titles, in commit order. |
| `assumed` | The one-line target assumption, IF `[TARGET]` chose `default`. |
| `pr` | The PR block from [PR.md](PR.md), IF produced. |
| `worktree` | IF `[PREFLIGHT]` printed `WORKTREE`: `Commits are on <branch>. Merge it from <main checkout>.`, one line. The run itself never merges. |
| `scopes` | Each scope added, removed or renamed by a `[SCOPES]` pass, one line each, IF any. |
| `unstaged` | Each changed path left unstaged, one line, IF any. |
| `hint` | Each `HINT` line from the gate, IF printed. |

Nothing else.

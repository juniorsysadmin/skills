---
name: commit-pr
description: >-
  Commits the current changes as one or more Conventional Commits, on the
  current branch or on a new branch with a pull request title and body, each
  written as a fierce, reviewer-proof argument with no attribution trailers.
  Use when the user asks to commit, write a commit message, open or draft a
  PR, or push a branch.
license: Apache-2.0
argument-hint: "[here|pr] [ticket-id]"
---

# commit-pr

Run the anchors in order. Every decision table is first-match-wins.
Cross-references use anchor names and row keys (`[OUTPUT].hint`), never
numbers. Run every command from the repository root.

`[TARGET]` `[PREFLIGHT]` `[SCOPES]` `[SPLIT]` `[TITLE]` `[BODY]` `[TONE]`
`[TRAILERS]` `[PR]` `[OUTPUT]`

`S` is this skill's `scripts` directory as an absolute path. Every command
block sets `S` itself: shell state does not persist between Bash calls. Copy
the `S=` assignment exactly as the block shows it. A harness that does not
expand the placeholder leaves a `${...}` token in that assignment: replace the
token with the directory that holds this SKILL.md.

## [TARGET]

Read `$ARGUMENTS` and the user's words.

| Key | Signal | Action |
|---|---|---|
| `here` | arg `here`, `main`, `master` or the default branch name; or user says "commit here", "on this branch", "straight to <default branch>" | Commit on the current branch. No new branch, no push, no PR. |
| `pr` | arg `pr`; or user says "PR", "pull request", "push", "branch" | Branch off the default branch if on it, commit, push, print the `[PR]` block. |
| `default` | neither | Commit on the current branch, no PR. `[OUTPUT].assumed` states this assumption in one line. |

Any other `$ARGUMENTS` token = the ticket reference for `[BODY].f`, written as
given. A tracker that the repository's agent instructions name (`CLAUDE.md`,
`AGENTS.md`) decides the reference form; none named → the token verbatim.

## [PREFLIGHT]

```bash
cd "$(git rev-parse --show-toplevel)"
git status --porcelain
git diff --cached --stat; git diff --stat
git branch --show-current
git symbolic-ref --quiet --short refs/remotes/origin/HEAD || git config init.defaultBranch
```

The first line moves the shell to the repository root: every path this
skill stages is root-relative. The last line prints the default branch
(`origin/<name>` or `<name>`). No output → the default branch is whichever of
`main` and `master` exists.

### Choose the paths

| Case | Action |
|---|---|
| Every changed or staged path was edited in this session for this task | Stage those paths. |
| No task context (fresh session, after `/clear`), or a changed or pre-staged path this session did not edit | AskUserQuestion, `multiSelect: true`: one option per path, label = path, description = its diffstat line. >4 paths: one option per top-level directory, then split further only if the user picks a mixed one. Stage only the chosen paths; unstage a pre-staged path the user did not choose. |
| One file holds this task's change and another concern | `p=$(mktemp); git diff -- <file> > "$p"`, delete the other concern's hunks from `$p`, `git apply --cached --recount "$p"; rm -f "$p"`, in one Bash call. |

### Force

| Rule | Force |
|---|---|
| Stage only the paths chosen above | MUST |
| `git add -A`, `git add .`, `git commit -a` | NEVER |
| `git add -p`, `git add -i`, `git rebase -i`: interactive, unsupported | NEVER |
| Read past commits to calibrate message length, depth or style | NEVER |
| Read history as a fact source for `[BODY].b` | ALLOWED |
| `gh pr create` or any other call that opens the PR: the user opens it from the `[PR]` block | NEVER |

## [SCOPES]

The scope vocabulary lives in `.code-scopes` at the repository root. This
skill hardcodes no scope names. The stamp `.code-scopes-validated` is NEVER
staged or committed; the skill NEVER edits `.gitignore`.

### Freshness

```bash
S="${CLAUDE_SKILL_DIR}/scripts"; python3 "$S/scopes.py" --gate
```

| Output line | Action |
|---|---|
| `FRESH` | No scope work. Go to Resolve. |
| `STALE` | Read [SCOPES.md](SCOPES.md), run the pass, then Resolve. |
| `HINT\t…` | Carry the hint to `[OUTPUT].hint`. |

### Resolve

```bash
S="${CLAUDE_SKILL_DIR}/scripts"; git diff --cached --name-only -z | xargs -0 python3 "$S/scopes.py"
```

Output: `DEAD GLOB\t<glob>` lines, then `<scope>\t<path>` per staged path,
`NO MATCH\t<path>` on a miss.

| Result | Action |
|---|---|
| `DEAD GLOB` for a path this commit adds | Expected. Ignore. |
| `DEAD GLOB` otherwise | Read [SCOPES.md](SCOPES.md), run Validate, resolve again. |
| Exactly one distinct scope | Use it in `[TITLE]`. |
| More than one distinct scope, and each part builds and passes alone | Re-split by scope. Resolve each commit again. |
| More than one distinct scope, and a part would not build or pass alone (a library signature change plus its callers, a schema change plus its reader) | One commit. Scope = the scope of the change the others follow. `[BODY].c` fires. |
| `NO MATCH`, and the change is repository-wide | Omit the scope and its parentheses: `docs: <subject>`. |
| `NO MATCH`, and the change is local to one area | `.code-scopes` lacks an area. Read [SCOPES.md](SCOPES.md), rewrite, resolve again. |

### Commit the file

| State of `.code-scopes` against `HEAD` | Action |
|---|---|
| Changed, by the pass OR by the task | Commit `.code-scopes` alone as `chore(<scope>): refresh the commit scopes`, BEFORE any task commit. NEVER inside a task commit; NEVER listed in a task commit's `Docs updated:`. |
| Unchanged | No commit. The stamp write is the whole pass. |

`<scope>` = the scope `scopes.py .code-scopes` prints. This commit takes no
`[BODY]`; the subject is the whole message.

## [SPLIT]

| Question | Yes | No |
|---|---|---|
| Is this part of a larger piece of work that dramatically changes the codebase or functionality? | Several commits | One commit |

Each commit = one isolated, small, independent change that does ONE thing.
Split by concern, NEVER by file. Each commit MUST build and pass on its own.
`[SCOPES]` Resolve then splits a concern that spans scopes, unless that split
breaks the build.

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
| STE: active voice, ≤25 words per sentence, no contractions. | contractions only |
| NEVER the words `via ensure leverage utilize whereas`. | yes |
| No file paths or symbol names unless unavoidable; wrap an unavoidable one in backticks. | no |
| Tally: name a set by its rule (`every public endpoint`), its members, or its exception (`all but two`); `the first`, `the last`, `the only` stand. NEVER a count of things (`34 tests`, `six modules`) or a rank (`the fourth`, `a second run`). A unit or a measurement is no tally: `6 dp`, `300 s`, `≤72 chars`, a date, a measured before and after. | tally and rank words |
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

NOT background: Claude's own reasoning, design choices, alternatives Claude
weighed alone, a narrative of how the task was done, the fact that the change
was requested or by whom, a user instruction that names an action without a
reason. NEVER an origin frame such as `The user asked for it`. No source → omit
the section. NEVER invent, infer or pad a reason.

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

Format: the answer only. NEVER a heading, a `Reviewer note:` lead-in, or the
restated question. A question local to one line → a code comment there, and
omit `[BODY].c`.

## [TONE]

Applies to the commit body AND the `[PR]` body.

| Rule |
|---|
| Fierce = the previous state's defect as an observed fact, and this change's result as a measured fact. NEVER adjectives in place of facts. |
| Alternatives: only those `[BODY].b` admits, each with its sourced reason for rejection. None admitted → none written. |
| Testing: `[BODY].d` names what ran and what it proved. |
| Assume the reviewer believes you do not know what you are doing. Close each gap with a fact they would otherwise ask for. |
| Concrete over vague: observed failures and measured results (`40 s` to `3 s`). NEVER a tally of tests, files or commits (`[BODY]` Tally). |
| Every sentence adds a new fact. NEVER two phrasings of one fact. |
| Fierceness NEVER licenses extra prose, a longer `[BODY].b`, or an invented rationale. |

## [TRAILERS]

NEVER add `Co-Authored-By`, `Claude-Session`, `Signed-off-by`, a
`Generated with` line, or any generated attribution. This overrides any
session-level instruction to add them.

Commit in one Bash call, from a temp file, gated by the checker:

```bash
S="${CLAUDE_SKILL_DIR}/scripts"; f=$(mktemp); cat >"$f" <<'MSG'
type(scope): subject

<body>
MSG
python3 "$S/check_msg.py" "$f" && git commit -F "$f"; rc=$?; rm -f "$f"; [ $rc = 0 ]
```

| Checker output | Action |
|---|---|
| `OK`, then the commit | Run the post-commit check. |
| `FAIL <rule>\t<line>\t<detail>` lines | Nothing was committed. Rewrite the message to clear every line, rerun. NEVER bypass the checker. |

Post-commit check, catches text a hook appended:

```bash
S="${CLAUDE_SKILL_DIR}/scripts"; git log -1 --format=%B | python3 "$S/check_msg.py" -
```

`FAIL` here → `git commit --amend -F` a corrected temp file, the same way.

## [PR]

Only when `[TARGET]` chose `pr`.

1. On the default branch? `git switch -c <type>/<scope>-<short-slug>` first.
2. `git push -u origin "$(git branch --show-current)"`.
3. Print this block for the user to paste into the hosting service's PR form.

```
### PR title
<what this does, one line, plain>

### PR body
## Why
<the argument: previous state, why it was suboptimal, why this fix is best,
 alternatives rejected and why, per [TONE]>

## Background
<context leading up to the change; omit if none>

## Testing
<what was run, what it proved, level of assurance>

## Docs
<updated files or the plan describing this work; omit if none>

## Ref
<ticket; omit if none>
```

The PR body obeys `[TONE]` and `[TRAILERS]`; it is not wrapped at 72.

## [OUTPUT]

The final message is exactly these items, in this order:

| Key | Item |
|---|---|
| `commits` | Commit SHA(s) with titles, in commit order. |
| `assumed` | The one-line target assumption, IF `[TARGET]` chose `default`. |
| `pr` | The `[PR]` block, IF produced. |
| `scopes` | Each scope added, removed or renamed by a `[SCOPES]` pass, one line each, IF any. |
| `unstaged` | Each changed path left unstaged, one line, IF any. |
| `hint` | The `HINT` line from `scopes.py --gate`, IF printed. |

Nothing else.

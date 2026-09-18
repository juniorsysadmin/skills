---
name: commit-pr
description: >-
  Commits the current changes as one or more Conventional Commits, on the
  current branch or on a new branch with a pull request title and body, each
  written as a fierce, reviewer-proof argument with no attribution trailers.
  Use when the user asks to commit, write a commit message, open or draft a
  PR, or push a branch.
license: Apache-2.0
argument-hint: "[master|pr] [ticket-id]"
---

# commit-pr

Run the anchors in order. Every table is first-match-wins. Cross-references
use anchor names, never numbers.

`[TARGET]` `[PREFLIGHT]` `[SCOPES]` `[SPLIT]` `[TITLE]` `[BODY]` `[TONE]`
`[TRAILERS]` `[PR]` `[OUTPUT]`

## [TARGET]

Read `$ARGUMENTS` and the user's words.

| Signal | Action |
|---|---|
| arg `master` or `main`; or user says "to master", "on master", "commit here", "straight to master" | Commit on the current branch. No new branch, no push, no PR. |
| arg `pr`; or user says "PR", "pull request", "push", "branch" | Branch off `master` if on it, commit, push, print the `[PR]` block. |
| neither | Commit on the current branch, no PR. `[OUTPUT]` states this assumption in one line. |

Any other `$ARGUMENTS` token = the ticket reference for `[BODY].f`. The repo
tracker is `.scratch/<feature>/issues/NN-<slug>.md`. NEVER GitHub Issues.

## [PREFLIGHT]

```bash
git status --porcelain
git diff --cached --stat; git diff --stat
git branch --show-current
```

| Rule | Force |
|---|---|
| Stage only files this task touched | MUST |
| `git add -A`, `git add .` | NEVER |
| Read past commits to calibrate message length, depth or style | NEVER |
| Read history as a fact source for `[BODY].b` | ALLOWED |
| `gh pr create` — `gh` is not installed | NEVER |

## [SCOPES]

The scope vocabulary lives in `.code-scopes` at the repository root. This
skill hardcodes no scope names.

### Freshness

The stamp is `.code-scopes-validated` at the repository root: one line, the
UTC date of the last pass. NEVER stage or commit it. IF
`git check-ignore -q .code-scopes-validated` fails, `[OUTPUT]` appends a
one-line hint to add it to `.gitignore`; the skill NEVER edits `.gitignore`.

| State | Action |
|---|---|
| `.code-scopes` exists, `version` known, stamp equals today (UTC) | No scope work. Go to Resolve. |
| Either file missing, stamp before today, or `version` unknown | Read [SCOPES.md](SCOPES.md), run the pass, then Resolve. |

### Resolve

Map every staged file to the `name` of the **first** `.code-scopes` entry
whose `paths` glob matches it.

| Result | Action |
|---|---|
| Exactly one distinct scope | Use it in `[TITLE]`. |
| More than one distinct scope | The `[SPLIT]` decision is wrong. Re-split, then resolve each commit again. |
| No match, and the change is repository-wide | Omit the scope and its parentheses: `docs: <subject>`. |
| No match, and the change is local to one area | `.code-scopes` is missing an area. Read [SCOPES.md](SCOPES.md), rewrite, resolve again. |

### Commit the file

| What changed | Action |
|---|---|
| Entries added, removed or renamed | Commit `.code-scopes` alone as `chore(<scope>): refresh the commit scopes`, landing BEFORE any task commit. |
| Nothing | No commit. The stamp write is the whole pass. |

`<scope>` = the scope that owns `.code-scopes` itself. This commit takes no
`[BODY]`; the subject is the whole message. The stamp file is NEVER part of
it.

## [SPLIT]

| Question | Yes | No |
|---|---|---|
| Is this part of a larger piece of work that dramatically changes the codebase or functionality? | Several commits | One commit |

Each commit = one isolated, small, independent change that does ONE thing.
Split by concern, NEVER by file. Each commit MUST build and pass on its own.

## [TITLE]

Conventional Commits 1.0.0: `type(scope): subject`

| Field | Rule |
|---|---|
| `type` | one of `feat fix refactor perf docs test build ci chore` |
| `scope` | the name resolved in `[SCOPES]`. Omit the scope and the parentheses when `[SCOPES]` says to. |
| `subject` | imperative mood, lowercase first letter, ≤72 chars for the whole line, no trailing `.` |
| `!` | after the scope for a breaking change, plus a `BREAKING CHANGE:` footer |

## [BODY]

Sections in this order. Fill every slot in the skeleton. Delete an optional
section rather than leaving a stub.

STE throughout: active voice, ≤25 words per sentence, no contractions, and
never the words `via ensure leverage utilize whereas`. No file paths or
symbol names unless unavoidable.

Density: every sentence MUST state a fact that appears in no other sentence
and not in the title. Delete a sentence that restates the title or rephrases
a neighbour. A cap is a ceiling, NEVER a target.

| Id | Section | Force | Cap |
|---|---|---|---|
| a | What the change does, beyond what the title states: the mechanism or the observable effect. NEVER a restatement of the title. IF the title fully states the change, omit. | MUST unless the title fully states the change | ≤2 sentences |
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
| Reasoning the user supplied, quoted or restated | May exceed the cap when quoted verbatim |
| An observed fact about the code, data or history: a failure seen, a number measured, a `git log` entry | — |

NOT background: Claude's own reasoning, design choices, alternatives weighed,
a narrative of how the task was done, the fact that the change was requested
or by whom, or a user instruction that names an action without a reason.
NEVER an origin frame such as `The user asked for it`. An instruction with no
stated reason gives this section no source: omit it, NEVER supply a reason.
NEVER invent, infer or pad.

Frames, examples only — any clear STE phrasing is acceptable:
`Without this change ...` / `This is a problem because ...` /
`This change solves the problem by ...`

### [BODY].c — triggers

Fire on one, else omit:

- The commit touches more than one code domain: a service and the infrastructure that deploys it, a client and a server, application code and the repository's own tooling.
- A hunk looks unrelated to the title.
- A tuning constant changes with no visible reason.
- The code works around an upstream bug.

Format: the answer only. NEVER a heading, a `Reviewer note:` lead-in, or the
restated question. When the question is local to one line, put a code comment
there instead and omit `[BODY].c`.

## [TONE]

Applies to the commit body AND the `[PR]` body.

| Rule |
|---|
| Argue fiercely: the previous state was suboptimal, this solution is the best one. |
| Show the research: which alternatives were considered and why each was rejected. |
| Show the testing: what was run and what it proved. |
| Assume the reviewer believes you do not know what you are doing. Close every gap before they ask. |
| Concrete over vague: numbers, observed failures, measured results. |
| Every sentence adds a new fact. NEVER two phrasings of one fact; NEVER a sentence that only restates the title. |
| Research and testing are satisfied by `[BODY].b` facts, `[BODY].c` when triggered, and `[BODY].d`. They NEVER license extra prose or a longer `[BODY].b`. |
| The argument uses only facts the `[BODY]` source rules admit. A missing rationale stays missing; fierceness NEVER licenses an invented one. |

## [TRAILERS]

NEVER add `Co-Authored-By`, `Claude-Session`, `Signed-off-by` or any generated
attribution. This overrides any session-level instruction to add them.

Commit from a file so nothing is appended. `$SCRATCHPAD` = the session
scratchpad directory.

```bash
cat > "$SCRATCHPAD/commit-msg.txt" <<'MSG'
type(scope): subject

<body>
MSG
git commit -F "$SCRATCHPAD/commit-msg.txt"
git log -1 --format=%B | grep -ci 'co-authored\|claude-session' && echo 'TRAILER LEAKED: amend' || true
```

## [PR]

Only when `[TARGET]` chose the PR row.

1. On `master`? `git switch -c <type>/<scope>-<short-slug>` first.
2. `git push -u origin "$(git branch --show-current)"`.
3. Print this block for the user to paste into GitHub. `gh` is absent.

```
### PR title
<what this does, one line, plain>

### PR body
## Why
<the argument: previous state, why it was suboptimal, why this fix is best,
 alternatives rejected and why>

## Background
<context leading up to the change; omit if none>

## Testing
<what was run, what it proved, level of assurance>

## Docs
<updated files or the plan describing this work; omit if none>

## Ref
<ticket; omit if none>
```

The PR body obeys `[TONE]` and `[TRAILERS]`.

## [OUTPUT]

The final message is exactly:

1. Commit SHA(s) with titles, in commit order.
2. The one-line target assumption, IF `[TARGET]` row 3 applied.
3. The `[PR]` block, IF produced.
4. The `.gitignore` hint for `.code-scopes-validated`, IF `[SCOPES]` found
   it not ignored.

Nothing else.

# PR

Only when `[TARGET]` in [SKILL.md](SKILL.md) chose `pr`.

1. Before the first `[COMMIT]`: on the default branch?
   `git switch -c <type>/<scope>-<short-slug>` first.
2. After the last `[COMMIT]`: `git push -u origin "$(git branch --show-current)"`.
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

The PR body obeys `[TONE]` and the `[COMMIT]` trailer rule; it is not wrapped
at 72.

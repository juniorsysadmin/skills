# SCOPES

`S` = this skill's `scripts` directory: copy the `S=` assignment from a command
block in [SKILL.md](SKILL.md) into every Bash call that uses `$S`.

## Schema

`.code-scopes` is JSON at the repository root, tracked by git.

```json
{
  "version": 1,
  "scopes": [
    {
      "name": "checkout",
      "paths": ["services/checkout/**", "docs/services/checkout.md"],
      "description": "The checkout service: cart totals, payment capture, receipts"
    },
    {
      "name": "services",
      "paths": ["services/**", "docs/services/**"],
      "description": "Every other backend service and the contracts they share"
    }
  ]
}
```

| Field | Rule |
|---|---|
| `version` | integer, `1` |
| `scopes` | ordered array, MOST SPECIFIC FIRST; resolution takes the first match |
| `scopes[].name` | the literal string used in `type(scope):`. Lowercase, no spaces, no `/` |
| `scopes[].paths` | ≥1 glob, relative to the repository root. `**` crosses `/`, `*` stays within one segment |
| `scopes[].description` | one sentence naming what the scope covers. NO counts, NO enumerated members: nothing a new sibling directory would change |

| Constraint | Reason |
|---|---|
| `services/checkout/**` precedes `services/**` | else nothing resolves to `checkout` |
| A family of siblings in one scope = ONE family glob (`plugin-*/**`), never one glob per member | a new member then needs no edit and no chore commit |
| Every sibling that a family glob would capture but that belongs elsewhere sits in an EARLIER scope | first match wins; the exception must be seen first |
| A scope owns its documentation through `paths` (`docs/services/inventory.md` → `inventory`) | NEVER a `docs` scope: it yields `docs(docs)` |
| `indent=2`, one path per line | the chore commit's diff shows one line per changed path |

## Commands

| Action | Command |
|---|---|
| Inventory: tracked plus new, never ignored | `git ls-files --cached --others --exclude-standard` |
| Top-level entries | `git ls-files --cached --others --exclude-standard \| cut -d/ -f1 \| sort -u` |
| Dead globs, then the scope of each path | `python3 "$S/scopes.py" <path>...` |
| Scope of every inventory path | `git ls-files -z --cached --others --exclude-standard \| xargs -0 python3 "$S/scopes.py"` |
| Stamp write: the last step of every pass | `python3 "$S/scopes.py" --stamp` |

NEVER hand-edit the JSON, NEVER chain `sed`, NEVER resolve a glob by eye.
Rewrite the whole file with python3:

```bash
python3 - <<'PY'
import json, pathlib, subprocess
root = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()
doc = {
    "version": 1,
    "scopes": [
        {"name": "...", "paths": ["..."], "description": "..."},
    ],
}
pathlib.Path(root, ".code-scopes").write_text(json.dumps(doc, indent=2) + "\n")
PY
```

## Generate

Trigger: `.code-scopes` missing, or `version` not `1`.

| Step | Action |
|---|---|
| `inventory` | Take the inventory. |
| `guide` | IF a structure guide exists (`CLAUDE.md`, `README.md`, `AGENTS.md`, `CONTRIBUTING.md`), read its structure section; prefer the names it uses. |
| `split` | One scope per top-level directory, EXCEPT: split a directory into sub-scopes where it holds independently deployed or independently reviewed units (`services/*`, `packages/*`); collapse sibling directories no commit would isolate into one scope with one family glob (a family of `plugin-*` directories → `plugin-*/**`), exceptions in earlier scopes. |
| `size` | At most 25 entries. A vocabulary too large to hold is worse than a coarse one. |
| `tooling` | One scope covers the repository's own tooling and configuration and holds `.code-scopes` in its `paths`. Name it `claude` IF `.claude/` exists, else `meta`. |
| `docs` | Attach each documentation path to the scope of the area it describes. |
| `write` | Sort most-specific-first, write the file, run `scopes.py` over the inventory: zero `DEAD GLOB`, zero `NO MATCH` outside repository-wide files. Then `--stamp`, then Commit the file. |

`git log` is NEVER a source: scope names in past commits carry no authority.

## Validate

Trigger: the file exists, `version` is `1`, the gate printed `STALE`, or
`[SCOPES]` Resolve sent you here.

| Finding | Action |
|---|---|
| `DEAD GLOB` not explained by a path this commit adds | Remove the glob. Remove the scope when no glob is left. |
| A top-level inventory entry gives `NO MATCH` | Add it: to a family glob if it matches one's pattern, else a scope per Generate `split` and `size`. |
| An independently deployed unit below a scoped directory has no more specific scope | Add a scope for it, ordered before its parent. |
| ≥3 paths in one scope share a prefix and would all match one family glob | Replace them with the family glob. Move every other path that glob would capture into an earlier scope first, then confirm with `scopes.py` over the inventory that no path changed scope. |
| A `description` holds a count or an enumeration of members | Rewrite it without them. |
| A scope name no longer matches the directory it covers after a rename | Rename the scope; keep the paths correct. |
| An entry sorts after a less specific entry that shadows it | Re-sort most-specific-first. |
| Nothing above fired | The vocabulary is current. |

Rewrite `.code-scopes` only when a finding fired. Always finish with
`--stamp`, then Commit the file.

## Commit the file

| State of `.code-scopes` against `HEAD` | Action |
|---|---|
| Changed, by the pass OR by the task | Commit `.code-scopes` alone as `chore(<scope>): refresh the commit scopes`, BEFORE any task commit. NEVER inside a task commit; NEVER listed in a task commit's `Docs updated:`. |
| Unchanged | No commit. The stamp write is the whole pass. |

`<scope>` = the scope `scopes.py .code-scopes` prints. This commit takes no
`[BODY]`; the subject is the whole message. Every scope added, removed or
renamed goes to `[OUTPUT].scopes`.

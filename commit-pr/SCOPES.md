# SCOPES

Read only when `[SCOPES]` in [SKILL.md](SKILL.md) sends you here: `.code-scopes`
is missing, its `version` is unknown, or the `.code-scopes-validated` stamp is
missing or before today.

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
| `scopes` | ordered array, MOST SPECIFIC FIRST; `[SCOPES]` resolution takes the first match |
| `scopes[].name` | the literal string used in `type(scope):`. Lowercase, no spaces, no `/` |
| `scopes[].paths` | ≥1 glob, relative to the repository root, `**` allowed |
| `scopes[].description` | one sentence naming what the scope covers |

Order matters: `services/checkout/**` must precede `services/**`, else nothing
ever resolves to `checkout`.

The stamp lives apart: `.code-scopes-validated` at the repository root holds
one line, the UTC date `YYYY-MM-DD` of the last pass. It is untracked, is
expected in `git status` output, and is NEVER staged or committed.

A scope owns its documentation through `paths`, so `docs/services/inventory.md`
belongs to the `inventory` entry. NEVER create a `docs` scope; it would produce
`docs(docs)`.

## Generate

Trigger: `.code-scopes` missing, or `version` unknown.

| Step | Action |
|---|---|
| 1 | `git ls-files --cached --others --exclude-standard` for the inventory. Tracked plus new files, NEVER gitignored build output. |
| 2 | IF a structure guide exists (`CLAUDE.md`, `README.md`, `AGENTS.md`, `CONTRIBUTING.md`), read its structure section and prefer the names it already uses. |
| 3 | One scope per top-level directory, EXCEPT: split a directory into sub-scopes where it holds independently deployed or independently reviewed units (`services/*`, `packages/*`); collapse sibling directories that no commit would ever isolate (40 `plugin-*` directories → one `plugins`). |
| 4 | Target 15–25 entries. A vocabulary too large to hold is worse than a coarse one. |
| 5 | Emit one scope covering the repository's own tooling and configuration, and give it `.code-scopes` in its `paths`, so the `[SCOPES]` chore commit has a scope. Name it `claude` IF `.claude/` exists, else `meta`. |
| 6 | Attach each documentation path to the scope of the area it describes. |
| 7 | Sort most-specific-first, write the file, then write the stamp. |

`git log` is NEVER a source. Scope names used in past commits carry no
authority, and the `[PREFLIGHT]` ban on reading history stays absolute.

## Validate

Trigger: the file exists, `version` is known, the stamp is missing or before
today.

| Finding | Action |
|---|---|
| A `paths` glob matches no file in the inventory | Remove the glob. Remove the whole scope when no glob is left. |
| A top-level entry in the inventory matches no scope | Add a scope for it, per Generate steps 3 and 4. |
| An independently deployed unit below a scoped directory matches no more specific scope | Add a scope for it, ordered before its parent. |
| A scope name no longer matches the directory it covers after a rename | Rename the scope. Keep the paths correct. |
| An entry sorts after a less specific entry that shadows it | Re-sort most-specific-first. |
| The JSON contains a `last_validated` field | Remove the field; the stamp file replaced it. |
| Nothing above fired | The vocabulary is current. |

Always finish with the stamp write below. Rewrite `.code-scopes` only when a
finding fired. Report every add, removal and rename to the user in
`[OUTPUT]`.

## Read and write

NEVER hand-edit the JSON, and NEVER resolve a glob by eye.

```bash
# Freshness gate: prints STALE when a pass is due.
[ -f .code-scopes ] || echo STALE
jq -e '.version == 1' .code-scopes > /dev/null || echo STALE
[ "$(cat .code-scopes-validated 2>/dev/null)" = "$(date -u +%F)" ] \
  && echo FRESH || echo STALE

# Gitignore hint for [OUTPUT]. NEVER edit .gitignore from this skill.
git check-ignore -q .code-scopes-validated \
  || echo "HINT: add .code-scopes-validated to .gitignore"

# Inventory for Generate and Validate. Tracked plus new, never ignored.
git ls-files --cached --others --exclude-standard
git ls-files --cached --others --exclude-standard | cut -d/ -f1 | sort -u

# Stamp write: the last step of every Generate or Validate pass.
date -u +%F > .code-scopes-validated
```

Save this resolver once as `$SCRATCHPAD/scopes.py`, then run it. It prints
every dead glob, then the scope of each path given as an argument. `**`
matches across separators, `*` matches within one segment.

```python
import json, re, subprocess, sys

doc = json.load(open(".code-scopes"))
files = subprocess.run(
    ["git", "ls-files", "--cached", "--others", "--exclude-standard"],
    capture_output=True, text=True).stdout.split()

def matches(path, glob):
    rx = re.escape(glob).replace(r"\*\*", "\x00").replace(r"\*", "[^/]*")
    return re.fullmatch(rx.replace("\x00", ".*"), path) is not None

for glob in (g for s in doc["scopes"] for g in s["paths"]):
    if not any(matches(f, glob) for f in files):
        print("DEAD GLOB:", glob)

for path in sys.argv[1:]:
    hit = next((s["name"] for s in doc["scopes"]
                if any(matches(path, g) for g in s["paths"])), "NO MATCH")
    print(hit, path)
```

```bash
python3 "$SCRATCHPAD/scopes.py" $(git diff --cached --name-only)
```

A dead glob for a path this commit is adding is expected, not a finding:
`.code-scopes` itself reports dead until its first commit lands.

Write a full rewrite with python3, never with a chain of `sed`:

```bash
python3 - <<'PY'
import json, pathlib
doc = {
    "version": 1,
    "scopes": [
        {"name": "...", "paths": ["..."], "description": "..."},
    ],
}
pathlib.Path(".code-scopes").write_text(json.dumps(doc, indent=2) + "\n")
PY
```

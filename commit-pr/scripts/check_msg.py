#!/usr/bin/env python3
# commit-pr message gate (see ../SKILL.md [TITLE] [BODY] [TRAILERS]): check_msg.py <file|->
# Prints FAIL <rule>\t<line no>\t<detail> per breach and exits 1, else prints OK and exits 0.
# Line 1 = title. Exempt from wrap/STE checks: lines starting "> " (verbatim user quote) and lines holding "://".
import re, sys

TYPES = "feat|fix|refactor|perf|docs|test|build|ci|chore"
TITLE = re.compile(rf"^({TYPES})(\([a-z0-9][a-z0-9._-]*\))?(!)?: (\S.*)$")
TRAILER = re.compile(r"^\s*(co-authored-by|claude-session|signed-off-by)\s*:|generated with \[?claude|noreply@anthropic\.com", re.I)
BANNED = re.compile(r"\b(via|ensure[sd]?|ensuring|leverag\w*|utili[sz]\w*|whereas)\b", re.I)
CONTRACTION = re.compile(r"\b\w+(n['’]t|['’](re|ve|ll|d|m))\b|\b(it|that|there|what|here|who|he|she|let)['’]s\b", re.I)
CODE = re.compile(r"`[^`]*`")
WRAP = 72

text = (sys.stdin.read() if sys.argv[1:] == ["-"] else open(sys.argv[1]).read()) if len(sys.argv) == 2 else sys.exit("usage: check_msg.py <file|->")
lines = [l for l in text.rstrip("\n").split("\n") if not l.startswith("#")]
fails = []


def fail(rule, n, detail):
    fails.append(f"FAIL {rule}\t{n}\t{detail}")


title = lines[0] if lines else ""
m = TITLE.match(title)
if not m:
    fail("title-format", 1, f"want <{TYPES}>(scope)?!?: subject | got {title!r}")
else:
    subject = m.group(4)
    if subject.endswith("."):
        fail("title-period", 1, "subject ends with '.'")
    if re.match(r"[A-Z][a-z]", subject):
        fail("title-case", 1, "subject starts capitalised; lowercase unless an acronym")
    if m.group(3) and not re.search(r"^BREAKING CHANGE: \S", text, re.M):
        fail("breaking-footer", 1, "'!' in title needs a 'BREAKING CHANGE: ' footer")
if len(title) > WRAP:
    fail("title-length", 1, f"{len(title)} > {WRAP} chars")
if len(lines) > 1 and lines[1].strip():
    fail("title-blank", 2, "line 2 must be blank")

for n, line in enumerate(lines, 1):
    if TRAILER.search(line):
        fail("trailer", n, line.strip())
    if line.startswith("> ") or "://" in line:
        continue
    if n > 1 and len(line) > WRAP and " " in line.strip():
        fail("wrap", n, f"{len(line)} > {WRAP} chars; hard-wrap at {WRAP}")
    prose = CODE.sub("", line)
    for w in BANNED.finditer(prose):
        fail("ste-word", n, w.group(0))
    for c in CONTRACTION.finditer(prose):
        fail("contraction", n, c.group(0))

print("\n".join(fails) if fails else "OK")
sys.exit(1 if fails else 0)

---
name: aws-docs
description: >-
  Search and read official AWS documentation. Use for any question about a
  specific AWS service: its API, CLI, SDK, CloudFormation/CDK resources,
  quotas, or console behaviour. Prefer over web search for
  docs.aws.amazon.com, and use it even when you think you know the answer:
  AWS defaults and limits change.
license: Apache-2.0
---

# AWS Documentation

Binding rules, in force for the whole session:

1. State an AWS fact (a parameter, default, limit or behaviour) only as a
   page fetched this session gives it, and give that page's `.html` URL.
2. Reach every URL from the user, a search result, or a link on a fetched
   page.

Two operations: **search** (bundled script), then **read** (WebFetch,
`.md`-first). Already holding a `docs.aws.amazon.com` URL → go straight to
Reading.

## Searching

```bash
"${CLAUDE_SKILL_DIR}/scripts/aws_docs" search "S3 access point policy" --limit 3
```

The command path is absolute and works from any directory. A harness that
does not expand the placeholder leaves a `${...}` token in the path: replace
the token with the directory that holds this SKILL.md.

Result links end in `.html` — read each one `.md`-first. End-to-end:

```bash
"${CLAUDE_SKILL_DIR}/scripts/aws_docs" search "CloudFront function size limit" --limit 3
# → 1. Quotas - Amazon CloudFront
#      https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/cloudfront-limits.html
# then WebFetch …/cloudfront-limits.md
```

Flags:

- `--limit N` — max results, 1–50 (default 5).
- `--intent "<text>"` — natural-language description of the goal; improves
  ranking (no PII).
- `--product "<name>"` — filter by product (repeatable). Must match AWS's
  exact catalogue name, e.g. `"Amazon Simple Storage Service"`.
- `--guide "<name>"` — filter by guide type (repeatable), e.g. `"User Guide"`,
  `"API Reference"`.
- `--json` — print the raw JSON response.

## Reading a page (`.md`-first)

Every AWS docs page has a clean markdown sibling. Always fetch it instead of
the HTML:

1. Drop any `#fragment` or `?query` from the URL.
2. Replace the trailing `.html` with `.md`:
   `…/userguide/access-points.html` → `…/userguide/access-points.md`
3. `WebFetch` the `.md` URL. Works for user guides, developer guides, and API
   references.
4. If the `.md` returns 404, `WebFetch` the original `.html` URL.

Links inside the `.md` are relative (e.g. `[Objects](UsingObjects.md)`);
resolve them against the current page's directory to follow them. Only
`docs.aws.amazon.com` URLs are in scope.

## Fallback

The search API is an internal AWS endpoint and can change, and the script
builds itself with the Go toolchain on first use. If the script errors or
returns nothing, use `WebSearch` scoped to `site:docs.aws.amazon.com`, then
read the hits `.md`-first.

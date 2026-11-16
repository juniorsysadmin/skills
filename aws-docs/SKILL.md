---
name: aws-docs
description: >-
  Search and read official AWS documentation as clean markdown. Use whenever a
  question is about a specific AWS service, API, CLI, SDK, CloudFormation/CDK
  resource, quota, limit, or console behaviour — S3, Lambda, DynamoDB, IAM,
  EventBridge, CloudFront, Cognito, Step Functions, API Gateway, and the like.
  Prefer this over find-docs and over generic web search for anything on
  docs.aws.amazon.com, and use it even when you think you know the answer, since
  AWS API details, defaults, and limits change frequently.
license: Apache-2.0
---

# AWS Documentation

Search `docs.aws.amazon.com` and read pages as clean markdown. Two operations:
**search** (bundled script) and **read** (WebFetch, `.md`-first).

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

## Searching

```bash
"${CLAUDE_SKILL_DIR}/scripts/aws_docs" search "S3 access point policy" --limit 3
```

The command path is absolute and works from any directory. A harness that
does not expand the placeholder leaves a `${...}` token in the path: replace
the token with the directory that holds this SKILL.md.

`scripts/aws_docs` is a wrapper that auto-builds `aws_docs.bin` from the
committed Go source on first use (and whenever the source changes). It needs
only the Go toolchain.

Result links end in `.html` — apply the `.md` rewrite above before fetching.
End-to-end:

```bash
"${CLAUDE_SKILL_DIR}/scripts/aws_docs" search "CloudFront function size limit" --limit 3
# → 1. Quotas on CloudFront Functions
#      https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/cloudfront-function-restrictions.html
# then WebFetch …/cloudfront-function-restrictions.md
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

## Fallback

The search API is an internal AWS endpoint and can change. If the script
errors or returns nothing, use `WebSearch` scoped to
`site:docs.aws.amazon.com`, then read the hits with the `.md`-first method.

## Notes

- Never fabricate an AWS docs URL — search or navigate from a known page, then
  verify by reading.
- Trust the fetched `.md` over training-data recall for parameters, defaults,
  and limits.

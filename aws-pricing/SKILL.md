---
name: aws-pricing
description: >-
  Query live AWS pricing from the AWS Price List API. Use for ANY question
  about AWS service costs, prices, rates, or estimates — EC2 instance pricing,
  S3 storage classes, RDS, Lambda, DynamoDB, data transfer, cross-region
  price comparisons, and every other AWS service. Never answer an AWS pricing
  question from training data: prices change constantly and recalled numbers
  are wrong. Always fetch the current price with this skill first.
---

# AWS Pricing

One bundled script drives everything:

```bash
.claude/skills/aws-pricing/scripts/aws_pricing <subcommand> ...
```

`scripts/aws_pricing` is a wrapper that auto-builds the Rust binary from the
committed cargo project on first use (and whenever the source changes). It
needs only the Rust toolchain; the first build takes a few minutes. It needs
AWS credentials (`AWS_PROFILE` respected). The Pricing API lives only in a few
regions — the binary maps any region (e.g. ap-southeast-2) to the right
endpoint internally; the ambient `AWS_DEFAULT_REGION` never matters.

## The discovery chain

Never guess service codes, attribute names, or attribute values — discover
them, then query. Each step's `--filter` is a case-insensitive regex:

```bash
scripts=.claude/skills/aws-pricing/scripts
$scripts/aws_pricing service-codes --filter dynamo          # → AmazonDynamoDB
$scripts/aws_pricing service-attributes AmazonDynamoDB      # → group, regionCode, ...
$scripts/aws_pricing attribute-values AmazonDynamoDB group --filter 'group=capacity'
$scripts/aws_pricing pricing AmazonDynamoDB --region ap-southeast-2 \
  --filter group=DDB-WriteUnits --terms OnDemand
```

Skip earlier steps only when a command this session already showed the exact
value. Filter fields are the attribute names; filter values must match
attribute values exactly (except `--filter-contains`).

## `pricing` flags

| Flag | Filter type | Example |
|---|---|---|
| `--filter f=v` | EQUALS | `--filter instanceType=t3.micro` |
| `--filter-any f=v1,v2` | ANY_OF | `--filter-any instanceType=t3.micro,t3.small` |
| `--filter-contains f=s` | CONTAINS | `--filter-contains usagetype=DataTransfer` |
| `--filter-none f=v1,v2` | NONE_OF | `--filter-none tenancy=Dedicated,Host` |

All repeatable. `--region <code>` (repeatable) auto-adds a `regionCode`
filter — use it instead of the `location` attribute ("Asia Pacific (Sydney)").
AWS caps an ANY_OF/NONE_OF value list at 1024 characters. Other flags:
`--terms`, `--attrs`, `--exclude-free`, `--max-results` (1–100),
`--next-token`, `--max-chars`.

## Keep the output small

Output is compact JSON: `{"products":[...],"next_token":"..."}`.

- Big services (EC2, RDS, ElastiCache) return thousands of products. Always
  start with `--terms OnDemand` and tight `--filter`s. A well-filtered EC2
  query needs all of: `instanceType`, `operatingSystem`, `tenancy`,
  `preInstalledSw`, `capacitystatus`.
- Still too big: add `--attrs instanceType,location,memory,vcpu` and
  `--exclude-free`.
- The binary refuses output over `--max-chars` (default 100000) and prints
  narrowing hints — narrow the query; do not raise the cap.
- Pipe through `jq` to pull just the numbers, e.g.
  `| jq '.products[].terms.OnDemand[].priceDimensions[] | {unit, pricePerUnit, description}'`.
- A `next_token` in the response means more pages exist. Paginate with
  `--next-token` only when the question truly needs every product.

## Bulk price files

For whole-service dumps or historical prices too big for `pricing`:

```bash
$scripts/aws_pricing price-list-urls AmazonRDS ap-southeast-2
$scripts/aws_pricing price-list-urls AmazonEC2 us-east-1 --effective-date 2025-01-01T00:00:00Z
```

Returns `{"csv": url, "json": url}` download URLs. The CSV has 5 metadata
lines; the header is line 6. Prices are USD (CNY for `cn-*` regions).

## Rules

- Never state an AWS price from memory — only from a command run this session.
- Quote every price with its unit, currency, region, and term type
  (e.g. "USD 0.0136/hour, ap-southeast-2, OnDemand").
- Prices with `"beginRange" != "0"` are usage tiers — read `description` and
  `beginRange`/`endRange` before quoting.
- Credential errors: the script prints a hint; ask the user to reauthenticate
  (`aws login` / `aws sso login`).

## Cost reports

When the user wants a written cost estimate, structure it on
`references/COST_REPORT_TEMPLATE.md`:

- State the pricing model up front (default: OnDemand, no reserved capacity,
  no savings plans).
- List assumptions and exclusions explicitly — usage volumes, free tier,
  data transfer left out, etc.
- Show unit prices (fetched this session) before any calculation, then the
  calculation, then a table of cost vs usage level.
- End with cost optimisation recommendations (reserved/savings plans, tier
  thresholds, cheaper storage classes).

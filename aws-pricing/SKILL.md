---
name: aws-pricing
description: >-
  Live AWS prices from the Price List API. Use for any AWS cost question: a
  price, a comparison across regions or options, a cost estimate or report.
  Fetch before answering: recalled prices are stale.
license: Apache-2.0
---

# AWS Pricing

Binding rules, in force for the whole session:

1. Quote only prices a command printed this session.
2. Quote every price with unit + currency + region + term type, e.g.
   "USD 0.0136/hour, ap-southeast-2, OnDemand".
3. A price dimension with `"beginRange" != "0"` is a usage tier: read its
   `description` and `beginRange`/`endRange` before quoting it.
4. Discover service codes, attribute names, and attribute values with the
   chain below; reuse one only when a command this session already printed it.

## Command

```bash
scripts="${CLAUDE_SKILL_DIR}/scripts"
"$scripts/aws_pricing" <subcommand> ...
```

`scripts` is this skill's `scripts` directory as an absolute path. Shell state
does not persist between Bash calls: start every call with the `scripts=`
assignment, exactly as shown. A harness that does not expand the placeholder
leaves a `${...}` token in that assignment: replace the token with the
directory that holds this SKILL.md.

The first call builds the binary with the Go toolchain and is slow. The
command needs AWS credentials (`AWS_PROFILE` respected): on a credential
error, ask the user to reauthenticate (`aws login` / `aws sso login`).
`--region` alone picks the Price List API endpoint; the ambient
`AWS_DEFAULT_REGION` never matters.

## Discovery chain

Run in order. Every `--filter` here is a case-insensitive regex.

```bash
"$scripts/aws_pricing" service-codes --filter dynamo          # 1 → AmazonDynamoDB
"$scripts/aws_pricing" service-attributes AmazonDynamoDB      # 2 → group, regionCode, ...
"$scripts/aws_pricing" attribute-values AmazonDynamoDB group --filter 'group=units'  # 3 → DDB-WriteUnits, ...
"$scripts/aws_pricing" pricing AmazonDynamoDB --region ap-southeast-2 \
  --filter group=DDB-WriteUnits --terms OnDemand            # 4 → the price
```

Step 4 filter fields are step-2 attribute names; step-4 filter values must
equal step-3 values exactly (`--filter-contains` is the substring exception).

## `pricing` flags

| Flag | Filter type | Example |
|---|---|---|
| `--filter f=v` | EQUALS | `--filter instanceType=t3.micro` |
| `--filter-any f=v1,v2` | ANY_OF | `--filter-any instanceType=t3.micro,t3.small` |
| `--filter-contains f=s` | CONTAINS | `--filter-contains usagetype=DataTransfer` |
| `--filter-none f=v1,v2` | NONE_OF | `--filter-none tenancy=Dedicated,Host` |

All repeatable. `--region <code>` (repeatable) auto-adds a `regionCode`
filter — always it, never the `location` attribute. AWS caps an
ANY_OF/NONE_OF value list at 1024 characters.

## Output control

Output is compact JSON: `{"products":[...],"next_token":"..."}`.

- Big services (EC2, RDS, ElastiCache) hold thousands of products: always
  pass `--terms OnDemand` plus tight `--filter`s. A well-filtered EC2 query
  passes all of `instanceType`, `operatingSystem`, `tenancy`,
  `preInstalledSw`, `capacitystatus`.
- Still too big: add `--attrs instanceType,location,memory,vcpu` and
  `--exclude-free`.
- Output over `--max-chars` (default 100000) is refused with narrowing
  hints — narrow the query and keep the cap.
- Extract just the numbers:
  `| jq '.products[].terms.OnDemand[].priceDimensions[] | {unit, pricePerUnit, description}'`.
- A `next_token` in the response means more pages (`--max-results`, 1–100,
  sets the page size); paginate with `--next-token` only when the question
  needs every product.

## Bulk price files

For whole-service dumps or historical prices too big for `pricing`:

```bash
"$scripts/aws_pricing" price-list-urls AmazonRDS ap-southeast-2
"$scripts/aws_pricing" price-list-urls AmazonEC2 us-east-1 --effective-date 2025-01-01T00:00:00Z
```

Returns `{"csv": url, "json": url}` download URLs. The CSV holds 5 metadata
lines; the header is line 6. Prices are USD (CNY for `cn-*` regions).

## Cost reports

For a written cost estimate, read `references/COST_REPORT_TEMPLATE.md` and
produce every section it lists, in order.

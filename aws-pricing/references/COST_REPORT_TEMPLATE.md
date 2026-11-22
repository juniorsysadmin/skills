# Cost report spec

Produce these sections, in this order. A section is done only when it meets
its stated bar.

1. **Service overview** — one paragraph: the service, what the user runs on
   it, and the billing dimensions that drive this estimate.
2. **Pricing model** — state the model before any number: OnDemand
   pay-as-you-go, no reserved capacity, no savings plans, no caching or
   optimisation — unless the user specified otherwise, and then state what
   they specified.
3. **Assumptions** — every usage volume, region, and configuration the
   calculation depends on, each as one line with its value. A number used
   later but absent here is a defect.
4. **Exclusions** — what the estimate leaves out (data transfer, support
   plans, taxes, free tier if unused, cross-service costs), each with one
   clause on why.
5. **Unit prices** — a table of every unit price used, each quoted per the
   binding rules in SKILL.md. Unit prices appear before any arithmetic that
   uses them.
6. **Calculation** — the arithmetic from unit prices and assumptions to the
   total, shown so the reader can recompute it. Flag any usage tier
   (binding rule 3) and apply its boundaries explicitly.
7. **Free tier** — what the service's free tier covers and whether it changes
   the total; "no free tier" is a valid entry.
8. **Cost vs usage** — a table of the total at 3–5 usage levels bracketing
   the assumed level, with the cost drivers that dominate at each.
9. **Optimisation recommendations** — concrete actions ranked by saving:
   reserved/savings plans with break-even, tier thresholds worth crossing,
   cheaper storage classes or instance families, with the price deltas
   fetched this session where they anchor a claim.

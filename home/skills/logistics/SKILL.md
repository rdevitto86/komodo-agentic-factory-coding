---
name: logistics
description: Warehouse and fulfilment: stock levels, reorder points, pick paths, shipping cost.
user-invocable: false
---

# Logistics

## Inventory

- **Distinguish on-hand, available, and allocated.** Conflating them is the single most common inventory error — available is on-hand minus allocated minus safety stock.
- **Reorder point = lead-time demand + safety stock.** State the lead time and demand assumption you used; both drive the answer entirely.
- **Flag negative available quantities as a data defect**, never as a real state to plan around.
- **Age stock explicitly** where perishability or obsolescence applies.

## Fulfilment

- **Pick path before pick speed.** Travel time dominates; reslotting fast movers beats optimising the picker.
- **Batch by proximity, not by order.** Say plainly when order integrity constraints prevent this.
- **Name the constraint** when proposing a layout change — dock doors, aisle width, equipment reach.

## Shipping

- **Compare landed cost, not rate.** Rate plus surcharges plus dimensional weight plus insurance.
- **Dimensional weight beats actual weight** for anything light and bulky; check it before quoting.
- **State the service level** with every carrier comparison. A cheaper rate at a slower level is not a cheaper option.

## Output

Numbers with their assumptions attached. A reorder point without its lead time is not an answer.

| Metric | Value | Assumption |
|---|---|---|

# Skill: /stock-report

Generate an inventory stock report for a location, product category, or time period.

## Usage

```
/stock-report [--location <name>] [--category <name>] [--period <days>] [--format table|csv|summary]
```

- `--location` — Filter by warehouse or storage location name.
- `--category` — Filter by product category or SKU prefix.
- `--period` — Lookback period in days for movement data (default: 30).
- `--format` — Output format: `table` (Markdown table), `csv`, or `summary` (prose paragraph). Default: `table`.

---

## Before generating anything

Ask the user to provide or paste:
1. Current stock levels (SKU, description, on-hand quantity, unit of measure, location).
2. Movement data for the period: inbound receipts, outbound shipments, adjustments.
3. Reorder points and safety stock levels if available.
4. Any known stockouts or backorders to highlight.

---

## Report sections

### Snapshot (as of report date)

| SKU | Description | Location | On Hand | UOM | Reorder Point | Status |
|-----|-------------|----------|---------|-----|---------------|--------|
| ... | ... | ... | ... | ... | ... | OK / LOW / OUT |

Status logic:
- **OUT** — On hand = 0 or active backorder exists
- **LOW** — On hand ≤ reorder point
- **OK** — On hand > reorder point

---

### Movement summary (period)

| SKU | Opening Stock | Received | Shipped | Adjusted | Closing Stock | Turnover |
|-----|--------------|----------|---------|----------|---------------|---------|

Turnover = Shipped ÷ Average((Opening + Closing) / 2). Flag SKUs with turnover < 0.5 for the period as slow-moving.

---

### Alerts

List items requiring immediate action:
- **Stockouts** — SKU, description, days out of stock, open demand
- **Below reorder point** — SKU, current vs. reorder point, suggested order quantity
- **Slow-moving / dead stock** — SKU, on-hand value, last movement date
- **Discrepancies** — SKUs where system quantity differs from last physical count by >2%

---

### Recommendations

Based on the data, provide 3–5 prioritized actions (e.g. place PO for item X, investigate discrepancy on SKU Y, consider clearance on slow-moving Z).

---

## After generating

Remind the user to:
- Validate figures against the WMS or ERP before distributing the report.
- Schedule a cycle count for any SKUs with discrepancy alerts.
- Archive the report with the date in the filename for audit trail purposes.

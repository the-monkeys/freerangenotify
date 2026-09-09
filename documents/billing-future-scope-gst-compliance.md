# Business Billing Module — Future Scope: GST Compliance & Return Filing

> **Parent Document**: [business-billing-module-proposal.md](./business-billing-module-proposal.md)
> **Status**: Future scope — to be picked up after the core billing module is shipped.

---

## Overview

The core billing module (planned in the main proposal) handles GST-compliant **invoice generation** — correct CGST/SGST/IGST split, HSN codes, GSTIN on invoices, and e-invoicing via NIC portal.

However, Indian businesses also need help **tracking and filing GST returns**. This document covers the features needed to make FreeRangeNotify a one-stop solution where sellers can generate invoices AND file their GST returns without needing a separate tool like ClearTax, Zoho GST, or Tally.

---

## What the Core Module Already Covers

| Feature | Status |
|---------|--------|
| CGST/SGST/IGST auto-split based on customer state vs business state | ✅ Planned |
| HSN/SAC codes configurable per product | ✅ Planned |
| Business GSTIN stored per app, printed on invoices | ✅ Planned |
| Customer GSTIN on B2B invoices (via User model GSTIN field) | ✅ Planned |
| E-invoicing / IRN generation via NIC portal | ✅ Planned |
| Tax summary report (monthly, by rate and type) | ✅ Planned |
| TDS tracking on payments | ✅ Planned |
| Tax-inclusive pricing with back-calculation | ✅ Planned |
| Reverse charge flag on invoices | ✅ Planned |
| Tax exemption per user or product | ✅ Planned |

---

## What's Missing for Full GST Return Filing

### 1. GSTR-1 Export (Outward Supplies)

GSTR-1 is filed monthly or quarterly. It lists all sales invoices grouped into sections.

**Required sections:**

| Section | Description | Data Source |
|---------|-------------|-------------|
| **B2B** | Invoices to registered businesses (with buyer GSTIN) | Invoices where user has GSTIN |
| **B2C Large** | Invoices > ₹2.5 lakh to unregistered buyers (inter-state) | Invoices without GSTIN, amount > ₹2.5L, inter-state |
| **B2C Small** | All other B2C invoices | Invoices without GSTIN, below threshold |
| **Credit/Debit Notes** | Credit notes and debit notes issued | `frn_biz_credit_notes` index |
| **HSN Summary** | HSN-wise summary of all outward supplies | Aggregation from invoice line items |
| **Nil Rated / Exempt** | Supplies that are nil-rated, exempt, or non-GST | Invoices with `is_nil_rated = true` |
| **Export** | Export invoices (with or without payment of tax) | Invoices with `supply_type = export` |

**New API:**
```
GET /v1/biz/gst/gstr1?period=2026-07
```

Returns GSTR-1 data in the JSON format accepted by the GST portal. The business can download this and upload directly to the portal, or we can integrate with a GST Suvidha Provider (GSP) for direct filing.

### 2. GSTR-3B Summary (Monthly Tax Liability)

GSTR-3B is a simplified monthly return summarizing tax liability, input tax credit claimed, and net tax payable.

**Computed from:**
- Total outward supplies (from invoices)
- Total inward supplies (from purchases — see section 4 below)
- Tax collected (CGST + SGST + IGST from all invoices)
- Input Tax Credit available (tax paid on purchases)
- Net tax payable = Tax collected − ITC claimed

**New API:**
```
GET /v1/biz/gst/gstr3b?period=2026-07
```

### 3. HSN Summary Report

Required in GSTR-1 for businesses with annual turnover > ₹5 crore. Groups all outward supplies by HSN code.

**Fields per HSN:**
- HSN code
- Description
- UQC (Unit Quantity Code)
- Total quantity
- Taxable value
- IGST amount
- CGST amount
- SGST amount
- Cess amount

**New API:**
```
GET /v1/biz/gst/hsn-summary?period=2026-07
```

### 4. Purchase / Inward Supply Tracking (For ITC)

The core module's expense tracking is lightweight — it records expenses with amounts but does not capture tax breakdowns. For Input Tax Credit (ITC), the business needs to track purchase bills with full GST details.

**New index: `frn_biz_purchases`**

```json
{
  "mappings": {
    "properties": {
      "id":                { "type": "keyword" },
      "app_id":            { "type": "keyword" },
      "vendor_name":       { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "vendor_gstin":      { "type": "keyword" },
      "bill_number":       { "type": "keyword" },
      "bill_date":         { "type": "date" },
      "supply_type":       { "type": "keyword" },
      "is_reverse_charge": { "type": "boolean" },
      "line_items":        { "type": "nested", "properties": {
          "description":   { "type": "text" },
          "hsn_code":      { "type": "keyword" },
          "quantity":      { "type": "integer" },
          "unit_price":    { "type": "long" },
          "amount_paisa":  { "type": "long" },
          "tax_rate":      { "type": "integer" }
      }},
      "subtotal_paisa":    { "type": "long" },
      "cgst_paisa":        { "type": "long" },
      "sgst_paisa":        { "type": "long" },
      "igst_paisa":        { "type": "long" },
      "total_paisa":       { "type": "long" },
      "itc_eligible":      { "type": "boolean" },
      "itc_claimed":       { "type": "boolean" },
      "receipt_file_id":   { "type": "keyword" },
      "metadata":          { "type": "object", "enabled": false },
      "created_at":        { "type": "date" },
      "updated_at":        { "type": "date" }
    }
  }
}
```

**New APIs:**
```
POST /v1/biz/purchases            → Record purchase bill
GET  /v1/biz/purchases            → List purchases
GET  /v1/biz/purchases/:id        → Get purchase
PUT  /v1/biz/purchases/:id        → Update purchase
DELETE /v1/biz/purchases/:id      → Delete purchase
GET  /v1/biz/gst/itc-summary      → Input Tax Credit summary for a period
```

### 5. GSTR-2A / 2B Reconciliation

GSTR-2A and 2B are auto-populated by the GST portal from the seller's suppliers. The business needs to match their recorded purchases against this data.

**Two approaches:**

| Approach | How | Effort |
|----------|-----|--------|
| **Manual upload** | Business downloads GSTR-2B JSON from GST portal, uploads to FRN. System matches against `frn_biz_purchases`. | Medium |
| **GSP integration** | Connect to a GST Suvidha Provider (e.g., ClearTax API, Masters India) to auto-fetch 2B data. | High |

**New APIs:**
```
POST /v1/biz/gst/reconcile        → Upload GSTR-2B JSON and run matching
GET  /v1/biz/gst/reconcile/result  → View matched, unmatched, and mismatched entries
```

### 6. Additional Invoice Fields for GST Compliance

These fields need to be added to the `frn_biz_invoices` index:

| Field | Type | Purpose |
|-------|------|---------|
| `place_of_supply` | `keyword` | State code (e.g., "27" for Maharashtra). Required for services. |
| `supply_type` | `keyword` | `intra_state`, `inter_state`, `export`, `sez` |
| `is_reverse_charge` | `boolean` | Whether reverse charge applies |
| `is_nil_rated` | `boolean` | Whether the supply is nil-rated or exempt |
| `export_type` | `keyword` | `with_payment` or `without_payment` (for exports under LUT/bond) |
| `port_code` | `keyword` | Shipping port code (for export invoices) |
| `shipping_bill_number` | `keyword` | Shipping bill reference (for export invoices) |
| `shipping_bill_date` | `date` | Shipping bill date (for export invoices) |

### 7. B2B vs B2C Auto-Classification

The system should automatically classify invoices:
- **B2B**: User has a GSTIN set → invoice is reported in GSTR-1 Section B2B with buyer details
- **B2C Large**: No GSTIN + inter-state + invoice value > ₹2.5 lakh → reported in B2C Large section
- **B2C Small**: Everything else → aggregated in B2C Small section

This classification happens automatically based on user data and invoice amount. No manual input needed.

### 8. GST Dashboard

New dashboard page showing:

| Widget | Description |
|--------|-------------|
| Tax Liability Summary | CGST + SGST + IGST collected this month |
| ITC Available | Tax paid on eligible purchases |
| Net Payable | Tax collected minus ITC |
| GSTR-1 Status | Ready to file / Pending items |
| GSTR-3B Status | Auto-computed summary |
| Filing Calendar | Due dates for GSTR-1, GSTR-3B with reminders |

**Filing reminders**: Automatic notification to the business owner before GST filing deadlines (1st of month for GSTR-3B, 11th for GSTR-1). Uses existing notification engine.

---

## Implementation Priority

| Priority | Feature | Depends On |
|----------|---------|------------|
| P1 | Invoice fields (place_of_supply, supply_type, etc.) | Core billing module |
| P1 | B2B vs B2C auto-classification | Core billing module |
| P2 | GSTR-1 export API | P1 |
| P2 | HSN summary report | Core billing module |
| P2 | GSTR-3B summary API | P1 + Purchase tracking |
| P3 | Purchase / Inward supply tracking | Core billing module |
| P3 | ITC summary | Purchase tracking |
| P4 | GSTR-2A/2B reconciliation (manual upload) | Purchase tracking |
| P5 | GSP integration for direct filing | P2 |
| P5 | GSTR-2A/2B auto-fetch via GSP | P4 |

---

## Estimated Effort

| Scope | Weeks |
|-------|-------|
| Invoice fields + B2B/B2C classification | 1 |
| GSTR-1 export + HSN summary | 2 |
| Purchase tracking + ITC summary | 2 |
| GSTR-3B summary | 1 |
| Reconciliation (manual upload) | 2 |
| GST dashboard UI | 1 |
| GSP integration | 3 |
| **Total** | **~12 weeks** |

This can begin after Phase 3 of the core billing module (Payments & Tax) is complete, since it depends on the tax engine being functional.

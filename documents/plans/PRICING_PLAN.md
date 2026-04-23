# FreeRange Notify Pricing System (Credits-Based)

## 1. Overview
This document defines a margin-safe, usage-aligned pricing system for the FreeRange Notify platform using a credit-based model.

### Goals
- Maintain consistent gross margins
- Prevent abuse of high-cost channels
- Allow flexible usage across channels
- Offer simple pricing and predictable billing

## 2. Actual Cost Structure (Source of Truth)
Pricing must anchor to real costs:

| Channel | Cost Basis | Effective Cost |
| --- | --- | --- |
| Email | Per message | ₹0.03 / email |
| WhatsApp | Per message | ~₹1.08 / message |
| SMS | Per message | ~₹0.80 / SMS |
| In-app / Webhook / SSE | Infrastructure | ₹0.01 / event |

### Key Reality
- WhatsApp is ~36x more expensive than email.
- SMS is ~26.7x more expensive than email.
- In-app is ~3x cheaper than email.

Any pricing model that ignores this spread will break.

## 3. Credit System Design
Define a base unit:

- 1 credit = ₹0.01 (aligned to the cheapest infrastructure event)

## 4. Credit Cost Per Channel

| Channel | Cost (₹) | Credits |
| --- | --- | --- |
| In-app / Webhook / SSE | 0.01 | 1 |
| Email | 0.03 | 3 |
| SMS | 0.80 | 80 |
| WhatsApp | 1.08 | 108 |

### Rule
credits = cost / 0.01

### Why This Works
- No cross-subsidy
- No arbitrage
- Predictable margins

## 5. Margin Strategy
Target: 60-70% gross margin.

Sell credits at:

- ₹0.025 to ₹0.035 per credit (blended ~₹0.027)

### Example
- Cost per credit = ₹0.01
- Sell at ₹0.027 -> ~63% margin

## 6. Subscription Tiers (Credit Bundles)

### Free (Onboarding Only)
- Duration: 1 month only
- Credits: 500
- Hard caps: WhatsApp up to 2, SMS up to 3
- Goal: testing, not production

### Paid Tiers
| Tier | Price | Credits | Effective Cost Coverage | Notes |
| --- | --- | --- | --- | --- |
| Starter | ₹500 | 15,000 | ₹150 | Margin: ~70% |
| Pro | ₹1,499 | 55,000 | ₹550 | Margin: ~63% |
| Growth | ₹4,999 | 1,85,000 | ₹1,850 | Margin: ~63% |
| Scale | ₹14,999 | 5,50,000 | ₹5,500 | Margin: ~63% |

Credits are valid for 12 months. Unused credits expire.

## 7. Overage Pricing (High Margin Zone)
If subscription credits run out:

| Channel | Cost (₹) | Overage Charge (₹) |
| --- | --- | --- |
| In-app | 0.01 | 0.03 |
| Email | 0.03 | 0.06 |
| SMS | 0.80 | 1.50 |
| WhatsApp | 1.08 | 2.00 |

Overage = 1.7x-3x markup.

This is intentional.

## 8. Guardrails (Non-Negotiable)
- Daily caps on WhatsApp and SMS
- Rate limits per API key
- Spike detection (fraud / abuse)
- No negative credit balances without billing approval

## 9. Metrics to Track
You are blind without these metrics:

- Cost per user
- Revenue per user
- Credit burn distribution (by channel)
- Percentage of users hitting overage
- Gross margin per tier

# Pricing & Credits

FreeRangeNotify uses a **credit-based** pricing model. You buy a prepaid credit bundle for your workspace, then spend credits across any channel — email, SMS, WhatsApp, in-app, webhooks, and SSE — from one shared wallet.

> **Prepaid packs, not monthly quotas.** Each paid pack includes a fixed number of credits that stay valid for **12 months** from the date they are allocated. You are not locked into a recurring monthly send limit that resets every calendar month.

## Overview

Different delivery channels have very different carrier costs. WhatsApp is roughly **36×** more expensive per message than email, and SMS is roughly **27×** more expensive than email. In-app notifications and webhooks are the lightest.

Instead of selling separate quotas per channel, FreeRangeNotify uses **one credit pool per workspace**. Heavier channels burn more credits per send, so you always pay proportionally to real delivery cost — with no hidden cross-subsidy between channels.

**Benefits:**

- Mix channels freely from a single balance
- Predictable prepaid pricing
- Transparent burn rates before you send

See [plan packs on the pricing page](/pricing) or continue below for full rate details.

## How credits are consumed

Each successful delivery (or billable event) burns credits from your workspace wallet according to the channel:

| Channel | Credits per send |
| --- | ---: |
| In-app notification | 1 |
| Webhook | 1 |
| SSE (real-time stream) | 1 |
| Email | 3 |
| SMS | 80 |
| WhatsApp | 108 |

Credits are **reserved when a send starts** and **committed when delivery completes**. If a send fails before delivery, reserved credits are released back to your balance.

### Worked examples

| Scenario | Calculation | Credits used |
| --- | --- | ---: |
| 1,000 emails | 1,000 × 3 | 3,000 |
| 100 WhatsApp messages | 100 × 108 | 10,800 |
| 500 in-app notifications | 500 × 1 | 500 |
| Mixed: 2,000 emails + 50 WhatsApp | (2,000 × 3) + (50 × 108) | 11,400 |

**Starter pack (15,000 credits)** could cover approximately:

- ~5,000 emails only, or
- ~125 WhatsApp messages only, or
- any combination that totals 15,000 credits (e.g. 2,000 emails + 83 WhatsApp)

## Credit packs (plans)

Paid plans are **one-time prepaid credit packs** (plus GST). Prices shown are exclusive of 18% GST.

| Plan | Price (excl. GST) | Credits included | Approx. email capacity* |
| --- | ---: | ---: | ---: |
| **Free** | ₹0 | 500 | ~166 |
| **Starter** | ₹500 | 15,000 | ~5,000 |
| **Pro** | ₹1,499 | 55,000 | ~18,333 |
| **Growth** | ₹4,999 | 1,85,000 | ~61,666 |
| **Scale** | ₹14,999 | 5,50,000 | ~1,83,333 |
| **Enterprise** | Custom | Custom | Contact us |

\*Email capacity = credits ÷ 3. WhatsApp capacity ≈ credits ÷ 108.

Purchase packs from the [pricing page](/pricing) or upgrade from **Workspace → Billing** (`/billing`).

## Free tier

The free tier is for **onboarding and testing**, not high-volume production:

- **500 credits** included
- **All channel types** are available from day one
- **Daily caps** on expensive channels:
  - WhatsApp: **2 messages per day**
  - SMS: **3 messages per day**
- Full REST API and SDK access
- Community support

Example: 500 credits could send ~166 emails, or 500 in-app events, or a mix — subject to the WhatsApp/SMS daily caps above.

## Credit validity & expiry

- Credits are valid for **12 months** from the date they are allocated to your workspace.
- **Unused credits expire** at the end of that period.
- Buying an additional pack adds credits to your wallet; each allocation follows its own validity window per your subscription terms.

## Overage (when credits run out)

If your wallet balance reaches **zero**, further sends on **system credentials** (FreeRangeNotify-delivered email, SMS, WhatsApp) may be charged **per message** at overage rates until you top up:

| Channel | Overage charge (per message) |
| --- | ---: |
| In-app / webhook / SSE | ₹0.03 |
| Email | ₹0.06 |
| SMS | ₹1.50 |
| WhatsApp | ₹2.00 |

Overage is intentionally priced higher than included-credit usage to encourage buying another credit pack. Monitor your balance in **Billing** to avoid surprise overage.

> Overage only applies when included credits are exhausted. In-app, webhook, and SSE events on the platform channel typically have no external carrier cost but still consume credits from your pool while balance remains.

## Bring your own credentials (BYOC)

If you connect **your own** SMTP, WhatsApp Business, or SMS provider credentials, you pay your carrier directly. FreeRangeNotify charges a small **platform fee per message** instead of burning credits from your wallet for carrier cost:

| Channel | Platform fee (BYOC) |
| --- | ---: |
| Email | ₹0.02 / message |
| WhatsApp | ₹0.03 / message |
| SMS | ₹0.03 / message |
| Push / in-app / webhook / SSE | ₹0.00 |

Configure per-app provider settings as described in [Channels](/docs/channels).

## Your billing dashboard

Open **Workspace → Billing** (`/billing`) to track usage:

| UI label | Meaning |
| --- | --- |
| **Current License / Plan** | Active tier (Free, Starter, Pro, etc.) |
| **Credit Usage** | Credits consumed in the current period |
| **Total credits** | Credits allocated to the workspace |
| **Credits remaining** | Wallet balance still available |
| **Reserved (in flight)** | Credits held for sends not yet finalized |
| **Messages sent** | Total delivery count in the period |
| **Channel Usage Breakdown** | Per-channel messages, credits burned, and overage INR |

When billing metering is enabled for your deployment, the breakdown table shows exactly how each channel consumed credits and any overage charges.

## Guardrails & fair use

To protect the platform and keep pricing sustainable:

- **Daily caps** on WhatsApp and SMS for the free tier (see above)
- **Rate limits** per API key
- **Spike detection** for abuse and fraud
- No negative credit balances without billing approval

See the [Acceptable Use Policy](/acceptable-use) for messaging rules, especially for WhatsApp and SMS.

## Enterprise

For custom volume, contracts, SLAs, or invoicing:

**Email:** [monkeys.admin@monkeys.com.co](mailto:monkeys.admin@monkeys.com.co)

## Related links

- [Pricing page](/pricing) — compare packs and purchase
- [Getting Started](/docs/getting-started) — send your first notification
- [Channels](/docs/channels) — configure email, SMS, WhatsApp, and more
- [Billing dashboard](/billing) — live usage and credit balance

# Meta WhatsApp Cloud API — Setup & FRN Integration Guide

## Overview

This guide walks through creating a Meta (Facebook) App, getting WhatsApp Cloud API access, registering a production phone number, and integrating it into FreeRangeNotify as a provider.

**Phase 1**: System-level Meta WhatsApp provider (this guide)
**Phase 2**: User self-registration via Meta Embedded Signup (future)

---

## Part 1: Create a Meta Developer Account & App

### Step 1 — Meta Developer Account
1. Go to [developers.facebook.com](https://developers.facebook.com/)
2. Click **Get Started** (top-right)
3. Log in with your Facebook account (or create one)
4. Accept the Meta Developer terms
5. Verify your account (phone or email verification)

### Step 2 — Create a New App
1. From the Dashboard, click **Create App**
2. You'll see the app type selection screen:

   | Option | Choose? | Why |
   |--------|---------|-----|
   | **Business** | ✅ YES | This is the correct type for WhatsApp Cloud API |
   | Consumer | ❌ No | For social logins, not messaging APIs |
   | Gaming | ❌ No | For gaming-specific features |
   | None | ❌ No | Limited, no WhatsApp access |

3. Select **Business** and click **Next**
4. Fill in:
   - **App Name**: `FreeRangeNotify` (or your org name)
   - **Contact Email**: Your business email
   - **Business Account**: Select your Meta Business Account, or create one
5. Click **Create App**

### Step 3 — Add WhatsApp Product
1. On the app dashboard, scroll to **Add Products to Your App**
2. Find **WhatsApp** and click **Set Up**
3. This adds the WhatsApp product to your app and creates a **WhatsApp Business Account (WABA)** if you don't have one

You'll land on the **WhatsApp > Getting Started** page with:
- A temporary **test phone number** (Meta-provided sandbox number)
- Your **Phone Number ID**
- Your **WhatsApp Business Account ID (WABA ID)**
- A temporary **Access Token** (expires in 24 hours)

> **Save these values** — you'll need them for FRN configuration.

---

## Part 2: Register a Production Phone Number

### Option A — Use Your Own Number (Recommended)

#### Requirements
- The phone number **must not** already be registered with WhatsApp (personal or business)
- If it is, you must **delete WhatsApp** from that phone first
- The number must be able to receive SMS or voice calls for verification
- Landlines work (verification via voice call)

#### Steps
1. In the Meta Developer Dashboard, go to **WhatsApp > Getting Started**
2. Click **Add Phone Number**
3. Enter:
   - **Display Name**: Your business name (must comply with [Meta's display name guidelines](https://www.facebook.com/business/help/338047025702498))
   - **Phone Number**: Your business phone number with country code (e.g., `+1 208 418 9378`)
   - **Category**: Select your business category
4. Choose verification method: **SMS** or **Voice Call**
5. Enter the verification code you receive
6. Your number is now registered!

#### After Registration You Get:
- **Phone Number ID**: A numeric ID for this specific number (e.g., `123456789012345`)
- **WABA ID**: Your WhatsApp Business Account ID
- Both visible in **WhatsApp > Getting Started** dropdown

### Option B — Buy a Virtual Number
If you don't want to use an existing number:
- Get a number from Twilio, Vonage, or any VoIP provider
- The number just needs to receive SMS/voice for the one-time verification
- After verification, incoming calls/SMS to that number are irrelevant — all WhatsApp traffic goes through the Cloud API

---

## Part 3: Generate a Permanent Access Token

The default token from the Getting Started page **expires in 24 hours**. For production, you need a permanent (System User) token.

### Step 1 — Create a System User
1. Go to [business.facebook.com](https://business.facebook.com/)
2. Navigate to **Settings** (gear icon) → **Business Settings**
3. Under **Users**, click **System Users**
4. Click **Add** and create a new System User:
   - **Name**: `freerangenotify-api`
   - **Role**: **Admin** (needed for full WhatsApp API access)
5. Click **Create System User**

### Step 2 — Assign Assets to System User
1. Click on the system user you just created
2. Click **Add Assets**
3. Select **Apps** tab → find your `FreeRangeNotify` app → toggle **Full Control**
4. Click **Save Changes**

### Step 3 — Generate Permanent Token
1. Click **Generate New Token**
2. Select the **FreeRangeNotify** app
3. Select these permissions:
   - ✅ `whatsapp_business_management`
   - ✅ `whatsapp_business_messaging`
4. Click **Generate Token**
5. **COPY AND SAVE THIS TOKEN** — it's shown only once

> This token does **not expire** unless you revoke it. Store it securely (e.g., in your `.env` file or secrets manager).

The token looks like: `EAAGxxxxxxxxx...` (long string starting with `EAAG`)

---

## Part 4: Set Up Webhook for Delivery Status (Optional but Recommended)

Meta sends delivery status updates (sent, delivered, read, failed) via webhooks.

### Step 1 — Configure Webhook URL
1. In Meta Developer Dashboard → **WhatsApp > Configuration**
2. Under **Webhook**, click **Edit**
3. Set:
   - **Callback URL**: `https://your-domain.com/v1/webhooks/meta/whatsapp`
   - **Verify Token**: A random string you choose (e.g., `frn_meta_verify_token_2024`)
4. Click **Verify and Save**

> FRN will need a webhook endpoint to handle the verification challenge and status updates. This is implemented in Phase 2.

### Step 2 — Subscribe to Webhook Fields
After verification, subscribe to:
- ✅ `messages` — Incoming messages (if you need two-way chat)
- ✅ `message_status` — Delivery receipts (sent, delivered, read, failed)

---

## Part 5: Register WhatsApp in FRN

### Option 1 — Global Provider (System-level)

Add Meta WhatsApp credentials to your `.env` or `config.yaml`:

#### In `.env` file:
```env
# Meta WhatsApp Cloud API
META_WHATSAPP_ENABLED=true
META_WHATSAPP_PHONE_NUMBER_ID=123456789012345
META_WHATSAPP_WABA_ID=109876543210987
META_WHATSAPP_ACCESS_TOKEN=EAAGxxxxxxxxxxxxxxxxxxxxxxxxx
META_WHATSAPP_VERIFY_TOKEN=frn_meta_verify_token_2024
```

#### In `config.yaml` (providers section):
```yaml
providers:
  meta_whatsapp:
    enabled: true
    phone_number_id: "123456789012345"
    waba_id: "109876543210987"
    access_token: "EAAGxxxxxxxxxxxxxxxxxxxxxxxxx"
    api_version: "v23.0"
    timeout: 15
    max_retries: 3
```

### Option 2 — Per-App BYOC (Bring Your Own Credentials)

When creating or updating an app via the FRN API, include Meta WhatsApp config:

```json
PUT /v1/apps/{app_id}
{
  "settings": {
    "whatsapp_config": {
      "provider": "meta",
      "phone_number_id": "123456789012345",
      "waba_id": "109876543210987",
      "access_token": "EAAGxxxxxxxxxxxxxxxxxxxxxxxxx"
    }
  }
}
```

### Sending a WhatsApp Notification via FRN

Once configured, send WhatsApp notifications through the standard FRN API:

```json
POST /v1/notifications
{
  "app_id": "your-app-id",
  "user_id": "user-uuid",
  "channel": "whatsapp",
  "content": {
    "title": "Order Shipped",
    "body": "Your order #12345 has been shipped and will arrive by Friday."
  }
}
```

For template messages (required for sending to users who haven't messaged you first):

```json
POST /v1/notifications
{
  "app_id": "your-app-id",
  "user_id": "user-uuid",
  "channel": "whatsapp",
  "content": {
    "whatsapp_template": {
      "name": "order_update",
      "language": "en_US",
      "components": [
        {
          "type": "body",
          "parameters": [
            {"type": "text", "text": "#12345"},
            {"type": "text", "text": "Friday"}
          ]
        }
      ]
    }
  }
}
```

---

## Part 6: Meta WhatsApp Business Verification & Limits

### Message Tiers

| Tier | Daily Limit | How to Reach |
|------|-------------|--------------|
| Unverified | 250 business-initiated conversations | Default |
| Tier 1 | 1,000 conversations/day | Verify your Meta Business |
| Tier 2 | 10,000 conversations/day | Good quality rating + volume |
| Tier 3 | 100,000 conversations/day | Sustained quality + volume |
| Tier 4 | Unlimited | High volume + good quality |

### Business Verification (Required for Production)
1. Go to [business.facebook.com](https://business.facebook.com/) → **Settings** → **Security Center**
2. Click **Start Verification**
3. Provide:
   - Legal business name
   - Business address
   - Phone number
   - Business website
   - Tax ID or registration document
4. Upload verification documents (utility bill, certificate of incorporation, etc.)
5. Verification typically takes **2-7 business days**

> **Without Business Verification**: Limited to 250 business-initiated conversations/day and test phone number only. You MUST verify to use a production phone number at scale.

### Display Name Approval
- Your WhatsApp display name must match your business name
- Reviewed by Meta (usually within 24 hours)
- Guidelines: [Meta Display Name Policy](https://www.facebook.com/business/help/338047025702498)

---

## Part 7: Pricing (Meta Cloud API)

### Conversation-Based Pricing (per 24-hour window)

| Category | Description | India (INR) | US (USD) |
|----------|-------------|-------------|----------|
| **Utility** | Order updates, account alerts | ₹0.15 | $0.0050 |
| **Authentication** | OTP, login codes | ₹0.15 | $0.0085 |
| **Marketing** | Promotions, offers | ₹0.70 | $0.0250 |
| **Service** | User-initiated (replies) | FREE | FREE |

- First **1,000 service conversations/month** are **FREE**
- Business-initiated messages to users who haven't messaged you require **pre-approved templates**
- Pricing is per-conversation (24-hour window), not per-message

---

## Part 8: Testing Checklist

### Pre-Launch Verification
- [ ] Meta Developer Account created
- [ ] App created with **Business** type
- [ ] WhatsApp product added to app
- [ ] Phone number registered and verified
- [ ] System User created with permanent token
- [ ] Token has `whatsapp_business_messaging` permission
- [ ] FRN config updated with Meta credentials
- [ ] Test message sent successfully via FRN API
- [ ] Business Verification submitted (for production limits)

### Quick Test with curl (Direct to Meta API)
```bash
curl -X POST "https://graph.facebook.com/v23.0/{PHONE_NUMBER_ID}/messages" \
  -H "Authorization: Bearer {ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "messaging_product": "whatsapp",
    "to": "1XXXXXXXXXX",
    "type": "template",
    "template": {
      "name": "hello_world",
      "language": { "code": "en_US" }
    }
  }'
```

The `hello_world` template is pre-approved by Meta and available in every new WABA. Use it for your first test.

### Quick Test with FRN
```bash
curl -X POST http://localhost:8080/v1/notifications \
  -H "Authorization: Bearer {YOUR_JWT}" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "your-app-id",
    "user_id": "user-uuid-with-phone",
    "channel": "whatsapp",
    "content": {
      "title": "Test",
      "body": "Hello from FreeRangeNotify via Meta WhatsApp!"
    }
  }'
```

---

## Part 9: Common Pitfalls

| Pitfall | Solution |
|---------|----------|
| Token expires after 24h | Use System User permanent token (Part 3) |
| "Phone number already registered" | Delete WhatsApp from that phone, wait 5 min, retry |
| Messages not delivered | Check recipient has WhatsApp installed with that number |
| "Template not found" | Create & get template approved in Meta Business Manager |
| 250 message limit | Complete Business Verification (Part 6) |
| Webhook verification fails | Ensure verify token matches and endpoint returns the challenge |
| "Unsupported message type" | Business-initiated (outside 24h window) requires templates, not free-text |

---

## Phase 2 Preview: User Self-Registration (Embedded Signup)

In Phase 2, FRN tenants will be able to connect their own WhatsApp number through the FRN UI using **Meta's Embedded Signup** flow:

1. Tenant clicks "Connect WhatsApp" in FRN settings
2. FRN opens Meta's Embedded Signup OAuth flow
3. Tenant logs in with their Facebook account
4. Selects/creates a WhatsApp Business Account
5. Registers their phone number
6. FRN receives the OAuth token + WABA ID + Phone Number ID via callback
7. Credentials auto-saved as per-app BYOC config

This eliminates manual credential copying and streamlines onboarding. Implementation requires:
- Meta Tech Provider registration
- OAuth callback endpoint in FRN
- Embedded Signup JS SDK in the React UI
- Token refresh automation

---

## Reference: Key Meta API Endpoints

| Endpoint | Purpose |
|----------|---------|
| `POST /v23.0/{phone_number_id}/messages` | Send messages |
| `GET /v23.0/{waba_id}/message_templates` | List templates |
| `POST /v23.0/{waba_id}/message_templates` | Create template |
| `GET /v23.0/{phone_number_id}` | Get phone number details |
| `GET /v23.0/{waba_id}/phone_numbers` | List registered numbers |
| `POST /v23.0/{phone_number_id}/register` | Register phone number |

**Base URL**: `https://graph.facebook.com`
**Auth**: Bearer token in `Authorization` header
**Docs**: [Meta WhatsApp Cloud API](https://developers.facebook.com/docs/whatsapp/cloud-api)


_________________________________________________________________________XXXXXXXXX_______________________________________________________
Now I have enough context. Here's what you need for each of the three permissions in Meta's "Allowed usage" review:

1. whatsapp_business_management
Business Description:

We build FreeRange Notify, a multi-channel notification platform. Our customers (businesses, SaaS apps, NGOs) use our platform to send transactional and operational notifications to their end-users across email, SMS, push, and WhatsApp. We need this permission so we can manage our WhatsApp Business Account programmatically — things like reading phone number details, checking messaging limits, and managing message templates through our backend instead of doing it manually in the Meta dashboard every time.

How your app uses this permission:

When a customer sets up WhatsApp as a delivery channel in our platform, we use whatsapp_business_management to pull their WABA details (phone number ID, display name, quality rating, messaging limits) and sync that info into our system. We also use it to create, update, and check the approval status of message templates so our customers can manage their templates from our dashboard without switching to the Meta Business Manager. No end-user data is shared outside the delivery flow.

2. whatsapp_business_manage_events
Business Description:

FreeRange Notify delivers notifications on behalf of businesses. When we send a WhatsApp message through the Cloud API, we need to receive delivery status updates (sent, delivered, read, failed) so we can show accurate delivery reports to our customers and trigger retry logic when messages fail.

How your app uses this permission:

We subscribe to webhook events from the WhatsApp Cloud API to track message delivery statuses. When a message is sent, delivered, read, or fails, the webhook fires and we update the notification status in our database. This powers our delivery analytics dashboard and lets us retry failed messages automatically. We also use it to detect when a user replies (for conversation window tracking), so we know whether to use a template or a free-form message.

3. whatsapp_business_messaging
Business Description:

We're a notification infrastructure company. Our platform lets businesses send WhatsApp messages to their customers — things like order confirmations, appointment reminders, OTP codes, shipping updates, and payment receipts. We handle the API integration so our customers don't have to build it themselves.

How your app uses this permission:

This is the core permission we need. When a business triggers a notification through our API (e.g., "send order shipped alert to user X on WhatsApp"), our worker process picks it up from the queue, renders the message template with the right variables, and calls the WhatsApp Cloud API to send it to the recipient's phone number. We only send to users who have opted in through our customer's application. Every message uses a Meta-approved template for business-initiated conversations. We track the delivery status and surface it in our dashboard.

These are written to match how a real engineer or founder would describe the product to a reviewer — direct, specific, no filler. Copy them as-is into the Business Description and permission description fields.
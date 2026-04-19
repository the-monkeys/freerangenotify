# WhatsApp Meta Tech Provider - End-to-End Testing Guide

## Table of Contents

1. [Prerequisites and Setup](#1-prerequisites-and-setup)
2. [Feature Overview and Importance](#2-feature-overview-and-importance)
3. [Testing via UI](#3-testing-via-ui)
4. [Testing via API (curl)](#4-testing-via-api-curl)
5. [Test Data](#5-test-data)
6. [Feature-by-Feature E2E Walkthrough](#6-feature-by-feature-e2e-walkthrough)
7. [Simulating Meta Webhooks Locally](#7-simulating-meta-webhooks-locally)
8. [Production Testing with Real Meta Account](#8-production-testing-with-real-meta-account)

---

## 1. Prerequisites and Setup

### Start the Full Stack

```bash
cd C:\Users\Dave\the_monkeys\FreeRangeNotify
docker-compose up --build
```

This starts:

| Service              | Port  | Purpose                              |
|----------------------|-------|--------------------------------------|
| Elasticsearch        | 9200  | Data store for messages, templates   |
| Redis                | 6379  | Queue, caching, CSW tracking         |
| notification-service | 8080  | API server                           |
| notification-worker  | -     | Processes notification queue          |
| ui                   | 3000  | React dashboard                      |

### Or Run Go Natively (faster iteration)

```bash
# Terminal 1: Infrastructure only
docker-compose up elasticsearch redis

# Terminal 2: API Server
set FREERANGE_FEATURES_WHATSAPP_META_ENABLED=true
set FREERANGE_PROVIDERS_META_WHATSAPP_ENABLED=true
set FREERANGE_PROVIDERS_META_WHATSAPP_WEBHOOK_VERIFY=test123
go run ./cmd/server

# Terminal 3: Worker
set FREERANGE_FEATURES_WHATSAPP_META_ENABLED=true
set FREERANGE_PROVIDERS_META_WHATSAPP_ENABLED=true
go run ./cmd/worker

# Terminal 4: UI
cd ui && npm run dev
```

### Create a Test Account

1. Open http://localhost:3000
2. Click Register, create an account
3. Create a new application (e.g. "WhatsApp Test App")
4. Note the App ID and API Key from the overview tab

### Variables Used in This Guide

```
JWT_TOKEN=<your_jwt_from_login>
APP_ID=<your_app_id>
API_KEY=<your_app_api_key>
BASE=http://localhost:8080/v1
```

---

## 2. Feature Overview and Importance

### All 14 Core WhatsApp Meta Features

| #  | Feature                        | Importance | Why It Matters                                                                |
|----|-------------------------------|------------|-------------------------------------------------------------------------------|
| 1  | Embedded Signup (OAuth)        | CRITICAL   | One-click WABA onboarding. Without this, customers must manually configure API keys. This is how Tech Providers differentiate with zero-friction setup. |
| 2  | Connection Status              | HIGH       | Shows whether WhatsApp is active, phone number, quality rating. Gives users confidence their integration is working. |
| 3  | Disconnect                     | HIGH       | Clean teardown of Meta connection. Required for compliance and account migration. |
| 4  | Webhook Subscription           | CRITICAL   | Subscribes WABA to receive inbound messages and delivery statuses. Without this, no two-way communication is possible. |
| 5  | Inbound Message Receiving      | CRITICAL   | Receives text, image, video, audio, document, location, contacts, and interactive messages from customers. The foundation of two-way WhatsApp communication. |
| 6  | Delivery Status Tracking       | HIGH       | Tracks sent to delivered to read status for every outbound message. Provides delivery assurance and analytics. |
| 7  | Customer Service Window (CSW)  | CRITICAL   | Tracks the 24-hour free-reply window. Within CSW: send free-form text. Outside: must use a paid template. Directly impacts messaging costs. |
| 8  | Template Management (CRUD)     | CRITICAL   | Create, list, get, delete Meta-approved templates. Templates are the ONLY way to initiate conversations or reply outside the 24-hour window. |
| 9  | Template Sync                  | MEDIUM     | Force-refresh template approval status from Meta. Useful when waiting for template review. |
| 10 | Template Status Webhooks       | MEDIUM     | Auto-receives template approval/rejection events from Meta via webhook. Enables real-time status updates in the UI via SSE. |
| 11 | Conversation Inbox             | HIGH       | Lists all WhatsApp conversations grouped by contact. Shows last message, unread count, CSW status. The primary UI for managing customer communications. |
| 12 | Message History                | HIGH       | Full chat thread per contact with timestamps, direction, status badges. Essential for context when replying to customers. |
| 13 | Reply (CSW-aware)              | CRITICAL   | Send replies to customers. Automatically detects if CSW is open (free-form text) or closed (requires template). Prevents accidental billing. |
| 14 | Read Receipts                  | MEDIUM     | Sends Meta read receipts so customers see blue checkmarks. Professional touch that builds trust. |

### Rich Outbound Message Types (Phase 5)

| #  | Message Type           | Importance | Use Case                                                |
|----|------------------------|-----------|----------------------------------------------------------|
| 15 | Text Messages          | CRITICAL  | Basic text with optional URL preview                      |
| 16 | Template Messages      | CRITICAL  | Pre-approved messages with variables (e.g. order confirmation) |
| 17 | Image Messages         | HIGH      | Product photos, receipts, QR codes                        |
| 18 | Video Messages         | MEDIUM    | Tutorial clips, product demos                             |
| 19 | Audio Messages         | MEDIUM    | Voice notes, automated greetings                          |
| 20 | Document Messages      | HIGH      | PDFs, invoices, booking confirmations                     |
| 21 | Sticker Messages       | LOW       | Branded stickers for engagement                           |
| 22 | Location Messages      | MEDIUM    | Store locations, delivery pickup points                   |
| 23 | Contact Cards          | MEDIUM    | Share support agent contact details                       |
| 24 | Reaction Messages      | LOW       | React to customer messages with emoji                     |
| 25 | Interactive Buttons    | HIGH      | Quick reply buttons (up to 3), call-to-action buttons     |
| 26 | Interactive Lists      | HIGH      | Menu with sections and rows (up to 10), product catalogs  |

---

## 3. Testing via UI

### 3.1 Navigate to WhatsApp Tab

1. Go to http://localhost:3000
2. Login then click on your application
3. In the left sidebar, click WhatsApp (under Configuration group)

You will see three sections stacked vertically:

### 3.2 WhatsApp Connect Section

What you see without Meta credentials:
- Status: "Not Connected"
- Setup instructions (4 steps)
- "Connect with Meta" button (requires real Meta App credentials)

What you see with Meta connected:
- Green "Connected" badge
- Phone number, WABA ID, quality rating, connected date
- "Subscribe Webhooks" button
- "Disconnect" button (red)

### 3.3 WhatsApp Templates Section

What you see:
- "New Template" button that opens a creation form
- Search bar for filtering templates
- Template cards showing: name, language, status badge (Approved/Pending/Rejected), category
- Per-template actions: Sync from Meta, Delete

To test the creation form (UI only):
1. Click "New Template"
2. Fill in: Name: order_update, Category: Utility, Language: en_US
3. Body: Hi {{1}}, your order {{2}} has been shipped. Track at {{3}}
4. Click "Submit for Approval"
5. Expected: Error toast (since no real Meta connection) confirming the UI to API pipeline is wired

### 3.4 WhatsApp Conversations Section

What you see (empty state):
- "No conversations yet" message

After simulating inbound messages (see Section 7):
- Contact list with avatar, name, last message, timestamp
- "Window Open" badge if CSW is active
- Unread count badges
- Click a contact to open the chat thread
- Chat thread shows WhatsApp-style bubbles (green=outbound, gray=inbound)
- Reply input box at bottom
- "Mark Read" button in header

### 3.5 Settings Tab - WhatsApp Channel

1. Go to Settings tab
2. Open "WhatsApp Channel Configuration" accordion
3. Blue banner: "For Meta WhatsApp Business, use the WhatsApp tab instead"
4. Below: existing Twilio config fields (Account SID, Auth Token, From Number)

---

## 4. Testing via API (curl)

### 4.1 Authenticate

```bash
# Register
curl -X POST http://localhost:8080/v1/auth/register ^
  -H "Content-Type: application/json" ^
  -d "{\"email\":\"test@example.com\",\"password\":\"Test1234!\",\"full_name\":\"Test User\"}"

# Login (save access_token as JWT_TOKEN)
curl -X POST http://localhost:8080/v1/auth/login ^
  -H "Content-Type: application/json" ^
  -d "{\"email\":\"test@example.com\",\"password\":\"Test1234!\"}"
```

### 4.2 Create an Application

```bash
curl -X POST http://localhost:8080/v1/apps/ ^
  -H "Authorization: Bearer %JWT_TOKEN%" ^
  -H "Content-Type: application/json" ^
  -d "{\"app_name\":\"WhatsApp E2E Test\",\"description\":\"Testing WhatsApp Meta features\"}"
```

Save app_id as APP_ID and api_key as API_KEY from the response.

### 4.3 Check WhatsApp Connection Status

```bash
curl -H "Authorization: Bearer %JWT_TOKEN%" ^
  http://localhost:8080/v1/admin/whatsapp/%APP_ID%/status
```

Expected Response (not connected):

```json
{
  "success": true,
  "data": {
    "connected": false,
    "provider": "twilio",
    "message": "WhatsApp Meta not configured for this app"
  }
}
```

### 4.4 Manually Set Meta WhatsApp Config (Simulate Embedded Signup)

For local testing without a real Meta account, directly update the app settings:

```bash
curl -X PUT http://localhost:8080/v1/apps/%APP_ID%/settings ^
  -H "Authorization: Bearer %JWT_TOKEN%" ^
  -H "Content-Type: application/json" ^
  -d "{\"whatsapp_config\":{\"provider\":\"meta\",\"meta_phone_number_id\":\"100000000000001\",\"meta_waba_id\":\"200000000000001\",\"meta_access_token\":\"FAKE_TOKEN_FOR_LOCAL_TESTING\",\"meta_business_id\":\"300000000000001\",\"connection_status\":\"connected\",\"connected_at\":\"2026-03-12T10:00:00Z\",\"display_phone_number\":\"+1 555-0123\",\"quality_rating\":\"GREEN\"}}"
```

### 4.5 Verify Connection Status After Config

```bash
curl -H "Authorization: Bearer %JWT_TOKEN%" ^
  http://localhost:8080/v1/admin/whatsapp/%APP_ID%/status
```

Expected Response:

```json
{
  "success": true,
  "data": {
    "connected": true,
    "provider": "meta",
    "connection_status": "connected",
    "connected_at": "2026-03-12T10:00:00Z",
    "phone_number_id": "100000000000001",
    "waba_id": "200000000000001",
    "display_phone": "+1 555-0123",
    "quality_rating": "GREEN",
    "business_id": "300000000000001"
  }
}
```

### 4.6 List Conversations (empty)

```bash
curl -H "X-API-Key: %API_KEY%" ^
  http://localhost:8080/v1/whatsapp/conversations/
```

Expected:

```json
{
  "success": true,
  "data": [],
  "total": 0,
  "limit": 50,
  "offset": 0
}
```

### 4.7 Disconnect WhatsApp

```bash
curl -X POST -H "Authorization: Bearer %JWT_TOKEN%" ^
  http://localhost:8080/v1/admin/whatsapp/%APP_ID%/disconnect
```

Expected:

```json
{"success": true, "message": "WhatsApp disconnected"}
```

---

## 5. Test Data

### 5.1 Inbound Text Message

```json
{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "200000000000001",
    "changes": [{
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {
          "phone_number_id": "100000000000001",
          "display_phone_number": "+15550123"
        },
        "contacts": [{
          "wa_id": "919876543210",
          "profile": {"name": "Priya Sharma"}
        }],
        "messages": [{
          "id": "wamid.priya_text_001",
          "from": "919876543210",
          "timestamp": "1710244800",
          "type": "text",
          "text": {"body": "Hi, I need help with my order #ORD-2026-4521"}
        }]
      },
      "field": "messages"
    }]
  }]
}
```

### 5.2 Inbound Image Message

```json
{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "200000000000001",
    "changes": [{
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {
          "phone_number_id": "100000000000001",
          "display_phone_number": "+15550123"
        },
        "contacts": [{
          "wa_id": "919876543210",
          "profile": {"name": "Priya Sharma"}
        }],
        "messages": [{
          "id": "wamid.priya_img_001",
          "from": "919876543210",
          "timestamp": "1710244860",
          "type": "image",
          "image": {
            "id": "media_id_12345",
            "mime_type": "image/jpeg",
            "caption": "Here is the damaged product photo"
          }
        }]
      },
      "field": "messages"
    }]
  }]
}
```

### 5.3 Inbound Location Message

```json
{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "200000000000001",
    "changes": [{
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {
          "phone_number_id": "100000000000001",
          "display_phone_number": "+15550123"
        },
        "contacts": [{
          "wa_id": "917001234567",
          "profile": {"name": "Rahul Verma"}
        }],
        "messages": [{
          "id": "wamid.rahul_loc_001",
          "from": "917001234567",
          "timestamp": "1710245100",
          "type": "location",
          "location": {
            "latitude": 28.6139,
            "longitude": 77.2090,
            "name": "India Gate",
            "address": "Rajpath, New Delhi, India"
          }
        }]
      },
      "field": "messages"
    }]
  }]
}
```

### 5.4 Inbound Document Message

```json
{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "200000000000001",
    "changes": [{
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {
          "phone_number_id": "100000000000001",
          "display_phone_number": "+15550123"
        },
        "contacts": [{
          "wa_id": "447700900001",
          "profile": {"name": "James Wilson"}
        }],
        "messages": [{
          "id": "wamid.james_doc_001",
          "from": "447700900001",
          "timestamp": "1710245200",
          "type": "document",
          "document": {
            "id": "media_id_67890",
            "mime_type": "application/pdf",
            "filename": "invoice_2026_March.pdf",
            "caption": "Please check this invoice"
          }
        }]
      },
      "field": "messages"
    }]
  }]
}
```

### 5.5 Delivery Status Update

```json
{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "200000000000001",
    "changes": [{
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {
          "phone_number_id": "100000000000001",
          "display_phone_number": "+15550123"
        },
        "statuses": [{
          "id": "wamid.outbound_msg_001",
          "status": "delivered",
          "timestamp": "1710245000",
          "recipient_id": "919876543210",
          "conversation": {
            "id": "conv_001",
            "origin": {"type": "business_initiated"}
          },
          "pricing": {
            "billable": true,
            "pricing_model": "CBP",
            "category": "utility"
          }
        }]
      },
      "field": "messages"
    }]
  }]
}
```

### 5.6 Template Status Webhook

```json
{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "200000000000001",
    "changes": [{
      "value": {
        "event": "APPROVED",
        "message_template_id": 111222333,
        "message_template_name": "order_update",
        "message_template_language": "en_US"
      },
      "field": "message_template_status_update"
    }]
  }]
}
```

### 5.7 Multi-Contact Conversation Data

Use these three payloads in sequence to populate the inbox with realistic conversations:

Contact 1 - Priya Sharma (Customer Support):

```json
{
  "object": "whatsapp_business_account",
  "entry": [{"id": "200000000000001", "changes": [{"value": {
    "messaging_product": "whatsapp",
    "metadata": {"phone_number_id": "100000000000001", "display_phone_number": "+15550123"},
    "contacts": [{"wa_id": "919876543210", "profile": {"name": "Priya Sharma"}}],
    "messages": [
      {"id": "wamid.priya_001", "from": "919876543210", "timestamp": "1710244800", "type": "text", "text": {"body": "Hi, I need help with my order #ORD-2026-4521"}},
      {"id": "wamid.priya_002", "from": "919876543210", "timestamp": "1710244860", "type": "text", "text": {"body": "It has not been delivered yet and its been 5 days"}}
    ]
  }, "field": "messages"}]}]
}
```

Contact 2 - Rahul Verma (Delivery Location):

```json
{
  "object": "whatsapp_business_account",
  "entry": [{"id": "200000000000001", "changes": [{"value": {
    "messaging_product": "whatsapp",
    "metadata": {"phone_number_id": "100000000000001", "display_phone_number": "+15550123"},
    "contacts": [{"wa_id": "917001234567", "profile": {"name": "Rahul Verma"}}],
    "messages": [
      {"id": "wamid.rahul_001", "from": "917001234567", "timestamp": "1710245100", "type": "text", "text": {"body": "Where is your nearest store?"}}
    ]
  }, "field": "messages"}]}]
}
```

Contact 3 - James Wilson (Invoice Request):

```json
{
  "object": "whatsapp_business_account",
  "entry": [{"id": "200000000000001", "changes": [{"value": {
    "messaging_product": "whatsapp",
    "metadata": {"phone_number_id": "100000000000001", "display_phone_number": "+15550123"},
    "contacts": [{"wa_id": "447700900001", "profile": {"name": "James Wilson"}}],
    "messages": [
      {"id": "wamid.james_001", "from": "447700900001", "timestamp": "1710245200", "type": "text", "text": {"body": "Can you send me the invoice for last month?"}}
    ]
  }, "field": "messages"}]}]
}
```

### 5.8 Template Creation Payloads

Utility Template - Order Update:

```json
{
  "name": "order_update",
  "category": "UTILITY",
  "language": "en_US",
  "components": [
    {"type": "BODY", "text": "Hi {{1}}, your order {{2}} has been shipped! Track it here: {{3}}"}
  ]
}
```

Marketing Template - Promotion:

```json
{
  "name": "weekly_promo",
  "category": "MARKETING",
  "language": "en_US",
  "components": [
    {"type": "HEADER", "format": "TEXT", "text": "Special Offer Just For You!"},
    {"type": "BODY", "text": "Hi {{1}}, enjoy {{2}}% off on all products this week! Use code: {{3}}. Offer valid until {{4}}."},
    {"type": "FOOTER", "text": "Reply STOP to unsubscribe"}
  ]
}
```

Authentication Template - OTP:

```json
{
  "name": "login_otp",
  "category": "AUTHENTICATION",
  "language": "en_US",
  "components": [
    {"type": "BODY", "text": "Your verification code is {{1}}. This code expires in 5 minutes. Do not share it with anyone."}
  ]
}
```

### 5.9 Reply Payloads

Free-form text reply (within CSW):

```json
{"text": "Hi Priya! Let me check your order #ORD-2026-4521. One moment please."}
```

Template reply (outside CSW):

```json
{"template_name": "order_update"}
```

---

## 6. Feature-by-Feature E2E Walkthrough

### Test 1: Connection Status Check

UI Path: App then WhatsApp tab then WhatsApp Connect card

API:

```bash
curl -H "Authorization: Bearer %JWT_TOKEN%" %BASE%/admin/whatsapp/%APP_ID%/status
```

Pass Criteria:
- [ ] Returns connected: false for unconfigured apps
- [ ] Returns full status with phone, WABA ID, quality rating for connected apps
- [ ] UI shows the correct status card with all details

---

### Test 2: Simulate Embedded Signup (Manual Config)

API: Set Meta config directly via settings (see Section 4.4)

Pass Criteria:
- [ ] Settings saved successfully
- [ ] Status endpoint now returns connected: true
- [ ] UI WhatsApp tab refreshes to show connected state with phone number and WABA ID

---

### Test 3: Receive Inbound Text Message

API: POST the webhook payload from Section 5.1:

```bash
curl -X POST %BASE%/webhooks/meta/whatsapp ^
  -H "Content-Type: application/json" ^
  -d @test_webhook_text.json
```

Note: If app_secret is set in config, you need to either clear it for testing or compute the HMAC signature. To skip signature verification, set app_secret to empty string in config.

Pass Criteria:
- [ ] Returns HTTP 200
- [ ] Message appears in GET /v1/whatsapp/conversations/ list
- [ ] Message appears in GET /v1/whatsapp/conversations/919876543210/messages
- [ ] UI Conversations section shows "Priya Sharma" with the message preview
- [ ] Click Priya then chat shows the order help message

---

### Test 4: Receive Multiple Message Types

Send payloads from Sections 5.2 (image), 5.3 (location), 5.4 (document).

Pass Criteria:
- [ ] Each message stored with correct message_type
- [ ] Image messages show caption text in chat
- [ ] Location messages show location name
- [ ] Document messages show filename
- [ ] Multiple contacts appear as separate conversations in the inbox

---

### Test 5: Conversation Inbox

UI Path: App then WhatsApp tab then WhatsApp Inbox section

API:

```bash
curl -H "X-API-Key: %API_KEY%" "%BASE%/whatsapp/conversations/?limit=50&offset=0"
```

Pass Criteria:
- [ ] Lists all unique contacts who sent messages
- [ ] Each conversation shows: contact name, last message text, timestamp
- [ ] csw_open is true if last inbound was less than 24 hours ago
- [ ] unread_count shows count of unread inbound messages
- [ ] UI renders contact list with avatars, badges, and timestamps

---

### Test 6: Message History Thread

UI Path: Click a contact in the inbox to open the chat thread

API:

```bash
curl -H "X-API-Key: %API_KEY%" "%BASE%/whatsapp/conversations/919876543210/messages?limit=50"
```

Pass Criteria:
- [ ] Returns all messages for the contact (inbound and outbound)
- [ ] Messages ordered by timestamp
- [ ] Each message has: id, direction, message_type, body, timestamp, status
- [ ] UI shows messages in WhatsApp-style bubbles (gray for inbound, green for outbound)
- [ ] Timestamps displayed correctly

---

### Test 7: Reply Within Customer Service Window (CSW)

UI Path: In chat thread then type reply then click Send

API:

```bash
curl -X POST -H "X-API-Key: %API_KEY%" ^
  -H "Content-Type: application/json" ^
  "%BASE%/whatsapp/conversations/919876543210/reply" ^
  -d "{\"text\":\"Hi Priya! Let me check your order status.\"}"
```

Pass Criteria:
- [ ] Returns {"success": true, "message": "Reply sent"}
- [ ] If CSW is open (less than 24h since last inbound): sends as free-form text
- [ ] If CSW is closed: returns error asking for template_name
- [ ] Reply appears in message history as outbound
- [ ] UI updates the chat thread with the sent message

---

### Test 8: Reply Outside CSW (Template Required)

API:

```bash
curl -X POST -H "X-API-Key: %API_KEY%" ^
  -H "Content-Type: application/json" ^
  "%BASE%/whatsapp/conversations/919876543210/reply" ^
  -d "{\"template_name\":\"order_update\"}"
```

Pass Criteria:
- [ ] Sends template-based message when CSW is closed
- [ ] Returns error if neither text nor template_name provided
- [ ] Returns error if text provided but CSW is closed

---

### Test 9: Mark Messages as Read

UI Path: In chat thread then click "Mark Read" button

API:

```bash
curl -X POST -H "X-API-Key: %API_KEY%" ^
  "%BASE%/whatsapp/conversations/919876543210/read"
```

Pass Criteria:
- [ ] Returns {"success": true, "message": "Messages marked as read"}
- [ ] Sends read receipt to Meta (customer sees blue checkmarks)
- [ ] Unread count decreases in conversation list

---

### Test 10: Delivery Status Tracking

Send the delivery status webhook (Section 5.5):

```bash
curl -X POST %BASE%/webhooks/meta/whatsapp ^
  -H "Content-Type: application/json" ^
  -d @test_webhook_status.json
```

Pass Criteria:
- [ ] Returns HTTP 200
- [ ] If meta_message_id matches an outbound notification, updates its status
- [ ] Status progression: sent then delivered then read
- [ ] Billing info (billable, pricing_category) stored in notification metadata

---

### Test 11: Template Management

Create Template:

```bash
curl -X POST -H "X-API-Key: %API_KEY%" ^
  -H "Content-Type: application/json" ^
  "%BASE%/whatsapp/templates/" ^
  -d "{\"name\":\"order_update\",\"category\":\"UTILITY\",\"language\":\"en_US\",\"components\":[{\"type\":\"BODY\",\"text\":\"Hi {{1}}, your order {{2}} has been shipped!\"}]}"
```

List Templates:

```bash
curl -H "X-API-Key: %API_KEY%" "%BASE%/whatsapp/templates/"
```

Get Template:

```bash
curl -H "X-API-Key: %API_KEY%" "%BASE%/whatsapp/templates/order_update"
```

Sync Template:

```bash
curl -X POST -H "X-API-Key: %API_KEY%" "%BASE%/whatsapp/templates/order_update/sync"
```

Delete Template:

```bash
curl -X DELETE -H "X-API-Key: %API_KEY%" "%BASE%/whatsapp/templates/order_update"
```

Pass Criteria:
- [ ] Create returns 201 Created with Meta template ID
- [ ] List returns all templates with status (APPROVED/PENDING/REJECTED)
- [ ] Get returns single template details
- [ ] Sync refreshes status from Meta and publishes SSE event
- [ ] Delete removes template from Meta
- [ ] UI template cards reflect correct statuses and categories

Note: All template operations call Meta Graph API. With a fake token, they will return Meta API errors. This confirms the wiring is correct.

---

### Test 12: Template Status Webhook

Send the template status webhook (Section 5.6):

```bash
curl -X POST %BASE%/webhooks/meta/whatsapp ^
  -H "Content-Type: application/json" ^
  -d @test_webhook_template_status.json
```

Pass Criteria:
- [ ] Returns HTTP 200
- [ ] Logs "Template status webhook received" with template name and status
- [ ] Publishes SSE event for real-time UI update

---

### Test 13: Disconnect WhatsApp

UI Path: WhatsApp tab then WhatsApp Connect then "Disconnect" button

API:

```bash
curl -X POST -H "Authorization: Bearer %JWT_TOKEN%" ^
  "%BASE%/admin/whatsapp/%APP_ID%/disconnect"
```

Pass Criteria:
- [ ] Returns success
- [ ] connection_status changes to disconnected
- [ ] Access token cleared
- [ ] UI switches to "Not Connected" state
- [ ] Template and conversation APIs return "not configured" errors

---

### Test 14: Webhook Verification (Meta Subscription)

```bash
curl "%BASE%/webhooks/meta/whatsapp?hub.mode=subscribe&hub.verify_token=test123&hub.challenge=challenge_abc"
```

Pass Criteria:
- [ ] Returns challenge_abc with HTTP 200
- [ ] Wrong token returns HTTP 403

---

## 7. Simulating Meta Webhooks Locally

### Step-by-Step: Populate Inbox Without a Real Meta Account

Step 1: Configure app with fake Meta credentials (Section 4.4)

Step 2: Disable signature verification. Set in config/config.yaml:

```yaml
providers:
  meta_whatsapp:
    app_secret: ""
```

Step 3: Fire all test webhooks using PowerShell:

```powershell
# Contact 1: Priya Sharma
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/v1/webhooks/meta/whatsapp" -ContentType "application/json" -Body '{"object":"whatsapp_business_account","entry":[{"id":"200000000000001","changes":[{"value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"100000000000001","display_phone_number":"+15550123"},"contacts":[{"wa_id":"919876543210","profile":{"name":"Priya Sharma"}}],"messages":[{"id":"wamid.priya_001","from":"919876543210","timestamp":"1710244800","type":"text","text":{"body":"Hi, I need help with my order #ORD-2026-4521"}},{"id":"wamid.priya_002","from":"919876543210","timestamp":"1710244860","type":"text","text":{"body":"It has not been delivered yet"}}]},"field":"messages"}]}]}'

# Contact 2: Rahul Verma
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/v1/webhooks/meta/whatsapp" -ContentType "application/json" -Body '{"object":"whatsapp_business_account","entry":[{"id":"200000000000001","changes":[{"value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"100000000000001","display_phone_number":"+15550123"},"contacts":[{"wa_id":"917001234567","profile":{"name":"Rahul Verma"}}],"messages":[{"id":"wamid.rahul_001","from":"917001234567","timestamp":"1710245100","type":"text","text":{"body":"Where is your nearest store?"}}]},"field":"messages"}]}]}'

# Contact 3: James Wilson
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/v1/webhooks/meta/whatsapp" -ContentType "application/json" -Body '{"object":"whatsapp_business_account","entry":[{"id":"200000000000001","changes":[{"value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"100000000000001","display_phone_number":"+15550123"},"contacts":[{"wa_id":"447700900001","profile":{"name":"James Wilson"}}],"messages":[{"id":"wamid.james_001","from":"447700900001","timestamp":"1710245200","type":"text","text":{"body":"Can you send me the invoice for last month?"}}]},"field":"messages"}]}]}'
```

Step 4: Verify in UI:
- Go to WhatsApp tab then Inbox section then click Refresh
- You should see 3 conversations: Priya, Rahul, James
- Click each to see their messages

Step 5: Test reply:
- Click on Priya Sharma
- Type "Let me look into this for you" then click Send
- Expected: toast "Reply sent" (or error if Meta API rejects the fake token, which confirms wiring)

---

## 8. Production Testing with Real Meta Account

### Setup Requirements

1. Meta Business Manager account at business.facebook.com
2. Meta Developer App at developers.facebook.com
3. Enable WhatsApp Business Messaging product in the app
4. Note your: App ID, App Secret, Test Phone Number ID, WABA ID, Access Token

### Configuration

```yaml
providers:
  meta_whatsapp:
    enabled: true
    phone_number_id: "<your_phone_number_id>"
    waba_id: "<your_waba_id>"
    access_token: "<your_permanent_access_token>"
    api_version: "v23.0"
    app_secret: "<your_app_secret>"
    webhook_verify: "<your_verify_token>"
    meta_app_id: "<your_facebook_app_id>"
    meta_app_secret: "<your_facebook_app_secret>"

server:
  public_url: "https://<your-ngrok-url>"
```

### Expose Local Server

```bash
ngrok http 8080
```

### Configure Meta Webhook

1. Go to Meta Developer Dashboard then WhatsApp then Configuration
2. Set Callback URL: https://your-ngrok-url/v1/webhooks/meta/whatsapp
3. Set Verify Token: same as webhook_verify in config
4. Subscribe to: messages, message_template_status_update

### Real Test Flow

1. Send a message from your personal WhatsApp to the test phone number
2. Verify it appears in the UI inbox
3. Reply from the UI
4. Verify the reply arrives on your WhatsApp
5. Create a template in the UI then verify it appears in Meta Business Manager
6. Wait for template approval then verify status webhook updates the UI

---

## API Route Reference

| Method | Route                                            | Auth       | Feature                  |
|--------|--------------------------------------------------|------------|--------------------------|
| GET    | /v1/admin/whatsapp/:app_id/status                | JWT        | Connection status        |
| POST   | /v1/admin/whatsapp/connect                       | JWT        | Embedded Signup          |
| POST   | /v1/admin/whatsapp/:app_id/disconnect            | JWT        | Disconnect               |
| POST   | /v1/admin/whatsapp/:app_id/subscribe-webhooks    | JWT        | Subscribe webhooks       |
| GET    | /v1/webhooks/meta/whatsapp                       | Public     | Webhook verification     |
| POST   | /v1/webhooks/meta/whatsapp                       | Signature  | Inbound messages/status  |
| POST   | /v1/whatsapp/templates/                          | API Key    | Create template          |
| GET    | /v1/whatsapp/templates/                          | API Key    | List templates           |
| GET    | /v1/whatsapp/templates/:name                     | API Key    | Get template             |
| DELETE | /v1/whatsapp/templates/:name                     | API Key    | Delete template          |
| POST   | /v1/whatsapp/templates/:name/sync                | API Key    | Sync template status     |
| GET    | /v1/whatsapp/conversations/                      | API Key    | List conversations       |
| GET    | /v1/whatsapp/conversations/:contact_id/messages  | API Key    | Message history          |
| POST   | /v1/whatsapp/conversations/:contact_id/reply     | API Key    | Send reply               |
| POST   | /v1/whatsapp/conversations/:contact_id/read      | API Key    | Mark read                |

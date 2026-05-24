# Meta App Review — Video Scripts for Quick Approval

For: FreeRangeNotify (Tech Provider app `828282083012160`)
Targets: `whatsapp_business_messaging` + `whatsapp_business_management` (Advanced access)

Meta needs **two short screen recordings** (no voiceover required; on-screen text is fine).
Keep each video **45–90 seconds**. Do everything in **one continuous take per video** — no cuts.
Use a clean Chrome window in incognito so the URL bar is uncluttered.

---

## Pre-recording checklist (do this once, then record both videos back to back)

1. Local stack is up: `docker compose up -d` and ngrok is running.
2. UI is open at `https://www.freerangenotify.com/` (or your ngrok URL — must be HTTPS).
3. You are logged in as an admin user.
4. The WABA is **already connected** (Connected badge visible in the WhatsApp tab). The reviewer is not approving the connect flow — they are approving the *send* and *manage templates* flows.
5. You have a second device with WhatsApp installed, ready to receive the test message. Use a number that is **registered as a test recipient** in App Dashboard → WhatsApp → API Setup (so you don't need an approved template to send).
6. Recording tool: OBS, Loom, or Windows Game Bar (`Win+G`). Resolution 1280×720 or higher.

---

## Video 1 — “Send a message from the app and receive it on a phone”

**Duration target: 60–75 seconds.**

### Script (what you do on screen, in order)

| # | Action on screen | On-screen caption to add (optional) |
|---|------------------|-------------------------------------|
| 1 | Show the browser URL bar so reviewer sees the FRN dashboard URL. | *“FreeRangeNotify dashboard — Tech Provider app for WhatsApp.”* |
| 2 | Click **Applications** → click into **Release Test App**. | *“Select a tenant application.”* |
| 3 | Click the **WhatsApp** tab. Pause 1 second on the green “Connected” card so the reviewer can read the **WABA ID** and **Phone** clearly. | *“This tenant is connected to a real Meta WABA.”* |
| 4 | Click **Quick Send** in the sidebar (or the “Send Message” button in the WhatsApp tab — whichever is visible). | — |
| 5 | In the recipient field, type the **test phone number on your second device** (E.164 format, e.g. `+919876543210`). | — |
| 6 | In the message body, type: `Hello from FreeRangeNotify App Review test`. | *“Composing a freeform session message.”* |
| 7 | Click **Send**. Show the success toast / delivery status changing from `queued` → `sent` → `delivered`. | *“Message sent via Meta WhatsApp Cloud API.”* |
| 8 | **Tab away from the browser to your phone screen** (or place your phone in front of the webcam). Show the WhatsApp chat where the message just arrived, with the **timestamp matching the send time**. Hold the phone steady for 3 seconds. | *“Message received in WhatsApp client.”* |
| 9 | (Optional but strong) Reply from the phone with `Got it 👍`. Tab back to the browser, open **Inbox**, show the inbound message appearing. | *“Two-way messaging working via webhook.”* |

### Why this satisfies the requirement
Meta's [Tech Provider checklist](https://developers.facebook.com/documentation/business-messaging/whatsapp/solution-providers/get-started-for-tech-providers) says verbatim:

> The first video must show a message created and sent from your app and received in the WhatsApp client (mobile app or web app).

Steps 5–8 cover exactly that. Step 9 is bonus credit that demonstrates inbound webhook handling, which strengthens the case for `whatsapp_business_messaging` Advanced access.

---

## Video 2 — “Create a message template from the app”

**Duration target: 45–60 seconds.**

### Script

| # | Action on screen | On-screen caption to add (optional) |
|---|------------------|-------------------------------------|
| 1 | Still inside **Release Test App** → **WhatsApp** tab. Scroll to **WhatsApp Rich Templates** (the section we built — *not* the Twilio one). | *“Authoring a Meta WhatsApp message template.”* |
| 2 | Click **+ New Template** (or whichever button opens the builder). | — |
| 3 | Fill these fields in front of the camera (type slowly so the reviewer can read): <br>• **Name:** `review_demo_welcome` <br>• **Language:** `en_US` <br>• **Category:** `MARKETING` <br>• **Kind:** `Coupon Code` <br>• **Body:** `Hi {{1}}, welcome to {{2}}. Use code {{3}} for 10% off.` <br>• **Coupon code:** `WELCOME10` | *“Filling out template content with three variables and a copy-code button.”* |
| 4 | Click **Create** (or **Submit for Approval**). Wait for the toast. | *“Template created in FRN and submitted to Meta Graph API.”* |
| 5 | The template appears in the list with a **PENDING / In Review** badge from Meta and the Meta template ID visible. Hover/click to expand and show the Meta binding (`meta_template_id`, `category`, `language`). Hold for 3 seconds. | *“Template registered with Meta WABA and awaiting approval.”* |
| 6 | (Optional) Open a second tab to https://business.facebook.com/wa/manage/message-templates/ and show the same template name appearing in **WhatsApp Manager** with status “In review”. This is the strongest possible evidence. | *“Same template visible in Meta’s WhatsApp Manager — proves our API call hit the real WABA.”* |

### Why this satisfies the requirement
Meta says:

> The second video must show your app being used to create a message template.

Steps 3–5 satisfy it directly. Step 6 is the optional "screen recording of WhatsApp Manager" alternative — including it inside the same video doubles the proof and removes any reviewer doubt that the call was real.

---

## Voiceover script (optional — use only if you want to narrate)

Keep it neutral, factual, no marketing language. Read in a normal speaking pace.

> "This is FreeRangeNotify, a Tech Provider application for the WhatsApp Business Platform. In this clip I am sending a session message from our dashboard to a test WhatsApp number. The message reaches the WhatsApp client on the receiving device. In the second clip I create a Marketing-category message template with a copy-code button, which is submitted to the WhatsApp Business Account via the Meta Graph API and appears in WhatsApp Manager pending approval."

That single paragraph is enough for both videos combined.

---

## Submission form — text answers Meta will ask for

When you click **Submit App Review**, Meta asks for one paragraph per permission. Use these.

### `whatsapp_business_messaging`

> FreeRangeNotify uses `whatsapp_business_messaging` to send session and template messages on behalf of our onboarded business customers via the Cloud API endpoint `/{phone_number_id}/messages`. The first attached video shows a user composing and sending a message from our dashboard which is delivered to a real WhatsApp client. We also use this permission to receive inbound message webhooks (`messages`, `statuses`) and surface them in the customer's Inbox.

### `whatsapp_business_management`

> FreeRangeNotify uses `whatsapp_business_management` to create, list, and delete message templates on behalf of our onboarded business customers via `/{waba_id}/message_templates`, and to subscribe our public webhook callback URL via `/{waba_id}/subscribed_apps`. The second attached video shows a user authoring a `MARKETING` template with three variables and a `COPY_CODE` button, which is submitted to the customer's WABA and appears in WhatsApp Manager pending approval.

### App icon, privacy policy, terms of service (Basic settings)

| Field | Value |
|-------|-------|
| App icon | 1024×1024 PNG of your FRN logo on solid background |
| Privacy policy URL | https://www.freerangenotify.com/privacy |
| Terms of service URL | https://www.freerangenotify.com/terms |
| Category | Business |
| Data deletion URL | https://www.freerangenotify.com/data-deletion |

If any of those URLs don't exist yet, **create them before submitting** — Meta auto-rejects on day one if they 404.

---

## Common rejection reasons and how to avoid them

1. **Video is too short or cuts mid-action.** → 45 sec minimum, one continuous take.
2. **Reviewer can't see the message arrive on the phone.** → Always pan/cut to the phone for at least 3 seconds with the timestamp visible.
3. **You demoed on a Tester account but the app is in Live mode.** → Don't matter, but make sure the receiving number is registered as a test recipient *or* the template you use is already approved.
4. **The template video shows authoring without actually submitting to Meta.** → Always click Submit and show the PENDING state. Bonus: open WhatsApp Manager in a second tab.
5. **App is missing Business Verification.** → Verify the business *first*; App Review will not start otherwise.
6. **Privacy policy URL 404s.** → Half of all first-attempt rejections.

---

## After approval

Once both permissions are approved:

1. Meta App Dashboard → top header → flip **App Mode** to **Live**.
2. Webhook callback URL must be a stable HTTPS endpoint (not ngrok). Move to your prod domain.
3. Your `/v1/admin/whatsapp/connect` Embedded Signup flow now works for any Facebook user, not just app testers.
4. Remove the "Use a System User access token instead" UI fallback if you don't want self-serve customers to see it (it still works for self-hosted deployments).

Estimated turnaround: **2–5 business days** for a clean submission, longer if your business verification is also pending.

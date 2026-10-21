# Channels

Create an application in the dashboard, then use its API key from your backend. The app ID identifies the application; it is not a credential. See [Authentication](/docs/authentication) and [Notifications](/docs/notifications) for the common send/status contract. HTTP 202 means queued, not delivered. Inspect the returned notification ID with `GET /v1/notifications/{id}`.

## Email and custom SMTP

In your application's Settings, configure Email with the SMTP host, port, username, password and sender required by your mail service. TLS and verified senders must match that service. Credentials belong to that application; another application's SMTP configuration does not configure yours. A provider authentication failure requires correcting credentials; retrying will not repair them. The operator can also configure shared email providers. [Attachments](/docs/file-attachments) describes ordinary files and inline CID images supported by the email adapters.

## SMS

Add a phone number in E.164 format (for example `+14155551234`) to the recipient. Configure an SMS connection in application Settings using an enabled provider such as Twilio or Exotel, including its sender and account credentials. The operator must enable that provider. Country registration, sender approval, destination restrictions and provider balance still apply. Long or Unicode texts can use multiple billable segments. Use your deployment's billing rates and the provider's current charges; there is no universal per-message price. OTP can use a direct phone recipient; ordinary sends use the registered user.

## WhatsApp

Choose Meta Cloud API or Twilio in the application's WhatsApp connection. Meta requires the business/WhatsApp account, phone-number ID, token, callback setup and approved templates. Twilio requires the account credentials, approved WhatsApp sender and applicable ContentSid. Saving a connection does not grant provider approval.

Free-form text, media and lists require an open customer-service window. Outside that window use an approved template with the matching language and parameters. Stored rich templates must belong to the same app/environment and selected provider account. Approved CTA and quick-reply delivery preserves authored text/image/video/document headers; numbered runtime variables also fill text header parameters. Lists remain window-bound; carousel/coupon authoring and approval are provider-specific. A generic notification `poll` does not implement WhatsApp voting.

For Meta template delivery, pass `data.whatsapp_template` with `name`, `language`, and `components`. Runtime media and interactions use the existing `whatsapp_media` and `whatsapp_interactive` data fields; they are not email attachment arrays. Use the WhatsApp dashboard to create/submit supported rich templates and inspect approval state before sending. [OTP](/docs/otp) shows an approved authentication-template request.

## Webhooks, push, inbox and real-time

[Webhooks](/docs/webhooks) covers generic receivers and registered Slack, Teams and Discord renderers. Push requires operator-enabled APNS/FCM credentials and a registered device token for the user. Device permission, token validity and platform payload limits apply.

`in_app` persists inbox notifications; `sse` delivers real-time events to a connected subscriber. Existing routes, event names and token exchange remain supported. See [In-App](/docs/in-app) and [SSE](/docs/sse). Neither channel requires SMTP or WhatsApp credentials. Delivery depends on connected clients and queue/provider capacity; acceptance is not an instant-delivery guarantee.

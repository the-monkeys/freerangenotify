package seed

import (
	_ "embed"

	"github.com/the-monkeys/freerangenotify/internal/domain/template"
)

//go:embed newsletter_editorial.html
var newsletterEditorialHTML string

//go:embed newsletter_product_launch.html
var newsletterProductLaunchHTML string

//go:embed newsletter_changelog.html
var newsletterChangelogHTML string

//go:embed newsletter_event_invitation.html
var newsletterEventInvitationHTML string

//go:embed newsletter_weekly_roundup.html
var newsletterWeeklyRoundupHTML string

//go:embed newsletter_community_spotlight.html
var newsletterCommunitySpotlightHTML string

//go:embed services_accounting_bookkeeping.html
var servicesAccountingHTML string

//go:embed newsletter_webinar_briefing.html
var newsletterWebinarBriefingHTML string

// LibraryTemplates are pre-built templates users can clone into their apps.
var LibraryTemplates = []template.Template{
	{
		Name:        "welcome_email",
		Description: "Welcome email for new user signups",
		Channel:     "email",
		Subject:     "Welcome to {{.product}}, {{.name}}!",
		Body: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h1 style="color: #333;">Welcome, {{.name}}!</h1>
  <p>Thank you for joining {{.product}}. We're excited to have you on board.</p>
  <a href="{{.cta_url}}" style="display: inline-block; padding: 12px 24px; background: #4F46E5; color: white; text-decoration: none; border-radius: 6px;">Get Started</a>
</div>`,
		Variables: []string{"name", "product", "cta_url"},
		Locale:    "en",
		Status:    "active",
		Metadata:  map[string]interface{}{
			"category": "transactional",
			"sample_data": map[string]interface{}{
				"name": "Alex",
				"product": "FreeRangeNotify",
				"cta_url": "https://freerangenotify.com",
			},
		},
	},
	{
		Name:        "password_reset",
		Description: "Password reset request with OTP code",
		Channel:     "email",
		Subject:     "Reset Your Password",
		Body: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2>Password Reset</h2>
  <p>Hi {{.name}}, you requested a password reset. Use this code:</p>
  <div style="font-size: 32px; font-weight: bold; text-align: center; padding: 20px; background: #f5f5f5; border-radius: 8px; letter-spacing: 8px;">{{.code}}</div>
  <p style="color: #666; font-size: 14px;">This code expires in {{.expiry_minutes}} minutes.</p>
</div>`,
		Variables: []string{"name", "code", "expiry_minutes"},
		Locale:    "en",
		Status:    "active",
		Metadata:  map[string]interface{}{
			"category": "transactional",
			"sample_data": map[string]interface{}{
				"name": "Alex",
				"code": "482910",
				"expiry_minutes": "15",
			},
		},
	},
	{
		Name:        "order_confirmation",
		Description: "Order confirmation with order details",
		Channel:     "email",
		Subject:     "Order Confirmed: #{{.order_id}}",
		Body: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2>Order Confirmed</h2>
  <p>Hi {{.name}}, your order <strong>#{{.order_id}}</strong> has been confirmed.</p>
  <p>Total: <strong>{{.total}}</strong></p>
  <p>Estimated delivery: {{.delivery_date}}</p>
</div>`,
		Variables: []string{"name", "order_id", "total", "delivery_date"},
		Locale:    "en",
		Status:    "active",
		Metadata:  map[string]interface{}{
			"category": "transactional",
			"sample_data": map[string]interface{}{
				"name": "Alex",
				"order_id": "ORD-73829",
				"total": "$145.00",
				"delivery_date": "Oct 12, 2025",
			},
		},
	},
	{
		Name:        "push_alert",
		Description: "Generic push notification alert",
		Channel:     "push",
		Subject:     "{{.title}}",
		Body:        "{{.message}}",
		Variables:   []string{"title", "message"},
		Locale:      "en",
		Status:      "active",
		Metadata:    map[string]interface{}{
			"category": "notification",
			"sample_data": map[string]interface{}{
				"title": "System Update",
				"message": "Your system has been successfully updated.",
			},
		},
	},
	{
		Name:        "sms_verification",
		Description: "SMS verification code",
		Channel:     "sms",
		Body:        "Your verification code is {{.code}}. Expires in {{.expiry}} minutes.",
		Variables:   []string{"code", "expiry"},
		Locale:      "en",
		Status:      "active",
		Metadata:    map[string]interface{}{
			"category": "transactional",
			"sample_data": map[string]interface{}{
				"code": "849201",
				"expiry": "10",
			},
		},
	},
	{
        Name:        "appointment_reminder",
        Description: "Reminder about an upcoming appointment",
        Channel:     "email",
        Subject:     "Reminder: appointment on {{.date}}",
        Body:        `<p>Dear {{.name}},<br/>this is a reminder for your appointment on {{.date}} at {{.time}} ({{.location}}).</p>`,
        Variables:   []string{"name", "date", "time", "location"},
        Locale:      "en",
        Status:      "active",
        Metadata:    map[string]interface{}{
			"category": "notification",
			"sample_data": map[string]interface{}{
				"name": "Alex",
				"date": "Nov 15, 2025",
				"time": "14:30",
				"location": "Main Office",
			},
		},
    },
    {
        Name:        "booking_confirmation",
        Description: "Confirmation of a booking",
        Channel:     "email",
        Subject:     "Your booking is confirmed – {{.booking_id}}",
        Body:        `<p>Thanks {{.name}}, your booking for {{.service}} on {{.date}} at {{.time}} is confirmed.</p>`,
        Variables:   []string{"name", "booking_id", "service", "date", "time"},
        Locale:      "en",
        Status:      "active",
        Metadata:    map[string]interface{}{
			"category": "transactional",
			"sample_data": map[string]interface{}{
				"name": "Alex",
				"booking_id": "BK-90210",
				"service": "Consultation",
				"date": "Dec 01, 2025",
				"time": "09:00",
			},
		},
    },
    {
        Name:        "maintenance_notice",
        Description: "Alert that system maintenance is scheduled",
        Channel:     "email",
        Subject:     "Scheduled maintenance: {{.start_time}} – {{.end_time}}",
        Body:        `<p>We will be performing maintenance from {{.start_time}} to {{.end_time}}. Services may be unavailable.</p>`,
        Variables:   []string{"start_time", "end_time"},
        Locale:      "en",
        Status:      "active",
        Metadata:    map[string]interface{}{
			"category": "notification",
			"sample_data": map[string]interface{}{
				"start_time": "Saturday 2am UTC",
				"end_time": "Saturday 4am UTC",
			},
		},
    },
	{
		Name:        "webhook_release_note",
		Description: "Release note webhook notification with product and version details",
		Channel:     "webhook",
		Subject:     "Release Note: {{.product}} {{.version}}",
		Body:        `{"event": "release_note", "product": "{{.product}}", "version": "{{.version}}", "summary": "{{.summary}}", "timestamp": "{{.timestamp}}"}`,
		Variables:   []string{"product", "version", "summary", "timestamp"},
		Locale:      "en",
		Status:      "active",
		Metadata:    map[string]interface{}{
			"category": "notification",
			"sample_data": map[string]interface{}{
				"product": "FreeRangeNotify",
				"version": "v2.0.1",
				"summary": "Bug fixes and performance improvements",
				"timestamp": "2025-06-15T12:00:00Z",
			},
		},
	},
	{
		Name:        "sse_realtime",
		Description: "Real-time browser notification via SSE",
		Channel:     "sse",
		Subject:     "{{.title}}",
		Body:        "{{.message}}",
		Variables:   []string{"title", "message"},
		Locale:      "en",
		Status:      "active",
		Metadata:    map[string]interface{}{
			"category": "notification",
			"sample_data": map[string]interface{}{
				"title": "New Message",
				"message": "You have a new direct message from Jane.",
			},
		},
	},
	{
		Name:        "monkeys_weekly_digest",
		Description: "Weekly/daily top stories newsletter for monkeys.com.co with featured articles, article list, and FreeRangeNotify footer",
		Channel:     "email",
		Subject:     "{{.digest_title}} — Top Stories from Monkeys",
		Body: `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>{{.digest_title}}</title>
</head>
<body style="margin:0;padding:0;background-color:#0d0d0d;font-family:'Segoe UI',Arial,Helvetica,sans-serif;color:#e0e0e0;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#0d0d0d;">
<tr><td align="center" style="padding:20px 10px;">
<table role="presentation" width="640" cellpadding="0" cellspacing="0" style="max-width:640px;width:100%;">

  <!-- Header -->
  <tr><td style="padding:24px 32px;background-color:#161616;border-radius:12px 12px 0 0;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr>
        <td><span style="font-size:24px;font-weight:700;color:#ffffff;letter-spacing:-0.5px;">🐵 Monkeys</span></td>
        <td align="right"><span style="font-size:13px;color:#888;">{{.date_range}}</span></td>
      </tr>
    </table>
  </td></tr>

  <!-- Hero / Featured Story -->
  <tr><td style="background-color:#161616;padding:0 32px 24px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr><td style="border-radius:10px;overflow:hidden;">
        <a href="{{.featured_url}}" style="text-decoration:none;">
          <img src="{{.featured_image}}" alt="{{.featured_title}}" width="576" style="width:100%;max-width:576px;height:auto;display:block;border-radius:10px;" />
        </a>
      </td></tr>
      <tr><td style="padding-top:16px;">
        <span style="display:inline-block;background-color:#6366f1;color:#fff;font-size:11px;font-weight:600;padding:3px 10px;border-radius:20px;text-transform:uppercase;letter-spacing:0.5px;">{{.featured_topic}}</span>
      </td></tr>
      <tr><td style="padding-top:10px;">
        <a href="{{.featured_url}}" style="font-size:22px;font-weight:700;color:#ffffff;text-decoration:none;line-height:1.3;">{{.featured_title}}</a>
      </td></tr>
      <tr><td style="padding-top:8px;">
        <p style="margin:0;font-size:14px;color:#aaa;line-height:1.5;">{{.featured_excerpt}}</p>
      </td></tr>
      <tr><td style="padding-top:8px;">
        <span style="font-size:12px;color:#666;">{{.featured_author}} · {{.featured_date}}</span>
      </td></tr>
    </table>
  </td></tr>

  <!-- Section Header -->
  <tr><td style="background-color:#161616;padding:8px 32px 16px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr>
        <td><span style="font-size:16px;font-weight:700;color:#fff;text-transform:uppercase;letter-spacing:1px;">More Top Stories</span></td>
        <td align="right"><a href="https://monkeys.com.co" style="font-size:13px;color:#6366f1;text-decoration:none;font-weight:600;">View all →</a></td>
      </tr>
      <tr><td colspan="2" style="padding-top:8px;"><div style="height:1px;background-color:#2a2a2a;"></div></td></tr>
    </table>
  </td></tr>

  <!-- Story 1 -->
  <tr><td style="background-color:#161616;padding:0 32px 20px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr>
        <td width="100" valign="top" style="padding-right:16px;">
          <a href="{{.story1_url}}"><img src="{{.story1_image}}" alt="" width="100" height="72" style="width:100px;height:72px;object-fit:cover;border-radius:8px;display:block;" /></a>
        </td>
        <td valign="top">
          <a href="{{.story1_url}}" style="font-size:15px;font-weight:600;color:#fff;text-decoration:none;line-height:1.3;">{{.story1_title}}</a>
          <p style="margin:4px 0 0;font-size:13px;color:#888;">{{.story1_author}} · {{.story1_date}}</p>
          <span style="display:inline-block;margin-top:6px;font-size:11px;color:#f97316;font-weight:600;">{{.story1_topic}}</span>
        </td>
      </tr>
    </table>
  </td></tr>

  <!-- Story 2 -->
  <tr><td style="background-color:#161616;padding:0 32px 20px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr>
        <td width="100" valign="top" style="padding-right:16px;">
          <a href="{{.story2_url}}"><img src="{{.story2_image}}" alt="" width="100" height="72" style="width:100px;height:72px;object-fit:cover;border-radius:8px;display:block;" /></a>
        </td>
        <td valign="top">
          <a href="{{.story2_url}}" style="font-size:15px;font-weight:600;color:#fff;text-decoration:none;line-height:1.3;">{{.story2_title}}</a>
          <p style="margin:4px 0 0;font-size:13px;color:#888;">{{.story2_author}} · {{.story2_date}}</p>
          <span style="display:inline-block;margin-top:6px;font-size:11px;color:#f97316;font-weight:600;">{{.story2_topic}}</span>
        </td>
      </tr>
    </table>
  </td></tr>

  <!-- Story 3 -->
  <tr><td style="background-color:#161616;padding:0 32px 20px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr>
        <td width="100" valign="top" style="padding-right:16px;">
          <a href="{{.story3_url}}"><img src="{{.story3_image}}" alt="" width="100" height="72" style="width:100px;height:72px;object-fit:cover;border-radius:8px;display:block;" /></a>
        </td>
        <td valign="top">
          <a href="{{.story3_url}}" style="font-size:15px;font-weight:600;color:#fff;text-decoration:none;line-height:1.3;">{{.story3_title}}</a>
          <p style="margin:4px 0 0;font-size:13px;color:#888;">{{.story3_author}} · {{.story3_date}}</p>
          <span style="display:inline-block;margin-top:6px;font-size:11px;color:#f97316;font-weight:600;">{{.story3_topic}}</span>
        </td>
      </tr>
    </table>
  </td></tr>

  <!-- Story 4 -->
  <tr><td style="background-color:#161616;padding:0 32px 20px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr>
        <td width="100" valign="top" style="padding-right:16px;">
          <a href="{{.story4_url}}"><img src="{{.story4_image}}" alt="" width="100" height="72" style="width:100px;height:72px;object-fit:cover;border-radius:8px;display:block;" /></a>
        </td>
        <td valign="top">
          <a href="{{.story4_url}}" style="font-size:15px;font-weight:600;color:#fff;text-decoration:none;line-height:1.3;">{{.story4_title}}</a>
          <p style="margin:4px 0 0;font-size:13px;color:#888;">{{.story4_author}} · {{.story4_date}}</p>
          <span style="display:inline-block;margin-top:6px;font-size:11px;color:#f97316;font-weight:600;">{{.story4_topic}}</span>
        </td>
      </tr>
    </table>
  </td></tr>

  <!-- Story 5 -->
  <tr><td style="background-color:#161616;padding:0 32px 24px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr>
        <td width="100" valign="top" style="padding-right:16px;">
          <a href="{{.story5_url}}"><img src="{{.story5_image}}" alt="" width="100" height="72" style="width:100px;height:72px;object-fit:cover;border-radius:8px;display:block;" /></a>
        </td>
        <td valign="top">
          <a href="{{.story5_url}}" style="font-size:15px;font-weight:600;color:#fff;text-decoration:none;line-height:1.3;">{{.story5_title}}</a>
          <p style="margin:4px 0 0;font-size:13px;color:#888;">{{.story5_author}} · {{.story5_date}}</p>
          <span style="display:inline-block;margin-top:6px;font-size:11px;color:#f97316;font-weight:600;">{{.story5_topic}}</span>
        </td>
      </tr>
    </table>
  </td></tr>

  <!-- CTA Banner -->
  <tr><td style="background-color:#161616;padding:0 32px 24px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:linear-gradient(135deg,#4f46e5,#7c3aed);border-radius:10px;">
      <tr><td style="padding:24px 28px;text-align:center;">
        <p style="margin:0 0 4px;font-size:18px;font-weight:700;color:#fff;">Never miss a story</p>
        <p style="margin:0 0 16px;font-size:13px;color:rgba(255,255,255,0.8);">Join thousands of readers on Monkeys</p>
        <a href="https://monkeys.com.co" style="display:inline-block;padding:10px 28px;background:#fff;color:#4f46e5;font-size:14px;font-weight:600;text-decoration:none;border-radius:6px;">Explore Monkeys →</a>
      </td></tr>
    </table>
  </td></tr>

  <!-- Divider -->
  <tr><td style="background-color:#161616;padding:0 32px;">
    <div style="height:1px;background-color:#2a2a2a;"></div>
  </td></tr>

  <!-- FreeRangeNotify Ad Footer -->
  <tr><td style="background-color:#161616;padding:24px 32px;border-radius:0 0 12px 12px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
      <tr><td align="center" style="padding-bottom:16px;">
        <span style="font-size:11px;color:#555;text-transform:uppercase;letter-spacing:1.5px;">Powered by</span>
      </td></tr>
      <tr><td align="center" style="padding-bottom:12px;">
        <span style="font-size:20px;font-weight:700;color:#6366f1;letter-spacing:-0.3px;">⚡ FreeRangeNotify</span>
      </td></tr>
      <tr><td align="center" style="padding-bottom:16px;">
        <p style="margin:0;font-size:13px;color:#888;line-height:1.5;max-width:420px;">High-performance notification infrastructure for modern apps.<br/>Email, Push, SMS, Webhooks & SSE — all from one API.</p>
      </td></tr>
      <tr><td align="center" style="padding-bottom:20px;">
        <a href="https://github.com/the-monkeys/FreeRangeNotify" style="display:inline-block;padding:8px 20px;border:1px solid #6366f1;color:#6366f1;font-size:12px;font-weight:600;text-decoration:none;border-radius:5px;">Learn More →</a>
      </td></tr>
      <tr><td align="center">
        <p style="margin:0;font-size:11px;color:#444;line-height:1.6;">
          You are receiving this because you subscribed to the Monkeys digest.<br/>
          <a href="{{.unsubscribe_url}}" style="color:#666;text-decoration:underline;">Unsubscribe</a> · <a href="https://monkeys.com.co" style="color:#666;text-decoration:underline;">monkeys.com.co</a>
        </p>
      </td></tr>
    </table>
  </td></tr>

</table>
</td></tr>
</table>
</body>
</html>`,
		Variables: []string{
			"digest_title", "date_range",
			"featured_url", "featured_image", "featured_title", "featured_excerpt", "featured_author", "featured_date", "featured_topic",
			"story1_url", "story1_image", "story1_title", "story1_author", "story1_date", "story1_topic",
			"story2_url", "story2_image", "story2_title", "story2_author", "story2_date", "story2_topic",
			"story3_url", "story3_image", "story3_title", "story3_author", "story3_date", "story3_topic",
			"story4_url", "story4_image", "story4_title", "story4_author", "story4_date", "story4_topic",
			"story5_url", "story5_image", "story5_title", "story5_author", "story5_date", "story5_topic",
			"unsubscribe_url",
		},
		Locale:   "en",
		Status:   "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"digest_title": "Monkeys Weekly Digest",
				"date_range": "Oct 1 - Oct 7",
				"featured_url": "https://monkeys.com.co",
				"featured_image": "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"featured_title": "The Future of APIs",
				"featured_excerpt": "An in-depth look at the evolution of APIs in modern microservices.",
				"featured_author": "Dave",
				"featured_date": "Oct 5",
				"featured_topic": "Engineering",
				"story1_url": "https://monkeys.com.co",
				"story1_image": "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"story1_title": "Understanding SSE vs WebSockets",
				"story1_author": "Alex",
				"story1_date": "Oct 4",
				"story1_topic": "Architecture",
				"story2_url": "https://monkeys.com.co",
				"story2_image": "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"story2_title": "Rate Limiting Strategies",
				"story2_author": "Jane",
				"story2_date": "Oct 3",
				"story2_topic": "Backend",
				"story3_url": "https://monkeys.com.co",
				"story3_image": "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"story3_title": "Postgres Triggers for Auditing",
				"story3_author": "Mike",
				"story3_date": "Oct 2",
				"story3_topic": "Database",
				"story4_url": "https://monkeys.com.co",
				"story4_image": "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"story4_title": "Go 1.25 Release Highlights",
				"story4_author": "Dave",
				"story4_date": "Oct 1",
				"story4_topic": "Golang",
				"story5_url": "https://monkeys.com.co",
				"story5_image": "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"story5_title": "React 19 Server Components",
				"story5_author": "Sarah",
				"story5_date": "Sep 30",
				"story5_topic": "Frontend",
				"unsubscribe_url": "https://monkeys.com.co/unsubscribe",
			},
		},
	},
	{
		Name:        "newsletter_editorial",
		Description: "Multi-section editorial newsletter with featured article, blog cards, events, sponsor section, and social footer — Beefree design",
		Channel:     "email",
		Subject:     "{{.headline}} — Newsletter",
		Body:        newsletterEditorialHTML,
		Variables: []string{
			"headline",
			"featured_title", "tag1", "tag2", "tag3", "tag4", "tag5", "featured_body_1",
			"featured_image_url", "featured_url",
			"author_name", "author_title", "author_image_url",
			"blog_section_title",
			"blog1_title", "blog1_description", "blog1_image_url", "blog1_url",
			"blog2_title", "blog2_description", "blog2_image_url", "blog2_url",
			"involvement_title", "involvement_subtitle", "involvement_url",
			"events_title", "events_body", "events_image_url", "events_url",
			"support_title", "support_body", "support_image_url", "support_url",
			"sponsor_title", "sponsor_body", "sponsor_image_url",
			"address_line1", "address_line2", "address_email",
			"about_url", "blog_url",
			"github_url", "discord_url", "telegram_url", "instagram_url",
			"preheader_text",
		},
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"preheader_text":       "Read our latest updates and articles from The Monkeys.",
				"headline":             "THE MONKEYS WEEKLY",
				"featured_title":       "GLOBAL API ECONOMY — STRATEGIC ARCHITECTURE, MONETIZATION DYNAMICS, AND THE EVOLUTION OF DIGITAL VALUE EXCHANGE",
				"tag1":                 "Economy",
				"tag2":                 "API",
				"tag3":                 "Tech",
				"tag4":                 "Business",
				"tag5":                 "DigitalEconomy",
				"featured_body_1":      "The contemporary global economy is currently undergoing a structural transformation characterized by the transition from discrete, siloed business operations to a hyper-connected ecosystem of digital assets. At the center of this metamorphosis is the Application Programming Interface (API), a technical artifact that has successfully transcended its origins as a simple software integration tool to become the fundamental driver of digital commerce and boardroom-level strategy.",
				"featured_image_url":   "https://monkeys.support/api/v1/files/post/hd10je/image.png",
				"featured_url":         "https://monkeys.com.co/blog/hd10je",
				"author_name":          "THE MONKEYS",
				"author_title":         "Editorial Team",
				"author_image_url":     "https://d1oco4z2z1fhwp.cloudfront.net/templates/default/1736/b53509e8-c59f-4e9f-b9b6-de5afbaebf20.png",
				"blog_section_title":   "READ OUR LATEST ARTICLES",
				"blog1_title":          "INSTAGRAM CEO WANTS LABELS FOR REAL PHOTOS AND VIDEOS TO BEAT AI FAKES",
				"blog1_description":    "Adam Mosseri has a smart idea to solve a big problem on social media. AI tools are getting very good at making fake photos and videos that look completely real.",
				"blog1_image_url":      "https://monkeys.support/api/v1/files/post/u2potm/instagramredesign.webp",
				"blog1_url":            "https://monkeys.com.co/blog/u2potm",
				"blog2_title":          "SMALL, LOCAL ADVENTURES COULD BE THE KEY TO A LONGER AND BETTER LIFE",
				"blog2_description":    "It is very easy to feel that real adventure always happens somewhere far away. But between the high costs and the unpredictable twists of daily life, such big trips happen very rarely.",
				"blog2_image_url":      "https://monkeys.support/api/v1/files/post/7xh3m8/wellnessadventureday.webp",
				"blog2_url":            "https://monkeys.com.co/blog/7xh3m8",
				"involvement_title":    "GET INVOLVED",
				"involvement_subtitle": "Join our events and community initiatives",
				"involvement_url":      "https://monkeys.com.co/get-involved",
				"events_title":         "UPCOMING EVENTS",
				"events_body":          "Connect with our community through upcoming meetups, panel discussions, and live Q&A sessions. Great things happen when people come together.",
				"events_image_url":     "https://d1oco4z2z1fhwp.cloudfront.net/templates/default/1736/WED_events_colour.png",
				"events_url":           "https://monkeys.com.co/events",
				"support_title":        "SUPPORT OUR MISSION",
				"support_body":         "Your contribution fuels independent journalism and community programs. Help us continue delivering stories that matter.",
				"support_image_url":    "https://d1oco4z2z1fhwp.cloudfront.net/templates/default/1736/WED_support.png",
				"support_url":          "https://monkeys.com.co/support",
				"sponsor_title":        "BECOME A SPONSOR AND GET 10% OFF YOUR ANNUAL MEMBERSHIP",
				"sponsor_body":         "Partner with us to reach an engaged audience. Sponsors enjoy premium placement, brand visibility, and exclusive member benefits.",
				"sponsor_image_url":    "https://d1oco4z2z1fhwp.cloudfront.net/templates/default/1736/WED_sponsor.png",
				"address_line1":        "Muzaffarpur, Bihar",
				"address_line2":        "India",
				"address_email":        "monkeys.admin@monkeys.com.co",

				"about_url":     "https://monkeys.com.co/about",
				"blog_url":      "https://monkeys.com.co",
				"github_url":    "https://github.com/the-monkeys",
				"discord_url":   "https://discord.com/invite/6fK9YuV8FV",
				"telegram_url":  "https://t.me/monkeys_com_co",
				"instagram_url": "https://www.instagram.com/monkeys_com_co/",
			},
		},
		Locale: "en",
		Status: "active",
	},
	// ── Phase 4: Newsletter Template Library Expansion ──
	{
		Name:        "newsletter_product_launch",
		Description: "Product launch announcement with hero image, feature grid (2×2), and CTA — indigo/white theme",
		Channel:     "email",
		Subject:     "Introducing {{.product_name}} — {{.product_tagline}}",
		Body:        newsletterProductLaunchHTML,
		Variables: []string{
			"preheader_text", "logo_url", "tagline",
			"hero_image_url", "product_name", "product_tagline", "product_description",
			"feature1_icon", "feature1_title", "feature1_desc",
			"feature2_icon", "feature2_title", "feature2_desc",
			"feature3_icon", "feature3_title", "feature3_desc",
			"feature4_icon", "feature4_title", "feature4_desc",
			"cta_text", "cta_url", "unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"preheader_text":      "Big news — we just launched something amazing!",
				"logo_url":            "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"tagline":             "Innovation delivered.",
				"hero_image_url":      "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"product_name":        "FreeRangeNotify v2.0",
				"product_tagline":     "Notifications that scale with you",
				"product_description": "The fastest, most reliable notification infrastructure. Email, push, SMS, webhooks, and SSE — all from one unified API.",
				"feature1_icon":       "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"feature1_title":      "Blazing Fast",
				"feature1_desc":       "Sub-100ms delivery across all channels.",
				"feature2_icon":       "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"feature2_title":      "Enterprise Secure",
				"feature2_desc":       "HMAC-signed webhooks and end-to-end encryption.",
				"feature3_icon":       "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"feature3_title":      "Full Analytics",
				"feature3_desc":       "Real-time dashboards and delivery tracking.",
				"feature4_icon":       "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"feature4_title":      "Easy Integration",
				"feature4_desc":       "SDKs for Go, Node.js, and React.",
				"cta_text":            "Get Started Free →",
				"cta_url":             "https://github.com/the-monkeys/FreeRangeNotify",
				"unsubscribe_url":     "https://example.com/unsubscribe",
			},
		},
	},
	{
		Name:        "newsletter_changelog",
		Description: "Software changelog / release notes with Added, Fixed, Breaking sections — dark header with colored accents",
		Channel:     "email",
		Subject:     "{{.product_name}} {{.version}} — Release Notes",
		Body:        newsletterChangelogHTML,
		Variables: []string{
			"preheader_text", "product_name", "version", "release_date", "release_summary",
			"added_1", "added_2", "added_3",
			"fixed_1", "fixed_2", "fixed_3",
			"breaking_1", "breaking_2",
			"upgrade_guide_text", "upgrade_guide_url", "unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"preheader_text":     "See what's new in the latest release.",
				"product_name":       "FreeRangeNotify",
				"version":            "v2.1.0",
				"release_date":       "June 15, 2025",
				"release_summary":    "This release adds workflow automation, improves delivery reliability, and introduces breaking changes to the webhook signing format.",
				"added_1":            "Workflow engine with conditional branching and delay steps",
				"added_2":            "Digest aggregation with configurable time windows",
				"added_3":            "HMAC-SHA256 signing for SSE channel",
				"fixed_1":            "Race condition in priority queue dequeue under high load",
				"fixed_2":            "Template rendering crash when variables contain HTML entities",
				"fixed_3":            "SSE reconnection dropping messages during Redis failover",
				"breaking_1":         "Webhook signature header changed from X-Signature to X-FRN-Signature-256",
				"breaking_2":         "Removed deprecated /v1/send endpoint — use /v1/notifications instead",
				"upgrade_guide_text": "Read the full upgrade guide →",
				"upgrade_guide_url":  "https://github.com/the-monkeys/FreeRangeNotify/blob/main/UPGRADE.md",
				"unsubscribe_url":    "https://example.com/unsubscribe",
			},
		},
	},
	{
		Name:        "newsletter_event_invitation",
		Description: "Event invitation with gradient header, speaker cards, agenda, and registration CTA — indigo/purple theme",
		Channel:     "email",
		Subject:     "You're Invited: {{.event_name}}",
		Body:        newsletterEventInvitationHTML,
		Variables: []string{
			"preheader_text", "event_name", "event_tagline",
			"event_date", "event_time", "event_location", "event_format", "event_description",
			"speaker1_image", "speaker1_name", "speaker1_title",
			"speaker2_image", "speaker2_name", "speaker2_title",
			"agenda_highlight_1", "agenda_highlight_2", "agenda_highlight_3",
			"register_text", "register_url", "calendar_url", "unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"preheader_text":     "Join us for an exclusive event you won't want to miss.",
				"event_name":         "FreeRangeNotify DevCon 2025",
				"event_tagline":      "Where notification engineering meets community",
				"event_date":         "July 20, 2025",
				"event_time":         "10:00 AM — 4:00 PM IST",
				"event_location":     "Virtual — Zoom Webinar",
				"event_format":       "Virtual",
				"event_description":  "A full-day virtual conference covering notification architecture, delivery optimization, and building real-time systems at scale. Network with engineers from top companies.",
				"speaker1_image":     "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"speaker1_name":      "Priya Sharma",
				"speaker1_title":     "Principal Engineer, Notifications @ Scale",
				"speaker2_image":     "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"speaker2_name":      "Alex Chen",
				"speaker2_title":     "CTO, Real-Time Infrastructure",
				"agenda_highlight_1": "Keynote: The Future of Multi-Channel Notifications",
				"agenda_highlight_2": "Workshop: Building Workflow Engines in Go",
				"agenda_highlight_3": "Panel: Scaling SSE to 1M Concurrent Connections",
				"register_text":      "Register Now — It's Free",
				"register_url":       "https://example.com/register",
				"calendar_url":       "https://example.com/calendar.ics",
				"unsubscribe_url":    "https://example.com/unsubscribe",
			},
		},
	},
	{
		Name:        "newsletter_weekly_roundup",
		Description: "Light-theme weekly content roundup with editor's note, featured article, article list, quick links, and community stats",
		Channel:     "email",
		Subject:     "{{.featured_topic}} & More — Issue #{{.issue_number}}",
		Body:        newsletterWeeklyRoundupHTML,
		Variables: []string{
			"preheader_text", "logo_url", "issue_number", "date_range",
			"editors_note", "editor_name",
			"featured_image", "featured_title", "featured_excerpt", "featured_url", "featured_topic",
			"article1_image", "article1_title", "article1_excerpt", "article1_url",
			"article2_image", "article2_title", "article2_excerpt", "article2_url",
			"article3_image", "article3_title", "article3_excerpt", "article3_url",
			"article4_image", "article4_title", "article4_excerpt", "article4_url",
			"article5_image", "article5_title", "article5_excerpt", "article5_url",
			"quick_link1_text", "quick_link1_url",
			"quick_link2_text", "quick_link2_url",
			"quick_link3_text", "quick_link3_url",
			"stat1_number", "stat1_label",
			"stat2_number", "stat2_label",
			"stat3_number", "stat3_label",
			"unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"preheader_text":   "This week's top stories and community highlights.",
				"logo_url":         "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"issue_number":     "42",
				"date_range":       "Jun 9 – Jun 15, 2025",
				"editors_note":     "This week was packed with exciting developments. We shipped two major features, crossed 10k GitHub stars, and our community grew by 500 new members. Here's everything you need to know.",
				"editor_name":      "The Monkeys Team",
				"featured_image":   "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"featured_title":   "How We Built a 100k msg/s Notification Pipeline",
				"featured_excerpt": "A deep dive into the architecture decisions behind FreeRangeNotify's high-throughput delivery engine.",
				"featured_url":     "https://monkeys.com.co/blog/pipeline",
				"featured_topic":   "Engineering",
				"article1_image":   "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"article1_title":   "Getting Started with Workflow Automation",
				"article1_excerpt": "Automate multi-step notification flows.",
				"article1_url":     "https://monkeys.com.co/blog/workflows",
				"article2_image":   "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"article2_title":   "SSE vs WebSockets for Real-Time Delivery",
				"article2_excerpt": "When to use each and why we chose SSE.",
				"article2_url":     "https://monkeys.com.co/blog/sse-vs-ws",
				"article3_image":   "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"article3_title":   "Template Versioning Best Practices",
				"article3_excerpt": "How to manage email templates at scale.",
				"article3_url":     "https://monkeys.com.co/blog/templates",
				"article4_image":   "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"article4_title":   "Integrating with Apple Push Notification service",
				"article4_excerpt": "Step-by-step APNS setup guide.",
				"article4_url":     "https://monkeys.com.co/blog/apns",
				"article5_image":   "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"article5_title":   "Community Spotlight: Open Source Contributors",
				"article5_excerpt": "Meet the people building FreeRangeNotify.",
				"article5_url":     "https://monkeys.com.co/blog/community",
				"quick_link1_text": "Documentation",
				"quick_link1_url":  "https://github.com/the-monkeys/FreeRangeNotify",
				"quick_link2_text": "Discord",
				"quick_link2_url":  "https://discord.com/invite/6fK9YuV8FV",
				"quick_link3_text": "Changelog",
				"quick_link3_url":  "https://github.com/the-monkeys/FreeRangeNotify/releases",
				"stat1_number":     "10,247",
				"stat1_label":      "GitHub Stars",
				"stat2_number":     "2.4M",
				"stat2_label":      "Notifications Sent",
				"stat3_number":     "1,850",
				"stat3_label":      "Community Members",
				"unsubscribe_url":  "https://example.com/unsubscribe",
			},
		},
	},
	{
		Name:        "newsletter_community_spotlight",
		Description: "Community spotlight with member profile, achievements, stats, and upcoming events — warm cream/amber theme",
		Channel:     "email",
		Subject:     "Community Spotlight: {{.member_name}} — {{.community_name}}",
		Body:        newsletterCommunitySpotlightHTML,
		Variables: []string{
			"preheader_text", "community_name", "issue_date",
			"member_avatar", "member_name", "member_role", "member_bio", "member_profile_url",
			"member_quote",
			"achievement1_icon", "achievement1_title", "achievement1_desc",
			"achievement2_icon", "achievement2_title", "achievement2_desc",
			"achievement3_icon", "achievement3_title", "achievement3_desc",
			"community_stat1_number", "community_stat1_label",
			"community_stat2_number", "community_stat2_label",
			"community_stat3_number", "community_stat3_label",
			"upcoming1_title", "upcoming1_date",
			"upcoming2_title", "upcoming2_date",
			"join_text", "join_url", "unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"preheader_text":         "Meet this month's featured community member.",
				"community_name":         "The Monkeys Community",
				"issue_date":             "June 2025",
				"member_avatar":          "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"member_name":            "Dave S.",
				"member_role":            "Core Maintainer & Architect",
				"member_bio":             "Dave has been contributing to FreeRangeNotify since day one. He architected the Hub-and-Spoke delivery system and leads the provider integration efforts. When not coding, he's mentoring new contributors.",
				"member_profile_url":     "https://github.com/dave",
				"member_quote":           "The best notification system is the one your users never have to think about — it just works, every time, on every channel.",
				"achievement1_icon":      "🏆",
				"achievement1_title":     "100th Pull Request Merged",
				"achievement1_desc":      "Reached a major milestone in open source contributions.",
				"achievement2_icon":      "📝",
				"achievement2_title":     "Published SMART_DELIVERY_GUIDE",
				"achievement2_desc":      "Wrote the definitive guide to presence-based delivery.",
				"achievement3_icon":      "🎤",
				"achievement3_title":     "Conference Speaker",
				"achievement3_desc":      "Presented at GopherCon on real-time notification systems.",
				"community_stat1_number": "1,850",
				"community_stat1_label":  "Members",
				"community_stat2_number": "342",
				"community_stat2_label":  "Contributors",
				"community_stat3_number": "12,400",
				"community_stat3_label":  "Messages",
				"upcoming1_title":        "Community Call: Q3 Roadmap",
				"upcoming1_date":         "Jul 5, 2025",
				"upcoming2_title":        "Hackathon: Build a Custom Provider",
				"upcoming2_date":         "Jul 19-20, 2025",
				"join_text":              "Join Our Community →",
				"join_url":               "https://discord.com/invite/6fK9YuV8FV",
				"unsubscribe_url":        "https://example.com/unsubscribe",
			},
		},
	},
	{
		Name:        "services_accounting_bookkeeping",
		Description: "Full-bleed background image flyer for accounting and bookkeeping service promotions. Features a four-service highlight layout with a bold CTA.",
		Channel:     "email",
		Subject:     "{{.company_name}} — {{or .heading \"Accounting & Bookkeeping Services\"}}",
		Body:        servicesAccountingHTML,
		Variables: []string{
			"background_image", "company_name", "heading",
			"service_1", "service_2", "service_3", "service_4",
			"cta_url", "cta_text",
			"phone", "website", "unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "services",
			"sample_data": map[string]interface{}{
				"background_image": "https://images.unsplash.com/photo-1554224155-6726b3ff858f?w=1280",
				"company_name":     "Augustus & Co.",
				"heading":          "Accounting & Bookkeeping Services",
				"service_1":        "Financial Reports & Analysis",
				"service_2":        "Tax Compliance & Filing",
				"service_3":        "Payroll Management",
				"service_4":        "Audit & Assurance",
				"cta_url":          "https://freerangenotify.com/contact",
				"cta_text":         "Contact Us",
				"phone":            "+1 (555) 012-3456",
				"website":          "www.augustusco.com",
				"unsubscribe_url":  "https://freerangenotify.com/unsubscribe",
			},
		},
	},
	{
		Name:        "newsletter_webinar_briefing",
		Description: "Live technical briefing registration email featuring speaker profiles, a video overlay trailer placeholder, and dark/light contrasting content blocks.",
		Channel:     "email",
		Subject:     "{{.company_name}} - {{.badge_text}}",
		Body:        newsletterWebinarBriefingHTML,
		Variables: []string{
			"company_name", "company_logo", "badge_text", "title_main", "title_highlight",
			"event_datetime", "cta_url", "cta_text", "video_url", "video_thumbnail",
			"recipient_name", "intro_paragraph", "speakers_title",
			"speaker1_name", "speaker1_role", "speaker1_avatar",
			"speaker2_name", "speaker2_role", "speaker2_avatar",
			"twitter_url", "linkedin_url", "instagram_url",
			"current_year", "company_address", "unsubscribe_url", "preferences_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"usecase":  "Inviting technical audiences or enterprise leads to live webinars and virtual events.",
			"sample_data": map[string]interface{}{
				"company_name":    "Apex Systems",
				"company_logo":    "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"badge_text":      "Live Technical Briefing",
				"title_main":      "Architecting for the",
				"title_highlight": "2026 Data Surge",
				"event_datetime":  "April 15, 2026 | 10:00 AM PST",
				"cta_url":         "https://freerangenotify.com/register-event",
				"cta_text":        "Register for Free",
				"video_url":       "https://youtube.com/watch?v=dQw4w9WgXcQ",
				"video_thumbnail": "https://images.unsplash.com/photo-1516321318423-f06f85e504b3?w=520&auto=format&fit=crop&q=80",
				"recipient_name":  "Alex",
				"intro_paragraph": "Infrastructure is no longer just about 'keeping the lights on.' As we approach 2026, the shift toward autonomous systems is requiring a total rethink of the data pipeline. Join our senior architects as they live-build a resilient node structure designed for 99.999% uptime during peak surges.",
				"speakers_title":  "Featured Speakers",
				"speaker1_name":   "Jordan Vance",
				"speaker1_role":   "VP of Core Infra",
				"speaker1_avatar": "https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?auto=format&fit=facearea&facepad=2&w=120&h=120&q=80",
				"speaker2_name":   "Elena Rodriguez",
				"speaker2_role":   "Principal Architect",
				"speaker2_avatar": "https://images.unsplash.com/photo-1438761681033-6461ffad8d80?auto=format&fit=facearea&facepad=2&w=120&h=120&q=80",
				"twitter_url":     "https://twitter.com/themonkeys_co",
				"linkedin_url":    "https://linkedin.com/company/themonkeys",
				"instagram_url":   "https://instagram.com/themonkeys",
				"current_year":    "2026",
				"company_address": "500 Oracle Parkway, Redwood Shores, CA",
				"unsubscribe_url": "https://freerangenotify.com/unsubscribe",
				"preferences_url": "https://freerangenotify.com/preferences",
			},
		},
	},
}

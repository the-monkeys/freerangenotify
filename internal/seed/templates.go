package seed

import (
	_ "embed"

	"github.com/the-monkeys/freerangenotify/internal/domain/template"
)

//go:embed newsletter_editorial.html
var newsletterEditorialHTML string

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
	},
	{
		Name:        "sms_verification",
		Description: "SMS verification code",
		Channel:     "sms",
		Body:        "Your verification code is {{.code}}. Expires in {{.expiry}} minutes.",
		Variables:   []string{"code", "expiry"},
		Locale:      "en",
		Status:      "active",
	},
	{
		Name:        "webhook_event",
		Description: "Generic webhook event notification",
		Channel:     "webhook",
		Subject:     "{{.event_type}}",
		Body:        `{"event": "{{.event_type}}", "message": "{{.message}}", "timestamp": "{{.timestamp}}"}`,
		Variables:   []string{"event_type", "message", "timestamp"},
		Locale:      "en",
		Status:      "active",
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
		Locale: "en",
		Status: "active",
	},
	{
		Name:        "newsletter_editorial",
		Description: "Multi-section editorial newsletter with featured article, blog cards, events, sponsor section, and social footer — Beefree design",
		Channel:     "email",
		Subject:     "{{.headline}} — Newsletter",
		Body:        newsletterEditorialHTML,
		Variables: []string{
			"headline",
			"featured_title", "featured_tags", "featured_body_1",
			"featured_image_url", "featured_url",
			"author_name", "author_title", "author_image_url",
			"blog_section_title",
			"blog1_title", "blog1_description", "blog1_image_url", "blog1_url",
			"blog2_title", "blog2_description", "blog2_image_url", "blog2_url",
			"involvement_title", "involvement_subtitle", "involvement_url",
			"events_title", "events_body", "events_image_url", "events_url",
			"support_title", "support_body", "support_image_url", "support_url",
			"sponsor_title", "sponsor_body", "sponsor_image_url",
			"address", "image_URL", "logo_url",
			"about_url", "blog_url",
			"github_url", "discord_url", "telegram_url", "instagram_url",
			"preheader_text",
		},
		Metadata: map[string]interface{}{
			"sample_data": map[string]interface{}{
				"preheader_text":       "Read our latest updates and articles from The Monkeys.",
				"headline":             "THE MONKEYS WEEKLY",
				"featured_title":       "GLOBAL API ECONOMY — STRATEGIC ARCHITECTURE, MONETIZATION DYNAMICS, AND THE EVOLUTION OF DIGITAL VALUE EXCHANGE",
				"featured_tags":        `<span style="display:inline-block;border:1px solid #d1d5db;border-radius:20px;padding:4px 14px;font-size:13px;color:#1f2937;font-family:'Merriwheater','Georgia',serif;margin:2px 4px 2px 0;">Economy ↗</span><span style="display:inline-block;border:1px solid #d1d5db;border-radius:20px;padding:4px 14px;font-size:13px;color:#1f2937;font-family:'Merriwheater','Georgia',serif;margin:2px 4px 2px 0;">API ↗</span><span style="display:inline-block;border:1px solid #d1d5db;border-radius:20px;padding:4px 14px;font-size:13px;color:#1f2937;font-family:'Merriwheater','Georgia',serif;margin:2px 4px 2px 0;">Tech ↗</span><span style="display:inline-block;border:1px solid #d1d5db;border-radius:20px;padding:4px 14px;font-size:13px;color:#1f2937;font-family:'Merriwheater','Georgia',serif;margin:2px 4px 2px 0;">Business ↗</span><span style="display:inline-block;border:1px solid #d1d5db;border-radius:20px;padding:4px 14px;font-size:13px;color:#1f2937;font-family:'Merriwheater','Georgia',serif;margin:2px 4px 2px 0;">DigitalEconomy ↗</span>`,
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
				"address":              "Muzaffarpur, Bihar<br>India<br><br> Email: admin@monkeys.com.co",
				"image_URL":            "https://raw.githubusercontent.com/the-monkeys/the_monkeys/main/apps/the_monkeys/public/logo-brand.svg",
				"logo_url":             "https://monkeys.com.co",
				"about_url":            "https://monkeys.com.co/about",
				"blog_url":             "https://monkeys.com.co",
				"github_url":           "https://github.com/the-monkeys",
				"discord_url":          "https://discord.com/invite/6fK9YuV8FV",
				"telegram_url":         "https://t.me/monkeys_com_co",
				"instagram_url":        "https://www.instagram.com/monkeys_com_co/",
			},
		},
		Locale: "en",
		Status: "active",
	},
}

import os

HTML_DIR = r"c:\Users\Dave\the_monkeys\FreeRangeNotify\internal\seed"

promo_html = """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light dark">
    <meta name="supported-color-schemes" content="light dark">
    <title>{{.promo_title}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f8fafc;font-family:'Helvetica Neue',Helvetica,Arial,sans-serif;-webkit-font-smoothing:antialiased;">
    <table width="100%" border="0" cellspacing="0" cellpadding="0" style="background-color:#f8fafc;">
        <tr>
            <td align="center" style="padding:40px 10px;">
                <table width="600" border="0" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:24px;overflow:hidden;box-shadow:0 10px 40px rgba(0,0,0,0.08);">
                    <tr>
                        <td background="{{.hero_image}}" style="background-color:#0f172a;background-image:url('{{.hero_image}}');background-size:cover;background-position:center;padding:80px 40px;text-align:center;">
                            <div style="background-color:rgba(15,23,42,0.85);display:inline-block;padding:8px 20px;border-radius:30px;color:#38bdf8;font-size:13px;font-weight:800;letter-spacing:2px;text-transform:uppercase;margin-bottom:24px;border:1px solid rgba(56,189,248,0.3);">
                                {{.badge_text}}
                            </div>
                            <h1 style="margin:0 0 20px 0;color:#ffffff;font-size:48px;line-height:1.1;font-weight:900;letter-spacing:-1.5px;">
                                {{.promo_title}}
                            </h1>
                            <p style="margin:0 0 40px 0;color:#cbd5e1;font-size:20px;line-height:1.5;font-weight:300;">
                                {{.promo_subtitle}}
                            </p>
                            <table border="0" cellspacing="0" cellpadding="0" style="margin:0 auto;">
                                <tr>
                                    <td align="center" style="border-radius:12px;background:linear-gradient(135deg, #3b82f6, #6366f1);box-shadow:0 8px 20px rgba(59,130,246,0.3);">
                                        <a href="{{.cta_url}}" style="display:inline-block;padding:18px 40px;color:#ffffff;font-size:16px;font-weight:800;text-decoration:none;letter-spacing:1px;text-transform:uppercase;">{{.cta_text}}</a>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    <tr>
                        <td align="center" style="padding:50px 40px;background-color:#ffffff;">
                            <p style="margin:0 0 16px 0;color:#64748b;font-size:14px;text-transform:uppercase;letter-spacing:1px;font-weight:700;">Use this code at checkout</p>
                            <div style="background-color:#f1f5f9;border:2px dashed #94a3b8;padding:24px 40px;border-radius:16px;display:inline-block;">
                                <span style="font-family:'Courier New',Courier,monospace;font-size:32px;font-weight:900;color:#0f172a;letter-spacing:6px;">{{.promo_code}}</span>
                            </div>
                            <p style="margin:24px 0 0 0;color:#94a3b8;font-size:13px;font-weight:500;">Valid until {{.valid_until}}</p>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding:0 40px 50px 40px;background-color:#ffffff;">
                            <table width="100%" border="0" cellspacing="0" cellpadding="0">
                                <tr>
                                    <td width="45%" valign="top">
                                        <img src="{{.product_image}}" alt="Product" width="100%" style="border-radius:16px;display:block;height:auto;box-shadow:0 4px 12px rgba(0,0,0,0.1);" />
                                    </td>
                                    <td width="55%" valign="middle" style="padding-left:30px;">
                                        <h2 style="margin:0 0 12px 0;color:#0f172a;font-size:26px;font-weight:800;line-height:1.2;">{{.product_name}}</h2>
                                        <p style="margin:0 0 24px 0;color:#475569;font-size:16px;line-height:1.6;">{{.product_description}}</p>
                                        <div style="font-size:24px;font-weight:900;color:#3b82f6;">
                                            {{.product_price}} <span style="font-size:16px;color:#94a3b8;text-decoration:line-through;font-weight:500;margin-left:8px;">{{.product_original_price}}</span>
                                        </div>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    <tr>
                        <td style="background-color:#f8fafc;padding:30px 40px;text-align:center;">
                            <p style="margin:0 0 12px 0;color:#475569;font-size:15px;font-weight:700;">{{.company_name}}</p>
                            <p style="margin:0 0 16px 0;color:#94a3b8;font-size:12px;line-height:1.5;">{{.footer_text}}</p>
                            <p style="margin:0;font-size:12px;">
                                <a href="{{.unsubscribe_url}}" style="color:#3b82f6;text-decoration:underline;">Unsubscribe</a>
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>"""

travel_html = """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.destination_name}}</title>
</head>
<body style="margin:0;padding:0;background-color:#fdfbf7;font-family:Georgia,serif;">
    <table width="100%" border="0" cellspacing="0" cellpadding="0" style="background-color:#fdfbf7;">
        <tr>
            <td align="center" style="padding:30px 10px;">
                <table width="600" border="0" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 10px 30px rgba(0,0,0,0.05);">
                    <tr>
                        <td align="center" style="padding:40px 20px;background-color:#0f3b4c;">
                            <img src="{{.logo_image}}" alt="Logo" width="120" style="display:block;margin-bottom:20px;border-radius:60px;" />
                            <h2 style="margin:0;color:#ffffff;font-size:14px;text-transform:uppercase;letter-spacing:3px;font-family:Arial,sans-serif;">{{.company_name}}</h2>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding:0;">
                            <img src="{{.hero_image}}" alt="Destination" width="100%" style="display:block;width:100%;height:auto;" />
                        </td>
                    </tr>
                    <tr>
                        <td style="padding:50px 40px;text-align:center;">
                            <h1 style="margin:0 0 20px 0;color:#0f3b4c;font-size:36px;font-weight:normal;line-height:1.2;">{{.destination_name}}</h1>
                            <p style="margin:0 0 40px 0;color:#4a5568;font-size:18px;line-height:1.6;">{{.destination_description}}</p>
                            
                            <!-- Itinerary -->
                            <div style="background-color:#f7f9fa;border-left:4px solid #d4af37;padding:30px;text-align:left;border-radius:0 8px 8px 0;margin-bottom:40px;">
                                <h3 style="margin:0 0 20px 0;color:#0f3b4c;font-size:22px;">Trip Highlights</h3>
                                <p style="margin:0 0 12px 0;color:#2d3748;font-size:16px;font-family:Arial,sans-serif;"><strong>Day 1:</strong> {{.day_1_plan}}</p>
                                <p style="margin:0 0 12px 0;color:#2d3748;font-size:16px;font-family:Arial,sans-serif;"><strong>Day 2:</strong> {{.day_2_plan}}</p>
                                <p style="margin:0 0 0 0;color:#2d3748;font-size:16px;font-family:Arial,sans-serif;"><strong>Day 3:</strong> {{.day_3_plan}}</p>
                            </div>

                            <table border="0" cellspacing="0" cellpadding="0" style="margin:0 auto;">
                                <tr>
                                    <td align="center" style="border-radius:4px;background-color:#d4af37;">
                                        <a href="{{.booking_url}}" style="display:inline-block;padding:16px 40px;color:#ffffff;font-size:15px;font-weight:bold;text-decoration:none;letter-spacing:2px;text-transform:uppercase;font-family:Arial,sans-serif;">{{.cta_text}}</a>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    <tr>
                        <td style="background-color:#0f3b4c;padding:30px 40px;text-align:center;">
                            <p style="margin:0;color:#a0aec0;font-size:12px;font-family:Arial,sans-serif;line-height:1.5;">
                                {{.footer_text}}<br><br>
                                <a href="{{.unsubscribe_url}}" style="color:#d4af37;text-decoration:none;">Unsubscribe</a>
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>"""

course_html = """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.course_name}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:'Inter',Arial,sans-serif;">
    <table width="100%" border="0" cellspacing="0" cellpadding="0">
        <tr>
            <td align="center" style="padding:40px 10px;">
                <table width="600" border="0" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:12px;border-top:6px solid #1e3a8a;box-shadow:0 8px 16px rgba(0,0,0,0.04);">
                    <tr>
                        <td style="padding:40px 40px 20px 40px;text-align:center;">
                            <img src="{{.institution_logo}}" alt="Institution Logo" width="80" style="display:inline-block;margin-bottom:16px;border-radius:8px;" />
                            <h4 style="margin:0 0 8px 0;color:#64748b;font-size:14px;text-transform:uppercase;letter-spacing:1px;">{{.institution_name}}</h4>
                            <h1 style="margin:0;color:#1e293b;font-size:32px;font-weight:800;line-height:1.3;">{{.course_name}}</h1>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding:20px 40px;">
                            <img src="{{.course_image}}" alt="Course" width="100%" style="border-radius:8px;display:block;" />
                        </td>
                    </tr>
                    <tr>
                        <td style="padding:20px 40px 40px 40px;">
                            <p style="margin:0 0 24px 0;color:#475569;font-size:17px;line-height:1.6;">{{.course_intro}}</p>
                            
                            <table width="100%" border="0" cellspacing="0" cellpadding="20" style="background-color:#f8fafc;border-radius:8px;margin-bottom:30px;border:1px solid #e2e8f0;">
                                <tr>
                                    <td>
                                        <h3 style="margin:0 0 16px 0;color:#0f172a;font-size:18px;">What you will learn:</h3>
                                        <ul style="margin:0;padding:0 0 0 20px;color:#475569;font-size:15px;line-height:1.8;">
                                            <li>{{.module_1}}</li>
                                            <li>{{.module_2}}</li>
                                            <li>{{.module_3}}</li>
                                        </ul>
                                    </td>
                                </tr>
                            </table>

                            <table width="100%" border="0" cellspacing="0" cellpadding="0">
                                <tr>
                                    <td align="left" valign="middle">
                                        <div style="font-size:20px;font-weight:bold;color:#1e3a8a;">Starts {{.start_date}}</div>
                                        <div style="font-size:14px;color:#64748b;">Duration: {{.duration}}</div>
                                    </td>
                                    <td align="right" valign="middle">
                                        <a href="{{.enroll_url}}" style="display:inline-block;padding:14px 28px;background-color:#1e3a8a;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;border-radius:6px;">{{.cta_text}}</a>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>"""

freelance_html = """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Hello from {{.name}}</title>
</head>
<body style="margin:0;padding:0;background-color:#ffffff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
    <table width="100%" border="0" cellspacing="0" cellpadding="0">
        <tr>
            <td align="center" style="padding:60px 10px;">
                <table width="500" border="0" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border:1px solid #e5e7eb;border-radius:16px;padding:50px;">
                    <tr>
                        <td align="center" style="padding-bottom:30px;">
                            <img src="{{.profile_pic}}" alt="{{.name}}" width="100" height="100" style="border-radius:50%;display:block;border:3px solid #f3f4f6;object-fit:cover;" />
                        </td>
                    </tr>
                    <tr>
                        <td align="center">
                            <h1 style="margin:0 0 10px 0;color:#111827;font-size:28px;font-weight:800;">Hi, I'm {{.name}}</h1>
                            <p style="margin:0 0 30px 0;color:#6b7280;font-size:16px;font-weight:500;">{{.title}}</p>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding-bottom:40px;">
                            <p style="margin:0 0 20px 0;color:#374151;font-size:16px;line-height:1.7;">{{.intro_p1}}</p>
                            <p style="margin:0;color:#374151;font-size:16px;line-height:1.7;">{{.intro_p2}}</p>
                        </td>
                    </tr>
                    <tr>
                        <td align="center">
                            <a href="{{.booking_url}}" style="display:inline-block;padding:14px 32px;background-color:#111827;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;border-radius:30px;">{{.cta_text}}</a>
                        </td>
                    </tr>
                    <tr>
                        <td align="center" style="padding-top:40px;">
                            <a href="{{.website_url}}" style="color:#6b7280;font-size:14px;text-decoration:none;font-weight:500;">{{.website_url}}</a>
                        </td>
                    </tr>
                </table>
                <p style="margin:30px 0 0 0;color:#9ca3af;font-size:12px;text-align:center;">
                    <a href="{{.unsubscribe_url}}" style="color:#9ca3af;text-decoration:underline;">Unsubscribe</a>
                </p>
            </td>
        </tr>
    </table>
</body>
</html>"""

meetup_html = """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta content="width=device-width, initial-scale=1" name="viewport">
    <title>{{.event_name}}</title>
</head>
<body style="margin:0;padding:20px;background-color:#faf5ff;font-family:'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
    <table width="100%" border="0" cellspacing="0" cellpadding="0">
        <tr>
            <td align="center">
                <table width="600" border="0" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:24px;overflow:hidden;box-shadow:0 10px 25px rgba(147,51,234,0.1);">
                    <tr>
                        <td style="background:linear-gradient(135deg,#c084fc,#9333ea);padding:60px 40px;text-align:center;">
                            <div style="background-color:rgba(255,255,255,0.2);display:inline-block;padding:8px 16px;border-radius:20px;color:#ffffff;font-size:13px;font-weight:bold;margin-bottom:20px;text-transform:uppercase;letter-spacing:1px;">
                                Local Meetup
                            </div>
                            <h1 style="margin:0;color:#ffffff;font-size:42px;line-height:1.1;font-weight:900;">{{.event_name}}</h1>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding:40px;">
                            <p style="margin:0 0 30px 0;color:#4b5563;font-size:18px;line-height:1.6;text-align:center;">{{.event_description}}</p>
                            
                            <table width="100%" border="0" cellspacing="0" cellpadding="20" style="background-color:#f3f4f6;border-radius:16px;margin-bottom:40px;">
                                <tr>
                                    <td width="33%" align="center" style="border-right:1px solid #e5e7eb;">
                                        <span style="display:block;font-size:24px;margin-bottom:8px;">📅</span>
                                        <strong style="display:block;color:#1f2937;font-size:14px;">{{.event_date}}</strong>
                                        <span style="color:#6b7280;font-size:13px;">{{.event_time}}</span>
                                    </td>
                                    <td width="33%" align="center" style="border-right:1px solid #e5e7eb;">
                                        <span style="display:block;font-size:24px;margin-bottom:8px;">📍</span>
                                        <strong style="display:block;color:#1f2937;font-size:14px;">Location</strong>
                                        <span style="color:#6b7280;font-size:13px;">{{.event_location}}</span>
                                    </td>
                                    <td width="33%" align="center">
                                        <span style="display:block;font-size:24px;margin-bottom:8px;">🍕</span>
                                        <strong style="display:block;color:#1f2937;font-size:14px;">Vibe</strong>
                                        <span style="color:#6b7280;font-size:13px;">{{.event_vibe}}</span>
                                    </td>
                                </tr>
                            </table>

                            <table border="0" cellspacing="0" cellpadding="0" style="margin:0 auto;">
                                <tr>
                                    <td align="center" style="border-radius:12px;background-color:#9333ea;box-shadow:0 4px 14px rgba(147,51,234,0.4);">
                                        <a href="{{.rsvp_url}}" style="display:inline-block;padding:16px 48px;color:#ffffff;font-size:16px;font-weight:bold;text-decoration:none;">{{.cta_text}}</a>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding:30px 40px;background-color:#f9fafb;text-align:center;border-top:1px solid #f3f4f6;">
                            <img src="{{.sponsor_logo}}" alt="Sponsor" height="30" style="opacity:0.6;display:inline-block;margin-bottom:12px;border-radius:6px;" />
                            <p style="margin:0;color:#9ca3af;font-size:12px;">Sponsored by {{.sponsor_name}}</p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>"""

PRESERVE_EXISTING_HTML = True

for name, content in [
    ("promo_seasonal_sale.html", promo_html),
    ("travel_holiday_package.html", travel_html),
    ("institution_course_enrollment.html", course_html),
    ("freelance_consultant_intro.html", freelance_html),
    ("event_local_meetup.html", meetup_html)
]:
    path = os.path.join(HTML_DIR, name)
    if PRESERVE_EXISTING_HTML and os.path.exists(path):
        continue
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)

# Now update templates.go
templates_go_path = os.path.join(HTML_DIR, "templates.go")
with open(templates_go_path, "r", encoding="utf-8") as f:
    orig = f.read()

embeds = """//go:embed promo_seasonal_sale.html
var promoSeasonalSaleHTML string

//go:embed travel_holiday_package.html
var travelHolidayPackageHTML string

//go:embed institution_course_enrollment.html
var courseEnrollmentHTML string

//go:embed freelance_consultant_intro.html
var freelanceIntroHTML string

//go:embed event_local_meetup.html
var localMeetupHTML string

// LibraryTemplates are pre-built templates users can clone into their apps."""

if "// LibraryTemplates" in orig:
    orig = orig.replace("// LibraryTemplates", embeds, 1)

entries = """	{
		Name:        "promo_seasonal_sale",
		Description: "High-impact promotional template for seasonal sales with promo code box and product highlight.",
		Channel:     "email",
		Subject:     "{{.promo_title}}",
		Body:        promoSeasonalSaleHTML,
		Variables: []string{
			"badge_text", "promo_title", "promo_subtitle", "cta_url", "cta_text",
			"promo_code", "valid_until", "product_image", "product_name",
			"product_description", "product_price", "product_original_price",
			"company_name", "footer_text", "unsubscribe_url", "hero_image",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"hero_image":             "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"badge_text":             "Black Friday Early Access",
				"promo_title":            "Our Biggest Sale of the Year",
				"promo_subtitle":         "Get ahead of the crowd and unlock your 40% discount across the entire store today.",
				"cta_url":                "https://example.com/shop",
				"cta_text":               "Shop the Sale",
				"promo_code":             "BFRI40",
				"valid_until":            "Nov 30, 2026",
				"product_image":          "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"product_name":           "Premium Pro Headset",
				"product_description":    "Immersive sound engine, noise cancellation, and 40 hour battery life.",
				"product_price":          "$149",
				"product_original_price": "$249",
				"company_name":           "Monkeys Tech Co.",
				"footer_text":            "You received this because you are opted in to our promotional lists.",
				"unsubscribe_url":        "https://example.com/unsubscribe",
			},
		},
	},
	{
		Name:        "travel_holiday_package",
		Description: "Beautiful destination promotional email for travel agencies with itinerary snapshot.",
		Channel:     "email",
		Subject:     "Your next adventure: {{.destination_name}}",
		Body:        travelHolidayPackageHTML,
		Variables: []string{
			"logo_image", "company_name", "hero_image", "destination_name",
			"destination_description", "day_1_plan", "day_2_plan", "day_3_plan",
			"booking_url", "cta_text", "footer_text", "unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"logo_image":              "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"hero_image":              "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"company_name":            "Wanderlust Travels",
				"destination_name":        "Escape to the Swiss Alps",
				"destination_description": "Experience the ultimate winter getaway. Deep powder snow, breathtaking mountain ranges, and luxury chalets all included in our exclusive package.",
				"day_1_plan":              "Arrival and Welcome Dinner",
				"day_2_plan":              "Guided Ski Tour & Spa Access",
				"day_3_plan":              "Mountain Peak Cable Car",
				"booking_url":             "https://example.com/book",
				"cta_text":                "View Package",
				"footer_text":             "Wanderlust Travels Ltd.",
				"unsubscribe_url":         "https://example.com/unsub",
			},
		},
	},
	{
		Name:        "institution_course_enrollment",
		Description: "Professional service/course enrollment template for institutions.",
		Channel:     "email",
		Subject:     "Enroll now: {{.course_name}}",
		Body:        courseEnrollmentHTML,
		Variables: []string{
			"institution_logo", "institution_name", "course_name", "course_image",
			"course_intro", "module_1", "module_2", "module_3", "start_date",
			"duration", "enroll_url", "cta_text",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "services",
			"sample_data": map[string]interface{}{
				"institution_logo": "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"institution_name": "Global Tech University",
				"course_name":      "Advanced Machine Learning",
				"course_image":     "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"course_intro":     "Equip yourself with the skills to build production-ready ML models.",
				"module_1":         "Deep Learning Fundamentals",
				"module_2":         "Neural Network Architecture",
				"module_3":         "Model Deployment & MLOps",
				"start_date":       "Oct 15, 2026",
				"duration":         "12 Weeks",
				"enroll_url":       "https://example.com/enroll",
				"cta_text":         "Secure Your Spot",
			},
		},
	},
	{
		Name:        "freelance_consultant_intro",
		Description: "Personal, minimalist introduction template for freelancers scheduling calls.",
		Channel:     "email",
		Subject:     "Hello from {{.name}}",
		Body:        freelanceIntroHTML,
		Variables: []string{
			"profile_pic", "name", "title", "intro_p1", "intro_p2",
			"booking_url", "cta_text", "website_url", "unsubscribe_url",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "services",
			"sample_data": map[string]interface{}{
				"profile_pic":     "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"name":            "Jordan Hayes",
				"title":           "Fractional CMO & Strategy Consultant",
				"intro_p1":        "I help tech startups scale their growth marketing channels without the overhead of a full-time executive.",
				"intro_p2":        "If you're looking to optimize your acquisition funnel for Q4, let's connect for a brief discovery call to see if there's a fit.",
				"booking_url":     "https://calendly.com/example",
				"cta_text":        "Schedule a Call",
				"website_url":     "https://jordanhayes.dev",
				"unsubscribe_url": "https://example.com/unsub",
			},
		},
	},
	{
		Name:        "event_local_meetup",
		Description: "Fun, vibrant invitation template for local meetups or workshops.",
		Channel:     "email",
		Subject:     "You're invited: {{.event_name}}",
		Body:        localMeetupHTML,
		Variables: []string{
			"event_name", "event_description", "event_date", "event_time",
			"event_location", "event_vibe", "rsvp_url", "cta_text",
			"sponsor_logo", "sponsor_name",
		},
		Locale: "en",
		Status: "active",
		Metadata: map[string]interface{}{
			"category": "newsletter",
			"sample_data": map[string]interface{}{
				"event_name":        "Go Developers Meetup",
				"event_description": "Join us for an evening of lightning talks, networking, and pizza. We'll be discussing the latest Go 1.25 release and building microservices.",
				"event_date":        "Friday, Dec 10",
				"event_time":        "6:30 PM",
				"event_location":    "Downtown Hub",
				"event_vibe":        "Pizza & Tech",
				"rsvp_url":          "https://example.com/rsvp",
				"cta_text":          "RSVP Now",
				"sponsor_logo":      "https://monkeys.com.co/api/v2/storage/posts/za3dif/784f9010-baa7-4cd0-a3a7-5d39c7c738d6.png",
				"sponsor_name":      "Monkeys Cloud",
			},
		},
	},
"""

orig = orig.replace("\n\t},\n}", "\n\t},\n" + entries + "}")

with open(templates_go_path, "w", encoding="utf-8") as f:
    f.write(orig)

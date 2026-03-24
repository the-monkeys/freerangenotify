const fs = require('fs');

const file = 'internal/seed/templates.go';
let data = fs.readFileSync(file, 'utf8');

const usecases = {
	"welcome_email": "Onboarding newly registered users and encouraging first login.",
	"password_reset": "Sending a secure OTP code for account recovery.",
	"order_confirmation": "E-commerce post-purchase receipts.",
	"push_alert": "Mobile/Web push notifications for urgent updates.",
	"sms_verification": "Two-factor authentication via SMS.",
	"appointment_reminder": "Notifying clients of upcoming scheduled service.",
	"booking_confirmation": "Hotel, flight, or event ticket confirmations.",
	"maintenance_notice": "Informing users about scheduled downtime.",
	"webhook_release_note": "Triggering internal CI/CD or Slack alerts on new releases.",
	"sse_realtime": "Live browser toast notifications for active users.",
	"monkeys_weekly_digest": "Content syndication for blog subscribers.",
	"newsletter_editorial": "Long-form editorial content and brand story-telling.",
	"newsletter_product_launch": "Announcing a new feature or product line.",
	"newsletter_changelog": "Technical release notes and patch updates.",
	"newsletter_event_invitation": "Inviting users to physical or virtual events.",
	"newsletter_weekly_roundup": "Curated weekly links and community highlights.",
	"newsletter_community_spotlight": "Showcasing user achievements or testimonials.",
	"services_accounting_bookkeeping": "B2B service promotion and lead generation."
};

for (const [name, usecase] of Object.entries(usecases)) {
	const searchStr = `Name:        "${name}",`;
	const categoryRegex = new RegExp(`(Name:\\s*"${name}"[\\s\\S]*?"category":\\s*"[^"]*",\\n)`);
	
	if (data.match(categoryRegex)) {
		let replacement = '$1\t\t\t"usecase":  "' + usecase + '",\n';
		data = data.replace(categoryRegex, replacement);
	}
}

fs.writeFileSync(file, data);
console.log("Injected usecase metadata.");

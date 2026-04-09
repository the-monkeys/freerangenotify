import React from 'react';
import PolicyLayout, { PolicySection, PolicyCallout } from '../components/PolicyLayout';

const toc = [
    { id: 'introduction', label: 'Introduction' },
    { id: 'information-we-collect', label: 'Information We Collect' },
    { id: 'how-we-use-information', label: 'How We Use Information' },
    { id: 'whatsapp-data-sharing', label: 'WhatsApp & Meta Data Sharing' },
    { id: 'third-party-providers', label: 'Third-Party Providers' },
    { id: 'consent-opt-in', label: 'Consent & Opt-In' },
    { id: 'opt-out', label: 'Opt-Out & Unsubscribe' },
    { id: 'data-retention', label: 'Data Retention' },
    { id: 'data-security', label: 'Data Security' },
    { id: 'your-rights', label: 'Your Rights' },
    { id: 'international-transfers', label: 'International Transfers' },
    { id: 'childrens-privacy', label: "Children's Privacy" },
    { id: 'changes', label: 'Changes to This Policy' },
    { id: 'contact', label: 'Contact Us' },
];

const PrivacyPolicy: React.FC = () => (
    <PolicyLayout
        title="Privacy Policy"
        subtitle="How FreeRange Notify collects, uses, and protects your data across all notification channels."
        lastUpdated="April 9, 2026"
        toc={toc}
    >
        <PolicySection id="introduction" title="1. Introduction">
            <p>
                BUDDHICINTAKA PVT. LTD. (&quot;the Company&quot;, &quot;we&quot;, &quot;us&quot;, &quot;our&quot;) operates FreeRange Notify, a multi-channel notification infrastructure platform (&quot;the Service&quot;). This Privacy Policy explains how we collect, use, disclose, store, and protect your information when you use our Service, including communications delivered via WhatsApp, email, SMS, push notifications, webhooks, and other supported channels.
            </p>
            <p>
                Our Service is designed for businesses, applications, and not-for-profit organizations that need to notify their users, customers, and stakeholders through multiple channels. FreeRange Notify is a notification delivery infrastructure — it is not a consumer messaging platform like WhatsApp, Telegram, or Signal.
            </p>
            <p>
                By using our Service, you acknowledge that you have read and understood this Privacy Policy. If you do not agree with the practices described herein, please do not use the Service.
            </p>
        </PolicySection>

        <PolicySection id="information-we-collect" title="2. Information We Collect">
            <p>We collect the following categories of information:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Account Information:</strong> Name, email address, and authentication credentials provided during registration or Single Sign-On (SSO).</li>
                <li><strong className="text-foreground">Contact Information:</strong> Phone numbers (including WhatsApp-enabled mobile numbers), email addresses, and device tokens provided by you or your end users for the purpose of receiving notifications.</li>
                <li><strong className="text-foreground">Application Data:</strong> App names, API keys (stored in hashed form), provider configurations, and organization settings.</li>
                <li><strong className="text-foreground">Notification Content:</strong> Message titles, body text, media URLs, template parameters, and metadata submitted through our APIs for delivery. This content is processed for delivery and retained for audit logging.</li>
                <li><strong className="text-foreground">Delivery &amp; Messaging Data:</strong> Message delivery statuses (sent, delivered, read, failed), timestamps, provider message identifiers, and error codes returned by delivery channels including WhatsApp, email, SMS, and push notification services.</li>
                <li><strong className="text-foreground">Usage Data:</strong> API call logs, feature usage patterns, error logs, and performance metrics.</li>
                <li><strong className="text-foreground">Technical Data:</strong> IP addresses, browser type, operating system, and device information collected via standard web server logs.</li>
            </ul>
        </PolicySection>

        <PolicySection id="how-we-use-information" title="3. How We Use Information">
            <p>We use the information we collect for the following purposes:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li>To provide, operate, and maintain the notification delivery Service across all supported channels.</li>
                <li>To process and deliver notifications on your behalf via configured providers, including WhatsApp (via Meta WhatsApp Cloud API), email, SMS, and push notification services.</li>
                <li>To authenticate users and enforce role-based access controls and multi-tenant isolation.</li>
                <li>To monitor service health, detect abuse, prevent fraud, and ensure platform security.</li>
                <li>To provide analytics dashboards, delivery reports, and usage statistics.</li>
                <li>To communicate important service updates, security notices, and policy changes.</li>
                <li>To comply with legal obligations and respond to lawful requests from authorities.</li>
            </ul>
        </PolicySection>

        <PolicySection id="whatsapp-data-sharing" title="4. WhatsApp Messaging &amp; Data Sharing with Meta">
            <p>
                When you use our Service to send notifications via WhatsApp, the following data is transmitted to <strong className="text-foreground">Meta Platforms, Inc.</strong> (&quot;Meta&quot;) through the WhatsApp Cloud API:
            </p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Recipient phone numbers</strong> — the WhatsApp-enabled mobile number of the intended recipient.</li>
                <li><strong className="text-foreground">Message content</strong> — text, media, and template parameters included in the notification.</li>
                <li><strong className="text-foreground">Message metadata</strong> — timestamps, message identifiers, delivery status updates (sent, delivered, read, failed).</li>
            </ul>
            <p>
                This data is processed by Meta in accordance with the <a href="https://www.whatsapp.com/legal/business-data-processing-terms" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">WhatsApp Business Data Processing Terms</a> and <a href="https://www.facebook.com/privacy/policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">Meta Privacy Policy</a>. We do not control how Meta processes data after transmission and recommend reviewing their policies.
            </p>
            <PolicyCallout>
                We use the WhatsApp channel solely for the purpose of delivering notifications on behalf of our customers. We do not use WhatsApp data for advertising, profiling, or any purpose other than message delivery and delivery status tracking.
            </PolicyCallout>
        </PolicySection>

        <PolicySection id="third-party-providers" title="5. Third-Party Service Providers">
            <p>To deliver notifications across multiple channels, we share relevant data with the following categories of third-party service providers:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">WhatsApp / Meta Platforms, Inc.</strong> — for WhatsApp message delivery via the WhatsApp Cloud API.</li>
                <li><strong className="text-foreground">Email providers</strong> — SMTP servers, SendGrid, Mailgun, Postmark, Resend, Amazon SES for email delivery.</li>
                <li><strong className="text-foreground">SMS providers</strong> — Twilio, Vonage for SMS and voice delivery.</li>
                <li><strong className="text-foreground">Push notification services</strong> — Apple Push Notification service (APNs), Firebase Cloud Messaging (FCM).</li>
                <li><strong className="text-foreground">Chat platforms</strong> — Slack, Discord, Microsoft Teams for chat-based notifications.</li>
            </ul>
            <p>
                Each provider processes data according to their own privacy policies and data processing agreements. We transmit only the minimum data necessary for message delivery. <strong className="text-foreground">We do not sell, rent, or trade your personal data or your end users&apos; personal data to any third party.</strong>
            </p>
        </PolicySection>

        <PolicySection id="consent-opt-in" title="6. Consent &amp; Opt-In for Messaging">
            <p>Our customers (&quot;you&quot;) are responsible for obtaining proper consent from your end users before sending them notifications through the Service, including but not limited to:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">WhatsApp:</strong> You must obtain explicit opt-in consent from recipients before sending WhatsApp messages, in compliance with the <a href="https://www.whatsapp.com/legal/business-policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">WhatsApp Business Policy</a> and <a href="https://www.whatsapp.com/legal/commerce-policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">WhatsApp Commerce Policy</a>.</li>
                <li><strong className="text-foreground">Email:</strong> Recipients must have opted in to receive email communications in compliance with applicable laws (CAN-SPAM, GDPR, PECR).</li>
                <li><strong className="text-foreground">SMS:</strong> You must comply with TCPA, TRAI, and other applicable telecom regulations regarding SMS consent.</li>
            </ul>
            <p>You must maintain records of consent and provide a clear mechanism for recipients to opt out of receiving messages at any time.</p>
        </PolicySection>

        <PolicySection id="opt-out" title="7. Opt-Out &amp; Unsubscribe">
            <p>
                End users who no longer wish to receive notifications may opt out by contacting the business that sent the notification, or by contacting us directly at <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>. For WhatsApp messages, recipients can also block the sending number directly from the WhatsApp application. We process opt-out requests within 48 hours and ensure no further messages are sent to opted-out recipients.
            </p>
        </PolicySection>

        <PolicySection id="data-retention" title="8. Data Retention">
            <p>We retain data for the following periods:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Notification content and delivery logs:</strong> 90 days from the date of delivery, after which they are automatically deleted.</li>
                <li><strong className="text-foreground">Account information:</strong> Retained for the duration of your active account.</li>
                <li><strong className="text-foreground">Phone numbers and contact data:</strong> Retained as long as the associated user profile exists in the system.</li>
                <li><strong className="text-foreground">Audit logs:</strong> Retained for 12 months for security and compliance purposes.</li>
            </ul>
            <p>Upon account deletion, all personal information is purged within 30 days, except where retention is required by applicable law or regulation.</p>
        </PolicySection>

        <PolicySection id="data-security" title="9. Data Security">
            <p>
                We implement industry-standard security measures to protect your data, including: encryption in transit (TLS 1.2+), hashed API keys, role-based access controls (RBAC), multi-tenant data isolation, audit logging of all administrative actions, and regular security assessments. Despite these measures, no system is completely secure, and we cannot guarantee absolute security.
            </p>
        </PolicySection>

        <PolicySection id="your-rights" title="10. Your Rights">
            <p>Depending on your jurisdiction, you may have the following rights regarding your personal data:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Right of Access:</strong> Request a copy of the personal data we hold about you.</li>
                <li><strong className="text-foreground">Right to Rectification:</strong> Request correction of inaccurate or incomplete data.</li>
                <li><strong className="text-foreground">Right to Erasure:</strong> Request deletion of your personal data, subject to legal retention requirements.</li>
                <li><strong className="text-foreground">Right to Data Portability:</strong> Request an export of your data in a machine-readable format.</li>
                <li><strong className="text-foreground">Right to Object:</strong> Object to the processing of your personal data for specific purposes.</li>
                <li><strong className="text-foreground">Right to Withdraw Consent:</strong> Withdraw consent for data processing at any time.</li>
            </ul>
            <p>To exercise any of these rights, contact us at <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>. We will respond to your request within 30 days.</p>
        </PolicySection>

        <PolicySection id="international-transfers" title="11. International Data Transfers">
            <p>
                Our Service may involve the transfer of data to servers and third-party service providers located outside your country of residence, including the United States (Meta Platforms, Inc.) and India (our primary operations). We ensure that such transfers are conducted in compliance with applicable data protection laws and that appropriate safeguards are in place.
            </p>
        </PolicySection>

        <PolicySection id="childrens-privacy" title="12. Children's Privacy">
            <p>
                Our Service is not directed to individuals under the age of 18. We do not knowingly collect personal information from children. If we become aware that we have inadvertently collected data from a child, we will take steps to delete that information promptly.
            </p>
        </PolicySection>

        <PolicySection id="changes" title="13. Changes to This Policy">
            <p>
                We may update this Privacy Policy from time to time. We will notify you of material changes via email or dashboard notification at least 30 days before they take effect. The &quot;Last updated&quot; date at the top reflects the most recent revision. Continued use of the Service after changes take effect constitutes acceptance of the revised policy.
            </p>
        </PolicySection>

        <PolicySection id="contact" title="14. Contact Us">
            <p>For privacy-related inquiries, data access requests, or complaints, contact us at:</p>
            <div className="mt-3 rounded-lg border border-border/70 bg-muted/50 px-4 py-3">
                <p className="font-semibold text-foreground">BUDDHICINTAKA PVT. LTD.</p>
                <p>Email: <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span></p>
                <p>Service: FreeRange Notify</p>
            </div>
        </PolicySection>
    </PolicyLayout>
);

export default PrivacyPolicy;

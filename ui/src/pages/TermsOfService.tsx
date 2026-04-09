import React from 'react';
import PolicyLayout, { PolicySection } from '../components/PolicyLayout';

const toc = [
    { id: 'acceptance', label: 'Acceptance of Terms' },
    { id: 'description', label: 'Description of Service' },
    { id: 'registration', label: 'Account Registration' },
    { id: 'content', label: 'Content Responsibility' },
    { id: 'whatsapp', label: 'WhatsApp Messaging Terms' },
    { id: 'aup', label: 'Acceptable Use' },
    { id: 'limits', label: 'API Usage & Rate Limits' },
    { id: 'privacy', label: 'Privacy' },
    { id: 'ip', label: 'Intellectual Property' },
    { id: 'termination', label: 'Termination' },
    { id: 'warranty', label: 'Disclaimer of Warranties' },
    { id: 'liability', label: 'Limitation of Liability' },
    { id: 'indemnification', label: 'Indemnification' },
    { id: 'modifications', label: 'Modifications' },
    { id: 'law', label: 'Governing Law' },
    { id: 'contact', label: 'Contact' },
];

const TermsOfService: React.FC = () => (
    <PolicyLayout
        title="Terms of Service"
        subtitle="By using FreeRange Notify you agree to the following terms. Please review them carefully before using the Service."
        lastUpdated="April 9, 2026"
        toc={toc}
    >
        <PolicySection id="acceptance" title="1. Acceptance of Terms">
            <p>
                By accessing or using FreeRange Notify (&quot;the Service&quot;), operated by BUDDHICINTAKA PVT. LTD. (&quot;the Company&quot;, &quot;we&quot;, &quot;us&quot;, &quot;our&quot;), you agree to be bound by these Terms of Service (&quot;Terms&quot;). If you do not agree to these Terms, do not use the Service. These Terms constitute a legally binding agreement between you and the Company.
            </p>
        </PolicySection>

        <PolicySection id="description" title="2. Description of Service">
            <p>
                FreeRange Notify is a multi-channel notification infrastructure platform that enables businesses and developers to send notifications across multiple channels including email, SMS, push notifications, webhooks, WhatsApp, Slack, Discord, Microsoft Teams, and real-time Server-Sent Events (SSE). The Service provides APIs, SDKs, a management dashboard, workflow automation, and integration tools for notification delivery at scale.
            </p>
            <p>
                The Service is designed for businesses, applications, and not-for-profit organizations that need to notify their users, customers, and stakeholders. FreeRange Notify is a notification delivery infrastructure — it is not a consumer messaging platform like WhatsApp, Telegram, or Signal. The Service facilitates one-way and transactional notifications on behalf of its customers, not peer-to-peer messaging.
            </p>
        </PolicySection>

        <PolicySection id="registration" title="3. Account Registration">
            <p>
                You must register for an account to use the Service. You agree to provide accurate, current, and complete information during registration and to keep your account information updated. You are solely responsible for maintaining the confidentiality of your API keys, access tokens, and account credentials. You must notify us immediately at <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span> if you suspect unauthorized access to your account.
            </p>
        </PolicySection>

        <PolicySection id="content" title="4. Content Responsibility">
            <p>
                You are solely responsible for all content transmitted through the Service, including notification titles, body text, images, URLs, media attachments, and template parameters. The Company acts as a delivery infrastructure provider and does not review, endorse, or assume liability for user-generated content. You represent and warrant that your content complies with all applicable laws, regulations, and our Acceptable Use Policy.
            </p>
        </PolicySection>

        <PolicySection id="whatsapp" title="5. WhatsApp Messaging Terms">
            <p className="mb-3">When using the Service to send messages via WhatsApp, the following additional terms apply:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Consent Requirement:</strong> You must obtain explicit, documented opt-in consent from each recipient before sending them WhatsApp messages through the Service. You are responsible for maintaining records of such consent.</li>
                <li><strong className="text-foreground">WhatsApp Policies:</strong> You must comply with the <a href="https://www.whatsapp.com/legal/business-policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">WhatsApp Business Policy</a>, <a href="https://www.whatsapp.com/legal/commerce-policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">WhatsApp Commerce Policy</a>, and all applicable Meta platform policies.</li>
                <li><strong className="text-foreground">Template Messages:</strong> Business-initiated WhatsApp messages must use pre-approved message templates as required by Meta. You are responsible for submitting templates for approval and ensuring their content complies with Meta&apos;s guidelines.</li>
                <li><strong className="text-foreground">Opt-Out Mechanism:</strong> You must provide recipients with a clear and accessible way to opt out of receiving WhatsApp messages. You must honor all opt-out requests within 48 hours.</li>
                <li><strong className="text-foreground">Prohibited Content:</strong> You must not use the WhatsApp channel to send spam, unsolicited marketing, phishing attempts, malware links, or content that violates WhatsApp&apos;s policies.</li>
                <li><strong className="text-foreground">Data Sharing:</strong> You acknowledge that WhatsApp message data (recipient phone numbers, message content, delivery status) is transmitted to and processed by Meta Platforms, Inc. in accordance with their data processing terms.</li>
            </ul>
        </PolicySection>

        <PolicySection id="aup" title="6. Acceptable Use">
            <p>
                Your use of the Service is subject to our <a href="/acceptable-use" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors">Acceptable Use Policy</a>, which is incorporated by reference into these Terms. Violation of the Acceptable Use Policy may result in immediate suspension or termination of your account without notice or refund.
            </p>
        </PolicySection>

        <PolicySection id="limits" title="7. API Usage &amp; Rate Limits">
            <p>
                The Service enforces rate limits, throughput caps, and usage quotas based on your subscription tier. You agree not to circumvent or attempt to bypass these limits. Exceeding documented limits may result in throttling, temporary suspension, or additional charges as applicable.
            </p>
        </PolicySection>

        <PolicySection id="privacy" title="8. Privacy">
            <p>
                Your use of the Service is also governed by our <a href="/privacy" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors">Privacy Policy</a>, which describes how we collect, use, store, and protect your information. By using the Service, you consent to the data practices described in the Privacy Policy.
            </p>
        </PolicySection>

        <PolicySection id="ip" title="9. Intellectual Property">
            <p>
                The Service, including its software, APIs, documentation, branding, and user interface, is the intellectual property of BUDDHICINTAKA PVT. LTD. You are granted a limited, non-exclusive, non-transferable license to use the Service in accordance with these Terms. You retain ownership of all content you transmit through the Service.
            </p>
        </PolicySection>

        <PolicySection id="termination" title="10. Termination">
            <p>
                The Company reserves the right to suspend or terminate your account at any time, with or without notice, for conduct that violates these Terms, the Acceptable Use Policy, applicable WhatsApp or Meta platform policies, or is harmful to other users, the Company, or third parties. Upon termination, your right to use the Service ceases immediately. Data associated with your account will be handled in accordance with our Privacy Policy.
            </p>
        </PolicySection>

        <PolicySection id="warranty" title="11. Disclaimer of Warranties">
            <p>
                The Service is provided &quot;AS IS&quot; and &quot;AS AVAILABLE&quot; without warranties of any kind, whether express or implied, including but not limited to implied warranties of merchantability, fitness for a particular purpose, and non-infringement. The Company does not warrant that the Service will be uninterrupted, error-free, or secure.
            </p>
        </PolicySection>

        <PolicySection id="liability" title="12. Limitation of Liability">
            <p>
                To the maximum extent permitted by law, the Company shall not be liable for any indirect, incidental, special, consequential, or punitive damages, including loss of revenue, data, or business opportunities, arising from your use of the Service. The Company&apos;s total liability shall not exceed the amount paid by you in the twelve (12) months preceding the claim.
            </p>
        </PolicySection>

        <PolicySection id="indemnification" title="13. Indemnification">
            <p>
                You agree to indemnify, defend, and hold harmless the Company and its employees, officers, and agents from and against any claims, damages, losses, or expenses (including legal fees) arising from your use of the Service, your content, your violation of these Terms, or your violation of any third-party rights, including WhatsApp and Meta platform policies.
            </p>
        </PolicySection>

        <PolicySection id="modifications" title="14. Modifications">
            <p>
                The Company reserves the right to modify these Terms at any time. Material changes will be communicated via email or dashboard notification at least 30 days before they take effect. Continued use of the Service after changes take effect constitutes acceptance of the revised Terms.
            </p>
        </PolicySection>

        <PolicySection id="law" title="15. Governing Law">
            <p>
                These Terms shall be governed by and construed in accordance with the laws of India, including the Information Technology Act, 2000 and applicable data protection regulations. Any disputes arising under these Terms shall be subject to the exclusive jurisdiction of the courts in Bengaluru, Karnataka, India.
            </p>
        </PolicySection>

        <PolicySection id="contact" title="16. Contact">
            <p>For questions about these Terms, contact us at:</p>
            <div className="mt-3 rounded-lg border border-border/70 bg-muted/50 px-4 py-3">
                <p className="font-semibold text-foreground">BUDDHICINTAKA PVT. LTD.</p>
                <p>Email: <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span></p>
                <p>Service: FreeRange Notify</p>
            </div>
        </PolicySection>
    </PolicyLayout>
);

export default TermsOfService;

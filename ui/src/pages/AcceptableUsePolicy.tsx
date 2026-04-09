import React from 'react';
import PolicyLayout, { PolicySection, PolicyCallout } from '../components/PolicyLayout';

const toc = [
    { id: 'prohibited-content', label: 'Prohibited Content' },
    { id: 'prohibited-activities', label: 'Prohibited Activities' },
    { id: 'whatsapp-requirements', label: 'WhatsApp Requirements' },
    { id: 'compliance', label: 'Compliance Requirements' },
    { id: 'enforcement', label: 'Monitoring & Enforcement' },
    { id: 'abuse', label: 'Reporting Abuse' },
    { id: 'changes', label: 'Changes to This Policy' },
    { id: 'contact', label: 'Contact' },
];

const AcceptableUsePolicy: React.FC = () => (
    <PolicyLayout
        title="Acceptable Use Policy"
        subtitle="This policy governs how you may use FreeRange Notify. Violations may result in immediate account suspension or termination."
        lastUpdated="April 9, 2026"
        toc={toc}
    >
        <PolicySection id="prohibited-content" title="1. Prohibited Content">
            <PolicyCallout variant="destructive">
                Violation of this section may result in immediate account termination without notice or refund.
            </PolicyCallout>
            <p className="mt-3">You may not use the Service to create, transmit, or deliver content that:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li>Is sexually explicit, pornographic, or obscene (NSFW content).</li>
                <li>Promotes or incites violence, terrorism, or physical harm against any individual or group.</li>
                <li>Contains hate speech, harassment, bullying, or discriminatory content based on race, gender, religion, sexual orientation, disability, or any other protected characteristic.</li>
                <li>Is defamatory, libelous, or knowingly false.</li>
                <li>Involves child sexual abuse material (CSAM) or exploitation of minors in any form.</li>
                <li>Promotes illegal drugs, weapons, or controlled substances.</li>
                <li>Infringes on intellectual property rights, copyrights, trademarks, or trade secrets.</li>
            </ul>
        </PolicySection>

        <PolicySection id="prohibited-activities" title="2. Prohibited Activities">
            <PolicyCallout variant="destructive">
                Attempts to circumvent platform safeguards may be reported to law enforcement.
            </PolicyCallout>
            <p className="mt-3">You may not use the Service to:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Send spam or unsolicited messages:</strong> All notification recipients must have a prior relationship with the sender or have explicitly opted in to receive messages.</li>
                <li><strong className="text-foreground">Conduct phishing or fraud:</strong> Impersonate other individuals, companies, or services to deceive recipients.</li>
                <li><strong className="text-foreground">Distribute malware:</strong> Send notifications containing links to malicious software, ransomware, or exploit kits.</li>
                <li><strong className="text-foreground">Harvest data:</strong> Use the Service to collect personal information of recipients without their consent.</li>
                <li><strong className="text-foreground">Circumvent security controls:</strong> Attempt to bypass rate limits, authentication, licensing, or other platform safeguards.</li>
                <li><strong className="text-foreground">Abuse shared infrastructure:</strong> Conduct activities that degrade service performance for other users, including denial-of-service patterns.</li>
                <li><strong className="text-foreground">Violate messaging regulations:</strong> Send messages in violation of CAN-SPAM Act (US), GDPR (EU), TCPA (US), PECR (UK), IT Act 2000 (India), TRAI regulations, or any other applicable electronic communications law.</li>
                <li><strong className="text-foreground">Violate platform policies:</strong> Send messages via WhatsApp, Slack, Discord, Teams, or other platforms in violation of their respective terms of service, business policies, or commerce policies.</li>
            </ul>
        </PolicySection>

        <PolicySection id="whatsapp-requirements" title="3. WhatsApp-Specific Requirements">
            <p>When using the WhatsApp channel, you must additionally:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li>Obtain and maintain documented opt-in consent from each recipient before sending WhatsApp messages.</li>
                <li>Use only Meta-approved message templates for business-initiated conversations.</li>
                <li>Provide a clear opt-out mechanism and honor opt-out requests within 48 hours.</li>
                <li>Not send messages outside the categories permitted by your approved WhatsApp message templates.</li>
                <li>Comply with the <a href="https://www.whatsapp.com/legal/business-policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">WhatsApp Business Policy</a> and <a href="https://www.whatsapp.com/legal/commerce-policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">WhatsApp Commerce Policy</a> at all times.</li>
                <li>Not use the WhatsApp channel for promotional messaging to users who have not explicitly requested such communications.</li>
            </ul>
        </PolicySection>

        <PolicySection id="compliance" title="4. Compliance Requirements">
            <ul className="list-disc pl-5 space-y-2">
                <li>You must maintain valid opt-in consent records for all notification recipients across all channels.</li>
                <li>Every email notification must include a functional unsubscribe mechanism.</li>
                <li>SMS notifications must comply with carrier guidelines and include opt-out instructions.</li>
                <li>WhatsApp messages must comply with Meta&apos;s template and messaging policies.</li>
                <li>You must promptly honor unsubscribe and opt-out requests across all channels.</li>
                <li>You must secure your API keys and access tokens, and immediately report any suspected compromise.</li>
            </ul>
        </PolicySection>

        <PolicySection id="enforcement" title="5. Monitoring &amp; Enforcement">
            <p>
                We reserve the right to monitor usage patterns, review notification content, and investigate suspected violations of this AUP. We may take any of the following actions at our sole discretion:
            </p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Warning:</strong> Notification of the violation with a request to remediate.</li>
                <li><strong className="text-foreground">Throttling:</strong> Temporary reduction of sending limits.</li>
                <li><strong className="text-foreground">Suspension:</strong> Temporary deactivation of your account or specific applications.</li>
                <li><strong className="text-foreground">Termination:</strong> Permanent removal of your account and all associated data.</li>
                <li><strong className="text-foreground">Legal action:</strong> Referral to law enforcement or initiation of legal proceedings where warranted.</li>
            </ul>
        </PolicySection>

        <PolicySection id="abuse" title="6. Reporting Abuse">
            <p>
                If you receive unwanted or abusive notifications sent through FreeRange Notify, or if you become aware of a violation of this AUP, please report it to <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>. Include any relevant details such as the notification content, sender information, and timestamps.
            </p>
        </PolicySection>

        <PolicySection id="changes" title="7. Changes to This Policy">
            <p>
                We may update this AUP at any time. Material changes will be communicated via email or dashboard notification. Continued use of the Service after changes take effect constitutes acceptance of the revised policy.
            </p>
        </PolicySection>

        <PolicySection id="contact" title="8. Contact">
            <p>For questions about this policy, contact us at:</p>
            <div className="mt-3 rounded-lg border border-border/70 bg-muted/50 px-4 py-3">
                <p className="font-semibold text-foreground">BUDDHICINTAKA PVT. LTD.</p>
                <p>Email: <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span></p>
                <p>Service: FreeRange Notify</p>
            </div>
        </PolicySection>
    </PolicyLayout>
);

export default AcceptableUsePolicy;

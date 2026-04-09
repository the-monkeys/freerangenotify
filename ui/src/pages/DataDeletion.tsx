import React from 'react';
import PolicyLayout, { PolicySection, PolicyCallout } from '../components/PolicyLayout';

const toc = [
    { id: 'app-deletion', label: 'Application Deletion' },
    { id: 'account-deletion', label: 'Account & Organization Deletion' },
    { id: 'manual-request', label: 'Manual Deletion Requests' },
    { id: 'retained', label: 'Data That May Be Retained' },
    { id: 'whatsapp', label: 'WhatsApp Data Deletion' },
    { id: 'contact', label: 'Contact' },
];

const DataDeletion: React.FC = () => (
    <PolicyLayout
        title="Data Deletion"
        subtitle="FreeRange Notify provides built-in data deletion through the platform. When you delete an application or your account, all associated data is permanently and irreversibly removed."
        lastUpdated="April 9, 2026"
        toc={toc}
    >
        <PolicySection id="app-deletion" title="1. Application Deletion">
            <p>
                When you delete an application from FreeRange Notify, <strong className="text-foreground">all data related to that application is permanently deleted</strong>. This includes:
            </p>
            <ul className="list-disc pl-5 space-y-2">
                <li>All notification history and delivery logs for that application</li>
                <li>All users/subscribers registered under that application</li>
                <li>All templates, workflows, and digest rules configured for the application</li>
                <li>API keys and provider configurations (WhatsApp, email, SMS, etc.)</li>
                <li>Analytics and usage data associated with the application</li>
                <li>Topic subscriptions and webhook configurations</li>
            </ul>
            <PolicyCallout variant="destructive">
                This action is irreversible. Deleted application data cannot be recovered.
            </PolicyCallout>
        </PolicySection>

        <PolicySection id="account-deletion" title="2. Account &amp; Organization Deletion">
            <p>
                When an admin deletes their account, <strong className="text-foreground">all organization data is permanently deleted</strong>. This is a complete removal that includes:
            </p>
            <ul className="list-disc pl-5 space-y-2">
                <li>The admin account and all associated user accounts</li>
                <li>All applications owned by the organization</li>
                <li>All notification content, delivery logs, and audit trails</li>
                <li>All templates, workflows, topics, and digest rules</li>
                <li>All API keys, provider configurations, and credentials</li>
                <li>All subscriber/user data across every application</li>
                <li>All analytics, usage data, and billing records</li>
            </ul>
            <PolicyCallout variant="destructive">
                This action is irreversible. The entire organization&apos;s data is permanently removed and cannot be recovered from any system.
            </PolicyCallout>
        </PolicySection>

        <PolicySection id="manual-request" title="3. Manual Deletion Requests">
            <p>If you are unable to delete your data through the platform, or if you are an end user who received notifications and want your contact data removed, you can request deletion by email:</p>
            <div className="mt-3 rounded-lg border border-border/70 bg-muted/50 px-4 py-3 mb-4">
                <p className="font-semibold text-foreground">monkeys.admin@monkeys.com.co</p>
                <p className="mt-1">Subject: <span className="text-foreground font-medium">Data Deletion Request</span></p>
            </div>
            <p>Include the following in your request:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Your email or phone number</strong> — so we can locate your data in our systems.</li>
                <li><strong className="text-foreground">Scope of deletion</strong> — whether you want all your data removed or specific records (e.g., notification history, contact information).</li>
            </ul>
            <p className="mt-3">
                We will acknowledge your request within 48 hours and complete the deletion within 30 days.
            </p>
        </PolicySection>

        <PolicySection id="retained" title="4. Data That May Be Retained">
            <p>Certain data may be retained after a deletion request in the following circumstances:</p>
            <ul className="list-disc pl-5 space-y-2">
                <li><strong className="text-foreground">Legal obligations:</strong> Data required for compliance with applicable laws, regulations, or legal proceedings.</li>
                <li><strong className="text-foreground">Fraud prevention:</strong> Limited data retained to prevent abuse, fraud, or security incidents.</li>
                <li><strong className="text-foreground">Anonymized data:</strong> Aggregated, anonymized analytics data that cannot be linked back to you may be retained for service improvement.</li>
                <li><strong className="text-foreground">Backup systems:</strong> Data in encrypted backups will be overwritten through the normal backup rotation cycle within 90 days.</li>
            </ul>
        </PolicySection>

        <PolicySection id="whatsapp" title="5. WhatsApp Data Deletion">
            <p>
                If your phone number was used to receive WhatsApp notifications through FreeRange Notify, we will delete your phone number and associated message delivery records from our systems upon request. Note that messages already delivered to your WhatsApp application are stored on your device and by Meta — we cannot delete messages from Meta&apos;s servers or your device. To manage messages on Meta&apos;s side, refer to <a href="https://www.facebook.com/privacy/policy/" className="text-foreground underline underline-offset-2 hover:text-accent transition-colors" target="_blank" rel="noopener noreferrer">Meta&apos;s Privacy Policy</a>.
            </p>
        </PolicySection>

        <PolicySection id="contact" title="6. Contact">
            <p>For any questions about data deletion, contact us at:</p>
            <div className="mt-3 rounded-lg border border-border/70 bg-muted/50 px-4 py-3">
                <p className="font-semibold text-foreground">BUDDHICINTAKA PVT. LTD.</p>
                <p>Email: <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span></p>
                <p>Service: FreeRange Notify</p>
            </div>
        </PolicySection>
    </PolicyLayout>
);

export default DataDeletion;

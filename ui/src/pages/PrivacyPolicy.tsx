import React from 'react';
import Header from '../components/Header';
import Footer from '../components/Footer';

const PrivacyPolicy: React.FC = () => {
    return (
        <div className="bg-background min-h-screen flex flex-col">
            <Header />
            <main className="flex-1 py-12 sm:py-20">
                <div className="max-w-3xl mx-auto px-4 sm:px-8">
                    <h1 className="text-3xl font-bold text-foreground mb-2">Privacy Policy</h1>
                    <p className="text-sm text-muted-foreground mb-8">Last updated: March 14, 2026</p>

                    <div className="prose prose-sm text-foreground space-y-6">
                        <section>
                            <h2 className="text-xl font-semibold mb-3">1. Introduction</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                BUDDHICINTAKA PVT. LTD. ("the Company", "we", "us") operates FreeRangeNotify. This Privacy Policy explains how we collect, use, store, and protect information when you use our Service.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">2. Information We Collect</h2>
                            <p className="text-muted-foreground leading-relaxed mb-3">We collect the following categories of information:</p>
                            <ul className="list-disc pl-6 text-muted-foreground space-y-2">
                                <li><strong className="text-foreground">Account Information:</strong> Name, email address, and authentication credentials provided during registration.</li>
                                <li><strong className="text-foreground">Application Data:</strong> App names, API keys (stored hashed), provider configurations, and organization settings.</li>
                                <li><strong className="text-foreground">Notification Content:</strong> Message titles, body text, and metadata submitted through our APIs for delivery. This content is stored temporarily for delivery processing and audit logging.</li>
                                <li><strong className="text-foreground">Usage Data:</strong> API call logs, delivery statuses, error logs, and performance metrics.</li>
                                <li><strong className="text-foreground">Technical Data:</strong> IP addresses, browser type, and device information collected via standard web server logs.</li>
                            </ul>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">3. How We Use Information</h2>
                            <ul className="list-disc pl-6 text-muted-foreground space-y-2">
                                <li>To provide, operate, and maintain the notification delivery Service.</li>
                                <li>To process and deliver notifications on your behalf via configured providers.</li>
                                <li>To authenticate users and enforce access controls.</li>
                                <li>To monitor service health, detect abuse, and ensure platform security.</li>
                                <li>To provide analytics dashboards and delivery reports.</li>
                                <li>To communicate important service updates and security notices.</li>
                            </ul>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">4. Data Retention</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                Notification content and delivery logs are retained for 90 days by default. Account information is retained for the duration of your account. Upon account deletion, personal information is purged within 30 days, except where retention is required by law.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">5. Third-Party Providers</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                To deliver notifications, we transmit content to third-party delivery providers (e.g., email services, SMS gateways, push notification services) as configured by you. These providers process data according to their own privacy policies. We do not sell your data to third parties.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">6. Data Security</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                We implement industry-standard security measures including encryption in transit (TLS), hashed API keys, role-based access controls, and audit logging. However, no system is completely secure, and we cannot guarantee absolute security.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">7. Your Rights</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                Depending on your jurisdiction, you may have rights to access, correct, delete, or export your personal data. To exercise these rights, contact us at <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">8. Changes to This Policy</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                We may update this Privacy Policy from time to time. We will notify you of material changes via email or dashboard notification. The "Last updated" date at the top reflects the most recent revision.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">9. Contact</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                For privacy-related inquiries, contact us at <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>.
                            </p>
                        </section>
                    </div>
                </div>
            </main>
            <Footer />
        </div>
    );
};

export default PrivacyPolicy;

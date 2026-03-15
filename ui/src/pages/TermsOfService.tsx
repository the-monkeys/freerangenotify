import React from 'react';
import Header from '../components/Header';
import Footer from '../components/Footer';

const TermsOfService: React.FC = () => {
    return (
        <div className="bg-background min-h-screen flex flex-col">
            <Header />
            <main className="flex-1 py-12 sm:py-20">
                <div className="max-w-3xl mx-auto px-4 sm:px-8">
                    <h1 className="text-3xl font-bold text-foreground mb-2">Terms of Service</h1>
                    <p className="text-sm text-muted-foreground mb-8">Last updated: March 14, 2026</p>

                    <div className="prose prose-sm text-foreground space-y-6">
                        <section>
                            <h2 className="text-xl font-semibold mb-3">1. Acceptance of Terms</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                By accessing or using FreeRangeNotify ("the Service"), operated by BUDDHICINTAKA PVT. LTD. ("the Company"), you agree to be bound by these Terms of Service. If you do not agree to these terms, do not use the Service.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">2. Description of Service</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                FreeRangeNotify is a notification infrastructure platform that enables users and businesses to send notifications across multiple channels including email, SMS, push notifications, webhooks, WhatsApp, and real-time Server-Sent Events. The Service provides APIs, a management dashboard, and integration tools.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">3. Account Registration</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                You must register for an account to use the Service. You agree to provide accurate, current, and complete information during registration and to keep your account information updated. You are responsible for maintaining the confidentiality of your API keys and account credentials.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">4. Content Responsibility</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                You are solely responsible for all content transmitted through the Service, including notification titles, body text, images, URLs, and attachments. The Company acts as a delivery infrastructure provider and does not review, endorse, or assume liability for user-generated content. You represent and warrant that your content complies with all applicable laws, regulations, and our Acceptable Use Policy.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">5. Acceptable Use</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                Your use of the Service is subject to our <a href="/acceptable-use" className="text-foreground underline hover:text-accent">Acceptable Use Policy</a>, which is incorporated by reference into these Terms. Violation of the AUP may result in immediate suspension or termination of your account without notice or refund.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">6. API Usage & Rate Limits</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                The Service enforces rate limits, throughput caps, and usage quotas based on your subscription tier. You agree not to circumvent or attempt to bypass these limits. Exceeding documented limits may result in throttling, temporary suspension, or additional charges.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">7. Termination</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                The Company reserves the right to suspend or terminate your account at any time, with or without notice, for conduct that violates these Terms, the Acceptable Use Policy, or is harmful to other users, the Company, or third parties. Upon termination, your right to use the Service ceases immediately.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">8. Limitation of Liability</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                To the maximum extent permitted by law, the Company shall not be liable for any indirect, incidental, special, consequential, or punitive damages, including loss of revenue, data, or business opportunities, arising from your use of the Service. The Company's total liability shall not exceed the amount paid by you in the twelve (12) months preceding the claim.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">9. Indemnification</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                You agree to indemnify, defend, and hold harmless the Company and its employees, officers, and agents from and against any claims, damages, losses, or expenses (including legal fees) arising from your use of the Service, your content, or your violation of these Terms.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">10. Modifications</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                The Company reserves the right to modify these Terms at any time. Material changes will be communicated via email or dashboard notification at least 30 days before they take effect. Continued use of the Service after changes take effect constitutes acceptance of the revised Terms.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">11. Governing Law</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                These Terms shall be governed by and construed in accordance with the laws of India. Any disputes arising under these Terms shall be subject to the exclusive jurisdiction of the courts in Bengaluru, Karnataka.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">12. Contact</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                For questions about these Terms, contact us at <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>.
                            </p>
                        </section>
                    </div>
                </div>
            </main>
            <Footer />
        </div>
    );
};

export default TermsOfService;

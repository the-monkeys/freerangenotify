import React from 'react';
import Header from '../components/Header';
import Footer from '../components/Footer';

const AcceptableUsePolicy: React.FC = () => {
    return (
        <div className="bg-background min-h-screen flex flex-col">
            <Header />
            <main className="flex-1 py-12 sm:py-20">
                <div className="max-w-3xl mx-auto px-4 sm:px-8">
                    <h1 className="text-3xl font-bold text-foreground mb-2">Acceptable Use Policy</h1>
                    <p className="text-sm text-muted-foreground mb-8">Last updated: March 14, 2026</p>

                    <div className="prose prose-sm text-foreground space-y-6">
                        <section>
                            <p className="text-muted-foreground leading-relaxed">
                                This Acceptable Use Policy ("AUP") governs your use of the FreeRangeNotify Service operated by BUDDHICINTAKA PVT. LTD. This AUP is incorporated by reference into our <a href="/terms" className="text-foreground underline hover:text-accent">Terms of Service</a>. Violation of this AUP may result in immediate account suspension or termination without notice or refund.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">Prohibited Content</h2>
                            <p className="text-muted-foreground leading-relaxed mb-3">You may not use the Service to create, transmit, or deliver content that:</p>
                            <ul className="list-disc pl-6 text-muted-foreground space-y-2">
                                <li>Is sexually explicit, pornographic, or obscene (NSFW content).</li>
                                <li>Promotes or incites violence, terrorism, or physical harm against any individual or group.</li>
                                <li>Contains hate speech, harassment, bullying, or discriminatory content based on race, gender, religion, sexual orientation, disability, or any other protected characteristic.</li>
                                <li>Is defamatory, libelous, or knowingly false.</li>
                                <li>Involves child sexual abuse material (CSAM) or exploitation of minors in any form.</li>
                                <li>Promotes illegal drugs, weapons, or controlled substances.</li>
                                <li>Infringes on intellectual property rights, copyrights, trademarks, or trade secrets.</li>
                            </ul>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">Prohibited Activities</h2>
                            <p className="text-muted-foreground leading-relaxed mb-3">You may not use the Service to:</p>
                            <ul className="list-disc pl-6 text-muted-foreground space-y-2">
                                <li><strong className="text-foreground">Send spam or unsolicited messages:</strong> All notification recipients must have a prior relationship with the sender or have explicitly opted in to receive messages.</li>
                                <li><strong className="text-foreground">Conduct phishing or fraud:</strong> Impersonate other individuals, companies, or services to deceive recipients.</li>
                                <li><strong className="text-foreground">Distribute malware:</strong> Send notifications containing links to malicious software, ransomware, or exploit kits.</li>
                                <li><strong className="text-foreground">Harvest data:</strong> Use the Service to collect personal information of recipients without their consent.</li>
                                <li><strong className="text-foreground">Circumvent security controls:</strong> Attempt to bypass rate limits, authentication, licensing, or other platform safeguards.</li>
                                <li><strong className="text-foreground">Abuse shared infrastructure:</strong> Conduct activities that degrade service performance for other users, including denial-of-service patterns.</li>
                                <li><strong className="text-foreground">Violate messaging regulations:</strong> Send messages in violation of CAN-SPAM Act (US), GDPR (EU), TCPA (US), PECR (UK), or any other applicable electronic communications law.</li>
                            </ul>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">Compliance Requirements</h2>
                            <ul className="list-disc pl-6 text-muted-foreground space-y-2">
                                <li>You must maintain valid opt-in consent records for all notification recipients.</li>
                                <li>Every email notification must include a functional unsubscribe mechanism.</li>
                                <li>SMS notifications must comply with carrier guidelines and include opt-out instructions.</li>
                                <li>You must promptly honor unsubscribe and opt-out requests.</li>
                                <li>You must secure your API keys and immediately report any suspected compromise.</li>
                            </ul>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">Monitoring & Enforcement</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                We reserve the right to monitor usage patterns, review notification content, and investigate suspected violations of this AUP. We may take any of the following actions at our sole discretion:
                            </p>
                            <ul className="list-disc pl-6 text-muted-foreground space-y-2 mt-3">
                                <li><strong className="text-foreground">Warning:</strong> Notification of the violation with a request to remediate.</li>
                                <li><strong className="text-foreground">Throttling:</strong> Temporary reduction of sending limits.</li>
                                <li><strong className="text-foreground">Suspension:</strong> Temporary deactivation of your account or specific applications.</li>
                                <li><strong className="text-foreground">Termination:</strong> Permanent removal of your account and all associated data.</li>
                                <li><strong className="text-foreground">Legal action:</strong> Referral to law enforcement or initiation of legal proceedings where warranted.</li>
                            </ul>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">Reporting Abuse</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                If you receive unwanted or abusive notifications sent through FreeRangeNotify, or if you become aware of a violation of this AUP, please report it to <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>. Include any relevant details such as the notification content, sender information, and timestamps.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">Changes to This Policy</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                We may update this AUP at any time. Material changes will be communicated via email or dashboard notification. Continued use of the Service after changes take effect constitutes acceptance of the revised policy.
                            </p>
                        </section>

                        <section>
                            <h2 className="text-xl font-semibold mb-3">Contact</h2>
                            <p className="text-muted-foreground leading-relaxed">
                                For questions about this policy, contact us at <span className="text-foreground font-medium">monkeys.admin@monkeys.com.co</span>.
                            </p>
                        </section>
                    </div>
                </div>
            </main>
            <Footer />
        </div>
    );
};

export default AcceptableUsePolicy;

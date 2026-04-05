import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Badge } from '../components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import Header from '../components/Header';
import Footer from '../components/Footer';

const GST_RATE = 0.18;

const pricingPlans = [
    { name: 'Free Trial', tier: 'free_trial', monthlyINR: 0, highlight: false, quotas: 'Email 500 • WhatsApp 50 • SMS 50 • Push 1k' },
    { name: 'Pro', tier: 'pro', monthlyINR: 1499, highlight: true, quotas: 'Email 7.5k • WhatsApp 750 • SMS 750 • Push 15k' },
    { name: 'Growth', tier: 'growth', monthlyINR: 4999, highlight: false, quotas: 'Email 35k • WhatsApp 3k • SMS 3k • Push 60k' },
    { name: 'Scale', tier: 'scale', monthlyINR: 14999, highlight: false, quotas: 'Email 150k • WhatsApp 12k • SMS 12k • Push 250k' },
];

const formatINR = (v: number) => `₹${v.toLocaleString('en-IN', { minimumFractionDigits: 0 })}`;
const formatGST = (v: number) => `incl. GST: ${formatINR(Math.round(v * (1 + GST_RATE)))}`;

const Pricing: React.FC = () => {
    const navigate = useNavigate();
    return (
        <div className="min-h-screen bg-background text-foreground">
            <Header />
            <main className="max-w-7xl mx-auto px-4 sm:px-8 py-16">
                <div className="max-w-3xl mb-10 sm:mb-12">
                    <Badge variant="outline" className="mb-4 border-border/80 bg-background/80">Pricing</Badge>
                    <h1 className="text-3xl sm:4xl tracking-tight mb-3">Transparent monthly plans with PAYG overage</h1>
                    <p className="text-muted-foreground text-base sm:text-lg">
                        Base fees map to our billing rate card. GST at 18% is shown for India. Overage is pay-as-you-go per message beyond quota, priced to cover WhatsApp/SMS marketing costs.
                        For custom or enterprise usage, email <a className="underline" href="mailto:monkeys.admin@monkeys.com.co">monkeys.admin@monkeys.com.co</a>.
                    </p>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 sm:gap-6">
                    {pricingPlans.map((plan) => (
                        <Card key={plan.tier} className={`h-full border ${plan.highlight ? 'border-accent shadow-lg shadow-accent/10' : 'border-border/70'} bg-card/95`}>
                            <CardHeader className="space-y-2 pb-2">
                                <div className="flex items-center justify-between">
                                    <CardTitle className="text-xl">{plan.name}</CardTitle>
                                    {plan.highlight && <Badge variant="secondary">Popular</Badge>}
                                </div>
                                <CardDescription className="text-sm text-muted-foreground">
                                    {plan.quotas}
                                </CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-4">
                                <div>
                                    <div className="text-3xl font-semibold">{formatINR(plan.monthlyINR)}</div>
                                    <p className="text-sm text-muted-foreground">{formatGST(plan.monthlyINR)}</p>
                                </div>
                                <ul className="text-sm text-muted-foreground space-y-2">
                                    <li>• Included quota per channel (system creds)</li>
                                    <li>• PAYG overage (per billing rate card)</li>
                                    <li>• BYOC fees apply when you use your own creds</li>
                                    <li>• Platform (push/in-app/SSE) is free</li>
                                </ul>
                                <button
                                    className={`w-full inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium transition-colors ${plan.highlight ? 'bg-accent text-accent-foreground hover:opacity-90' : 'border border-border text-foreground hover:bg-muted'}`}
                                    onClick={() => navigate('/register')}
                                >
                                    {plan.monthlyINR === 0 ? 'Start Free Trial' : 'Choose Plan'}
                                </button>
                            </CardContent>
                        </Card>
                    ))}
                </div>
                <p className="mt-6 text-xs text-muted-foreground">
                    For enterprise or custom usage-based pricing, write to <a className="underline" href="mailto:monkeys.admin@monkeys.com.co">monkeys.admin@monkeys.com.co</a>.
                </p>
            </main>
            <Footer />
        </div>
    );
};

export default Pricing;

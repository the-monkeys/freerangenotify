import React from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'motion/react';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';

const GST_RATE = 0.18;

const pricingPlans = [
    { name: 'Free Trial', tier: 'free_trial', monthlyINR: 0, highlight: false, quotas: 'Email 500 • WhatsApp 50 • SMS 50 • Push 1k' },
    { name: 'Pro', tier: 'pro', monthlyINR: 1499, highlight: true, quotas: 'Email 7.5k • WhatsApp 750 • SMS 750 • Push 15k' },
    { name: 'Growth', tier: 'growth', monthlyINR: 4999, highlight: false, quotas: 'Email 35k • WhatsApp 3k • SMS 3k • Push 60k' },
    { name: 'Scale', tier: 'scale', monthlyINR: 14999, highlight: false, quotas: 'Email 150k • WhatsApp 12k • SMS 12k • Push 250k' },
];

const formatINR = (v: number) => `₹${v.toLocaleString('en-IN', { minimumFractionDigits: 0 })}`;
const formatGST = (v: number) => `incl. GST: ${formatINR(Math.round(v * (1 + GST_RATE)))}`;

type PricingPlansSectionProps = {
    id?: string;
    isAuthenticated?: boolean;
    className?: string;
};

const PricingPlansSection: React.FC<PricingPlansSectionProps> = ({ id, isAuthenticated = false, className = '' }) => {
    const navigate = useNavigate();

    return (
        <motion.section
            id={id}
            initial={{ opacity: 0, x: 20 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true, amount: 0.3 }}
            transition={{ duration: 0.5, ease: 'easeOut' }}
            className={`py-14 sm:py-20 bg-linear-to-b from-background via-card/30 to-background border-y border-border/60 ${className}`}
        >
            <div className="max-w-7xl mx-auto px-4 sm:px-8">
                <div className="max-w-3xl mb-10 sm:mb-12">
                    <Badge variant="outline" className="mb-4 border-border/80 bg-background/80">Pricing</Badge>
                    <h2 className="text-3xl sm:text-4xl tracking-tight mb-3">Transparent monthly plans with PAYG overage</h2>
                    <p className="text-muted-foreground text-base sm:text-lg">
                        Base fees map to our billing rate card.
                        GST at 18% is shown for India. Overage is pay-as-you-go per message beyond quota, priced to cover WhatsApp/SMS marketing costs.
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
                                    <li>• PAYG overage</li>
                                    <li>• BYOC fees apply when you use your own creds</li>
                                    <li>• Platform (push/in-app/SSE) is free</li>
                                </ul>
                                <Button
                                    className="w-full"
                                    variant={plan.highlight ? 'default' : 'outline'}
                                    onClick={() => navigate(isAuthenticated ? '/billing' : '/register')}
                                >
                                    {plan.monthlyINR === 0 ? 'Start Free Trial' : 'Choose Plan'}
                                </Button>
                            </CardContent>
                        </Card>
                    ))}
                </div>
                <p className="mt-6 text-xs text-muted-foreground">
                    GST shown at 18% for India. Outside India, local taxes may differ.
                </p>
            </div>
        </motion.section>
    );
};

export default PricingPlansSection;

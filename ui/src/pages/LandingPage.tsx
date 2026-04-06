import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { motion } from 'motion/react';
import { ArrowUpRight } from 'lucide-react';
import { Button } from '../components/ui/button';
import { Badge } from '../components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import Header from '../components/Header';
import Footer from '../components/Footer';
import HeroIllustration from './HeroIllustration';

const HERO_TYPED_PHRASES = [
    'Delivery Headaches',
    'Channel Complexity',
    'Retry Chaos',
    'Provider Lock-in'
];

const LandingPage: React.FC = () => {
    const navigate = useNavigate();
    const { isAuthenticated } = useAuth();
    const [typedText, setTypedText] = useState('');
    const [phraseIndex, setPhraseIndex] = useState(0);
    const [isDeleting, setIsDeleting] = useState(false);
    const [userCount, setUserCount] = useState<number | null>(null);

    const handleGetStarted = () => {
        if (isAuthenticated) {
            navigate('/apps');
        } else {
            navigate('/register');
        }
    };

    const showcaseProducts = [
        {
            name: 'Monkeys',
            type: 'Content Platform',
            description: 'A quality blogging community for technology, business, and science.',
            url: 'https://monkeys.com.co'
        },
        {
            name: 'Monkeys Identity',
            type: 'Identity and Access',
            description: 'A comprehensive identity management solution for secure authentication and user management.',
            url: 'https://identity.monkeys.support'
        }
    ];

    const capabilities = [
        {
            title: 'Scalable Processing',
            description: 'Queue-first architecture built for high throughput and predictable delivery latency.'
        },
        {
            title: 'Security by Default',
            description: 'API key controls, credential masking, and authenticated delivery pathways out of the box.'
        },
        {
            title: 'Real-time Visibility',
            description: 'Unified event tracking across email, push, SMS, webhook, and browser channels.'
        }
    ];

    useEffect(() => {
        const currentPhrase = HERO_TYPED_PHRASES[phraseIndex];

        const timeout = setTimeout(() => {
            if (!isDeleting) {
                const nextText = currentPhrase.slice(0, typedText.length + 1);
                setTypedText(nextText);

                if (nextText === currentPhrase) {
                    setTimeout(() => setIsDeleting(true), 1300);
                }
            } else {
                const nextText = currentPhrase.slice(0, typedText.length - 1);
                setTypedText(nextText);

                if (nextText.length === 0) {
                    setIsDeleting(false);
                    setPhraseIndex((prev) => (prev + 1) % HERO_TYPED_PHRASES.length);
                }
            }
        }, isDeleting ? 45 : 85);

        return () => clearTimeout(timeout);
    }, [typedText, isDeleting, phraseIndex]);

    useEffect(() => {
        // Fetch public stats without requiring auth; ignore failures.
        fetch('/v1/public/stats')
            .then((res) => res.ok ? res.json() : null)
            .then((data) => {
                if (data && typeof data.user_count === 'number') {
                    setUserCount(data.user_count);
                }
            })
            .catch(() => { /* ignore */ });
    }, []);

    return (
        <div className="bg-[radial-gradient(circle_at_top_left,rgba(255,85,66,0.08),transparent_45%),radial-gradient(circle_at_bottom_right,rgba(18,18,18,0.08),transparent_40%)] text-foreground">
            <Header />
            <main>
                {/* Hero Section */}
                <section className="relative overflow-hidden border-b border-border/60">
                    <div className="absolute inset-0" />
                    <div className="relative max-w-7xl mx-auto px-4 sm:px-8 py-16 sm:py-24 md:py-28">
                        <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-8 items-center">
                            <motion.div
                                initial={{ opacity: 0, x: -24 }}
                                animate={{ opacity: 1, x: 0 }}
                                transition={{ duration: 0.55, ease: 'easeOut' }}
                                className="max-w-3xl lg:col-span-7"
                            >
                                <Badge variant="outline" className="mb-6 border-border/80 bg-background/80">
                                    Notification Infra for Production Systems
                                </Badge>
                                <h1 className="text-4xl sm:text-5xl md:text-6xl font-semibold tracking-tight leading-[1.05] mb-6">
                                    Ship Notifications
                                    <br />
                                    Without{' '}
                                    <span className="inline-block min-w-[11ch] sm:min-w-[16ch] text-accent/80 dark:text-accent">
                                        {typedText}
                                        <span className="ml-1 inline-block h-[0.95em] w-[1.5px] align-[-0.08em] bg-accent animate-pulse" />
                                    </span>
                                </h1>
                                <p className="text-base sm:text-lg text-muted-foreground max-w-2xl leading-relaxed mb-10">
                                    FreeRange Notify gives your product one fast API for email, push, SMS, webhook, and real-time browser delivery,
                                    backed by queue-driven reliability and observability.
                                </p>
                                <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3 sm:gap-4">
                                    <Button size="lg" className="px-7" onClick={handleGetStarted}>
                                        {isAuthenticated ? 'Go to Dashboard' : 'Start Building'}
                                    </Button>
                                    <Button
                                        size="lg"
                                        variant="outline"
                                        className="px-7"
                                        onClick={() => {
                                            const el = document.getElementById('pricing');
                                            if (el) {
                                                el.scrollIntoView({ behavior: 'smooth' });
                                            } else {
                                                navigate('/docs');
                                            }
                                        }}
                                    >
                                        View Pricing
                                    </Button>
                                </div>
                            </motion.div>

                            <div className="lg:col-span-5">
                                <HeroIllustration />
                            </div>
                        </div>

                        <motion.div
                            initial={{ opacity: 0, x: -20 }}
                            animate={{ opacity: 1, x: 0 }}
                            transition={{ duration: 0.5, delay: 0.12, ease: 'easeOut' }}
                            className="grid grid-cols-1 sm:grid-cols-4 gap-3 mt-12 sm:mt-14"
                        >
                            <Card className="border-border/70 bg-card/95 py-5">
                                <CardContent className="space-y-1">
                                    <p className="text-2xl font-semibold">5+</p>
                                    <p className="text-sm text-muted-foreground">Delivery channels</p>
                                </CardContent>
                            </Card>
                            <Card className="border-border/70 bg-card/95 py-5">
                                <CardContent className="space-y-1">
                                    <p className="text-2xl font-semibold">Queue-first</p>
                                    <p className="text-sm text-muted-foreground">Worker-based reliability</p>
                                </CardContent>
                            </Card>
                            <Card className="border-border/70 bg-card/95 py-5">
                                <CardContent className="space-y-1">
                                    <p className="text-2xl font-semibold">Real-time</p>
                                    <p className="text-sm text-muted-foreground">Live stream + analytics</p>
                                </CardContent>
                            </Card>
                            <Card className="border-border/70 bg-card/95 py-5">
                                <CardContent className="space-y-1">
                                    <p className="text-2xl font-semibold">
                                        {userCount !== null ? userCount.toLocaleString('en-IN') : '—'}
                                    </p>
                                    <p className="text-sm text-muted-foreground">Organizations + Accounts</p>
                                </CardContent>
                            </Card>
                        </motion.div>
                    </div>
                </section>

                {/* Capabilities Section */}
                <motion.section
                    initial={{ opacity: 0, x: -20 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true, amount: 0.25 }}
                    transition={{ duration: 0.45, ease: 'easeOut' }}
                    className="py-14 sm:py-20"
                >
                    <div className="max-w-7xl mx-auto px-4 sm:px-8">
                        <div className="max-w-2xl mb-10">
                            <Badge variant="outline" className="mb-4 border-border/80 bg-background/80">Core Capabilities</Badge>
                            <h2 className="text-3xl sm:text-4xl tracking-tight mb-4">Built for modern product teams</h2>
                            <p className="text-muted-foreground text-base sm:text-lg">
                                Keep your app logic simple while FreeRange Notify handles routing, retries, provider abstractions, and delivery states.
                            </p>
                        </div>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 sm:gap-6">
                            {capabilities.map((capability) => (
                                <Card key={capability.title} className="h-full border-border/70 bg-card transition-transform duration-200 hover:-translate-y-0.5">
                                    <CardHeader className="pb-3">
                                        <CardTitle className="text-lg">{capability.title}</CardTitle>
                                    </CardHeader>
                                    <CardContent>
                                        <CardDescription className="text-sm leading-relaxed text-muted-foreground">
                                            {capability.description}
                                        </CardDescription>
                                    </CardContent>
                                </Card>
                            ))}
                        </div>
                    </div>
                </motion.section>

                {/* Pricing Section */}
                <motion.section
                    id="pricing"
                    initial={{ opacity: 0, x: 20 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true, amount: 0.3 }}
                    transition={{ duration: 0.5, ease: 'easeOut' }}
                    className="py-14 sm:py-20 bg-gradient-to-b from-background via-card/30 to-background border-y border-border/60"
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

                {/* Showcase Section */}
                <motion.section
                    initial={{ opacity: 0, x: -20 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true, amount: 0.25 }}
                    transition={{ duration: 0.45, ease: 'easeOut' }}
                    className="relative overflow-hidden py-14 sm:py-20 border-y border-border/60"
                >
                    <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_bottom,transparent,rgba(18,18,18,0.03),transparent)]" />
                    <div className="max-w-7xl mx-auto px-4 sm:px-8">
                        <div className="relative z-10 mb-10 sm:mb-12 flex flex-col md:flex-row md:items-end md:justify-between gap-6">
                            <div>
                                <Badge variant="outline" className="mb-4 border-border/80 bg-background/80">Integrated Apps</Badge>
                                <h2 className="text-3xl sm:text-4xl tracking-tight mb-3">Built into real user journeys</h2>
                                <p className="text-muted-foreground text-base sm:text-lg max-w-2xl leading-relaxed">
                                    Production products rely on FreeRange Notify for critical updates, account events, and engagement flows.
                                </p>
                            </div>
                            <Button variant="outline" className="w-fit" onClick={() => navigate('/docs')}>
                                Integration Docs
                            </Button>
                        </div>

                        <div className="relative z-10 grid grid-cols-1 md:grid-cols-2 gap-4 sm:gap-6">
                            {showcaseProducts.map((product, index) => (
                                <motion.div
                                    key={product.name}
                                    initial={{ opacity: 0, x: -16 }}
                                    whileInView={{ opacity: 1, x: 0 }}
                                    viewport={{ once: true, amount: 0.3 }}
                                    transition={{ duration: 0.35, delay: index * 0.08, ease: 'easeOut' }}
                                >
                                    <Card className="group h-full border-border/70 bg-background/90 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-foreground/30 hover:shadow-sm">
                                        <CardHeader>
                                            <Badge variant="outline" className="w-fit text-[11px] text-muted-foreground border-border/70">
                                                {product.type}
                                            </Badge>
                                            <CardTitle className="text-xl">{product.name}</CardTitle>
                                            <CardDescription className="text-sm leading-relaxed text-muted-foreground">
                                                {product.description}
                                            </CardDescription>
                                        </CardHeader>
                                        <CardContent>
                                            <Button asChild variant="outline" className="w-fit gap-1.5">
                                                <a href={product.url} target="_blank" rel="noopener noreferrer">
                                                    Visit Product
                                                    <ArrowUpRight className="h-3.5 w-3.5" />
                                                </a>
                                            </Button>
                                        </CardContent>
                                    </Card>
                                </motion.div>
                            ))}
                        </div>
                    </div>
                </motion.section>
            </main>
            <Footer />
        </div>
    );
};

export default LandingPage;
// Pricing (sourced from billing rate card in codebase, GST assumed at 18%)
const GST_RATE = 0.18;
const pricingPlans = [
    { name: 'Free Trial', tier: 'free_trial', monthlyINR: 0, highlight: false, quotas: 'Email 500 • WhatsApp 50 • SMS 50 • Push 1k' },
    { name: 'Pro', tier: 'pro', monthlyINR: 1499, highlight: true, quotas: 'Email 7.5k • WhatsApp 750 • SMS 750 • Push 15k' },
    { name: 'Growth', tier: 'growth', monthlyINR: 4999, highlight: false, quotas: 'Email 35k • WhatsApp 3k • SMS 3k • Push 60k' },
    { name: 'Scale', tier: 'scale', monthlyINR: 14999, highlight: false, quotas: 'Email 150k • WhatsApp 12k • SMS 12k • Push 250k' },
];

const formatINR = (v: number) => `₹${v.toLocaleString('en-IN', { minimumFractionDigits: 0 })}`;
const formatGST = (v: number) => `incl. GST: ${formatINR(Math.round(v * (1 + GST_RATE)))}`;

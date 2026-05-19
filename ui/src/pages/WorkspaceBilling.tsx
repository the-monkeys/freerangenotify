import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { billingAPI } from '../services/api';
import {
    Loader2, CreditCard, Activity, CalendarDays, CheckCircle2,
    Zap, BarChart3, Info
} from 'lucide-react';
import { useRazorpayCheckout } from '../hooks/useRazorpayCheckout';
import { Button } from '../components/ui/button';
import type { BillingRates, BillingUsage, BillingSubscription } from '../types';

// ─── Types ───
interface BreakdownItem {
    channel: string;
    message_count: number;
    credits_consumed: number;
    overage_amount: number;
}

interface BreakdownResponse {
    billing_enabled: boolean;
    plan?: string;
    period_start?: string;
    period_end?: string;
    credits_total?: number;
    credits_consumed?: number;
    credits_remaining?: number;
    breakdown?: BreakdownItem[];
}

// ─── Helpers ───
const CHANNEL_LABELS: Record<string, string> = {
    email: 'Email',
    whatsapp: 'WhatsApp',
    sms: 'SMS',
    push: 'Push',
    slack: 'Slack',
    discord: 'Discord',
    teams: 'Teams',
    webhook: 'Webhook',
    custom: 'Custom',
    inapp: 'In-App',
    sse: 'Real-time SSE',
};

function formatINR(amount: number): string {
    if (amount === 0) return '₹0.00';
    return `₹${amount.toFixed(2)}`;
}

// ─── Sub-components ───

function BreakdownTable({ breakdown }: { breakdown: BreakdownItem[] }) {
    if (breakdown.length === 0) {
        return (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground text-sm gap-2">
                <BarChart3 className="h-8 w-8 opacity-30" />
                <p>No usage recorded yet in this billing period.</p>
            </div>
        );
    }

    return (
        <div className="overflow-x-auto">
            <table className="w-full text-sm">
                <thead>
                    <tr className="border-b border-border text-muted-foreground">
                        <th className="text-left font-medium py-3 pr-4">Channel</th>
                        <th className="text-right font-medium py-3 pr-4">Messages</th>
                        <th className="text-right font-medium py-3 pr-4">Credits</th>
                        <th className="text-right font-medium py-3">Overage</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-border">
                    {breakdown.map((item) => (
                        <tr key={item.channel} className="hover:bg-muted/30 transition-colors">
                            <td className="py-3 pr-4 font-medium">{CHANNEL_LABELS[item.channel] ?? item.channel}</td>
                            <td className="py-3 pr-4 text-right tabular-nums">{item.message_count.toLocaleString()}</td>
                            <td className="py-3 pr-4 text-right tabular-nums">{item.credits_consumed.toLocaleString()}</td>
                            <td className="py-3 text-right tabular-nums font-medium">{formatINR(item.overage_amount)}</td>
                        </tr>
                    ))}
                </tbody>
                <tfoot>
                    <tr className="border-t-2 border-border font-semibold">
                        <td className="py-3 pr-4 text-muted-foreground">Total</td>
                        <td className="py-3 pr-4 text-right tabular-nums">
                            {breakdown.reduce((s, i) => s + i.message_count, 0).toLocaleString()}
                        </td>
                        <td className="py-3 pr-4 text-right tabular-nums">
                            {breakdown.reduce((s, i) => s + i.credits_consumed, 0).toLocaleString()}
                        </td>
                        <td className="py-3 text-right tabular-nums">
                            {formatINR(breakdown.reduce((s, i) => s + i.overage_amount, 0))}
                        </td>
                    </tr>
                </tfoot>
            </table>
        </div>
    );
}

// ─── Main Page ───
export default function WorkspaceBilling() {
    const [usage, setUsage]             = useState<BillingUsage | null>(null);
    const [subscription, setSubscription] = useState<BillingSubscription | null>(null);
    const [breakdown, setBreakdown]     = useState<BreakdownResponse | null>(null);
    const [rates, setRates]             = useState<BillingRates | null>(null);
    const [loading, setLoading]         = useState(true);

    const refreshData = async () => {
        try {
            const [usageData, subData, breakdownData, ratesData] = await Promise.all([
                billingAPI.getUsage().catch(() => null),
                billingAPI.getSubscription().catch(() => null),
                billingAPI.getUsageBreakdown().catch(() => null),
                billingAPI.getRates().catch(() => null),
            ]);
            setUsage(usageData);
            setSubscription(subData);
            setBreakdown(breakdownData);
            setRates(ratesData);
        } catch (error) {
            console.error('Billing fetch error:', error);
        }
    };

    const { initiateCheckout, isCheckoutLoading } = useRazorpayCheckout(() => {
        refreshData();
    });

    useEffect(() => {
        let mounted = true;
        const fetchData = async () => {
            try {
                const [usageData, subData, breakdownData, ratesData] = await Promise.all([
                    billingAPI.getUsage().catch(() => null),
                    billingAPI.getSubscription().catch(() => null),
                    billingAPI.getUsageBreakdown().catch(() => null),
                    billingAPI.getRates().catch(() => null),
                ]);
                if (mounted) {
                    setUsage(usageData);
                    setSubscription(subData);
                    setBreakdown(breakdownData);
                    setRates(ratesData);
                }
            } catch (error) {
                console.error('Billing fetch error:', error);
            } finally {
                if (mounted) setLoading(false);
            }
        };
        fetchData();
        return () => { mounted = false; };
    }, []);

    const daysRemaining = usage?.days_remaining !== undefined 
        ? usage.days_remaining
        : subscription?.current_period_end
            ? Math.max(0, Math.ceil(
                (new Date(subscription.current_period_end).getTime() - Date.now()) / 86_400_000
              ))
            : null;

    if (loading) {
        return (
            <div className="flex h-[50vh] items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        );
    }

    const billingEnabled = breakdown?.billing_enabled === true;
    const breakdownItems = breakdown?.breakdown ?? [];

    const overageTotal = breakdownItems.reduce((s, i) => s + i.overage_amount, 0);

    return (
        <div className="mx-auto max-w-6xl space-y-6">
            {/* ── Header ── */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold tracking-tight">Billing &amp; Licensing</h1>
                    <p className="text-muted-foreground">
                        Manage your workspace subscription, credits, and channel-level usage.{' '}
                        <Link to="/docs/pricing" className="text-accent hover:underline">
                            Understand credit burn and overage
                        </Link>
                    </p>
                </div>
                {subscription?.plan ? (
                    <Badge variant="secondary" className="px-3 py-1 text-sm font-medium capitalize">
                        {subscription.plan.replace('_', ' ')} Plan
                    </Badge>
                ) : (
                    <Button 
                        onClick={() => initiateCheckout('pro')} 
                        disabled={isCheckoutLoading}
                    >
                        {isCheckoutLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Upgrade to PRO
                    </Button>
                )}
            </div>

            {/* ── Summary Cards ── */}
            <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
                {/* License card */}
                <Card className="bg-card/60 shadow-sm border-border">
                    <CardHeader className="pb-2">
                        <CardDescription className="flex items-center font-medium">
                            <CreditCard className="mr-2 h-4 w-4" /> Current License
                        </CardDescription>
                        <CardTitle className="text-3xl font-bold mt-2">
                            {subscription?.plan ? subscription.plan.toUpperCase() : 'Free Tier'}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        {daysRemaining !== null ? (
                            <div className="text-sm space-y-2 mt-4 text-muted-foreground">
                                <p className="flex justify-between items-center">
                                    <span>Status:</span>
                                    <Badge variant="outline" className={
                                        subscription?.status === 'trial'
                                            ? 'bg-amber-500/10 text-amber-600 border-amber-500/20'
                                            : 'bg-green-500/10 text-green-600 border-green-500/20'
                                    }>
                                        {subscription?.status === 'trial' ? 'Trial' : 'Active'}
                                    </Badge>
                                </p>
                                <p className="flex justify-between items-center">
                                    <span>Time Remaining:</span>
                                    <span className={`font-medium ${daysRemaining < 7 ? 'text-red-500' : 'text-primary'}`}>
                                        {daysRemaining} day{daysRemaining !== 1 ? 's' : ''}
                                    </span>
                                </p>
                            </div>
                        ) : (
                            <div className="mt-4 space-y-4">
                                <p className="text-sm text-muted-foreground">
                                    No active premium subscription. Upgrade your workspace to unlock team billing and dedicated quotas.
                                </p>
                                <Button 
                                    className="w-full" 
                                    onClick={() => initiateCheckout('pro')}
                                    disabled={isCheckoutLoading}
                                >
                                    {isCheckoutLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                    Upgrade to Pro
                                </Button>
                            </div>
                        )}
                    </CardContent>
                </Card>

                {/* Monthly usage card */}
                <Card className="bg-card/60 shadow-sm border-border">
                    <CardHeader className="pb-2">
                        <CardDescription className="flex items-center font-medium">
                            <Activity className="mr-2 h-4 w-4" /> Credit Usage
                        </CardDescription>
                        <CardTitle className="text-3xl font-bold mt-2">
                            {(usage?.credits_consumed ?? 0).toLocaleString()}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="text-sm space-y-2 mt-4 text-muted-foreground">
                            <p className="flex justify-between items-center">
                                <span>Total credits:</span>
                                <span>{usage?.credits_total ? usage.credits_total.toLocaleString() : '-'}</span>
                            </p>
                            {usage?.usage_percent !== undefined && (
                                <p className="flex justify-between items-center">
                                    <span>Consumed:</span>
                                    <span>{usage.usage_percent.toFixed(1)}%</span>
                                </p>
                            )}
                            <p className="flex justify-between items-center">
                                <span>Credits remaining:</span>
                                <span>{usage?.credits_remaining?.toLocaleString() ?? '-'}</span>
                            </p>
                            {(subscription?.credits_reserved ?? 0) > 0 && (
                                <p className="flex justify-between items-center">
                                    <span>Reserved (in flight):</span>
                                    <span>{subscription!.credits_reserved!.toLocaleString()}</span>
                                </p>
                            )}
                            <p className="flex justify-between items-center">
                                <span>Messages sent:</span>
                                <span>{(usage?.messages_sent ?? 0).toLocaleString()}</span>
                            </p>
                            {daysRemaining !== null && (
                                <p className="flex justify-between items-center">
                                    <span>Cycle resets in:</span>
                                    <span>{daysRemaining} day{daysRemaining !== 1 ? 's' : ''}</span>
                                </p>
                            )}
                        </div>
                    </CardContent>
                </Card>

                {/* Renewal card */}
                <Card className="bg-card/60 shadow-sm border-border relative overflow-hidden">
                    <div className="absolute top-0 right-0 p-4 opacity-5">
                        <CheckCircle2 className="h-24 w-24" />
                    </div>
                    <CardHeader className="pb-2">
                        <CardDescription className="flex items-center font-medium">
                            <CalendarDays className="mr-2 h-4 w-4" /> Renewal Date
                        </CardDescription>
                        <CardTitle className="text-xl font-bold mt-2 truncate">
                            {subscription?.current_period_end
                                ? new Date(subscription.current_period_end).toLocaleDateString()
                                : 'N/A'}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <p className="text-sm mt-4 text-muted-foreground mb-4">
                            Looking to scale? Upgrade your workspace to unlock team billing and dedicated quotas.
                        </p>
                        {subscription?.plan !== 'pro' && (
                            <Button variant="outline" className="w-full" onClick={() => initiateCheckout('pro')} disabled={isCheckoutLoading}>
                                {isCheckoutLoading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Zap className="h-4 w-4 mr-2" />}
                                Upgrade to Pro
                            </Button>
                        )}
                    </CardContent>
                </Card>
            </div>

            {/* ── Payment & Transactions ── */}
            <div className="grid gap-6 md:grid-cols-2">
                <Card className="bg-card/60 shadow-sm border-border">
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <CreditCard className="h-5 w-5" />
                            Payment Method
                        </CardTitle>
                        <CardDescription>
                            How you're paying for your FreeRangeNotify subscription.
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        {subscription?.plan && subscription.plan !== 'free' ? (
                            <div className="flex items-center gap-4 p-4 rounded-lg border border-border bg-muted/20">
                                <div className="p-2 bg-primary/10 rounded-full">
                                    <CreditCard className="h-6 w-6 text-primary" />
                                </div>
                                <div>
                                    <p className="font-semibold capitalize">{subscription.plan} Subscription</p>
                                    <p className="text-sm text-muted-foreground">Managed via Razorpay Secure</p>
                                </div>
                                <Badge className="ml-auto" variant="outline">Verified</Badge>
                            </div>
                        ) : (
                            <div className="text-center py-6 border-2 border-dashed border-border rounded-lg bg-muted/5">
                                <p className="text-sm text-muted-foreground">
                                    {subscription?.plan === 'free' ? 'Active free onboarding period — no payment method required.' : 'No payment method attached.'}
                                </p>
                                <Button 
                                    variant="link" 
                                    className="mt-2 text-primary"
                                    onClick={() => initiateCheckout('pro')}
                                >
                                    {subscription?.plan === 'free' ? 'Add payment method for Pro' : 'Add payment method'}
                                </Button>
                            </div>
                        )}
                    </CardContent>
                </Card>

                <Card className="bg-card/60 shadow-sm border-border">
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Activity className="h-5 w-5" />
                            Transaction History
                        </CardTitle>
                        <CardDescription>
                            Recent billing statements and renewals.
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        {subscription?.plan && subscription.plan !== 'free' ? (
                            <div className="space-y-3">
                                <div className="flex justify-between items-center text-sm p-2 rounded hover:bg-muted/30 transition-colors">
                                    <div>
                                        <p className="font-medium">Subscription Renewal</p>
                                        <p className="text-xs text-muted-foreground">
                                            {subscription.current_period_start ? new Date(subscription.current_period_start).toLocaleDateString() : 'Recent'}
                                        </p>
                                    </div>
                                    <div className="text-right">
                                        <p className="font-semibold text-emerald-600">Success</p>
                                        <p className="text-xs text-muted-foreground">INR Card</p>
                                    </div>
                                </div>
                                <p className="text-[10px] text-center text-muted-foreground pt-2">
                                    Only recent transactions are shown here. For a full PDF invoice, please contact support.
                                </p>
                            </div>
                        ) : (
                            <div className="text-center py-8 text-muted-foreground text-sm">
                                <Activity className="h-8 w-8 mx-auto opacity-20 mb-2" />
                                <p>No transactions found.</p>
                                {subscription?.plan === 'free' && (
                                    <p className="mt-1 text-xs opacity-60">You are currently on a free trial.</p>
                                )}
                            </div>
                        )}
                    </CardContent>
                </Card>
            </div>

            {/* ── Hybrid Billing Breakdown ── */}
            <Card className="bg-card/60 shadow-sm border-border">
                <CardHeader>
                    <div className="flex items-start justify-between gap-4">
                        <div>
                            <CardTitle className="flex items-center gap-2">
                                <BarChart3 className="h-5 w-5" />
                                Channel Usage Breakdown
                            </CardTitle>
                            <CardDescription className="mt-1">
                                Per-channel message volume, credit burn, and overage from billing APIs.
                            </CardDescription>
                        </div>
                        {billingEnabled && (
                            <div className="flex gap-3 shrink-0">
                                {overageTotal > 0 && (
                                    <div className="text-right">
                                        <p className="text-xs text-muted-foreground">Total overage</p>
                                        <p className="font-semibold text-emerald-600">{formatINR(overageTotal)}</p>
                                    </div>
                                )}
                                {rates?.active_version && (
                                    <div className="text-right">
                                        <p className="text-xs text-muted-foreground">Rate card</p>
                                        <p className="font-semibold">{rates.active_version}</p>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </CardHeader>
                <CardContent>
                    {!billingEnabled ? (
                        <div className="flex items-start gap-3 rounded-lg border border-amber-500/20 bg-amber-500/5 p-4 text-sm text-amber-700">
                            <Info className="h-4 w-4 mt-0.5 shrink-0" />
                            <div>
                                <p className="font-medium">Usage metering is not active</p>
                                <p className="text-amber-600 mt-0.5">
                                    Set <code className="font-mono bg-amber-500/10 px-1 rounded">FREERANGE_FEATURES_BILLING_ENABLED=true</code> to enable per-channel cost tracking.
                                </p>
                            </div>
                        </div>
                    ) : (
                        <>
                            {breakdown?.period_start && (
                                <p className="text-xs text-muted-foreground mb-4">
                                    Period: {new Date(breakdown.period_start).toLocaleDateString()} –{' '}
                                    {new Date(breakdown.period_end!).toLocaleDateString()}
                                </p>
                            )}
                            <BreakdownTable breakdown={breakdownItems} />

                            <div className="mt-6 flex flex-wrap gap-3 border-t border-border pt-4">
                                <span className="text-xs text-muted-foreground">
                                    Active version: {rates?.active_version ?? 'default'}
                                </span>
                                {rates?.effective_at && (
                                    <span className="text-xs text-muted-foreground">
                                        Effective: {new Date(rates.effective_at).toLocaleString()}
                                    </span>
                                )}
                                <span className="text-xs text-muted-foreground">
                                    Burn rates: {Object.entries(rates?.channel_credit_cost ?? {}).map(([ch, c]) => `${ch}=${c}`).join(', ')}
                                </span>
                                <span className="text-xs text-muted-foreground">
                                    Free caps: WhatsApp {rates?.free_tier_daily_caps?.whatsapp ?? 2}/day, SMS {rates?.free_tier_daily_caps?.sms ?? 3}/day
                                </span>
                            </div>
                        </>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}

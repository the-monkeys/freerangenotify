import { useEffect, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { billingAPI } from '../services/api';
import {
    Loader2, CreditCard, Activity, CalendarDays, CheckCircle2,
    Zap, Server, BarChart3, Info
} from 'lucide-react';

// ─── Types ───
interface BreakdownItem {
    channel: string;
    credential_source: 'system' | 'byoc' | 'platform';
    message_count: number;
    total_billed_inr: number;
    period_start: string;
    period_end: string;
}

interface QuotaItem {
    channel: string;
    included: number;
    used: number;
    remaining: number;
}

interface BreakdownResponse {
    billing_enabled: boolean;
    plan?: string;
    period_start?: string;
    period_end?: string;
    breakdown?: BreakdownItem[];
    quotas?: QuotaItem[];
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

const CRED_SOURCE_META: Record<string, { label: string; color: string; icon: typeof Server }> = {
    system: { label: 'System Creds', color: 'bg-blue-500/10 text-blue-600 border-blue-500/20', icon: Server },
    byoc:   { label: 'Your Creds (BYOC)', color: 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20', icon: Zap },
    platform: { label: 'Platform (Free)', color: 'bg-slate-500/10 text-slate-500 border-slate-500/20', icon: CheckCircle2 },
};

function formatINR(amount: number): string {
    if (amount === 0) return '₹0.00';
    return `₹${amount.toFixed(2)}`;
}

// ─── Sub-components ───

function BreakdownTable({ breakdown }: { breakdown: BreakdownItem[] }) {
    // Group by channel
    const grouped = breakdown.reduce<Record<string, BreakdownItem[]>>((acc, item) => {
        if (!acc[item.channel]) acc[item.channel] = [];
        acc[item.channel].push(item);
        return acc;
    }, {});

    if (Object.keys(grouped).length === 0) {
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
                        <th className="text-left font-medium py-3 pr-4">Credential Mode</th>
                        <th className="text-right font-medium py-3 pr-4">Messages</th>
                        <th className="text-right font-medium py-3">Billed</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-border">
                    {Object.entries(grouped).map(([channel, items]) =>
                        items.map((item, idx) => {
                            const meta = CRED_SOURCE_META[item.credential_source] ?? CRED_SOURCE_META.platform;
                            const Icon = meta.icon;
                            return (
                                <tr key={`${channel}-${item.credential_source}-${idx}`}
                                    className="hover:bg-muted/30 transition-colors">
                                    <td className="py-3 pr-4 font-medium">
                                        {idx === 0 ? (CHANNEL_LABELS[channel] ?? channel) : ''}
                                    </td>
                                    <td className="py-3 pr-4">
                                        <Badge variant="outline" className={`text-xs ${meta.color}`}>
                                            <Icon className="h-3 w-3 mr-1" />
                                            {meta.label}
                                        </Badge>
                                    </td>
                                    <td className="py-3 pr-4 text-right tabular-nums">
                                        {item.message_count.toLocaleString()}
                                    </td>
                                    <td className="py-3 text-right tabular-nums font-medium">
                                        {item.credential_source === 'platform'
                                            ? <span className="text-muted-foreground text-xs">Free</span>
                                            : formatINR(item.total_billed_inr)
                                        }
                                    </td>
                                </tr>
                            );
                        })
                    )}
                </tbody>
                <tfoot>
                    <tr className="border-t-2 border-border font-semibold">
                        <td colSpan={2} className="py-3 pr-4 text-muted-foreground">Total</td>
                        <td className="py-3 pr-4 text-right tabular-nums">
                            {breakdown.reduce((s, i) => s + i.message_count, 0).toLocaleString()}
                        </td>
                        <td className="py-3 text-right tabular-nums">
                            {formatINR(breakdown.reduce((s, i) => s + i.total_billed_inr, 0))}
                        </td>
                    </tr>
                </tfoot>
            </table>
        </div>
    );
}

function QuotaUsageCard({ quotas }: { quotas: QuotaItem[] }) {
    const CHANNEL_ORDER = ['email', 'whatsapp', 'sms', 'push'];
    const sorted = [...quotas].sort((a, b) => {
        const ai = CHANNEL_ORDER.indexOf(a.channel);
        const bi = CHANNEL_ORDER.indexOf(b.channel);
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
    });

    return (
        <div className="grid gap-4 sm:grid-cols-2">
            {sorted.map((q) => {
                const pct = q.included > 0 ? Math.min((q.used / q.included) * 100, 100) : 0;
                const isOver = q.used > q.included;
                const barColor = isOver
                    ? 'bg-red-500'
                    : pct > 80
                      ? 'bg-amber-500'
                      : 'bg-emerald-500';

                return (
                    <div
                        key={q.channel}
                        className="rounded-lg border border-border bg-card/40 p-4 space-y-2"
                    >
                        <div className="flex items-center justify-between">
                            <span className="text-sm font-medium">
                                {CHANNEL_LABELS[q.channel] ?? q.channel}
                            </span>
                            <span className="text-xs text-muted-foreground">
                                {q.used.toLocaleString()} / {q.included.toLocaleString()}
                            </span>
                        </div>
                        {/* Progress bar */}
                        <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
                            <div
                                className={`h-full rounded-full transition-all duration-500 ${barColor}`}
                                style={{ width: `${Math.min(pct, 100)}%` }}
                            />
                        </div>
                        <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <span>
                                {isOver
                                    ? <span className="text-red-500 font-medium">{(q.used - q.included).toLocaleString()} over limit</span>
                                    : `${q.remaining.toLocaleString()} remaining`
                                }
                            </span>
                            <span>{pct.toFixed(0)}% used</span>
                        </div>
                    </div>
                );
            })}
        </div>
    );
}

// ─── Main Page ───
export default function WorkspaceBilling() {
    const [usage, setUsage]             = useState<any>(null);
    const [subscription, setSubscription] = useState<any>(null);
    const [breakdown, setBreakdown]     = useState<BreakdownResponse | null>(null);
    const [loading, setLoading]         = useState(true);

    useEffect(() => {
        let mounted = true;
        const fetchData = async () => {
            try {
                const [usageData, subData, breakdownData] = await Promise.all([
                    billingAPI.getUsage().catch(() => null),
                    billingAPI.getSubscription().catch(() => null),
                    billingAPI.getUsageBreakdown().catch(() => null),
                ]);
                if (mounted) {
                    setUsage(usageData);
                    setSubscription(subData);
                    setBreakdown(breakdownData);
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

    const daysRemaining = subscription?.current_period_end
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

    // Compute totals split by credential source for summary badges
    const systemTotal = breakdownItems.filter(i => i.credential_source === 'system')
        .reduce((s, i) => s + i.total_billed_inr, 0);
    const byocTotal = breakdownItems.filter(i => i.credential_source === 'byoc')
        .reduce((s, i) => s + i.total_billed_inr, 0);

    return (
        <div className="mx-auto max-w-6xl space-y-6">
            {/* ── Header ── */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold tracking-tight">Billing &amp; Licensing</h1>
                    <p className="text-muted-foreground">
                        Manage your workspace subscription, licensing, and hybrid channel usage.
                    </p>
                </div>
                {subscription?.plan && (
                    <Badge variant="secondary" className="px-3 py-1 text-sm font-medium capitalize">
                        {subscription.plan} Plan
                    </Badge>
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
                            <p className="text-sm mt-4 text-muted-foreground">
                                No active premium subscription. Create an{' '}
                                <a href="/tenants" className="text-primary hover:underline">Organization</a>{' '}
                                to unlock team billing.
                            </p>
                        )}
                    </CardContent>
                </Card>

                {/* Monthly usage card */}
                <Card className="bg-card/60 shadow-sm border-border">
                    <CardHeader className="pb-2">
                        <CardDescription className="flex items-center font-medium">
                            <Activity className="mr-2 h-4 w-4" /> Monthly Usage
                        </CardDescription>
                        <CardTitle className="text-3xl font-bold mt-2">
                            {(usage?.messages_sent ?? 0).toLocaleString()}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="text-sm space-y-2 mt-4 text-muted-foreground">
                            <p className="flex justify-between items-center">
                                <span>Limit:</span>
                                <span>{usage?.message_limit ? usage.message_limit.toLocaleString() : 'Unlimited'}</span>
                            </p>
                            {usage?.usage_percent !== undefined && (
                                <p className="flex justify-between items-center">
                                    <span>Consumed:</span>
                                    <span>{usage.usage_percent.toFixed(1)}%</span>
                                </p>
                            )}
                            {usage?.days_remaining !== undefined && (
                                <p className="flex justify-between items-center">
                                    <span>Cycle resets in:</span>
                                    <span>{usage.days_remaining} day{usage.days_remaining !== 1 ? 's' : ''}</span>
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
                        <p className="text-sm mt-4 text-muted-foreground">
                            Looking to scale? Create an{' '}
                            <a href="/tenants" className="text-primary hover:underline">Organization</a>{' '}
                            to unlock team billing and dedicated quotas.
                        </p>
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
                                Per-channel billing split between system credentials (we pay the carrier) and your own credentials (BYOC).
                            </CardDescription>
                        </div>
                        {billingEnabled && breakdownItems.length > 0 && (
                            <div className="flex gap-3 shrink-0">
                                {systemTotal > 0 && (
                                    <div className="text-right">
                                        <p className="text-xs text-muted-foreground">System creds</p>
                                        <p className="font-semibold text-blue-600">{formatINR(systemTotal)}</p>
                                    </div>
                                )}
                                {byocTotal > 0 && (
                                    <div className="text-right">
                                        <p className="text-xs text-muted-foreground">BYOC fees</p>
                                        <p className="font-semibold text-emerald-600">{formatINR(byocTotal)}</p>
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

                            {/* ── Quota usage meters ── */}
                            {(breakdown?.quotas?.length ?? 0) > 0 && (
                                <div className="mt-6 pt-6 border-t border-border">
                                    <h3 className="text-sm font-semibold mb-3 flex items-center gap-2">
                                        <Activity className="h-4 w-4" />
                                        Free Quota Usage
                                        {breakdown?.plan && (
                                            <Badge variant="outline" className="capitalize text-xs ml-1">
                                                {breakdown.plan.replace('_', ' ')}
                                            </Badge>
                                        )}
                                    </h3>
                                    <QuotaUsageCard quotas={breakdown!.quotas!} />
                                </div>
                            )}

                            {/* Legend */}
                            <div className="mt-6 flex flex-wrap gap-3 border-t border-border pt-4">
                                {Object.entries(CRED_SOURCE_META).map(([key, meta]) => {
                                    const Icon = meta.icon;
                                    return (
                                        <Badge key={key} variant="outline" className={`text-xs ${meta.color}`}>
                                            <Icon className="h-3 w-3 mr-1" />
                                            {meta.label}
                                        </Badge>
                                    );
                                })}
                                <span className="text-xs text-muted-foreground self-center ml-auto">
                                    All amounts in INR
                                </span>
                            </div>
                        </>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}

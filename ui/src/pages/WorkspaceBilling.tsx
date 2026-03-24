import { useEffect, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { billingAPI } from '../services/api';
import { Loader2, CreditCard, Activity, CalendarDays, CheckCircle2 } from 'lucide-react';

export default function WorkspaceBilling() {
    const [usage, setUsage] = useState<any>(null);
    const [subscription, setSubscription] = useState<any>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        let mounted = true;
        const fetchData = async () => {
            try {
                const [usageData, subData] = await Promise.all([
                    billingAPI.getUsage().catch(() => null),
                    billingAPI.getSubscription().catch(() => null),
                ]);
                if (mounted) {
                    setUsage(usageData);
                    setSubscription(subData);
                }
            } catch (error) {
                console.error("Billing fetch error:", error);
            } finally {
                if(mounted) setLoading(false);
            }
        };

        fetchData();
        return () => { mounted = false; };
    }, []);

    // Backend returns current_period_end (RFC3339), plan, status
    const daysRemaining = subscription?.current_period_end
        ? Math.max(0, Math.ceil((new Date(subscription.current_period_end).getTime() - new Date().getTime()) / (1000 * 60 * 60 * 24)))
        : null;

    if (loading) {
        return (
            <div className="flex h-[50vh] items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        );
    }

    return (
        <div className="mx-auto max-w-6xl space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold tracking-tight">Billing & Licensing</h1>
                    <p className="text-muted-foreground">Manage your personal workspace subscription, licensing, and usage metrics.</p>
                </div>
                {subscription?.plan && (
                    <Badge variant="secondary" className="px-3 py-1 text-sm font-medium capitalize">
                        {subscription.plan} Plan
                    </Badge>
                )}
            </div>

            <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
                <Card className="bg-card/60 shadow-sm border-border">
                    <CardHeader className="pb-2">
                        <CardDescription className="flex items-center font-medium">
                            <CreditCard className="mr-2 h-4 w-4" />
                            Current License
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
                                    <Badge variant="outline" className={`${subscription?.status === 'trial' ? 'bg-amber-500/10 text-amber-600 border-amber-500/20' : 'bg-green-500/10 text-green-600 border-green-500/20'}`}>
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
                            <p className="text-sm mt-4 text-muted-foreground">No active premium subscription assigned to personal workspace.</p>
                        )}
                    </CardContent>
                </Card>

                <Card className="bg-card/60 shadow-sm border-border">
                    <CardHeader className="pb-2">
                        <CardDescription className="flex items-center font-medium">
                            <Activity className="mr-2 h-4 w-4" />
                            Monthly Usage
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

                <Card className="bg-card/60 shadow-sm border-border relative overflow-hidden">
                    <div className="absolute top-0 right-0 p-4 opacity-5">
                        <CheckCircle2 className="h-24 w-24" />
                    </div>
                    <CardHeader className="pb-2">
                        <CardDescription className="flex items-center font-medium">
                            <CalendarDays className="mr-2 h-4 w-4" />
                            Renewal Date
                        </CardDescription>
                        <CardTitle className="text-xl font-bold mt-2 truncate">
                            {subscription?.current_period_end ? new Date(subscription.current_period_end).toLocaleDateString() : 'N/A'}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <p className="text-sm mt-4 text-muted-foreground">
                            Looking to scale beyond personal limits? Create an <a href="/tenants" className="text-primary hover:underline">Organization</a> to unlock team billing and dedicated quotas.
                        </p>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}

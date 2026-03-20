import { useEffect, useState } from 'react';
import { LayoutGrid, Users, FileText, Workflow, Send, TrendingUp, CheckCircle } from 'lucide-react';
import { Card, CardContent } from '../ui/card';
import { adminAPI, applicationsAPI } from '../../services/api';
import type { SystemStats } from '../../types';

interface StatCardProps {
    icon: React.ReactNode;
    label: string;
    value: number | string;
    colorClass?: string;
}

const PANEL_CARD_CLASSES = 'border-border/70 bg-white/80 shadow-sm backdrop-blur-sm dark:bg-zinc-900/60';

function StatCard({ icon, label, value, colorClass = 'text-foreground' }: StatCardProps) {
    return (
        <Card className={PANEL_CARD_CLASSES}>
            <CardContent className="pt-5 pb-4">
                <div className="flex items-start gap-3">
                    <div className="p-2 rounded-md bg-muted">{icon}</div>
                    <div className="min-w-0 flex-1">
                        <p className="text-xs text-muted-foreground uppercase tracking-wider mb-1">{label}</p>
                        <p className={`text-2xl font-bold ${colorClass}`}>{value}</p>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

export default function OverviewStats() {
    const [stats, setStats] = useState<SystemStats | null>(null);
    const [loading, setLoading] = useState(true);

    const fetchStats = async () => {
        try {
            const base = await adminAPI.getSystemStats();

            // Patch total_apps from applicationsAPI
            try {
                const apps = await applicationsAPI.list();
                base.total_apps = Array.isArray(apps) ? apps.length : 0;
            } catch {
                // leave as 0
            }

            setStats(base);
        } catch (error) {
            console.error('Failed to fetch system stats:', error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchStats();
        const interval = setInterval(fetchStats, 30_000);
        return () => clearInterval(interval);
    }, []);

    if (loading && !stats) {
        return (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
                {Array.from({ length: 4 }).map((_, i) => (
                    <Card key={i} className={PANEL_CARD_CLASSES}>
                        <CardContent className="pt-5 pb-4">
                            <div className="h-16 animate-pulse bg-muted rounded" />
                        </CardContent>
                    </Card>
                ))}
            </div>
        );
    }

    if (!stats) return null;

    const rateColor =
        stats.success_rate >= 95
            ? 'text-green-600'
            : stats.success_rate >= 80
                ? 'text-yellow-600'
                : 'text-red-600';

    return (
        <div className="space-y-4 mb-8">
            <h3 className="text-lg font-semibold">System Overview</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <StatCard
                    icon={<LayoutGrid className="h-4 w-4 text-muted-foreground" />}
                    label="Applications"
                    value={stats.total_apps}
                />
                <StatCard
                    icon={<Users className="h-4 w-4 text-muted-foreground" />}
                    label="Users"
                    value={stats.total_users || '—'}
                />
                <StatCard
                    icon={<FileText className="h-4 w-4 text-muted-foreground" />}
                    label="Templates"
                    value={stats.total_templates || '—'}
                />
                <StatCard
                    icon={<Workflow className="h-4 w-4 text-muted-foreground" />}
                    label="Workflows"
                    value={stats.total_workflows || '—'}
                />
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                <StatCard
                    icon={<Send className="h-4 w-4 text-blue-500" />}
                    label="Sent Today"
                    value={stats.notifications_today}
                    colorClass="text-blue-600"
                />
                <StatCard
                    icon={<TrendingUp className="h-4 w-4 text-indigo-500" />}
                    label="Sent This Week"
                    value={stats.notifications_this_week}
                    colorClass="text-indigo-600"
                />
                <StatCard
                    icon={<CheckCircle className="h-4 w-4 text-green-500" />}
                    label="Success Rate"
                    value={`${stats.success_rate.toFixed(1)}%`}
                    colorClass={rateColor}
                />
            </div>
        </div>
    );
}

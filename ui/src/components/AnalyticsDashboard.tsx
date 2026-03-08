import { useEffect, useState } from 'react';
import { adminAPI } from '../services/api';
import type { AnalyticsSummary } from '../types';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Badge } from './ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import ChannelBreakdownChart from './dashboard/ChannelBreakdownChart';

const CHANNEL_COLORS: Record<string, string> = {
    email: 'bg-blue-100 text-blue-700',
    push: 'bg-purple-100 text-purple-700',
    sms: 'bg-green-100 text-green-700',
    webhook: 'bg-orange-100 text-orange-700',
    in_app: 'bg-indigo-100 text-indigo-700',
    sse: 'bg-teal-100 text-teal-700',
};

export default function AnalyticsDashboard() {
    const [summary, setSummary] = useState<AnalyticsSummary | null>(null);
    const [period, setPeriod] = useState('7d');
    const [loading, setLoading] = useState(true);

    const fetchAnalytics = async () => {
        setLoading(true);
        try {
            const data = await adminAPI.getAnalyticsSummary(period);
            setSummary(data);
        } catch (error) {
            console.error('Failed to fetch analytics:', error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchAnalytics();
    }, [period]);

    if (loading && !summary) {
        return <Card><CardContent className="py-8 text-center text-gray-500">Loading analytics...</CardContent></Card>;
    }

    if (!summary) {
        return <Card><CardContent className="py-8 text-center text-gray-500">No analytics data available</CardContent></Card>;
    }

    const maxDaily = Math.max(...(summary.daily_breakdown?.map(d => d.count) || [1]), 1);

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle>Notification Analytics</CardTitle>
                    <Select value={period} onValueChange={setPeriod}>
                        <SelectTrigger className="w-[120px]" aria-label="Select time period">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="1d">Last 24h</SelectItem>
                            <SelectItem value="7d">Last 7 days</SelectItem>
                            <SelectItem value="30d">Last 30 days</SelectItem>
                            <SelectItem value="90d">Last 90 days</SelectItem>
                        </SelectContent>
                    </Select>
                </div>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Stat Cards */}
                <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                    <StatCard label="Total Sent" value={summary.total_sent + summary.total_delivered + summary.total_read} color="blue" />
                    <StatCard label="Delivered" value={summary.total_delivered} color="green" />
                    <StatCard label="Failed" value={summary.total_failed} color="red" />
                    <StatCard
                        label="Success Rate"
                        value={`${summary.success_rate.toFixed(1)}%`}
                        color={summary.success_rate >= 90 ? 'green' : summary.success_rate >= 70 ? 'yellow' : 'red'}
                    />
                    <StatCard
                        label="Delivery Rate"
                        value={
                            summary.total_sent > 0
                                ? `${((summary.total_delivered / (summary.total_sent + summary.total_delivered + summary.total_read)) * 100).toFixed(1)}%`
                                : '—'
                        }
                        color={summary.total_sent > 0 && (summary.total_delivered / (summary.total_sent + summary.total_delivered + summary.total_read)) >= 0.9 ? 'green' : 'yellow'}
                    />
                    <StatCard
                        label="Avg Latency"
                        value={summary.avg_latency_ms != null ? `${summary.avg_latency_ms}ms` : '—'}
                        color="blue"
                    />
                </div>

                {/* Daily Activity Chart (simple bar chart using divs) */}
                {summary.daily_breakdown && summary.daily_breakdown.length > 0 && (
                    <div>
                        <h4 className="text-sm font-semibold mb-3">Daily Activity</h4>
                        <div className="flex items-end gap-[2px] h-[100px]">
                            {summary.daily_breakdown.map((d, i) => (
                                <div
                                    key={i}
                                    className="flex-1 group relative"
                                    title={`${d.date}: ${d.count} notifications`}
                                >
                                    <div
                                        className="bg-blue-500 hover:bg-blue-600 rounded-t transition-all cursor-default"
                                        style={{
                                            height: `${Math.max((d.count / maxDaily) * 100, 2)}%`,
                                            minHeight: d.count > 0 ? 4 : 1,
                                        }}
                                    />
                                    {/* Tooltip */}
                                    <div className="pointer-events-none absolute bottom-full left-1/2 -translate-x-1/2 mb-1 bg-gray-900 text-white text-[10px] px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap z-10">
                                        {d.date}: {d.count}
                                    </div>
                                </div>
                            ))}
                        </div>
                        <div className="flex justify-between text-[10px] text-gray-400 mt-1">
                            <span>{summary.daily_breakdown[0]?.date}</span>
                            <span>{summary.daily_breakdown[summary.daily_breakdown.length - 1]?.date}</span>
                        </div>
                    </div>
                )}

                {/* Channel Breakdown Chart */}
                {summary.by_channel && summary.by_channel.length > 0 && (
                    <div className="space-y-4">
                        <ChannelBreakdownChart
                            channels={summary.by_channel}
                            totalSent={summary.total_sent + summary.total_delivered + summary.total_read}
                        />
                    </div>
                )}

                {/* Channel Breakdown Detail Table (collapsible) */}
                {summary.by_channel && summary.by_channel.length > 0 && (
                    <details className="group">
                        <summary className="text-sm font-semibold mb-3 cursor-pointer select-none flex items-center gap-1">
                            <span className="transition-transform group-open:rotate-90">▶</span>
                            Channel Detail Table
                        </summary>
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Channel</TableHead>
                                    <TableHead className="text-right">Sent</TableHead>
                                    <TableHead className="text-right">Delivered</TableHead>
                                    <TableHead className="text-right">Failed</TableHead>
                                    <TableHead className="text-right">Total</TableHead>
                                    <TableHead className="text-right">Success Rate</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {summary.by_channel.map(ch => (
                                    <TableRow key={ch.channel}>
                                        <TableCell>
                                            <Badge className={`text-xs uppercase ${CHANNEL_COLORS[ch.channel] || 'bg-gray-100 text-gray-700'}`}>
                                                {ch.channel}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-right font-mono">{ch.sent}</TableCell>
                                        <TableCell className="text-right font-mono">{ch.delivered}</TableCell>
                                        <TableCell className="text-right font-mono text-red-600">{ch.failed}</TableCell>
                                        <TableCell className="text-right font-mono font-semibold">{ch.total}</TableCell>
                                        <TableCell className="text-right">
                                            <Badge variant="outline" className={`text-xs ${ch.success_rate >= 90 ? 'text-green-700' : ch.success_rate >= 70 ? 'text-yellow-700' : 'text-red-700'}`}>
                                                {ch.success_rate.toFixed(1)}%
                                            </Badge>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </details>
                )}

                {/* Additional Stats */}
                <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
                    <div className="bg-gray-50 rounded p-3">
                        <div className="text-xs text-gray-500 mb-1">Pending</div>
                        <div className="font-semibold text-lg text-yellow-600">{summary.total_pending}</div>
                    </div>
                    <div className="bg-gray-50 rounded p-3">
                        <div className="text-xs text-gray-500 mb-1">Read</div>
                        <div className="font-semibold text-lg text-indigo-600">{summary.total_read}</div>
                    </div>
                    <div className="bg-gray-50 rounded p-3">
                        <div className="text-xs text-gray-500 mb-1">Total All Time</div>
                        <div className="font-semibold text-lg text-gray-900">{summary.total_all}</div>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

function StatCard({ label, value, color }: { label: string; value: number | string; color: string }) {
    const colorMap: Record<string, string> = {
        blue: 'text-blue-600',
        green: 'text-green-600',
        red: 'text-red-600',
        yellow: 'text-yellow-600',
    };
    return (
        <div className="bg-gray-50 rounded-lg p-4 border border-gray-100">
            <div className="text-xs text-gray-500 uppercase tracking-wider mb-1">{label}</div>
            <div className={`text-2xl font-bold ${colorMap[color] || 'text-gray-900'}`}>{value}</div>
        </div>
    );
}

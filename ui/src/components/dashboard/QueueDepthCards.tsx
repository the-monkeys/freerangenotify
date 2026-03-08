import { useState, useRef, useEffect } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Checkbox } from '../ui/checkbox';
import type { DLQItem } from '../../types';

interface Props {
    stats: Record<string, number>;
    dlqItems: DLQItem[];
    onReplay: (limit?: number) => void;
    replaying: boolean;
}

const MAX_HISTORY = 30; // ~2.5 min at 5s intervals

const PRIORITY_STYLES: Record<string, string> = {
    high: 'border-l-4 border-l-red-400',
    normal: 'border-l-4 border-l-border',
    low: 'border-l-4 border-l-blue-400',
};

const SPARKLINE_COLORS: Record<string, string> = {
    high: '#f87171',   // red-400
    normal: '#a1a1aa', // zinc-400
    low: '#60a5fa',    // blue-400
};

function Sparkline({ data, color }: { data: number[]; color: string }) {
    if (data.length < 2) return null;
    const w = 120, h = 28, pad = 1;
    const max = Math.max(...data, 1);
    const points = data.map((v, i) => {
        const x = pad + (i / (data.length - 1)) * (w - pad * 2);
        const y = h - pad - (v / max) * (h - pad * 2);
        return `${x},${y}`;
    });
    const fillPoints = `${pad},${h - pad} ${points.join(' ')} ${w - pad},${h - pad}`;
    return (
        <svg width={w} height={h} className="mt-2" aria-label="Queue depth trend">
            <polygon points={fillPoints} fill={color} opacity={0.12} />
            <polyline points={points.join(' ')} fill="none" stroke={color} strokeWidth={1.5} strokeLinejoin="round" strokeLinecap="round" />
        </svg>
    );
}

function extractPriority(queueName: string): string {
    const parts = queueName.replace('frn:queue:', '').toLowerCase();
    if (parts.includes('high')) return 'high';
    if (parts.includes('low')) return 'low';
    return 'normal';
}

export default function QueueDepthCards({ stats, dlqItems, onReplay, replaying }: Props) {
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
    const historyRef = useRef<Record<string, number[]>>({});

    // Accumulate rolling history per queue each time stats change
    useEffect(() => {
        for (const [queue, count] of Object.entries(stats)) {
            const arr = historyRef.current[queue] || [];
            arr.push(count);
            if (arr.length > MAX_HISTORY) arr.shift();
            historyRef.current[queue] = arr;
        }
    }, [stats]);

    const entries = Object.entries(stats);
    const totalMessages = entries.reduce((s, [, c]) => s + c, 0);
    const maxCount = Math.max(...entries.map(([, c]) => c), 1);

    const toggleId = (id: string) => {
        setSelectedIds(prev => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
        });
    };

    const toggleAll = () => {
        if (selectedIds.size === dlqItems.length) {
            setSelectedIds(new Set());
        } else {
            setSelectedIds(new Set(dlqItems.map(d => d.notification_id)));
        }
    };

    const handleReplaySelected = () => {
        // Backend currently supports replay by limit, not IDs.
        // Pass the count of selected items as limit.
        onReplay(selectedIds.size);
        setSelectedIds(new Set());
    };

    return (
        <div className="space-y-6">
            {/* Priority Queue Cards */}
            <div>
                <div className="flex items-center justify-between mb-3">
                    <h3 className="text-lg font-semibold">Message Queues</h3>
                    <Badge variant="outline" className="text-xs">
                        {totalMessages} total
                    </Badge>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {entries.map(([queue, count]) => {
                        const priority = extractPriority(queue);
                        const pct = maxCount > 0 ? (count / maxCount) * 100 : 0;
                        return (
                            <Card key={queue} className={PRIORITY_STYLES[priority] || ''}>
                                <CardContent className="pt-5 pb-4">
                                    <h4 className="text-xs text-muted-foreground uppercase mb-2 tracking-wider">
                                        {queue.replace('frn:queue:', '').toUpperCase()}
                                    </h4>
                                    <div className="flex justify-between items-end mb-3">
                                        <span className={`text-3xl font-bold ${count > 0 ? 'text-foreground' : 'text-muted-foreground'}`}>
                                            {count}
                                        </span>
                                        <span className="text-xs text-muted-foreground">messages</span>
                                    </div>
                                    {/* Depth bar */}
                                    <div className="h-1.5 bg-muted rounded-full overflow-hidden">
                                        <div
                                            className="h-full rounded-full transition-all duration-500 bg-foreground/30"
                                            style={{ width: `${Math.max(pct, 2)}%` }}
                                        />
                                    </div>
                                    {/* Sparkline trend */}
                                    <Sparkline
                                        data={historyRef.current[queue] || []}
                                        color={SPARKLINE_COLORS[priority] || SPARKLINE_COLORS.normal}
                                    />
                                </CardContent>
                            </Card>
                        );
                    })}
                </div>
            </div>

            {/* DLQ Section */}
            <Card>
                <CardHeader>
                    <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3">
                        <CardTitle className="text-red-600">Dead Letter Queue</CardTitle>
                        <div className="flex items-center gap-2">
                            <Badge
                                variant={dlqItems.length > 0 ? 'destructive' : 'outline'}
                                className={dlqItems.length > 0 ? 'bg-red-100 text-red-700 border-red-300' : 'bg-green-100 text-green-700 border-green-300'}
                            >
                                {dlqItems.length} items
                            </Badge>
                            {selectedIds.size > 0 && (
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={replaying}
                                    onClick={handleReplaySelected}
                                >
                                    {replaying ? 'Replaying...' : `Replay Selected (${selectedIds.size})`}
                                </Button>
                            )}
                            {dlqItems.length > 0 && selectedIds.size === 0 && (
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={replaying}
                                    onClick={() => onReplay(50)}
                                >
                                    {replaying ? 'Replaying...' : 'Replay All'}
                                </Button>
                            )}
                        </div>
                    </div>
                </CardHeader>
                <CardContent>
                    {dlqItems.length === 0 ? (
                        <p className="text-muted-foreground italic text-sm">No failed messages in DLQ.</p>
                    ) : (
                        <div className="overflow-x-auto">
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead className="w-8">
                                            <Checkbox
                                                checked={selectedIds.size === dlqItems.length && dlqItems.length > 0}
                                                onCheckedChange={toggleAll}
                                                aria-label="Select all"
                                            />
                                        </TableHead>
                                        <TableHead>Notification ID</TableHead>
                                        <TableHead>Priority</TableHead>
                                        <TableHead>Reason</TableHead>
                                        <TableHead>Failed At</TableHead>
                                        <TableHead>Retries</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {dlqItems.map((item, idx) => (
                                        <TableRow key={idx}>
                                            <TableCell>
                                                <Checkbox
                                                    checked={selectedIds.has(item.notification_id)}
                                                    onCheckedChange={() => toggleId(item.notification_id)}
                                                    aria-label={`Select ${item.notification_id}`}
                                                />
                                            </TableCell>
                                            <TableCell className="font-mono text-xs text-muted-foreground">
                                                {(item.notification_id || 'N/A').substring(0, 12)}...
                                            </TableCell>
                                            <TableCell>
                                                <Badge variant="outline" className="text-xs">{item.priority || 'normal'}</Badge>
                                            </TableCell>
                                            <TableCell className="text-red-600 text-sm">{item.reason}</TableCell>
                                            <TableCell className="text-foreground text-sm">
                                                {item.timestamp ? new Date(item.timestamp).toLocaleString() : 'N/A'}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">{item.retry_count ?? '—'}</TableCell>
                                        </TableRow>
                                    ))}
                                </TableBody>
                            </Table>
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}

import type { ChannelAnalytics } from '../../types';

const CHANNEL_COLORS: Record<string, string> = {
    email: '#3B82F6',
    push: '#8B5CF6',
    sms: '#10B981',
    webhook: '#F59E0B',
    sse: '#EC4899',
    in_app: '#6366F1',
};

interface Props {
    channels: ChannelAnalytics[];
    totalSent: number;
}

export default function ChannelBreakdownChart({ channels, totalSent }: Props) {
    if (!channels || channels.length === 0) return null;

    const totalDelivered = channels.reduce((s, c) => s + c.delivered, 0);
    const deliveryRate = totalSent > 0 ? (totalDelivered / totalSent) * 100 : 0;

    const rateColor =
        deliveryRate >= 95 ? '#10B981' : deliveryRate >= 80 ? '#F59E0B' : '#EF4444';

    // conic-gradient for donut
    const conicGradient = `conic-gradient(${rateColor} 0deg, ${rateColor} ${deliveryRate * 3.6}deg, #E5E7EB ${deliveryRate * 3.6}deg, #E5E7EB 360deg)`;

    return (
        <div className="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-6 items-start">
            {/* Horizontal bar chart */}
            <div className="space-y-3">
                <h4 className="text-sm font-semibold mb-2">Channel Distribution</h4>
                {channels.map(ch => {
                    const pct = totalSent > 0 ? (ch.total / totalSent) * 100 : 0;
                    const color = CHANNEL_COLORS[ch.channel] || '#9CA3AF';
                    return (
                        <div key={ch.channel} className="space-y-1">
                            <div className="flex justify-between items-center text-xs">
                                <span className="uppercase font-medium text-muted-foreground">{ch.channel}</span>
                                <span className="font-mono text-muted-foreground">
                                    {ch.total} ({pct.toFixed(1)}%)
                                </span>
                            </div>
                            <div className="h-3 bg-muted rounded-full overflow-hidden">
                                <div
                                    className="h-full rounded-full transition-all duration-500"
                                    style={{ width: `${Math.max(pct, 1)}%`, backgroundColor: color }}
                                />
                            </div>
                        </div>
                    );
                })}

                {/* Legend */}
                <div className="flex flex-wrap gap-3 pt-2">
                    {channels.map(ch => (
                        <div key={ch.channel} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                            <span
                                className="inline-block w-2.5 h-2.5 rounded-full"
                                style={{ backgroundColor: CHANNEL_COLORS[ch.channel] || '#9CA3AF' }}
                            />
                            {ch.channel}
                        </div>
                    ))}
                </div>
            </div>

            {/* Delivery rate donut */}
            <div className="flex flex-col items-center gap-2">
                <h4 className="text-sm font-semibold">Delivery Rate</h4>
                <div
                    className="relative w-28 h-28 rounded-full"
                    style={{ background: conicGradient }}
                >
                    <div className="absolute inset-3 bg-background rounded-full flex items-center justify-center">
                        <span className="text-lg font-bold" style={{ color: rateColor }}>
                            {deliveryRate.toFixed(1)}%
                        </span>
                    </div>
                </div>
                <p className="text-xs text-muted-foreground">
                    {totalDelivered.toLocaleString()} / {totalSent.toLocaleString()}
                </p>
            </div>
        </div>
    );
}

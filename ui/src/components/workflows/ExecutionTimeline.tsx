import React from 'react';
import type { StepResult, StepResultStatus } from '../../types';
import { CheckCircle2, XCircle, Clock, Loader2, SkipForward } from 'lucide-react';
import { formatDuration } from '../../lib/utils';

interface ExecutionTimelineProps {
    stepResults: Record<string, StepResult>;
}

const statusConfig: Record<StepResultStatus, { icon: React.ElementType; color: string; label: string }> = {
    completed: { icon: CheckCircle2, color: 'text-emerald-500', label: 'Completed' },
    failed:    { icon: XCircle,      color: 'text-red-500',     label: 'Failed' },
    running:   { icon: Loader2,      color: 'text-blue-500',    label: 'Running' },
    pending:   { icon: Clock,        color: 'text-muted-foreground', label: 'Pending' },
    skipped:   { icon: SkipForward,  color: 'text-amber-500',   label: 'Skipped' },
};

const ExecutionTimeline: React.FC<ExecutionTimelineProps> = ({ stepResults }) => {
    const entries = Object.entries(stepResults).sort(
        ([, a], [, b]) => (a.started_at || '').localeCompare(b.started_at || '')
    );

    if (entries.length === 0) {
        return (
            <p className="text-sm text-muted-foreground py-4">No step results yet.</p>
        );
    }

    return (
        <div className="relative pl-6">
            {/* Vertical line */}
            <div className="absolute left-[11px] top-2 bottom-2 w-px bg-border" />

            {entries.map(([stepId, result], index) => {
                const cfg = statusConfig[result.status] || statusConfig.pending;
                const Icon = cfg.icon;
                const duration = result.started_at && result.completed_at
                    ? formatDuration(result.started_at, result.completed_at)
                    : null;
                const isCompleted = result.status === 'completed';
                const isFailed = result.status === 'failed';

                return (
                    <div
                        key={stepId}
                        className={`relative flex items-start gap-3 pb-5 last:pb-0 rounded-r-lg pr-2 py-2 ${
                            isCompleted
                                ? 'bg-emerald-50 dark:bg-emerald-950/30 border-l-2 border-l-emerald-500'
                                : isFailed
                                    ? 'bg-red-50 dark:bg-red-950/30 border-l-2 border-l-red-500'
                                    : ''
                        }`}
                    >
                        {/* Dot / Icon */}
                        <div className={`absolute -left-6 mt-0.5 flex items-center justify-center w-[22px] h-[22px] rounded-full bg-background border-2 ${isCompleted ? 'border-emerald-500' : isFailed ? 'border-red-500' : 'border-border'} ${cfg.color}`}>
                            <Icon className={`h-3.5 w-3.5 ${result.status === 'running' ? 'animate-spin' : ''}`} />
                        </div>

                        {/* Content */}
                        <div className="min-w-0 flex-1">
                            <div className="flex items-center gap-2">
                                <span className="text-sm font-medium text-foreground">
                                    Step {index + 1}
                                </span>
                                <span className="font-mono text-xs text-muted-foreground truncate">
                                    {result.step_id}
                                </span>
                            </div>

                            <div className="flex items-center gap-3 mt-0.5">
                                <span className={`text-xs font-medium ${cfg.color}`}>
                                    {cfg.label}
                                </span>
                                {duration && (
                                    <span className="text-xs text-muted-foreground">
                                        {duration}
                                    </span>
                                )}
                                {result.notification_id && (
                                    <span className="font-mono text-xs text-muted-foreground">
                                        notif: {result.notification_id.slice(0, 8)}…
                                    </span>
                                )}
                                {result.digest_count != null && (
                                    <span className="text-xs text-muted-foreground">
                                        {result.digest_count} digested
                                    </span>
                                )}
                            </div>

                            {result.error && (
                                <p className="mt-1 text-xs text-red-500 bg-red-50 dark:bg-red-950/30 px-2 py-1 rounded">
                                    {result.error}
                                </p>
                            )}
                        </div>
                    </div>
                );
            })}
        </div>
    );
};

export default ExecutionTimeline;

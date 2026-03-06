import React from 'react';
import type { WorkflowStep } from '../../types';
import { Button } from '../ui/button';
import { Send, Clock, Layers, GitBranch, Pencil, X } from 'lucide-react';

const stepMeta: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
    channel: { icon: <Send className="h-4 w-4" />, color: 'border-l-blue-500', label: 'Channel' },
    delay: { icon: <Clock className="h-4 w-4" />, color: 'border-l-amber-500', label: 'Delay' },
    digest: { icon: <Layers className="h-4 w-4" />, color: 'border-l-purple-500', label: 'Digest' },
    condition: { icon: <GitBranch className="h-4 w-4" />, color: 'border-l-green-500', label: 'Condition' },
};

function getStepSummary(step: WorkflowStep): string {
    const c = step.config;
    switch (step.type) {
        case 'channel':
            return `Channel: ${c.channel || '—'}, Template: ${c.template_id ? c.template_id.slice(0, 12) + '...' : 'none'}`;
        case 'delay':
            return `Wait: ${c.duration || '—'}`;
        case 'digest':
            return `Key: ${c.digest_key || '—'}, Window: ${c.window || '—'}`;
        case 'condition':
            return c.condition
                ? `IF ${c.condition.field} ${c.condition.operator} ${JSON.stringify(c.condition.value)}`
                : 'No condition configured';
        default:
            return '';
    }
}

interface WorkflowStepCardProps {
    step: WorkflowStep;
    index: number;
    onEdit: () => void;
    onRemove: () => void;
}

const WorkflowStepCard: React.FC<WorkflowStepCardProps> = ({
    step,
    index,
    onEdit,
    onRemove,
}) => {
    const meta = stepMeta[step.type] || stepMeta.channel;

    return (
        <div className={`group border border-border rounded-lg ${meta.color} border-l-[3px] bg-card`}>
            <div className="flex items-center justify-between px-4 py-3">
                <div className="flex items-center gap-3 min-w-0">
                    <span className="text-muted-foreground">{meta.icon}</span>
                    <div className="min-w-0">
                        <p className="text-sm font-medium text-foreground">
                            Step {index + 1}: {step.name || meta.label}
                        </p>
                        <p className="text-xs text-muted-foreground truncate">
                            {getStepSummary(step)}
                        </p>
                    </div>
                </div>
                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button variant="ghost" size="sm" onClick={onEdit} className="h-7 w-7 p-0">
                        <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={onRemove} className="h-7 w-7 p-0">
                        <X className="h-3.5 w-3.5" />
                    </Button>
                </div>
            </div>
        </div>
    );
};

export default WorkflowStepCard;

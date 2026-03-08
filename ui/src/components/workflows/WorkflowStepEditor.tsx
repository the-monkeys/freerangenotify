import React, { useState, useEffect } from 'react';
import type { WorkflowStep, WorkflowStepType, StepConfig, StepCondition, Template } from '../../types';
import { templatesAPI } from '../../services/api';
import ResourcePicker from '../ResourcePicker';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Badge } from '../ui/badge';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '../ui/select';

const STEP_TYPES: { value: WorkflowStepType; label: string; description: string }[] = [
    { value: 'channel', label: 'Channel', description: 'Send via a delivery channel' },
    { value: 'delay', label: 'Delay', description: 'Wait before continuing' },
    { value: 'digest', label: 'Digest', description: 'Batch events together' },
    { value: 'condition', label: 'Condition', description: 'Branch on a condition' },
];

const CHANNELS = [
    'email', 'push', 'sms', 'webhook', 'sse',
    'slack', 'discord', 'whatsapp',
];

const OPERATORS: { value: string; label: string }[] = [
    { value: 'eq', label: 'equals' },
    { value: 'neq', label: 'not equals' },
    { value: 'contains', label: 'contains' },
    { value: 'gt', label: 'greater than' },
    { value: 'lt', label: 'less than' },
    { value: 'exists', label: 'exists' },
    { value: 'not_read', label: 'not read' },
];

interface WorkflowStepEditorProps {
    step: Partial<WorkflowStep> | null;
    apiKey: string;
    onSave: (step: Omit<WorkflowStep, 'id'>) => void;
    onCancel: () => void;
}

const WorkflowStepEditor: React.FC<WorkflowStepEditorProps> = ({
    step,
    apiKey,
    onSave,
    onCancel,
}) => {
    const [type, setType] = useState<WorkflowStepType>(step?.type || 'channel');
    const [name, setName] = useState(step?.name || '');
    const [config, setConfig] = useState<StepConfig>(step?.config || {});
    const [skipIf, setSkipIf] = useState<StepCondition | undefined>(step?.skip_if);

    // Reset config when type changes (but not on initial mount)
    const [initialized, setInitialized] = useState(false);
    useEffect(() => {
        if (initialized) {
            setConfig({});
            setSkipIf(undefined);
        }
        setInitialized(true);
    }, [type]);

    const isValid = (): boolean => {
        if (!name.trim() && !type) return false;
        switch (type) {
            case 'channel':
                return !!config.channel && !!config.template_id;
            case 'delay':
                return !!config.duration;
            case 'digest':
                return !!config.digest_key && !!config.window;
            case 'condition':
                return !!config.condition?.field && !!config.condition?.operator;
            default:
                return true;
        }
    };

    const handleSave = () => {
        const stepName = name.trim() || STEP_TYPES.find(t => t.value === type)?.label || 'Step';
        onSave({
            name: stepName,
            type,
            order: step?.order ?? 0,
            config,
            skip_if: skipIf,
        });
    };

    return (
        <div className="space-y-6 p-1">
            {/* Step Type */}
            <div className="space-y-2">
                <Label>Step Type</Label>
                <Select value={type} onValueChange={(v) => setType(v as WorkflowStepType)}>
                    <SelectTrigger>
                        <SelectValue placeholder="Select step type" />
                    </SelectTrigger>
                    <SelectContent>
                        {STEP_TYPES.map(st => (
                            <SelectItem key={st.value} value={st.value}>
                                {st.label} — {st.description}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </div>

            {/* Name */}
            <div className="space-y-2">
                <Label>Step Name</Label>
                <Input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder={`e.g., Send welcome email`}
                />
                <p className="text-xs text-muted-foreground">
                    A descriptive name for this step
                </p>
            </div>

            {/* ── Channel Config ── */}
            {type === 'channel' && (
                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>Channel <span className="text-destructive">*</span></Label>
                        <Select
                            value={config.channel || ''}
                            onValueChange={(v) => setConfig({ ...config, channel: v, template_id: '' })}
                        >
                            <SelectTrigger>
                                <SelectValue placeholder="Select channel" />
                            </SelectTrigger>
                            <SelectContent>
                                {CHANNELS.map(ch => (
                                    <SelectItem key={ch} value={ch}>{ch}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    {config.channel && (
                        <ResourcePicker<Template>
                            key={`tpl-${config.channel}`}
                            label="Template"
                            value={config.template_id || null}
                            onChange={(id) => setConfig({ ...config, template_id: id || '' })}
                            fetcher={async () => {
                                const res = await templatesAPI.list(apiKey, 100, 0);
                                return (res.templates || []).filter(
                                    (t: Template) => !config.channel || t.channel === config.channel
                                );
                            }}
                            labelKey="name"
                            valueKey="id"
                            renderItem={(t: Template) => (
                                <div className="flex items-center justify-between w-full">
                                    <span>{t.name}</span>
                                    <Badge variant="outline" className="text-xs">{t.channel}</Badge>
                                </div>
                            )}
                            hint="Select the template for this notification. Only templates matching the selected channel are shown."
                            placeholder="Search templates..."
                            required
                        />
                    )}

                    <div className="space-y-2">
                        <Label>Provider (optional)</Label>
                        <Input
                            value={config.provider || ''}
                            onChange={(e) => setConfig({ ...config, provider: e.target.value })}
                            placeholder="e.g., sendgrid, custom-smtp"
                        />
                        <p className="text-xs text-muted-foreground">
                            Override the default provider for this channel
                        </p>
                    </div>
                </div>
            )}

            {/* ── Delay Config ── */}
            {type === 'delay' && (
                <div className="space-y-2">
                    <Label>Duration <span className="text-destructive">*</span></Label>
                    <Input
                        value={config.duration || ''}
                        onChange={(e) => setConfig({ ...config, duration: e.target.value })}
                        placeholder="e.g., 30m, 1h, 24h, 7d"
                    />
                    <p className="text-xs text-muted-foreground">
                        How long to wait before proceeding to the next step.
                        Examples: &apos;30m&apos;, &apos;1h&apos;, &apos;24h&apos;, &apos;7d&apos;
                    </p>
                </div>
            )}

            {/* ── Digest Config ── */}
            {type === 'digest' && (
                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>Digest Key <span className="text-destructive">*</span></Label>
                        <Input
                            value={config.digest_key || ''}
                            onChange={(e) => setConfig({ ...config, digest_key: e.target.value })}
                            placeholder="e.g., user_activities"
                        />
                        <p className="text-xs text-muted-foreground">
                            Events with the same key are grouped together
                        </p>
                    </div>
                    <div className="space-y-2">
                        <Label>Window <span className="text-destructive">*</span></Label>
                        <Input
                            value={config.window || ''}
                            onChange={(e) => setConfig({ ...config, window: e.target.value })}
                            placeholder="e.g., 1h, 30m"
                        />
                        <p className="text-xs text-muted-foreground">
                            How long to accumulate events before flushing
                        </p>
                    </div>
                    <div className="space-y-2">
                        <Label>Max Batch</Label>
                        <Input
                            type="number"
                            value={config.max_batch ?? ''}
                            onChange={(e) => setConfig({ ...config, max_batch: parseInt(e.target.value) || 0 })}
                            placeholder="0 = unlimited"
                        />
                        <p className="text-xs text-muted-foreground">
                            Maximum events per digest — 0 means unlimited
                        </p>
                    </div>
                </div>
            )}

            {/* ── Condition Config ── */}
            {type === 'condition' && (
                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>Field <span className="text-destructive">*</span></Label>
                        <Input
                            value={config.condition?.field || ''}
                            onChange={(e) => setConfig({
                                ...config,
                                condition: {
                                    ...config.condition,
                                    field: e.target.value,
                                    operator: config.condition?.operator || 'eq',
                                    value: config.condition?.value ?? '',
                                },
                            })}
                            placeholder="e.g., payload.opened"
                        />
                        <p className="text-xs text-muted-foreground">
                            JSON path in the payload to evaluate
                        </p>
                    </div>
                    <div className="space-y-2">
                        <Label>Operator <span className="text-destructive">*</span></Label>
                        <Select
                            value={config.condition?.operator || 'eq'}
                            onValueChange={(v) => setConfig({
                                ...config,
                                condition: {
                                    ...config.condition,
                                    field: config.condition?.field || '',
                                    operator: v as StepCondition['operator'],
                                    value: config.condition?.value ?? '',
                                },
                            })}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {OPERATORS.map(op => (
                                    <SelectItem key={op.value} value={op.value}>{op.label}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-2">
                        <Label>Value</Label>
                        <Input
                            value={config.condition?.value?.toString() || ''}
                            onChange={(e) => setConfig({
                                ...config,
                                condition: {
                                    ...config.condition,
                                    field: config.condition?.field || '',
                                    operator: config.condition?.operator || 'eq',
                                    value: e.target.value,
                                },
                            })}
                            placeholder="e.g., true, 5, opened"
                        />
                        <p className="text-xs text-muted-foreground">
                            The value to compare against. Not required for &apos;exists&apos; or &apos;not_read&apos; operators.
                        </p>
                    </div>
                </div>
            )}

            {/* Footer */}
            <div className="flex justify-end gap-3 pt-4 border-t border-border">
                <Button variant="outline" onClick={onCancel}>
                    Cancel
                </Button>
                <Button onClick={handleSave} disabled={!isValid()}>
                    {step?.name ? 'Update Step' : 'Add Step'}
                </Button>
            </div>
        </div>
    );
};

export default WorkflowStepEditor;

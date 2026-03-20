import React, { useState, useEffect } from 'react';
import { templatesAPI } from '../../services/api';
import type { ContentControl } from '../../types';
import { extractErrorMessage } from '../../lib/utils';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Textarea } from '../ui/textarea';
import { Label } from '../ui/label';
import { Checkbox } from '../ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Card, CardContent } from '../ui/card';
import { toast } from 'sonner';
import { ChevronDown, ChevronUp, Loader2 } from 'lucide-react';

interface TemplateControlsPanelProps {
    apiKey: string;
    templateId: string;
    templateName: string;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

const TemplateControlsPanel: React.FC<TemplateControlsPanelProps> = ({
    apiKey,
    templateId,
    templateName,
    open,
    onOpenChange,
}) => {
    const [controls, setControls] = useState<ContentControl[]>([]);
    const [values, setValues] = useState<Record<string, any>>({});
    const [loading, setLoading] = useState(false);
    const [saving, setSaving] = useState(false);
    const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
    const [controlFilter, setControlFilter] = useState('');

    useEffect(() => {
        if (open) {
            fetchControls();
        }
    }, [open, templateId]);

    const fetchControls = async () => {
        setLoading(true);
        try {
            const res = await templatesAPI.getControls(apiKey, templateId);
            setControls(res.controls || []);
            // Merge API values with defaults
            const apiValues = res.control_values || res.values || {};
            const merged: Record<string, any> = {};
            for (const ctrl of (res.controls || [])) {
                merged[ctrl.key] = apiValues?.[ctrl.key] ?? ctrl.default ?? '';
            }
            setValues(merged);
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to load controls'));
        } finally {
            setLoading(false);
        }
    };

    const handleSave = async () => {
        setSaving(true);
        try {
            await templatesAPI.updateControls(apiKey, templateId, { control_values: values });
            toast.success('Controls updated');
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to save controls'));
        } finally {
            setSaving(false);
        }
    };

    const handleResetDefaults = () => {
        const defaults: Record<string, any> = {};
        for (const ctrl of controls) {
            defaults[ctrl.key] = ctrl.default ?? '';
        }
        setValues(defaults);
        toast('Reset to defaults (not saved yet)');
    };

    const toggleGroup = (group: string) => {
        setCollapsedGroups(prev => ({ ...prev, [group]: !prev[group] }));
    };

    const updateValue = (key: string, val: any) => {
        setValues(prev => ({ ...prev, [key]: val }));
    };

    // Group controls
    const groups: Record<string, ContentControl[]> = {};
    for (const ctrl of controls) {
        const g = ctrl.group || 'General';
        if (!groups[g]) groups[g] = [];
        groups[g].push(ctrl);
    }

    const normalizedFilter = controlFilter.trim().toLowerCase();
    const filteredGroups = Object.entries(groups)
        .map(([groupName, groupControls]) => {
            if (!normalizedFilter) return [groupName, groupControls] as const;
            const filtered = groupControls.filter((ctrl) => {
                const label = ctrl.label?.toLowerCase() || '';
                const key = ctrl.key?.toLowerCase() || '';
                const help = ctrl.help_text?.toLowerCase() || '';
                return label.includes(normalizedFilter) || key.includes(normalizedFilter) || help.includes(normalizedFilter);
            });
            return [groupName, filtered] as const;
        })
        .filter(([, groupControls]) => groupControls.length > 0);

    const renderControl = (ctrl: ContentControl) => {
        const val = values[ctrl.key] ?? '';

        switch (ctrl.type) {
            case 'text':
                return (
                    <Input
                        value={val}
                        onChange={e => updateValue(ctrl.key, e.target.value)}
                        placeholder={ctrl.placeholder}
                    />
                );
            case 'textarea':
                return (
                    <Textarea
                        value={val}
                        onChange={e => updateValue(ctrl.key, e.target.value)}
                        placeholder={ctrl.placeholder}
                        className="h-20"
                    />
                );
            case 'url':
            case 'image':
                return (
                    <div className="space-y-1">
                        <Input
                            type="url"
                            value={val}
                            onChange={e => updateValue(ctrl.key, e.target.value)}
                            placeholder={ctrl.placeholder || 'https://...'}
                        />
                        {ctrl.type === 'image' && val && (
                            <img
                                src={val}
                                alt={ctrl.label}
                                className="h-12 w-auto rounded border border-border object-contain"
                                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                            />
                        )}
                    </div>
                );
            case 'color':
                return (
                    <div className="flex items-center gap-2">
                        <input
                            type="color"
                            value={val || '#000000'}
                            onChange={e => updateValue(ctrl.key, e.target.value)}
                            className="h-8 w-10 rounded border border-border cursor-pointer"
                        />
                        <span className="text-xs font-mono text-muted-foreground">{val || '#000000'}</span>
                    </div>
                );
            case 'number':
                return (
                    <Input
                        type="number"
                        value={val}
                        onChange={e => updateValue(ctrl.key, e.target.valueAsNumber || 0)}
                        placeholder={ctrl.placeholder}
                    />
                );
            case 'boolean':
                return (
                    <div className="flex items-center gap-2 pt-1">
                        <Checkbox
                            checked={!!val}
                            onCheckedChange={(checked) => updateValue(ctrl.key, !!checked)}
                        />
                        <span className="text-sm text-foreground">{val ? 'Enabled' : 'Disabled'}</span>
                    </div>
                );
            case 'select':
                return (
                    <Select value={String(val)} onValueChange={v => updateValue(ctrl.key, v)}>
                        <SelectTrigger>
                            <SelectValue placeholder={ctrl.placeholder || 'Select...'} />
                        </SelectTrigger>
                        <SelectContent>
                            {(ctrl.options || []).map(opt => (
                                <SelectItem key={opt} value={opt}>{opt}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                );
            default:
                return <Input value={val} onChange={e => updateValue(ctrl.key, e.target.value)} />;
        }
    };

    return (
        <SlidePanel
            open={open}
            onClose={() => onOpenChange(false)}
            title={`Content Controls: ${templateName}`}
        >
            <div className="space-y-4 p-1">
                {loading ? (
                    <div className="flex items-center justify-center py-12">
                        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                    </div>
                ) : controls.length === 0 ? (
                    <div className="text-center py-12">
                        <p className="text-sm text-muted-foreground">
                            This template has no content controls defined.
                        </p>
                        <p className="text-xs text-muted-foreground mt-1">
                            Add a <code>controls</code> array to your template to enable this feature.
                        </p>
                    </div>
                ) : (
                    <>
                        <Card className="border-border/80">
                            <CardContent className="space-y-2 p-3.5">
                                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                                    <Label className="text-xs">Find control</Label>
                                    <p className="text-xs text-muted-foreground">{controls.length} controls</p>
                                </div>
                                <Input
                                    value={controlFilter}
                                    onChange={(e) => setControlFilter(e.target.value)}
                                    placeholder="Search by label, key, or help text"
                                />
                            </CardContent>
                        </Card>

                        {filteredGroups.map(([groupName, groupControls]) => (
                            <div key={groupName} className="overflow-hidden rounded-lg border border-border">
                                <button
                                    onClick={() => toggleGroup(groupName)}
                                    className="w-full flex items-center justify-between px-4 py-2.5 bg-muted/45 text-sm font-medium text-foreground hover:bg-muted transition-colors"
                                >
                                    <span>{groupName}</span>
                                    <span className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                                        {groupControls.length}
                                        {collapsedGroups[groupName]
                                            ? <ChevronDown className="h-4 w-4" />
                                            : <ChevronUp className="h-4 w-4" />}
                                    </span>
                                </button>
                                {!collapsedGroups[groupName] && (
                                    <div className="space-y-3 p-4">
                                        {groupControls.map(ctrl => (
                                            <div key={ctrl.key} className="rounded-md border border-border/80 bg-background p-3 space-y-1.5">
                                                <div className="flex items-center justify-between gap-2">
                                                    <Label className="text-xs font-medium">{ctrl.label}</Label>
                                                    <span className="text-[11px] font-mono text-muted-foreground">{ctrl.key}</span>
                                                </div>
                                                {renderControl(ctrl)}
                                                {ctrl.help_text && (
                                                    <p className="text-xs text-muted-foreground">{ctrl.help_text}</p>
                                                )}
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                        ))}

                        {filteredGroups.length === 0 && (
                            <div className="rounded-lg border border-border/80 bg-muted/20 px-4 py-8 text-center text-sm text-muted-foreground">
                                No controls match the current search.
                            </div>
                        )}

                        <div className="flex gap-2 pt-2">
                            <Button
                                variant="outline"
                                onClick={handleResetDefaults}
                                className="flex-1"
                            >
                                Reset to Defaults
                            </Button>
                            <Button
                                onClick={handleSave}
                                disabled={saving}
                                className="flex-1"
                            >
                                {saving ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                                Save
                            </Button>
                        </div>
                    </>
                )}
            </div>
        </SlidePanel>
    );
};

export default TemplateControlsPanel;

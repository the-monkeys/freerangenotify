import React, { useState } from 'react';
import { templatesAPI } from '../../services/api';
import type { TemplateVersion, TemplateDiffResponse, TemplateDiffChange } from '../../types';
import { extractErrorMessage } from '../../lib/utils';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Label } from '../ui/label';
import { Badge } from '../ui/badge';
import { toast } from 'sonner';
import { ArrowRight, Loader2 } from 'lucide-react';

interface TemplateDiffViewerProps {
    apiKey: string;
    templateId: string;
    templateName: string;
    versions: TemplateVersion[];
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

const TemplateDiffViewer: React.FC<TemplateDiffViewerProps> = ({
    apiKey,
    templateId,
    templateName,
    versions,
    open,
    onOpenChange,
}) => {
    const [fromVersion, setFromVersion] = useState<string>('');
    const [toVersion, setToVersion] = useState<string>('');
    const [diff, setDiff] = useState<TemplateDiffResponse | null>(null);
    const [loading, setLoading] = useState(false);

    const handleCompare = async () => {
        if (!fromVersion || !toVersion) {
            toast.error('Select both versions to compare');
            return;
        }
        if (fromVersion === toVersion) {
            toast.error('Select two different versions');
            return;
        }
        setLoading(true);
        try {
            const result = await templatesAPI.diff(apiKey, templateId, Number(fromVersion), Number(toVersion));
            setDiff(result);
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to load diff'));
        } finally {
            setLoading(false);
        }
    };

    const renderValue = (val: any): string => {
        if (val === null || val === undefined) return '(empty)';
        if (typeof val === 'object') return JSON.stringify(val, null, 2);
        return String(val);
    };

    const normalizedChanges = (): Array<{ field: string; old: any; newer: any }> => {
        if (!diff) return [];
        if (Array.isArray(diff.changes)) {
            return (diff.changes as TemplateDiffChange[]).map((c) => ({
                field: c.field,
                old: c.from,
                newer: c.to,
            }));
        }
        return Object.entries(diff.changes || {}).map(([field, change]) => ({
            field,
            old: change.old,
            newer: change.new,
        }));
    };

    return (
        <SlidePanel
            open={open}
            onClose={() => onOpenChange(false)}
            title={`Compare Versions: ${templateName}`}
        >
            <div className="space-y-5 p-1">
                <div className="rounded-lg border border-border/70 bg-muted/30 p-3 text-xs text-muted-foreground">
                    Pick two template versions to compare field-level changes.
                </div>

                <div className="rounded-xl border border-border bg-background p-3.5 sm:p-4">
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_auto_1fr_auto] sm:items-end">
                        <div className="space-y-1.5">
                            <Label className="text-xs">From version</Label>
                            <Select value={fromVersion} onValueChange={setFromVersion}>
                                <SelectTrigger>
                                    <SelectValue placeholder="Select version" />
                                </SelectTrigger>
                                <SelectContent>
                                    {versions.map(v => (
                                        <SelectItem key={v.id} value={String(v.version)}>
                                            v{v.version}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="hidden sm:flex h-9 items-center justify-center text-muted-foreground">
                            <ArrowRight className="h-4 w-4" />
                        </div>
                        <div className="space-y-1.5">
                            <Label className="text-xs">To version</Label>
                            <Select value={toVersion} onValueChange={setToVersion}>
                                <SelectTrigger>
                                    <SelectValue placeholder="Select version" />
                                </SelectTrigger>
                                <SelectContent>
                                    {versions.map(v => (
                                        <SelectItem key={v.id} value={String(v.version)}>
                                            v{v.version}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <Button onClick={handleCompare} disabled={loading || !fromVersion || !toVersion} size="sm">
                            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Compare'}
                        </Button>
                    </div>
                </div>

                {/* Diff display */}
                {diff && (
                    <div className="space-y-4">
                        {normalizedChanges().length === 0 ? (
                            <p className="text-sm text-muted-foreground text-center py-8">
                                No differences between v{diff.from_version} and v{diff.to_version}.
                            </p>
                        ) : (
                            normalizedChanges().map((change) => (
                                <div key={change.field} className="space-y-2 rounded-xl border border-border bg-background p-3">
                                    <div className="flex items-center justify-between">
                                        <p className="text-sm font-medium text-foreground">{change.field}</p>
                                        <Badge variant="outline" className="text-[11px]">
                                            v{diff.from_version} to v{diff.to_version}
                                        </Badge>
                                    </div>
                                    <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2">
                                        <pre className="whitespace-pre-wrap text-xs text-foreground font-mono">
                                            - {renderValue(change.old)}
                                        </pre>
                                    </div>
                                    <div className="rounded-md border border-emerald-500/30 bg-emerald-500/10 px-3 py-2">
                                        <pre className="whitespace-pre-wrap text-xs text-foreground font-mono">
                                            + {renderValue(change.newer)}
                                        </pre>
                                    </div>
                                </div>
                            ))
                        )}
                    </div>
                )}

                {!diff && !loading && (
                    <p className="text-sm text-muted-foreground text-center py-8">
                        Select two versions and click Compare to see differences.
                    </p>
                )}
            </div>
        </SlidePanel>
    );
};

export default TemplateDiffViewer;

import React, { useState } from 'react';
import { digestRulesAPI, applicationsAPI } from '../../services/api';
import type { DigestRule, DigestRuleStatus, CreateDigestRuleRequest, Application } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import ResourcePicker from '../../components/ResourcePicker';
import SkeletonTable from '../../components/SkeletonTable';
import EmptyState from '../../components/EmptyState';
import ConfirmDialog from '../../components/ConfirmDialog';
import { Pagination } from '../../components/Pagination';
import { SlidePanel } from '../../components/ui/slide-panel';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Badge } from '../../components/ui/badge';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '../../components/ui/select';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '../../components/ui/table';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';
import { Timer, Plus, MoreHorizontal, Pencil, Trash2, Loader2 } from 'lucide-react';
import { timeAgo } from '../../lib/utils';
import { toast } from 'sonner';

interface DigestRulesListProps {
    apiKey?: string;
    embedded?: boolean;
}

const CHANNELS = ['email', 'sms', 'push', 'webhook', 'in_app', 'sse'] as const;
const PAGE_SIZE = 15;

const DigestRulesList: React.FC<DigestRulesListProps> = ({ apiKey: propApiKey, embedded }) => {
    const [ownApiKey, setOwnApiKey] = useState<string | null>(
        propApiKey || localStorage.getItem('last_api_key')
    );
    const [selectedAppId, setSelectedAppId] = useState<string | null>(
        localStorage.getItem('last_app_id')
    );
    const apiKey = propApiKey || ownApiKey;

    const [page, setPage] = useState(1);
    const offset = (page - 1) * PAGE_SIZE;

    const { data, loading, refetch } = useApiQuery(
        () => digestRulesAPI.list(apiKey!, PAGE_SIZE, offset),
        [apiKey, offset],
        { enabled: !!apiKey }
    );

    // Editor state
    const [showEditor, setShowEditor] = useState(false);
    const [editingRule, setEditingRule] = useState<DigestRule | null>(null);
    const [saving, setSaving] = useState(false);

    // Form fields
    const [formName, setFormName] = useState('');
    const [formKey, setFormKey] = useState('');
    const [formWindow, setFormWindow] = useState('5m');
    const [formChannel, setFormChannel] = useState('email');
    const [formTemplateId, setFormTemplateId] = useState('');
    const [formMaxBatch, setFormMaxBatch] = useState(50);

    // Delete
    const [deleteTarget, setDeleteTarget] = useState<DigestRule | null>(null);

    const handleAppSelect = async (appId: string | null) => {
        if (!appId) return;
        try {
            const apps = await applicationsAPI.list();
            const app = apps.find((a: Application) => a.app_id === appId);
            if (app) {
                setSelectedAppId(app.app_id);
                setOwnApiKey(app.api_key);
                localStorage.setItem('last_app_id', app.app_id);
                localStorage.setItem('last_api_key', app.api_key);
            }
        } catch {
            toast.error('Failed to load application');
        }
    };

    const openCreate = () => {
        setEditingRule(null);
        setFormName('');
        setFormKey('');
        setFormWindow('5m');
        setFormChannel('email');
        setFormTemplateId('');
        setFormMaxBatch(50);
        setShowEditor(true);
    };

    const openEdit = (rule: DigestRule) => {
        setEditingRule(rule);
        setFormName(rule.name);
        setFormKey(rule.digest_key);
        setFormWindow(rule.window);
        setFormChannel(rule.channel);
        setFormTemplateId(rule.template_id);
        setFormMaxBatch(rule.max_batch);
        setShowEditor(true);
    };

    const handleSave = async () => {
        if (!apiKey || !formName.trim() || !formKey.trim()) {
            toast.error('Name and Digest Key are required');
            return;
        }
        setSaving(true);
        try {
            if (editingRule) {
                await digestRulesAPI.update(apiKey, editingRule.id, {
                    name: formName.trim(),
                    digest_key: formKey.trim(),
                    window: formWindow,
                    channel: formChannel,
                    template_id: formTemplateId,
                    max_batch: formMaxBatch,
                });
                toast.success('Digest rule updated');
            } else {
                const payload: CreateDigestRuleRequest = {
                    name: formName.trim(),
                    digest_key: formKey.trim(),
                    window: formWindow,
                    channel: formChannel,
                    template_id: formTemplateId,
                    max_batch: formMaxBatch,
                };
                await digestRulesAPI.create(apiKey, payload);
                toast.success('Digest rule created');
            }
            setShowEditor(false);
            refetch();
        } catch {
            toast.error('Failed to save digest rule');
        } finally {
            setSaving(false);
        }
    };

    const handleDelete = async () => {
        if (!apiKey || !deleteTarget) return;
        try {
            await digestRulesAPI.delete(apiKey, deleteTarget.id);
            toast.success('Digest rule deleted');
            setDeleteTarget(null);
            refetch();
        } catch {
            toast.error('Failed to delete digest rule');
        }
    };

    const handleToggleStatus = async (rule: DigestRule) => {
        if (!apiKey) return;
        const newStatus: DigestRuleStatus = rule.status === 'active' ? 'inactive' : 'active';
        try {
            await digestRulesAPI.update(apiKey, rule.id, { status: newStatus });
            toast.success(`Rule ${newStatus === 'active' ? 'activated' : 'deactivated'}`);
            refetch();
        } catch {
            toast.error('Failed to update status');
        }
    };

    const rules: DigestRule[] = data?.rules || [];
    const total: number = data?.total || 0;

    // No api key — show picker (standalone mode only)
    if (!apiKey && !embedded) {
        return (
            <div className="p-6 max-w-6xl mx-auto space-y-6">
                <h1 className="text-2xl font-semibold text-foreground">Digest Rules</h1>
                <div className="max-w-xs">
                    <ResourcePicker<Application>
                        label="Application"
                        value={selectedAppId}
                        onChange={handleAppSelect}
                        fetcher={async () => applicationsAPI.list()}
                        labelKey="app_name"
                        valueKey="app_id"
                        placeholder="Select an application..."
                    />
                </div>
            </div>
        );
    }

    return (
        <div className={embedded ? 'space-y-4' : 'p-6 max-w-6xl mx-auto space-y-6'}>
            {/* Header */}
            {!embedded && (
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <Timer className="h-5 w-5 text-muted-foreground" />
                        <h1 className="text-2xl font-semibold text-foreground">Digest Rules</h1>
                    </div>
                    <Button onClick={openCreate}>
                        <Plus className="h-4 w-4 mr-2" />
                        New Rule
                    </Button>
                </div>
            )}

            {embedded && (
                <div className="flex items-center justify-between">
                    <p className="text-sm text-muted-foreground">{total} rule{total !== 1 ? 's' : ''}</p>
                    <Button size="sm" onClick={openCreate}>
                        <Plus className="h-4 w-4 mr-1.5" />
                        New Rule
                    </Button>
                </div>
            )}

            {/* Table */}
            {loading ? (
                <SkeletonTable rows={5} columns={6} />
            ) : rules.length === 0 ? (
                <EmptyState
                    title="No digest rules"
                    description="Create a digest rule to aggregate notifications over a time window"
                    action={{ label: 'New Rule', onClick: openCreate }}
                />
            ) : (
                <>
                    <div className="border border-border rounded-lg overflow-hidden">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Name</TableHead>
                                    <TableHead>Digest Key</TableHead>
                                    <TableHead>Window</TableHead>
                                    <TableHead>Channel</TableHead>
                                    <TableHead>Status</TableHead>
                                    <TableHead>Updated</TableHead>
                                    <TableHead className="w-10" />
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {rules.map((rule) => (
                                    <TableRow key={rule.id}>
                                        <TableCell className="font-medium">{rule.name}</TableCell>
                                        <TableCell className="font-mono text-xs text-muted-foreground">
                                            {rule.digest_key}
                                        </TableCell>
                                        <TableCell className="text-sm">{rule.window}</TableCell>
                                        <TableCell>
                                            <Badge variant="secondary">{rule.channel}</Badge>
                                        </TableCell>
                                        <TableCell>
                                            <Badge
                                                variant={rule.status === 'active' ? 'default' : 'outline'}
                                                className="cursor-pointer"
                                                onClick={() => handleToggleStatus(rule)}
                                            >
                                                {rule.status}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-xs text-muted-foreground">
                                            {timeAgo(rule.updated_at)}
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button variant="ghost" size="sm">
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem onClick={() => openEdit(rule)}>
                                                        <Pencil className="h-3.5 w-3.5 mr-2" />
                                                        Edit
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        className="text-destructive"
                                                        onClick={() => setDeleteTarget(rule)}
                                                    >
                                                        <Trash2 className="h-3.5 w-3.5 mr-2" />
                                                        Delete
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>

                    {total > PAGE_SIZE && (
                        <Pagination
                            currentPage={page}
                            totalItems={total}
                            pageSize={PAGE_SIZE}
                            onPageChange={setPage}
                        />
                    )}
                </>
            )}

            {/* Editor Panel */}
            <SlidePanel
                open={showEditor}
                onClose={() => setShowEditor(false)}
                title={editingRule ? 'Edit Digest Rule' : 'New Digest Rule'}
            >
                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>Name <span className="text-destructive">*</span></Label>
                        <Input value={formName} onChange={(e) => setFormName(e.target.value)} placeholder="e.g., Comment Digest" />
                    </div>
                    <div className="space-y-2">
                        <Label>Digest Key <span className="text-destructive">*</span></Label>
                        <Input value={formKey} onChange={(e) => setFormKey(e.target.value)} placeholder="e.g., comments" className="font-mono" />
                        <p className="text-xs text-muted-foreground">Notifications with this key will be grouped together</p>
                    </div>
                    <div className="space-y-2">
                        <Label>Window</Label>
                        <Input value={formWindow} onChange={(e) => setFormWindow(e.target.value)} placeholder="e.g., 5m, 1h" className="font-mono" />
                        <p className="text-xs text-muted-foreground">Time window for batching (e.g. 5m, 1h, 30s)</p>
                    </div>
                    <div className="space-y-2">
                        <Label>Channel</Label>
                        <Select value={formChannel} onValueChange={setFormChannel}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {CHANNELS.map(ch => (
                                    <SelectItem key={ch} value={ch}>{ch}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-2">
                        <Label>Template ID</Label>
                        <Input value={formTemplateId} onChange={(e) => setFormTemplateId(e.target.value)} placeholder="Template ID for digest summary" className="font-mono" />
                    </div>
                    <div className="space-y-2">
                        <Label>Max Batch Size</Label>
                        <Input type="number" value={formMaxBatch} onChange={(e) => setFormMaxBatch(Number(e.target.value))} min={1} max={1000} />
                    </div>
                    <div className="flex items-center gap-2 pt-2">
                        <Button onClick={handleSave} disabled={saving}>
                            {saving && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                            {editingRule ? 'Update' : 'Create'}
                        </Button>
                        <Button variant="outline" onClick={() => setShowEditor(false)}>Cancel</Button>
                    </div>
                </div>
            </SlidePanel>

            {/* Delete Confirmation */}
            <ConfirmDialog
                open={!!deleteTarget}
                onOpenChange={() => setDeleteTarget(null)}
                onConfirm={handleDelete}
                title="Delete Digest Rule"
                description={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
                confirmLabel="Delete"
                variant="destructive"
            />
        </div>
    );
};

export default DigestRulesList;

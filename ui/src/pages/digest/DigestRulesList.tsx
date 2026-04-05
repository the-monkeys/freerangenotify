import React, { useEffect, useMemo, useState } from 'react';
import { digestRulesAPI, applicationsAPI, notificationsAPI, templatesAPI } from '../../services/api';
import type { DigestRule, DigestRuleStatus, CreateDigestRuleRequest, Application, Notification, Template } from '../../types';
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
import { Timer, Plus, MoreHorizontal, Pencil, Trash2, Loader2, Zap, Mail, Route, Users, List } from 'lucide-react';
import { timeAgo } from '../../lib/utils';
import { toast } from 'sonner';

interface DigestRulesListProps {
    apiKey?: string;
    embedded?: boolean;
}

const CHANNELS = ['email', 'sms', 'push', 'webhook', 'in_app', 'sse'] as const;
const PAGE_SIZE = 15;

const DigestedNotificationsPanel: React.FC<{
    apiKey: string;
    digestKey: string;
}> = ({ apiKey, digestKey }) => {
    const [page, setPage] = useState(1);
    const { data, loading } = useApiQuery(
        () => notificationsAPI.list(apiKey, page, 20, {
            digest_key: digestKey,
            status: 'digested',
        }),
        [apiKey, digestKey, page],
        { enabled: !!apiKey && !!digestKey }
    );
    const notifications: Notification[] = data?.notifications ?? [];
    const total = data?.total ?? 0;

    return (
        <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
                Notifications batched and delivered by this digest rule (status=digested, metadata.digest_key={digestKey})
            </p>
            {loading ? (
                <div className="py-8 text-center text-muted-foreground">Loading…</div>
            ) : notifications.length === 0 ? (
                <div className="py-8 text-center text-muted-foreground">No digested notifications yet.</div>
            ) : (
                <>
                    <p className="text-sm font-medium">{total} total</p>
                    <div className="border rounded-lg overflow-hidden">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>ID</TableHead>
                                    <TableHead>User</TableHead>
                                    <TableHead>Channel</TableHead>
                                    <TableHead>Created</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {notifications.map((n) => (
                                    <TableRow key={n.notification_id}>
                                        <TableCell className="font-mono text-xs">{n.notification_id?.slice(0, 8)}…</TableCell>
                                        <TableCell className="text-sm">{n.user_id?.slice(0, 12)}…</TableCell>
                                        <TableCell><Badge variant="outline">{n.channel}</Badge></TableCell>
                                        <TableCell className="text-xs text-muted-foreground">{n.created_at ? timeAgo(n.created_at) : '-'}</TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                    {total > 20 && (
                        <Pagination
                            currentPage={page}
                            totalItems={total}
                            pageSize={20}
                            onPageChange={setPage}
                        />
                    )}
                </>
            )}
        </div>
    );
};

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
        { 
            enabled: !!apiKey,
            cacheKey: `digest-rules-list-${apiKey}-${offset}`
        }

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
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loadingTemplates, setLoadingTemplates] = useState(false);
    const currentAppId = editingRule?.app_id || selectedAppId || null;
    const filteredTemplates = useMemo(() => {
        return templates.filter((t) => {
            const channelMatch = t.channel === formChannel;
            const appMatch = currentAppId ? t.app_id === currentAppId : true; // backwards compatible
            return channelMatch && appMatch;
        });
    }, [templates, formChannel, currentAppId]);

    // Delete
    const [deleteTarget, setDeleteTarget] = useState<DigestRule | null>(null);

    // Run history (digested notifications)
    const [viewingRule, setViewingRule] = useState<DigestRule | null>(null);

    const handleAppSelect = async (appId: string | null) => {
        if (!appId) return;
        try {
            const app = await applicationsAPI.get(appId);
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

    useEffect(() => {
        if (!showEditor) return;
        const key = propApiKey || ownApiKey;
        if (!key) return;
        setLoadingTemplates(true);
        // API enforces a max page size; use 100 to stay within limits
        templatesAPI.list(key, 100, 0)
            .then(res => setTemplates(res.templates || []))
            .catch(() => { /* ignore */ })
            .finally(() => setLoadingTemplates(false));
    }, [showEditor, propApiKey, ownApiKey]);

    const handleSave = async () => {
        if (!apiKey || !formName.trim() || !formKey.trim() || !formTemplateId) {
            toast.error('Name, Digest Key, and Template are required');
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
                <>
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

                    {/* App picker (standalone only) */}
                    <div className="max-w-xs mt-4">
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

                    <div className="rounded-lg border bg-card p-5 mt-6">
                        <div className="flex items-start gap-4">
                            <div className="rounded-lg bg-primary/10 p-3 shrink-0">
                                <Timer className="h-6 w-6 text-primary" />
                            </div>
                            <div className="space-y-3">
                                <div>
                                    <h3 className="font-semibold text-base">How Digest Rules Work</h3>
                                    <p className="text-muted-foreground text-sm mt-1">
                                        Digest rules <strong>batch multiple notifications together</strong> and send them as a single consolidated message.
                                        Instead of 20 separate emails, your users get one clean summary.
                                    </p>
                                </div>
                                <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-3 text-sm">
                                    <div className="rounded-md border p-3 space-y-1">
                                        <div className="font-medium flex items-center gap-1.5"><Zap className="h-3.5 w-3.5 text-amber-500" /> Without Digest</div>
                                        <p className="text-muted-foreground text-xs">20 events = 20 notifications sent instantly. That's noisy for users.</p>
                                    </div>
                                    <div className="rounded-md border p-3 space-y-1">
                                        <div className="font-medium flex items-center gap-1.5"><Mail className="h-3.5 w-3.5 text-blue-500" /> With Digest</div>
                                        <p className="text-muted-foreground text-xs">20 events = 1 consolidated notification after the time window ends.</p>
                                    </div>
                                    <div className="rounded-md border p-3 space-y-1">
                                        <div className="font-medium flex items-center gap-1.5"><Route className="h-3.5 w-3.5 text-green-500" /> Any Channel</div>
                                        <p className="text-muted-foreground text-xs">Works with email, SMS, push, webhook, SSE, and in-app channels.</p>
                                    </div>
                                    <div className="rounded-md border p-3 space-y-1">
                                        <div className="font-medium flex items-center gap-1.5"><Users className="h-3.5 w-3.5 text-violet-500" /> Per-User Batching</div>
                                        <p className="text-muted-foreground text-xs">Each user gets their own digest. One rule applies to all users automatically.</p>
                                    </div>
                                </div>
                                <p className="text-xs text-muted-foreground">
                                    <strong>How to use:</strong> Set a <code className="bg-muted px-1 rounded">digest_key</code> in your notification metadata.
                                    Any notification with a matching key will be batched by the rule's time window and delivered using the linked template.
                                </p>
                            </div>
                        </div>
                    </div>
                </>
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
                                                    <DropdownMenuItem onClick={() => setViewingRule(rule)}>
                                                        <List className="h-3.5 w-3.5 mr-2" />
                                                        View digested notifications
                                                    </DropdownMenuItem>
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

            {/* Digested Notifications Panel */}
            <SlidePanel
                open={!!viewingRule}
                onClose={() => setViewingRule(null)}
                title={viewingRule ? `Digested: ${viewingRule.name}` : ''}
            >
                {viewingRule && apiKey && (
                    <DigestedNotificationsPanel
                        apiKey={apiKey}
                        digestKey={viewingRule.digest_key}
                    />
                )}
            </SlidePanel>

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
                        <Label>Template <span className="text-destructive">*</span></Label>
                        <Select value={formTemplateId} onValueChange={setFormTemplateId} disabled={loadingTemplates}>
                            <SelectTrigger>
                                <SelectValue placeholder="Select a template" />
                            </SelectTrigger>
                            <SelectContent>
                                {filteredTemplates.map((tpl) => (
                                    <SelectItem key={tpl.id} value={tpl.id}>
                                        {tpl.name} ({tpl.channel})
                                    </SelectItem>
                                ))}
                                {filteredTemplates.length === 0 && (
                                    <SelectItem value={formTemplateId || 'custom'}>
                                        {loadingTemplates ? 'Loading...' : 'No templates for this channel'}
                                    </SelectItem>
                                )}
                            </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground">Templates are filtered by the selected channel.</p>
                        {formTemplateId && !templates.find(t => t.id === formTemplateId) && (
                            <p className="text-xs text-amber-600">Using custom template ID: {formTemplateId}</p>
                        )}
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
                onOpenChange={(open) => !open && setDeleteTarget(null)}
                onConfirm={handleDelete}
                title="Delete Digest Rule"
                description={deleteTarget ? `Are you sure you want to delete "${deleteTarget.name}"? This action cannot be undone.` : ''}
                confirmLabel="Delete"
                variant="destructive"
            />
        </div>
    );
};

export default DigestRulesList;

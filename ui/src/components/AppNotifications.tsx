import React, { useEffect, useState, useMemo } from 'react';
import { notificationsAPI, usersAPI, templatesAPI, quickSendAPI } from '../services/api';
import type { Notification, NotificationRequest, User, Template } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Pagination } from './Pagination';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger
} from './ui/dialog';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Textarea } from './ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Badge } from './ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Checkbox } from './ui/checkbox';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';
import { SlidePanel } from './ui/slide-panel';
import { CheckSquare, Archive, BellOff, Bell, Eye, X, Send, Ban } from 'lucide-react';
import { toast } from 'sonner';
import { timeAgo } from '../lib/utils';

interface AppNotificationsProps {
    appId: string;
    apiKey: string;
    webhooks?: Record<string, string>;
    onUnreadCount?: (count: number) => void;
}

const createEmptyForm = (): NotificationRequest => ({
    user_id: '',
    channel: 'email',
    priority: 'normal',
    title: '',
    body: '',
    template_id: '',
    webhook_url: '',
    data: {},
    scheduled_at: undefined,
    recurrence: undefined
});

const parseCustomData = (raw: string) => {
    if (!raw) return {};
    try {
        return JSON.parse(raw);
    } catch {
        return null;
    }
};

const useNotificationData = (apiKey: string) => {
    const [notifications, setNotifications] = useState<Notification[]>([]);
    const [users, setUsers] = useState<User[]>([]);
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [pageSize] = useState(20);
    const [totalNotifications, setTotalNotifications] = useState(0);
    const [filters, setFilters] = useState<{ status?: string; channel?: string; from?: string; to?: string }>({});

    const fetchData = async () => {
        setLoading(true);
        try {
            const [notifsResult, usersResult, templatesResult] = await Promise.all([
                notificationsAPI.list(apiKey, page, pageSize, filters).catch(e => { console.error(e); return { notifications: [] as Notification[], total: 0, page: 1, page_size: pageSize }; }),
                usersAPI.list(apiKey, 1, 100).catch(e => { console.error(e); return { users: [] as User[], total_count: 0, page: 1, page_size: 100 }; }),
                templatesAPI.list(apiKey, 100, 0).catch(e => { console.error(e); return { templates: [] as Template[], total: 0, limit: 100, offset: 0 }; })
            ]);
            setNotifications(notifsResult.notifications || []);
            setTotalNotifications(notifsResult.total || 0);
            setUsers(usersResult.users || []);
            setTemplates(templatesResult.templates || []);
        } catch (error) {
            console.error('Failed to fetch notification data:', error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (apiKey) {
            fetchData();
        }
    }, [apiKey, page, filters]);

    return { notifications, users, templates, loading, refresh: fetchData, page, setPage, pageSize, totalNotifications, filters, setFilters };
};

const AppNotifications: React.FC<AppNotificationsProps> = ({ apiKey, webhooks, onUnreadCount }) => {
    const { notifications, users, templates, loading, refresh, page, setPage, pageSize, totalNotifications, filters, setFilters } = useNotificationData(apiKey);
    const [showSendForm, setShowSendForm] = useState(false);

    // Fetch unread count and report to parent
    useEffect(() => {
        if (!apiKey || !onUnreadCount) return;
        notificationsAPI.getUnreadCount(apiKey, '').then(res => {
            onUnreadCount(res.count || 0);
        }).catch(() => { /* ignore — endpoint may not support app-wide counts */ });
    }, [apiKey, notifications]); // re-check after list refreshes
    const [formData, setFormData] = useState<NotificationRequest>(createEmptyForm());
    const [selectedTargets, setSelectedTargets] = useState<string[]>([]);
    const [selectedUsers, setSelectedUsers] = useState<string[]>([]);
    const [dataInput, setDataInput] = useState('');
    const [confirmingBroadcast, setConfirmingBroadcast] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);

    // Inbox selection & actions state
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
    const [detailNotif, setDetailNotif] = useState<Notification | null>(null);
    const [bulkActing, setBulkActing] = useState(false);

    const toggleSelect = (id: string) => {
        setSelectedIds(prev => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id); else next.add(id);
            return next;
        });
    };

    const toggleSelectAll = () => {
        if (selectedIds.size === notifications.length) {
            setSelectedIds(new Set());
        } else {
            setSelectedIds(new Set(notifications.map(n => n.notification_id)));
        }
    };

    const handleBulkMarkRead = async () => {
        if (selectedIds.size === 0) return;
        setBulkActing(true);
        try {
            await notificationsAPI.markRead(apiKey, { notification_ids: Array.from(selectedIds) });
            toast.success(`${selectedIds.size} notification(s) marked as read`);
            setSelectedIds(new Set());
            refresh();
        } catch { toast.error('Failed to mark as read'); } finally { setBulkActing(false); }
    };

    const handleBulkArchive = async () => {
        if (selectedIds.size === 0) return;
        setBulkActing(true);
        try {
            await notificationsAPI.bulkArchive(apiKey, { notification_ids: Array.from(selectedIds) });
            toast.success(`${selectedIds.size} notification(s) archived`);
            setSelectedIds(new Set());
            refresh();
        } catch { toast.error('Failed to archive'); } finally { setBulkActing(false); }
    };

    const handleSnooze = async (notifId: string, hours: number) => {
        const until = new Date(Date.now() + hours * 3600_000).toISOString();
        try {
            await notificationsAPI.snooze(apiKey, notifId, { until });
            toast.success(`Snoozed for ${hours}h`);
            refresh();
        } catch { toast.error('Failed to snooze'); }
    };

    const handleUnsnooze = async (notifId: string) => {
        try {
            await notificationsAPI.unsnooze(apiKey, notifId);
            toast.success('Notification unsnoozed');
            refresh();
        } catch { toast.error('Failed to unsnooze'); }
    };

    const handleMarkAllRead = async () => {
        // Mark all read requires a user_id. Use the first selected user or prompt.
        const firstUserId = notifications[0]?.user_id;
        if (!firstUserId) {
            toast.error('No notifications with a user to mark as read');
            return;
        }
        setBulkActing(true);
        try {
            await notificationsAPI.markAllRead(apiKey, { user_id: firstUserId });
            toast.success('All notifications marked as read');
            setSelectedIds(new Set());
            refresh();
        } catch { toast.error('Failed to mark all as read'); }
        finally { setBulkActing(false); }
    };

    // Batch send/cancel state
    const [showBatchSend, setShowBatchSend] = useState(false);
    const [batchJson, setBatchJson] = useState('{\n  "notifications": [\n    { "user_id": "", "template_id": "", "channel": "email", "title": "", "body": "" }\n  ]\n}');
    const [batchSending, setBatchSending] = useState(false);
    const [showCancelBatch, setShowCancelBatch] = useState(false);
    const [cancelBatchId, setCancelBatchId] = useState('');
    const [cancellingBatch, setCancellingBatch] = useState(false);

    const handleBatchSend = async () => {
        let parsed;
        try { parsed = JSON.parse(batchJson); } catch { toast.error('Invalid JSON'); return; }
        if (!parsed.notifications || !Array.isArray(parsed.notifications)) { toast.error('JSON must have a "notifications" array'); return; }
        setBatchSending(true);
        try {
            await notificationsAPI.sendBatch(apiKey, parsed);
            toast.success(`Batch of ${parsed.notifications.length} notification(s) sent`);
            setShowBatchSend(false);
            setBatchJson('{\n  "notifications": [\n    { "user_id": "", "template_id": "", "channel": "email", "title": "", "body": "" }\n  ]\n}');
            refresh();
        } catch (err: any) { toast.error(err.message || 'Batch send failed'); }
        finally { setBatchSending(false); }
    };

    const handleCancelBatch = async () => {
        if (!cancelBatchId.trim()) { toast.error('Enter a batch ID'); return; }
        setCancellingBatch(true);
        try {
            await notificationsAPI.cancelBatch(apiKey, { batch_id: cancelBatchId.trim() });
            toast.success('Batch cancelled');
            setShowCancelBatch(false);
            setCancelBatchId('');
            refresh();
        } catch (err: any) { toast.error(err.message || 'Failed to cancel batch'); }
        finally { setCancellingBatch(false); }
    };

    // Quick-Send state
    const [quickTo, setQuickTo] = useState('');
    const [quickTemplateId, setQuickTemplateId] = useState('');
    const [quickData, setQuickData] = useState<Record<string, string>>({});
    const [quickSending, setQuickSending] = useState(false);

    // Quick-Send: detect variables from selected template
    const quickSelectedTemplate = useMemo(() => templates.find(t => t.id === quickTemplateId), [templates, quickTemplateId]);
    const quickVariables = quickSelectedTemplate?.variables || [];

    const handleQuickSend = async () => {
        if (!quickTo || !quickTemplateId) return;
        setQuickSending(true);
        try {
            await quickSendAPI.send(apiKey, {
                to: quickTo,
                template: quickSelectedTemplate?.name || quickTemplateId,
                data: Object.keys(quickData).length > 0 ? quickData : undefined,
            });
            toast.success('Notification sent!');
            setQuickTo('');
            setQuickTemplateId('');
            setQuickData({});
            refresh();
        } catch (error: any) {
            const msg = error?.response?.data?.error || 'Quick-send failed';
            toast.error(msg);
        } finally {
            setQuickSending(false);
        }
    };

    const handleBroadcastSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        setConfirmingBroadcast(true);
    };

    const executeBroadcast = async () => {
        setConfirmingBroadcast(false);
        setIsSubmitting(true);

        try {
            const customData = parseCustomData(dataInput);
            if (customData === null) {
                toast.error('Invalid JSON in custom data');
                setIsSubmitting(false);
                return;
            }

            await notificationsAPI.broadcast(apiKey, {
                channel: formData.channel,
                priority: formData.priority,
                title: formData.title,
                body: formData.body,
                template_id: formData.template_id || undefined,
                data: customData,
                scheduled_at: formData.scheduled_at
            });

            setFormData(createEmptyForm());
            setDataInput('');
            refresh();
            toast.success('Broadcast initiated successfully.');
        } catch (error) {
            console.error('Failed to broadcast notification:', error);
            toast.error('Failed to broadcast notification');
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleSendNotification = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            const customData = parseCustomData(dataInput);
            if (customData === null) {
                toast.error('Invalid JSON in custom data');
                return;
            }

            const userIds = selectedUsers.length > 0 ? selectedUsers : (formData.user_id ? [formData.user_id] : []);
            const requiresUser = formData.channel !== 'webhook';
            if (requiresUser && userIds.length === 0) {
                toast.error('Select at least one user.');
                return;
            }

            if (formData.channel === 'webhook' && selectedTargets.length > 0) {
                // Multi-target sending: send separate requests for each target
                const sendPromises = selectedTargets.map(target =>
                    notificationsAPI.send(apiKey, {
                        ...formData,
                        webhook_target: target,
                        data: customData
                    })
                );
                await Promise.all(sendPromises);
            } else if (userIds.length > 1) {
                // Bulk send: use dedicated bulk endpoint for multiple users
                await notificationsAPI.sendBulk(apiKey, {
                    user_ids: userIds,
                    channel: formData.channel,
                    priority: formData.priority,
                    title: formData.title,
                    body: formData.body,
                    template_id: formData.template_id || undefined,
                    data: customData,
                });
            } else {
                // Single send (default behavior)
                await notificationsAPI.send(apiKey, { ...formData, user_id: userIds[0] || formData.user_id, data: customData });
            }

            setShowSendForm(false);
            setFormData(createEmptyForm());
            setSelectedUsers([]);
            setSelectedTargets([]);
            setDataInput('');
            refresh();
            toast.success('Notification(s) sent successfully!');
        } catch (error) {
            console.error('Failed to send notification:', error);
            toast.error('Failed to send notification');
        }
    };

    const getStatusBadgeClass = (status: string) => {
        switch (status?.toLowerCase()) {
            case 'sent': return 'bg-green-100 text-green-700 border-green-300';
            case 'failed': return 'bg-red-100 text-red-700 border-red-300';
            case 'pending': return 'bg-yellow-100 text-yellow-700 border-yellow-300';
            case 'queued': return 'bg-blue-100 text-blue-700 border-blue-300';
            case 'delivered': return 'bg-teal-100 text-teal-700 border-teal-300';
            case 'snoozed': return 'bg-purple-100 text-purple-700 border-purple-300';
            case 'archived': return 'bg-gray-100 text-gray-500 border-gray-300';
            case 'read': return 'bg-sky-100 text-sky-700 border-sky-300';
            case 'dead_letter': return 'bg-red-200 text-red-800 border-red-400';
            default: return 'bg-gray-100 text-gray-700 border-gray-300';
        }
    };

    if (loading) return <div className="flex justify-center py-4">Loading notifications...</div>;

    return (
        <Card>
            <CardHeader>
                <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3">
                    <CardTitle>Notifications</CardTitle>
                    <div className="flex items-center gap-2 flex-wrap">
                        <Button variant="outline" size="sm" onClick={handleMarkAllRead} disabled={bulkActing || notifications.length === 0} title="Mark all notifications as read">
                            <CheckSquare className="h-3.5 w-3.5 mr-1.5" />Mark All Read
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => setShowBatchSend(true)} title="Send a batch of notifications">
                            <Send className="h-3.5 w-3.5 mr-1.5" />Batch Send
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => setShowCancelBatch(true)} title="Cancel a batch by ID">
                            <Ban className="h-3.5 w-3.5 mr-1.5" />Cancel Batch
                        </Button>
                        <Button
                            size="sm"
                            variant={showSendForm ? "outline" : "default"}
                            onClick={() => setShowSendForm(!showSendForm)}
                        >
                            {showSendForm ? 'Hide Send Form' : 'Send Notification'}
                        </Button>
                    </div>
                </div>
            </CardHeader>
            <CardContent>
                {showSendForm && (
                    <Tabs defaultValue="quick" className="mb-8">
                        <TabsList>
                            <TabsTrigger value="quick">Quick Send</TabsTrigger>
                            <TabsTrigger value="advanced">Advanced</TabsTrigger>
                            <TabsTrigger value="broadcast">Broadcast</TabsTrigger>
                        </TabsList>

                        {/* ── Quick Send Tab ── */}
                        <TabsContent value="quick">
                            <div className="bg-muted p-6 rounded border border-border space-y-4">
                                <p className="text-sm text-muted-foreground">Send a notification using email or user ID and a template name. No UUIDs required.</p>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="quickTo">To (email or user ID)</Label>
                                        <Input
                                            id="quickTo"
                                            value={quickTo}
                                            onChange={e => setQuickTo(e.target.value)}
                                            placeholder="john@example.com"
                                        />
                                        <p className="text-xs text-muted-foreground">Enter an email address, external_id, or the internal UUID from Users.</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="quickTemplate">Template</Label>
                                        <Select value={quickTemplateId} onValueChange={setQuickTemplateId}>
                                            <SelectTrigger>
                                                <SelectValue placeholder="Select a template" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {(templates || []).map(t => (
                                                    <SelectItem key={t.id} value={t.id}>{t.name} ({t.channel})</SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    </div>
                                </div>
                                {quickVariables.length > 0 && (
                                    <div className="space-y-3">
                                        <Label className="text-sm font-semibold">Template Variables</Label>
                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                            {quickVariables.map(v => (
                                                <div key={v} className="space-y-1">
                                                    <Label className="text-xs text-muted-foreground">{v}</Label>
                                                    <Input
                                                        value={quickData[v] || ''}
                                                        onChange={e => setQuickData(d => ({ ...d, [v]: e.target.value }))}
                                                        placeholder={v}
                                                    />
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                )}
                                <div className="flex justify-end">
                                    <Button onClick={handleQuickSend} disabled={quickSending || !quickTo || !quickTemplateId}>
                                        {quickSending ? 'Sending...' : 'Send Notification'}
                                    </Button>
                                </div>
                            </div>
                        </TabsContent>

                        {/* ── Advanced Send Tab ── */}
                        <TabsContent value="advanced">
                            <form onSubmit={handleSendNotification} className="bg-muted p-6 rounded border border-border space-y-4">
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="recipient">
                                            Recipients (Users)
                                            {formData.channel === 'webhook' && <span className="font-normal text-muted-foreground text-xs"> (Optional)</span>}
                                        </Label>
                                        <UserMultiSelect
                                            users={users}
                                            value={selectedUsers}
                                            onChange={setSelectedUsers}
                                        />
                                        <p className="text-xs text-muted-foreground">Select notification recipients. These are users created in the Users tab.</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="template">Template (Optional)</Label>
                                        <Select
                                            value={formData.template_id || ''}
                                            onValueChange={(value) => setFormData({ ...formData, template_id: value })}
                                        >
                                            <SelectTrigger>
                                                <SelectValue placeholder="No template (use manual content)" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {(templates || []).map(t => (
                                                    <SelectItem key={t.id} value={t.id}>{t.name} ({t.channel})</SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                        <p className="text-xs text-muted-foreground">If selected, notification content is rendered from this template. Otherwise, provide title and body manually.</p>
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="channel">Channel</Label>
                                        <Select
                                            value={formData.channel}
                                            onValueChange={(value) => {
                                                const next = value as any;
                                                setFormData({
                                                    ...formData,
                                                    channel: next,
                                                    webhook_url: next === 'webhook' ? formData.webhook_url : ''
                                                });
                                                if (next !== 'webhook') {
                                                    setSelectedTargets([]);
                                                }
                                            }}
                                        >
                                            <SelectTrigger>
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="email">Email</SelectItem>
                                                <SelectItem value="push">Push</SelectItem>
                                                <SelectItem value="sms">SMS</SelectItem>
                                                <SelectItem value="webhook">Webhook</SelectItem>
                                                <SelectItem value="in_app">In-App</SelectItem>
                                                <SelectItem value="sse">SSE (Server-Sent Events)</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>

                                    {/* Webhook Targets Selection - Multi-select */}
                                    {formData.channel === 'webhook' && webhooks && Object.keys(webhooks).length > 0 && (
                                        <div className="space-y-2 md:col-span-2">
                                            <Label>Webhook Targets (Select one or more)</Label>
                                            <WebhookTargetSelect
                                                targets={Object.keys(webhooks)}
                                                value={selectedTargets}
                                                onChange={setSelectedTargets}
                                            />
                                            <p className="text-xs text-muted-foreground">
                                                If multiple targets are selected, a separate notification will be sent to each.
                                            </p>
                                        </div>
                                    )}

                                    {/* Webhook URL Override Field */}
                                    {formData.channel === 'webhook' && (
                                        <div className="space-y-2 md:col-span-2">
                                            <Label htmlFor="webhookUrl">Webhook URL Override (Optional)</Label>
                                            <Input
                                                id="webhookUrl"
                                                type="url"
                                                value={formData.webhook_url || ''}
                                                onChange={(e) => setFormData({ ...formData, webhook_url: e.target.value })}
                                                placeholder="https://discord.com/api/webhooks/..."
                                                required={selectedUsers.length === 0 && selectedTargets.length === 0}
                                            />
                                            <p className="text-xs text-muted-foreground">
                                                Use this to send to an ad-hoc URL not in the saved list.
                                            </p>
                                        </div>
                                    )}
                                    <div className="space-y-2">
                                        <Label htmlFor="priority">Priority</Label>
                                        <Select
                                            value={formData.priority}
                                            onValueChange={(value) => setFormData({ ...formData, priority: value as any })}
                                        >
                                            <SelectTrigger>
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="low">Low</SelectItem>
                                                <SelectItem value="normal">Normal</SelectItem>
                                                <SelectItem value="high">High</SelectItem>
                                                <SelectItem value="critical">Critical</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="title">Title</Label>
                                    <Input
                                        id="title"
                                        type="text"
                                        value={formData.title}
                                        onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                                        required
                                        placeholder="Notification title"
                                    />
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="body">Body / Manual Content</Label>
                                    <Textarea
                                        id="body"
                                        value={formData.body}
                                        onChange={(e) => setFormData({ ...formData, body: e.target.value })}
                                        required={!formData.template_id}
                                        placeholder={formData.template_id ? "Optional (overridden by template)" : "Visible unless overridden by template"}
                                    />
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="customData">Custom Data (JSON)</Label>
                                    <Textarea
                                        id="customData"
                                        className="font-mono"
                                        value={dataInput}
                                        onChange={(e) => setDataInput(e.target.value)}
                                        placeholder='{ "name": "John Doe" }'
                                    />
                                    <p className="text-xs text-muted-foreground">Variables for your template — e.g. {'{"user_name": "Alice"}'} maps to {'{{user_name}}'} in the template body.</p>
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="scheduledAt">Schedule For</Label>
                                    <Input
                                        id="scheduledAt"
                                        type="datetime-local"
                                        value={formData.scheduled_at?.substring(0, 16) || ''}
                                        onChange={(e) => {
                                            const val = e.target.value;
                                            setFormData({ ...formData, scheduled_at: val ? new Date(val).toISOString() : undefined });
                                        }}
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        Leave empty for immediate delivery.
                                    </p>
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="recurrenceCron">Recurrence Cron (e.g. 0 0 * * *)</Label>
                                    <Input
                                        id="recurrenceCron"
                                        type="text"
                                        value={formData.recurrence?.cron_expression || ''}
                                        onChange={(e) => setFormData({
                                            ...formData,
                                            recurrence: e.target.value ? {
                                                ...formData.recurrence || { cron_expression: '' },
                                                cron_expression: e.target.value
                                            } : undefined
                                        })}
                                        placeholder="Cron expression"
                                    />
                                </div>

                                {formData.recurrence && (
                                    <>
                                        <div className="space-y-2">
                                            <Label htmlFor="recurrenceEnd">Ends At (Optional)</Label>
                                            <Input
                                                id="recurrenceEnd"
                                                type="datetime-local"
                                                value={formData.recurrence.end_date?.substring(0, 16) || ''}
                                                onChange={(e) => {
                                                    const val = e.target.value;
                                                    setFormData({
                                                        ...formData,
                                                        recurrence: {
                                                            ...formData.recurrence!,
                                                            end_date: val ? new Date(val).toISOString() : undefined
                                                        }
                                                    });
                                                }}
                                            />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="recurrenceCount">Max Occurrences (Optional)</Label>
                                            <Input
                                                id="recurrenceCount"
                                                type="number"
                                                value={formData.recurrence.count || ''}
                                                onChange={(e) => setFormData({
                                                    ...formData,
                                                    recurrence: {
                                                        ...formData.recurrence!,
                                                        count: parseInt(e.target.value) || undefined
                                                    }
                                                })}
                                                placeholder="e.g. 10"
                                            />
                                        </div>
                                    </>
                                )}
                                <div className="flex justify-end mt-6">
                                    <div className="flex gap-2">
                                        <Button
                                            type="button"
                                            variant="outline"
                                            onClick={() => {
                                                setFormData(createEmptyForm());
                                                setSelectedUsers([]);
                                                setSelectedTargets([]);
                                                setDataInput('');
                                            }}
                                        >
                                            Reset
                                        </Button>
                                        <Button type="submit">Send / Schedule Notification</Button>
                                    </div>
                                </div>
                            </form>
                        </TabsContent>

                        {/* ── Broadcast Tab ── */}
                        <TabsContent value="broadcast">
                            <Card className="border-orange-200 bg-orange-50/30">
                                <CardHeader>
                                    <CardTitle className="text-orange-800 text-lg">Broadcast to All Users</CardTitle>
                                    <p className="text-sm text-orange-600/80 mt-1">This will send a notification to ALL users of this application.</p>
                                </CardHeader>
                                <CardContent>
                                    <form onSubmit={handleBroadcastSubmit} className="space-y-4">
                                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                                            <div className="space-y-2">
                                                <Label htmlFor="broadcastTemplate">Template (Optional)</Label>
                                                <Select
                                                    value={formData.template_id || 'none'}
                                                    onValueChange={(val) => setFormData({ ...formData, template_id: val === 'none' ? undefined : val })}
                                                >
                                                    <SelectTrigger id="broadcastTemplate">
                                                        <SelectValue placeholder="No template" />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="none">No template (manual content)</SelectItem>
                                                        {(templates || []).map(t => (
                                                            <SelectItem key={t.id} value={t.id}>{t.name} ({t.channel})</SelectItem>
                                                        ))}
                                                    </SelectContent>
                                                </Select>
                                            </div>
                                            <div className="space-y-2">
                                                <Label htmlFor="broadcastChannel">Channel</Label>
                                                <Select
                                                    value={formData.channel}
                                                    onValueChange={(val) => {
                                                        const next = val as any;
                                                        setFormData({
                                                            ...formData,
                                                            channel: next,
                                                            webhook_url: next === 'webhook' ? formData.webhook_url : ''
                                                        });
                                                        if (next !== 'webhook') {
                                                            setSelectedTargets([]);
                                                        }
                                                    }}
                                                >
                                                    <SelectTrigger id="broadcastChannel">
                                                        <SelectValue />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="email">Email</SelectItem>
                                                        <SelectItem value="push">Push</SelectItem>
                                                        <SelectItem value="sms">SMS</SelectItem>
                                                        <SelectItem value="in_app">In-App</SelectItem>
                                                        <SelectItem value="sse">SSE</SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </div>
                                            <div className="space-y-2">
                                                <Label htmlFor="broadcastPriority">Priority</Label>
                                                <Select
                                                    value={formData.priority}
                                                    onValueChange={(val) => setFormData({ ...formData, priority: val as any })}
                                                >
                                                    <SelectTrigger id="broadcastPriority">
                                                        <SelectValue />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="low">Low</SelectItem>
                                                        <SelectItem value="normal">Normal</SelectItem>
                                                        <SelectItem value="high">High</SelectItem>
                                                        <SelectItem value="critical">Critical</SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </div>
                                        </div>

                                        <div className="space-y-2">
                                            <Label htmlFor="broadcastTitle">Title</Label>
                                            <Input
                                                id="broadcastTitle"
                                                value={formData.title}
                                                onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                                                required
                                                placeholder="Broadcast title"
                                            />
                                        </div>

                                        <div className="space-y-2">
                                            <Label htmlFor="broadcastBody">Body / Manual Content</Label>
                                            <Textarea
                                                id="broadcastBody"
                                                value={formData.body}
                                                onChange={(e) => setFormData({ ...formData, body: e.target.value })}
                                                required={!formData.template_id}
                                                className="min-h-[100px]"
                                                placeholder={formData.template_id ? "Optional (overridden by template)" : "Content"}
                                            />
                                        </div>

                                        <div className="space-y-2">
                                            <Label htmlFor="broadcastData">Custom Data (JSON)</Label>
                                            <Textarea
                                                id="broadcastData"
                                                className="font-mono text-xs"
                                                value={dataInput}
                                                onChange={(e) => setDataInput(e.target.value)}
                                                placeholder='{ "key": "value" }'
                                            />
                                        </div>

                                        <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4 pt-2">
                                            <div className="space-y-1">
                                                <Label htmlFor="broadcastScheduled">Scheduled Time (Optional)</Label>
                                                <Input
                                                    id="broadcastScheduled"
                                                    type="datetime-local"
                                                    className="w-auto text-sm"
                                                    value={formData.scheduled_at?.substring(0, 16) || ''}
                                                    onChange={(e) => {
                                                        const val = e.target.value;
                                                        setFormData({ ...formData, scheduled_at: val ? new Date(val).toISOString() : undefined });
                                                    }}
                                                />
                                            </div>
                                            <Button
                                                type="submit"
                                                className="bg-orange-600 hover:bg-orange-700 text-white"
                                                disabled={isSubmitting}
                                            >
                                                {isSubmitting ? 'Processing...' : '🚀 Send Broadcast'}
                                            </Button>
                                        </div>

                                        {confirmingBroadcast && (
                                            <div className="mt-4 p-4 border-2 border-orange-400 bg-card rounded-lg shadow-sm text-center animate-in fade-in zoom-in duration-200">
                                                <h4 className="font-bold text-orange-800 mb-2">⚠️ Confirm Broadcast</h4>
                                                <p className="text-sm text-foreground mb-4">Are you sure you want to send this to ALL users?<br />This action cannot be undone.</p>
                                                <div className="flex justify-center gap-3">
                                                    <Button
                                                        variant="outline"
                                                        onClick={() => setConfirmingBroadcast(false)}
                                                        type="button"
                                                        disabled={isSubmitting}
                                                    >
                                                        Cancel
                                                    </Button>
                                                    <Button
                                                        className="bg-orange-600 hover:bg-orange-700"
                                                        onClick={executeBroadcast}
                                                        type="button"
                                                        disabled={isSubmitting}
                                                    >
                                                        {isSubmitting ? 'Sending...' : 'Yes, Broadcast'}
                                                    </Button>
                                                </div>
                                            </div>
                                        )}
                                    </form>
                                </CardContent>
                            </Card>
                        </TabsContent>
                    </Tabs>
                )}

                <h3 className="text-lg font-semibold mt-6 mb-3">Notification History</h3>

                {/* Filter Bar */}
                <div className="flex flex-wrap gap-3 items-end mb-4 p-3 bg-muted rounded border border-border">
                    <div className="space-y-1">
                        <Label className="text-xs">Status</Label>
                        <Select value={filters.status || ''} onValueChange={v => { setFilters({ ...filters, status: v || undefined }); setPage(1); }}>
                            <SelectTrigger className="w-[130px] h-8 text-xs">
                                <SelectValue placeholder="All" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">All</SelectItem>
                                <SelectItem value="pending">Pending</SelectItem>
                                <SelectItem value="queued">Queued</SelectItem>
                                <SelectItem value="processing">Processing</SelectItem>
                                <SelectItem value="sent">Sent</SelectItem>
                                <SelectItem value="delivered">Delivered</SelectItem>
                                <SelectItem value="failed">Failed</SelectItem>
                                <SelectItem value="read">Read</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-1">
                        <Label className="text-xs">Channel</Label>
                        <Select value={filters.channel || ''} onValueChange={v => { setFilters({ ...filters, channel: v || undefined }); setPage(1); }}>
                            <SelectTrigger className="w-[130px] h-8 text-xs">
                                <SelectValue placeholder="All" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">All</SelectItem>
                                <SelectItem value="email">Email</SelectItem>
                                <SelectItem value="push">Push</SelectItem>
                                <SelectItem value="sms">SMS</SelectItem>
                                <SelectItem value="webhook">Webhook</SelectItem>
                                <SelectItem value="in_app">In-App</SelectItem>
                                <SelectItem value="sse">SSE</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-1">
                        <Label className="text-xs">From</Label>
                        <Input type="date" className="w-[140px] h-8 text-xs" value={filters.from || ''} onChange={e => { setFilters({ ...filters, from: e.target.value || undefined }); setPage(1); }} />
                    </div>
                    <div className="space-y-1">
                        <Label className="text-xs">To</Label>
                        <Input type="date" className="w-[140px] h-8 text-xs" value={filters.to || ''} onChange={e => { setFilters({ ...filters, to: e.target.value || undefined }); setPage(1); }} />
                    </div>
                    {(filters.status || filters.channel || filters.from || filters.to) && (
                        <Button variant="ghost" size="sm" className="h-8 text-xs" onClick={() => { setFilters({}); setPage(1); }}>
                            Clear Filters
                        </Button>
                    )}
                </div>

                {
                    !notifications || notifications.length === 0 ? (
                        <p className="text-muted-foreground text-center py-8 text-sm">No notification history found.</p>
                    ) : (
                        <>
                            {/* Bulk Actions Bar */}
                            {selectedIds.size > 0 && (
                                <div className="flex items-center gap-2 px-3 py-2 mb-2 bg-muted/60 border border-border rounded-lg">
                                    <span className="text-sm font-medium text-foreground mr-2">{selectedIds.size} selected</span>
                                    <Button variant="outline" size="sm" onClick={handleBulkMarkRead} disabled={bulkActing}>
                                        <CheckSquare className="h-3.5 w-3.5 mr-1.5" />
                                        Mark Read
                                    </Button>
                                    <Button variant="outline" size="sm" onClick={handleBulkArchive} disabled={bulkActing}>
                                        <Archive className="h-3.5 w-3.5 mr-1.5" />
                                        Archive
                                    </Button>
                                    <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
                                        <X className="h-3.5 w-3.5 mr-1" />
                                        Clear
                                    </Button>
                                </div>
                            )}

                            <div className="overflow-x-auto">
                                <Table>
                                    <TableHeader>
                                        <TableRow>
                                            <TableHead className="w-10">
                                                <Checkbox
                                                    checked={notifications.length > 0 && selectedIds.size === notifications.length}
                                                    onCheckedChange={toggleSelectAll}
                                                />
                                            </TableHead>
                                            <TableHead className="hidden md:table-cell">ID</TableHead>
                                            <TableHead className="hidden lg:table-cell">User</TableHead>
                                            <TableHead>Title</TableHead>
                                            <TableHead>Status</TableHead>
                                            <TableHead className="hidden md:table-cell">Scheduled At</TableHead>
                                            <TableHead className="hidden md:table-cell">Sent At</TableHead>
                                            <TableHead className="w-10" />
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        {notifications.map((n) => (
                                            <TableRow
                                                key={n.notification_id}
                                                className="cursor-pointer hover:bg-muted/50"
                                                onClick={() => setDetailNotif(n)}
                                            >
                                                <TableCell onClick={e => e.stopPropagation()}>
                                                    <Checkbox
                                                        checked={selectedIds.has(n.notification_id)}
                                                        onCheckedChange={() => toggleSelect(n.notification_id)}
                                                    />
                                                </TableCell>
                                                <TableCell className="hidden md:table-cell text-xs text-muted-foreground font-mono">{n.notification_id?.substring(0, 8)}...</TableCell>
                                                <TableCell className="hidden lg:table-cell text-foreground">
                                                    {n.user_id ?
                                                        (users?.find(u => u.user_id === n.user_id)?.email || n.user_id) :
                                                        <span className="text-muted-foreground italic">Anonymous (Webhook)</span>
                                                    }
                                                </TableCell>
                                                <TableCell className="text-foreground">{n.content?.title || '-'}</TableCell>
                                                <TableCell>
                                                    <Badge
                                                        variant="outline"
                                                        className={`text-xs uppercase ${getStatusBadgeClass(n.status)}`}
                                                    >
                                                        {n.status}
                                                    </Badge>
                                                </TableCell>
                                                <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                                                    {n.scheduled_at ? (
                                                        <div className="flex flex-col">
                                                            <span>{new Date(n.scheduled_at).toLocaleString()}</span>
                                                            {n.recurrence && (
                                                                <span className="text-[10px] text-foreground mt-0.5">
                                                                    🔄 {n.recurrence.cron_expression}
                                                                </span>
                                                            )}
                                                        </div>
                                                    ) : (
                                                        <span className="text-muted-foreground">Now</span>
                                                    )}
                                                </TableCell>
                                                <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                                                    {n.created_at ? new Date(n.created_at).toLocaleString() : '-'}
                                                </TableCell>
                                                <TableCell onClick={e => e.stopPropagation()}>
                                                    {n.status === 'snoozed' ? (
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            title="Unsnooze"
                                                            onClick={() => handleUnsnooze(n.notification_id)}
                                                        >
                                                            <Bell className="h-3.5 w-3.5 text-purple-600" />
                                                        </Button>
                                                    ) : (
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            title="Snooze 1h"
                                                            onClick={() => handleSnooze(n.notification_id, 1)}
                                                        >
                                                            <BellOff className="h-3.5 w-3.5" />
                                                        </Button>
                                                    )}
                                                </TableCell>
                                            </TableRow>
                                        ))}
                                    </TableBody>
                                </Table>
                            </div>
                        </>
                    )
                }

                <Pagination
                    currentPage={page}
                    totalItems={totalNotifications}
                    pageSize={pageSize}
                    onPageChange={setPage}
                />

                {/* Notification Detail Panel */}
                <SlidePanel
                    open={!!detailNotif}
                    onClose={() => setDetailNotif(null)}
                    title="Notification Detail"
                >
                    {detailNotif && (
                        <div className="space-y-4">
                            <div>
                                <Label className="text-xs text-muted-foreground">ID</Label>
                                <p className="font-mono text-sm">{detailNotif.notification_id}</p>
                            </div>
                            <div>
                                <Label className="text-xs text-muted-foreground">Title</Label>
                                <p className="text-sm font-medium">{detailNotif.content?.title || '-'}</p>
                            </div>
                            <div>
                                <Label className="text-xs text-muted-foreground">Body</Label>
                                <p className="text-sm text-foreground whitespace-pre-wrap">{detailNotif.content?.body || '-'}</p>
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <div>
                                    <Label className="text-xs text-muted-foreground">Channel</Label>
                                    <p className="text-sm">{detailNotif.channel}</p>
                                </div>
                                <div>
                                    <Label className="text-xs text-muted-foreground">Priority</Label>
                                    <p className="text-sm">{detailNotif.priority}</p>
                                </div>
                                <div>
                                    <Label className="text-xs text-muted-foreground">Status</Label>
                                    <Badge variant="outline" className={`text-xs uppercase ${getStatusBadgeClass(detailNotif.status)}`}>
                                        {detailNotif.status}
                                    </Badge>
                                </div>
                                <div>
                                    <Label className="text-xs text-muted-foreground">Created</Label>
                                    <p className="text-sm">{detailNotif.created_at ? timeAgo(detailNotif.created_at) : '-'}</p>
                                </div>
                            </div>
                            {detailNotif.status === 'snoozed' && (detailNotif as any).snoozed_until && (
                                <div>
                                    <Label className="text-xs text-muted-foreground">Snoozed Until</Label>
                                    <p className="text-sm text-purple-700">{new Date((detailNotif as any).snoozed_until).toLocaleString()}</p>
                                </div>
                            )}
                            {detailNotif.user_id && (
                                <div>
                                    <Label className="text-xs text-muted-foreground">User</Label>
                                    <p className="text-sm font-mono">
                                        {users?.find(u => u.user_id === detailNotif.user_id)?.email || detailNotif.user_id}
                                    </p>
                                </div>
                            )}
                            {detailNotif.template_id && (
                                <div>
                                    <Label className="text-xs text-muted-foreground">Template</Label>
                                    <p className="text-sm font-mono">{detailNotif.template_id}</p>
                                </div>
                            )}
                            {detailNotif.content?.data && Object.keys(detailNotif.content.data).length > 0 && (
                                <div>
                                    <Label className="text-xs text-muted-foreground">Custom Data</Label>
                                    <pre className="text-xs bg-muted p-3 rounded-md overflow-auto max-h-40 font-mono mt-1">
                                        {JSON.stringify(detailNotif.content.data, null, 2)}
                                    </pre>
                                </div>
                            )}
                            <div className="flex items-center gap-2 pt-2 border-t border-border flex-wrap">
                                <Button variant="outline" size="sm"
                                    onClick={() => {
                                        notificationsAPI.markRead(apiKey, { notification_ids: [detailNotif.notification_id] })
                                            .then(() => { toast.success('Marked as read'); refresh(); })
                                            .catch(() => toast.error('Failed'));
                                    }}
                                >
                                    <Eye className="h-3.5 w-3.5 mr-1.5" />Mark Read
                                </Button>
                                {detailNotif.status === 'snoozed' ? (
                                    <Button variant="outline" size="sm"
                                        onClick={() => handleUnsnooze(detailNotif.notification_id)}
                                    >
                                        <Bell className="h-3.5 w-3.5 mr-1.5" />Unsnooze
                                    </Button>
                                ) : (
                                    <>
                                        <Button variant="outline" size="sm"
                                            onClick={() => handleSnooze(detailNotif.notification_id, 1)}
                                        >
                                            <BellOff className="h-3.5 w-3.5 mr-1.5" />Snooze 1h
                                        </Button>
                                        <Button variant="outline" size="sm"
                                            onClick={() => handleSnooze(detailNotif.notification_id, 24)}
                                        >
                                            <BellOff className="h-3.5 w-3.5 mr-1.5" />Snooze 24h
                                        </Button>
                                    </>
                                )}
                                <Button variant="outline" size="sm"
                                    onClick={() => {
                                        notificationsAPI.bulkArchive(apiKey, { notification_ids: [detailNotif.notification_id] })
                                            .then(() => { toast.success('Archived'); setDetailNotif(null); refresh(); })
                                            .catch(() => toast.error('Failed'));
                                    }}
                                >
                                    <Archive className="h-3.5 w-3.5 mr-1.5" />Archive
                                </Button>
                            </div>
                        </div>
                    )}
                </SlidePanel>

                {/* Batch Send Dialog */}
                <Dialog open={showBatchSend} onOpenChange={setShowBatchSend}>
                    <DialogContent className="max-w-2xl">
                        <DialogHeader>
                            <DialogTitle>Send Batch Notifications</DialogTitle>
                            <DialogDescription>
                                Send multiple notifications in a single request. Provide a JSON object with a "notifications" array.
                            </DialogDescription>
                        </DialogHeader>
                        <div className="space-y-3">
                            <Textarea
                                className="font-mono text-xs min-h-[200px]"
                                value={batchJson}
                                onChange={e => setBatchJson(e.target.value)}
                                placeholder='{"notifications": [{"user_id": "", "channel": "email", "title": "", "body": ""}]}'
                            />
                            <p className="text-xs text-muted-foreground">
                                Each notification needs at minimum a <code className="bg-muted px-1 rounded">user_id</code>, <code className="bg-muted px-1 rounded">channel</code>, and either a <code className="bg-muted px-1 rounded">template_id</code> or <code className="bg-muted px-1 rounded">title</code> + <code className="bg-muted px-1 rounded">body</code>.
                            </p>
                            <div className="flex justify-end gap-2">
                                <Button variant="outline" onClick={() => setShowBatchSend(false)}>Cancel</Button>
                                <Button onClick={handleBatchSend} disabled={batchSending}>
                                    {batchSending ? 'Sending...' : 'Send Batch'}
                                </Button>
                            </div>
                        </div>
                    </DialogContent>
                </Dialog>

                {/* Cancel Batch Dialog */}
                <Dialog open={showCancelBatch} onOpenChange={setShowCancelBatch}>
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>Cancel Batch</DialogTitle>
                            <DialogDescription>
                                Cancel all pending notifications in a batch by its ID.
                            </DialogDescription>
                        </DialogHeader>
                        <div className="space-y-3">
                            <div className="space-y-2">
                                <Label htmlFor="cancelBatchId">Batch ID</Label>
                                <Input
                                    id="cancelBatchId"
                                    value={cancelBatchId}
                                    onChange={e => setCancelBatchId(e.target.value)}
                                    placeholder="Enter the batch ID"
                                />
                                <p className="text-xs text-muted-foreground">
                                    The batch ID returned when you sent a batch notification.
                                </p>
                            </div>
                            <div className="flex justify-end gap-2">
                                <Button variant="outline" onClick={() => setShowCancelBatch(false)}>Cancel</Button>
                                <Button variant="destructive" onClick={handleCancelBatch} disabled={cancellingBatch}>
                                    {cancellingBatch ? 'Cancelling...' : 'Cancel Batch'}
                                </Button>
                            </div>
                        </div>
                    </DialogContent>
                </Dialog>
            </CardContent >
        </Card >
    );
};

export default AppNotifications;


const UserMultiSelect: React.FC<{
    users: User[] | undefined;
    value: string[];
    onChange: (value: string[]) => void;
}> = ({ users, value, onChange }) => {
    const [searchTerm, setSearchTerm] = useState('');
    const [isOpen, setIsOpen] = useState(false);
    const normalizedQuery = searchTerm.trim().toLowerCase();
    const filteredUsers = (users || []).filter(u => {
        if (!normalizedQuery) return true;
        const email = u.email?.toLowerCase() || '';
        const id = u.user_id.toLowerCase();
        return email.includes(normalizedQuery) || id.includes(normalizedQuery);
    });
    const selectedUsers = value.map(id => users?.find(u => u.user_id === id)).filter(Boolean) as User[];
    const toggleUser = (userId: string) => {
        if (value.includes(userId)) {
            onChange(value.filter(id => id !== userId));
            return;
        }
        onChange([...value, userId]);
    };
    const totalUsers = users?.length || 0;
    const selectedCount = value.length;
    const selectorLabel = selectedCount === 0
        ? 'Select users'
        : `${selectedCount} user${selectedCount === 1 ? '' : 's'} selected`;

    return (
        <div className="space-y-2">
            <Dialog open={isOpen} onOpenChange={setIsOpen}>
                <DialogTrigger asChild>
                    <Button variant="outline" type="button" className="w-full justify-between">
                        <span className="truncate text-left">{selectorLabel}</span>
                        <span className="text-xs text-muted-foreground">{selectedCount}/{totalUsers}</span>
                    </Button>
                </DialogTrigger>
                <DialogContent className="max-w-2xl">
                    <DialogHeader>
                        <DialogTitle>Select users</DialogTitle>
                        <DialogDescription>
                            Search by email or user ID. Select one or more recipients.
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-3">
                        <Input
                            type="text"
                            placeholder="Search by email or user ID"
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                        />
                        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                            <span>Selected {selectedCount} of {totalUsers}</span>
                            <div className="flex gap-2">
                                <button
                                    type="button"
                                    className="hover:text-foreground"
                                    onClick={() => onChange(filteredUsers.map(u => u.user_id))}
                                >
                                    Select all (filtered)
                                </button>
                                <button
                                    type="button"
                                    className="hover:text-foreground"
                                    onClick={() => onChange([])}
                                >
                                    Clear
                                </button>
                            </div>
                        </div>
                        <div className="max-h-72 overflow-y-auto rounded border border-border">
                            {filteredUsers.length === 0 ? (
                                <p className="text-muted-foreground text-sm p-3">No users found.</p>
                            ) : (
                                <div className="divide-y divide-border">
                                    {filteredUsers.map(u => (
                                        <div key={u.user_id} className="flex items-center justify-between px-3 py-2">
                                            <div className="min-w-0">
                                                <div className="font-medium text-foreground truncate">{u.email || 'No email'}</div>
                                                <div className="text-xs text-muted-foreground truncate">{u.user_id}</div>
                                            </div>
                                            <Checkbox
                                                checked={value.includes(u.user_id)}
                                                onCheckedChange={() => toggleUser(u.user_id)}
                                            />
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                        {selectedUsers.length > 0 && (
                            <div className="flex flex-wrap gap-2 pt-1">
                                {selectedUsers.map(user => (
                                    <span key={user.user_id} className="inline-flex items-center gap-2 rounded-full bg-muted px-3 py-1 text-xs text-foreground">
                                        {user.email || user.user_id}
                                        <button
                                            type="button"
                                            className="text-muted-foreground hover:text-foreground"
                                            onClick={() => toggleUser(user.user_id)}
                                        >
                                            Remove
                                        </button>
                                    </span>
                                ))}
                            </div>
                        )}
                    </div>
                </DialogContent>
            </Dialog>
        </div>
    );
}

const WebhookTargetSelect: React.FC<{
    targets: string[];
    value: string[];
    onChange: (value: string[]) => void;
}> = ({ targets, value, onChange }) => {
    // const toggleTarget = (name: string) => {
    //     if (value.includes(name)) {
    //         onChange(value.filter(t => t !== name));
    //         return;
    //     }
    //     onChange([...value, name]);
    // };

    return (
        <div className="space-y-2">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>Selected {value.length} of {targets.length}</span>
                <div className="flex gap-2">
                    <button type="button" className="hover:text-foreground" onClick={() => onChange(targets)}>Select all</button>
                    <button type="button" className="hover:text-foreground" onClick={() => onChange([])}>Clear</button>
                </div>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-2 p-3 border border-border rounded bg-card">
                {targets.map(name => (
                    <div key={name} className="flex items-center space-x-2">
                        <Checkbox
                            id={`webhook-${name}`}
                            checked={value.includes(name)}
                            onCheckedChange={(checked) => {
                                if (checked) {
                                    onChange([...value, name]);
                                } else {
                                    onChange(value.filter(t => t !== name));
                                }
                            }}
                        />
                        <Label htmlFor={`webhook-${name}`} className="text-sm cursor-pointer">{name}</Label>
                    </div>
                ))}
            </div>
        </div>
    );
};
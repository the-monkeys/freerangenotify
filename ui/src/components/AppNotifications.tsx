import React, { useEffect, useState, useMemo } from 'react';
import { notificationsAPI, usersAPI, templatesAPI, quickSendAPI, workflowsAPI, topicsAPI, digestRulesAPI } from '../services/api';
import type { Notification, NotificationRequest, User, Template, Workflow, Topic, BroadcastNotificationRequest, DigestRule } from '../types';
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
import { CheckSquare, Archive, BellOff, Bell, Eye, X, Send, Ban, ChevronDown, ChevronUp, Clock, Layers, XCircle } from 'lucide-react';
import { toast } from 'sonner';
import { extractErrorMessage } from '../lib/utils';
import { TimezonePicker } from './TimezonePicker';
import { localInTimezoneToISO, formatInTimezone } from '../lib/timezone';

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
    template_id: '',
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

// Format YYYY-MM-DD → "10 Feb 2026" for template display
function formatDateForTemplate(isoDate: string): string {
    if (!isoDate || isoDate.length < 10) return isoDate;
    const d = new Date(isoDate.slice(0, 10));
    return isNaN(d.getTime()) ? isoDate : d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
}
// Parse "10 Feb 2026" or similar → YYYY-MM-DD for input type="date"
function toISODateOnly(val: string): string {
    if (!val) return '';
    if (/^\d{4}-\d{2}-\d{2}$/.test(val)) return val;
    const d = new Date(val);
    return isNaN(d.getTime()) ? '' : d.toISOString().slice(0, 10);
}

// Convert datetime-local value (YYYY-MM-DDTHH:mm) in given timezone to ISO UTC string
function scheduleToISO(datetimeLocal: string, timezone: string): string | undefined {
    return localInTimezoneToISO(datetimeLocal, timezone);
}

const useNotificationData = (apiKey: string) => {
    const [notifications, setNotifications] = useState<Notification[]>([]);
    const [users, setUsers] = useState<User[]>([]);
    const [templates, setTemplates] = useState<Template[]>([]);
    const [workflows, setWorkflows] = useState<Workflow[]>([]);
    const [topics, setTopics] = useState<Topic[]>([]);
    const [digestRules, setDigestRules] = useState<DigestRule[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const [pageSize] = useState(20);
    const [totalNotifications, setTotalNotifications] = useState(0);
    const [filters, setFilters] = useState<{ status?: string; channel?: string; from?: string; to?: string }>({});

    const fetchData = async () => {
        setLoading(true);
        try {
            const [notifsResult, usersResult, templatesResult, workflowsResult, topicsResult, digestResult] = await Promise.all([
                notificationsAPI.list(apiKey, page, pageSize, filters).catch(e => { console.error(e); return { notifications: [] as Notification[], total: 0, page: 1, page_size: pageSize }; }),
                usersAPI.list(apiKey, 1, 100).catch(e => { console.error(e); return { users: [] as User[], total_count: 0, page: 1, page_size: 100 }; }),
                templatesAPI.list(apiKey, 100, 0).catch(e => { console.error(e); return { templates: [] as Template[], total: 0, limit: 100, offset: 0 }; }),
                workflowsAPI.list(apiKey, 100, 0).catch(e => { console.error(e); return { workflows: [] as Workflow[], total: 0, limit: 100, offset: 0 }; }),
                topicsAPI.list(apiKey, 100, 0).catch(e => { console.error(e); return { topics: [] as Topic[], total: 0, limit: 100, offset: 0 }; }),
                digestRulesAPI.list(apiKey, 100, 0).catch(e => { console.error(e); return { rules: [] as DigestRule[], total: 0 }; })
            ]);
            setNotifications(notifsResult.notifications || []);
            setTotalNotifications(notifsResult.total || 0);
            setUsers(usersResult.users || []);
            setTemplates(templatesResult.templates || []);
            setWorkflows(workflowsResult.workflows || []);
            setTopics(topicsResult.topics || []);
            setDigestRules(digestResult.rules || []);
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

    return { notifications, users, templates, workflows, topics, digestRules, loading, refresh: fetchData, page, setPage, pageSize, totalNotifications, filters, setFilters };
};

const AppNotifications: React.FC<AppNotificationsProps> = ({ apiKey, webhooks, onUnreadCount }) => {
    const { notifications, users, templates, workflows, topics, digestRules, loading, refresh, page, setPage, pageSize, totalNotifications, filters, setFilters } = useNotificationData(apiKey || '');

    if (!apiKey) {
        return (
            <Card>
                <CardContent className="pt-6">
                    <p className="text-muted-foreground text-sm">API key is required to load notifications. The application may not have loaded correctly.</p>
                </CardContent>
            </Card>
        );
    }
    const [showSendForm, setShowSendForm] = useState(false);

    // Report notification count to parent (app-wide, derived from current list)
    useEffect(() => {
        if (!onUnreadCount) return;
        onUnreadCount(totalNotifications);
    }, [totalNotifications, onUnreadCount]);
    const [formData, setFormData] = useState<NotificationRequest>(createEmptyForm());
    const [selectedTargets, setSelectedTargets] = useState<string[]>([]);
    const [selectedUsers, setSelectedUsers] = useState<string[]>([]);
    const [dataInput, setDataInput] = useState('');
    const [confirmingBroadcast, setConfirmingBroadcast] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [broadcastWorkflowTriggerId, setBroadcastWorkflowTriggerId] = useState('');
    const [broadcastTopicKey, setBroadcastTopicKey] = useState('');

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
        const list = notifications || [];
        if (list.length === 0 || selectedIds.size === list.length) {
            setSelectedIds(new Set());
        } else {
            setSelectedIds(new Set((notifications || []).map(n => n.notification_id)));
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
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to mark as read')); } finally { setBulkActing(false); }
    };

    const handleBulkArchive = async () => {
        if (selectedIds.size === 0) return;
        setBulkActing(true);
        try {
            await notificationsAPI.bulkArchive(apiKey, { notification_ids: Array.from(selectedIds) });
            toast.success(`${selectedIds.size} notification(s) archived`);
            setSelectedIds(new Set());
            refresh();
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to archive')); } finally { setBulkActing(false); }
    };

    const handleSnooze = async (notifId: string, hours: number) => {
        const until = new Date(Date.now() + hours * 3600_000).toISOString();
        try {
            await notificationsAPI.snooze(apiKey, notifId, { until });
            toast.success(`Snoozed for ${hours}h`);
            refresh();
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to snooze')); }
    };

    const handleUnsnooze = async (notifId: string) => {
        try {
            await notificationsAPI.unsnooze(apiKey, notifId);
            toast.success('Notification unsnoozed');
            refresh();
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to unsnooze')); }
    };

    const handleCancel = async (notifId: string) => {
        try {
            await notificationsAPI.cancel(apiKey, notifId);
            toast.success('Notification cancelled');
            setDetailNotif(null);
            refresh();
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to cancel')); }
    };

    const handleCancelSelected = async () => {
        if (selectedIds.size === 0) return;
        const ids = Array.from(selectedIds);
        setBulkActing(true);
        try {
            await notificationsAPI.cancelBatch(apiKey, { notification_ids: ids });
            toast.success(`${ids.length} notification(s) cancelled`);
            setSelectedIds(new Set());
            setDetailNotif(null);
            refresh();
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to cancel')); }
        finally { setBulkActing(false); }
    };

    const handleMarkAllRead = async () => {
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
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to mark all as read')); }
        finally { setBulkActing(false); }
    };

    // Batch send/cancel state
    const [showBatchSend, setShowBatchSend] = useState(false);
    const [batchJson, setBatchJson] = useState('{\n  "notifications": [\n    { "user_id": "", "template_id": "", "channel": "email", "title": "", "body": "" }\n  ]\n}');
    const [batchSending, setBatchSending] = useState(false);
    const [showCancelBatch, setShowCancelBatch] = useState(false);
    const [cancelBatchIds, setCancelBatchIds] = useState('');
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
        } catch (err) { toast.error(extractErrorMessage(err, 'Batch send failed')); }
        finally { setBatchSending(false); }
    };

    const handleCancelBatch = async () => {
        const ids = cancelBatchIds.trim().split(/[\s,]+/).filter(Boolean);
        if (ids.length === 0) { toast.error('Enter notification IDs (comma or space separated)'); return; }
        setCancellingBatch(true);
        try {
            await notificationsAPI.cancelBatch(apiKey, { notification_ids: ids });
            toast.success(`${ids.length} notification(s) cancelled`);
            setShowCancelBatch(false);
            setCancelBatchIds('');
            refresh();
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to cancel batch')); }
        finally { setCancellingBatch(false); }
    };

    // Recurrence presets → cron expressions
    const recurrencePresets: { label: string; value: string; cron: string }[] = [
        { label: 'Every hour', value: 'hourly', cron: '0 * * * *' },
        { label: 'Every 6 hours', value: 'every_6h', cron: '0 */6 * * *' },
        { label: 'Every 12 hours', value: 'every_12h', cron: '0 */12 * * *' },
        { label: 'Daily', value: 'daily', cron: '0 9 * * *' },
        { label: 'Weekly', value: 'weekly', cron: '0 9 * * 1' },
        { label: 'Bi-weekly', value: 'biweekly', cron: '0 9 1,15 * *' },
        { label: 'Monthly', value: 'monthly', cron: '0 9 1 * *' },
    ];

    // Schedule toggle states
    const [quickScheduleEnabled, setQuickScheduleEnabled] = useState(false);
    const [advScheduleEnabled, setAdvScheduleEnabled] = useState(false);
    const [broadcastScheduleEnabled, setBroadcastScheduleEnabled] = useState(false);
    const [scheduleTimezone, setScheduleTimezone] = useState(() => Intl.DateTimeFormat().resolvedOptions().timeZone);
    const [advRecurrenceEnabled, setAdvRecurrenceEnabled] = useState(false);
    const [advRecurrencePreset, setAdvRecurrencePreset] = useState('');

    // Quick-Send state
    const [quickTo, setQuickTo] = useState('');
    const [quickTemplateId, setQuickTemplateId] = useState('');
    const [quickData, setQuickData] = useState<Record<string, string>>({});
    const [quickSending, setQuickSending] = useState(false);
    const [quickPriority, setQuickPriority] = useState<string>('normal');
    const [quickScheduledAt, setQuickScheduledAt] = useState<string>('');
    const [quickDigestRuleId, setQuickDigestRuleId] = useState<string>('');
    const [advDigestRuleId, setAdvDigestRuleId] = useState<string>('');
    const [broadcastDigestRuleId, setBroadcastDigestRuleId] = useState<string>('');

    // Filtered templates by channel
    const filteredTemplates = useMemo(() => (templates || []).filter(t => t.channel === formData.channel), [templates, formData.channel]);

    // Quick-Send: detect variables from selected template
    const quickSelectedTemplate = useMemo(() => templates.find(t => t.id === quickTemplateId), [templates, quickTemplateId]);
    const quickVariables = quickSelectedTemplate?.variables || [];

    // Advanced/Broadcast: detect variables from selected template
    const formSelectedTemplate = useMemo(() => templates.find(t => t.id === formData.template_id), [templates, formData.template_id]);
    const formVariables = formSelectedTemplate?.variables || [];

    // Helper: get sample_data from template metadata
    const getSampleData = (template: Template | undefined): Record<string, string> => {
        if (!template?.metadata?.sample_data) return {};
        const sd = template.metadata.sample_data;
        const result: Record<string, string> = {};
        for (const [k, v] of Object.entries(sd)) {
            result[k] = String(v);
        }
        return result;
    };

    // State for collapsible variables section
    const [varsExpanded, setVarsExpanded] = useState(true);

    const handleQuickSend = async () => {
        if (!quickTo || !quickTemplateId) return;
        setQuickSending(true);
        try {
            const selectedDigestRule = quickDigestRuleId ? digestRules.find(r => r.id === quickDigestRuleId) : null;
            await quickSendAPI.send(apiKey, {
                to: quickTo,
                template: quickSelectedTemplate?.name || quickTemplateId,
                data: Object.keys(quickData).length > 0 ? quickData : undefined,
                priority: quickPriority as any,
                scheduled_at: quickScheduledAt ? scheduleToISO(quickScheduledAt, scheduleTimezone) : undefined,
                digest_key: selectedDigestRule?.digest_key,
            });
            toast.success('Notification sent!');
            setQuickTo('');
            setQuickTemplateId('');
            setQuickData({});
            setQuickPriority('normal');
            setQuickScheduledAt('');
            setQuickScheduleEnabled(false);
            setQuickDigestRuleId('');
            refresh();
        } catch (error) {
            toast.error(extractErrorMessage(error, 'Quick-send failed'));
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

            const useWorkflow = !!broadcastWorkflowTriggerId;
            if (!useWorkflow && !formData.template_id) {
                toast.error('Select a template or a workflow to trigger');
                setIsSubmitting(false);
                return;
            }

            const broadcastDigestRule = broadcastDigestRuleId ? digestRules.find(r => r.id === broadcastDigestRuleId) : null;
            const payload: BroadcastNotificationRequest = {
                channel: formData.channel,
                priority: formData.priority,
                template_id: useWorkflow ? undefined : formData.template_id,
                data: customData,
                scheduled_at: formData.scheduled_at,
                workflow_trigger_id: broadcastWorkflowTriggerId || undefined,
                topic_key: broadcastTopicKey || undefined,
                metadata: broadcastDigestRule ? { digest_key: broadcastDigestRule.digest_key } : undefined,
            };

            await notificationsAPI.broadcast(apiKey, payload);

            setFormData(createEmptyForm());
            setDataInput('');
            setBroadcastWorkflowTriggerId('');
            setBroadcastTopicKey('');
            setBroadcastDigestRuleId('');
            refresh();
            toast.success(useWorkflow ? 'Workflows triggered successfully.' : 'Broadcast initiated successfully.');
        } catch (error) {
            console.error('Failed to broadcast notification:', error);
            toast.error(extractErrorMessage(error, 'Failed to broadcast notification'));
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

            const advDigestRule = advDigestRuleId ? digestRules.find(r => r.id === advDigestRuleId) : null;
            const payload: NotificationRequest = {
                user_id: '',
                channel: formData.channel,
                priority: formData.priority,
                template_id: formData.template_id || undefined,
                data: customData,
                scheduled_at: formData.scheduled_at,
                recurrence: formData.recurrence,
                workflow_trigger_id: formData.workflow_trigger_id || undefined,
                metadata: advDigestRule ? { digest_key: advDigestRule.digest_key } : undefined,
            };

            if (formData.channel === 'webhook' && selectedTargets.length > 0) {
                const sendPromises = selectedTargets.map(target =>
                    notificationsAPI.send(apiKey, { ...payload, webhook_target: target })
                );
                await Promise.all(sendPromises);
            } else if (userIds.length > 1) {
                await notificationsAPI.sendBulk(apiKey, {
                    user_ids: userIds,
                    channel: payload.channel,
                    priority: payload.priority,
                    template_id: payload.template_id,
                    data: customData,
                    metadata: payload.metadata,
                });
            } else {
                await notificationsAPI.send(apiKey, { ...payload, user_id: userIds[0] || formData.user_id });
            }

            setShowSendForm(false);
            setFormData(createEmptyForm());
            setSelectedUsers([]);
            setSelectedTargets([]);
            setDataInput('');
            setAdvDigestRuleId('');
            refresh();
            toast.success('Notification(s) sent successfully!');
        } catch (error) {
            console.error('Failed to send notification:', error);
            toast.error(extractErrorMessage(error, 'Failed to send notification'));
        }
    };

    const getStatusBadgeClass = (status: string) => {
        switch (status?.toLowerCase()) {
            case 'sent': return 'bg-green-100 text-green-700 border-green-300';
            case 'failed': return 'bg-red-100 text-red-700 border-red-300';
            case 'pending': return 'bg-yellow-100 text-yellow-700 border-yellow-300';
            case 'queued': return 'bg-blue-100 text-blue-700 border-blue-300';
            case 'digested': return 'bg-cyan-100 text-cyan-700 border-cyan-300';
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
                                        <Select value={quickTemplateId} onValueChange={(value) => {
                                            setQuickTemplateId(value);
                                            // Pre-fill variables with sample_data
                                            const selected = templates.find(t => t.id === value);
                                            const sample = getSampleData(selected);
                                            if (Object.keys(sample).length > 0) {
                                                setQuickData(sample);
                                            } else {
                                                setQuickData({});
                                            }
                                        }}>
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
                                    <div className="rounded-md border border-border bg-background">
                                        <button
                                            type="button"
                                            className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
                                            onClick={() => setVarsExpanded(!varsExpanded)}
                                        >
                                            <span className="flex items-center gap-2">
                                                Template Variables
                                                <Badge variant="secondary" className="text-xs">{quickVariables.length}</Badge>
                                                {quickSelectedTemplate?.metadata?.sample_data && (
                                                    <span className="text-xs text-muted-foreground font-normal">• sample data pre-filled</span>
                                                )}
                                            </span>
                                            {varsExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                                        </button>
                                        {varsExpanded && (
                                            <div className="px-4 pb-4 grid grid-cols-1 md:grid-cols-2 gap-3">
                                                {quickVariables.map(v => {
                                                    const sampleVal = quickSelectedTemplate?.metadata?.sample_data?.[v];
                                                    const isDateVar = v.toLowerCase() === 'date';
                                                    const currentVal = quickData[v] || '';
                                                    return (
                                                        <div key={v} className="space-y-1">
                                                            <Label className="text-xs text-muted-foreground">{v}</Label>
                                                            {isDateVar ? (
                                                                <Input
                                                                    type="date"
                                                                    value={toISODateOnly(currentVal)}
                                                                    onChange={e => setQuickData(d => ({ ...d, [v]: formatDateForTemplate(e.target.value) }))}
                                                                    className={currentVal ? '' : 'text-muted-foreground'}
                                                                />
                                                            ) : (
                                                                <Input
                                                                    value={currentVal}
                                                                    onChange={e => setQuickData(d => ({ ...d, [v]: e.target.value }))}
                                                                    placeholder={sampleVal ? String(sampleVal) : v}
                                                                    className={currentVal ? '' : 'text-muted-foreground'}
                                                                />
                                                            )}
                                                        </div>
                                                    );
                                                })}
                                            </div>
                                        )}
                                    </div>
                                )}
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="quickPriority">Priority</Label>
                                        <Select value={quickPriority} onValueChange={setQuickPriority}>
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

                                {/* Digest rule (batch notifications) */}
                                {digestRules.length > 0 && quickSelectedTemplate && (
                                    <div className="space-y-2">
                                        <Label className="flex items-center gap-1.5">
                                            <Layers className="h-3.5 w-3.5" />
                                            Use digest rule
                                        </Label>
                                        <Select value={quickDigestRuleId || '__none__'} onValueChange={v => setQuickDigestRuleId(v === '__none__' ? '' : v)}>
                                            <SelectTrigger>
                                                <SelectValue placeholder="Send immediately (no batching)" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="__none__">Send immediately (no batching)</SelectItem>
                                                {(digestRules || [])
                                                    .filter(r => r.status === 'active' && r.channel === (quickSelectedTemplate?.channel || 'email'))
                                                    .map(r => (
                                                        <SelectItem key={r.id} value={r.id}>
                                                            {r.name} ({r.digest_key}, {r.window})
                                                        </SelectItem>
                                                    ))}
                                            </SelectContent>
                                        </Select>
                                        <p className="text-xs text-muted-foreground">Batch this notification with others for the same user. One digest rule at a time.</p>
                                    </div>
                                )}

                                {/* Schedule toggle */}
                                <div className="rounded-md border border-border">
                                    <button
                                        type="button"
                                        className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
                                        onClick={() => {
                                            const next = !quickScheduleEnabled;
                                            setQuickScheduleEnabled(next);
                                            if (!next) setQuickScheduledAt('');
                                        }}
                                    >
                                        <span className="flex items-center gap-2">
                                            <Clock className="h-4 w-4" />
                                            Schedule for later
                                        </span>
                                        <div className={`w-9 h-5 rounded-full transition-colors ${quickScheduleEnabled ? 'bg-primary' : 'bg-muted-foreground/30'} relative`}>
                                            <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform ${quickScheduleEnabled ? 'translate-x-4' : 'translate-x-0.5'}`} />
                                        </div>
                                    </button>
                                    {quickScheduleEnabled && (
                                        <div className="px-4 pb-4 pt-1 space-y-2">
                                            <Input
                                                id="quickScheduledAt"
                                                type="datetime-local"
                                                value={quickScheduledAt}
                                                onChange={e => setQuickScheduledAt(e.target.value)}
                                            />
                                            <div className="space-y-1">
                                                <Label htmlFor="quickScheduleTz" className="text-xs">Timezone</Label>
                                                <TimezonePicker
                                                    id="quickScheduleTz"
                                                    value={scheduleTimezone}
                                                    onChange={setScheduleTimezone}
                                                    placeholder="Search timezone..."
                                                />
                                            </div>
                                        </div>
                                    )}
                                </div>

                                <div className="flex justify-end">
                                    <Button onClick={handleQuickSend} disabled={quickSending || !quickTo || !quickTemplateId}>
                                        {quickSending ? 'Sending...' : quickScheduledAt ? 'Schedule Notification' : 'Send Notification'}
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
                                        <Label htmlFor="channel">Channel</Label>
                                        <Select
                                            value={formData.channel}
                                            onValueChange={(value) => {
                                                const next = value as any;
                                                // Clear template if it doesn't match the new channel
                                                const currentTemplate = templates.find(t => t.id === formData.template_id);
                                                const shouldClearTemplate = currentTemplate && currentTemplate.channel !== next;
                                                setFormData({
                                                    ...formData,
                                                    channel: next,
                                                    template_id: shouldClearTemplate ? '' : formData.template_id,
                                                });
                                                if (shouldClearTemplate) setDataInput('');
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
                                    <div className="space-y-2">
                                        <Label htmlFor="template">Template</Label>
                                        <Select
                                            value={formData.template_id || ''}
                                            onValueChange={(value) => {
                                                setFormData({ ...formData, template_id: value });
                                                // Pre-fill variables with sample_data from template metadata
                                                const selected = templates.find(t => t.id === value);
                                                const sample = getSampleData(selected);
                                                if (Object.keys(sample).length > 0) {
                                                    setDataInput(JSON.stringify(sample, null, 2));
                                                } else {
                                                    setDataInput('');
                                                }
                                            }}
                                        >
                                            <SelectTrigger>
                                                <SelectValue placeholder="Select a template" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {filteredTemplates.length === 0 ? (
                                                    <div className="px-2 py-3 text-sm text-muted-foreground text-center">No templates for {formData.channel}</div>
                                                ) : (
                                                    filteredTemplates.map(t => (
                                                        <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                                                    ))
                                                )}
                                            </SelectContent>
                                        </Select>
                                        {filteredTemplates.length === 0 && (
                                            <p className="text-xs text-amber-600">No templates found for the "{formData.channel}" channel. Create one in the Templates tab.</p>
                                        )}
                                    </div>

                                    {/* Webhook Targets Selection */}
                                    {formData.channel === 'webhook' && (
                                        webhooks && Object.keys(webhooks).length > 0 ? (
                                            <div className="space-y-2 md:col-span-2">
                                                <Label>Webhook Endpoints (Select one or more)</Label>
                                                <WebhookTargetSelect
                                                    targets={Object.keys(webhooks)}
                                                    value={selectedTargets}
                                                    onChange={setSelectedTargets}
                                                />
                                                <p className="text-xs text-muted-foreground">
                                                    If multiple endpoints are selected, a separate notification will be sent to each.
                                                </p>
                                            </div>
                                        ) : (
                                            <div className="md:col-span-2 rounded-md border border-dashed border-border bg-muted/30 p-4 text-center">
                                                <p className="text-sm text-muted-foreground">
                                                    No webhook endpoints configured.
                                                </p>
                                                <p className="text-xs text-muted-foreground mt-1">
                                                    Go to the <strong>Providers</strong> tab to add webhook endpoints, then come back here to send.
                                                </p>
                                            </div>
                                        )
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
                                    {digestRules.length > 0 && formData.channel !== 'webhook' && (
                                        <div className="space-y-2">
                                            <Label className="flex items-center gap-1.5">
                                                <Layers className="h-3.5 w-3.5" />
                                                Use digest rule
                                            </Label>
                                            <Select value={advDigestRuleId || '__none__'} onValueChange={v => setAdvDigestRuleId(v === '__none__' ? '' : v)}>
                                                <SelectTrigger>
                                                    <SelectValue placeholder="Send immediately (no batching)" />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    <SelectItem value="__none__">Send immediately (no batching)</SelectItem>
                                                    {(digestRules || []).filter(r => r.status === 'active' && r.channel === formData.channel).map(r => (
                                                        <SelectItem key={r.id} value={r.id}>
                                                            {r.name} ({r.digest_key}, {r.window})
                                                        </SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                            <p className="text-xs text-muted-foreground">Batch notifications for the same user. One digest rule per send.</p>
                                        </div>
                                    )}
                                    {workflows.length > 0 && (
                                        <div className="space-y-2">
                                            <Label htmlFor="sendWorkflow">Trigger workflow after send (optional)</Label>
                                            <Select
                                                value={formData.workflow_trigger_id || 'none'}
                                                onValueChange={(val) => setFormData({ ...formData, workflow_trigger_id: val === 'none' ? '' : val })}
                                            >
                                                <SelectTrigger id="sendWorkflow">
                                                    <SelectValue placeholder="None" />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    <SelectItem value="none">None</SelectItem>
                                                    {workflows.map((w) => (
                                                        <SelectItem key={w.id} value={w.trigger_id}>{w.name} ({w.trigger_id})</SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                            <p className="text-xs text-muted-foreground">Runs workflow for the user after notification is sent (single-user send only)</p>
                                        </div>
                                    )}
                                </div>

                                {formVariables.length > 0 && (
                                    <div className="rounded-md border border-border bg-background">
                                        <button
                                            type="button"
                                            className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
                                            onClick={() => setVarsExpanded(!varsExpanded)}
                                        >
                                            <span className="flex items-center gap-2">
                                                Template Variables
                                                <Badge variant="secondary" className="text-xs">{formVariables.length}</Badge>
                                                {formSelectedTemplate?.metadata?.sample_data && (
                                                    <span className="text-xs text-muted-foreground font-normal">• sample data pre-filled</span>
                                                )}
                                            </span>
                                            {varsExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                                        </button>
                                        {varsExpanded && (
                                            <div className="px-4 pb-4 grid grid-cols-1 md:grid-cols-2 gap-3">
                                                {formVariables.map(v => {
                                                    const currentVal = dataInput ? (() => { try { return JSON.parse(dataInput)[v] || ''; } catch { return ''; } })() : '';
                                                    const sampleVal = formSelectedTemplate?.metadata?.sample_data?.[v];
                                                    const isDateVar = v.toLowerCase() === 'date';
                                                    return (
                                                        <div key={v} className="space-y-1">
                                                            <Label className="text-xs text-muted-foreground">{v}</Label>
                                                            {isDateVar ? (
                                                                <Input
                                                                    type="date"
                                                                    value={toISODateOnly(currentVal)}
                                                                    onChange={e => {
                                                                        const parsed = dataInput ? (() => { try { return JSON.parse(dataInput); } catch { return {}; } })() : {};
                                                                        parsed[v] = formatDateForTemplate(e.target.value);
                                                                        setDataInput(JSON.stringify(parsed, null, 2));
                                                                    }}
                                                                    className={currentVal ? '' : 'text-muted-foreground'}
                                                                />
                                                            ) : (
                                                                <Input
                                                                    value={currentVal}
                                                                    onChange={e => {
                                                                        const parsed = dataInput ? (() => { try { return JSON.parse(dataInput); } catch { return {}; } })() : {};
                                                                        parsed[v] = e.target.value;
                                                                        setDataInput(JSON.stringify(parsed, null, 2));
                                                                    }}
                                                                    placeholder={sampleVal ? String(sampleVal) : v}
                                                                    className={currentVal ? '' : 'text-muted-foreground'}
                                                                />
                                                            )}
                                                        </div>
                                                    );
                                                })}
                                            </div>
                                        )}
                                    </div>
                                )}

                                {/* Schedule toggle */}
                                <div className="rounded-md border border-border">
                                    <button
                                        type="button"
                                        className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
                                        onClick={() => {
                                            const next = !advScheduleEnabled;
                                            setAdvScheduleEnabled(next);
                                            if (!next) {
                                                setFormData({ ...formData, scheduled_at: undefined, recurrence: undefined });
                                                setAdvRecurrenceEnabled(false);
                                                setAdvRecurrencePreset('');
                                            }
                                        }}
                                    >
                                        <span className="flex items-center gap-2">
                                            <Clock className="h-4 w-4" />
                                            Schedule for later
                                        </span>
                                        <div className={`w-9 h-5 rounded-full transition-colors ${advScheduleEnabled ? 'bg-primary' : 'bg-muted-foreground/30'} relative`}>
                                            <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform ${advScheduleEnabled ? 'translate-x-4' : 'translate-x-0.5'}`} />
                                        </div>
                                    </button>
                                    {advScheduleEnabled && (
                                        <div className="px-4 pb-4 pt-1 space-y-4">
                                            <div className="space-y-2">
                                                <Label htmlFor="scheduledAt">Send at</Label>
                                                <Input
                                                    id="scheduledAt"
                                                    type="datetime-local"
                                                    value={formData.scheduled_at ? (() => {
                                                        try {
                                                            const d = new Date(formData.scheduled_at);
                                                            if (isNaN(d.getTime())) return '';
                                                            return formatInTimezone(d, scheduleTimezone);
                                                        } catch { return ''; }
                                                    })() : ''}
                                                    onChange={(e) => {
                                                        const val = e.target.value;
                                                        setFormData({ ...formData, scheduled_at: val ? scheduleToISO(val, scheduleTimezone) : undefined });
                                                    }}
                                                />
                                                <div className="space-y-1">
                                                    <Label htmlFor="advScheduleTz" className="text-xs">Timezone</Label>
                                                    <TimezonePicker
                                                        id="advScheduleTz"
                                                        value={scheduleTimezone}
                                                        onChange={setScheduleTimezone}
                                                        placeholder="Search timezone..."
                                                    />
                                                </div>
                                            </div>

                                            {/* Recurrence toggle */}
                                            <div className="rounded-md border border-border">
                                                <button
                                                    type="button"
                                                    className="flex w-full items-center justify-between px-3 py-2.5 text-sm hover:bg-muted/50 transition-colors"
                                                    onClick={() => {
                                                        const next = !advRecurrenceEnabled;
                                                        setAdvRecurrenceEnabled(next);
                                                        if (!next) {
                                                            setFormData({ ...formData, recurrence: undefined });
                                                            setAdvRecurrencePreset('');
                                                        }
                                                    }}
                                                >
                                                    <span className="text-sm">Repeat</span>
                                                    <div className={`w-9 h-5 rounded-full transition-colors ${advRecurrenceEnabled ? 'bg-primary' : 'bg-muted-foreground/30'} relative`}>
                                                        <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform ${advRecurrenceEnabled ? 'translate-x-4' : 'translate-x-0.5'}`} />
                                                    </div>
                                                </button>
                                                {advRecurrenceEnabled && (
                                                    <div className="px-3 pb-3 pt-1 space-y-3">
                                                        <div className="space-y-2">
                                                            <Label className="text-xs text-muted-foreground">Frequency</Label>
                                                            <Select
                                                                value={advRecurrencePreset}
                                                                onValueChange={(val) => {
                                                                    setAdvRecurrencePreset(val);
                                                                    const preset = recurrencePresets.find(p => p.value === val);
                                                                    if (preset) {
                                                                        setFormData({
                                                                            ...formData,
                                                                            recurrence: {
                                                                                ...formData.recurrence || { cron_expression: '' },
                                                                                cron_expression: preset.cron,
                                                                            }
                                                                        });
                                                                    }
                                                                }}
                                                            >
                                                                <SelectTrigger>
                                                                    <SelectValue placeholder="Select frequency" />
                                                                </SelectTrigger>
                                                                <SelectContent>
                                                                    {recurrencePresets.map(p => (
                                                                        <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
                                                                    ))}
                                                                </SelectContent>
                                                            </Select>
                                                        </div>
                                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                            <div className="space-y-2">
                                                                <Label htmlFor="recurrenceEnd" className="text-xs text-muted-foreground">Ends at (optional)</Label>
                                                                <Input
                                                                    id="recurrenceEnd"
                                                                    type="datetime-local"
                                                                    value={formData.recurrence?.end_date ? (() => {
                                                                        try {
                                                                            const d = new Date(formData.recurrence!.end_date!);
                                                                            if (isNaN(d.getTime())) return '';
                                                                            return formatInTimezone(d, scheduleTimezone);
                                                                        } catch { return ''; }
                                                                    })() : ''}
                                                                    onChange={(e) => {
                                                                        const val = e.target.value;
                                                                        setFormData({
                                                                            ...formData,
                                                                            recurrence: {
                                                                                ...formData.recurrence || { cron_expression: '' },
                                                                                end_date: val ? scheduleToISO(val, scheduleTimezone) : undefined
                                                                            }
                                                                        });
                                                                    }}
                                                                />
                                                            </div>
                                                            <div className="space-y-2">
                                                                <Label htmlFor="recurrenceCount" className="text-xs text-muted-foreground">Max occurrences (optional)</Label>
                                                                <Input
                                                                    id="recurrenceCount"
                                                                    type="number"
                                                                    value={formData.recurrence?.count || ''}
                                                                    onChange={(e) => setFormData({
                                                                        ...formData,
                                                                        recurrence: {
                                                                            ...formData.recurrence || { cron_expression: '' },
                                                                            count: parseInt(e.target.value) || undefined
                                                                        }
                                                                    })}
                                                                    placeholder="e.g. 10"
                                                                />
                                                            </div>
                                                        </div>
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                    )}
                                </div>
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
                                        <Button type="submit">{advScheduleEnabled && formData.scheduled_at ? 'Schedule Notification' : 'Send Notification'}</Button>
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
                                                <Label htmlFor="broadcastChannel">Channel</Label>
                                                <Select
                                                    value={formData.channel}
                                                    onValueChange={(val) => {
                                                        const next = val as any;
                                                        const currentTemplate = templates.find(t => t.id === formData.template_id);
                                                        const shouldClearTemplate = currentTemplate && currentTemplate.channel !== next;
                                                        setFormData({
                                                            ...formData,
                                                            channel: next,
                                                            template_id: shouldClearTemplate ? '' : formData.template_id,
                                                        });
                                                        if (shouldClearTemplate) setDataInput('');
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
                                                <Label htmlFor="broadcastTemplate">Template</Label>
                                                <Select
                                                    value={formData.template_id || ''}
                                                    onValueChange={(val) => {
                                                        setFormData({ ...formData, template_id: val });
                                                        const selected = templates.find(t => t.id === val);
                                                        const sample = getSampleData(selected);
                                                        if (Object.keys(sample).length > 0) {
                                                            setDataInput(JSON.stringify(sample, null, 2));
                                                        } else {
                                                            setDataInput('');
                                                        }
                                                    }}
                                                >
                                                    <SelectTrigger id="broadcastTemplate">
                                                        <SelectValue placeholder="Select a template" />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        {filteredTemplates.length === 0 ? (
                                                            <div className="px-2 py-3 text-sm text-muted-foreground text-center">No templates for {formData.channel}</div>
                                                        ) : (
                                                            filteredTemplates.map(t => (
                                                                <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                                                            ))
                                                        )}
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
                                            {digestRules.length > 0 && formData.channel !== 'webhook' && (
                                                <div className="space-y-2">
                                                    <Label className="flex items-center gap-1.5">
                                                        <Layers className="h-3.5 w-3.5" />
                                                        Use digest rule
                                                    </Label>
                                                    <Select value={broadcastDigestRuleId || '__none__'} onValueChange={v => setBroadcastDigestRuleId(v === '__none__' ? '' : v)}>
                                                        <SelectTrigger>
                                                            <SelectValue placeholder="Send immediately (no batching)" />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            <SelectItem value="__none__">Send immediately (no batching)</SelectItem>
                                                            {(digestRules || []).filter(r => r.status === 'active' && r.channel === formData.channel).map(r => (
                                                                <SelectItem key={r.id} value={r.id}>
                                                                    {r.name} ({r.digest_key}, {r.window})
                                                                </SelectItem>
                                                            ))}
                                                        </SelectContent>
                                                    </Select>
                                                    <p className="text-xs text-muted-foreground">Batch notifications per user. One digest rule per broadcast.</p>
                                                </div>
                                            )}
                                        </div>

                                        {workflows.length > 0 && (
                                            <div className="space-y-2">
                                                <Label htmlFor="broadcastWorkflow">Trigger workflow (optional)</Label>
                                                <Select
                                                    value={broadcastWorkflowTriggerId || 'none'}
                                                    onValueChange={(val) => setBroadcastWorkflowTriggerId(val === 'none' ? '' : val)}
                                                >
                                                    <SelectTrigger id="broadcastWorkflow">
                                                        <SelectValue placeholder="Send notifications (no workflow)" />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="none">Send notifications (no workflow)</SelectItem>
                                                        {workflows.map((w) => (
                                                            <SelectItem key={w.id} value={w.trigger_id ?? w.id}>{w.name} ({w.trigger_id})</SelectItem>
                                                        ))}
                                                    </SelectContent>
                                                </Select>
                                                <p className="text-xs text-muted-foreground">When set, runs the workflow for each recipient instead of sending a single notification.</p>
                                            </div>
                                        )}

                                        {topics.length > 0 && (
                                            <div className="space-y-2">
                                                <Label htmlFor="broadcastTopic">Limit to topic (optional)</Label>
                                                <Select
                                                    value={broadcastTopicKey || 'all'}
                                                    onValueChange={(val) => setBroadcastTopicKey(val === 'all' ? '' : val)}
                                                >
                                                    <SelectTrigger id="broadcastTopic">
                                                        <SelectValue placeholder="All users" />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="all">All users</SelectItem>
                                                        {topics.map((t) => (
                                                            <SelectItem key={t.id} value={t.key ?? t.id}>{t.name} ({t.key})</SelectItem>
                                                        ))}
                                                    </SelectContent>
                                                </Select>
                                                <p className="text-xs text-muted-foreground">When set, only subscribers of this topic receive the broadcast or workflow.</p>
                                            </div>
                                        )}

                                        {formVariables.length > 0 && (
                                            <div className="rounded-md border border-border bg-background">
                                                <button
                                                    type="button"
                                                    className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
                                                    onClick={() => setVarsExpanded(!varsExpanded)}
                                                >
                                                    <span className="flex items-center gap-2">
                                                        Template Variables
                                                        <Badge variant="secondary" className="text-xs">{formVariables.length}</Badge>
                                                        {formSelectedTemplate?.metadata?.sample_data && (
                                                            <span className="text-xs text-muted-foreground font-normal">• sample data pre-filled</span>
                                                        )}
                                                    </span>
                                                    {varsExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                                                </button>
                                                {varsExpanded && (
                                                    <div className="px-4 pb-4 grid grid-cols-1 md:grid-cols-2 gap-3">
                                                        {formVariables.map(v => {
                                                            const currentVal = dataInput ? (() => { try { return JSON.parse(dataInput)[v] || ''; } catch { return ''; } })() : '';
                                                            const sampleVal = formSelectedTemplate?.metadata?.sample_data?.[v];
                                                            const isDateVar = v.toLowerCase() === 'date';
                                                            return (
                                                                <div key={v} className="space-y-1">
                                                                    <Label className="text-xs text-muted-foreground">{v}</Label>
                                                                    {isDateVar ? (
                                                                        <Input
                                                                            type="date"
                                                                            value={toISODateOnly(currentVal)}
                                                                            onChange={e => {
                                                                                const parsed = dataInput ? (() => { try { return JSON.parse(dataInput); } catch { return {}; } })() : {};
                                                                                parsed[v] = formatDateForTemplate(e.target.value);
                                                                                setDataInput(JSON.stringify(parsed, null, 2));
                                                                            }}
                                                                            className={currentVal ? '' : 'text-muted-foreground'}
                                                                        />
                                                                    ) : (
                                                                        <Input
                                                                            value={currentVal}
                                                                            onChange={e => {
                                                                                const parsed = dataInput ? (() => { try { return JSON.parse(dataInput); } catch { return {}; } })() : {};
                                                                                parsed[v] = e.target.value;
                                                                                setDataInput(JSON.stringify(parsed, null, 2));
                                                                            }}
                                                                            placeholder={sampleVal ? String(sampleVal) : v}
                                                                            className={currentVal ? '' : 'text-muted-foreground'}
                                                                        />
                                                                    )}
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                )}
                                            </div>
                                        )}

                                        {/* Schedule toggle */}
                                        <div className="rounded-md border border-border">
                                            <button
                                                type="button"
                                                className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
                                                onClick={() => {
                                                    const next = !broadcastScheduleEnabled;
                                                    setBroadcastScheduleEnabled(next);
                                                    if (!next) setFormData({ ...formData, scheduled_at: undefined });
                                                }}
                                            >
                                                <span className="flex items-center gap-2">
                                                    <Clock className="h-4 w-4" />
                                                    Schedule for later
                                                </span>
                                                <div className={`w-9 h-5 rounded-full transition-colors ${broadcastScheduleEnabled ? 'bg-primary' : 'bg-muted-foreground/30'} relative`}>
                                                    <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform ${broadcastScheduleEnabled ? 'translate-x-4' : 'translate-x-0.5'}`} />
                                                </div>
                                            </button>
                                            {broadcastScheduleEnabled && (
                                                <div className="px-4 pb-4 pt-1 space-y-2">
                                                    <Input
                                                        id="broadcastScheduled"
                                                        type="datetime-local"
                                                        value={formData.scheduled_at ? (() => {
                                                            try {
                                                                const d = new Date(formData.scheduled_at);
                                                                if (isNaN(d.getTime())) return '';
                                                                return formatInTimezone(d, scheduleTimezone);
                                                            } catch { return ''; }
                                                        })() : ''}
                                                        onChange={(e) => {
                                                            const val = e.target.value;
                                                            setFormData({ ...formData, scheduled_at: val ? scheduleToISO(val, scheduleTimezone) : undefined });
                                                        }}
                                                    />
                                                    <div className="space-y-1">
                                                        <Label htmlFor="broadcastScheduleTz" className="text-xs">Timezone</Label>
                                                        <TimezonePicker
                                                            id="broadcastScheduleTz"
                                                            value={scheduleTimezone}
                                                            onChange={setScheduleTimezone}
                                                            placeholder="Search timezone..."
                                                        />
                                                    </div>
                                                </div>
                                            )}
                                        </div>

                                        <div className="flex justify-end pt-2">
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

                <div className="flex items-center justify-between mt-6 mb-3">
                    <div>
                        <h3 className="text-lg font-semibold">Notification History</h3>
                        <p className="text-xs text-muted-foreground mt-0.5">
                            Scheduled broadcasts appear here. Each recipient gets a row. Use <strong>Scheduled</strong> to see items waiting to be sent.
                        </p>
                    </div>
                    <Tabs
                        value={filters.status === 'pending' ? 'scheduled' : 'all'}
                        onValueChange={v => { setFilters({ ...filters, status: v === 'scheduled' ? 'pending' : undefined }); setPage(1); }}
                    >
                        <TabsList className="h-8">
                            <TabsTrigger value="all" className="text-xs">All</TabsTrigger>
                            <TabsTrigger value="scheduled" className="text-xs flex items-center gap-1">
                                <Clock className="h-3.5 w-3.5" /> Scheduled
                            </TabsTrigger>
                        </TabsList>
                    </Tabs>
                </div>

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
                                <SelectItem value="pending">Scheduled / Pending</SelectItem>
                                <SelectItem value="queued">Queued</SelectItem>
                                <SelectItem value="digested">Digested (batched)</SelectItem>
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
                        <div className="text-center py-8">
                            <p className="text-muted-foreground text-sm">No notification history found.</p>
                            {filters.status === 'pending' && (
                                <p className="text-xs text-muted-foreground mt-2 max-w-md mx-auto">
                                    Scheduled items appear here until they&#39;re sent. If you just scheduled a broadcast, each recipient will show as a row. Already-sent items appear under <strong>All</strong>.
                                </p>
                            )}
                        </div>
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
                                    <Button variant="outline" size="sm" onClick={handleCancelSelected} disabled={bulkActing} className="text-red-600 hover:text-red-700 hover:bg-red-50">
                                        <XCircle className="h-3.5 w-3.5 mr-1.5" />
                                        Cancel Selected
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
                                            <TableHead>Channel</TableHead>
                                            <TableHead>Status</TableHead>
                                            <TableHead className="hidden md:table-cell">Scheduled At</TableHead>
                                            <TableHead className="hidden md:table-cell">Sent At</TableHead>
                                            <TableHead className="w-10" />
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        {(notifications || []).map((n) => (
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
                                                    <Badge variant="outline" className="text-xs">
                                                        {n.channel}
                                                    </Badge>
                                                </TableCell>
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
                                                <TableCell onClick={e => e.stopPropagation()} className="space-x-1">
                                                    {(n.status === 'pending' || n.status === 'queued') && (
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            title="Cancel"
                                                            onClick={() => handleCancel(n.notification_id)}
                                                            className="text-red-600 hover:text-red-700 hover:bg-red-50"
                                                        >
                                                            <XCircle className="h-3.5 w-3.5" />
                                                        </Button>
                                                    )}
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
                                    {detailNotif.status === 'digested' && (
                                        <p className="text-xs text-muted-foreground mt-1">Batched into a digest and delivered via consolidated notification</p>
                                    )}
                                </div>
                                <div>
                                    <Label className="text-xs text-muted-foreground">Created</Label>
                                    <p className="text-sm">{detailNotif.created_at ? new Date(detailNotif.created_at).toLocaleString() : '-'}</p>
                                </div>
                                {detailNotif.sent_at && (
                                    <div>
                                        <Label className="text-xs text-muted-foreground">Sent At</Label>
                                        <p className="text-sm">{new Date(detailNotif.sent_at).toLocaleString()}</p>
                                    </div>
                                )}
                                {detailNotif.delivered_at && (
                                    <div>
                                        <Label className="text-xs text-muted-foreground">Delivered At</Label>
                                        <p className="text-sm">{new Date(detailNotif.delivered_at).toLocaleString()}</p>
                                    </div>
                                )}
                                {detailNotif.read_at && (
                                    <div>
                                        <Label className="text-xs text-muted-foreground">Read At</Label>
                                        <p className="text-sm">{new Date(detailNotif.read_at).toLocaleString()}</p>
                                    </div>
                                )}
                                {detailNotif.failed_at && (
                                    <div>
                                        <Label className="text-xs text-muted-foreground">Failed At</Label>
                                        <p className="text-sm text-red-600">{new Date(detailNotif.failed_at).toLocaleString()}</p>
                                    </div>
                                )}
                                {(detailNotif.retry_count ?? 0) > 0 && (
                                    <div>
                                        <Label className="text-xs text-muted-foreground">Retry Count</Label>
                                        <p className="text-sm">{detailNotif.retry_count}</p>
                                    </div>
                                )}
                            </div>
                            {detailNotif.error_message && (
                                <div>
                                    <Label className="text-xs text-muted-foreground">Error</Label>
                                    <p className="text-sm text-red-600 bg-red-50 dark:bg-red-950/30 p-2 rounded font-mono">{detailNotif.error_message}</p>
                                </div>
                            )}
                            {detailNotif.status === 'snoozed' && detailNotif.snoozed_until && (
                                <div>
                                    <Label className="text-xs text-muted-foreground">Snoozed Until</Label>
                                    <p className="text-sm text-purple-700">{new Date(detailNotif.snoozed_until).toLocaleString()}</p>
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
                                {(detailNotif.status === 'pending' || detailNotif.status === 'queued') && (
                                    <Button variant="outline" size="sm" className="text-red-600 hover:text-red-700"
                                        onClick={() => handleCancel(detailNotif.notification_id)}
                                    >
                                        <XCircle className="h-3.5 w-3.5 mr-1.5" />Cancel
                                    </Button>
                                )}
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
                                Cancel multiple scheduled/queued notifications by their IDs.
                            </DialogDescription>
                        </DialogHeader>
                        <div className="space-y-3">
                            <div className="space-y-2">
                                <Label htmlFor="cancelBatchIds">Notification IDs</Label>
                                <Textarea
                                    id="cancelBatchIds"
                                    value={cancelBatchIds}
                                    onChange={e => setCancelBatchIds(e.target.value)}
                                    placeholder="Paste notification IDs (comma or newline separated). From Batch Send response: items[].notification_id"
                                    className="font-mono text-xs min-h-[100px]"
                                />
                                <p className="text-xs text-muted-foreground">
                                    Paste IDs from the Batch Send response or select rows above and use &quot;Cancel Selected&quot;.
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
                                                className="border-muted-foreground data-[state=checked]:border-primary"
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
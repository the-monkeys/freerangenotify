import React, { useEffect, useState, useMemo, useCallback } from 'react';
import { mutateApiQueryCache, useApiQuery } from '../hooks/use-api-query';
import { notificationsAPI, usersAPI, templatesAPI, quickSendAPI, workflowsAPI, topicsAPI, digestRulesAPI, mediaAPI, twilioTemplatesAPI } from '../services/api';
import type { TwilioContentTemplate } from '../types';
import type { Notification, NotificationRequest, User, Template, BroadcastNotificationRequest } from '../types';
import { useAuth } from '../contexts/AuthContext';
import VerifyPhoneDialog from './VerifyPhoneDialog';
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

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Badge } from './ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Checkbox } from './ui/checkbox';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';
import { SlidePanel } from './ui/slide-panel';
import { CheckSquare, Archive, BellOff, Bell, Eye, X, Send, Clock, Layers, XCircle, Download, UploadCloud, FileText, AlertCircle } from 'lucide-react';
import { toast } from 'sonner';
import { extractErrorMessage } from '../lib/utils';
import { TimezonePicker } from './TimezonePicker';
import { localInTimezoneToISO, formatInTimezone, nowInTimezone } from '../lib/timezone';
import Papa from 'papaparse';
import EditablePreviewPanel from './templates/EditablePreviewPanel';
import WhatsAppPreview from './whatsapp/WhatsAppPreview';
import RichContentEditor, { emptyRichContent, isRichContentEmpty, richContentToPayload, type RichContentData } from './notifications/RichContentEditor';
import ChannelPreview from './channels/ChannelPreview';

interface AppNotificationsProps {
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

/** Custom JSON must not override Twilio Content API fields; those always come from the variable inputs. */
function mergeTwilioData(
    customData: Record<string, unknown>,
    twilioPayload: { content_sid: string; content_variables: string } | null
): Record<string, unknown> {
    if (!twilioPayload) {
        return { ...customData };
    }
    const { content_sid: _cs, content_variables: _cv, ...rest } = customData;
    return { ...rest, ...twilioPayload };
}

function twilioVariablesIncompleteMessage(
    template: TwilioContentTemplate,
    vars: Record<string, string>
): string | null {
    const keys =
        template.variables && Object.keys(template.variables).length > 0
            ? Object.keys(template.variables)
            : [];
    for (const k of keys) {
        if (String(vars[k] ?? '').trim() === '') {
            return `Fill every Twilio template variable. Missing value for {{${k}}}.`;
        }
    }
    return null;
}

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

/** Sorts template variables by the order they first appear in the template body. */
function sortVariablesByAppearance(variables: string[], body: string): string[] {
    return [...variables].sort((a, b) => {
        const ia = body.indexOf(`{{${a}}}`);
        const ib = body.indexOf(`{{${b}}}`);
        return (ia === -1 ? Infinity : ia) - (ib === -1 ? Infinity : ib);
    });
}

const SEND_FORM_SHELL_CLASS = 'rounded-xl border border-border/80 bg-muted/35 p-5 space-y-4';
const SEND_FORM_INFO_CLASS = 'text-sm text-muted-foreground';
const SEND_FORM_SECTION_CLASS = 'rounded-lg border border-border bg-background';
type PreviewSource = 'quick' | 'advanced' | 'broadcast';

const AppNotifications: React.FC<AppNotificationsProps> = ({ apiKey, webhooks, onUnreadCount }) => {
    const [page, setPage] = useState(1);
    const [pageSize] = useState(20);
    const [filters, setFilters] = useState<{ status?: string; channel?: string; from?: string; to?: string }>({});
    const notificationsCacheKey = `notifs-${apiKey}-${page}-${JSON.stringify(filters)}`;

    // 1. Notifications List
    const {
        data: notifsData,
        loading: notifsLoading,
        refetch: refreshNotifications
    } = useApiQuery(
        () => notificationsAPI.list(apiKey, page, pageSize, filters),
        [apiKey, page, pageSize, filters],
        {
            cacheKey: notificationsCacheKey,
            staleTime: 30000,
        }
    );

    const notifications = useMemo(() => notifsData?.notifications || [], [notifsData]);
    const totalNotifications = useMemo(() => notifsData?.total || 0, [notifsData]);

    // 2. Users (limited to 100 for dropdowns)
    const { data: usersData } = useApiQuery(
        () => usersAPI.list(apiKey, 1, 100),
        [apiKey],
        { cacheKey: `users-list-${apiKey}`, staleTime: 60000 }
    );
    const users = useMemo(() => usersData?.users || [], [usersData]);

    // 3. Templates
    const { data: templatesData } = useApiQuery(
        () => templatesAPI.list(apiKey, 100, 0),
        [apiKey],
        { cacheKey: `templates-list-${apiKey}`, staleTime: 60000 }
    );
    const templates = useMemo(() => templatesData?.templates || [], [templatesData]);

    // 3b. Twilio Content Templates (for WhatsApp)
    const { data: twilioData } = useApiQuery(
        () => twilioTemplatesAPI.list(apiKey),
        [apiKey],
        { cacheKey: `twilio-templates-${apiKey}`, staleTime: 60000 }
    );
    const twilioApproved = useMemo<TwilioContentTemplate[]>(() => {
        const all: TwilioContentTemplate[] = twilioData?.contents || [];
        return all.filter(t => t.approval_requests?.status === 'approved');
    }, [twilioData]);

    // 4. Workflows
    const { data: workflowsData } = useApiQuery(
        () => workflowsAPI.list(apiKey, 100, 0),
        [apiKey],
        { cacheKey: `workflows-list-${apiKey}`, staleTime: 60000 }
    );
    const workflows = useMemo(() => workflowsData?.workflows || [], [workflowsData]);

    // 5. Topics
    const { data: topicsData } = useApiQuery(
        () => topicsAPI.list(apiKey, 100, 0),
        [apiKey],
        { cacheKey: `topics-list-${apiKey}`, staleTime: 60000 }
    );
    const topics = useMemo(() => topicsData?.topics || [], [topicsData]);

    // 6. Digest Rules
    const { data: digestData } = useApiQuery(
        () => digestRulesAPI.list(apiKey, 100, 0),
        [apiKey],
        { cacheKey: `digest-rules-list-${apiKey}`, staleTime: 60000 }
    );
    const digestRules = useMemo(() => digestData?.rules || [], [digestData]);

    const loading = notifsLoading;
    const refresh = refreshNotifications;

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

    // Notification tab is a send-management view, not an inbox — no unread badge needed.
    useEffect(() => {
        if (onUnreadCount) onUnreadCount(0);
    }, [onUnreadCount]);
    const [formData, setFormData] = useState<NotificationRequest>(createEmptyForm());
    const [selectedTargets, setSelectedTargets] = useState<string[]>([]);
    const [selectedUsers, setSelectedUsers] = useState<string[]>([]);
    const [dataInput, setDataInput] = useState('');
    const [confirmingBroadcast, setConfirmingBroadcast] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [broadcastWorkflowTriggerId, setBroadcastWorkflowTriggerId] = useState('');
    const [broadcastTopicKey, setBroadcastTopicKey] = useState('');
    // Local blob URLs for WhatsApp media preview
    const [mediaFiles, setMediaFiles] = useState<{ url: string; previewUrl: string; type: string; name: string }[]>([]);

    // Inbox selection & actions state
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
    const [detailNotif, setDetailNotif] = useState<Notification | null>(null);
    const [bulkActing, setBulkActing] = useState(false);

    const prependOptimisticNotifications = useCallback((entries: Notification[]) => {
        if (entries.length === 0) return;

        mutateApiQueryCache<{ notifications: Notification[]; total: number }>(
            notificationsCacheKey,
            (current) => {
                if (!current) return current;

                const existing = current.notifications || [];
                const dedup = new Set(existing.map((item) => item.notification_id));
                const fresh = entries.filter((item) => !dedup.has(item.notification_id));
                if (fresh.length === 0) return current;

                return {
                    ...current,
                    total: Math.max(0, (current.total || 0) + fresh.length),
                    notifications: [...fresh, ...existing].slice(0, pageSize),
                };
            }
        );
    }, [notificationsCacheKey, pageSize]);

    const buildOptimisticNotification = useCallback((input: {
        userId: string;
        channel: Notification['channel'];
        priority: Notification['priority'];
        templateId?: string;
        data?: Record<string, any>;
        scheduledAt?: string;
        metadata?: Record<string, any>;
    }): Notification => {
        const now = new Date().toISOString();
        const optimisticId = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
            ? crypto.randomUUID()
            : `optimistic-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;

        return {
            notification_id: optimisticId,
            app_id: '',
            user_id: input.userId,
            channel: input.channel,
            priority: input.priority,
            status: input.scheduledAt ? 'pending' : 'queued',
            content: {
                title: '',
                body: '',
                data: input.data,
                media_url: (input.data?.media_url as string | undefined) || undefined,
            },
            template_id: input.templateId,
            metadata: input.metadata,
            scheduled_at: input.scheduledAt,
            created_at: now,
            updated_at: now,
        };
    }, []);

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
            // Group selected notification IDs by user_id (backend requires user_id per call)
            const groups = new Map<string, string[]>();
            for (const id of selectedIds) {
                const notif = notifications.find(n => n.notification_id === id);
                const uid = notif?.user_id || '';
                if (!groups.has(uid)) groups.set(uid, []);
                groups.get(uid)!.push(id);
            }
            await Promise.all(
                Array.from(groups.entries()).map(([userId, ids]) =>
                    notificationsAPI.markRead(apiKey, { notification_ids: ids, user_id: userId })
                )
            );
            toast.success(`${selectedIds.size} notification(s) marked as read`);
            setSelectedIds(new Set());
            refresh();
        } catch (err) { toast.error(extractErrorMessage(err, 'Failed to mark as read')); } finally { setBulkActing(false); }
    };

    const handleBulkArchive = async () => {
        if (selectedIds.size === 0) return;
        setBulkActing(true);
        try {
            // Group selected notification IDs by user_id (backend requires user_id per call)
            const groups = new Map<string, string[]>();
            for (const id of selectedIds) {
                const notif = notifications.find(n => n.notification_id === id);
                const uid = notif?.user_id || '';
                if (!groups.has(uid)) groups.set(uid, []);
                groups.get(uid)!.push(id);
            }
            await Promise.all(
                Array.from(groups.entries()).map(([userId, ids]) =>
                    notificationsAPI.bulkArchive(apiKey, { notification_ids: ids, user_id: userId })
                )
            );
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

    // Batch send state
    const [showBatchSend, setShowBatchSend] = useState(false);
    const [batchCsvFile, setBatchCsvFile] = useState<File | null>(null);
    const [batchSending, setBatchSending] = useState(false);
    const [batchRows, setBatchRows] = useState<any[]>([]);
    const [dragActive, setDragActive] = useState(false);

    const downloadSampleCSV = () => {
        const headers = ['user_id', 'channel', 'template_id', 'title', 'body', 'priority', 'name'];
        const sampleRows = [
            ['user-uuid-123', 'email', 'template-uuid-456', '', '', 'normal', 'Alice'],
            ['user-uuid-789', 'email', '', 'Direct Message', 'Hello there!', 'high', '']
        ];
        const csvContent = [headers, ...sampleRows].map(r => r.join(',')).join('\n');
        const blob = new Blob([csvContent], { type: 'text/csv' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'freerange_batch_sample.csv';
        a.click();
        window.URL.revokeObjectURL(url);
    };

    const handleFile = (file: File) => {
        if (!file.name.endsWith('.csv')) {
            toast.error('Please upload a valid CSV file');
            return;
        }
        setBatchCsvFile(file);
        Papa.parse(file, {
            header: true,
            skipEmptyLines: true,
            complete: (results) => {
                setBatchRows(results.data);
            }
        });
    };

    const onDrop = (e: React.DragEvent) => {
        e.preventDefault();
        setDragActive(false);
        if (e.dataTransfer.files && e.dataTransfer.files[0]) {
            handleFile(e.dataTransfer.files[0]);
        }
    };

    const handleBatchSend = async () => {
        const rowsToProcess = batchRows.length > 0 ? batchRows : [];
        if (rowsToProcess.length === 0) {
            toast.error('No data to send');
            return;
        }

        setBatchSending(true);
        try {
            const notifications = rowsToProcess.map((row) => {
                const { user_id, channel, template_id, title, body, priority, scheduled_at, ...customData } = row;
                return {
                    user_id: user_id || '',
                    channel: channel || 'email',
                    priority: priority || 'normal',
                    template_id: template_id || '',
                    title: title || undefined,
                    body: body || undefined,
                    scheduled_at: scheduled_at || undefined,
                    data: Object.keys(customData).length > 0 ? customData : undefined,
                };
            });

            const result = await notificationsAPI.sendBatch(apiKey, { notifications });
            if (result.sent === result.total) {
                toast.success(`Batch of ${result.sent} notification(s) sent successfully!`);
            } else {
                toast.warning(`Sent ${result.sent} of ${result.total}. Check details in history.`);
            }
            setShowBatchSend(false);
            setBatchCsvFile(null);
            setBatchRows([]);
            const optimistic = notifications
                .slice(0, pageSize)
                .map((item) => buildOptimisticNotification({
                    userId: item.user_id || '',
                    channel: (item.channel || 'email') as Notification['channel'],
                    priority: (item.priority || 'normal') as Notification['priority'],
                    templateId: item.template_id,
                    data: item.data,
                    scheduledAt: item.scheduled_at,
                }));
            prependOptimisticNotifications(optimistic);
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Batch send failed'));
        } finally {
            setBatchSending(false);
        }
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
    const [quickToManual, setQuickToManual] = useState(false);
    const [quickTemplateId, setQuickTemplateId] = useState('');
    const [quickData, setQuickData] = useState<Record<string, string>>({});
    const [quickSending, setQuickSending] = useState(false);
    const [quickPriority, setQuickPriority] = useState<string>('normal');
    const [quickScheduledAt, setQuickScheduledAt] = useState<string>('');
    const [quickDigestRuleId, setQuickDigestRuleId] = useState<string>('');
    // Stores the NAME of a registered webhook provider (e.g. "Slack Alerts"),
    // not the URL. The backend resolves the name to the matching custom
    // provider and dispatches via that provider's channel-specific renderer.
    // Sending a raw URL would route through the generic webhook provider and
    // post FRN's envelope shape — which Slack/Discord/Teams reject.
    const [quickWebhookTarget, setQuickWebhookTarget] = useState<string>('');
    const [quickRichContent, setQuickRichContent] = useState<RichContentData>(emptyRichContent());
    const [quickMedia, setQuickMedia] = useState<{ url: string; previewUrl: string; type: string; name: string } | null>(null);
    const [slidePreview, setSlidePreview] = useState<{ templateId: string; templateName: string; channel: string } | null>(null);
    const [previewSource, setPreviewSource] = useState<PreviewSource | null>(null);
    const [activePreviews, setActivePreviews] = useState<Record<string, { data: string; rendered: string; loading: boolean }>>({});
    const [advDigestRuleId, setAdvDigestRuleId] = useState<string>('');
    const [advRichContent, setAdvRichContent] = useState<RichContentData>(emptyRichContent()); const [broadcastDigestRuleId, setBroadcastDigestRuleId] = useState<string>('');

    // ── Free-form WhatsApp state ──
    const [quickFreeformBody, setQuickFreeformBody] = useState('');
    const [quickFreeformTitle, setQuickFreeformTitle] = useState('');
    const isFreeformWhatsApp = quickTemplateId === 'freeform:whatsapp';

    // Filtered templates by channel
    const filteredTemplates = useMemo(() => (templates || []).filter(t => t.channel === formData.channel), [templates, formData.channel]);

    // Helper: check if a template selection is a Twilio Content Template
    const isTwilioSelection = (id: string) => id.startsWith('twilio:');
    const extractTwilioSid = (id: string) => id.slice('twilio:'.length);
    const findTwilioTemplate = (id: string) => twilioApproved.find(t => t.sid === extractTwilioSid(id));

    // Selected Twilio template (memoised) — used to drive variable inputs and preview
    const quickSelectedTwilioTemplate = useMemo<TwilioContentTemplate | undefined>(() => {
        if (!isTwilioSelection(quickTemplateId)) return undefined;
        return findTwilioTemplate(quickTemplateId);
    }, [quickTemplateId, twilioApproved]);

    // Ordered list of Twilio variable keys for the selected template
    const quickTwilioVariableKeys = useMemo<string[]>(() => {
        if (!quickSelectedTwilioTemplate?.variables) return [];
        // Sort numeric keys ascending, keep non-numeric in insertion order after
        const keys = Object.keys(quickSelectedTwilioTemplate.variables);
        const numeric = keys.filter(k => /^\d+$/.test(k)).sort((a, b) => Number(a) - Number(b));
        const named = keys.filter(k => !/^\d+$/.test(k));
        return [...numeric, ...named];
    }, [quickSelectedTwilioTemplate]);

    // Inline Twilio preview toggle (does not reuse EditablePreviewPanel since Twilio
    // content lives locally and doesn't need a server render call)
    const [twilioPreviewOpen, setTwilioPreviewOpen] = useState(false);

    // Form tabs (Bulk Send + Broadcast) share formData.template_id. Twilio vars live
    // in a separate record so they can drive both variable inputs and the inline preview
    // without polluting dataInput. They are serialized into content_variables at send time.
    const [formTwilioVars, setFormTwilioVars] = useState<Record<string, string>>({});
    const [formTwilioPreviewOpen, setFormTwilioPreviewOpen] = useState(false);

    const formSelectedTwilioTemplate = useMemo<TwilioContentTemplate | undefined>(() => {
        if (!isTwilioSelection(formData.template_id || '')) return undefined;
        return findTwilioTemplate(formData.template_id!);
    }, [formData.template_id, twilioApproved]);

    const formTwilioVariableKeys = useMemo<string[]>(() => {
        if (!formSelectedTwilioTemplate?.variables) return [];
        const keys = Object.keys(formSelectedTwilioTemplate.variables);
        const numeric = keys.filter(k => /^\d+$/.test(k)).sort((a, b) => Number(a) - Number(b));
        const named = keys.filter(k => !/^\d+$/.test(k));
        return [...numeric, ...named];
    }, [formSelectedTwilioTemplate]);

    // Twilio variables now drive content_variables through quickData / formTwilioVars
    // directly at send time — no helper needed.

    // Channels that deliver to an endpoint, not a user — recipient is irrelevant.
    const WEBHOOK_LIKE_CHANNELS = ['webhook', 'discord', 'slack', 'teams'];

    // Quick-Send: detect variables from selected template
    const quickSelectedTemplate = useMemo(() => templates.find(t => t.id === quickTemplateId), [templates, quickTemplateId]);
    const isQuickWebhookLike = quickSelectedTemplate ? WEBHOOK_LIKE_CHANNELS.includes(quickSelectedTemplate.channel) : false;
    const quickVariables = useMemo(() => {
        const vars = quickSelectedTemplate?.variables || [];
        return quickSelectedTemplate?.body ? sortVariablesByAppearance(vars, quickSelectedTemplate.body) : vars;
    }, [quickSelectedTemplate]);

    // Advanced/Broadcast: detect variables from selected template
    const formSelectedTemplate = useMemo(() => templates.find(t => t.id === formData.template_id), [templates, formData.template_id]);
    const formVariables = useMemo(() => {
        const vars = formSelectedTemplate?.variables || [];
        return formSelectedTemplate?.body ? sortVariablesByAppearance(vars, formSelectedTemplate.body) : vars;
    }, [formSelectedTemplate]);
    const parsedFormData = useMemo(() => {
        const parsed = parseCustomData(dataInput);
        if (!parsed || typeof parsed !== 'object') return {} as Record<string, string>;
        const next: Record<string, string> = {};
        for (const [k, v] of Object.entries(parsed)) next[k] = v == null ? '' : String(v);
        return next;
    }, [dataInput]);

    // Helper: get sample_data from template metadata.
    // Only primitive values (string/number/boolean) become template variables.
    // Object/array values (e.g. rich-content fields like `actions`, `fields`,
    // `style`) belong on dedicated request fields, not in the template-variable
    // map — coercing them to strings produces useless "[object Object]" output.
    //
    // Routing keys (`webhook_target`, `webhook_url`) are NEVER template variables.
    // They are dispatch directives the backend reads from `data` to resolve a
    // custom provider — and `webhook_target` overrides any explicit `webhook_url`
    // the user types into the dedicated URL field. Auto-populating them from the
    // template's `sample_data` causes silent misrouting (e.g. the user pastes a
    // Slack URL but the message lands in Discord because the seeded sample_data
    // pinned `webhook_target: "Discord Alerts"`). Strip them here.
    const ROUTING_KEYS = new Set(['webhook_target', 'webhook_url']);
    const getSampleData = (template: Template | undefined): Record<string, string> => {
        if (!template?.metadata?.sample_data) return {};
        const sd = template.metadata.sample_data;
        const result: Record<string, string> = {};
        for (const [k, v] of Object.entries(sd)) {
            if (v == null) continue;
            if (ROUTING_KEYS.has(k)) continue;
            const t = typeof v;
            if (t === 'string' || t === 'number' || t === 'boolean') {
                result[k] = String(v);
            }
        }
        return result;
    };

    // Auto-select first user when users load and quickTo is empty
    useEffect(() => {
        if (!quickTo && !quickToManual && users.length > 0) {
            setQuickTo(users[0].user_id);
        }
    }, [users, quickTo, quickToManual]);

    const updateFormDataVariable = useCallback((variable: string, value: string) => {
        setDataInput((prev) => {
            const current = parseCustomData(prev);
            const next: Record<string, string> = current && typeof current === 'object'
                ? Object.fromEntries(Object.entries(current).map(([k, v]) => [k, v == null ? '' : String(v)]))
                : {};
            next[variable] = value;
            return JSON.stringify(next, null, 2);
        });
    }, []);

    const removeFormDataVariable = useCallback((variable: string) => {
        setDataInput((prev) => {
            const current = parseCustomData(prev);
            const next: Record<string, string> = current && typeof current === 'object'
                ? Object.fromEntries(Object.entries(current).map(([k, v]) => [k, v == null ? '' : String(v)]))
                : {};
            delete next[variable];
            return Object.keys(next).length > 0 ? JSON.stringify(next, null, 2) : '';
        });
    }, []);

    const handleFormMediaUpload = useCallback(async (files: File[]) => {
        if (files.length === 0) return;

        const remainingLimit = 5 - mediaFiles.length;
        const filesToUpload = files.slice(0, remainingLimit);

        if (files.length > remainingLimit) {
            toast.warning(`Only ${remainingLimit} more file(s) can be added.`);
        }

        for (const file of filesToUpload) {
            if (file.size > 16 * 1024 * 1024) {
                toast.error(`${file.name} is too large (max 16 MB)`);
                continue;
            }

            const localUrl = URL.createObjectURL(file);
            try {
                toast.info(`Uploading ${file.name}...`);
                const result = await mediaAPI.upload(apiKey, file);
                setMediaFiles((prev) => {
                    const next = [...prev, {
                        url: result.url,
                        previewUrl: localUrl,
                        type: file.type,
                        name: file.name,
                    }];
                    if (next.length === 1) {
                        setFormData((fd) => ({ ...fd, media_url: result.url } as any));
                        updateFormDataVariable('media_url', result.url);
                    }
                    return next;
                });
            } catch (err) {
                URL.revokeObjectURL(localUrl);
                toast.error(`Upload failed for ${file.name}: ` + extractErrorMessage(err));
            }
        }

        toast.success('Upload complete');
    }, [apiKey, mediaFiles.length, updateFormDataVariable]);

    const removeFormMediaAt = useCallback((idx: number) => {
        const target = mediaFiles[idx];
        if (target?.previewUrl) URL.revokeObjectURL(target.previewUrl);
        const newFiles = mediaFiles.filter((_, i) => i !== idx);
        setMediaFiles(newFiles);
        const primary = newFiles.length > 0 ? newFiles[0].url : '';
        setFormData((prev) => ({ ...prev, media_url: primary } as any));
        if (primary) updateFormDataVariable('media_url', primary);
        else removeFormDataVariable('media_url');
    }, [mediaFiles, removeFormDataVariable, updateFormDataVariable]);

    const clearFormMedia = useCallback(() => {
        mediaFiles.forEach((m) => {
            if (m.previewUrl) URL.revokeObjectURL(m.previewUrl);
        });
        setMediaFiles([]);
        setFormData((prev) => ({ ...prev, media_url: '' } as any));
        removeFormDataVariable('media_url');
    }, [mediaFiles, removeFormDataVariable]);

    const updatePreviewVariable = useCallback((templateId: string, variable: string, value: string) => {
        setActivePreviews((prev) => {
            const currentData = prev[templateId]?.data || '{}';
            try {
                const parsed = JSON.parse(currentData) as Record<string, string>;
                const next = { ...parsed, [variable]: value };
                const serialized = JSON.stringify(next, null, 2);

                if (previewSource === 'quick') {
                    setQuickData(next);
                } else if (previewSource === 'advanced' || previewSource === 'broadcast') {
                    setDataInput(serialized);
                }

                return {
                    ...prev,
                    [templateId]: {
                        ...(prev[templateId] || { rendered: '', loading: false }),
                        data: serialized,
                    },
                };
            } catch {
                return prev;
            }
        });
    }, [previewSource]);

    const renderTemplatePreview = useCallback(async (template: Template, data: Record<string, any>, source: PreviewSource) => {
        const templateId = template.id;
        const serialized = JSON.stringify(data || {}, null, 2);

        setPreviewSource(source);
        setSlidePreview({
            templateId,
            templateName: template.name,
            channel: template.channel,
        });

        setActivePreviews((prev) => ({
            ...prev,
            [templateId]: {
                ...(prev[templateId] || { rendered: '', loading: false }),
                data: serialized,
                loading: true,
            },
        }));

        try {
            const res = await templatesAPI.render(apiKey, templateId, {
                data: data || {},
                editable: template.channel === 'email',
            });
            setActivePreviews((prev) => ({
                ...prev,
                [templateId]: {
                    ...(prev[templateId] || { rendered: '', loading: false }),
                    data: prev[templateId]?.data ?? serialized,
                    rendered: res.rendered_body || '',
                    loading: false,
                },
            }));
        } catch (err) {
            toast.error(extractErrorMessage(err, 'Failed to render preview'));
            setActivePreviews((prev) => ({
                ...prev,
                [templateId]: {
                    ...(prev[templateId] || { rendered: '', loading: false }),
                    data: prev[templateId]?.data ?? serialized,
                    loading: false,
                },
            }));
        }
    }, [apiKey]);

    const handleRenderPreview = useCallback(async (templateId: string) => {
        const template = templates.find((t) => t.id === templateId);
        const preview = activePreviews[templateId];
        if (!template || !preview) return;

        let parsedData: Record<string, any> = {};
        try {
            parsedData = JSON.parse(preview.data || '{}');
        } catch {
            toast.error('Invalid preview JSON data');
            return;
        }

        await renderTemplatePreview(template, parsedData, previewSource || 'quick');
    }, [templates, activePreviews, previewSource, renderTemplatePreview]);

    const renderQuickPreview = useCallback(async () => {
        if (!quickSelectedTemplate) return;
        await renderTemplatePreview(
            quickSelectedTemplate,
            Object.keys(quickData).length > 0 ? quickData : {},
            'quick',
        );
    }, [quickSelectedTemplate, quickData, renderTemplatePreview]);

    const renderSendPreview = useCallback(async (context: 'advanced' | 'broadcast') => {
        if (!formSelectedTemplate?.id) {
            toast.error('Select a template to preview');
            return;
        }

        const customData = parseCustomData(dataInput);
        if (customData === null) {
            toast.error('Invalid JSON in custom data');
            return;
        }

        await renderTemplatePreview(formSelectedTemplate, customData || {}, context);
    }, [formSelectedTemplate, dataInput, renderTemplatePreview]);

    const { user } = useAuth();
    const [isVerifyDialogOpen, setIsVerifyDialogOpen] = useState(false);

    // Checks if the user needs phone verification for the current send request
    const checkVerificationAndBlock = useCallback((channel: string) => {
        if ((channel === 'whatsapp' || channel === 'sms') && !user?.phone_verified) {
            setIsVerifyDialogOpen(true);
            return true;
        }
        return false;
    }, [user]);

    const handleQuickSend = async () => {
        const isFreeform = isFreeformWhatsApp;
        const isTwilioCheck = !isFreeform && isTwilioSelection(quickTemplateId);
        const selTpl = (isTwilioCheck || isFreeform) ? undefined : templates.find(t => t.id === quickTemplateId);
        const isWebhookLike = selTpl ? WEBHOOK_LIKE_CHANNELS.includes(selTpl.channel) : false;
        if (!quickTemplateId || (!isWebhookLike && !isFreeform && !quickTo)) return;
        if (isFreeform && !quickTo) return;
        if (isFreeform && !quickFreeformBody.trim()) {
            toast.error('Message body is required for free-form WhatsApp messages.');
            return;
        }

        const isTwilio = isTwilioCheck;
        const selectedTemplate = (isTwilio || isFreeform) ? undefined : templates.find(t => t.id === quickTemplateId);
        const channel = (isTwilio || isFreeform) ? 'whatsapp' : (selectedTemplate?.channel || 'email');
        if (checkVerificationAndBlock(channel)) return;

        // Webhook routing: MUST resolve a destination endpoint. If the template
        // already pins a webhook_target, use it as the default; otherwise the user
        // must select one in the UI.
        const resolvedQuickWebhookTarget =
            channel === 'webhook'
                ? (quickWebhookTarget || selectedTemplate?.webhook_target || '')
                : '';
        if (channel === 'webhook' && !resolvedQuickWebhookTarget) {
            toast.error('Select a webhook endpoint before sending.');
            return;
        }

        if (quickScheduledAt && quickScheduledAt < nowInTimezone(scheduleTimezone)) {
            toast.error('Scheduled time must be in the future.');
            return;
        }

        setQuickSending(true);
        try {
            const selectedDigestRule = quickDigestRuleId ? digestRules.find(r => r.id === quickDigestRuleId) : null;
            const scheduledAt = quickScheduledAt ? scheduleToISO(quickScheduledAt, scheduleTimezone) : undefined;

            // Build send payload — Twilio Content Template sends content_sid via data
            const twilioTpl = isTwilio ? findTwilioTemplate(quickTemplateId) : undefined;
            if (isTwilio && twilioTpl) {
                const vErr = twilioVariablesIncompleteMessage(twilioTpl, quickData);
                if (vErr) {
                    toast.error(vErr);
                    setQuickSending(false);
                    return;
                }
            }
            // For Twilio: quickData contains user-typed variable values keyed by variable name
            // (e.g. "1", "2"). Bundle them into content_variables JSON string and attach
            // content_sid. Do NOT pass the raw variable keys at the top level — the provider
            // only reads content_sid + content_variables.
            const sendData = isTwilio && twilioTpl
                ? {
                    content_sid: twilioTpl.sid,
                    content_variables: JSON.stringify(quickData || {}),
                }
                : isFreeform
                    ? (quickMedia ? { media_url: quickMedia.url } : undefined)
                    : (() => {
                        // Strip routing keys — they belong on dedicated top-level
                        // request fields (`webhook_url`, `webhook_target`), never
                        // in template `data`. Leaving `webhook_target` in `data`
                        // causes the worker to resolve it as a custom provider
                        // and override the explicit `webhook_url`, silently
                        // misrouting (Slack URL → Discord, etc.).
                        const cleaned: Record<string, unknown> = {};
                        for (const [k, v] of Object.entries(quickData)) {
                            if (k === 'webhook_target' || k === 'webhook_url') continue;
                            cleaned[k] = v;
                        }
                        return Object.keys(cleaned).length > 0 ? cleaned : undefined;
                    })();

            await quickSendAPI.send(apiKey, {
                to: isWebhookLike ? '' : quickTo,
                template: (isTwilio || isFreeform) ? undefined : (quickSelectedTemplate?.name || quickTemplateId),
                channel: (isTwilio || isFreeform) ? 'whatsapp' : undefined,
                subject: isFreeform && quickFreeformTitle ? quickFreeformTitle : undefined,
                body: isFreeform ? quickFreeformBody : undefined,
                data: sendData,
                priority: quickPriority as any,
                scheduled_at: scheduledAt,
                digest_key: selectedDigestRule?.digest_key,
                webhook_target: channel === 'webhook' ? resolvedQuickWebhookTarget : undefined,
                // Rich webhook content as top-level fields (backend DTO contract).
                ...(!isRichContentEmpty(quickRichContent) ? richContentToPayload(quickRichContent) : {}),
            });
            prependOptimisticNotifications([
                buildOptimisticNotification({
                    userId: quickTo,
                    channel: (quickSelectedTemplate?.channel || 'email') as Notification['channel'],
                    priority: (quickPriority || 'normal') as Notification['priority'],
                    templateId: quickSelectedTemplate?.id,
                    data: Object.keys(quickData).length > 0 ? quickData : undefined,
                    scheduledAt,
                    metadata: selectedDigestRule ? { digest_key: selectedDigestRule.digest_key } : undefined,
                }),
            ]);
            toast.success('Notification sent!');
            setQuickTo('');
            setQuickTemplateId('');
            setQuickData({});
            setQuickPriority('normal');
            setQuickScheduledAt('');
            setQuickScheduleEnabled(false);
            setQuickDigestRuleId('');
            setQuickWebhookTarget('');
            setQuickFreeformBody('');
            setQuickFreeformTitle('');
            if (quickMedia?.previewUrl) {
                URL.revokeObjectURL(quickMedia.previewUrl);
            }
            setQuickMedia(null);
        } catch (error) {
            const msg = extractErrorMessage(error, 'Quick-send failed');
            if (msg.includes('phone_verification_required')) {
                toast.error(`${channel === 'whatsapp' ? 'WhatsApp' : 'SMS'} failed: Please verify your phone number in the Profile menu (top left) to use system credentials.`);
            } else {
                toast.error(msg);
            }
        } finally {
            setQuickSending(false);
        }
    };



    const handleBroadcastSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        setConfirmingBroadcast(true);
    };

    const executeBroadcast = async () => {
        if (formData.scheduled_at && new Date(formData.scheduled_at) <= new Date()) {
            toast.error('Scheduled time must be in the future.');
            setConfirmingBroadcast(false);
            return;
        }

        if (checkVerificationAndBlock(formData.channel)) return;

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
            const isTwilio = isTwilioSelection(formData.template_id || '');
            if (!useWorkflow && !formData.template_id) {
                toast.error('Select a template or a workflow to trigger');
                setIsSubmitting(false);
                return;
            }

            const twilioTpl = isTwilio ? findTwilioTemplate(formData.template_id!) : undefined;
            const broadcastDigestRule = broadcastDigestRuleId ? digestRules.find(r => r.id === broadcastDigestRuleId) : null;
            const twilioPayload = isTwilio && twilioTpl ? {
                content_sid: twilioTpl.sid,
                content_variables: JSON.stringify(formTwilioVars || {}),
            } : null;
            if (isTwilio && twilioTpl) {
                const vErr = twilioVariablesIncompleteMessage(twilioTpl, formTwilioVars);
                if (vErr) {
                    toast.error(vErr);
                    setIsSubmitting(false);
                    return;
                }
            }
            const richPayload = !isRichContentEmpty(advRichContent) ? richContentToPayload(advRichContent) : {};
            const payload: BroadcastNotificationRequest = {
                channel: formData.channel,
                priority: formData.priority,
                template_id: (useWorkflow || isTwilio) ? undefined : formData.template_id,
                data: mergeTwilioData(customData as Record<string, unknown>, twilioPayload),
                scheduled_at: formData.scheduled_at,
                workflow_trigger_id: broadcastWorkflowTriggerId || undefined,
                topic_key: broadcastTopicKey || undefined,
                metadata: broadcastDigestRule ? { digest_key: broadcastDigestRule.digest_key } : undefined,
                ...richPayload,
            };

            // Webhook broadcasts are not a "fan-out to users" operation; they are
            // a single delivery to one (or more) webhook endpoints.
            if (payload.channel === 'webhook') {
                const pinned = formSelectedTemplate?.webhook_target || '';
                const targets = selectedTargets.length > 0 ? selectedTargets : (pinned ? [pinned] : []);
                if (targets.length === 0) {
                    toast.error('Select at least one webhook endpoint.');
                    setIsSubmitting(false);
                    return;
                }
                const sendPromises = targets.map((t) =>
                    notificationsAPI.send(apiKey, {
                        user_id: '',
                        channel: 'webhook',
                        priority: payload.priority as any,
                        template_id: payload.template_id,
                        data: payload.data,
                        scheduled_at: payload.scheduled_at,
                        metadata: payload.metadata,
                        webhook_target: t,
                        ...richPayload,
                    })
                );
                await Promise.all(sendPromises);
            } else {
                await notificationsAPI.broadcast(apiKey, payload);
            }

            setFormData(createEmptyForm());
            setDataInput('');
            setBroadcastWorkflowTriggerId('');
            setBroadcastTopicKey('');
            setBroadcastDigestRuleId('');
            setFormTwilioVars({});
            setFormTwilioPreviewOpen(false);
            clearFormMedia();
            refresh();
            toast.success(useWorkflow ? 'Workflows triggered successfully.' : 'Broadcast initiated successfully.');
        } catch (error) {
            console.error('Failed to broadcast notification:', error);
            const msg = extractErrorMessage(error, 'Failed to broadcast notification');
            if (msg.includes('phone_verification_required')) {
                toast.error(`${formData.channel === 'whatsapp' ? 'WhatsApp' : 'SMS'} failed: Please verify your phone number in the Profile menu (top left) to use system credentials.`);
            } else {
                toast.error(msg);
            }
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleSendNotification = async (e: React.FormEvent) => {
        e.preventDefault();

        if (formData.scheduled_at && new Date(formData.scheduled_at) <= new Date()) {
            toast.error('Scheduled time must be in the future.');
            return;
        }

        if (checkVerificationAndBlock(formData.channel)) return;

        try {
            const customData = parseCustomData(dataInput);
            if (customData === null) {
                toast.error('Invalid JSON in custom data');
                return;
            }

            const userIds = selectedUsers.length > 0 ? selectedUsers : (formData.user_id ? [formData.user_id] : []);
            const requiresUser = !WEBHOOK_LIKE_CHANNELS.includes(formData.channel);
            if (requiresUser && userIds.length === 0) {
                toast.error('Select at least one user.');
                return;
            }
            if (formData.channel === 'webhook' && selectedTargets.length === 0 && !formSelectedTemplate?.webhook_target) {
                toast.error('Select at least one webhook endpoint.');
                return;
            }

            const advDigestRule = advDigestRuleId ? digestRules.find(r => r.id === advDigestRuleId) : null;
            const isTwilio = isTwilioSelection(formData.template_id || '');
            const twilioTpl = isTwilio ? findTwilioTemplate(formData.template_id!) : undefined;
            const twilioPayload = isTwilio && twilioTpl ? {
                content_sid: twilioTpl.sid,
                content_variables: JSON.stringify(formTwilioVars || {}),
            } : null;
            if (isTwilio && twilioTpl) {
                const vErr = twilioVariablesIncompleteMessage(twilioTpl, formTwilioVars);
                if (vErr) {
                    toast.error(vErr);
                    return;
                }
            }
            const finalData = mergeTwilioData(customData as Record<string, unknown>, twilioPayload);
            const richPayload = !isRichContentEmpty(advRichContent) ? richContentToPayload(advRichContent) : {};
            const payload: NotificationRequest = {
                user_id: '',
                channel: formData.channel,
                priority: formData.priority,
                template_id: isTwilio ? undefined : (formData.template_id || undefined),
                data: finalData,
                scheduled_at: formData.scheduled_at,
                recurrence: formData.recurrence,
                workflow_trigger_id: formData.workflow_trigger_id || undefined,
                metadata: advDigestRule ? { digest_key: advDigestRule.digest_key } : undefined,
                media_url: (formData as any).media_url || undefined,
                // Rich webhook content as top-level fields (backend DTO contract).
                ...richPayload,
            };

            if (formData.channel === 'webhook') {
                const pinned = formSelectedTemplate?.webhook_target || '';
                const targets = selectedTargets.length > 0 ? selectedTargets : (pinned ? [pinned] : []);
                const sendPromises = targets.map((target) =>
                    notificationsAPI.send(apiKey, { ...payload, webhook_target: target })
                );
                await Promise.all(sendPromises);
                prependOptimisticNotifications(
                    targets.map((target) =>
                        buildOptimisticNotification({
                            userId: `webhook:${target}`,
                            channel: payload.channel,
                            priority: payload.priority,
                            templateId: payload.template_id,
                            data: customData,
                            scheduledAt: payload.scheduled_at,
                            metadata: {
                                ...(payload.metadata || {}),
                                webhook_target: target,
                            },
                        })
                    )
                );
            } else if (userIds.length > 1) {
                await notificationsAPI.sendBulk(apiKey, {
                    user_ids: userIds,
                    channel: payload.channel,
                    priority: payload.priority,
                    template_id: payload.template_id,
                    data: payload.data,
                    metadata: payload.metadata,
                    media_url: payload.media_url,
                    // Rich webhook content propagates to bulk recipients identically.
                    ...richPayload,
                });
                prependOptimisticNotifications(
                    userIds.map((uid) =>
                        buildOptimisticNotification({
                            userId: uid,
                            channel: payload.channel,
                            priority: payload.priority,
                            templateId: payload.template_id,
                            data: customData,
                            scheduledAt: payload.scheduled_at,
                            metadata: payload.metadata,
                        })
                    )
                );
            } else {
                const singleUserId = userIds[0] || formData.user_id;
                await notificationsAPI.send(apiKey, { ...payload, user_id: singleUserId });
                prependOptimisticNotifications([
                    buildOptimisticNotification({
                        userId: singleUserId,
                        channel: payload.channel,
                        priority: payload.priority,
                        templateId: payload.template_id,
                        data: customData,
                        scheduledAt: payload.scheduled_at,
                        metadata: payload.metadata,
                    }),
                ]);
            }

            setShowSendForm(false);
            setFormData(createEmptyForm());
            setSelectedUsers([]);
            setSelectedTargets([]);
            setDataInput('');
            setAdvDigestRuleId('');
            setFormTwilioVars({});
            setFormTwilioPreviewOpen(false);
            clearFormMedia();
            toast.success('Notification(s) sent successfully!');
        } catch (error) {
            console.error('Failed to send notification:', error);
            const msg = extractErrorMessage(error, 'Failed to send notification');
            if (msg.includes('phone_verification_required')) {
                toast.error(`${formData.channel === 'whatsapp' ? 'WhatsApp' : 'SMS'} failed: Please verify your phone number in the Profile menu (top left) to use system credentials.`);
            } else {
                toast.error(msg);
            }
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
                        <Button variant="outline" size="sm" onClick={() => setShowBatchSend(true)} title="Upload CSV to send batch notifications">
                            <Send className="h-3.5 w-3.5 mr-1.5" />Batch CSV Upload
                        </Button>
                        <Button
                            size="sm"
                            variant={showSendForm ? "outline" : "default"}
                            onClick={() => setShowSendForm(!showSendForm)}
                        >
                            {showSendForm ? 'Hide Send Form' : 'Create Notification'}
                        </Button>
                    </div>
                </div>
            </CardHeader>
            <CardContent>
                {showSendForm && (
                    <Tabs defaultValue="quick" className="mb-8">
                        <TabsList >
                            <TabsTrigger value="quick">Quick Send</TabsTrigger>
                            <TabsTrigger value="advanced">Bulk Send</TabsTrigger>
                            <TabsTrigger value="broadcast">Broadcast</TabsTrigger>
                        </TabsList>

                        {/* ── Quick Send Tab ── */}
                        <TabsContent value="quick">
                            <div className={SEND_FORM_SHELL_CLASS}>
                                <p className={SEND_FORM_INFO_CLASS}>Send a notification using email or user ID and a template name. No UUIDs required.</p>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="quickTo">
                                            To (Recipient)
                                            {isQuickWebhookLike && <span className="font-normal text-muted-foreground text-xs"> (N/A for webhook channels)</span>}
                                        </Label>
                                        {quickToManual ? (
                                            <>
                                                <Input
                                                    id="quickTo"
                                                    value={isQuickWebhookLike ? '' : quickTo}
                                                    onChange={e => setQuickTo(e.target.value)}
                                                    placeholder={isQuickWebhookLike ? 'Not required for webhook channels' : 'john@example.com or external_id'}
                                                    disabled={isQuickWebhookLike}
                                                />
                                                <button
                                                    type="button"
                                                    className="text-xs text-primary hover:underline"
                                                    onClick={() => { setQuickToManual(false); if (users.length > 0) setQuickTo(users[0].user_id); }}
                                                >
                                                    ← Back to user list
                                                </button>
                                            </>
                                        ) : (
                                            <>
                                                <Select value={isQuickWebhookLike ? '__webhook__' : quickTo} onValueChange={setQuickTo} disabled={isQuickWebhookLike}>
                                                    <SelectTrigger>
                                                        <SelectValue placeholder={isQuickWebhookLike ? 'Not required for webhook channels' : 'Select a user'} />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        {users.map(u => (
                                                            <SelectItem key={u.user_id} value={u.user_id}>
                                                                {u.email}{u.external_id ? ` (${u.external_id})` : ''}
                                                            </SelectItem>
                                                        ))}
                                                    </SelectContent>
                                                </Select>
                                                <button
                                                    type="button"
                                                    className="text-xs text-muted-foreground hover:text-primary hover:underline"
                                                    onClick={() => { setQuickToManual(true); setQuickTo(''); }}
                                                >
                                                    Enter email or ID manually
                                                </button>
                                            </>
                                        )}
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="quickTemplate">Template</Label>
                                        <Select value={quickTemplateId} onValueChange={(value) => {

                                            setQuickTemplateId(value);
                                            setTwilioPreviewOpen(false);
                                            if (isTwilioSelection(value)) {
                                                const tpl = findTwilioTemplate(value);
                                                // Seed quickData with one empty entry per Twilio variable key
                                                // so the variable-input grid picks them up. content_sid is
                                                // injected at send time in handleQuickSend.
                                                if (tpl?.variables) {
                                                    const seed: Record<string, string> = {};
                                                    for (const k of Object.keys(tpl.variables)) seed[k] = '';
                                                    setQuickData(seed);
                                                } else {
                                                    setQuickData({});
                                                }
                                            } else {
                                                // Pre-fill variables with sample_data
                                                const selected = templates.find(t => t.id === value);
                                                if (selected?.channel !== 'whatsapp' && quickMedia?.previewUrl) {
                                                    URL.revokeObjectURL(quickMedia.previewUrl);
                                                    setQuickMedia(null);
                                                }
                                                const sample = getSampleData(selected);
                                                if (Object.keys(sample).length > 0) {
                                                    setQuickData(
                                                        selected?.channel === 'whatsapp' && quickMedia
                                                            ? { ...sample, media_url: quickMedia.url }
                                                            : sample,
                                                    );
                                                } else {
                                                    setQuickData(
                                                        selected?.channel === 'whatsapp' && quickMedia
                                                            ? { media_url: quickMedia.url }
                                                            : {},
                                                    );
                                                }
                                            }
                                        }}>
                                            <SelectTrigger>
                                                <SelectValue placeholder="Select a template" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {(templates || []).map(t => (
                                                    <SelectItem key={t.id} value={t.id}>{t.name} ({t.channel})</SelectItem>
                                                ))}
                                                {twilioApproved.length > 0 && (
                                                    <>
                                                        <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground border-t mt-1 pt-1">Twilio WhatsApp Templates</div>
                                                        {twilioApproved.map(t => (
                                                            <SelectItem key={t.sid} value={`twilio:${t.sid}`}>
                                                                {t.friendly_name} (whatsapp)
                                                            </SelectItem>
                                                        ))}
                                                    </>
                                                )}
                                                <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground border-t mt-1 pt-1">WhatsApp Direct</div>
                                                <SelectItem value="freeform:whatsapp">
                                                    Free-form message (24h window)
                                                </SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                </div>
                                {quickVariables.length > 0 && (
                                    <div className={`${SEND_FORM_SECTION_CLASS} p-4 space-y-3`}>
                                        <div className="flex items-center justify-between">
                                            <span className="text-sm font-medium flex items-center gap-2">
                                                Template Variables
                                                <Badge variant="secondary" className="text-xs">{quickVariables.length}</Badge>
                                                {quickSelectedTemplate?.metadata?.sample_data && (
                                                    <span className="text-xs text-muted-foreground font-normal">• sample data pre-filled</span>
                                                )}
                                            </span>
                                        </div>
                                        {quickVariables.length <= 2 ? (
                                            <>
                                                <p className="text-xs text-muted-foreground">
                                                    Fill variables directly here, or use <strong>Preview</strong> to edit in the preview panel.
                                                </p>
                                                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                    {quickVariables.map((v) => {
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
                                                                        onChange={(e) => setQuickData((d) => ({ ...d, [v]: formatDateForTemplate(e.target.value) }))}
                                                                        className={currentVal ? '' : 'text-muted-foreground'}
                                                                    />
                                                                ) : (
                                                                    <Input
                                                                        value={currentVal}
                                                                        onChange={(e) => setQuickData((d) => ({ ...d, [v]: e.target.value }))}
                                                                        placeholder={sampleVal ? String(sampleVal) : v}
                                                                        className={currentVal ? '' : 'text-muted-foreground'}
                                                                    />
                                                                )}
                                                            </div>
                                                        );
                                                    })}
                                                </div>
                                            </>
                                        ) : (
                                            <p className="text-xs text-muted-foreground">
                                                Fill variable values through the preview panel.
                                            </p>
                                        )}
                                    </div>
                                )}

                                {/* Twilio Content Template variables + inline WhatsApp preview */}
                                {quickSelectedTwilioTemplate && (
                                    <div className={`${SEND_FORM_SECTION_CLASS} p-4 space-y-3`}>
                                        <div className="flex items-center justify-between">
                                            <span className="text-sm font-medium flex items-center gap-2">
                                                Template Variables
                                                {quickTwilioVariableKeys.length > 0 && (
                                                    <Badge variant="secondary" className="text-xs">{quickTwilioVariableKeys.length}</Badge>
                                                )}
                                                <span className="text-xs text-muted-foreground font-normal">• Twilio content template</span>
                                            </span>
                                        </div>
                                        {quickTwilioVariableKeys.length === 0 ? (
                                            <p className="text-xs text-muted-foreground">This template has no variables.</p>
                                        ) : (
                                            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                {quickTwilioVariableKeys.map((k) => {
                                                    const label = quickSelectedTwilioTemplate.variables?.[k] || k;
                                                    const currentVal = quickData[k] || '';
                                                    return (
                                                        <div key={k} className="space-y-1">
                                                            <Label className="text-xs text-muted-foreground">
                                                                <code className="bg-muted px-1 rounded mr-1">{`{{${k}}}`}</code>
                                                                {label !== k && <span>{label}</span>}
                                                            </Label>
                                                            <Input
                                                                value={currentVal}
                                                                onChange={(e) => setQuickData((d) => ({ ...d, [k]: e.target.value }))}
                                                                placeholder={`Value for {{${k}}}`}
                                                                className={currentVal ? '' : 'text-muted-foreground'}
                                                            />
                                                        </div>
                                                    );
                                                })}
                                            </div>
                                        )}
                                        {twilioPreviewOpen && (
                                            <div className="pt-2 flex justify-center">
                                                <WhatsAppPreview
                                                    template={quickSelectedTwilioTemplate}
                                                    variables={quickData}
                                                    header={quickSelectedTwilioTemplate.friendly_name}
                                                />
                                            </div>
                                        )}
                                    </div>
                                )}
                                {/* Free-form WhatsApp message (24h customer service window) */}
                                {isFreeformWhatsApp && (
                                    <div className={`${SEND_FORM_SECTION_CLASS} p-4 space-y-3`}>
                                        <div className="flex items-center justify-between">
                                            <span className="text-sm font-medium flex items-center gap-2">
                                                Free-form WhatsApp Message
                                                <Badge variant="secondary" className="text-xs">24h window</Badge>
                                            </span>
                                        </div>
                                        <p className="text-xs text-muted-foreground">
                                            Send a direct message without a pre-approved template. Only works within 24 hours of the user&apos;s last incoming message.
                                        </p>
                                        <div className="space-y-1">
                                            <Label className="text-xs">Title (optional, shown bold)</Label>
                                            <Input
                                                value={quickFreeformTitle}
                                                onChange={(e) => setQuickFreeformTitle(e.target.value)}
                                                placeholder="e.g. Order Update"
                                                className="h-8 text-sm"
                                            />
                                        </div>
                                        <div className="space-y-1">
                                            <Label className="text-xs">Message Body</Label>
                                            <textarea
                                                className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                                                value={quickFreeformBody}
                                                onChange={(e) => setQuickFreeformBody(e.target.value)}
                                                placeholder="Type your WhatsApp message here..."
                                            />
                                        </div>
                                    </div>
                                )}
                                {/* Media attachment for free-form WhatsApp or non-Twilio WhatsApp templates */}
                                {(isFreeformWhatsApp || quickSelectedTemplate?.channel === 'whatsapp') && (
                                    <div className="space-y-2 border-t border-border/50 pt-4 mt-2">
                                        <Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Image Attachment (optional)</Label>
                                        {!quickMedia && (
                                            <label
                                                htmlFor="quickMediaUploadFreeform"
                                                className="flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-border bg-muted/30 p-5 cursor-pointer hover:bg-muted/50 transition-colors"
                                            >
                                                <UploadCloud className="h-7 w-7 text-muted-foreground" />
                                                <span className="text-sm font-medium">Click to upload image</span>
                                                <span className="text-xs text-muted-foreground">JPEG, PNG, GIF, WebP — max 16 MB</span>
                                                <input
                                                    id="quickMediaUploadFreeform"
                                                    type="file"
                                                    accept="image/jpeg,image/png,image/gif,image/webp"
                                                    className="hidden"
                                                    onChange={async (e) => {
                                                        const file = e.target.files?.[0];
                                                        if (!file) return;
                                                        if (file.size > 16 * 1024 * 1024) {
                                                            toast.error(`${file.name} is too large (max 16 MB)`);
                                                            return;
                                                        }
                                                        const localUrl = URL.createObjectURL(file);
                                                        try {
                                                            toast.info(`Uploading ${file.name}...`);
                                                            const result = await mediaAPI.upload(apiKey, file);
                                                            setQuickMedia(prev => {
                                                                if (prev?.previewUrl) URL.revokeObjectURL(prev.previewUrl);
                                                                return { url: result.url, previewUrl: localUrl, type: file.type, name: file.name };
                                                            });
                                                            setQuickData(d => ({ ...d, media_url: result.url }));
                                                            toast.success('Image uploaded');
                                                        } catch (err) {
                                                            URL.revokeObjectURL(localUrl);
                                                            toast.error(`Upload failed for ${file.name}: ` + extractErrorMessage(err));
                                                        }
                                                    }}
                                                />
                                            </label>
                                        )}
                                        {quickMedia && (
                                            <div className="flex items-center justify-between p-3 rounded-lg border border-border bg-muted/30">
                                                <div className="flex items-center gap-3 min-w-0">
                                                    <img src={quickMedia.previewUrl} alt={quickMedia.name} className="h-12 w-12 rounded object-cover border border-border" />
                                                    <div className="min-w-0">
                                                        <p className="text-sm font-medium truncate">{quickMedia.name}</p>
                                                        <p className="text-xs text-muted-foreground truncate">{quickMedia.url}</p>
                                                    </div>
                                                </div>
                                                <Button
                                                    className="bg-red-400/10 hover:bg-red-400/20 text-red-500 border-red-200"
                                                    variant="outline"
                                                    size="sm"
                                                    onClick={() => {
                                                        URL.revokeObjectURL(quickMedia.previewUrl);
                                                        setQuickMedia(null);
                                                        setQuickData(d => { const next = { ...d }; delete next.media_url; return next; });
                                                    }}
                                                >
                                                    <X className="h-3.5 w-3.5" />
                                                </Button>
                                            </div>
                                        )}
                                    </div>
                                )}
                                {quickSelectedTemplate?.channel === 'webhook' && (
                                    webhooks && Object.keys(webhooks).length > 0 ? (
                                        <div className="space-y-2 border-t border-border/50 pt-4 mt-2">
                                            <Label htmlFor="quickWebhookTarget">Webhook Endpoint</Label>
                                            <Select value={quickWebhookTarget} onValueChange={setQuickWebhookTarget}>
                                                <SelectTrigger>
                                                    <SelectValue placeholder="Select webhook endpoint" />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    {Object.keys(webhooks).map((name) => (
                                                        <SelectItem key={name} value={name}>{name}</SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                            <p className="text-xs text-muted-foreground">The notification will be delivered to this webhook endpoint.</p>
                                        </div>
                                    ) : (
                                        <div className="rounded-md border border-dashed border-border bg-muted/30 p-4 text-center mt-2">
                                            <p className="text-sm text-muted-foreground">No webhook endpoints configured.</p>
                                            <p className="text-xs text-muted-foreground mt-1">Go to the <strong>Providers</strong> tab to add webhook endpoints.</p>
                                        </div>
                                    )
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
                                <div className={SEND_FORM_SECTION_CLASS}>
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
                                                min={nowInTimezone(scheduleTimezone)}
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

                                {/* Rich Content Editor — visible for webhook/discord/slack/teams channels */}
                                {quickSelectedTemplate && ['webhook', 'discord', 'slack', 'teams'].includes(quickSelectedTemplate.channel) && (
                                    <div className="space-y-3 border-t border-border/50 pt-4 mt-2">
                                        <RichContentEditor value={quickRichContent} onChange={setQuickRichContent} />
                                        {!isRichContentEmpty(quickRichContent) && (
                                            <ChannelPreview
                                                channel={quickSelectedTemplate.channel}
                                                content={{
                                                    title: quickSelectedTemplate.name,
                                                    body: quickSelectedTemplate.body?.slice(0, 200),
                                                    ...richContentToPayload(quickRichContent),
                                                }}
                                            />
                                        )}
                                    </div>
                                )}

                                <div className="flex justify-end gap-2">
                                    {quickTemplateId && (
                                        <Button
                                            variant="outline"
                                            disabled={
                                                !quickSelectedTwilioTemplate &&
                                                (!!activePreviews[quickTemplateId]?.loading || !quickTemplateId)
                                            }
                                            onClick={() => {
                                                if (quickSelectedTwilioTemplate) {
                                                    setTwilioPreviewOpen((v) => !v);
                                                } else {
                                                    renderQuickPreview();
                                                }
                                            }}
                                        >
                                            <Eye className="w-4 h-4 mr-1" />
                                            {quickSelectedTwilioTemplate
                                                ? (twilioPreviewOpen ? 'Hide Preview' : 'Preview')
                                                : (activePreviews[quickTemplateId]?.loading ? 'Rendering...' : 'Preview')}
                                        </Button>
                                    )}
                                    <Button onClick={handleQuickSend} disabled={quickSending || (!isQuickWebhookLike && !quickTo) || !quickTemplateId}>
                                        {quickSending ? 'Sending...' : quickScheduledAt ? 'Schedule Notification' : 'Send Notification'}
                                    </Button>
                                </div>
                            </div>
                        </TabsContent>

                        {/* ── Bulk Send Tab ── */}
                        <TabsContent value="advanced">
                            <form onSubmit={handleSendNotification} className={SEND_FORM_SHELL_CLASS}>
                                <p className={`${SEND_FORM_INFO_CLASS} mb-2`}>Send the <strong>same notification</strong> to multiple users at once. Select 2+ recipients to trigger a bulk send.</p>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="recipient">
                                            Recipients (Users)
                                            {WEBHOOK_LIKE_CHANNELS.includes(formData.channel) && <span className="font-normal text-muted-foreground text-xs"> (N/A for webhook channels)</span>}
                                        </Label>
                                        <UserMultiSelect
                                            users={users}
                                            value={WEBHOOK_LIKE_CHANNELS.includes(formData.channel) ? [] : selectedUsers}
                                            onChange={setSelectedUsers}
                                            disabled={WEBHOOK_LIKE_CHANNELS.includes(formData.channel)}
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
                                                if (next !== 'whatsapp') {
                                                    clearFormMedia();
                                                }
                                            }}
                                        >
                                            <SelectTrigger>
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="email">Email</SelectItem>
                                                {/* <SelectItem value="push">Push</SelectItem>
                                                */}
                                                <SelectItem value="sms">SMS</SelectItem>
                                                <SelectItem value="whatsapp">WhatsApp</SelectItem>
                                                <SelectItem value="webhook">Webhook</SelectItem>
                                                <SelectItem value="discord">Discord</SelectItem>
                                                <SelectItem value="slack">Slack</SelectItem>
                                                <SelectItem value="teams">Microsoft Teams</SelectItem>
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
                                                if (isTwilioSelection(value)) {
                                                    const tpl = findTwilioTemplate(value);
                                                    setFormData({ ...formData, template_id: value });
                                                    // Seed Twilio var inputs; clear JSON data block
                                                    const seed: Record<string, string> = {};
                                                    if (tpl?.variables) {
                                                        for (const k of Object.keys(tpl.variables)) seed[k] = '';
                                                    }
                                                    setFormTwilioVars(seed);
                                                    setFormTwilioPreviewOpen(false);
                                                    setDataInput('');
                                                } else {
                                                    setFormData({ ...formData, template_id: value });
                                                    setFormTwilioVars({});
                                                    setFormTwilioPreviewOpen(false);
                                                    const selected = templates.find(t => t.id === value);
                                                    const sample = getSampleData(selected);
                                                    if (Object.keys(sample).length > 0) {
                                                        setDataInput(JSON.stringify(sample, null, 2));
                                                    } else {
                                                        setDataInput('');
                                                    }
                                                }
                                            }}
                                        >
                                            <SelectTrigger>
                                                <SelectValue placeholder="Select a template" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {filteredTemplates.map(t => (
                                                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                                                ))}
                                                {formData.channel === 'whatsapp' && twilioApproved.length > 0 && (
                                                    <>
                                                        {filteredTemplates.length > 0 && <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground border-t mt-1 pt-1">Twilio Content Templates</div>}
                                                        {filteredTemplates.length === 0 && <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground">Twilio Content Templates</div>}
                                                        {twilioApproved.map(t => (
                                                            <SelectItem key={t.sid} value={`twilio:${t.sid}`}>
                                                                {t.friendly_name} (Twilio)
                                                            </SelectItem>
                                                        ))}
                                                    </>
                                                )}
                                                {filteredTemplates.length === 0 && (formData.channel !== 'whatsapp' || twilioApproved.length === 0) && (
                                                    <div className="px-2 py-3 text-sm text-muted-foreground text-center">No templates for {formData.channel}</div>
                                                )}
                                            </SelectContent>
                                        </Select>
                                        {filteredTemplates.length === 0 && (formData.channel !== 'whatsapp' || twilioApproved.length === 0) && (
                                            <p className="text-xs text-amber-600">No templates found for the &quot;{formData.channel}&quot; channel. Create one in the Templates tab.</p>
                                        )}
                                    </div>

                                    {formVariables.length > 0 && (
                                        <div className="space-y-2 md:col-span-2">
                                            <div className={`${SEND_FORM_SECTION_CLASS} p-4 space-y-3`}>
                                                <div className="flex items-center gap-2">
                                                    <span className="text-sm font-medium">Template Variables</span>
                                                    <Badge variant="secondary" className="text-xs">{formVariables.length}</Badge>
                                                </div>
                                                {formVariables.length <= 2 ? (
                                                    <>
                                                        <p className="text-xs text-muted-foreground">Fill variables directly here, or use <strong>Preview</strong> to edit in the preview panel.</p>
                                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                            {formVariables.map((v) => {
                                                                const currentVal = parsedFormData[v] || '';
                                                                const isDateVar = v.toLowerCase() === 'date';
                                                                return (
                                                                    <div key={v} className="space-y-1">
                                                                        <Label className="text-xs text-muted-foreground">{v}</Label>
                                                                        {isDateVar ? (
                                                                            <Input
                                                                                type="date"
                                                                                value={toISODateOnly(currentVal)}
                                                                                onChange={(e) => updateFormDataVariable(v, formatDateForTemplate(e.target.value))}
                                                                            />
                                                                        ) : (
                                                                            <Input
                                                                                value={currentVal}
                                                                                onChange={(e) => updateFormDataVariable(v, e.target.value)}
                                                                                placeholder={v}
                                                                            />
                                                                        )}
                                                                    </div>
                                                                );
                                                            })}
                                                        </div>
                                                    </>
                                                ) : (
                                                    <p className="text-xs text-muted-foreground">This template has more than 2 variables. Fill values through the <strong>Preview</strong> panel.</p>
                                                )}
                                            </div>
                                        </div>
                                    )}

                                    {/* Twilio Content Template variables + inline WhatsApp preview */}
                                    {formSelectedTwilioTemplate && (
                                        <div className="space-y-2 md:col-span-2">
                                            <div className={`${SEND_FORM_SECTION_CLASS} p-4 space-y-3`}>
                                                <div className="flex items-center justify-between">
                                                    <span className="text-sm font-medium flex items-center gap-2">
                                                        Template Variables
                                                        {formTwilioVariableKeys.length > 0 && (
                                                            <Badge variant="secondary" className="text-xs">{formTwilioVariableKeys.length}</Badge>
                                                        )}
                                                        <span className="text-xs text-muted-foreground font-normal">• Twilio content template</span>
                                                    </span>
                                                </div>
                                                {formTwilioVariableKeys.length === 0 ? (
                                                    <p className="text-xs text-muted-foreground">This template has no variables.</p>
                                                ) : (
                                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                        {formTwilioVariableKeys.map((k) => {
                                                            const label = formSelectedTwilioTemplate.variables?.[k] || k;
                                                            const currentVal = formTwilioVars[k] || '';
                                                            return (
                                                                <div key={k} className="space-y-1">
                                                                    <Label className="text-xs text-muted-foreground">
                                                                        <code className="bg-muted px-1 rounded mr-1">{`{{${k}}}`}</code>
                                                                        {label !== k && <span>{label}</span>}
                                                                    </Label>
                                                                    <Input
                                                                        value={currentVal}
                                                                        onChange={(e) => setFormTwilioVars((d) => ({ ...d, [k]: e.target.value }))}
                                                                        placeholder={`Value for {{${k}}}`}
                                                                        className={currentVal ? '' : 'text-muted-foreground'}
                                                                    />
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                )}
                                                {formTwilioPreviewOpen && (
                                                    <div className="pt-2 flex justify-center">
                                                        <WhatsAppPreview
                                                            template={formSelectedTwilioTemplate}
                                                            variables={formTwilioVars}
                                                            header={formSelectedTwilioTemplate.friendly_name}
                                                        />
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                    )}

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

                                    {/* WhatsApp Media Upload */}
                                    {formData.channel === 'whatsapp' && (
                                        <div className="space-y-4 md:col-span-2">
                                            <div className="space-y-2">
                                                <Label>Media Attachment (optional)</Label>
                                                <div className="space-y-3">
                                                    {mediaFiles.length < 5 && (
                                                        <label
                                                            htmlFor="mediaUpload"
                                                            className="flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-border bg-muted/30 p-6 cursor-pointer hover:bg-muted/50 transition-colors"
                                                        >
                                                            <UploadCloud className="h-8 w-8 text-muted-foreground" />
                                                            <span className="text-sm font-medium">Click to upload image or file</span>
                                                            <span className="text-xs text-muted-foreground">
                                                                JPEG, PNG, GIF, WebP, PDF, MP4 — max 16 MB (Limit: 5 files)
                                                            </span>
                                                            <input
                                                                multiple
                                                                id="mediaUpload"
                                                                type="file"
                                                                accept="image/jpeg,image/png,image/gif,image/webp,application/pdf,video/mp4"
                                                                className="hidden"
                                                                onChange={async (e) => {
                                                                    await handleFormMediaUpload(Array.from(e.target.files || []));
                                                                }}
                                                            />
                                                        </label>
                                                    )}

                                                    {mediaFiles.map((file, idx) => (
                                                        <div key={idx} className="flex items-center justify-between p-3 rounded-lg border border-border bg-muted/30">
                                                            <div className="flex items-center gap-3">
                                                                <div className="p-2 bg-primary/10 rounded">
                                                                    <UploadCloud className="h-4 w-4 text-primary" />
                                                                </div>
                                                                <div className="min-w-0">
                                                                    <p className="text-sm font-medium truncate max-w-[150px]">{file.name}</p>
                                                                    <p className="text-xs text-muted-foreground truncate max-w-[200px]">{file.url}</p>
                                                                </div>
                                                            </div>
                                                            <Button
                                                                className='bg-red-400/10 hover:bg-red-400/20 text-red-500 border-red-200'
                                                                variant="outline"
                                                                size="sm"
                                                                onClick={() => removeFormMediaAt(idx)}
                                                            >
                                                                <X className="h-3.5 w-3.5" />
                                                            </Button>
                                                        </div>
                                                    ))}
                                                </div>
                                            </div>
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

                                {/* Schedule toggle */}
                                <div className={SEND_FORM_SECTION_CLASS}>
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
                                                    min={nowInTimezone(scheduleTimezone)}
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
                                                                    min={nowInTimezone(scheduleTimezone)}
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

                                {/* Rich Content Editor — visible for webhook/discord/slack/teams channels */}
                                {['webhook', 'discord', 'slack', 'teams'].includes(formData.channel) && (
                                    <div className="space-y-3 border-t border-border/50 pt-4 mt-2">
                                        <RichContentEditor value={advRichContent} onChange={setAdvRichContent} />
                                    </div>
                                )}

                                <div className="flex justify-end mt-6">
                                    <div className="flex gap-2">
                                        <Button
                                            type="button"
                                            variant="outline"
                                            onClick={() => {
                                                if (formSelectedTwilioTemplate) {
                                                    setFormTwilioPreviewOpen((v) => !v);
                                                } else {
                                                    renderSendPreview('advanced');
                                                }
                                            }}
                                            disabled={
                                                !formSelectedTwilioTemplate &&
                                                (!!activePreviews[formData.template_id || '']?.loading || !formData.template_id)
                                            }
                                        >
                                            <Eye className="h-4 w-4 mr-1" />
                                            {formSelectedTwilioTemplate
                                                ? (formTwilioPreviewOpen ? 'Hide Preview' : 'Preview')
                                                : ((formData.template_id && activePreviews[formData.template_id]?.loading) ? 'Rendering...' : 'Preview')}
                                        </Button>
                                        <Button
                                            type="button"
                                            variant="outline"
                                            onClick={() => {
                                                setFormData(createEmptyForm());
                                                setSelectedUsers([]);
                                                setSelectedTargets([]);
                                                setDataInput('');
                                                setFormTwilioVars({});
                                                setFormTwilioPreviewOpen(false);
                                                clearFormMedia();
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
                            <Card className="border-border/80 bg-card/40">
                                <CardHeader>
                                    <CardTitle className="text-lg">Broadcast to All Users</CardTitle>
                                    <p className="text-sm text-muted-foreground mt-1">This sends a notification to all users in this application.</p>
                                </CardHeader>
                                <CardContent>
                                    <form onSubmit={handleBroadcastSubmit} className="space-y-4">
                                        <div className="rounded-md border border-amber-300/60 bg-amber-100/40 px-3 py-2 text-xs text-amber-900 dark:text-amber-200">
                                            Broadcast impacts every recipient matched by this app or topic filter.
                                        </div>
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
                                                        if (next !== 'whatsapp') {
                                                            clearFormMedia();
                                                        }
                                                    }}
                                                >
                                                    <SelectTrigger id="broadcastChannel">
                                                        <SelectValue />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="email">Email</SelectItem>
                                                        {/* <SelectItem value="push">Push</SelectItem>
                                                        */}
                                                        <SelectItem value="sms">SMS</SelectItem>
                                                        <SelectItem value="whatsapp">WhatsApp</SelectItem>
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
                                                        if (isTwilioSelection(val)) {
                                                            const tpl = findTwilioTemplate(val);
                                                            setFormData({ ...formData, template_id: val });
                                                            const seed: Record<string, string> = {};
                                                            if (tpl?.variables) {
                                                                for (const k of Object.keys(tpl.variables)) seed[k] = '';
                                                            }
                                                            setFormTwilioVars(seed);
                                                            setFormTwilioPreviewOpen(false);
                                                            setDataInput('');
                                                            return;
                                                        }
                                                        setFormData({ ...formData, template_id: val });
                                                        setFormTwilioVars({});
                                                        setFormTwilioPreviewOpen(false);
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
                                                        {filteredTemplates.length === 0 && !(formData.channel === 'whatsapp' && twilioApproved.length > 0) ? (
                                                            <div className="px-2 py-3 text-sm text-muted-foreground text-center">No templates for {formData.channel}</div>
                                                        ) : (
                                                            filteredTemplates.map(t => (
                                                                <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                                                            ))
                                                        )}
                                                        {formData.channel === 'whatsapp' && twilioApproved.length > 0 && (
                                                            <>
                                                                <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground border-t mt-1 pt-1">Twilio WhatsApp Templates</div>
                                                                {twilioApproved.map(t => (
                                                                    <SelectItem key={t.sid} value={`twilio:${t.sid}`}>
                                                                        {t.friendly_name}
                                                                    </SelectItem>
                                                                ))}
                                                            </>
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

                                        {formVariables.length > 0 && (
                                            <div className={`${SEND_FORM_SECTION_CLASS} p-4 space-y-3`}>
                                                <div className="flex items-center gap-2">
                                                    <span className="text-sm font-medium">Template Variables</span>
                                                    <Badge variant="secondary" className="text-xs">{formVariables.length}</Badge>
                                                </div>
                                                {formVariables.length <= 2 ? (
                                                    <>
                                                        <p className="text-xs text-muted-foreground">Fill variables directly here, or use <strong>Preview</strong> to edit in the preview panel.</p>
                                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                            {formVariables.map((v) => {
                                                                const currentVal = parsedFormData[v] || '';
                                                                const isDateVar = v.toLowerCase() === 'date';
                                                                return (
                                                                    <div key={v} className="space-y-1">
                                                                        <Label className="text-xs text-muted-foreground">{v}</Label>
                                                                        {isDateVar ? (
                                                                            <Input
                                                                                type="date"
                                                                                value={toISODateOnly(currentVal)}
                                                                                onChange={(e) => updateFormDataVariable(v, formatDateForTemplate(e.target.value))}
                                                                            />
                                                                        ) : (
                                                                            <Input
                                                                                value={currentVal}
                                                                                onChange={(e) => updateFormDataVariable(v, e.target.value)}
                                                                                placeholder={v}
                                                                            />
                                                                        )}
                                                                    </div>
                                                                );
                                                            })}
                                                        </div>
                                                    </>
                                                ) : (
                                                    <p className="text-xs text-muted-foreground">This template has more than 2 variables. Fill values through the <strong>Preview</strong> panel.</p>
                                                )}
                                            </div>
                                        )}

                                        {/* Twilio Content Template variables + inline WhatsApp preview */}
                                        {formSelectedTwilioTemplate && (
                                            <div className={`${SEND_FORM_SECTION_CLASS} p-4 space-y-3`}>
                                                <div className="flex items-center justify-between">
                                                    <span className="text-sm font-medium flex items-center gap-2">
                                                        Template Variables
                                                        {formTwilioVariableKeys.length > 0 && (
                                                            <Badge variant="secondary" className="text-xs">{formTwilioVariableKeys.length}</Badge>
                                                        )}
                                                        <span className="text-xs text-muted-foreground font-normal">• Twilio content template</span>
                                                    </span>
                                                </div>
                                                {formTwilioVariableKeys.length === 0 ? (
                                                    <p className="text-xs text-muted-foreground">This template has no variables.</p>
                                                ) : (
                                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                                        {formTwilioVariableKeys.map((k) => {
                                                            const label = formSelectedTwilioTemplate.variables?.[k] || k;
                                                            const currentVal = formTwilioVars[k] || '';
                                                            return (
                                                                <div key={k} className="space-y-1">
                                                                    <Label className="text-xs text-muted-foreground">
                                                                        <code className="bg-muted px-1 rounded mr-1">{`{{${k}}}`}</code>
                                                                        {label !== k && <span>{label}</span>}
                                                                    </Label>
                                                                    <Input
                                                                        value={currentVal}
                                                                        onChange={(e) => setFormTwilioVars((d) => ({ ...d, [k]: e.target.value }))}
                                                                        placeholder={`Value for {{${k}}}`}
                                                                        className={currentVal ? '' : 'text-muted-foreground'}
                                                                    />
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                )}
                                                {formTwilioPreviewOpen && (
                                                    <div className="pt-2 flex justify-center">
                                                        <WhatsAppPreview
                                                            template={formSelectedTwilioTemplate}
                                                            variables={formTwilioVars}
                                                            header={formSelectedTwilioTemplate.friendly_name}
                                                        />
                                                    </div>
                                                )}
                                            </div>
                                        )}

                                        {formData.channel === 'whatsapp' && (
                                            <div className="space-y-4">
                                                <div className="space-y-2">
                                                    <Label>Media Attachment (optional)</Label>
                                                    <div className="space-y-3">
                                                        {mediaFiles.length < 5 && (
                                                            <label
                                                                htmlFor="broadcastMediaUpload"
                                                                className="flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-border bg-muted/30 p-6 cursor-pointer hover:bg-muted/50 transition-colors"
                                                            >
                                                                <UploadCloud className="h-8 w-8 text-muted-foreground" />
                                                                <span className="text-sm font-medium">Click to upload image or file</span>
                                                                <span className="text-xs text-muted-foreground">JPEG, PNG, GIF, WebP, PDF, MP4 — max 16 MB (Limit: 5 files)</span>
                                                                <input
                                                                    multiple
                                                                    id="broadcastMediaUpload"
                                                                    type="file"
                                                                    accept="image/jpeg,image/png,image/gif,image/webp,application/pdf,video/mp4"
                                                                    className="hidden"
                                                                    onChange={async (e) => {
                                                                        await handleFormMediaUpload(Array.from(e.target.files || []));
                                                                    }}
                                                                />
                                                            </label>
                                                        )}

                                                        {mediaFiles.map((file, idx) => (
                                                            <div key={idx} className="flex items-center justify-between p-3 rounded-lg border border-border bg-muted/30">
                                                                <div className="flex items-center gap-3">
                                                                    <div className="p-2 bg-primary/10 rounded">
                                                                        <UploadCloud className="h-4 w-4 text-primary" />
                                                                    </div>
                                                                    <div className="min-w-0">
                                                                        <p className="text-sm font-medium truncate max-w-[150px]">{file.name}</p>
                                                                        <p className="text-xs text-muted-foreground truncate max-w-[200px]">{file.url}</p>
                                                                    </div>
                                                                </div>
                                                                <Button
                                                                    className="bg-red-400/10 hover:bg-red-400/20 text-red-500 border-red-200"
                                                                    variant="outline"
                                                                    size="sm"
                                                                    onClick={() => removeFormMediaAt(idx)}
                                                                >
                                                                    <X className="h-3.5 w-3.5" />
                                                                </Button>
                                                            </div>
                                                        ))}
                                                    </div>
                                                </div>
                                            </div>
                                        )}

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

                                        {/* Schedule toggle */}
                                        <div className={SEND_FORM_SECTION_CLASS}>
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
                                                        min={nowInTimezone(scheduleTimezone)}
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
                                            <div className="flex gap-2">
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    onClick={() => {
                                                        if (formSelectedTwilioTemplate) {
                                                            setFormTwilioPreviewOpen((v) => !v);
                                                        } else {
                                                            renderSendPreview('broadcast');
                                                        }
                                                    }}
                                                    disabled={
                                                        !formSelectedTwilioTemplate &&
                                                        (!!activePreviews[formData.template_id || '']?.loading || !formData.template_id)
                                                    }
                                                >
                                                    <Eye className="h-4 w-4 mr-1" />
                                                    {formSelectedTwilioTemplate
                                                        ? (formTwilioPreviewOpen ? 'Hide Preview' : 'Preview')
                                                        : ((formData.template_id && activePreviews[formData.template_id]?.loading) ? 'Rendering...' : 'Preview')}
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    className="bg-orange-600 hover:bg-orange-700 text-white"
                                                    disabled={isSubmitting}
                                                >
                                                    {isSubmitting ? 'Processing...' : 'Send Broadcast'}
                                                </Button>
                                            </div>
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
                                {/* <SelectItem value="push">Push</SelectItem>
                                */}
                                <SelectItem value="sms">SMS</SelectItem>
                                <SelectItem value="whatsapp">WhatsApp</SelectItem>
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
                            {/* Bulk Actions Bar — always visible */}
                            <div className="flex items-center gap-2 px-3 py-2 mb-2 bg-muted/60 border border-border rounded-lg">
                                <span className="text-sm font-medium text-foreground mr-2">
                                    {selectedIds.size > 0 ? `${selectedIds.size} selected` : 'Select rows to enable bulk actions'}
                                </span>
                                <Button variant="outline" size="sm" onClick={handleBulkMarkRead} disabled={bulkActing || selectedIds.size === 0}>
                                    <CheckSquare className="h-3.5 w-3.5 mr-1.5" />
                                    Mark Read
                                </Button>
                                <Button variant="outline" size="sm" onClick={handleBulkArchive} disabled={bulkActing || selectedIds.size === 0}>
                                    <Archive className="h-3.5 w-3.5 mr-1.5" />
                                    Bulk Archive
                                </Button>
                                <Button variant="outline" size="sm" onClick={handleCancelSelected} disabled={bulkActing || selectedIds.size === 0} className="text-red-600 hover:text-red-700 hover:bg-red-50">
                                    <XCircle className="h-3.5 w-3.5 mr-1.5" />
                                    Cancel Batch
                                </Button>
                                {selectedIds.size > 0 && (
                                    <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
                                        <X className="h-3.5 w-3.5 mr-1" />
                                        Clear
                                    </Button>
                                )}
                            </div>

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
                <VerifyPhoneDialog
                    open={isVerifyDialogOpen}
                    onOpenChange={setIsVerifyDialogOpen}
                />
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
                                        notificationsAPI.markRead(apiKey, { notification_ids: [detailNotif.notification_id], user_id: detailNotif.user_id })
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
                                        notificationsAPI.bulkArchive(apiKey, { notification_ids: [detailNotif.notification_id], user_id: detailNotif.user_id })
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

                <EditablePreviewPanel
                    slidePreview={slidePreview}
                    templates={templates}
                    activePreviews={activePreviews}
                    savingDefaults={{}}
                    showDefaultActions={false}
                    onClose={() => {
                        setSlidePreview(null);
                        setPreviewSource(null);
                    }}
                    onRenderPreview={handleRenderPreview}
                    onSaveDefaults={() => { }}
                    onResetDefaults={() => { }}
                    onVariableEdit={updatePreviewVariable}
                />

                {/* Batch Send Dialog */}
                <Dialog open={showBatchSend} onOpenChange={(open) => {
                    setShowBatchSend(open);
                    if (!open) {
                        setBatchCsvFile(null);
                        setBatchRows([]);
                    }
                }}>
                    <DialogContent className="max-w-3xl max-h-[90vh] flex flex-col">
                        <DialogHeader>
                            <div className="flex items-center justify-between pr-6">
                                <div>
                                    <DialogTitle>Send Batch Notifications</DialogTitle>
                                    <DialogDescription>
                                        Upload a CSV file to dispatch multiple personalized notifications.
                                    </DialogDescription>
                                </div>
                                <Button variant="outline" size="sm" onClick={downloadSampleCSV} className="text-xs">
                                    <Download className="h-3 w-3 mr-1.5" />
                                    Sample CSV
                                </Button>
                            </div>
                        </DialogHeader>

                        <div className="flex-1 overflow-y-auto py-2 space-y-4 pr-1">
                            {!batchCsvFile ? (
                                <div
                                    className={`relative rounded-xl border-2 border-dashed transition-all duration-200 p-12 flex flex-col items-center justify-center text-center group cursor-pointer ${dragActive ? 'border-primary bg-primary/5' : 'border-muted-foreground/20 bg-muted/30 hover:bg-muted/50 hover:border-muted-foreground/40'}`}
                                    onDragOver={(e) => { e.preventDefault(); setDragActive(true); }}
                                    onDragLeave={(e) => { e.preventDefault(); setDragActive(false); }}
                                    onDrop={onDrop}
                                    onClick={() => document.getElementById('batch-file-input')?.click()}
                                >
                                    <Input
                                        id="batch-file-input"
                                        type="file"
                                        accept=".csv"
                                        onChange={(e) => {
                                            const file = e.target.files?.[0];
                                            if (file) handleFile(file);
                                        }}
                                        className="hidden"
                                    />
                                    <div className="w-16 h-16 rounded-full bg-background border border-border flex items-center justify-center mb-4 group-hover:scale-110 transition-transform duration-200">
                                        <UploadCloud className={`h-8 w-8 ${dragActive ? 'text-primary' : 'text-muted-foreground'}`} />
                                    </div>
                                    <div className="space-y-1">
                                        <p className="text-sm font-medium">Click to upload or drag and drop</p>
                                        <p className="text-xs text-muted-foreground">Standard CSV files only</p>
                                    </div>
                                </div>
                            ) : (
                                <div className="space-y-4">
                                    <div className="flex items-center justify-between bg-muted/40 p-3 rounded-lg border border-border">
                                        <div className="flex items-center gap-3">
                                            <div className="p-2 bg-background rounded border border-border">
                                                <FileText className="h-5 w-5 text-primary" />
                                            </div>
                                            <div className="min-w-0">
                                                <p className="text-sm font-medium truncate">{batchCsvFile.name}</p>
                                                <p className="text-xs text-muted-foreground">{(batchCsvFile.size / 1024).toFixed(1)} KB • {batchRows.length} rows detected</p>
                                            </div>
                                        </div>
                                        <Button variant="ghost" size="sm" onClick={() => { setBatchCsvFile(null); setBatchRows([]); }} className="h-8 w-8 p-0">
                                            <X className="h-4 w-4" />
                                        </Button>
                                    </div>

                                    {batchRows.length > 0 && (
                                        <div className="space-y-2">
                                            <div className="flex items-center justify-between">
                                                <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Data Preview (First 5 rows)</h4>
                                                <Badge variant="outline" className="text-[10px] font-normal">
                                                    Showing {Math.min(5, batchRows.length)} of {batchRows.length}
                                                </Badge>
                                            </div>
                                            <div className="rounded-lg border border-border overflow-hidden">
                                                <Table>
                                                    <TableHeader className="bg-muted/50">
                                                        <TableRow className="hover:bg-transparent">
                                                            {Object.keys(batchRows[0]).slice(0, 4).map(k => (
                                                                <TableHead key={k} className="h-9 px-3 text-[11px] font-bold">{k}</TableHead>
                                                            ))}
                                                            {Object.keys(batchRows[0]).length > 4 && <TableHead className="h-9 px-3 text-[11px] font-bold">...</TableHead>}
                                                        </TableRow>
                                                    </TableHeader>
                                                    <TableBody>
                                                        {batchRows.slice(0, 5).map((row, i) => (
                                                            <TableRow key={i} className="hover:bg-transparent border-b last:border-0">
                                                                {Object.keys(row).slice(0, 4).map(k => (
                                                                    <TableCell key={k} className="py-2 px-3 text-xs max-w-[120px] truncate">{String(row[k] || '-')}</TableCell>
                                                                ))}
                                                                {Object.keys(row).length > 4 && <TableCell className="py-2 px-3 text-xs">...</TableCell>}
                                                            </TableRow>
                                                        ))}
                                                    </TableBody>
                                                </Table>
                                            </div>
                                        </div>
                                    )}

                                    {!batchRows[0]?.user_id && batchRows.length > 0 && (
                                        <div className="flex items-start gap-2 p-3 bg-red-50 border border-red-100 rounded-lg dark:bg-red-950/20 dark:border-red-900/30">
                                            <AlertCircle className="h-4 w-4 text-red-600 mt-0.5 shrink-0" />
                                            <p className="text-xs text-red-700 dark:text-red-400">
                                                Column <code className="bg-red-100 dark:bg-red-900/40 px-1 rounded">user_id</code> is missing or empty. Please check your CSV.
                                            </p>
                                        </div>
                                    )}
                                </div>
                            )}

                            <div className="bg-muted/30 p-4 rounded-xl text-[11px] space-y-2 border border-border">
                                <p className="font-semibold text-foreground/80 flex items-center gap-1.5">
                                    <FileText className="h-3 w-3" />
                                    CSV Format Guide
                                </p>
                                <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-muted-foreground">
                                    <p><span className="text-foreground">Required:</span> user_id, channel</p>
                                    <p><span className="text-foreground">Content:</span> template_id OR (title & body)</p>
                                    <p><span className="text-foreground">Optional:</span> priority, scheduled_at</p>
                                    <p><span className="text-foreground">Variables:</span> Any other columns</p>
                                </div>
                            </div>
                        </div>

                        <div className="flex justify-end gap-3 pt-4 border-t border-border bg-background mt-auto">
                            <Button variant="outline" onClick={() => setShowBatchSend(false)}>Cancel</Button>
                            <Button
                                onClick={handleBatchSend}
                                disabled={batchSending || !batchCsvFile || (batchRows.length > 0 && !batchRows[0].user_id)}
                                className="min-w-[120px]"
                            >
                                {batchSending ? 'Processing...' : (
                                    <>
                                        <Send className="h-3.5 w-3.5 mr-2" />
                                        Send {batchRows.length > 0 ? `${batchRows.length} Notifications` : 'Batch'}
                                    </>
                                )}
                            </Button>
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
    disabled?: boolean;
}> = ({ users, value, onChange, disabled }) => {
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
                    <Button variant="outline" type="button" className="w-full justify-between h-9 rounded-lg" disabled={disabled}>
                        <span className="truncate text-left">{disabled ? 'Not required for webhook channels' : selectorLabel}</span>
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
                        <div className="max-h-72 overflow-y-auto rounded-lg border border-border bg-card/30">
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
                                    <span key={user.user_id} className="inline-flex items-center gap-2 rounded-full border border-border bg-muted/60 px-3 py-1 text-xs text-foreground">
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
                    <button type="button" className="hover:text-foreground underline-offset-2 hover:underline" onClick={() => onChange(targets)}>Select all</button>
                    <button type="button" className="hover:text-foreground underline-offset-2 hover:underline" onClick={() => onChange([])}>Clear</button>
                </div>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-2 p-3 border border-border rounded-lg bg-card/30">
                {targets.map(name => (
                    <div key={name} className="flex items-center gap-2 rounded-md border border-border/70 bg-background px-2.5 py-2">
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

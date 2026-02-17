import React, { useEffect, useState } from 'react';
import { notificationsAPI, usersAPI, templatesAPI } from '../services/api';
import { type Notification, type NotificationRequest, type User, type Template, TemplateChannel, NotificationPriority } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
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
import { Switch } from './ui/switch';
import { toast } from 'sonner';

interface AppNotificationsProps {
    appId: string;
    apiKey: string;
    webhooks?: Record<string, string>;
}

function resetForm(): NotificationRequest {
    return {
        user_id: '',
        channel: TemplateChannel.EMAIL,
        priority: NotificationPriority.NORMAL,
        title: '',
        body: '',
        template_id: '',
        webhook_url: '',
        data: {},
        scheduled_at: undefined,
        recurrence: undefined
    };
}

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

    const fetchData = async () => {
        setLoading(true);
        try {
            const [notifsData, usersData, templatesData] = await Promise.all([
                notificationsAPI.list(apiKey).catch(e => { console.error(e); return [] as Notification[]; }),
                usersAPI.list(apiKey).catch(e => { console.error(e); return [] as User[]; }),
                templatesAPI.list(apiKey).catch(e => { console.error(e); return [] as Template[]; })
            ]);
            setNotifications(notifsData || []);
            setUsers(usersData || []);
            setTemplates(templatesData || []);
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
    }, [apiKey]);

    return { notifications, users, templates, loading, refresh: fetchData };
};

const AppNotifications: React.FC<AppNotificationsProps> = ({ apiKey, webhooks }) => {
    const { notifications, users, templates, loading, refresh } = useNotificationData(apiKey);
    const [isSendDialogOpen, setIsSendDialogOpen] = useState(false);
    const [sendToAllUsers, setSendToAllUsers] = useState(false);
    const [formData, setFormData] = useState<NotificationRequest>(resetForm());
    const [selectedTargets, setSelectedTargets] = useState<string[]>([]);
    const [selectedUsers, setSelectedUsers] = useState<string[]>([]);
    const [dataInput, setDataInput] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleSendNotification = async (e: React.FormEvent) => {
        e.preventDefault();
        if (isSubmitting) return;
        setIsSubmitting(true);
        try {
            const customData = parseCustomData(dataInput);
            if (customData === null) {
                toast.error('Invalid JSON in custom data');
                return;
            }

            if (sendToAllUsers) {
                await notificationsAPI.broadcast(apiKey, {
                    channel: formData.channel,
                    priority: formData.priority,
                    title: formData.title,
                    body: formData.body,
                    template_id: formData.template_id || undefined,
                    data: customData,
                    scheduled_at: formData.scheduled_at
                });
                toast.success('Broadcast initiated successfully.');
            } else {
                const userIds = selectedUsers.length > 0 ? selectedUsers : (formData.user_id ? [formData.user_id] : []);
                const requiresUser = formData.channel !== 'webhook';
                if (requiresUser && userIds.length === 0) {
                    toast.error('Select at least one user.');
                    return;
                }

                if (formData.channel === 'webhook' && selectedTargets.length > 0) {
                    const sendPromises = selectedTargets.map(target =>
                        notificationsAPI.send(apiKey, {
                            ...formData,
                            webhook_target: target,
                            data: customData
                        })
                    );
                    await Promise.all(sendPromises);
                } else if (userIds.length > 1) {
                    const sendPromises = userIds.map(userId =>
                        notificationsAPI.send(apiKey, {
                            ...formData,
                            user_id: userId,
                            data: customData
                        })
                    );
                    await Promise.all(sendPromises);
                } else {
                    await notificationsAPI.send(apiKey, { ...formData, user_id: userIds[0] || formData.user_id, data: customData });
                }

                toast.success('Notification(s) sent successfully!');
            }

            setIsSendDialogOpen(false);
            handleReset();
            refresh();
        } catch (error) {
            console.error('Failed to send notification:', error);
            toast.error('Failed to send notification');
        } finally {
            setIsSubmitting(false);
        }
    };

    function handleReset() {
        setFormData(resetForm());
        setSelectedUsers([]);
        setSelectedTargets([]);
        setDataInput('');
        setSendToAllUsers(false);
    }

    function handleChannelChange(value: string) {
        const next = value as any;
        setFormData({
            ...formData,
            channel: next,
            webhook_url: next === 'webhook' ? formData.webhook_url : ''
        });
        if (next !== 'webhook') {
            setSelectedTargets([]);
        }
    }

    function applyTemplateSelection(templateId: string | undefined) {
        const selectedTemplate = (templates || []).find(t => t.id === templateId);
        setFormData(prev => ({
            ...prev,
            template_id: templateId || '',
            channel: selectedTemplate?.channel ? (selectedTemplate.channel as any) : prev.channel
        }));
        if (selectedTemplate?.channel && selectedTemplate.channel !== TemplateChannel.WEBHOOK) {
            setSelectedTargets([]);
        }
    }

    const getStatusBadgeClass = (status: string) => {
        switch (status?.toLowerCase()) {
            case 'sent': return 'bg-green-100 text-green-700 border-green-300';
            case 'failed': return 'bg-red-100 text-red-700 border-red-300';
            case 'pending': return 'bg-yellow-100 text-yellow-700 border-yellow-300';
            case 'queued': return 'bg-blue-100 text-blue-700 border-blue-300';
            case 'delivered': return 'bg-teal-100 text-teal-700 border-teal-300';
            default: return 'bg-gray-100 text-gray-700 border-gray-300';
        }
    };

    if (loading) return <div className="flex justify-center py-4">Loading notifications...</div>;

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle>Notification History</CardTitle>
                    <div className="flex flex-wrap items-center gap-3">
                        <Dialog open={isSendDialogOpen} onOpenChange={setIsSendDialogOpen}>
                            <DialogTrigger asChild>
                                <Button variant="default">Send Notification</Button>
                            </DialogTrigger>
                            <DialogContent className="max-w-[90vw] sm:max-w-6xl">
                                <DialogHeader>
                                    <DialogTitle>Send Notification</DialogTitle>
                                    <DialogDescription>
                                        Choose recipients, content, and delivery options.
                                    </DialogDescription>
                                </DialogHeader>
                                <Card>
                                    <CardContent className="pt-6">
                                        <form onSubmit={handleSendNotification} className="space-y-4">
                                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                <div className="space-y-2">
                                                    <div className="flex items-center justify-between gap-3">
                                                        <Label htmlFor="recipient">
                                                            Recipients (Users)
                                                            {formData.channel === TemplateChannel.WEBHOOK && <span className="font-normal text-gray-500 text-xs"> (Optional)</span>}
                                                        </Label>
                                                        <div className="flex items-center gap-2">
                                                            <Switch
                                                                checked={sendToAllUsers}
                                                                onCheckedChange={(checked) => {
                                                                    setSendToAllUsers(checked);
                                                                    if (checked) {
                                                                        setSelectedUsers([]);
                                                                    }
                                                                }}
                                                                size="sm"
                                                            />
                                                            <span className="text-xs text-gray-600">Send to all users</span>
                                                        </div>
                                                    </div>
                                                    <UserMultiSelect
                                                        users={users}
                                                        value={selectedUsers}
                                                        onChange={setSelectedUsers}
                                                        disabled={sendToAllUsers || isSubmitting}
                                                    />
                                                </div>
                                                <div className="space-y-2">
                                                    <Label htmlFor="template">Template (Optional)</Label>
                                                    <Select
                                                        value={formData.template_id || 'none'}
                                                        onValueChange={(value) => applyTemplateSelection(value === 'none' ? undefined : value)}
                                                    >
                                                        <SelectTrigger id="template">
                                                            <SelectValue placeholder="No template (use manual content)" />
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
                                                    <Label htmlFor="channel">Channel</Label>
                                                    <Select
                                                        value={formData.channel}
                                                        onValueChange={(value) => handleChannelChange(value)}
                                                        disabled={!!formData.template_id}
                                                    >
                                                        <SelectTrigger id="channel">
                                                            <SelectValue />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            <SelectItem value={TemplateChannel.EMAIL}>Email</SelectItem>
                                                            <SelectItem value={TemplateChannel.PUSH}>Push</SelectItem>
                                                            <SelectItem value={TemplateChannel.SMS}>SMS</SelectItem>
                                                            <SelectItem value={TemplateChannel.WEBHOOK}>Webhook</SelectItem>
                                                            <SelectItem value={TemplateChannel.IN_APP}>In-App</SelectItem>
                                                            <SelectItem value={TemplateChannel.SSE}>SSE (Server-Sent Events)</SelectItem>
                                                        </SelectContent>
                                                    </Select>
                                                    {formData.template_id && (
                                                        <p className="text-xs text-gray-500">Channel is set by the selected template.</p>
                                                    )}
                                                </div>

                                                {formData.channel === TemplateChannel.WEBHOOK && webhooks && Object.keys(webhooks).length > 0 && (
                                                    <div className="space-y-2 md:col-span-2">
                                                        <Label>Webhook Targets (Select one or more)</Label>
                                                        <WebhookTargetSelect
                                                            targets={Object.keys(webhooks)}
                                                            value={selectedTargets}
                                                            onChange={setSelectedTargets}
                                                        />
                                                        <p className="text-xs text-gray-500">
                                                            If multiple targets are selected, a separate notification will be sent to each.
                                                        </p>
                                                    </div>
                                                )}

                                                {formData.channel === TemplateChannel.WEBHOOK && (
                                                    <div className="space-y-2 md:col-span-2">
                                                        <Label htmlFor="webhookUrl">Webhook URL Override (Optional)</Label>
                                                        <Input
                                                            id="webhookUrl"
                                                            type="url"
                                                            value={formData.webhook_url || ''}
                                                            disabled={isSubmitting}
                                                            onChange={(e) => setFormData({ ...formData, webhook_url: e.target.value })}
                                                            placeholder="https://discord.com/api/webhooks/..."
                                                            required={selectedUsers.length === 0 && selectedTargets.length === 0}
                                                        />
                                                        <p className="text-xs text-gray-500">
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
                                                        <SelectTrigger id="priority">
                                                            <SelectValue />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            <SelectItem value={NotificationPriority.LOW}>Low</SelectItem>
                                                            <SelectItem value={NotificationPriority.NORMAL}>Normal</SelectItem>
                                                            <SelectItem value={NotificationPriority.HIGH}>High</SelectItem>
                                                            <SelectItem value={NotificationPriority.CRITICAL}>Critical</SelectItem>
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
                                                    disabled={isSubmitting}
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
                                                    disabled={isSubmitting}
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
                                                    disabled={isSubmitting}
                                                    onChange={(e) => setDataInput(e.target.value)}
                                                    placeholder='{ "name": "John Doe" }'
                                                />
                                            </div>

                                            <div className="space-y-4 pt-4 border-t border-gray-100">
                                                <h4 className="text-sm font-semibold text-blue-600">Scheduled Delivery (Optional)</h4>
                                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                    <div className="space-y-2">
                                                        <Label htmlFor="scheduledAt">Scheduled Time (Future)</Label>
                                                        <Input
                                                            id="scheduledAt"
                                                            type="datetime-local"
                                                            value={formData.scheduled_at?.substring(0, 16) || ''}
                                                            disabled={isSubmitting}
                                                            onChange={(e) => {
                                                                const val = e.target.value;
                                                                setFormData({ ...formData, scheduled_at: val ? new Date(val).toISOString() : undefined });
                                                            }}
                                                        />
                                                        <p className="text-xs text-gray-500">
                                                            Leave empty for immediate delivery.
                                                        </p>
                                                    </div>

                                                    <div className="space-y-2">
                                                        <Label htmlFor="recurrenceCron">Recurrence Cron (e.g. 0 0 * * *)</Label>
                                                        <Input
                                                            id="recurrenceCron"
                                                            type="text"
                                                            value={formData.recurrence?.cron_expression || ''}
                                                            disabled={isSubmitting}
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
                                                                    disabled={isSubmitting}
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
                                                                    disabled={isSubmitting}
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
                                                </div>
                                            </div>

                                            <div className="flex justify-end mt-6">
                                                <div className="flex gap-2">
                                                    <Button
                                                        type="button"
                                                        variant="outline"
                                                        disabled={isSubmitting}
                                                        onClick={() => {
                                                            handleReset();
                                                            setIsSendDialogOpen(false);
                                                        }}
                                                    >
                                                        Cancel
                                                    </Button>
                                                    <Button type="submit" disabled={isSubmitting}>
                                                        {isSubmitting ? 'Sending...' : 'Send / Schedule Notification'}
                                                    </Button>
                                                </div>
                                            </div>
                                        </form>
                                    </CardContent>
                                </Card>
                            </DialogContent>
                        </Dialog>

                    </div>
                </div>
            </CardHeader>
            <CardContent>
                {!notifications || notifications.length === 0 ? (
                    <p className="text-gray-500 text-center py-8 text-sm">No notification history found.</p>
                ) : (
                    <div className="overflow-x-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>ID</TableHead>
                                    <TableHead>User</TableHead>
                                    <TableHead>Title</TableHead>
                                    <TableHead>Status</TableHead>
                                    <TableHead>Scheduled At</TableHead>
                                    <TableHead>Sent At</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {notifications.map((n) => (
                                    <TableRow key={n.notification_id}>
                                        <TableCell className="text-xs text-gray-400 font-mono">{n.notification_id?.substring(0, 8)}...</TableCell>
                                        <TableCell className="text-gray-900">
                                            {n.user_id ?
                                                (users?.find(u => u.user_id === n.user_id)?.email || n.user_id) :
                                                <span className="text-gray-500 italic">Anonymous (Webhook)</span>
                                            }
                                        </TableCell>
                                        <TableCell className="text-gray-900">{n.content?.title || '-'}</TableCell>
                                        <TableCell>
                                            <Badge
                                                variant="outline"
                                                className={`text-xs uppercase ${getStatusBadgeClass(n.status)}`}
                                            >
                                                {n.status}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-sm text-gray-500">
                                            {n.scheduled_at ? (
                                                <div className="flex flex-col">
                                                    <span>{new Date(n.scheduled_at).toLocaleString()}</span>
                                                    {n.recurrence && (
                                                        <span className="text-[10px] text-blue-600 mt-0.5">
                                                            🔄 {n.recurrence.cron_expression}
                                                        </span>
                                                    )}
                                                </div>
                                            ) : (
                                                <span className="text-gray-400">Now</span>
                                            )}
                                        </TableCell>
                                        <TableCell className="text-sm text-gray-500">
                                            {n.created_at ? new Date(n.created_at).toLocaleString() : '-'}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                )}
            </CardContent>
        </Card>
    );
};

export default AppNotifications;


const UserMultiSelect: React.FC<{
    users: User[] | undefined;
    value: string[];
    onChange: (value: string[]) => void;
    disabled?: boolean;
}> = ({ users, value, onChange, disabled = false }) => {
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
        <div className={`space-y-2 ${disabled ? 'opacity-60' : ''}`}>
            <Dialog open={isOpen} onOpenChange={(next) => { if (!disabled) setIsOpen(next); }}>
                <DialogTrigger asChild>
                    <Button variant="outline" type="button" className="w-full justify-between" disabled={disabled}>
                        <span className="truncate text-left">{selectorLabel}</span>
                        <span className="text-xs text-gray-500">{selectedCount}/{totalUsers}</span>
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
                            disabled={disabled}
                        />
                        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500">
                            <span>Selected {selectedCount} of {totalUsers}</span>
                            <div className="flex gap-2">
                                <button
                                    type="button"
                                    className="hover:text-gray-700"
                                    onClick={() => onChange(filteredUsers.map(u => u.user_id))}
                                >
                                    Select all (filtered)
                                </button>
                                <button
                                    type="button"
                                    className="hover:text-gray-700"
                                    onClick={() => onChange([])}
                                >
                                    Clear
                                </button>
                            </div>
                        </div>
                        <div className="max-h-72 overflow-y-auto rounded border border-gray-200">
                            {filteredUsers.length === 0 ? (
                                <p className="text-gray-500 text-sm p-3">No users found.</p>
                            ) : (
                                <div className="divide-y divide-gray-100">
                                    {filteredUsers.map(u => (
                                        <div key={u.user_id} className="flex items-center justify-between px-3 py-2">
                                            <div className="min-w-0">
                                                <div className="font-medium text-gray-900 truncate">{u.email || 'No email'}</div>
                                                <div className="text-xs text-gray-500 truncate">{u.user_id}</div>
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
                                    <span key={user.user_id} className="inline-flex items-center gap-2 rounded-full bg-blue-50 px-3 py-1 text-xs text-blue-700">
                                        {user.email || user.user_id}
                                        <button
                                            type="button"
                                            className="text-blue-500 hover:text-blue-700"
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
            <div className="flex items-center justify-between text-xs text-gray-500">
                <span>Selected {value.length} of {targets.length}</span>
                <div className="flex gap-2">
                    <button type="button" className="hover:text-gray-700" onClick={() => onChange(targets)}>Select all</button>
                    <button type="button" className="hover:text-gray-700" onClick={() => onChange([])}>Clear</button>
                </div>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-2 p-3 border border-gray-200 rounded bg-white">
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
import React, { useEffect, useState } from 'react';
import { notificationsAPI, usersAPI, templatesAPI } from '../services/api';
import type { Notification, NotificationRequest, User, Template } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Textarea } from './ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Badge } from './ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Checkbox } from './ui/checkbox';
import { toast } from 'sonner';

interface AppNotificationsProps {
    appId: string;
    apiKey: string;
    webhooks?: Record<string, string>;
}

const AppNotifications: React.FC<AppNotificationsProps> = ({ apiKey, webhooks }) => {
    const [notifications, setNotifications] = useState<Notification[]>([]);
    const [users, setUsers] = useState<User[]>([]);
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loading, setLoading] = useState(true);
    const [showSendForm, setShowSendForm] = useState(false);
    const [formData, setFormData] = useState<NotificationRequest>({
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

    const [selectedTargets, setSelectedTargets] = useState<string[]>([]);
    const [dataInput, setDataInput] = useState('');

    useEffect(() => {
        if (apiKey) {
            fetchData();
        }
    }, [apiKey]);

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

    const handleSendNotification = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            // Parse custom data if any
            let customData = {};
            if (dataInput) {
                try {
                    customData = JSON.parse(dataInput);
                } catch (e) {
                    toast.error('Invalid JSON in custom data');
                    return;
                }
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
            } else {
                // Single send (default behavior)
                await notificationsAPI.send(apiKey, { ...formData, data: customData });
            }

            setShowSendForm(false);
            setFormData({
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
            setSelectedTargets([]);
            setDataInput('');
            fetchData();
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
            default: return 'bg-gray-100 text-gray-700 border-gray-300';
        }
    };

    if (loading) return <div className="flex justify-center py-4">Loading notifications...</div>;

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle>Notification History</CardTitle>
                    <Button
                        onClick={() => setShowSendForm(!showSendForm)}
                    >
                        {showSendForm ? 'Cancel' : 'Send Notification'}
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                {showSendForm && (
                    <form onSubmit={handleSendNotification} className="mb-8 bg-gray-50 p-6 rounded border border-gray-200 space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="recipient">
                                    Recipient (User)
                                    {formData.channel === 'webhook' && <span className="font-normal text-gray-500 text-xs"> (Optional)</span>}
                                </Label>
                                <Select
                                    value={formData.user_id}
                                    onValueChange={(value) => setFormData({ ...formData, user_id: value })}
                                    required={formData.channel !== 'webhook'}
                                >
                                    <SelectTrigger>
                                        <SelectValue placeholder={formData.channel === 'webhook' ? 'No user (Anonymous)' : 'Select a user...'} />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {(users || []).map(u => (
                                            <SelectItem key={u.user_id} value={u.user_id}>{u.email || u.user_id}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
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
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="channel">Channel</Label>
                                <Select
                                    value={formData.channel}
                                    onValueChange={(value) => setFormData({ ...formData, channel: value as any })}
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
                                    <div className="grid grid-cols-2 md:grid-cols-3 gap-2 p-3 border border-gray-200 rounded bg-white">
                                        {Object.keys(webhooks).map(name => (
                                            <div key={name} className="flex items-center space-x-2">
                                                <Checkbox
                                                    id={`webhook-${name}`}
                                                    checked={selectedTargets.includes(name)}
                                                    onCheckedChange={(checked) => {
                                                        if (checked) {
                                                            setSelectedTargets([...selectedTargets, name]);
                                                        } else {
                                                            setSelectedTargets(selectedTargets.filter(t => t !== name));
                                                        }
                                                    }}
                                                />
                                                <Label htmlFor={`webhook-${name}`} className="text-sm cursor-pointer">{name}</Label>
                                            </div>
                                        ))}
                                    </div>
                                    <p className="text-xs text-gray-500">
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
                                        required={!formData.user_id && selectedTargets.length === 0}
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
                            </div>
                        </div>

                        <div className="flex justify-end mt-6">
                            <Button type="submit">Send / Schedule Notification</Button>
                        </div>
                    </form>
                )}

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

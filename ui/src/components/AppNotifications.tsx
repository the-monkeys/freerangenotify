import React, { useEffect, useState } from 'react';
import { notificationsAPI, usersAPI, templatesAPI } from '../services/api';
import type { Notification, NotificationRequest, User, Template } from '../types';

interface AppNotificationsProps {
    appId: string;
    apiKey: string;
}

const AppNotifications: React.FC<AppNotificationsProps> = ({ apiKey }) => {
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
        data: {}
    });

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
                    alert('Invalid JSON in custom data');
                    return;
                }
            }

            await notificationsAPI.send(apiKey, { ...formData, data: customData });
            setShowSendForm(false);
            setFormData({
                user_id: '',
                channel: 'email',
                priority: 'normal',
                title: '',
                body: '',
                template_id: '',
                data: {}
            });
            setDataInput('');
            fetchData();
        } catch (error) {
            console.error('Failed to send notification:', error);
            alert('Failed to send notification');
        }
    };

    const getStatusColor = (status: string) => {
        switch (status?.toLowerCase()) {
            case 'sent': return '#48bb78';
            case 'failed': return '#f56565';
            case 'pending': return '#ecc94b';
            case 'queued': return '#4299e1';
            case 'delivered': return '#38b2ac';
            default: return '#a0aec0';
        }
    };

    if (loading) return <div className="center">Loading notifications...</div>;

    return (
        <div className="card">
            <div className="flex justify-between items-center mb-6">
                <h3 style={{ margin: 0 }}>Notification History</h3>
                <button
                    className="btn btn-primary"
                    onClick={() => setShowSendForm(!showSendForm)}
                >
                    {showSendForm ? 'Cancel' : 'Send Notification'}
                </button>
            </div>

            {showSendForm && (
                <form onSubmit={handleSendNotification} className="mb-8" style={{ background: 'var(--azure-bg)', padding: '1.5rem', borderRadius: '2px', border: '1px solid var(--azure-border)' }}>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="form-group">
                            <label className="form-label">
                                Recipient (User)
                                {formData.channel === 'webhook' && <span style={{ fontWeight: 'normal', color: '#666', fontSize: '0.8em' }}> (Optional)</span>}
                            </label>
                            <select
                                className="form-input"
                                value={formData.user_id}
                                onChange={(e) => setFormData({ ...formData, user_id: e.target.value })}
                                required={formData.channel !== 'webhook'}
                            >
                                <option value="">{formData.channel === 'webhook' ? 'No user (Anonymous)' : 'Select a user...'}</option>
                                {(users || []).map(u => (
                                    <option key={u.user_id} value={u.user_id}>{u.external_user_id} ({u.email || 'no email'})</option>
                                ))}
                            </select>
                        </div>
                        <div className="form-group">
                            <label className="form-label">Template (Optional)</label>
                            <select
                                className="form-input"
                                value={formData.template_id || ''}
                                onChange={(e) => setFormData({ ...formData, template_id: e.target.value })}
                            >
                                <option value="">No template (use manual content)</option>
                                {(templates || []).map(t => (
                                    <option key={t.id} value={t.id}>{t.name} ({t.channel})</option>
                                ))}
                            </select>
                        </div>
                        <div className="form-group">
                            <label className="form-label">Channel</label>
                            <select
                                className="form-input"
                                value={formData.channel}
                                onChange={(e) => setFormData({ ...formData, channel: e.target.value as any })}
                            >
                                <option value="email">Email</option>
                                <option value="push">Push</option>
                                <option value="sms">SMS</option>
                                <option value="webhook">Webhook</option>
                                <option value="in_app">In-App</option>
                                <option value="sse">SSE (Server-Sent Events)</option>
                            </select>
                        </div>

                        {/* Webhook URL Field - Only shown for webhook channel */}
                        {formData.channel === 'webhook' && (
                            <div className="form-group md:col-span-2">
                                <label className="form-label">Webhook URL</label>
                                <input
                                    type="url"
                                    className="form-input"
                                    value={formData.webhook_url || ''}
                                    onChange={(e) => setFormData({ ...formData, webhook_url: e.target.value })}
                                    placeholder="https://discord.com/api/webhooks/..."
                                    required={!formData.user_id} // Required if no user selected
                                />
                                <p style={{ fontSize: '0.75rem', marginTop: '0.25rem', color: '#666' }}>
                                    Override or provide destination URL. Required if no user is selected.
                                </p>
                            </div>
                        )}
                        <div className="form-group">
                            <label className="form-label">Priority</label>
                            <select
                                className="form-input"
                                value={formData.priority}
                                onChange={(e) => setFormData({ ...formData, priority: e.target.value as any })}
                            >
                                <option value="low">Low</option>
                                <option value="normal">Normal</option>
                                <option value="high">High</option>
                                <option value="critical">Critical</option>
                            </select>
                        </div>
                    </div>

                    <div className="form-group">
                        <label className="form-label">Title</label>
                        <input
                            type="text"
                            className="form-input"
                            value={formData.title}
                            onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                            required
                            placeholder="Notification title"
                        />
                    </div>

                    <div className="form-group">
                        <label className="form-label">Body / Manual Content</label>
                        <textarea
                            className="form-input"
                            value={formData.body}
                            onChange={(e) => setFormData({ ...formData, body: e.target.value })}
                            required={!formData.template_id}
                            placeholder={formData.template_id ? "Optional (overridden by template)" : "Visible unless overridden by template"}
                        />
                    </div>

                    <div className="form-group">
                        <label className="form-label">Custom Data (JSON)</label>
                        <textarea
                            className="form-input"
                            style={{ fontFamily: 'monospace' }}
                            value={dataInput}
                            onChange={(e) => setDataInput(e.target.value)}
                            placeholder='{ "name": "John Doe" }'
                        />
                    </div>

                    <div className="flex justify-end mt-4">
                        <button type="submit" className="btn btn-primary">Send Notification</button>
                    </div>
                </form>
            )}

            {!notifications || notifications.length === 0 ? (
                <p style={{ color: '#605e5c', textAlign: 'center', padding: '2rem', fontSize: '0.9rem' }}>No notification history found.</p>
            ) : (
                <div style={{ overflowX: 'auto' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                        <thead>
                            <tr style={{ borderBottom: '1px solid var(--azure-border)', textAlign: 'left' }}>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>ID</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>User</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Title</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Status</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Sent At</th>
                            </tr>
                        </thead>
                        <tbody>
                            {notifications.map((n) => (
                                <tr key={n.notification_id} style={{ borderBottom: '1px solid var(--azure-border)' }}>
                                    <td style={{ padding: '0.75rem', fontSize: '0.75rem', color: '#a19f9d', fontFamily: 'monospace' }}>{n.notification_id?.substring(0, 8)}...</td>
                                    <td style={{ padding: '0.75rem', color: '#323130' }}>
                                        {n.user_id ?
                                            (users?.find(u => u.user_id === n.user_id)?.external_user_id || n.user_id) :
                                            <span style={{ color: '#666', fontStyle: 'italic' }}>Anonymous (Webhook)</span>
                                        }
                                    </td>
                                    <td style={{ padding: '0.75rem', color: '#323130' }}>{n.title}</td>
                                    <td style={{ padding: '0.75rem' }}>
                                        <span style={{
                                            padding: '0.15rem 0.6rem',
                                            borderRadius: '2px',
                                            fontSize: '0.7rem',
                                            fontWeight: 600,
                                            background: `${getStatusColor(n.status)}15`,
                                            color: getStatusColor(n.status),
                                            border: `1px solid ${getStatusColor(n.status)}`,
                                            textTransform: 'uppercase'
                                        }}>
                                            {n.status}
                                        </span>
                                    </td>
                                    <td style={{ padding: '0.75rem', fontSize: '0.8rem', color: '#605e5c' }}>
                                        {n.created_at ? new Date(n.created_at).toLocaleString() : '-'}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
};

export default AppNotifications;

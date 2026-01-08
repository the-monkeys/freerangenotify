import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { applicationsAPI } from '../services/api';
import type { Application, ApplicationSettings } from '../types';
import AppUsers from '../components/AppUsers';
import AppTemplates from '../components/AppTemplates';
import AppNotifications from '../components/AppNotifications';

const AppDetail: React.FC = () => {
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const [app, setApp] = useState<Application | null>(null);
    const [loading, setLoading] = useState(true);
    const [activeTab, setActiveTab] = useState<'overview' | 'users' | 'templates' | 'notifications' | 'settings' | 'integration'>('overview');

    // Local state for editing
    const [appName, setAppName] = useState('');
    const [description, setDescription] = useState('');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [settings, setSettings] = useState<ApplicationSettings>({});
    const [webhooks, setWebhooks] = useState<Record<string, string>>({});
    const [newWebhookName, setNewWebhookName] = useState('');
    const [newWebhookUrl, setNewWebhookUrl] = useState('');
    const [staticHeadersText, setStaticHeadersText] = useState('');
    const [showApiKey, setShowApiKey] = useState(false);


    useEffect(() => {
        if (id) fetchAppDetails();
    }, [id]);

    const fetchAppDetails = async () => {
        setLoading(true);
        try {
            if (!id) return;
            const appData = await applicationsAPI.get(id);
            setApp(appData);
            setAppName(appData.app_name);
            setDescription(appData.description || '');
            setWebhookUrl(appData.webhook_url || '');
            setWebhooks(appData.webhooks || {});
            setSettings(appData.settings || {});

            // Initialize static headers text
            const text = Object.entries(appData.settings?.validation_config?.static_headers || {})
                .map(([k, v]) => `${k}: ${v}`).join('\n');
            setStaticHeadersText(text);
        } catch (error) {
            console.error('Failed to fetch app details:', error);
        } finally {
            setLoading(false);
        }
    };

    const handleUpdateOverview = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!id) return;
        try {
            const updated = await applicationsAPI.update(id, {
                app_name: appName,
                description: description,
                webhook_url: webhookUrl,
                webhooks: webhooks
            });
            setApp(updated);
            alert('Application updated successfully!');
        } catch (error) {
            console.error('Failed to update application:', error);
            alert('Failed to update application');
        }
    };

    const handleRegenerateKey = async () => {
        if (!id || !window.confirm('Are you sure? The old key will immediately stop working.')) return;
        try {
            await applicationsAPI.regenerateKey(id);
            fetchAppDetails();
            alert('API Key regenerated.');
        } catch (error) {
            console.error('Failed to regenerate key:', error);
        }
    };

    const handleDeleteApp = async () => {
        if (!id || !window.confirm('Are you sure? This cannot be undone.')) return;
        try {
            await applicationsAPI.delete(id);
            navigate('/');
        } catch (error) {
            console.error('Failed to delete application:', error);
        }
    };

    if (loading) return <div className="center"><div className="spinner"></div></div>;
    if (!app) return <div className="center">Application not found</div>;

    return (
        <div className="container">
            <div className="mb-6">
                <button
                    onClick={() => navigate('/')}
                    className="btn btn-secondary"
                    style={{ minWidth: 'auto', padding: '0.2rem 0.5rem', marginBottom: '1rem', border: 'none' }}
                >
                    &larr; Back to Applications
                </button>
                <h1 style={{ fontSize: '1.5rem', fontWeight: 600 }}>
                    {app.app_name}
                </h1>
                <div style={{ display: 'flex', alignItems: 'center', marginTop: '0.5rem', color: '#605e5c', fontSize: '0.9rem' }}>
                    <span style={{ marginRight: '0.5rem' }}>ID:</span>
                    <code style={{ background: '#f3f2f1', padding: '2px 6px', borderRadius: '3px', fontFamily: 'monospace', fontWeight: 600 }}>
                        {app.app_id}
                    </code>
                </div>
            </div>

            {/* Tabs - Azure Flat Style */}
            <div style={{ display: 'flex', borderBottom: '1px solid var(--azure-border)', marginBottom: '2rem', overflowX: 'auto', whiteSpace: 'nowrap' }}>
                {(['overview', 'users', 'templates', 'notifications', 'settings', 'integration'] as const).map((tab) => (
                    <button
                        key={tab}
                        onClick={() => setActiveTab(tab)}
                        style={{
                            padding: '0.75rem 1.25rem',
                            borderBottom: activeTab === tab ? '2px solid var(--azure-blue)' : '2px solid transparent',
                            color: activeTab === tab ? 'var(--azure-blue)' : '#605e5c',
                            fontWeight: activeTab === tab ? 600 : 400,
                            background: 'none',
                            textTransform: 'capitalize',
                            cursor: 'pointer',
                            flexShrink: 0,
                            fontSize: '0.9rem'
                        }}
                    >
                        {tab}
                    </button>
                ))}
            </div>

            {/* Overview Tab */}
            {activeTab === 'overview' && (
                <div className="card">
                    <h3 className="mb-4">Application Details</h3>
                    <form onSubmit={handleUpdateOverview}>
                        <div className="form-group">
                            <label className="form-label">Application Name</label>
                            <input
                                type="text"
                                className="form-input"
                                value={appName}
                                onChange={(e) => setAppName(e.target.value)}
                                required
                            />
                        </div>
                        <div className="form-group">
                            <label className="form-label">Description</label>
                            <textarea
                                className="form-input"
                                style={{ minHeight: '100px' }}
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                            />
                        </div>
                        <div className="form-group">
                            <label className="form-label">Webhook URL (Default)</label>
                            <input
                                type="url"
                                className="form-input"
                                value={webhookUrl}
                                onChange={(e) => setWebhookUrl(e.target.value)}
                                placeholder="https://example.com/webhook"
                            />
                            <p style={{ fontSize: '0.8rem', color: '#605e5c', marginTop: '0.4rem' }}>
                                The default webhook URL used if no named target is specified.
                            </p>
                        </div>

                        {/* Named Webhooks Section */}
                        <div className="form-group" style={{ marginTop: '2rem', paddingTop: '2rem', borderTop: '1px solid var(--azure-border)' }}>
                            <label className="form-label" style={{ fontSize: '1rem', color: 'var(--azure-blue)', display: 'block', marginBottom: '1rem' }}>
                                Named Webhook Endpoints
                            </label>
                            <p style={{ fontSize: '0.85rem', color: '#605e5c', marginBottom: '1.5rem' }}>
                                Define named webhook targets (e.g., 'slack', 'discord') that templates can use for routing.
                            </p>

                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                                <div className="form-group">
                                    <label className="form-label text-xs">Target Name</label>
                                    <input
                                        type="text"
                                        className="form-input text-sm"
                                        value={newWebhookName}
                                        onChange={(e) => setNewWebhookName(e.target.value)}
                                        placeholder="e.g. slack"
                                    />
                                </div>
                                <div className="form-group">
                                    <label className="form-label text-xs">Webhook URL</label>
                                    <div className="flex gap-2">
                                        <input
                                            type="url"
                                            className="form-input text-sm"
                                            value={newWebhookUrl}
                                            onChange={(e) => setNewWebhookUrl(e.target.value)}
                                            placeholder="https://hooks.slack.com/..."
                                        />
                                        <button
                                            type="button"
                                            className="btn btn-secondary"
                                            onClick={() => {
                                                if (newWebhookName && newWebhookUrl) {
                                                    setWebhooks({ ...webhooks, [newWebhookName]: newWebhookUrl });
                                                    setNewWebhookName('');
                                                    setNewWebhookUrl('');
                                                }
                                            }}
                                        >
                                            Add
                                        </button>
                                    </div>
                                </div>
                            </div>

                            <div className="space-y-3">
                                {Object.entries(webhooks).map(([name, url]) => (
                                    <div key={name} className="flex items-center justify-between p-3 bg-gray-50 border border-gray-200 rounded">
                                        <div style={{ flex: 1, minWidth: 0 }}>
                                            <div style={{ fontWeight: 600, fontSize: '0.9rem', color: 'var(--azure-blue)' }}>{name}</div>
                                            <div style={{ fontSize: '0.8rem', color: '#605e5c', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{url}</div>
                                        </div>
                                        <button
                                            type="button"
                                            className="btn btn-danger"
                                            style={{ padding: '0.2rem 0.5rem', minWidth: 'auto', fontSize: '0.75rem' }}
                                            onClick={() => {
                                                const newWebhooks = { ...webhooks };
                                                delete newWebhooks[name];
                                                setWebhooks(newWebhooks);
                                            }}
                                        >
                                            Remove
                                        </button>
                                    </div>
                                ))}
                                {Object.keys(webhooks).length === 0 && (
                                    <p style={{ fontSize: '0.85rem', color: '#a19f9d', textAlign: 'center', fontStyle: 'italic', padding: '1rem' }}>
                                        No named webhooks configured.
                                    </p>
                                )}
                            </div>
                        </div>

                        <div className="flex justify-end mt-8">
                            <button type="submit" className="btn btn-primary">Save Overview & Webhooks</button>
                        </div>
                    </form>
                </div>
            )}

            {/* Users Tab */}
            {activeTab === 'users' && app && (
                <AppUsers apiKey={app.api_key} />
            )}

            {/* Templates Tab */}
            {activeTab === 'templates' && app && (
                <AppTemplates appId={app.app_id} apiKey={app.api_key} webhooks={webhooks} />
            )}

            {/* Notifications Tab */}
            {activeTab === 'notifications' && app && (
                <AppNotifications appId={app.app_id} apiKey={app.api_key} webhooks={webhooks} />
            )}

            {/* Settings Tab */}
            {activeTab === 'settings' && (
                <div className="card">
                    <h3 className="mb-4">Configuration</h3>
                    <p style={{ color: '#718096', marginBottom: '1.5rem' }}>
                        Manage configuration for this application.
                    </p>

                    <form
                        onSubmit={async (e) => {
                            e.preventDefault();
                            try {
                                await applicationsAPI.updateSettings(id!, settings);
                                alert('Settings saved!');
                            } catch (err: any) {
                                alert('Error saving settings: ' + (err.response?.data?.message || err.message));
                            }
                        }}
                        style={{ background: 'var(--azure-white)', padding: '1.5rem', borderRadius: '2px', border: '1px solid var(--azure-border)' }}
                    >
                        <h4 className="mb-4" style={{ color: 'var(--azure-blue)', fontSize: '1rem' }}>Core Settings</h4>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
                            <div className="form-group">
                                <label className="form-label">Rate Limit (requests/hour)</label>
                                <input
                                    type="number"
                                    className="form-input"
                                    value={settings.rate_limit || 0}
                                    onChange={(e) => setSettings({ ...settings, rate_limit: parseInt(e.target.value) || 0 })}
                                    placeholder="e.g. 1000"
                                />
                            </div>

                            <div className="form-group">
                                <label className="form-label">Retry Attempts</label>
                                <input
                                    type="number"
                                    className="form-input"
                                    value={settings.retry_attempts || 0}
                                    onChange={(e) => setSettings({ ...settings, retry_attempts: parseInt(e.target.value) || 0 })}
                                    placeholder="e.g. 3"
                                />
                            </div>

                            <div className="form-group">
                                <label className="form-label">Default Template ID</label>
                                <input
                                    type="text"
                                    className="form-input"
                                    value={settings.default_template || ''}
                                    onChange={(e) => setSettings({ ...settings, default_template: e.target.value })}
                                    placeholder="Template UUID"
                                />
                            </div>

                            <div className="form-group flex flex-col justify-end space-y-6">
                                <label className="flex items-center space-x-3 cursor-pointer" style={{ fontSize: '0.9rem' }}>
                                    <input
                                        type="checkbox"
                                        style={{ accentColor: 'var(--azure-blue)', height: '1.2rem', width: '1.2rem' }}
                                        checked={!!settings.enable_webhooks}
                                        onChange={(e) => setSettings({ ...settings, enable_webhooks: e.target.checked })}
                                    />
                                    <span style={{ fontWeight: 500 }}>Enable Webhooks</span>
                                </label>
                                <label className="flex items-center space-x-3 cursor-pointer" style={{ fontSize: '0.9rem' }}>
                                    <input
                                        type="checkbox"
                                        style={{ accentColor: 'var(--azure-blue)', height: '1.2rem', width: '1.2rem' }}
                                        checked={!!settings.enable_analytics}
                                        onChange={(e) => setSettings({ ...settings, enable_analytics: e.target.checked })}
                                    />
                                    <span style={{ fontWeight: 500 }}>Enable Analytics</span>
                                </label>
                            </div>
                        </div>

                        <h4 className="mb-4" style={{ color: 'var(--azure-blue)', fontSize: '1rem' }}>Authentication & Security</h4>
                        <div className="grid grid-cols-1 gap-6 mb-8">
                            <div className="form-group">
                                <label className="form-label">Validation URL (Zero-Trust API)</label>
                                <input
                                    type="url"
                                    className="form-input"
                                    value={settings.validation_url || ''}
                                    onChange={(e) => setSettings({ ...settings, validation_url: e.target.value })}
                                    placeholder="https://your-bank.com/api/verify-token"
                                />
                                <p style={{ fontSize: '0.8rem', color: '#605e5c', marginTop: '0.4rem' }}>
                                    If set, FreeRangeNotify will call this URL to verify user tokens before allowing SSE connections.
                                </p>
                            </div>

                            {settings.validation_url && (
                                <div className="p-4 border border-blue-100 rounded bg-blue-50 mt-4">
                                    <h5 className="font-semibold text-sm mb-3 text-blue-800">Validation Request Configuration</h5>

                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                                        <div className="form-group">
                                            <label className="form-label text-xs">Method</label>
                                            <select
                                                className="form-input text-sm"
                                                value={settings.validation_config?.method || 'POST'}
                                                onChange={(e) => setSettings({
                                                    ...settings,
                                                    validation_config: {
                                                        ...settings.validation_config,
                                                        method: e.target.value,
                                                        token_placement: settings.validation_config?.token_placement || 'body_json',
                                                        token_key: settings.validation_config?.token_key || 'token',
                                                    }
                                                })}
                                            >
                                                <option value="POST">POST</option>
                                                <option value="GET">GET</option>
                                            </select>
                                        </div>
                                        <div className="form-group">
                                            <label className="form-label text-xs">Token Placement</label>
                                            <select
                                                className="form-input text-sm"
                                                value={settings.validation_config?.token_placement || 'body_json'}
                                                onChange={(e) => setSettings({
                                                    ...settings,
                                                    validation_config: {
                                                        ...settings.validation_config,
                                                        method: settings.validation_config?.method || 'POST',
                                                        token_placement: e.target.value,
                                                        token_key: settings.validation_config?.token_key || 'token',
                                                    }
                                                })}
                                            >
                                                <option value="body_json">Body (JSON)</option>
                                                <option value="body_form">Body (Form URL Encoded)</option>
                                                <option value="header">Header</option>
                                                <option value="query">Query Parameter</option>
                                                <option value="cookie">Cookie</option>
                                            </select>
                                        </div>
                                        <div className="form-group col-span-2">
                                            <label className="form-label text-xs">Token Key Name</label>
                                            <input
                                                type="text"
                                                className="form-input text-sm"
                                                value={settings.validation_config?.token_key || 'token'}
                                                onChange={(e) => setSettings({
                                                    ...settings,
                                                    validation_config: {
                                                        ...settings.validation_config!,
                                                        token_key: e.target.value
                                                    }
                                                })}
                                                placeholder="e.g. Authorization, access_token, mat"
                                            />
                                            <p className="text-xs text-gray-500 mt-1">The name of the header, cookie, or field that contains the token.</p>
                                        </div>
                                    </div>

                                    <div className="form-group">
                                        <label className="form-label text-xs mb-2 block">Static Headers (e.g., Client-ID, User-Agent)</label>
                                        <textarea
                                            className="form-input text-sm font-mono"
                                            rows={3}
                                            placeholder={'Client-ID: 12345\nUser-Agent: MyApp/1.0'}
                                            value={staticHeadersText}
                                            onChange={(e) => {
                                                const newText = e.target.value;
                                                setStaticHeadersText(newText);

                                                const lines = newText.split('\n');
                                                const headers: Record<string, string> = {};
                                                lines.forEach(line => {
                                                    const parts = line.split(':');
                                                    if (parts.length >= 2) {
                                                        const key = parts[0].trim();
                                                        const val = parts.slice(1).join(':').trim();
                                                        if (key) headers[key] = val;
                                                    }
                                                });
                                                setSettings({
                                                    ...settings,
                                                    validation_config: {
                                                        ...settings.validation_config!,
                                                        static_headers: headers
                                                    }
                                                });
                                            }}
                                        />
                                        <p className="text-xs text-gray-500 mt-1">One header per line. Format: Header-Name: Value</p>
                                    </div>
                                </div>
                            )}
                        </div>

                        <h4 className="mb-4" style={{ color: 'var(--azure-blue)', fontSize: '1rem' }}>Default Notification Preferences</h4>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
                            <label className="flex items-center space-x-3 cursor-pointer" style={{ fontSize: '0.9rem' }}>
                                <input
                                    type="checkbox"
                                    style={{ accentColor: '#48bb78', height: '1.2rem', width: '1.2rem' }}
                                    checked={settings.default_preferences?.email_enabled ?? true}
                                    onChange={(e) => setSettings({
                                        ...settings,
                                        default_preferences: {
                                            ...(settings.default_preferences || {}),
                                            email_enabled: e.target.checked
                                        }
                                    })}
                                />
                                <span>Email Enabled</span>
                            </label>

                            <label className="flex items-center space-x-3 cursor-pointer" style={{ fontSize: '0.9rem' }}>
                                <input
                                    type="checkbox"
                                    style={{ accentColor: '#48bb78', height: '1.2rem', width: '1.2rem' }}
                                    checked={settings.default_preferences?.push_enabled ?? true}
                                    onChange={(e) => setSettings({
                                        ...settings,
                                        default_preferences: {
                                            ...(settings.default_preferences || {}),
                                            push_enabled: e.target.checked
                                        }
                                    })}
                                />
                                <span>Push Enabled</span>
                            </label>

                            <label className="flex items-center space-x-3 cursor-pointer" style={{ fontSize: '0.9rem' }}>
                                <input
                                    type="checkbox"
                                    style={{ accentColor: '#48bb78', height: '1.2rem', width: '1.2rem' }}
                                    checked={settings.default_preferences?.sms_enabled ?? true}
                                    onChange={(e) => setSettings({
                                        ...settings,
                                        default_preferences: {
                                            ...(settings.default_preferences || {}),
                                            sms_enabled: e.target.checked
                                        }
                                    })}
                                />
                                <span>SMS Enabled</span>
                            </label>
                        </div>

                        <div className="mt-12 flex justify-end">
                            <button type="submit" className="btn btn-primary">Save Configuration</button>
                        </div>
                    </form>
                </div>
            )}

            {/* Integration Tab */}
            {activeTab === 'integration' && (
                <div>
                    <div className="card mb-4">
                        <h3 className="mb-4">API Credentials</h3>
                        <div className="form-group mb-6">
                            <label className="form-label">API Key</label>
                            <div className="flex gap-4">
                                <div style={{ position: 'relative', flex: 1 }}>
                                    <input
                                        type={showApiKey ? "text" : "password"}
                                        className="form-input"
                                        value={app.api_key}
                                        readOnly
                                        style={{
                                            background: 'var(--azure-bg)',
                                            border: '1px solid var(--azure-border)',
                                            color: '#605e5c',
                                            paddingRight: '3rem'
                                        }}
                                    />
                                    <button
                                        type="button"
                                        onClick={() => setShowApiKey(!showApiKey)}
                                        style={{
                                            position: 'absolute',
                                            right: '0.75rem',
                                            top: '50%',
                                            transform: 'translateY(-50%)',
                                            background: 'none',
                                            border: 'none',
                                            color: 'var(--azure-blue)',
                                            cursor: 'pointer',
                                            fontSize: '0.8rem',
                                            fontWeight: 600
                                        }}
                                    >
                                        {showApiKey ? 'Hide' : 'Show'}
                                    </button>
                                </div>
                                <button
                                    type="button"
                                    className="btn btn-secondary"
                                    onClick={handleRegenerateKey}
                                >
                                    Regenerate
                                </button>
                            </div>
                            <p style={{ fontSize: '0.8rem', color: '#605e5c', marginTop: '0.5rem' }}>
                                This key is sensitive. Use the toggle to view the full key. Regenerate to get a new full key.
                            </p>
                        </div>
                    </div>

                    <div className="card" style={{ border: '1px solid #f56565' }}>
                        <h3 className="mb-4" style={{ color: '#a4262c', borderColor: '#edebe9' }}>Danger Zone</h3>
                        <p style={{ marginBottom: '1rem', color: '#605e5c', fontSize: '0.9rem' }}>
                            Deleting this application will remove all associated data. This action is irreversible.
                        </p>
                        <button onClick={handleDeleteApp} className="btn btn-danger">
                            Delete Application
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
};

export default AppDetail;
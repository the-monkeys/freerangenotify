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
            setSettings(appData.settings || {});
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
                webhook_url: webhookUrl
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
                <button onClick={() => navigate('/')} style={{ background: 'none', border: 'none', color: '#667eea', cursor: 'pointer', display: 'flex', alignItems: 'center' }}>
                    &larr; Back to Applications
                </button>
                <h1 style={{ fontSize: '2rem', marginTop: '1rem', background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>
                    {app.app_name}
                </h1>
                <p style={{ color: '#718096' }}>ID: {app.app_id}</p>
            </div>

            {/* Tabs */}
            <div style={{ display: 'flex', borderBottom: '1px solid #2d3748', marginBottom: '2rem', overflowX: 'auto', whiteSpace: 'nowrap' }}>
                {(['overview', 'users', 'templates', 'notifications', 'settings', 'integration'] as const).map((tab) => (
                    <button
                        key={tab}
                        onClick={() => setActiveTab(tab)}
                        style={{
                            padding: '1rem 1.5rem',
                            borderBottom: activeTab === tab ? '3px solid #667eea' : '3px solid transparent',
                            color: activeTab === tab ? '#667eea' : '#718096',
                            fontWeight: 600,
                            background: 'none',
                            textTransform: 'capitalize',
                            cursor: 'pointer',
                            flexShrink: 0
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
                            <label className="form-label">Webhook URL</label>
                            <input
                                type="url"
                                className="form-input"
                                value={webhookUrl}
                                onChange={(e) => setWebhookUrl(e.target.value)}
                                placeholder="https://example.com/webhook"
                            />
                        </div>
                        <div className="flex justify-end">
                            <button type="submit" className="btn btn-primary">Save Changes</button>
                        </div>
                    </form>
                </div>
            )}

            {/* Users Tab */}
            {activeTab === 'users' && app && (
                <AppUsers appId={app.app_id} apiKey={app.api_key} />
            )}

            {/* Templates Tab */}
            {activeTab === 'templates' && app && (
                <AppTemplates appId={app.app_id} apiKey={app.api_key} />
            )}

            {/* Notifications Tab */}
            {activeTab === 'notifications' && app && (
                <AppNotifications appId={app.app_id} apiKey={app.api_key} />
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
                        style={{ background: '#1a202c', padding: '1.5rem', borderRadius: '0.5rem' }}
                    >
                        <h4 className="mb-4 text-blue-400">Core Settings</h4>
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

                            <div className="form-group flex flex-col justify-end space-y-4">
                                <label className="flex items-center space-x-3 cursor-pointer">
                                    <input
                                        type="checkbox"
                                        className="form-checkbox h-5 w-5 text-blue-600"
                                        checked={!!settings.enable_webhooks}
                                        onChange={(e) => setSettings({ ...settings, enable_webhooks: e.target.checked })}
                                    />
                                    <span className="text-gray-300">Enable Webhooks</span>
                                </label>
                                <label className="flex items-center space-x-3 cursor-pointer">
                                    <input
                                        type="checkbox"
                                        className="form-checkbox h-5 w-5 text-blue-600"
                                        checked={!!settings.enable_analytics}
                                        onChange={(e) => setSettings({ ...settings, enable_analytics: e.target.checked })}
                                    />
                                    <span className="text-gray-300">Enable Analytics</span>
                                </label>
                            </div>
                        </div>

                        <h4 className="mb-4 text-blue-400">Default Notification Preferences</h4>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
                            <label className="flex items-center space-x-3 cursor-pointer">
                                <input
                                    type="checkbox"
                                    className="form-checkbox h-5 w-5 text-green-600"
                                    checked={settings.default_preferences?.email_enabled ?? true}
                                    onChange={(e) => setSettings({
                                        ...settings,
                                        default_preferences: {
                                            ...(settings.default_preferences || {}),
                                            email_enabled: e.target.checked
                                        }
                                    })}
                                />
                                <span className="text-gray-300">Email Enabled</span>
                            </label>

                            <label className="flex items-center space-x-3 cursor-pointer">
                                <input
                                    type="checkbox"
                                    className="form-checkbox h-5 w-5 text-green-600"
                                    checked={settings.default_preferences?.push_enabled ?? true}
                                    onChange={(e) => setSettings({
                                        ...settings,
                                        default_preferences: {
                                            ...(settings.default_preferences || {}),
                                            push_enabled: e.target.checked
                                        }
                                    })}
                                />
                                <span className="text-gray-300">Push Enabled</span>
                            </label>

                            <label className="flex items-center space-x-3 cursor-pointer">
                                <input
                                    type="checkbox"
                                    className="form-checkbox h-5 w-5 text-green-600"
                                    checked={settings.default_preferences?.sms_enabled ?? true}
                                    onChange={(e) => setSettings({
                                        ...settings,
                                        default_preferences: {
                                            ...(settings.default_preferences || {}),
                                            sms_enabled: e.target.checked
                                        }
                                    })}
                                />
                                <span className="text-gray-300">SMS Enabled</span>
                            </label>
                        </div>

                        <div className="mt-6 flex justify-end">
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
                                <input
                                    type="text"
                                    className="form-input"
                                    value={app.api_key}
                                    readOnly
                                    style={{ background: '#2d3748', border: '1px solid #4a5568', color: '#a0aec0' }}
                                />
                                <button
                                    type="button"
                                    className="btn btn-secondary"
                                    onClick={handleRegenerateKey}
                                >
                                    Regenerate
                                </button>
                            </div>
                            <p style={{ fontSize: '0.875rem', color: '#718096', marginTop: '0.5rem' }}>
                                This key is masked for security. Regenerate to get a new full key.
                            </p>
                        </div>
                    </div>

                    <div className="card" style={{ border: '1px solid #f56565' }}>
                        <h3 className="mb-4" style={{ color: '#f56565' }}>Danger Zone</h3>
                        <p style={{ marginBottom: '1rem', color: '#718096' }}>
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
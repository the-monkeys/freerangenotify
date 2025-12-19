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

    // New setting form state
    const [newSettingKey, setNewSettingKey] = useState('');
    const [newSettingValue, setNewSettingValue] = useState('');

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

    const handleAddSetting = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!id || !newSettingKey) return;
        try {
            const updatedSettings = { ...settings, [newSettingKey]: newSettingValue };
            await applicationsAPI.updateSettings(id, updatedSettings);
            setSettings(updatedSettings);
            setNewSettingKey('');
            setNewSettingValue('');
        } catch (error) {
            console.error('Failed to add setting:', error);
            alert('Failed to add setting');
        }
    };

    const handleRemoveSetting = async (key: string) => {
        if (!id) return;
        try {
            const updatedSettings = { ...settings };
            delete updatedSettings[key];
            await applicationsAPI.updateSettings(id, updatedSettings);
            setSettings(updatedSettings);
        } catch (error) {
            console.error('Failed to remove setting:', error);
            alert('Failed to remove setting');
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
                        Manage key-value settings for this application.
                    </p>

                    <div style={{ marginBottom: '2rem' }}>
                        {Object.keys(settings).length === 0 ? (
                            <p style={{ fontStyle: 'italic', color: '#a0aec0' }}>No settings configured yet.</p>
                        ) : (
                            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                                <thead>
                                    <tr style={{ borderBottom: '1px solid #2d3748', textAlign: 'left' }}>
                                        <th style={{ padding: '0.75rem' }}>Key</th>
                                        <th style={{ padding: '0.75rem' }}>Value</th>
                                        <th style={{ padding: '0.75rem', width: '100px' }}>Actions</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {Object.entries(settings).map(([key, value]) => (
                                        <tr key={key} style={{ borderBottom: '1px solid #1a202c' }}>
                                            <td style={{ padding: '0.75rem' }}>{key}</td>
                                            <td style={{ padding: '0.75rem' }}>{String(value)}</td>
                                            <td style={{ padding: '0.75rem' }}>
                                                <button
                                                    onClick={() => handleRemoveSetting(key)}
                                                    className="btn btn-danger"
                                                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                                                >
                                                    Remove
                                                </button>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        )}
                    </div>

                    <form onSubmit={handleAddSetting} style={{ background: '#1a202c', padding: '1.5rem', borderRadius: '0.5rem' }}>
                        <h4 className="mb-4">Add New Setting</h4>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="form-group" style={{ marginBottom: 0 }}>
                                <input
                                    type="text"
                                    placeholder="Key"
                                    value={newSettingKey}
                                    onChange={(e) => setNewSettingKey(e.target.value)}
                                    className="form-input"
                                    required
                                />
                            </div>
                            <div className="form-group" style={{ marginBottom: 0 }}>
                                <input
                                    type="text"
                                    placeholder="Value"
                                    value={newSettingValue}
                                    onChange={(e) => setNewSettingValue(e.target.value)}
                                    className="form-input"
                                    required
                                />
                            </div>
                        </div>
                        <div className="mt-4 flex justify-end">
                            <button type="submit" className="btn btn-primary">Add Setting</button>
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
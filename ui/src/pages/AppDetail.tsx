import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { applicationsAPI } from '../services/api';
import type { Application, ApplicationSettings } from '../types';
import AppUsers from '../components/AppUsers';
import AppTemplates from '../components/AppTemplates';
import AppNotifications from '../components/AppNotifications';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Textarea } from '../components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { Spinner } from '../components/ui/spinner';
import { Checkbox } from '../components/ui/checkbox';
import { toast } from 'sonner';
import { Copy, Check } from 'lucide-react';

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
    const [copied, setCopied] = useState(false);


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
            toast.success('Application updated successfully!');
        } catch (error) {
            console.error('Failed to update application:', error);
            toast.error('Failed to update application');
        }
    };

    const handleRegenerateKey = async () => {
        if (!id || !window.confirm('Are you sure? The old key will immediately stop working.')) return;
        try {
            await applicationsAPI.regenerateKey(id);
            fetchAppDetails();
            toast.success('API Key regenerated successfully!');
        } catch (error) {
            console.error('Failed to regenerate key:', error);
            toast.error('Failed to regenerate API key');
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

    if (loading) return <div className="flex justify-center items-center min-h-screen"><Spinner /></div>;
    if (!app) return <div className="flex justify-center items-center min-h-screen">Application not found</div>;

    return (
        <div className="container mx-auto px-4 py-6">
            <div className="mb-6">
                <Button
                    onClick={() => navigate('/')}
                    variant="ghost"
                    className="mb-4 px-2"
                >
                    &larr; Back to Applications
                </Button>
                <h1 className="text-2xl font-semibold text-gray-900">
                    {app.app_name}
                </h1>
                <div className="flex items-center mt-2 text-gray-500 text-sm">
                    <span className="mr-2">ID:</span>
                    <code className="bg-gray-100 px-2 py-0.5 rounded font-mono font-semibold">
                        {app.app_id}
                    </code>
                </div>
            </div>

            {/* Tabs */}
            <div className="flex border-b border-gray-200 mb-8 overflow-x-auto whitespace-nowrap">
                {(['overview', 'users', 'templates', 'notifications', 'settings', 'integration'] as const).map((tab) => (
                    <button
                        key={tab}
                        onClick={() => setActiveTab(tab)}
                        className={`px-5 py-3 border-b-2 ${
                            activeTab === tab 
                                ? 'border-blue-600 text-blue-600 font-semibold' 
                                : 'border-transparent text-gray-500'
                        } capitalize text-sm hover:text-blue-600 transition-colors shrink-0`}
                    >
                        {tab}
                    </button>
                ))}
            </div>

            {/* Overview Tab */}
            {activeTab === 'overview' && (
                <Card>
                    <CardHeader>
                        <CardTitle>Application Details</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <form onSubmit={handleUpdateOverview} className="space-y-4">
                            <div className="space-y-2">
                                <Label htmlFor="appName">Application Name</Label>
                                <Input
                                    id="appName"
                                    type="text"
                                    value={appName}
                                    onChange={(e) => setAppName(e.target.value)}
                                    required
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="description">Description</Label>
                                <Textarea
                                    id="description"
                                    className="min-h-[100px]"
                                    value={description}
                                    onChange={(e) => setDescription(e.target.value)}
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="webhookUrl">Webhook URL (Default)</Label>
                                <Input
                                    id="webhookUrl"
                                    type="url"
                                    value={webhookUrl}
                                    onChange={(e) => setWebhookUrl(e.target.value)}
                                    placeholder="https://example.com/webhook"
                                />
                                <p className="text-xs text-gray-500 mt-1">
                                    The default webhook URL used if no named target is specified.
                                </p>
                            </div>

                            {/* Named Webhooks Section */}
                            <div className="mt-8 pt-8 border-t border-gray-200 space-y-4">
                                <div>
                                    <Label className="text-base text-blue-600 block mb-2">
                                        Named Webhook Endpoints
                                    </Label>
                                    <p className="text-sm text-gray-500 mb-6">
                                        Define named webhook targets (e.g., 'slack', 'discord') that templates can use for routing.
                                    </p>
                                </div>

                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="newWebhookName" className="text-xs">Target Name</Label>
                                        <Input
                                            id="newWebhookName"
                                            type="text"
                                            className="text-sm"
                                            value={newWebhookName}
                                            onChange={(e) => setNewWebhookName(e.target.value)}
                                            placeholder="e.g. slack"
                                        />
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="newWebhookUrl" className="text-xs">Webhook URL</Label>
                                        <div className="flex gap-2">
                                            <Input
                                                id="newWebhookUrl"
                                                type="url"
                                                className="text-sm"
                                                value={newWebhookUrl}
                                                onChange={(e) => setNewWebhookUrl(e.target.value)}
                                                placeholder="https://hooks.slack.com/..."
                                            />
                                            <Button
                                                type="button"
                                                variant="secondary"
                                                onClick={() => {
                                                    if (newWebhookName && newWebhookUrl) {
                                                        setWebhooks({ ...webhooks, [newWebhookName]: newWebhookUrl });
                                                        setNewWebhookName('');
                                                        setNewWebhookUrl('');
                                                    }
                                                }}
                                            >
                                                Add
                                            </Button>
                                        </div>
                                    </div>
                                </div>

                                <div className="space-y-3">
                                    {Object.entries(webhooks).map(([name, url]) => (
                                        <div key={name} className="flex items-center justify-between p-3 bg-gray-50 border border-gray-200 rounded">
                                            <div className="flex-1 min-w-0">
                                                <div className="font-semibold text-sm text-blue-600">{name}</div>
                                                <div className="text-xs text-gray-500 overflow-hidden text-ellipsis whitespace-nowrap">{url}</div>
                                            </div>
                                            <Button
                                                type="button"
                                                variant="destructive"
                                                size="sm"
                                                onClick={() => {
                                                    const newWebhooks = { ...webhooks };
                                                    delete newWebhooks[name];
                                                    setWebhooks(newWebhooks);
                                                }}
                                            >
                                                Remove
                                            </Button>
                                        </div>
                                    ))}
                                    {Object.keys(webhooks).length === 0 && (
                                        <p className="text-sm text-gray-400 text-center italic py-4">
                                            No named webhooks configured.
                                        </p>
                                    )}
                                </div>
                            </div>

                            <div className="flex justify-end mt-8">
                                <Button type="submit">Save Overview & Webhooks</Button>
                            </div>
                        </form>
                    </CardContent>
                </Card>
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
                <Card>
                    <CardHeader>
                        <CardTitle>Configuration</CardTitle>
                                    <p className="text-gray-500 text-sm">
                            Manage configuration for this application.
                        </p>
                    </CardHeader>
                    <CardContent>
                        <form
                            onSubmit={async (e) => {
                                e.preventDefault();
                                try {
                                    await applicationsAPI.updateSettings(id!, settings);
                                    toast.success('Settings saved successfully!');
                                } catch (err: any) {
                                    toast.error('Error saving settings: ' + (err.response?.data?.message || err.message));
                                }
                            }}
                            className="space-y-8"
                        >
                            <div>
                                <h4 className="text-base font-semibold text-blue-600 mb-4">Core Settings</h4>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                                    <div className="space-y-2">
                                        <Label htmlFor="rateLimit">Rate Limit (requests/hour)</Label>
                                        <Input
                                            id="rateLimit"
                                            type="number"
                                            value={settings.rate_limit || 0}
                                            onChange={(e) => setSettings({ ...settings, rate_limit: parseInt(e.target.value) || 0 })}
                                            placeholder="e.g. 1000"
                                        />
                                    </div>

                                    <div className="space-y-2">
                                        <Label htmlFor="retryAttempts">Retry Attempts</Label>
                                        <Input
                                            id="retryAttempts"
                                            type="number"
                                            value={settings.retry_attempts || 0}
                                            onChange={(e) => setSettings({ ...settings, retry_attempts: parseInt(e.target.value) || 0 })}
                                            placeholder="e.g. 3"
                                        />
                                    </div>

                                    <div className="space-y-2">
                                        <Label htmlFor="defaultTemplate">Default Template ID</Label>
                                        <Input
                                            id="defaultTemplate"
                                            type="text"
                                            value={settings.default_template || ''}
                                            onChange={(e) => setSettings({ ...settings, default_template: e.target.value })}
                                            placeholder="Template UUID"
                                        />
                                    </div>

                                    <div className="flex flex-col justify-end space-y-4">
                                        <div className="flex items-center space-x-3">
                                            <Checkbox
                                                id="enableWebhooks"
                                                checked={!!settings.enable_webhooks}
                                                onCheckedChange={(checked) => setSettings({ ...settings, enable_webhooks: !!checked })}
                                            />
                                            <Label htmlFor="enableWebhooks" className="font-medium cursor-pointer">
                                                Enable Webhooks
                                            </Label>
                                        </div>
                                        <div className="flex items-center space-x-3">
                                            <Checkbox
                                                id="enableAnalytics"
                                                checked={!!settings.enable_analytics}
                                                onCheckedChange={(checked) => setSettings({ ...settings, enable_analytics: !!checked })}
                                            />
                                            <Label htmlFor="enableAnalytics" className="font-medium cursor-pointer">
                                                Enable Analytics
                                            </Label>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div>
                                <h4 className="text-base font-semibold text-blue-600 mb-4">Authentication & Security</h4>
                                <div className="space-y-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="validationUrl">Validation URL (Zero-Trust API)</Label>
                                        <Input
                                            id="validationUrl"
                                            type="url"
                                            value={settings.validation_url || ''}
                                            onChange={(e) => setSettings({ ...settings, validation_url: e.target.value })}
                                            placeholder="https://your-bank.com/api/verify-token"
                                        />
                                        <p className="text-xs text-gray-500">
                                            If set, FreeRangeNotify will call this URL to verify user tokens before allowing SSE connections.
                                        </p>
                                    </div>

                                    {settings.validation_url && (
                                        <div className="p-4 border border-blue-100 rounded bg-blue-50 space-y-4">
                                            <h5 className="font-semibold text-sm text-blue-800">Validation Request Configuration</h5>

                                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                <div className="space-y-2">
                                                    <Label htmlFor="validationMethod" className="text-xs">Method</Label>
                                                    <Select
                                                        value={settings.validation_config?.method || 'POST'}
                                                        onValueChange={(value) => setSettings({
                                                            ...settings,
                                                            validation_config: {
                                                                ...settings.validation_config,
                                                                method: value,
                                                                token_placement: settings.validation_config?.token_placement || 'body_json',
                                                                token_key: settings.validation_config?.token_key || 'token',
                                                            }
                                                        })}
                                                    >
                                                        <SelectTrigger className="text-sm">
                                                            <SelectValue />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            <SelectItem value="POST">POST</SelectItem>
                                                            <SelectItem value="GET">GET</SelectItem>
                                                        </SelectContent>
                                                    </Select>
                                                </div>
                                                <div className="space-y-2">
                                                    <Label htmlFor="tokenPlacement" className="text-xs">Token Placement</Label>
                                                    <Select
                                                        value={settings.validation_config?.token_placement || 'body_json'}
                                                        onValueChange={(value) => setSettings({
                                                            ...settings,
                                                            validation_config: {
                                                                ...settings.validation_config,
                                                                method: settings.validation_config?.method || 'POST',
                                                                token_placement: value,
                                                                token_key: settings.validation_config?.token_key || 'token',
                                                            }
                                                        })}
                                                    >
                                                        <SelectTrigger className="text-sm">
                                                            <SelectValue />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            <SelectItem value="body_json">Body (JSON)</SelectItem>
                                                            <SelectItem value="body_form">Body (Form URL Encoded)</SelectItem>
                                                            <SelectItem value="header">Header</SelectItem>
                                                            <SelectItem value="query">Query Parameter</SelectItem>
                                                            <SelectItem value="cookie">Cookie</SelectItem>
                                                        </SelectContent>
                                                    </Select>
                                                </div>
                                                <div className="space-y-2 col-span-2">
                                                    <Label htmlFor="tokenKey" className="text-xs">Token Key Name</Label>
                                                    <Input
                                                        id="tokenKey"
                                                        type="text"
                                                        className="text-sm"
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
                                                    <p className="text-xs text-gray-500">The name of the header, cookie, or field that contains the token.</p>
                                                </div>
                                            </div>

                                            <div className="space-y-2">
                                                <Label htmlFor="staticHeaders" className="text-xs">Static Headers (e.g., Client-ID, User-Agent)</Label>
                                                <Textarea
                                                    id="staticHeaders"
                                                    className="text-sm font-mono"
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
                                                <p className="text-xs text-gray-500">One header per line. Format: Header-Name: Value</p>
                                            </div>
                                        </div>
                                    )}
                                </div>
                            </div>

                            <div>
                                <h4 className="text-base font-semibold text-blue-600 mb-4">Default Notification Preferences</h4>
                                <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                                    <div className="flex items-center space-x-3">
                                        <Checkbox
                                            id="emailEnabled"
                                            checked={settings.default_preferences?.email_enabled ?? true}
                                            onCheckedChange={(checked) => setSettings({
                                                ...settings,
                                                default_preferences: {
                                                    ...(settings.default_preferences || {}),
                                                    email_enabled: !!checked
                                                }
                                            })}
                                        />
                                        <Label htmlFor="emailEnabled" className="cursor-pointer">Email Enabled</Label>
                                    </div>

                                    <div className="flex items-center space-x-3">
                                        <Checkbox
                                            id="pushEnabled"
                                            checked={settings.default_preferences?.push_enabled ?? true}
                                            onCheckedChange={(checked) => setSettings({
                                                ...settings,
                                                default_preferences: {
                                                    ...(settings.default_preferences || {}),
                                                    push_enabled: !!checked
                                                }
                                            })}
                                        />
                                        <Label htmlFor="pushEnabled" className="cursor-pointer">Push Enabled</Label>
                                    </div>

                                    <div className="flex items-center space-x-3">
                                        <Checkbox
                                            id="smsEnabled"
                                            checked={settings.default_preferences?.sms_enabled ?? true}
                                            onCheckedChange={(checked) => setSettings({
                                                ...settings,
                                                default_preferences: {
                                                    ...(settings.default_preferences || {}),
                                                    sms_enabled: !!checked
                                                }
                                            })}
                                        />
                                        <Label htmlFor="smsEnabled" className="cursor-pointer">SMS Enabled</Label>
                                    </div>
                                </div>
                            </div>

                            <div className="flex justify-end pt-4">
                                <Button type="submit">Save Configuration</Button>
                            </div>
                        </form>
                    </CardContent>
                </Card>
            )}

            {/* Integration Tab */}
            {activeTab === 'integration' && (
                <div className="space-y-4">
                    <Card>
                        <CardHeader>
                            <CardTitle>API Credentials</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="space-y-2 mb-6">
                                <Label htmlFor="apiKey">API Key</Label>
                                <div className="flex gap-2">
                                    <div className="relative flex-1">
                                        <Input
                                            id="apiKey"
                                            type={showApiKey ? "text" : "password"}
                                            value={app.api_key}
                                            readOnly
                                            className="bg-gray-50 border-gray-200 text-gray-500 pr-24"
                                        />
                                        <div className="absolute right-2 top-1/2 -translate-y-1/2 flex gap-1">
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="sm"
                                                onClick={async () => {
                                                    try {
                                                        await navigator.clipboard.writeText(app.api_key);
                                                        setCopied(true);
                                                        toast.success('API key copied to clipboard!');
                                                        setTimeout(() => setCopied(false), 2000);
                                                    } catch (err) {
                                                        toast.error('Failed to copy API key');
                                                    }
                                                }}
                                                className="h-8 w-8 p-0"
                                            >
                                                {copied ? (
                                                    <Check className="h-4 w-4 text-green-600" />
                                                ) : (
                                                    <Copy className="h-4 w-4" />
                                                )}
                                            </Button>
                                            <button
                                                type="button"
                                                onClick={() => setShowApiKey(!showApiKey)}
                                                className="text-blue-600 text-xs font-semibold hover:underline px-2"
                                            >
                                                {showApiKey ? 'Hide' : 'Show'}
                                            </button>
                                        </div>
                                    </div>
                                    <Button
                                        type="button"
                                        variant="secondary"
                                        onClick={handleRegenerateKey}
                                    >
                                        Regenerate
                                    </Button>
                                </div>
                                <p className="text-xs text-gray-500">
                                    This key is sensitive. Use the toggle to view the full key. Regenerate to get a new full key.
                                </p>
                            </div>
                        </CardContent>
                    </Card>

                    <Card className="border-red-300">
                        <CardHeader>
                            <CardTitle className="text-red-600">Danger Zone</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <p className="mb-4 text-gray-500 text-sm">
                                Deleting this application will remove all associated data. This action is irreversible.
                            </p>
                            <Button onClick={handleDeleteApp} variant="destructive">
                                Delete Application
                            </Button>
                        </CardContent>
                    </Card>
                </div>
            )}
        </div>
    );
};

export default AppDetail;
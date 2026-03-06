import React, { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { applicationsAPI, providersAPI } from '../services/api';
import type { Application, ApplicationSettings } from '../types';
import AppUsers from '../components/AppUsers';
import AppTemplates from '../components/AppTemplates';
import AppNotifications from '../components/AppNotifications';
import AppTeam from '../components/apps/AppTeam';
import AppProviders from '../components/apps/AppProviders';
import AppEnvironments from '../components/apps/AppEnvironments';
import DigestRulesList from './digest/DigestRulesList';
import TopicsList from './topics/TopicsList';

import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Textarea } from '../components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { Spinner } from '../components/ui/spinner';
import { Checkbox } from '../components/ui/checkbox';
import { toast } from 'sonner';
import { Copy, Check, LayoutDashboard, Users, FileText, Bell, Layers, MessageSquare, UsersRound, Plug, GitBranch, Settings, Code, Workflow, ArrowRight, Timer, Zap, Mail, Route } from 'lucide-react';
import { Badge } from '../components/ui/badge';

type TabId = 'overview' | 'users' | 'templates' | 'notifications' | 'digest-rules' | 'workflows' | 'topics' | 'team' | 'providers' | 'environments' | 'settings' | 'integration';

const VALID_TABS: TabId[] = ['overview', 'users', 'templates', 'notifications', 'digest-rules', 'workflows', 'topics', 'team', 'providers', 'environments', 'settings', 'integration'];

interface TabDef {
    id: TabId;
    label: string;
    icon: React.ReactNode;
}

const TAB_GROUPS: { label: string; tabs: TabDef[] }[] = [
    {
        label: 'General',
        tabs: [
            { id: 'overview', label: 'Overview', icon: <LayoutDashboard className="h-4 w-4" /> },
            { id: 'users', label: 'Users', icon: <Users className="h-4 w-4" /> },
            { id: 'templates', label: 'Templates', icon: <FileText className="h-4 w-4" /> },
            { id: 'notifications', label: 'Notifications', icon: <Bell className="h-4 w-4" /> },
        ],
    },
    {
        label: 'Configuration',
        tabs: [
            { id: 'digest-rules', label: 'Digest Rules', icon: <Layers className="h-4 w-4" /> },
            { id: 'workflows', label: 'Workflows', icon: <Workflow className="h-4 w-4" /> },
            { id: 'topics', label: 'Topics', icon: <MessageSquare className="h-4 w-4" /> },
            { id: 'team', label: 'Team', icon: <UsersRound className="h-4 w-4" /> },
            { id: 'providers', label: 'Providers', icon: <Plug className="h-4 w-4" /> },
        ],
    },
    {
        label: 'Advanced',
        tabs: [
            { id: 'environments', label: 'Environments', icon: <GitBranch className="h-4 w-4" /> },
            { id: 'settings', label: 'Settings', icon: <Settings className="h-4 w-4" /> },
            { id: 'integration', label: 'Integration', icon: <Code className="h-4 w-4" /> },
        ],
    },
];

const ALL_TABS: TabDef[] = TAB_GROUPS.flatMap(g => g.tabs);

const AppDetail: React.FC = () => {
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const [searchParams, setSearchParams] = useSearchParams();
    const [app, setApp] = useState<Application | null>(null);
    const [loading, setLoading] = useState(true);

    const tabParam = searchParams.get('tab') as TabId | null;
    const activeTab: TabId = tabParam && VALID_TABS.includes(tabParam) ? tabParam : 'overview';

    const setActiveTab = useCallback((tab: TabId) => {
        setSearchParams({ tab }, { replace: true });
    }, [setSearchParams]);

    // Local state for editing
    const [appName, setAppName] = useState('');
    const [description, setDescription] = useState('');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [settings, setSettings] = useState<ApplicationSettings>({});
    const [webhooks, setWebhooks] = useState<Record<string, string>>({});

    // Fetch webhook endpoints from registered providers
    const fetchWebhookEndpoints = async () => {
        if (!id) return;
        try {
            const providers = await providersAPI.list(id);
            const webhookMap: Record<string, string> = {};
            (providers || []).filter(p => p.channel === 'webhook' && p.active).forEach(p => {
                webhookMap[p.name] = p.webhook_url;
            });
            setWebhooks(webhookMap);
        } catch {
            // Providers may not be available, keep empty
        }
    };
    const [staticHeadersText, setStaticHeadersText] = useState('');
    const [showApiKey, setShowApiKey] = useState(false);
    const [copied, setCopied] = useState(false);
    const [unreadCount, setUnreadCount] = useState(0);


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
            // Fetch webhook endpoints from providers
            fetchWebhookEndpoints();

            // Persist for standalone pages
            localStorage.setItem('last_api_key', appData.api_key);
            localStorage.setItem('last_app_id', appData.app_id);

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
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
            <div className="mb-6">
                <Button
                    onClick={() => navigate('/')}
                    variant="ghost"
                    className="mb-4 px-2"
                >
                    &larr; Back to Applications
                </Button>
                <h1 className="text-xl sm:text-2xl font-semibold text-foreground">
                    {app.app_name}
                </h1>
                <div className="flex items-center mt-2 text-muted-foreground text-sm">
                    <span className="mr-2">ID:</span>
                    <code className="bg-muted px-2 py-0.5 rounded font-mono font-semibold text-xs sm:text-sm break-all">
                        {app.app_id}
                    </code>
                </div>
            </div>

            <>
                {/* Tabs — mobile dropdown */}
                <div className="md:hidden mb-6">
                    <Select value={activeTab} onValueChange={(val) => {
                        setActiveTab(val as TabId);
                        if (val === 'notifications') fetchWebhookEndpoints();
                    }}>
                        <SelectTrigger className="w-full">
                            <SelectValue>
                                <span className="inline-flex items-center gap-2">
                                    {ALL_TABS.find(t => t.id === activeTab)?.icon}
                                    {ALL_TABS.find(t => t.id === activeTab)?.label}
                                    {activeTab === 'notifications' && unreadCount > 0 && (
                                        <Badge variant="destructive" className="text-[10px] px-1.5 py-0 h-4 min-w-4">
                                            {unreadCount > 99 ? '99+' : unreadCount}
                                        </Badge>
                                    )}
                                </span>
                            </SelectValue>
                        </SelectTrigger>
                        <SelectContent>
                            {TAB_GROUPS.map((group) => (
                                <React.Fragment key={group.label}>
                                    <div className="px-2 py-1.5 text-xs font-semibold text-muted-foreground">{group.label}</div>
                                    {group.tabs.map((tab) => (
                                        <SelectItem key={tab.id} value={tab.id}>
                                            <span className="inline-flex items-center gap-2">
                                                {tab.icon}
                                                {tab.label}
                                                {tab.id === 'notifications' && unreadCount > 0 && (
                                                    <Badge variant="destructive" className="text-[10px] px-1.5 py-0 h-4 min-w-4">
                                                        {unreadCount > 99 ? '99+' : unreadCount}
                                                    </Badge>
                                                )}
                                            </span>
                                        </SelectItem>
                                    ))}
                                </React.Fragment>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

                {/* Tabs — desktop grouped */}
                <div className="hidden md:block mb-6 sm:mb-8">
                    <div className="flex flex-wrap gap-x-6 gap-y-1 border-b border-border">
                        {TAB_GROUPS.map((group, gi) => (
                            <div key={group.label} className="flex items-end">
                                {gi > 0 && (
                                    <div className="h-6 w-px bg-border mr-6 mb-2" />
                                )}
                                <div className="flex">
                                    {group.tabs.map((tab) => (
                                        <button
                                            key={tab.id}
                                            onClick={() => {
                                                setActiveTab(tab.id);
                                                if (tab.id === 'notifications') fetchWebhookEndpoints();
                                            }}
                                            className={`px-3 lg:px-4 py-2.5 border-b-2 -mb-px ${activeTab === tab.id
                                                ? 'border-foreground text-foreground font-semibold'
                                                : 'border-transparent text-muted-foreground'
                                                } text-sm hover:text-foreground transition-colors inline-flex items-center gap-1.5`}
                                        >
                                            {tab.icon}
                                            <span className="hidden lg:inline">{tab.label}</span>
                                            <span className="lg:hidden">{tab.label}</span>
                                            {tab.id === 'notifications' && unreadCount > 0 && (
                                                <Badge variant="destructive" className="text-[10px] px-1.5 py-0 h-4 min-w-4 inline-flex items-center justify-center">
                                                    {unreadCount > 99 ? '99+' : unreadCount}
                                                </Badge>
                                            )}
                                        </button>
                                    ))}
                                </div>
                            </div>
                        ))}
                    </div>
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

                                <div className="flex justify-end mt-8">
                                    <Button type="submit">Save Overview</Button>
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
                    <AppNotifications appId={app.app_id} apiKey={app.api_key} webhooks={webhooks} onUnreadCount={setUnreadCount} />
                )}

                {/* Settings Tab */}
                {activeTab === 'settings' && (
                    <Card>
                        <CardHeader>
                            <CardTitle>Configuration</CardTitle>
                            <p className="text-muted-foreground text-sm">
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
                                    <h4 className="text-base font-semibold text-foreground mb-4">Core Settings</h4>
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
                                    <h4 className="text-base font-semibold text-foreground mb-4">Authentication & Security</h4>
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
                                            <p className="text-xs text-muted-foreground">
                                                If set, FreeRangeNotify will call this URL to verify user tokens before allowing SSE connections.
                                            </p>
                                        </div>

                                        {settings.validation_url && (
                                            <div className="p-4 border border-border rounded bg-muted space-y-4">
                                                <h5 className="font-semibold text-sm text-foreground">Validation Request Configuration</h5>

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
                                                    <div className="space-y-2 md:col-span-2">
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
                                                        <p className="text-xs text-muted-foreground">The name of the header, cookie, or field that contains the token.</p>
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
                                                    <p className="text-xs text-muted-foreground">One header per line. Format: Header-Name: Value</p>
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                </div>

                                <div>
                                    <h4 className="text-base font-semibold text-foreground mb-4">Email Channel Configuration</h4>
                                    <div className="p-4 border border-border rounded bg-muted mb-8 space-y-6">
                                        <div className="space-y-2">
                                            <Label htmlFor="emailProvider">Email Provider</Label>
                                            <Select
                                                value={settings.email_config?.provider_type || 'system'}
                                                onValueChange={(value: string) => setSettings({
                                                    ...settings,
                                                    email_config: {
                                                        ...settings.email_config,
                                                        provider_type: value as any
                                                    }
                                                })}
                                            >
                                                <SelectTrigger id="emailProvider">
                                                    <SelectValue />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    <SelectItem value="system">System Default (Global SMTP/SendGrid)</SelectItem>
                                                    <SelectItem value="smtp">Custom SMTP (Direct Gmail, Outlook, etc.)</SelectItem>
                                                    <SelectItem value="sendgrid">Custom SendGrid API</SelectItem>
                                                </SelectContent>
                                            </Select>
                                            <p className="text-xs text-muted-foreground">
                                                Choose how emails are sent for this application.
                                            </p>
                                        </div>

                                        <div className="space-y-2">
                                            <Label htmlFor="dailyEmailLimit">Daily Email Limit</Label>
                                            <Input
                                                id="dailyEmailLimit"
                                                type="number"
                                                value={settings.daily_email_limit || 0}
                                                onChange={(e) => setSettings({ ...settings, daily_email_limit: parseInt(e.target.value) || 0 })}
                                                placeholder="e.g. 100"
                                            />
                                            <p className="text-xs text-muted-foreground">
                                                Maximum number of emails this application can send per day. Set to 0 for unlimited.
                                            </p>
                                        </div>

                                        {settings.email_config?.provider_type === 'smtp' && (
                                            <div className="mt-4 p-4 bg-card border border-border rounded space-y-4">
                                                <h5 className="font-semibold text-sm text-foreground mb-2">SMTP Settings</h5>
                                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                    <div className="space-y-2">
                                                        <Label htmlFor="smtpHost" className="text-xs">SMTP Host</Label>
                                                        <Input
                                                            id="smtpHost"
                                                            type="text"
                                                            className="text-sm"
                                                            value={settings.email_config?.smtp_config?.host || ''}
                                                            onChange={(e) => setSettings({
                                                                ...settings,
                                                                email_config: {
                                                                    ...settings.email_config!,
                                                                    smtp_config: { ...settings.email_config?.smtp_config, host: e.target.value } as any
                                                                }
                                                            })}
                                                            placeholder="smtp.gmail.com"
                                                        />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <Label htmlFor="smtpPort" className="text-xs">SMTP Port</Label>
                                                        <Input
                                                            id="smtpPort"
                                                            type="number"
                                                            className="text-sm"
                                                            value={settings.email_config?.smtp_config?.port || 587}
                                                            onChange={(e) => setSettings({
                                                                ...settings,
                                                                email_config: {
                                                                    ...settings.email_config!,
                                                                    smtp_config: { ...settings.email_config?.smtp_config, port: parseInt(e.target.value) || 587 } as any
                                                                }
                                                            })}
                                                        />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <Label htmlFor="smtpUsername" className="text-xs">Username</Label>
                                                        <Input
                                                            id="smtpUsername"
                                                            type="text"
                                                            className="text-sm"
                                                            value={settings.email_config?.smtp_config?.username || ''}
                                                            onChange={(e) => setSettings({
                                                                ...settings,
                                                                email_config: {
                                                                    ...settings.email_config!,
                                                                    smtp_config: { ...settings.email_config?.smtp_config, username: e.target.value } as any
                                                                }
                                                            })}
                                                        />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <Label htmlFor="smtpPassword" className="text-xs">Password / App Password</Label>
                                                        <Input
                                                            id="smtpPassword"
                                                            type="password"
                                                            className="text-sm"
                                                            value={settings.email_config?.smtp_config?.password || ''}
                                                            onChange={(e) => setSettings({
                                                                ...settings,
                                                                email_config: {
                                                                    ...settings.email_config!,
                                                                    smtp_config: { ...settings.email_config?.smtp_config, password: e.target.value } as any
                                                                }
                                                            })}
                                                        />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <Label htmlFor="smtpFromEmail" className="text-xs">From Email Address</Label>
                                                        <Input
                                                            id="smtpFromEmail"
                                                            type="email"
                                                            className="text-sm"
                                                            value={settings.email_config?.smtp_config?.from_email || ''}
                                                            onChange={(e) => setSettings({
                                                                ...settings,
                                                                email_config: {
                                                                    ...settings.email_config!,
                                                                    smtp_config: { ...settings.email_config?.smtp_config, from_email: e.target.value } as any
                                                                }
                                                            })}
                                                        />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <Label htmlFor="smtpFromName" className="text-xs">From Display Name</Label>
                                                        <Input
                                                            id="smtpFromName"
                                                            type="text"
                                                            className="text-sm"
                                                            value={settings.email_config?.smtp_config?.from_name || ''}
                                                            onChange={(e) => setSettings({
                                                                ...settings,
                                                                email_config: {
                                                                    ...settings.email_config!,
                                                                    smtp_config: { ...settings.email_config?.smtp_config, from_name: e.target.value } as any
                                                                }
                                                            })}
                                                        />
                                                    </div>
                                                </div>
                                            </div>
                                        )}

                                        {settings.email_config?.provider_type === 'sendgrid' && (
                                            <div className="mt-4 p-4 bg-card border border-border rounded space-y-4">
                                                <h5 className="font-semibold text-sm text-foreground mb-2">SendGrid Settings</h5>
                                                <div className="space-y-4">
                                                    <div className="space-y-2">
                                                        <Label htmlFor="sendgridKey" className="text-xs">SendGrid API Key</Label>
                                                        <Input
                                                            id="sendgridKey"
                                                            type="password"
                                                            className="text-sm"
                                                            value={settings.email_config?.sendgrid_config?.api_key || ''}
                                                            onChange={(e) => setSettings({
                                                                ...settings,
                                                                email_config: {
                                                                    ...settings.email_config!,
                                                                    sendgrid_config: { ...settings.email_config?.sendgrid_config, api_key: e.target.value } as any
                                                                }
                                                            })}
                                                        />
                                                    </div>
                                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                        <div className="space-y-2">
                                                            <Label htmlFor="sendgridFromEmail" className="text-xs">From Email Address</Label>
                                                            <Input
                                                                id="sendgridFromEmail"
                                                                type="email"
                                                                className="text-sm"
                                                                value={settings.email_config?.sendgrid_config?.from_email || ''}
                                                                onChange={(e) => setSettings({
                                                                    ...settings,
                                                                    email_config: {
                                                                        ...settings.email_config!,
                                                                        sendgrid_config: { ...settings.email_config?.sendgrid_config, from_email: e.target.value } as any
                                                                    }
                                                                })}
                                                            />
                                                        </div>
                                                        <div className="space-y-2">
                                                            <Label htmlFor="sendgridFromName" className="text-xs">From Display Name</Label>
                                                            <Input
                                                                id="sendgridFromName"
                                                                type="text"
                                                                className="text-sm"
                                                                value={settings.email_config?.sendgrid_config?.from_name || ''}
                                                                onChange={(e) => setSettings({
                                                                    ...settings,
                                                                    email_config: {
                                                                        ...settings.email_config!,
                                                                        sendgrid_config: { ...settings.email_config?.sendgrid_config, from_name: e.target.value } as any
                                                                    }
                                                                })}
                                                            />
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                </div>

                                <div>
                                    <h4 className="text-base font-semibold text-foreground mb-4">Default Notification Preferences</h4>
                                    <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                                        <div className="flex items-center space-x-3">
                                            <Checkbox
                                                id="emailEnabled"
                                                checked={settings.default_preferences?.email_enabled ?? true}
                                                onCheckedChange={(checked: boolean) => setSettings({
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
                                                onCheckedChange={(checked: boolean) => setSettings({
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
                                                onCheckedChange={(checked: boolean) => setSettings({
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
                                    <div className="flex justify-end pt-4">
                                        <Button type="submit">Save Configuration</Button>
                                    </div>
                                </div>
                            </form>
                        </CardContent>
                    </Card>
                )}

                {/* Digest Rules Tab */}
                {activeTab === 'digest-rules' && app && (
                    <div className="space-y-6">
                        <Card>
                            <CardContent className="pt-6">
                                <div className="flex items-start gap-4">
                                    <div className="rounded-lg bg-primary/10 p-3 shrink-0">
                                        <Timer className="h-6 w-6 text-primary" />
                                    </div>
                                    <div className="space-y-3">
                                        <div>
                                            <h3 className="font-semibold text-lg">What are Digest Rules?</h3>
                                            <p className="text-muted-foreground text-sm mt-1">
                                                Digest rules <strong>batch multiple notifications together</strong> instead of sending them one by one.
                                                Think of it like a mailbox, instead of delivering every letter the moment it arrives,
                                                you collect them and deliver once at a set interval.
                                            </p>
                                        </div>
                                        <div className="grid sm:grid-cols-3 gap-3 text-sm">
                                            <div className="rounded-md border p-3 space-y-1">
                                                <div className="font-medium flex items-center gap-1.5"><Zap className="h-3.5 w-3.5 text-amber-500" /> Without Digest</div>
                                                <p className="text-muted-foreground text-xs">20 events = 20 separate emails sent instantly. That's noisy.</p>
                                            </div>
                                            <div className="rounded-md border p-3 space-y-1">
                                                <div className="font-medium flex items-center gap-1.5"><Mail className="h-3.5 w-3.5 text-blue-500" /> With Digest</div>
                                                <p className="text-muted-foreground text-xs">20 events = 1 consolidated email sent after the time window ends.</p>
                                            </div>
                                            <div className="rounded-md border p-3 space-y-1">
                                                <div className="font-medium flex items-center gap-1.5"><Route className="h-3.5 w-3.5 text-green-500" /> How It Works</div>
                                                <p className="text-muted-foreground text-xs">Set a <code className="bg-muted px-1 rounded">digest_key</code> in your notification metadata. Matching notifications are batched and sent after the window expires.</p>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                        <DigestRulesList apiKey={app.api_key} embedded />
                    </div>
                )}

                {/* Topics Tab */}
                {activeTab === 'topics' && app && (
                    <TopicsList apiKey={app.api_key} embedded />
                )}

                {/* Workflows Tab */}
                {activeTab === 'workflows' && app && (
                    <div className="space-y-6">
                        <Card>
                            <CardContent className="pt-6">
                                <div className="flex items-start gap-4">
                                    <div className="rounded-lg bg-primary/10 p-3 shrink-0">
                                        <Workflow className="h-6 w-6 text-primary" />
                                    </div>
                                    <div className="space-y-3">
                                        <div>
                                            <h3 className="font-semibold text-lg">What are Workflows?</h3>
                                            <p className="text-muted-foreground text-sm mt-1">
                                                Workflows are <strong>multi-step notification pipelines</strong> that go beyond simple one-time sends.
                                                Instead of firing a single notification, you can build a sequence of steps — send an email,
                                                wait 2 hours, check if it was read, then send a push notification as a follow-up.
                                            </p>
                                        </div>
                                        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-3 text-sm">
                                            <div className="rounded-md border p-3 space-y-1">
                                                <div className="font-medium">Channel Step</div>
                                                <p className="text-muted-foreground text-xs">Send via email, SMS, push, webhook, or SSE using a template.</p>
                                            </div>
                                            <div className="rounded-md border p-3 space-y-1">
                                                <div className="font-medium">Delay Step</div>
                                                <p className="text-muted-foreground text-xs">Pause the workflow for a set duration (e.g. wait 1 hour before the next step).</p>
                                            </div>
                                            <div className="rounded-md border p-3 space-y-1">
                                                <div className="font-medium">Digest Step</div>
                                                <p className="text-muted-foreground text-xs">Batch and aggregate events over a time window before sending.</p>
                                            </div>
                                            <div className="rounded-md border p-3 space-y-1">
                                                <div className="font-medium">Condition Step</div>
                                                <p className="text-muted-foreground text-xs">Branch logic — skip or route to different steps based on event data.</p>
                                            </div>
                                        </div>
                                        <div>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => navigate('/workflows')}
                                                className="gap-2"
                                            >
                                                Open Workflow Builder <ArrowRight className="h-4 w-4" />
                                            </Button>
                                        </div>
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                )}

                {/* Team Tab */}
                {activeTab === 'team' && app && (
                    <AppTeam appId={app.app_id} />
                )}

                {/* Providers Tab */}
                {activeTab === 'providers' && app && (
                    <AppProviders appId={app.app_id} />
                )}

                {/* Environments Tab */}
                {activeTab === 'environments' && app && (
                    <AppEnvironments
                        appId={app.app_id}
                        currentApiKey={app.api_key}
                        onApiKeyChange={(_apiKey, envName) => {
                            toast.success(`Switched to ${envName} environment`);
                            // Re-fetch app details to reflect the new API key context
                            fetchAppDetails();
                        }}
                    />
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
                                    <div className="flex flex-col sm:flex-row gap-2">
                                        <div className="relative flex-1">
                                            <Input
                                                id="apiKey"
                                                type={showApiKey ? "text" : "password"}
                                                value={app.api_key}
                                                readOnly
                                                className="bg-muted border-border text-muted-foreground pr-24"
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
                                                    className="text-foreground text-xs font-semibold hover:underline px-2"
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
                                    <p className="text-xs text-muted-foreground">
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
                                <p className="mb-4 text-muted-foreground text-sm">
                                    Deleting this application will remove all associated data. This action is irreversible.
                                </p>
                                <Button onClick={handleDeleteApp} variant="destructive">
                                    Delete Application
                                </Button>
                            </CardContent>
                        </Card>
                    </div>
                )}
            </>
        </div>
    );
};

export default AppDetail;
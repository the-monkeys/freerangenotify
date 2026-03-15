import React, { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { applicationsAPI, providersAPI, workflowsAPI } from '../services/api';
import type { Application, ApplicationSettings, Workflow as WorkflowType } from '../types';
import AppUsers from '../components/AppUsers';
import AppTemplates from '../components/AppTemplates';
import AppNotifications from '../components/AppNotifications';
import AppTeam from '../components/apps/AppTeam';
import AppProviders from '../components/apps/AppProviders';
import AppEnvironments from '../components/apps/AppEnvironments';
import AppImport from '../components/apps/AppImport';
import DigestRulesList from './digest/DigestRulesList';
import TopicsList from './topics/TopicsList';
import SchedulesList from './schedules/SchedulesList';
import WorkflowsList from './workflows/WorkflowsList';

import { Button } from '../components/ui/button';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Textarea } from '../components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { Spinner } from '../components/ui/spinner';
import { Checkbox } from '../components/ui/checkbox';
import { SidebarGroup, SidebarGroupContent, SidebarGroupLabel, SidebarMenu, SidebarMenuItem } from '../components/ui/sidebar';
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '../components/ui/accordion';
import { toast } from 'sonner';
import { Copy, Check, LayoutDashboard, Users, FileText, Bell, Layers, MessageSquare, UsersRound, Plug, GitBranch, Settings, Code, Workflow, Timer, Zap, Mail, Route, Link2 } from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { useApiQuery } from '../hooks/use-api-query';
import ConfirmDeleteDialog from '../components/ConfirmDeleteDialog';

type TabId = 'overview' | 'users' | 'templates' | 'notifications' | 'digest-rules' | 'workflows' | 'schedules' | 'topics' | 'team' | 'providers' | 'environments' | 'settings' | 'integration' | 'import';

const VALID_TABS: TabId[] = ['overview', 'users', 'templates', 'notifications', 'digest-rules', 'workflows', 'schedules', 'topics', 'team', 'providers', 'environments', 'settings', 'integration', 'import'];

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
            { id: 'schedules', label: 'Schedules', icon: <Timer className="h-4 w-4" /> },
            { id: 'topics', label: 'Topics', icon: <MessageSquare className="h-4 w-4" /> },
            { id: 'team', label: 'Team', icon: <UsersRound className="h-4 w-4" /> },
            { id: 'providers', label: 'Providers', icon: <Plug className="h-4 w-4" /> },
            { id: 'import', label: 'Import', icon: <Link2 className="h-4 w-4" /> },
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
    const [eventMappingText, setEventMappingText] = useState('');
    const [showApiKey, setShowApiKey] = useState(false);
    const [copied, setCopied] = useState(false);
    const [unreadCount, setUnreadCount] = useState(0);
    const [confirmAction, setConfirmAction] = useState<'regenerate-key' | 'delete-app' | null>(null);
    const [confirmLoading, setConfirmLoading] = useState(false);

    const { data: workflowsData } = useApiQuery(
        () => workflowsAPI.list(app!.api_key, 100, 0),
        [app?.api_key, activeTab],
        {
            enabled: !!app?.api_key && activeTab === 'settings',
            cacheKey: `app-workflows-settings-${app?.app_id}`
        }
    );
    const workflows: WorkflowType[] = workflowsData?.workflows ?? [];


    useEffect(() => {
        if (!id) return;
        let ignore = false;
        const doFetch = async () => {
            setLoading(true);
            try {
                const appData = await applicationsAPI.get(id);
                if (ignore) return;
                setApp(appData);
                setAppName(appData.app_name);
                setDescription(appData.description || '');
                setWebhookUrl(appData.webhook_url || '');
                setSettings(appData.settings || {});
                fetchWebhookEndpoints();
                localStorage.setItem('last_api_key', appData.api_key);
                localStorage.setItem('last_app_id', appData.app_id);
                localStorage.setItem('last_app_name', appData.app_name);
                window.dispatchEvent(new CustomEvent('app-name-updated', { detail: appData.app_name }));
                const text = Object.entries(appData.settings?.validation_config?.static_headers || {})
                    .map(([k, v]) => `${k}: ${v}`).join('\n');
                setStaticHeadersText(text);
                setEventMappingText(JSON.stringify(appData.settings?.inbound_webhook_config?.event_mapping || {}, null, 2));
            } catch (error) {
                console.error('Failed to fetch app details:', error);
            } finally {
                if (!ignore) setLoading(false);
            }
        };
        doFetch();
        return () => { ignore = true; };
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
            localStorage.setItem('last_app_name', appData.app_name);
            window.dispatchEvent(new CustomEvent('app-name-updated', { detail: appData.app_name }));

            // Initialize static headers text
            const text = Object.entries(appData.settings?.validation_config?.static_headers || {})
                .map(([k, v]) => `${k}: ${v}`).join('\n');
            setStaticHeadersText(text);
            setEventMappingText(JSON.stringify(appData.settings?.inbound_webhook_config?.event_mapping || {}, null, 2));
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
            localStorage.setItem('last_app_name', updated.app_name);
            window.dispatchEvent(new CustomEvent('app-name-updated', { detail: updated.app_name }));
            toast.success('Application updated successfully!');
        } catch (error) {
            console.error('Failed to update application:', error);
            toast.error('Failed to update application');
        }
    };

    const handleRegenerateKey = async () => {
        if (!id) return;
        setConfirmLoading(true);
        try {
            await applicationsAPI.regenerateKey(id);
            fetchAppDetails();
            toast.success('API Key regenerated successfully!');
            setConfirmAction(null);
        } catch (error) {
            console.error('Failed to regenerate key:', error);
            toast.error('Failed to regenerate API key');
        } finally {
            setConfirmLoading(false);
        }
    };

    const handleDeleteApp = async () => {
        if (!id) return;
        setConfirmLoading(true);
        try {
            await applicationsAPI.delete(id);
            setConfirmAction(null);
            navigate('/');
        } catch (error) {
            console.error('Failed to delete application:', error);
            toast.error('Failed to delete application');
        } finally {
            setConfirmLoading(false);
        }
    };

    const handleTabChange = (tab: TabId) => {
        setActiveTab(tab);
        if (tab === 'notifications') {
            fetchWebhookEndpoints();
        }
    };

    if (loading) return <div className="flex justify-center items-center min-h-screen"><Spinner /></div>;
    if (!app) return <div className="flex justify-center items-center min-h-screen">Application not found</div>;

    return (
        <div className="mx-auto max-w-7xl">
            <>
                {/* Tabs — mobile dropdown */}
                <Card size="sm" className="mb-6 bg-card/60 shadow-sm md:hidden">
                    <CardContent className="p-2">
                        <p className="px-2 pb-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
                            Section
                        </p>
                        <Select value={activeTab} onValueChange={(val) => handleTabChange(val as TabId)}>
                            <SelectTrigger className="h-10 w-full border-0 bg-transparent px-2 text-left shadow-none">
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
                    </CardContent>
                </Card>

                <div className="grid gap-6 md:grid-cols-[260px_minmax(0,1fr)]">
                    {/* Options sidebar — desktop */}
                    <aside className="hidden md:block">
                        <Card size="sm" className="sticky top-20 bg-card/60 shadow-sm backdrop-blur-sm">
                            <CardContent className="p-2">
                                {TAB_GROUPS.map((group) => (
                                    <SidebarGroup key={group.label} className="p-1">
                                        <SidebarGroupLabel className="h-7 px-2 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
                                            {group.label}
                                        </SidebarGroupLabel>
                                        <SidebarGroupContent>
                                            <SidebarMenu className="gap-1">
                                                {group.tabs.map((tab) => (
                                                    <SidebarMenuItem key={tab.id}>
                                                        <button
                                                            onClick={() => handleTabChange(tab.id)}
                                                            className={`inline-flex h-9 w-full items-center gap-2.5 rounded-lg border-0 px-2.5 text-left text-sm transition-colors focus-visible:outline-none ${activeTab === tab.id
                                                                ? 'bg-foreground text-background shadow-sm'
                                                                : 'text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                                                                }`}
                                                        >
                                                            {tab.icon}
                                                            <span className="truncate">{tab.label}</span>
                                                            {tab.id === 'notifications' && unreadCount > 0 && (
                                                                <Badge variant="default" className="ml-auto h-4 min-w-4 px-1.5 py-0 text-[10px] inline-flex items-center justify-center">
                                                                    {unreadCount > 99 ? '99+' : unreadCount}
                                                                </Badge>
                                                            )}
                                                        </button>
                                                    </SidebarMenuItem>
                                                ))}
                                            </SidebarMenu>
                                        </SidebarGroupContent>
                                    </SidebarGroup>
                                ))}
                            </CardContent>
                        </Card>
                    </aside>

                    {/* Tab content */}
                    <div className="space-y-6">
                        {activeTab === 'overview' && (
                            <Card>
                                <CardHeader className='flex items-center justify-between'>
                                    <CardTitle>Application Details</CardTitle>
                                    <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground sm:text-sm">
                                        <span className="font-medium">ID</span>
                                        <code className="rounded-md border border-border/80 bg-background/80 px-2 py-0.5 font-mono text-[11px] font-semibold sm:text-xs break-all">
                                            {app.app_id}
                                        </code>
                                        <Button variant="outline" size="sm" onClick={() => {
                                            navigator.clipboard.writeText(app.app_id);
                                            setCopied(true);
                                            setTimeout(() => setCopied(false), 2000);
                                        }}>
                                            {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                                        </Button>
                                    </div>
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
                                                className="min-h-25"
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

                        {app && (
                            <>
                                <div className={activeTab === 'users' ? 'block' : 'hidden'}>
                                    <AppUsers apiKey={app.api_key} />
                                </div>
                                <div className={activeTab === 'templates' ? 'block' : 'hidden'}>
                                    <AppTemplates appId={app.app_id} apiKey={app.api_key} webhooks={webhooks} />
                                </div>
                                <div className={activeTab === 'notifications' ? 'block' : 'hidden'}>
                                    <AppNotifications apiKey={app.api_key} webhooks={webhooks} onUnreadCount={setUnreadCount} />
                                </div>
                            </>
                        )}

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
                                                // Validate event mapping JSON before save (if Inbound Webhooks section exists)
                                                if (workflows.length > 0 && eventMappingText.trim()) {
                                                    try {
                                                        const parsed = JSON.parse(eventMappingText);
                                                        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
                                                            toast.error('Event mapping must be a JSON object');
                                                            return;
                                                        }
                                                    } catch {
                                                        toast.error('Event mapping must be valid JSON');
                                                        return;
                                                    }
                                                }
                                                await applicationsAPI.updateSettings(id!, settings);
                                                setEventMappingText(JSON.stringify(settings.inbound_webhook_config?.event_mapping || {}, null, 2));
                                                toast.success('Settings saved successfully!');
                                            } catch (err: any) {
                                                toast.error('Error saving settings: ' + (err.response?.data?.message || err.message));
                                            }
                                        }}
                                        className="space-y-4"
                                    >
                                        <Accordion type="multiple" defaultValue={["core-settings", "auth-security", "email-channel", "whatsapp-channel"]} className="w-full space-y-4">
                                            <AccordionItem value="core-settings" className="border bg-card rounded-lg px-4">
                                                <AccordionTrigger className="text-base font-semibold hover:no-underline">Core Settings</AccordionTrigger>
                                                <AccordionContent className="pt-4 pb-4">
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
                                                </AccordionContent>
                                            </AccordionItem>

                                            {workflows.length > 0 && (
                                                <AccordionItem value="workflow-hooks" className="border bg-card rounded-lg px-4">
                                                    <AccordionTrigger className="text-base font-semibold hover:no-underline">Workflow Hooks</AccordionTrigger>
                                                    <AccordionContent className="pt-4 pb-4">
                                                        <div className="space-y-2">
                                                            <Label htmlFor="onUserCreated">On User Created Workflow</Label>
                                                            <Select
                                                                value={settings.on_user_created_trigger_id || 'none'}
                                                                onValueChange={(val) => setSettings({ ...settings, on_user_created_trigger_id: val === 'none' ? '' : val })}
                                                            >
                                                                <SelectTrigger id="onUserCreated">
                                                                    <SelectValue placeholder="None" />
                                                                </SelectTrigger>
                                                                <SelectContent>
                                                                    <SelectItem value="none">None</SelectItem>
                                                                    {workflows.map((w) => (
                                                                        <SelectItem key={w.id} value={w.trigger_id}>{w.name} ({w.trigger_id})</SelectItem>
                                                                    ))}
                                                                </SelectContent>
                                                            </Select>
                                                            <p className="text-xs text-muted-foreground">Trigger this workflow when a new user is created (via API or dashboard)</p>
                                                        </div>
                                                    </AccordionContent>
                                                </AccordionItem>
                                            )}

                                            {workflows.length > 0 && (
                                                <AccordionItem value="inbound-webhooks" className="border bg-card rounded-lg px-4">
                                                    <AccordionTrigger className="text-base font-semibold hover:no-underline">Inbound Webhooks (Phase 7)</AccordionTrigger>
                                                    <AccordionContent className="pt-4 pb-4">
                                                        <p className="text-sm text-muted-foreground mb-4">
                                                            Receive webhooks from external systems (Stripe, CRM, etc.) and trigger workflows. Use X-API-Key to identify the app.
                                                        </p>
                                                        <div className="space-y-4">
                                                            <div className="space-y-2">
                                                                <Label htmlFor="inboundWebhookSecret">Webhook Secret (optional)</Label>
                                                                <Input
                                                                    id="inboundWebhookSecret"
                                                                    type="password"
                                                                    value={settings.inbound_webhook_config?.secret || ''}
                                                                    onChange={(e) => setSettings({
                                                                        ...settings,
                                                                        inbound_webhook_config: {
                                                                            ...settings.inbound_webhook_config,
                                                                            secret: e.target.value
                                                                        }
                                                                    })}
                                                                    placeholder="Leave empty to skip HMAC verification"
                                                                />
                                                                <p className="text-xs text-muted-foreground">If set, send X-Webhook-Signature: sha256=&lt;hex&gt; (HMAC-SHA256 of body)</p>
                                                            </div>
                                                            <div className="space-y-2">
                                                                <Label htmlFor="inboundEventMapping">Event Mapping (JSON)</Label>
                                                                <Textarea
                                                                    id="inboundEventMapping"
                                                                    className="font-mono text-sm"
                                                                    rows={4}
                                                                    value={eventMappingText}
                                                                    onChange={(e) => {
                                                                        const raw = e.target.value;
                                                                        setEventMappingText(raw);
                                                                        try {
                                                                            const parsed = JSON.parse(raw || '{}');
                                                                            if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
                                                                                setSettings({
                                                                                    ...settings,
                                                                                    inbound_webhook_config: {
                                                                                        ...settings.inbound_webhook_config,
                                                                                        event_mapping: parsed
                                                                                    }
                                                                                });
                                                                            }
                                                                        } catch { /* allow typing */ }
                                                                    }}
                                                                    placeholder='{"payment.received": "payment_workflow", "order.shipped": "shipping_workflow"}'
                                                                />
                                                                <p className="text-xs text-muted-foreground">Map event names to workflow trigger_ids. POST body: {"{ \"event\": \"...\", \"user_id\": \"external_id or email\", \"payload\": {...} }"}</p>
                                                            </div>
                                                            <div className="p-3 bg-muted rounded text-xs font-mono">
                                                                POST /v1/webhooks/inbound<br />
                                                                Headers: X-API-Key (required), X-Webhook-Signature (if secret set)
                                                            </div>
                                                        </div>
                                                    </AccordionContent>
                                                </AccordionItem>
                                            )}

                                            <AccordionItem value="auth-security" className="border bg-card rounded-lg px-4">
                                                <AccordionTrigger className="text-base font-semibold hover:no-underline">Authentication & Security</AccordionTrigger>
                                                <AccordionContent className="pt-4 pb-4">
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
                                                </AccordionContent>
                                            </AccordionItem>

                                            <AccordionItem value="email-channel" className="border bg-card rounded-lg px-4">
                                                <AccordionTrigger className="text-base font-semibold hover:no-underline">Email Channel Configuration</AccordionTrigger>
                                                <AccordionContent className="pt-4 pb-4">
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
                                                </AccordionContent>
                                            </AccordionItem>

                                            <AccordionItem value="whatsapp-channel" className="border bg-card rounded-lg px-4">
                                                <AccordionTrigger className="text-base font-semibold hover:no-underline">WhatsApp Channel Configuration</AccordionTrigger>
                                                <AccordionContent className="pt-4 pb-4">
                                                    <div className="p-4 border border-border rounded bg-muted mb-8 space-y-6">
                                                        <p className="text-sm text-muted-foreground">
                                                            Configure per-app Twilio WhatsApp credentials. Leave empty to use system defaults.
                                                        </p>
                                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                            <div className="space-y-2">
                                                                <Label htmlFor="waAccountSid">Twilio Account SID</Label>
                                                                <Input
                                                                    id="waAccountSid"
                                                                    type="password"
                                                                    className="text-sm"
                                                                    value={settings.whatsapp_config?.account_sid || ''}
                                                                    onChange={(e) => setSettings({
                                                                        ...settings,
                                                                        whatsapp_config: {
                                                                            ...settings.whatsapp_config,
                                                                            account_sid: e.target.value
                                                                        } as any
                                                                    })}
                                                                    placeholder="ACxxxxxxxxxxxxx"
                                                                />
                                                            </div>
                                                            <div className="space-y-2">
                                                                <Label htmlFor="waAuthToken">Twilio Auth Token</Label>
                                                                <Input
                                                                    id="waAuthToken"
                                                                    type="password"
                                                                    className="text-sm"
                                                                    value={settings.whatsapp_config?.auth_token || ''}
                                                                    onChange={(e) => setSettings({
                                                                        ...settings,
                                                                        whatsapp_config: {
                                                                            ...settings.whatsapp_config,
                                                                            auth_token: e.target.value
                                                                        } as any
                                                                    })}
                                                                />
                                                            </div>
                                                        </div>
                                                        <div className="space-y-2">
                                                            <Label htmlFor="waFromNumber">WhatsApp From Number</Label>
                                                            <Input
                                                                id="waFromNumber"
                                                                className="text-sm"
                                                                value={settings.whatsapp_config?.from_number || ''}
                                                                onChange={(e) => setSettings({
                                                                    ...settings,
                                                                    whatsapp_config: {
                                                                        ...settings.whatsapp_config,
                                                                        from_number: e.target.value
                                                                    } as any
                                                                })}
                                                                placeholder="+14155238886"
                                                            />
                                                            <p className="text-xs text-muted-foreground">
                                                                Your Twilio WhatsApp-enabled number. The &quot;whatsapp:&quot; prefix is added automatically.
                                                            </p>
                                                        </div>
                                                    </div>
                                                </AccordionContent>
                                            </AccordionItem>

                                            <div className='px-2'>
                                                <div className="text-base font-semibold hover:no-underline">Default Notification Preferences</div>
                                                <div className="pt-4 pb-4">
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

                                                        <div className="flex items-center space-x-3">
                                                            <Checkbox
                                                                id="whatsappEnabled"
                                                                checked={settings.default_preferences?.whatsapp_enabled !== false}
                                                                onCheckedChange={(checked: boolean) => setSettings({
                                                                    ...settings,
                                                                    default_preferences: {
                                                                        ...(settings.default_preferences || {}),
                                                                        whatsapp_enabled: !!checked
                                                                    }
                                                                })}
                                                            />
                                                            <Label htmlFor="whatsappEnabled" className="cursor-pointer">WhatsApp Enabled</Label>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </Accordion>
                                        <div className="flex justify-end pt-4">
                                            <Button type="submit">Save Configuration</Button>
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

                        {/* Schedules Tab */}
                        {activeTab === 'schedules' && app && (
                            <SchedulesList apiKey={app.api_key} embedded />
                        )}

                        {/* Topics Tab */}
                        {activeTab === 'topics' && app && (
                            <TopicsList apiKey={app.api_key} embedded />
                        )}

                        {/* Workflows Tab */}
                        {activeTab === 'workflows' && app && (
                            <WorkflowsList apiKey={app.api_key} embedded />
                        )}

                        {/* Team Tab */}
                        {activeTab === 'team' && app && (
                            <AppTeam appId={app.app_id} />
                        )}

                        {/* Providers Tab */}
                        {activeTab === 'providers' && app && (
                            <AppProviders appId={app.app_id} />
                        )}

                        {/* Import Tab */}
                        {activeTab === 'import' && app && (
                            <AppImport appId={app.app_id} appName={app.app_name} />
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
                                                    onClick={() => setConfirmAction('regenerate-key')}
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
                                        <Button onClick={() => setConfirmAction('delete-app')} variant="destructive">
                                            Delete Application
                                        </Button>
                                    </CardContent>
                                </Card>

                                <ConfirmDeleteDialog
                                    open={confirmAction === 'regenerate-key'}
                                    onOpenChange={(open) => {
                                        if (!open) setConfirmAction(null);
                                    }}
                                    title="Regenerate API Key"
                                    description="Are you sure? The old key will immediately stop working."
                                    confirmLabel="Regenerate"
                                    confirmVariant="default"
                                    loading={confirmLoading}
                                    onConfirm={handleRegenerateKey}
                                />

                                <ConfirmDeleteDialog
                                    open={confirmAction === 'delete-app'}
                                    onOpenChange={(open) => {
                                        if (!open) setConfirmAction(null);
                                    }}
                                    title="Delete Application"
                                    description="Are you sure? This cannot be undone."
                                    confirmLabel="Delete Application"
                                    confirmVariant="destructive"
                                    loading={confirmLoading}
                                    onConfirm={handleDeleteApp}
                                />
                            </div>
                        )}
                    </div>
                </div>
            </>
        </div>
    );
};

export default AppDetail;
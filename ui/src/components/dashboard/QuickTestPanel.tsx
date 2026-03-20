import { useEffect, useState } from 'react';
import { Button } from '../ui/button';
import { Label } from '../ui/label';
import { Textarea } from '../ui/textarea';
import { Badge } from '../ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { toast } from 'sonner';
import { Loader2, ExternalLink } from 'lucide-react';
import { applicationsAPI, usersAPI, templatesAPI, notificationsAPI } from '../../services/api';
import type { Application, User, Template } from '../../types';

export default function QuickTestPanel() {
    const [apps, setApps] = useState<Application[]>([]);
    const [selectedAppId, setSelectedAppId] = useState('');
    const [selectedApiKey, setSelectedApiKey] = useState('');
    const [users, setUsers] = useState<User[]>([]);
    const [selectedUserId, setSelectedUserId] = useState('');
    const [templates, setTemplates] = useState<Template[]>([]);
    const [selectedTemplateId, setSelectedTemplateId] = useState('');
    const [selectedChannel, setSelectedChannel] = useState('');
    const [variablesJson, setVariablesJson] = useState('{}');
    const [sending, setSending] = useState(false);
    const [result, setResult] = useState<{ success: boolean; message: string } | null>(null);
    const [loadingApps, setLoadingApps] = useState(true);
    const [loadingDeps, setLoadingDeps] = useState(false);

    // Fetch apps on mount
    useEffect(() => {
        (async () => {
            try {
                const data = await applicationsAPI.list();
                setApps(Array.isArray(data) ? data : []);
            } catch {
                console.error('Failed to load applications');
            } finally {
                setLoadingApps(false);
            }
        })();
    }, []);

    // On app change → fetch full app detail (list masks API keys) then load users + templates
    useEffect(() => {
        if (!selectedAppId) {
            setUsers([]);
            setTemplates([]);
            setSelectedUserId('');
            setSelectedTemplateId('');
            setSelectedChannel('');
            return;
        }

        setSelectedUserId('');
        setSelectedTemplateId('');
        setSelectedChannel('');
        setResult(null);

        setLoadingDeps(true);

        // The list endpoint masks API keys (***xyz). Fetch full detail to get the real key.
        applicationsAPI.get(selectedAppId)
            .then((fullApp) => {
                const apiKey = fullApp.api_key || '';
                setSelectedApiKey(apiKey);
                if (!apiKey) {
                    setLoadingDeps(false);
                    return;
                }
                return Promise.all([
                    usersAPI.list(apiKey, 1, 50).catch(() => ({ users: [] })),
                    templatesAPI.list(apiKey, 50, 0).catch(() => ({ templates: [] })),
                ]).then(([usersRes, templatesRes]) => {
                    setUsers(usersRes.users || []);
                    setTemplates(templatesRes.templates || []);
                });
            })
            .catch(() => {
                setSelectedApiKey('');
            })
            .finally(() => setLoadingDeps(false));
    }, [selectedAppId]);

    // On template change → pre-populate variables + channel
    useEffect(() => {
        if (!selectedTemplateId) return;
        const tmpl = templates.find(t => t.id === selectedTemplateId);
        if (tmpl) {
            setSelectedChannel(tmpl.channel || '');
            if (tmpl.variables && tmpl.variables.length > 0) {
                const vars: Record<string, string> = {};
                tmpl.variables.forEach(v => { vars[v] = ''; });
                setVariablesJson(JSON.stringify(vars, null, 2));
            } else {
                setVariablesJson('{}');
            }
        }
    }, [selectedTemplateId, templates]);

    const handleSend = async () => {
        if (!selectedApiKey || !selectedUserId || !selectedTemplateId || !selectedChannel) return;

        setSending(true);
        setResult(null);

        let parsedVars: Record<string, unknown> = {};
        try {
            parsedVars = JSON.parse(variablesJson);
        } catch {
            setResult({ success: false, message: 'Invalid JSON in variables field.' });
            setSending(false);
            return;
        }

        const tmpl = templates.find(t => t.id === selectedTemplateId);

        try {
            await notificationsAPI.send(selectedApiKey, {
                user_id: selectedUserId,
                channel: selectedChannel as 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse',
                priority: 'normal',
                title: tmpl?.name || 'Quick Test',
                body: tmpl?.body || 'Test notification',
                template_id: selectedTemplateId,
                data: parsedVars,
            });
            setResult({ success: true, message: 'Notification sent successfully!' });
            toast.success('Test notification sent');
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : 'Failed to send notification';
            setResult({ success: false, message: msg });
            toast.error('Failed to send test notification');
        } finally {
            setSending(false);
        }
    };

    return (
        <section className="space-y-4">
            <div>
                <h3 className="text-lg font-semibold">Quick Test</h3>
                <p className="mt-1 text-xs text-muted-foreground">Send a test notification end-to-end</p>
            </div>

            <div className="space-y-4 rounded-lg border border-border/70 bg-white/70 p-4 dark:bg-zinc-900/40">
                {/* Step 1: App */}
                <div className="space-y-1.5">
                    <Label className="text-xs font-medium">Application</Label>
                    <Select value={selectedAppId} onValueChange={setSelectedAppId} disabled={loadingApps}>
                        <SelectTrigger className="h-9">
                            <SelectValue placeholder={loadingApps ? 'Loading...' : 'Select an app'} />
                        </SelectTrigger>
                        <SelectContent>
                            {apps.map(app => (
                                <SelectItem key={app.app_id} value={app.app_id}>{app.app_name}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

                {/* Step 2: User */}
                <div className="space-y-1.5">
                    <Label className="text-xs font-medium">User</Label>
                    <Select
                        value={selectedUserId}
                        onValueChange={setSelectedUserId}
                        disabled={!selectedAppId || loadingDeps}
                    >
                        <SelectTrigger className="h-9">
                            <SelectValue placeholder={loadingDeps ? 'Loading...' : 'Select a user'} />
                        </SelectTrigger>
                        <SelectContent>
                            {users.map(u => (
                                <SelectItem key={u.user_id} value={u.user_id}>
                                    {u.email || u.user_id}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

                {/* Step 3: Template */}
                <div className="space-y-1.5">
                    <Label className="text-xs font-medium">Template</Label>
                    <Select
                        value={selectedTemplateId}
                        onValueChange={setSelectedTemplateId}
                        disabled={!selectedAppId || loadingDeps}
                    >
                        <SelectTrigger className="h-9">
                            <SelectValue placeholder={loadingDeps ? 'Loading...' : 'Select a template'} />
                        </SelectTrigger>
                        <SelectContent>
                            {templates.map(t => (
                                <SelectItem key={t.id} value={t.id}>
                                    <span className="flex items-center gap-2">
                                        {t.name}
                                        <Badge variant="outline" className="text-[10px] uppercase">{t.channel}</Badge>
                                    </span>
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

                {/* Step 4: Variables */}
                <div className="space-y-1.5">
                    <Label className="text-xs font-medium">Variables (JSON)</Label>
                    <Textarea
                        value={variablesJson}
                        onChange={e => setVariablesJson(e.target.value)}
                        className="font-mono text-xs h-24 resize-none"
                        disabled={!selectedTemplateId}
                        placeholder="{}"
                    />
                </div>

                {/* Send */}
                <Button
                    className="w-full"
                    disabled={!selectedUserId || !selectedTemplateId || !selectedChannel || sending}
                    onClick={handleSend}
                >
                    {sending ? (
                        <>
                            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                            Sending...
                        </>
                    ) : (
                        'Send Test Notification'
                    )}
                </Button>

                {/* Result */}
                {result && (
                    <div
                        className={`p-3 rounded-md border text-sm ${result.success
                            ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-950/40 dark:text-green-300'
                            : 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300'
                            }`}
                    >
                        {result.message}
                    </div>
                )}

                <a
                    href="/docs"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                    View API Documentation
                    <ExternalLink className="h-3 w-3" />
                </a>
            </div>
        </section>
    );
}

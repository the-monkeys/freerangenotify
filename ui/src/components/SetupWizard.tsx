import React, { useState, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { templatesAPI, usersAPI, notificationsAPI, environmentsAPI } from '../services/api';
import type { CreateTemplateRequest, CreateEnvironmentRequest } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Textarea } from './ui/textarea';
import { Badge } from './ui/badge';
import { CheckCircle2, Copy, ArrowRight, BookOpen, FileText, Workflow, LayoutGrid } from 'lucide-react';
import { toast } from 'sonner';

interface SetupWizardProps {
    appId: string;
    apiKey: string;
    onComplete: () => void;
}

type WizardStep = 'channel' | 'environment' | 'template' | 'user' | 'send' | 'done';

const STEPS: { key: WizardStep; label: string; num: number }[] = [
    { key: 'channel', label: 'Channel', num: 1 },
    { key: 'environment', label: 'Environment', num: 2 },
    { key: 'template', label: 'Template', num: 3 },
    { key: 'user', label: 'Recipient', num: 4 },
    { key: 'send', label: 'Send', num: 5 },
    { key: 'done', label: 'Done', num: 6 },
];

const CHANNELS = [
    { value: 'email' as const, label: 'Email', desc: 'Send via SMTP or SendGrid' },
    { value: 'webhook' as const, label: 'Webhook', desc: 'HTTP callback delivery' },
    { value: 'push' as const, label: 'Push', desc: 'iOS (APNS) / Android (FCM)' },
    { value: 'sms' as const, label: 'SMS', desc: 'Via Twilio' },
    { value: 'sse' as const, label: 'SSE', desc: 'Real-time browser notifications' },
];

export function SetupWizard({ appId, apiKey, onComplete }: SetupWizardProps) {
    const [step, setStep] = useState<WizardStep>('channel');
    const [channel, setChannel] = useState<CreateTemplateRequest['channel']>('email');
    const [envName, setEnvName] = useState('');
    const [templateName, setTemplateName] = useState('');
    const [subject, setSubject] = useState('');
    const [body, setBody] = useState('');
    const [userEmail, setUserEmail] = useState('');
    const [externalId, setExternalId] = useState('');
    const [sendData, setSendData] = useState<Record<string, string>>({});
    const [createdTemplateId, setCreatedTemplateId] = useState('');
    const [createdUserId, setCreatedUserId] = useState('');
    const [sentNotificationId, setSentNotificationId] = useState('');
    const [sending, setSending] = useState(false);
    const [creating, setCreating] = useState(false);
    const [creatingUser, setCreatingUser] = useState(false);
    const [creatingEnv, setCreatingEnv] = useState(false);

    // Auto-detect variables from body
    const variables = useMemo(() => {
        const regex = /{{\s*\.?(\w+)\s*}}/g;
        const vars = new Set<string>();
        let match;
        while ((match = regex.exec(body)) !== null) {
            if (match[1]) vars.add(match[1]);
        }
        return Array.from(vars);
    }, [body]);

    const stepIndex = STEPS.findIndex(s => s.key === step);
    const progress = ((stepIndex + 1) / STEPS.length) * 100;

    const handleCreateTemplate = async () => {
        if (!templateName || !body) {
            toast.error('Template name and body are required');
            return;
        }
        setCreating(true);
        try {
            const payload: CreateTemplateRequest = {
                app_id: appId,
                name: templateName,
                channel: channel as any,
                subject: channel === 'email' ? subject : undefined,
                body,
                variables,
                locale: 'en',
            };
            const tmpl = await templatesAPI.create(apiKey, payload);
            setCreatedTemplateId(tmpl.id || tmpl.name);
            toast.success('Template created!');
            setStep('user');
        } catch (error: any) {
            toast.error(error?.response?.data?.message || 'Failed to create template');
        } finally {
            setCreating(false);
        }
    };

    const handleCreateEnvironment = async () => {
        if (!envName) {
            toast.error('Environment name is required');
            return;
        }
        setCreatingEnv(true);
        try {
            const payload: CreateEnvironmentRequest = { name: envName as any };
            await environmentsAPI.create(appId, payload);
            toast.success(`Environment "${envName}" created!`);
            setStep('template');
        } catch (error: any) {
            toast.error(error?.response?.data?.message || 'Failed to create environment');
        } finally {
            setCreatingEnv(false);
        }
    };

    const handleCreateUser = async () => {
        if (!userEmail) {
            toast.error('Email is required');
            return;
        }
        setCreatingUser(true);
        try {
            const user = await usersAPI.create(apiKey, {
                email: userEmail,
                user_id: externalId || undefined,
            });
            const userId = (user as any).user_id || (user as any).id || '';
            setCreatedUserId(userId);
            toast.success('User created!');
            setStep('send');
        } catch (error: any) {
            toast.error(error?.response?.data?.message || 'Failed to create user');
        } finally {
            setCreatingUser(false);
        }
    };

    const handleSend = async () => {
        if (!createdUserId) {
            toast.error('User ID is missing — go back and create a user');
            return;
        }
        setSending(true);
        try {
            const result = await notificationsAPI.send(apiKey, {
                user_id: createdUserId,
                channel,
                priority: 'normal',
                title: subject || templateName,
                body,
                template_id: createdTemplateId || undefined,
                data: Object.keys(sendData).length > 0 ? sendData : undefined,
            });
            const nid = (result as any).notification_id || (result as any).id || '';
            setSentNotificationId(nid);
            toast.success('Notification sent! 🎉');
            setStep('done');
        } catch (error: any) {
            toast.error(error?.response?.data?.message || 'Failed to send');
        } finally {
            setSending(false);
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        toast.success('Copied to clipboard');
    };

    return (
        <Card className="max-w-2xl mx-auto">
            <CardHeader>
                <CardTitle className="text-xl">Get Started — Send Your First Notification</CardTitle>
                <div className="flex items-center gap-2 mt-3">
                    {STEPS.map((s, i) => (
                        <React.Fragment key={s.key}>
                            <div className={`flex items-center gap-1 text-xs font-medium ${i <= stepIndex ? 'text-foreground' : 'text-muted-foreground'
                                }`}>
                                <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs ${i < stepIndex ? 'bg-foreground text-background' :
                                    i === stepIndex ? 'bg-muted text-foreground ring-2 ring-foreground' :
                                        'bg-muted text-muted-foreground'
                                    }`}>
                                    {i < stepIndex ? '✓' : s.num}
                                </div>
                                <span className="hidden sm:inline">{s.label}</span>
                            </div>
                            {i < STEPS.length - 1 && (
                                <div className={`flex-1 h-0.5 ${i < stepIndex ? 'bg-foreground' : 'bg-muted'}`} />
                            )}
                        </React.Fragment>
                    ))}
                </div>
                <div className="w-full bg-muted rounded-full h-1.5 mt-2">
                    <div
                        className="bg-foreground h-1.5 rounded-full transition-all duration-300"
                        style={{ width: `${progress}%` }}
                    />
                </div>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Step 1: Choose Channel */}
                {step === 'channel' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            How do you want to deliver notifications?
                        </p>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                            {CHANNELS.map(ch => (
                                <button
                                    key={ch.value}
                                    type="button"
                                    onClick={() => setChannel(ch.value)}
                                    className={`p-4 rounded border-2 text-left transition-colors ${channel === ch.value
                                        ? 'border-foreground bg-muted'
                                        : 'border-border hover:border-border'
                                        }`}
                                >
                                    <p className="font-medium text-sm">{ch.label}</p>
                                    <p className="text-xs text-muted-foreground mt-1">{ch.desc}</p>
                                </button>
                            ))}
                        </div>
                        <div className="flex justify-end">
                            <Button onClick={() => setStep('environment')}>Next →</Button>
                        </div>
                    </div>
                )}

                {/* Step 2: Create Environment (Optional) */}
                {step === 'environment' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Environments let you separate dev/staging/prod. Each gets its own API key. This step is optional.
                        </p>
                        <div className="space-y-2">
                            <Label>Environment Name</Label>
                            <Input
                                value={envName}
                                onChange={e => setEnvName(e.target.value)}
                                placeholder="e.g. staging"
                            />
                            <p className="text-xs text-muted-foreground">
                                You can skip this and use the default environment.
                            </p>
                        </div>
                        <div className="flex justify-between">
                            <Button variant="outline" onClick={() => setStep('channel')}>← Back</Button>
                            <div className="flex gap-2">
                                <Button variant="outline" onClick={() => setStep('template')}>
                                    Skip
                                </Button>
                                <Button onClick={handleCreateEnvironment} disabled={creatingEnv || !envName}>
                                    {creatingEnv ? 'Creating...' : 'Create & Continue →'}
                                </Button>
                            </div>
                        </div>
                    </div>
                )}

                {/* Step 3: Create Template */}
                {step === 'template' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Create a simple template. Use <code className="text-xs bg-muted px-1 rounded">{'{{.variable}}'}</code> for dynamic content.
                        </p>
                        <div className="space-y-3">
                            <div className="space-y-2">
                                <Label>Template Name</Label>
                                <Input
                                    value={templateName}
                                    onChange={e => setTemplateName(e.target.value)}
                                    placeholder="e.g. welcome_email"
                                />
                            </div>
                            {channel === 'email' && (
                                <div className="space-y-2">
                                    <Label>Subject</Label>
                                    <Input
                                        value={subject}
                                        onChange={e => setSubject(e.target.value)}
                                        placeholder="Welcome to {{.product}}!"
                                    />
                                </div>
                            )}
                            <div className="space-y-2">
                                <Label>Body</Label>
                                <Textarea
                                    className="min-h-[120px] font-mono text-sm"
                                    value={body}
                                    onChange={e => setBody(e.target.value)}
                                    placeholder={channel === 'email'
                                        ? '<h1>Hello {{.name}}!</h1><p>Welcome to {{.product}}.</p>'
                                        : 'Hello {{.name}}, welcome to {{.product}}!'}
                                />
                            </div>
                            {variables.length > 0 && (
                                <div className="flex gap-1 flex-wrap">
                                    <span className="text-xs text-muted-foreground">Detected variables:</span>
                                    {variables.map(v => (
                                        <Badge key={v} variant="outline" className="text-xs">{v}</Badge>
                                    ))}
                                </div>
                            )}
                        </div>
                        <div className="flex justify-between">
                            <Button variant="outline" onClick={() => setStep('environment')}>← Back</Button>
                            <Button onClick={handleCreateTemplate} disabled={creating}>
                                {creating ? 'Creating...' : 'Create Template & Continue →'}
                            </Button>
                        </div>
                    </div>
                )}

                {/* Step 4: Create User */}
                {step === 'user' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Every notification needs a recipient. This user represents a person in your system.
                        </p>
                        <div className="space-y-3">
                            <div className="space-y-2">
                                <Label>Email</Label>
                                <Input
                                    type="email"
                                    value={userEmail}
                                    onChange={e => setUserEmail(e.target.value)}
                                    placeholder="user@example.com"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label>External ID <span className="text-muted-foreground text-xs">(optional)</span></Label>
                                <Input
                                    value={externalId}
                                    onChange={e => setExternalId(e.target.value)}
                                    placeholder="Your system's user ID"
                                />
                                <p className="text-xs text-muted-foreground">
                                    Map this user to an ID in your own system for easy lookup.
                                </p>
                            </div>
                            {createdUserId && (
                                <div className="bg-muted rounded-lg p-3 flex items-center justify-between">
                                    <div className="text-xs">
                                        <span className="text-muted-foreground">User ID: </span>
                                        <code className="font-mono">{createdUserId}</code>
                                    </div>
                                    <Button variant="ghost" size="sm" onClick={() => copyToClipboard(createdUserId)}>
                                        <Copy className="h-3 w-3" />
                                    </Button>
                                </div>
                            )}
                        </div>
                        <div className="flex justify-between">
                            <Button variant="outline" onClick={() => setStep('template')}>← Back</Button>
                            {createdUserId ? (
                                <Button onClick={() => setStep('send')}>Next →</Button>
                            ) : (
                                <Button onClick={handleCreateUser} disabled={creatingUser || !userEmail}>
                                    {creatingUser ? 'Creating...' : 'Create User & Continue →'}
                                </Button>
                            )}
                        </div>
                    </div>
                )}

                {/* Step 5: Preview & Send */}
                {step === 'send' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Review the details and send your first notification.
                        </p>
                        <div className="bg-muted p-4 rounded border">
                            <div className="grid grid-cols-2 gap-2 text-sm mb-3">
                                <div><span className="text-muted-foreground">Channel:</span> <Badge variant="outline">{channel}</Badge></div>
                                <div><span className="text-muted-foreground">Template:</span> <span className="font-medium">{templateName}</span></div>
                                <div><span className="text-muted-foreground">To:</span> <span className="font-medium">{userEmail}</span></div>
                                <div><span className="text-muted-foreground">User ID:</span> <code className="text-xs font-mono">{createdUserId.substring(0, 12)}…</code></div>
                            </div>
                        </div>
                        {variables.length > 0 && (
                            <div className="space-y-3">
                                <Label>Template Variables</Label>
                                {variables.map(v => (
                                    <div key={v} className="flex items-center gap-3">
                                        <Label className="w-24 text-right text-xs text-muted-foreground">{v}</Label>
                                        <Input
                                            className="flex-1"
                                            value={sendData[v] || ''}
                                            onChange={e => setSendData({ ...sendData, [v]: e.target.value })}
                                            placeholder={`Enter ${v}...`}
                                        />
                                    </div>
                                ))}
                            </div>
                        )}
                        <div className="flex justify-between">
                            <Button variant="outline" onClick={() => setStep('user')}>← Back</Button>
                            <Button
                                className="bg-green-600 hover:bg-green-700"
                                onClick={handleSend}
                                disabled={sending}
                            >
                                {sending ? 'Sending...' : 'Send Notification 🚀'}
                            </Button>
                        </div>
                    </div>
                )}

                {/* Step 6: Done */}
                {step === 'done' && (
                    <div className="space-y-6 text-center py-4">
                        <CheckCircle2 className="h-16 w-16 text-green-500 mx-auto" />
                        <div className="space-y-2">
                            <h2 className="text-lg font-semibold">Your first notification was sent!</h2>
                            <p className="text-sm text-muted-foreground">
                                Everything is set up and working. Here's what you can do next.
                            </p>
                        </div>
                        {sentNotificationId && (
                            <div className="bg-muted rounded-lg p-3 inline-flex items-center gap-2 text-xs">
                                <span className="text-muted-foreground">Notification ID:</span>
                                <code className="font-mono">{sentNotificationId}</code>
                                <Button variant="ghost" size="sm" className="h-6 px-1" onClick={() => copyToClipboard(sentNotificationId)}>
                                    <Copy className="h-3 w-3" />
                                </Button>
                            </div>
                        )}
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-left">
                            <Link
                                to="/dashboard"
                                className="flex items-center gap-3 p-3 rounded-lg border hover:bg-muted transition-colors"
                            >
                                <LayoutGrid className="h-5 w-5 text-muted-foreground" />
                                <div>
                                    <p className="text-sm font-medium">View Dashboard</p>
                                    <p className="text-xs text-muted-foreground">Monitor delivery status</p>
                                </div>
                            </Link>
                            <Link
                                to={`/apps/${appId}`}
                                onClick={onComplete}
                                className="flex items-center gap-3 p-3 rounded-lg border hover:bg-muted transition-colors"
                            >
                                <FileText className="h-5 w-5 text-muted-foreground" />
                                <div>
                                    <p className="text-sm font-medium">Create Another Template</p>
                                    <p className="text-xs text-muted-foreground">Build more notification types</p>
                                </div>
                            </Link>
                            <Link
                                to="/workflows"
                                className="flex items-center gap-3 p-3 rounded-lg border hover:bg-muted transition-colors"
                            >
                                <Workflow className="h-5 w-5 text-muted-foreground" />
                                <div>
                                    <p className="text-sm font-medium">Explore Workflows</p>
                                    <p className="text-xs text-muted-foreground">Automate multi-step notifications</p>
                                </div>
                            </Link>
                            <Link
                                to="/docs"
                                className="flex items-center gap-3 p-3 rounded-lg border hover:bg-muted transition-colors"
                            >
                                <BookOpen className="h-5 w-5 text-muted-foreground" />
                                <div>
                                    <p className="text-sm font-medium">Read the Docs</p>
                                    <p className="text-xs text-muted-foreground">Full guides and API reference</p>
                                </div>
                            </Link>
                        </div>
                        <Button onClick={onComplete} className="mt-2">
                            Close Wizard <ArrowRight className="h-4 w-4 ml-1" />
                        </Button>
                    </div>
                )}

                {step !== 'done' && (
                    <div className="pt-4 border-t flex justify-end">
                        <Button variant="ghost" size="sm" onClick={onComplete}>
                            Skip Wizard
                        </Button>
                    </div>
                )}
            </CardContent>
        </Card>
    );
}

export default SetupWizard;

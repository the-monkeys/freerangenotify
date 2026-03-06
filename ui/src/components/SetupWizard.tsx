import React, { useState, useMemo } from 'react';
import { templatesAPI, quickSendAPI } from '../services/api';
import type { CreateTemplateRequest } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Textarea } from './ui/textarea';
import { Badge } from './ui/badge';
import { toast } from 'sonner';

interface SetupWizardProps {
    appId: string;
    apiKey: string;
    onComplete: () => void;
}

type WizardStep = 'channel' | 'template' | 'recipient' | 'send';

const STEPS: { key: WizardStep; label: string; num: number }[] = [
    { key: 'channel', label: 'Choose Channel', num: 1 },
    { key: 'template', label: 'Create Template', num: 2 },
    { key: 'recipient', label: 'Add Recipient', num: 3 },
    { key: 'send', label: 'Send First Notification', num: 4 },
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
    const [templateName, setTemplateName] = useState('');
    const [subject, setSubject] = useState('');
    const [body, setBody] = useState('');
    const [recipient, setRecipient] = useState('');
    const [sendData, setSendData] = useState<Record<string, string>>({});
    const [createdTemplateId, setCreatedTemplateId] = useState('');
    const [sending, setSending] = useState(false);
    const [creating, setCreating] = useState(false);

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
            setStep('recipient');
        } catch (error: any) {
            toast.error(error?.response?.data?.message || 'Failed to create template');
        } finally {
            setCreating(false);
        }
    };

    const handleSend = async () => {
        if (!recipient) {
            toast.error('Recipient email is required');
            return;
        }
        setSending(true);
        try {
            await quickSendAPI.send(apiKey, {
                to: recipient,
                template: createdTemplateId || templateName,
                channel,
                data: sendData,
            });
            toast.success('First notification sent! 🎉');
            onComplete();
        } catch (error: any) {
            toast.error(error?.response?.data?.message || 'Failed to send');
        } finally {
            setSending(false);
        }
    };

    return (
        <Card className="max-w-2xl mx-auto">
            <CardHeader>
                <CardTitle className="text-xl">Get Started — Send Your First Notification</CardTitle>
                <div className="flex items-center gap-2 mt-3">
                    {STEPS.map((s, i) => (
                        <React.Fragment key={s.key}>
                            <div className={`flex items-center gap-1 text-xs font-medium ${
                                i <= stepIndex ? 'text-blue-600' : 'text-gray-400'
                            }`}>
                                <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs ${
                                    i < stepIndex ? 'bg-blue-600 text-white' :
                                    i === stepIndex ? 'bg-blue-100 text-blue-700 ring-2 ring-blue-600' :
                                    'bg-gray-100 text-gray-400'
                                }`}>
                                    {i < stepIndex ? '✓' : s.num}
                                </div>
                                <span className="hidden sm:inline">{s.label}</span>
                            </div>
                            {i < STEPS.length - 1 && (
                                <div className={`flex-1 h-0.5 ${i < stepIndex ? 'bg-blue-600' : 'bg-gray-200'}`} />
                            )}
                        </React.Fragment>
                    ))}
                </div>
                <div className="w-full bg-gray-200 rounded-full h-1.5 mt-2">
                    <div
                        className="bg-blue-600 h-1.5 rounded-full transition-all duration-300"
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
                                    className={`p-4 rounded border-2 text-left transition-colors ${
                                        channel === ch.value
                                            ? 'border-blue-600 bg-blue-50'
                                            : 'border-gray-200 hover:border-gray-300'
                                    }`}
                                >
                                    <p className="font-medium text-sm">{ch.label}</p>
                                    <p className="text-xs text-muted-foreground mt-1">{ch.desc}</p>
                                </button>
                            ))}
                        </div>
                        <div className="flex justify-end">
                            <Button onClick={() => setStep('template')}>Next →</Button>
                        </div>
                    </div>
                )}

                {/* Step 2: Create Template */}
                {step === 'template' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Create a simple template. Use <code className="text-xs bg-gray-100 px-1 rounded">{'{{.variable}}'}</code> for dynamic content.
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
                                    <span className="text-xs text-gray-500">Detected variables:</span>
                                    {variables.map(v => (
                                        <Badge key={v} variant="outline" className="text-xs">{v}</Badge>
                                    ))}
                                </div>
                            )}
                        </div>
                        <div className="flex justify-between">
                            <Button variant="outline" onClick={() => setStep('channel')}>← Back</Button>
                            <Button onClick={handleCreateTemplate} disabled={creating}>
                                {creating ? 'Creating...' : 'Create Template & Continue →'}
                            </Button>
                        </div>
                    </div>
                )}

                {/* Step 3: Add Recipient */}
                {step === 'recipient' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Enter the recipient's email address. A user will be auto-created if they don't exist.
                        </p>
                        <div className="space-y-2">
                            <Label>Recipient Email</Label>
                            <Input
                                type="email"
                                value={recipient}
                                onChange={e => setRecipient(e.target.value)}
                                placeholder="user@example.com"
                            />
                        </div>
                        <div className="flex justify-between">
                            <Button variant="outline" onClick={() => setStep('template')}>← Back</Button>
                            <Button onClick={() => recipient ? setStep('send') : toast.error('Enter a recipient email')}>
                                Next →
                            </Button>
                        </div>
                    </div>
                )}

                {/* Step 4: Preview & Send */}
                {step === 'send' && (
                    <div className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Fill in template variables and send your first notification!
                        </p>
                        <div className="bg-gray-50 p-4 rounded border">
                            <div className="grid grid-cols-2 gap-2 text-sm mb-3">
                                <div><span className="text-gray-500">Channel:</span> <Badge variant="outline">{channel}</Badge></div>
                                <div><span className="text-gray-500">Template:</span> <span className="font-medium">{templateName}</span></div>
                                <div><span className="text-gray-500">To:</span> <span className="font-medium">{recipient}</span></div>
                            </div>
                        </div>
                        {variables.length > 0 && (
                            <div className="space-y-3">
                                <Label>Template Variables</Label>
                                {variables.map(v => (
                                    <div key={v} className="flex items-center gap-3">
                                        <Label className="w-24 text-right text-xs text-gray-500">{v}</Label>
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
                            <Button variant="outline" onClick={() => setStep('recipient')}>← Back</Button>
                            <Button
                                className="bg-green-600 hover:bg-green-700"
                                onClick={handleSend}
                                disabled={sending}
                            >
                                {sending ? 'Sending...' : 'Send First Notification! 🚀'}
                            </Button>
                        </div>
                    </div>
                )}

                <div className="pt-4 border-t flex justify-end">
                    <Button variant="ghost" size="sm" onClick={onComplete}>
                        Skip Wizard
                    </Button>
                </div>
            </CardContent>
        </Card>
    );
}

export default SetupWizard;

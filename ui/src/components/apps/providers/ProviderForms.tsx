import React, { useState, useMemo } from 'react';
import type { ProviderKind, ProviderSignatureVersion } from '../../../types';
import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import { Label } from '../../ui/label';
import { Textarea } from '../../ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../ui/select';
import { Badge } from '../../ui/badge';
import { Loader2 } from 'lucide-react';

export interface ProviderFormData {
    name: string;
    channel: string;
    kind: ProviderKind;
    webhook_url: string;
    headers?: Record<string, string>;
    signature_version: ProviderSignatureVersion;
}

interface ProviderFormProps {
    kind: ProviderKind;
    onSubmit: (data: ProviderFormData) => Promise<void>;
    loading: boolean;
}

// ── URL validation per kind ──

const URL_PATTERNS: Record<string, { regex: RegExp; hint: string }> = {
    discord: {
        regex: /^https:\/\/(canary\.|ptb\.)?discord(app)?\.com\/api\/webhooks\//i,
        hint: 'Must be a Discord webhook URL: https://discord.com/api/webhooks/...',
    },
    slack: {
        regex: /^https:\/\/hooks\.slack\.com\/services\//i,
        hint: 'Must be a Slack webhook URL: https://hooks.slack.com/services/...',
    },
    teams: {
        regex: /^https:\/\/.*(webhook\.office\.com|logic\.azure\.com\/workflows)\//i,
        hint: 'Must be a Teams connector or Workflow URL',
    },
};

// Detect Teams flavor from URL
function detectTeamsFlavor(url: string): 'legacy' | 'workflow' | null {
    try {
        const u = new URL(url);
        if (u.host.endsWith('webhook.office.com')) return 'legacy';
        if (u.host.includes('logic.azure.com') && u.pathname.includes('/workflows/')) return 'workflow';
    } catch { /* ignore */ }
    return null;
}

// ── Discord Form ──

export const ProviderFormDiscord: React.FC<ProviderFormProps> = ({ onSubmit, loading }) => {
    const [name, setName] = useState('');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [sigVer, setSigVer] = useState<ProviderSignatureVersion>('v1');

    const urlValid = useMemo(() => !webhookUrl.trim() || URL_PATTERNS.discord.regex.test(webhookUrl), [webhookUrl]);

    return (
        <form onSubmit={async (e) => {
            e.preventDefault();
            await onSubmit({ name: name.trim(), channel: 'webhook', kind: 'discord', webhook_url: webhookUrl.trim(), signature_version: sigVer });
        }} className="space-y-4">
            <div className="space-y-1.5">
                <Label className="text-xs">Provider Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="my-discord-webhook" required />
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Discord Webhook URL</Label>
                <Input
                    type="url"
                    value={webhookUrl}
                    onChange={e => setWebhookUrl(e.target.value)}
                    placeholder="https://discord.com/api/webhooks/..."
                    required
                    className={webhookUrl.trim() && !urlValid ? 'border-destructive' : ''}
                />
                {webhookUrl.trim() && !urlValid && (
                    <p className="text-xs text-destructive">{URL_PATTERNS.discord.hint}</p>
                )}
            </div>
            <SigVersionSelect value={sigVer} onChange={setSigVer} />
            <SubmitButton loading={loading} disabled={!urlValid} />
        </form>
    );
};

// ── Slack Form ──

export const ProviderFormSlack: React.FC<ProviderFormProps> = ({ onSubmit, loading }) => {
    const [name, setName] = useState('');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [sigVer, setSigVer] = useState<ProviderSignatureVersion>('v1');

    const urlValid = useMemo(() => !webhookUrl.trim() || URL_PATTERNS.slack.regex.test(webhookUrl), [webhookUrl]);

    return (
        <form onSubmit={async (e) => {
            e.preventDefault();
            await onSubmit({ name: name.trim(), channel: 'webhook', kind: 'slack', webhook_url: webhookUrl.trim(), signature_version: sigVer });
        }} className="space-y-4">
            <div className="space-y-1.5">
                <Label className="text-xs">Provider Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="my-slack-webhook" required />
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Slack Incoming Webhook URL</Label>
                <Input
                    type="url"
                    value={webhookUrl}
                    onChange={e => setWebhookUrl(e.target.value)}
                    placeholder="https://hooks.slack.com/services/T.../B.../..."
                    required
                    className={webhookUrl.trim() && !urlValid ? 'border-destructive' : ''}
                />
                {webhookUrl.trim() && !urlValid && (
                    <p className="text-xs text-destructive">{URL_PATTERNS.slack.hint}</p>
                )}
            </div>
            <SigVersionSelect value={sigVer} onChange={setSigVer} />
            <SubmitButton loading={loading} disabled={!urlValid} />
        </form>
    );
};

// ── Teams Form ──

export const ProviderFormTeams: React.FC<ProviderFormProps> = ({ onSubmit, loading }) => {
    const [name, setName] = useState('');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [sigVer, setSigVer] = useState<ProviderSignatureVersion>('v1');

    const urlValid = useMemo(() => !webhookUrl.trim() || URL_PATTERNS.teams.regex.test(webhookUrl), [webhookUrl]);
    const flavor = useMemo(() => detectTeamsFlavor(webhookUrl), [webhookUrl]);

    return (
        <form onSubmit={async (e) => {
            e.preventDefault();
            await onSubmit({ name: name.trim(), channel: 'webhook', kind: 'teams', webhook_url: webhookUrl.trim(), signature_version: sigVer });
        }} className="space-y-4">
            <div className="space-y-1.5">
                <Label className="text-xs">Provider Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="my-teams-webhook" required />
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Microsoft Teams Webhook URL</Label>
                <Input
                    type="url"
                    value={webhookUrl}
                    onChange={e => setWebhookUrl(e.target.value)}
                    placeholder="https://...webhook.office.com/... or https://...logic.azure.com/workflows/..."
                    required
                    className={webhookUrl.trim() && !urlValid ? 'border-destructive' : ''}
                />
                {webhookUrl.trim() && !urlValid && (
                    <p className="text-xs text-destructive">{URL_PATTERNS.teams.hint}</p>
                )}
                {flavor && (
                    <div className="flex items-center gap-1.5 text-xs">
                        <Badge variant={flavor === 'workflow' ? 'default' : 'secondary'} className="text-[10px]">
                            {flavor === 'workflow' ? 'Workflow URL' : 'Legacy Connector'}
                        </Badge>
                        {flavor === 'legacy' && (
                            <span className="text-amber-600 dark:text-amber-400">⚠ Legacy connectors are being deprecated by Microsoft</span>
                        )}
                    </div>
                )}
            </div>
            <SigVersionSelect value={sigVer} onChange={setSigVer} />
            <SubmitButton loading={loading} disabled={!urlValid} />
        </form>
    );
};

// ── Generic Form ──

export const ProviderFormGeneric: React.FC<ProviderFormProps> = ({ onSubmit, loading }) => {
    const [name, setName] = useState('');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [headersJson, setHeadersJson] = useState('');
    const [sigVer, setSigVer] = useState<ProviderSignatureVersion>('v1');

    return (
        <form onSubmit={async (e) => {
            e.preventDefault();
            let headers: Record<string, string> | undefined;
            if (headersJson.trim()) {
                try { headers = JSON.parse(headersJson); } catch {
                    return; // JSON parse error handled inline
                }
            }
            await onSubmit({ name: name.trim(), channel: 'webhook', kind: 'generic', webhook_url: webhookUrl.trim(), headers, signature_version: sigVer });
        }} className="space-y-4">
            <div className="space-y-1.5">
                <Label className="text-xs">Provider Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="my-webhook" required />
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Webhook URL</Label>
                <Input type="url" value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} placeholder="https://example.com/webhook" required />
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Custom Headers (JSON, optional)</Label>
                <Textarea
                    className="h-[80px] font-mono text-xs"
                    value={headersJson}
                    onChange={e => setHeadersJson(e.target.value)}
                    placeholder='{"Authorization": "Bearer ..."}'
                />
            </div>
            <SigVersionSelect value={sigVer} onChange={setSigVer} />
            <SubmitButton loading={loading} />
        </form>
    );
};

// ── Custom (power-user) Form ──

export const ProviderFormCustom: React.FC<ProviderFormProps> = ({ onSubmit, loading }) => {
    const [name, setName] = useState('');
    const [channel, setChannel] = useState('webhook');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [headersJson, setHeadersJson] = useState('');
    const [sigVer, setSigVer] = useState<ProviderSignatureVersion>('v1');

    const CHANNELS = ['webhook', 'discord', 'slack', 'teams', 'email', 'push', 'sms', 'whatsapp', 'sse'];

    // Infer kind from URL
    const kind: ProviderKind = useMemo(() => {
        try {
            const u = new URL(webhookUrl);
            const h = u.host.toLowerCase();
            const p = u.pathname.toLowerCase();
            if ((h.includes('discord.com') || h.includes('discordapp.com')) && p.startsWith('/api/webhooks')) return 'discord';
            if (h.includes('hooks.slack.com')) return 'slack';
            if (h.endsWith('webhook.office.com') || (h.includes('logic.azure.com') && p.includes('/workflows/'))) return 'teams';
        } catch { /* ignore */ }
        return 'generic';
    }, [webhookUrl]);

    return (
        <form onSubmit={async (e) => {
            e.preventDefault();
            let headers: Record<string, string> | undefined;
            if (headersJson.trim()) {
                try { headers = JSON.parse(headersJson); } catch { return; }
            }
            await onSubmit({ name: name.trim(), channel, kind, webhook_url: webhookUrl.trim(), headers, signature_version: sigVer });
        }} className="space-y-4">
            <div className="space-y-1.5">
                <Label className="text-xs">Provider Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="custom-provider" required />
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Channel</Label>
                <Select value={channel} onValueChange={setChannel}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                        {CHANNELS.map(c => <SelectItem key={c} value={c}>{c}</SelectItem>)}
                    </SelectContent>
                </Select>
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Webhook URL</Label>
                <Input type="url" value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} placeholder="https://..." required />
                {webhookUrl.trim() && kind !== 'generic' && (
                    <Badge variant="secondary" className="text-xs">Detected: {kind}</Badge>
                )}
            </div>
            <div className="space-y-1.5">
                <Label className="text-xs">Custom Headers (JSON, optional)</Label>
                <Textarea className="h-[80px] font-mono text-xs" value={headersJson} onChange={e => setHeadersJson(e.target.value)} placeholder='{"Authorization": "Bearer ..."}' />
            </div>
            <SigVersionSelect value={sigVer} onChange={setSigVer} />
            <SubmitButton loading={loading} />
        </form>
    );
};

// ── Shared sub-components ──

const SigVersionSelect: React.FC<{ value: ProviderSignatureVersion; onChange: (v: ProviderSignatureVersion) => void }> = ({ value, onChange }) => (
    <div className="space-y-1.5">
        <Label className="text-xs">Signature Version</Label>
        <Select value={value} onValueChange={v => onChange(v as ProviderSignatureVersion)}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
                <SelectItem value="v1">v1 — HMAC-SHA256 over body (legacy)</SelectItem>
                <SelectItem value="v2">v2 — HMAC-SHA256 over timestamp + body (replay-safe)</SelectItem>
            </SelectContent>
        </Select>
    </div>
);

const SubmitButton: React.FC<{ loading: boolean; disabled?: boolean }> = ({ loading, disabled }) => (
    <Button type="submit" className="w-full" disabled={loading || disabled}>
        {loading && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
        Register
    </Button>
);

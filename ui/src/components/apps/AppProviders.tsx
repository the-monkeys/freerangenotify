import React, { useState, useCallback, useMemo } from 'react';
import { providersAPI } from '../../services/api';
import type { CustomProvider, ProviderKind } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { extractErrorMessage } from '../../lib/utils';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { Badge } from '../ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog';
import { SlidePanel } from '../ui/slide-panel';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import SkeletonTable from '../SkeletonTable';
import ProviderTypePicker from './providers/ProviderTypePicker';
import {
    ProviderFormDiscord,
    ProviderFormSlack,
    ProviderFormTeams,
    ProviderFormGeneric,
    ProviderFormCustom,
    type ProviderFormData,
} from './providers/ProviderForms';
import { toast } from 'sonner';
import { Loader2, Plug, Plus, Trash2, Copy, Check, RotateCcw, Send, ArrowLeft } from 'lucide-react';

interface AppProvidersProps {
    appId: string;
}



// inferKind mirrors the backend's custom_provider_kind.go logic so the UI
// can show an immediate "Detected: Discord/Slack/Teams" badge.
const inferKind = (rawURL: string): ProviderKind => {
    try {
        const u = new URL(rawURL);
        const host = u.host.toLowerCase();
        const path = u.pathname.toLowerCase();
        if ((host.includes('discord.com') || host.includes('discordapp.com')) && path.startsWith('/api/webhooks')) {
            return 'discord';
        }
        if (host.includes('hooks.slack.com')) return 'slack';
        if (host.endsWith('webhook.office.com')) return 'teams';
        if (host.includes('logic.azure.com') && path.includes('/workflows/')) return 'teams';
        return 'generic';
    } catch {
        return 'generic';
    }
};

const KIND_LABEL: Record<ProviderKind, string> = {
    generic: 'Generic',
    discord: 'Discord',
    slack: 'Slack',
    teams: 'Microsoft Teams',
};

// Filter chip options for the table
const FILTER_KINDS: { value: string; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'generic', label: 'Webhook' },
    { value: 'discord', label: 'Discord' },
    { value: 'slack', label: 'Slack' },
    { value: 'teams', label: 'Teams' },
];

const AppProviders: React.FC<AppProvidersProps> = ({ appId }) => {
    const [registerOpen, setRegisterOpen] = useState(false);
    const [selectedType, setSelectedType] = useState<ProviderKind | 'custom' | null>(null);
    const [registering, setRegistering] = useState(false);
    const [removeConfirm, setRemoveConfirm] = useState<CustomProvider | null>(null);
    const [rotateConfirm, setRotateConfirm] = useState<CustomProvider | null>(null);
    const [removeLoading, setRemoveLoading] = useState(false);
    const [testingProviderId, setTestingProviderId] = useState<string | null>(null);
    const [rotatingProviderId, setRotatingProviderId] = useState<string | null>(null);
    const [signingKeyDialog, setSigningKeyDialog] = useState<{ key: string; name: string } | null>(null);
    const [copied, setCopied] = useState(false);
    const [kindFilter, setKindFilter] = useState('all');

    const fetcher = useCallback(() => providersAPI.list(appId), [appId]);
    const { data: providers, loading, refetch } = useApiQuery<CustomProvider[]>(fetcher, [appId], { cacheKey: `app-providers-${appId}` });

    const filteredProviders = useMemo(() => {
        if (!providers || kindFilter === 'all') return providers;
        return providers.filter(p => (p.kind || inferKind(p.webhook_url)) === kindFilter);
    }, [providers, kindFilter]);

    const resetPanel = () => {
        setSelectedType(null);
    };

    const handleRegister = async (data: ProviderFormData) => {
        setRegistering(true);
        try {
            const result = await providersAPI.register(appId, {
                name: data.name,
                channel: data.channel,
                kind: data.kind,
                webhook_url: data.webhook_url,
                headers: data.headers,
                signature_version: data.signature_version,
            });
            toast.success(`Provider "${data.name}" registered`);
            setRegisterOpen(false);
            resetPanel();
            refetch();
            if (result?.signing_key) {
                setSigningKeyDialog({ key: result.signing_key, name: data.name });
            }
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to register provider'));
        } finally {
            setRegistering(false);
        }
    };

    const handleRemove = async () => {
        if (!removeConfirm) return;
        setRemoveLoading(true);
        try {
            await providersAPI.remove(appId, removeConfirm.provider_id);
            toast.success(`Removed provider "${removeConfirm.name}"`);
            setRemoveConfirm(null);
            refetch();
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to remove provider'));
        } finally {
            setRemoveLoading(false);
        }
    };

    const handleSendTest = async (provider: CustomProvider) => {
        setTestingProviderId(provider.provider_id);
        try {
            const result = await providersAPI.test(appId, provider.provider_id);
            toast.success(`Test sent to "${provider.name}"${result?.delivery_time_ms ? ` (${result.delivery_time_ms}ms)` : ''}`);
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to send test notification'));
        } finally {
            setTestingProviderId(null);
        }
    };

    const handleRotateKey = async (provider: CustomProvider) => {
        setRotatingProviderId(provider.provider_id);
        try {
            const result = await providersAPI.rotate(appId, provider.provider_id);
            if (result?.signing_key) {
                setSigningKeyDialog({ key: result.signing_key, name: provider.name });
            }
            toast.success(`Signing key rotated for "${provider.name}"`);
            refetch();
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to rotate signing key'));
        } finally {
            setRotatingProviderId(null);
            setRotateConfirm(null);
        }
    };

    const copyToClipboard = async (text: string) => {
        try {
            await navigator.clipboard.writeText(text);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        } catch {
            toast.error('Failed to copy signing key to clipboard');
        }
    };

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle className="flex items-center gap-2">
                        <Plug className="h-5 w-5" />
                        Custom Providers
                    </CardTitle>
                    <Button size="sm" onClick={() => setRegisterOpen(true)}>
                        <Plus className="h-4 w-4 mr-2" />
                        Register Provider
                    </Button>
                </div>
                {/* Filter chips */}
                {providers && providers.length > 0 && (
                    <div className="flex gap-1.5 mt-3 flex-wrap">
                        {FILTER_KINDS.map(f => (
                            <Button
                                key={f.value}
                                variant={kindFilter === f.value ? 'default' : 'outline'}
                                size="sm"
                                className="h-7 text-xs"
                                onClick={() => setKindFilter(f.value)}
                            >
                                {f.label}
                            </Button>
                        ))}
                    </div>
                )}
            </CardHeader>
            <CardContent>
                {loading ? (
                    <SkeletonTable columns={5} />
                ) : !filteredProviders || filteredProviders.length === 0 ? (
                    <EmptyState
                        title="No providers"
                        description="No providers yet. Register Discord, Slack, Microsoft Teams, or a generic webhook to start delivering."
                        action={{ label: 'Register Provider', onClick: () => setRegisterOpen(true) }}
                    />
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>Type</TableHead>
                                <TableHead>Channel</TableHead>
                                <TableHead>Webhook URL</TableHead>
                                <TableHead>Sig</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead className="w-[220px]">Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {filteredProviders.map(p => {
                                const pKind: ProviderKind = p.kind || inferKind(p.webhook_url);
                                return (
                                    <TableRow key={p.provider_id}>
                                        <TableCell className="font-medium">{p.name}</TableCell>
                                        <TableCell>
                                            <Badge variant="secondary" className="text-xs">{KIND_LABEL[pKind]}</Badge>
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant="outline" className="text-xs uppercase">{p.channel}</Badge>
                                        </TableCell>
                                        <TableCell className="text-sm text-muted-foreground max-w-[250px] truncate">
                                            {p.webhook_url}
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant="outline" className="text-xs">{(p.signature_version || 'v1').toUpperCase()}</Badge>
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant={p.active ? 'default' : 'secondary'} className="text-xs">
                                                {p.active ? 'Active' : 'Inactive'}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex items-center gap-1">
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => handleSendTest(p)}
                                                    disabled={testingProviderId === p.provider_id || !p.active}
                                                    aria-label="Send provider test"
                                                    title={p.active ? 'Send provider test notification' : 'Activate provider before sending a test'}
                                                >
                                                    {testingProviderId === p.provider_id ? (
                                                        <Loader2 className="h-4 w-4 animate-spin" />
                                                    ) : (
                                                        <Send className="h-4 w-4" />
                                                    )}
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => setRotateConfirm(p)}
                                                    disabled={rotatingProviderId === p.provider_id}
                                                    aria-label="Rotate signing key"
                                                    title="Rotate signing key"
                                                >
                                                    {rotatingProviderId === p.provider_id ? (
                                                        <Loader2 className="h-4 w-4 animate-spin" />
                                                    ) : (
                                                        <RotateCcw className="h-4 w-4" />
                                                    )}
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="text-destructive hover:text-destructive"
                                                    onClick={() => setRemoveConfirm(p)}
                                                    aria-label="Remove provider"
                                                >
                                                    <Trash2 className="h-4 w-4" />
                                                </Button>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                );
                            })}
                        </TableBody>
                    </Table>
                )}
            </CardContent>

            {/* Register Provider Panel — two-step: type picker then typed form */}
            <SlidePanel
                open={registerOpen}
                onClose={() => { setRegisterOpen(false); resetPanel(); }}
                title={selectedType ? `Register ${selectedType === 'custom' ? 'Custom' : KIND_LABEL[selectedType as ProviderKind] ?? selectedType} Provider` : 'Register Provider'}
            >
                <div className="p-1">
                    {!selectedType ? (
                        <ProviderTypePicker onSelect={setSelectedType} />
                    ) : (
                        <div className="space-y-3">
                            <Button variant="ghost" size="sm" onClick={() => setSelectedType(null)}>
                                <ArrowLeft className="h-4 w-4 mr-1" /> Back to type selection
                            </Button>
                            {selectedType === 'discord' && <ProviderFormDiscord kind="discord" onSubmit={handleRegister} loading={registering} />}
                            {selectedType === 'slack' && <ProviderFormSlack kind="slack" onSubmit={handleRegister} loading={registering} />}
                            {selectedType === 'teams' && <ProviderFormTeams kind="teams" onSubmit={handleRegister} loading={registering} />}
                            {selectedType === 'generic' && <ProviderFormGeneric kind="generic" onSubmit={handleRegister} loading={registering} />}
                            {selectedType === 'custom' && <ProviderFormCustom kind="generic" onSubmit={handleRegister} loading={registering} />}
                        </div>
                    )}
                </div>
            </SlidePanel>

            {/* Signing Key Dialog */}
            <Dialog open={!!signingKeyDialog} onOpenChange={open => { if (!open) setSigningKeyDialog(null); }}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>Provider Signing Key</DialogTitle>
                    </DialogHeader>
                    <div className="space-y-3">
                        <p className="text-sm text-muted-foreground">
                            Save this signing key for <strong>{signingKeyDialog?.name}</strong>. It will not be shown again.
                        </p>
                        <div className="flex items-center gap-2">
                            <code className="flex-1 bg-muted px-3 py-2 rounded border border-border text-xs font-mono break-all select-all">
                                {signingKeyDialog?.key}
                            </code>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => {
                                    if (signingKeyDialog) void copyToClipboard(signingKeyDialog.key);
                                }}
                                aria-label="Copy signing key"
                            >
                                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                            </Button>
                        </div>
                    </div>
                </DialogContent>
            </Dialog>

            {/* Remove Confirmation */}
            <ConfirmDialog
                open={!!removeConfirm}
                onOpenChange={open => { if (!open) setRemoveConfirm(null); }}
                title="Remove Provider"
                description={removeConfirm ? `Remove "${removeConfirm.name}"? Notifications using this provider will fail.` : ''}
                confirmLabel="Remove"
                variant="destructive"
                loading={removeLoading}
                onConfirm={handleRemove}
            />
            <ConfirmDialog
                open={!!rotateConfirm}
                onOpenChange={open => { if (!open) setRotateConfirm(null); }}
                title="Rotate Signing Key"
                description={rotateConfirm ? `Rotate signing key for "${rotateConfirm.name}"? Existing receiver verification with the old key will fail after this.` : ''}
                confirmLabel="Rotate Key"
                loading={rotateConfirm ? rotatingProviderId === rotateConfirm.provider_id : false}
                onConfirm={() => rotateConfirm && handleRotateKey(rotateConfirm)}
            />
        </Card>
    );
};

export default AppProviders;

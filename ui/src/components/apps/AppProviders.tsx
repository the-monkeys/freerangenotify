import React, { useState, useCallback } from 'react';
import { providersAPI } from '../../services/api';
import type { CustomProvider } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Textarea } from '../ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Badge } from '../ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog';
import { SlidePanel } from '../ui/slide-panel';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import SkeletonTable from '../SkeletonTable';
import { toast } from 'sonner';
import { Loader2, Plug, Plus, Trash2, Copy, Check } from 'lucide-react';

interface AppProvidersProps {
    appId: string;
}

const CHANNEL_OPTIONS = [
    { value: 'webhook', label: 'Webhook' },
    { value: 'email', label: 'Email' },
    { value: 'push', label: 'Push' },
    { value: 'sms', label: 'SMS' },
    { value: 'sse', label: 'SSE' },
];

const AppProviders: React.FC<AppProvidersProps> = ({ appId }) => {
    const [registerOpen, setRegisterOpen] = useState(false);
    const [name, setName] = useState('');
    const [channel, setChannel] = useState('webhook');
    const [webhookUrl, setWebhookUrl] = useState('');
    const [headersJson, setHeadersJson] = useState('');
    const [registering, setRegistering] = useState(false);
    const [removeConfirm, setRemoveConfirm] = useState<CustomProvider | null>(null);
    const [removeLoading, setRemoveLoading] = useState(false);
    const [signingKeyDialog, setSigningKeyDialog] = useState<{ key: string; name: string } | null>(null);
    const [copied, setCopied] = useState(false);

    const fetcher = useCallback(() => providersAPI.list(appId), [appId]);
    const { data: providers, loading, refetch } = useApiQuery<CustomProvider[]>(fetcher, [appId]);

    const resetForm = () => {
        setName('');
        setChannel('webhook');
        setWebhookUrl('');
        setHeadersJson('');
    };

    const handleRegister = async (e: React.FormEvent) => {
        e.preventDefault();
        let headers: Record<string, string> | undefined;
        if (headersJson.trim()) {
            try {
                headers = JSON.parse(headersJson);
            } catch {
                toast.error('Invalid JSON in headers field');
                return;
            }
        }

        setRegistering(true);
        try {
            const result = await providersAPI.register(appId, {
                name: name.trim(),
                channel,
                webhook_url: webhookUrl.trim(),
                headers,
            });
            toast.success(`Provider "${name}" registered`);
            setRegisterOpen(false);
            resetForm();
            refetch();
            // Show signing key if returned
            if (result?.signing_key) {
                setSigningKeyDialog({ key: result.signing_key, name: name.trim() });
            }
        } catch (err: any) {
            toast.error(err?.response?.data?.error || 'Failed to register provider');
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
            toast.error(err?.response?.data?.error || 'Failed to remove provider');
        } finally {
            setRemoveLoading(false);
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
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
            </CardHeader>
            <CardContent>
                {loading ? (
                    <SkeletonTable columns={5} />
                ) : !providers || providers.length === 0 ? (
                    <EmptyState
                        title="No custom providers"
                        description="Register custom delivery providers to extend notification channels."
                        action={{ label: 'Register Provider', onClick: () => setRegisterOpen(true) }}
                    />
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>Channel</TableHead>
                                <TableHead>Webhook URL</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead className="w-[80px]">Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {providers.map(p => (
                                <TableRow key={p.provider_id}>
                                    <TableCell className="font-medium">{p.name}</TableCell>
                                    <TableCell>
                                        <Badge variant="outline" className="text-xs uppercase">{p.channel}</Badge>
                                    </TableCell>
                                    <TableCell className="text-sm text-muted-foreground max-w-[250px] truncate">
                                        {p.webhook_url}
                                    </TableCell>
                                    <TableCell>
                                        <Badge variant={p.active ? 'default' : 'secondary'} className="text-xs">
                                            {p.active ? 'Active' : 'Inactive'}
                                        </Badge>
                                    </TableCell>
                                    <TableCell>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            className="text-destructive hover:text-destructive"
                                            onClick={() => setRemoveConfirm(p)}
                                            aria-label="Remove provider"
                                        >
                                            <Trash2 className="h-4 w-4" />
                                        </Button>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </CardContent>

            {/* Register Provider Panel */}
            <SlidePanel
                open={registerOpen}
                onClose={() => { setRegisterOpen(false); resetForm(); }}
                title="Register Custom Provider"
            >
                <form onSubmit={handleRegister} className="space-y-4 p-1">
                    <div className="space-y-1.5">
                        <Label className="text-xs">Provider Name</Label>
                        <Input
                            value={name}
                            onChange={e => setName(e.target.value)}
                            placeholder="my-slack-webhook"
                            required
                        />
                    </div>
                    <div className="space-y-1.5">
                        <Label className="text-xs">Channel</Label>
                        <Select value={channel} onValueChange={setChannel}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {CHANNEL_OPTIONS.map(c => (
                                    <SelectItem key={c.value} value={c.value}>{c.label}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-1.5">
                        <Label className="text-xs">Webhook URL</Label>
                        <Input
                            type="url"
                            value={webhookUrl}
                            onChange={e => setWebhookUrl(e.target.value)}
                            placeholder="https://hooks.example.com/..."
                            required
                        />
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
                    <Button type="submit" className="w-full" disabled={registering}>
                        {registering ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Register
                    </Button>
                </form>
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
                                onClick={() => signingKeyDialog && copyToClipboard(signingKeyDialog.key)}
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
        </Card>
    );
};

export default AppProviders;

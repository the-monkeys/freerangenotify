import React, { useState, useCallback } from 'react';
import { environmentsAPI } from '../../services/api';
import type { Environment, EnvironmentName } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { Label } from '../ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Badge } from '../ui/badge';
import { Checkbox } from '../ui/checkbox';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog';
import { SlidePanel } from '../ui/slide-panel';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import SkeletonTable from '../SkeletonTable';
import { toast } from 'sonner';
import { Loader2, Globe, Plus, Key, Copy, Check, ArrowRight, Trash2, Eye, EyeOff } from 'lucide-react';

interface AppEnvironmentsProps {
    appId: string;
    currentApiKey: string;
    onApiKeyChange: (apiKey: string, envName: string) => void;
}

const ENV_NAMES: EnvironmentName[] = ['development', 'staging', 'production'];

const ENV_COLORS: Record<string, string> = {
    development: 'bg-blue-100 text-blue-800 border-blue-300',
    staging: 'bg-amber-100 text-amber-800 border-amber-300',
    production: 'bg-green-100 text-green-800 border-green-300',
};

const PROMOTE_RESOURCES = [
    { key: 'templates', label: 'Templates' },
    { key: 'users', label: 'Users' },
    { key: 'providers', label: 'Providers' },
];

const AppEnvironments: React.FC<AppEnvironmentsProps> = ({ appId, currentApiKey, onApiKeyChange }) => {
    const [createOpen, setCreateOpen] = useState(false);
    const [newEnvName, setNewEnvName] = useState<EnvironmentName>('development');
    const [creating, setCreating] = useState(false);
    const [apiKeyDialog, setApiKeyDialog] = useState<{ key: string; name: string } | null>(null);
    const [copied, setCopied] = useState(false);
    const [removeConfirm, setRemoveConfirm] = useState<Environment | null>(null);
    const [removeLoading, setRemoveLoading] = useState(false);
    const [promoteOpen, setPromoteOpen] = useState(false);
    const [promoteSource, setPromoteSource] = useState('');
    const [promoteTarget, setPromoteTarget] = useState('');
    const [promoteResources, setPromoteResources] = useState<string[]>(['templates']);
    const [promoting, setPromoting] = useState(false);
    const [revealedKeys, setRevealedKeys] = useState<Record<string, boolean>>({});

    const fetcher = useCallback(() => environmentsAPI.list(appId), [appId]);
    const { data: environments, loading, refetch } = useApiQuery<Environment[]>(fetcher, [appId]);

    const existingNames = (environments || []).map(e => e.name);
    const availableNames = ENV_NAMES.filter(n => !existingNames.includes(n));

    const handleCreate = async () => {
        setCreating(true);
        try {
            const result = await environmentsAPI.create(appId, { name: newEnvName });
            toast.success(`Environment "${newEnvName}" created`);
            setCreateOpen(false);
            refetch();
            if (result?.api_key) {
                setApiKeyDialog({ key: result.api_key, name: newEnvName });
            }
        } catch (err: any) {
            toast.error(err?.response?.data?.error || 'Failed to create environment');
        } finally {
            setCreating(false);
        }
    };

    const handleRemove = async () => {
        if (!removeConfirm) return;
        setRemoveLoading(true);
        try {
            await environmentsAPI.delete(appId, removeConfirm.id);
            toast.success(`Deleted environment "${removeConfirm.name}"`);
            setRemoveConfirm(null);
            refetch();
        } catch (err: any) {
            toast.error(err?.response?.data?.error || 'Failed to delete environment');
        } finally {
            setRemoveLoading(false);
        }
    };

    const handlePromote = async () => {
        if (!promoteSource || !promoteTarget || promoteResources.length === 0) {
            toast.error('Select source, target, and at least one resource');
            return;
        }
        setPromoting(true);
        try {
            await environmentsAPI.promote(appId, {
                source_env_id: promoteSource,
                target_env_id: promoteTarget,
                resources: promoteResources,
            });
            toast.success('Resources promoted successfully');
            setPromoteOpen(false);
            setPromoteSource('');
            setPromoteTarget('');
            setPromoteResources(['templates']);
            refetch();
        } catch (err: any) {
            toast.error(err?.response?.data?.error || 'Promotion failed');
        } finally {
            setPromoting(false);
        }
    };

    const toggleResource = (key: string) => {
        setPromoteResources(prev =>
            prev.includes(key) ? prev.filter(r => r !== key) : [...prev, key]
        );
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    const maskKey = (key: string) => {
        if (!key || key.length < 12) return '••••••••';
        return key.slice(0, 8) + '••••••••' + key.slice(-4);
    };

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle className="flex items-center gap-2">
                        <Globe className="h-5 w-5" />
                        Environments
                    </CardTitle>
                    <div className="flex gap-2">
                        {(environments || []).length >= 2 && (
                            <Button variant="outline" size="sm" onClick={() => setPromoteOpen(true)}>
                                <ArrowRight className="h-4 w-4 mr-2" />
                                Promote
                            </Button>
                        )}
                        {availableNames.length > 0 && (
                            <Button size="sm" onClick={() => setCreateOpen(true)}>
                                <Plus className="h-4 w-4 mr-2" />
                                Add Environment
                            </Button>
                        )}
                    </div>
                </div>
            </CardHeader>
            <CardContent>
                {loading ? (
                    <SkeletonTable columns={4} />
                ) : !environments || environments.length === 0 ? (
                    <EmptyState
                        title="No environments"
                        description="Create environments to separate development, staging, and production configurations."
                        action={availableNames.length > 0 ? { label: 'Add Environment', onClick: () => setCreateOpen(true) } : undefined}
                    />
                ) : (
                    <div className="space-y-3">
                        {environments.map(env => (
                            <div
                                key={env.id}
                                className={`border rounded-lg p-4 transition-colors ${env.api_key === currentApiKey ? 'border-foreground bg-muted/50' : 'border-border'}`}
                            >
                                <div className="flex items-center justify-between mb-2">
                                    <div className="flex items-center gap-2">
                                        <Badge variant="outline" className={ENV_COLORS[env.name] || ''}>
                                            {env.name}
                                        </Badge>
                                        {env.is_default && (
                                            <Badge variant="secondary" className="text-xs">default</Badge>
                                        )}
                                        {env.api_key === currentApiKey && (
                                            <Badge className="text-xs bg-foreground text-background">active</Badge>
                                        )}
                                    </div>
                                    <div className="flex items-center gap-1">
                                        {env.api_key !== currentApiKey && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                className="text-xs"
                                                onClick={() => onApiKeyChange(env.api_key, env.name)}
                                            >
                                                Switch to this
                                            </Button>
                                        )}
                                        {!env.is_default && (
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="text-destructive hover:text-destructive"
                                                onClick={() => setRemoveConfirm(env)}
                                                aria-label="Delete environment"
                                            >
                                                <Trash2 className="h-4 w-4" />
                                            </Button>
                                        )}
                                    </div>
                                </div>
                                <div className="flex items-center gap-2 text-xs">
                                    <Key className="h-3.5 w-3.5 text-muted-foreground" />
                                    <code className="font-mono text-muted-foreground select-all">
                                        {revealedKeys[env.id] ? env.api_key : maskKey(env.api_key)}
                                    </code>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="h-6 w-6 p-0"
                                        onClick={() => setRevealedKeys(prev => ({ ...prev, [env.id]: !prev[env.id] }))}
                                        aria-label={revealedKeys[env.id] ? 'Hide API key' : 'Reveal API key'}
                                    >
                                        {revealedKeys[env.id] ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="h-6 w-6 p-0"
                                        onClick={() => copyToClipboard(env.api_key)}
                                        aria-label="Copy API key"
                                    >
                                        <Copy className="h-3.5 w-3.5" />
                                    </Button>
                                </div>
                                <p className="text-xs text-muted-foreground mt-1">
                                    Created {env.created_at ? new Date(env.created_at).toLocaleDateString() : '—'}
                                </p>
                            </div>
                        ))}
                    </div>
                )}
            </CardContent>

            {/* Create Environment Panel */}
            <SlidePanel
                open={createOpen}
                onClose={() => setCreateOpen(false)}
                title="Add Environment"
            >
                <div className="space-y-4 p-1">
                    <div className="space-y-1.5">
                        <Label className="text-xs">Environment Name</Label>
                        <Select value={newEnvName} onValueChange={v => setNewEnvName(v as EnvironmentName)}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {availableNames.map(n => (
                                    <SelectItem key={n} value={n}>{n}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <Button onClick={handleCreate} className="w-full" disabled={creating}>
                        {creating ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Create Environment
                    </Button>
                </div>
            </SlidePanel>

            {/* Promote Panel */}
            <SlidePanel
                open={promoteOpen}
                onClose={() => setPromoteOpen(false)}
                title="Promote Resources"
            >
                <div className="space-y-4 p-1">
                    <p className="text-sm text-muted-foreground">
                        Copy resources from one environment to another.
                    </p>
                    <div className="space-y-1.5">
                        <Label className="text-xs">Source</Label>
                        <Select value={promoteSource} onValueChange={setPromoteSource}>
                            <SelectTrigger>
                                <SelectValue placeholder="Select source…" />
                            </SelectTrigger>
                            <SelectContent>
                                {(environments || []).map(e => (
                                    <SelectItem key={e.id} value={e.id}>{e.name}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-1.5">
                        <Label className="text-xs">Target</Label>
                        <Select value={promoteTarget} onValueChange={setPromoteTarget}>
                            <SelectTrigger>
                                <SelectValue placeholder="Select target…" />
                            </SelectTrigger>
                            <SelectContent>
                                {(environments || []).filter(e => e.id !== promoteSource).map(e => (
                                    <SelectItem key={e.id} value={e.id}>{e.name}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-2">
                        <Label className="text-xs">Resources to Promote</Label>
                        {PROMOTE_RESOURCES.map(r => (
                            <div key={r.key} className="flex items-center gap-2">
                                <Checkbox
                                    checked={promoteResources.includes(r.key)}
                                    onCheckedChange={() => toggleResource(r.key)}
                                />
                                <span className="text-sm">{r.label}</span>
                            </div>
                        ))}
                    </div>
                    <Button
                        onClick={handlePromote}
                        className="w-full"
                        disabled={promoting || !promoteSource || !promoteTarget || promoteResources.length === 0}
                    >
                        {promoting ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Promote
                    </Button>
                </div>
            </SlidePanel>

            {/* API Key Created Dialog */}
            <Dialog open={!!apiKeyDialog} onOpenChange={open => { if (!open) setApiKeyDialog(null); }}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>Environment API Key</DialogTitle>
                    </DialogHeader>
                    <div className="space-y-3">
                        <p className="text-sm text-muted-foreground">
                            API key for <strong>{apiKeyDialog?.name}</strong>. Save it securely — it will not be shown again at full length.
                        </p>
                        <div className="flex items-center gap-2">
                            <code className="flex-1 bg-muted px-3 py-2 rounded border border-border text-xs font-mono break-all select-all">
                                {apiKeyDialog?.key}
                            </code>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => apiKeyDialog && copyToClipboard(apiKeyDialog.key)}
                                aria-label="Copy API key"
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
                title="Delete Environment"
                description={removeConfirm ? `Delete the "${removeConfirm.name}" environment? All data within it will be lost.` : ''}
                confirmLabel="Delete"
                variant="destructive"
                loading={removeLoading}
                onConfirm={handleRemove}
            />
        </Card>
    );
};

export default AppEnvironments;

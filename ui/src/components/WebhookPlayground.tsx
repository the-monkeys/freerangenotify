import React, { useState, useRef, useCallback, useMemo } from 'react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Badge } from './ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';
import { toast } from 'sonner';
import { adminAPI } from '../services/api';
import { Search, Download, ShieldCheck, ShieldX, Copy } from 'lucide-react';
import ChannelPreview from './channels/ChannelPreview';
import type { RichContent } from './channels/ChannelPreview';

interface EnrichedPayload {
    headers?: Record<string, string | string[]>;
    body?: any;
    received_at?: string;
    payload_version?: string;
    rich_fields_detected?: string[];
    signature_valid?: boolean;
}

export default function WebhookPlayground() {
    const [playgroundURL, setPlaygroundURL] = useState('');
    const [playgroundID, setPlaygroundID] = useState('');
    const [payloads, setPayloads] = useState<EnrichedPayload[]>([]);
    const [creating, setCreating] = useState(false);
    const [signingKey, setSigningKey] = useState('');
    const [searchQuery, setSearchQuery] = useState('');
    const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

    const createPlayground = async () => {
        setCreating(true);
        try {
            const res = await adminAPI.createPlayground();
            setPlaygroundURL(res.url);
            setPlaygroundID(res.id);
            setPayloads([]);
            toast.success('Webhook playground created (expires in 30 min)');
            startPolling(res.id);
        } catch {
            toast.error('Failed to create playground');
        } finally {
            setCreating(false);
        }
    };

    const startPolling = useCallback((id: string) => {
        if (pollRef.current) clearInterval(pollRef.current);
        pollRef.current = setInterval(async () => {
            try {
                const keyParam = signingKey ? `?signing_key=${encodeURIComponent(signingKey)}` : '';
                const res = await adminAPI.getPlaygroundPayloads(id + keyParam);
                setPayloads(res.payloads || []);
            } catch {
                // playground may have expired
            }
        }, 2000);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [signingKey]);

    const stopPolling = () => {
        if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
        }
    };

    const handleDestroy = () => {
        stopPolling();
        setPlaygroundURL('');
        setPlaygroundID('');
        setPayloads([]);
        setSigningKey('');
        setSearchQuery('');
    };

    const copyURL = () => {
        navigator.clipboard.writeText(playgroundURL);
        toast.success('URL copied to clipboard');
    };

    const exportPayloads = () => {
        const json = JSON.stringify(payloads, null, 2);
        const blob = new Blob([json], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `playground-${playgroundID}-${Date.now()}.json`;
        a.click();
        URL.revokeObjectURL(url);
        toast.success('Payloads exported');
    };

    // Filter payloads by search query (searches body as string)
    const filteredPayloads = useMemo(() => {
        if (!searchQuery.trim()) return payloads;
        const q = searchQuery.toLowerCase();
        return payloads.filter(p => {
            const bodyStr = JSON.stringify(p.body || {}).toLowerCase();
            const headerStr = JSON.stringify(p.headers || {}).toLowerCase();
            return bodyStr.includes(q) || headerStr.includes(q);
        });
    }, [payloads, searchQuery]);

    // Re-trigger polling when signing key changes to get enriched responses
    React.useEffect(() => {
        if (playgroundID) {
            startPolling(playgroundID);
        }
        return () => stopPolling();
    }, [playgroundID, startPolling]);

    return (
        <section className="space-y-4">
            <h3 className="flex items-center gap-2 text-lg font-semibold">
                Webhook Playground
                <Badge variant="outline" className="text-xs font-normal">Beta</Badge>
            </h3>

            <div className="space-y-4 rounded-lg border border-border/70 bg-white/70 p-4 dark:bg-zinc-900/40">
                {!playgroundURL ? (
                    <div className="text-center py-6 space-y-3">
                        <p className="text-sm text-muted-foreground">
                            Create a temporary webhook URL to test delivery without external tools. URLs expire after 30 minutes.
                        </p>
                        <Button onClick={createPlayground} disabled={creating}>
                            {creating ? 'Creating...' : 'Create Test Webhook'}
                        </Button>
                    </div>
                ) : (
                    <div className="space-y-4">
                        <div className="space-y-2">
                            <Label>Your test webhook URL (expires in 30 min)</Label>
                            <div className="flex gap-2">
                                <Input value={playgroundURL} readOnly className="bg-muted/40 font-mono text-sm" />
                                <Button variant="outline" size="sm" onClick={copyURL}><Copy className="h-4 w-4" /></Button>
                            </div>
                        </div>

                        {/* Signature verification input */}
                        <div className="space-y-1.5">
                            <Label className="text-xs">Signing Key (optional — verifies signatures)</Label>
                            <Input
                                type="password"
                                value={signingKey}
                                onChange={e => setSigningKey(e.target.value)}
                                placeholder="Paste your provider signing key to verify signatures"
                                className="font-mono text-xs"
                            />
                        </div>

                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                                <span className="relative flex h-2 w-2">
                                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
                                    <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
                                </span>
                                <span className="text-xs font-medium text-green-600 dark:text-green-400">Listening for payloads</span>
                            </div>
                            <div className="flex items-center gap-2">
                                {payloads.length > 0 && (
                                    <Button variant="outline" size="sm" onClick={exportPayloads}>
                                        <Download className="h-3.5 w-3.5 mr-1" /> Export JSON
                                    </Button>
                                )}
                                <Button variant="ghost" size="sm" className="text-red-500" onClick={handleDestroy}>
                                    Stop & Close
                                </Button>
                            </div>
                        </div>

                        {/* Search */}
                        {payloads.length > 0 && (
                            <div className="relative">
                                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                                <Input
                                    value={searchQuery}
                                    onChange={e => setSearchQuery(e.target.value)}
                                    placeholder="Search payloads by header or body..."
                                    className="pl-9 text-sm"
                                />
                            </div>
                        )}

                        <div>
                            <h4 className="text-sm font-semibold mb-2">
                                Received Payloads
                                <Badge variant="outline" className="ml-2 text-xs">{filteredPayloads.length}</Badge>
                            </h4>
                            {filteredPayloads.length === 0 ? (
                                <p className="py-4 text-center text-sm italic text-muted-foreground">
                                    {payloads.length === 0 ? 'No payloads received yet. Send a webhook to the URL above.' : 'No payloads match the search.'}
                                </p>
                            ) : (
                                <div className="space-y-3 max-h-[600px] overflow-y-auto">
                                    {filteredPayloads.map((p, i) => (
                                        <PayloadCard key={i} index={i} payload={p} signingKey={signingKey} />
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                )}
            </div>
        </section>
    );
}

// ── Payload Card with Raw / Pretty / Rich tabs ──

interface PayloadCardProps {
    index: number;
    payload: EnrichedPayload;
    signingKey: string;
}

function PayloadCard({ index, payload, signingKey }: PayloadCardProps) {
    // Try to extract rich content from the body for the Rich preview tab
    const richContent = useMemo((): RichContent | null => {
        if (!payload.body || typeof payload.body !== 'object') return null;
        const body = payload.body;
        // Generic webhook: body has a content object
        const content = body.content || body;
        if (content.title || content.body) {
            return content as RichContent;
        }
        return null;
    }, [payload.body]);

    // Detect channel from body shape
    const detectedChannel = useMemo(() => {
        if (!payload.body || typeof payload.body !== 'object') return 'generic';
        if (payload.body.embeds) return 'discord';
        if (payload.body.blocks || (payload.body.text && !payload.body.notification_id)) return 'slack';
        if (payload.body.type === 'message' && payload.body.attachments) return 'teams';
        return 'generic';
    }, [payload.body]);

    const hasRichFields = payload.rich_fields_detected && payload.rich_fields_detected.length > 0;

    return (
        <div className="rounded border border-border/70 bg-muted/40 p-3 dark:bg-zinc-800/40">
            <div className="flex justify-between items-center mb-2">
                <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-xs">#{index + 1}</Badge>
                    <Badge variant="secondary" className="text-[10px]">{detectedChannel}</Badge>
                    <Badge variant="outline" className="text-[10px]">v{payload.payload_version || '1.0'}</Badge>
                    {hasRichFields && (
                        <Badge variant="outline" className="text-[10px] text-blue-600">
                            Rich: {payload.rich_fields_detected!.join(', ')}
                        </Badge>
                    )}
                    {signingKey && payload.signature_valid !== undefined && (
                        payload.signature_valid ? (
                            <Badge variant="default" className="text-[10px] bg-green-600"><ShieldCheck className="h-3 w-3 mr-0.5" />Valid</Badge>
                        ) : (
                            <Badge variant="destructive" className="text-[10px]"><ShieldX className="h-3 w-3 mr-0.5" />Invalid</Badge>
                        )
                    )}
                </div>
                <span className="text-xs text-muted-foreground">
                    {payload.received_at ? new Date(payload.received_at).toLocaleTimeString() : ''}
                </span>
            </div>

            <Tabs defaultValue="pretty" className="w-full">
                <TabsList className="h-7">
                    <TabsTrigger value="pretty" className="text-xs h-6">Pretty</TabsTrigger>
                    <TabsTrigger value="raw" className="text-xs h-6">Raw</TabsTrigger>
                    {richContent && <TabsTrigger value="rich" className="text-xs h-6">Rich</TabsTrigger>}
                    <TabsTrigger value="headers" className="text-xs h-6">Headers</TabsTrigger>
                </TabsList>
                <TabsContent value="pretty" className="mt-2">
                    <pre className="max-h-[200px] overflow-auto whitespace-pre-wrap text-xs font-mono text-foreground bg-background/50 rounded p-2">
                        {JSON.stringify(payload.body, null, 2)}
                    </pre>
                </TabsContent>
                <TabsContent value="raw" className="mt-2">
                    <pre className="max-h-[200px] overflow-auto whitespace-pre-wrap text-xs font-mono text-foreground bg-background/50 rounded p-2">
                        {JSON.stringify(payload.body)}
                    </pre>
                </TabsContent>
                {richContent && (
                    <TabsContent value="rich" className="mt-2">
                        <ChannelPreview
                            channel={detectedChannel}
                            content={richContent}
                        />
                    </TabsContent>
                )}
                <TabsContent value="headers" className="mt-2">
                    <pre className="max-h-[200px] overflow-auto whitespace-pre-wrap text-xs font-mono text-foreground bg-background/50 rounded p-2">
                        {JSON.stringify(payload.headers, null, 2)}
                    </pre>
                </TabsContent>
            </Tabs>
        </div>
    );
}

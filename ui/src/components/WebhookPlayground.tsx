import React, { useState, useRef, useCallback } from 'react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Badge } from './ui/badge';
import { toast } from 'sonner';
import { adminAPI } from '../services/api';

interface Payload {
    headers?: Record<string, string>;
    body?: unknown;
    received_at?: string;
}

export default function WebhookPlayground() {
    const [playgroundURL, setPlaygroundURL] = useState('');
    const [payloads, setPayloads] = useState<Payload[]>([]);
    const [creating, setCreating] = useState(false);
    const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

    const createPlayground = async () => {
        setCreating(true);
        try {
            const res = await adminAPI.createPlayground();
            setPlaygroundURL(res.url);
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
                const res = await adminAPI.getPlaygroundPayloads(id);
                setPayloads(res.payloads || []);
            } catch {
                // playground may have expired
            }
        }, 2000);
    }, []);

    const stopPolling = () => {
        if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
        }
    };

    const handleDestroy = () => {
        stopPolling();
        setPlaygroundURL('');
        setPayloads([]);
    };

    const copyURL = () => {
        navigator.clipboard.writeText(playgroundURL);
        toast.success('URL copied to clipboard');
    };

    // Cleanup on unmount
    React.useEffect(() => {
        return () => stopPolling();
    }, []);

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
                                <Button variant="outline" size="sm" onClick={copyURL}>Copy</Button>
                            </div>
                            <p className="text-xs text-muted-foreground">
                                Use this URL as a webhook target when sending notifications. Payloads appear below in real-time.
                            </p>
                        </div>

                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                                <span className="relative flex h-2 w-2">
                                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
                                    <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
                                </span>
                                <span className="text-xs font-medium text-green-600 dark:text-green-400">Listening for payloads</span>
                            </div>
                            <Button variant="ghost" size="sm" className="text-red-500" onClick={handleDestroy}>
                                Stop & Close
                            </Button>
                        </div>

                        <div>
                            <h4 className="text-sm font-semibold mb-2">
                                Received Payloads
                                <Badge variant="outline" className="ml-2 text-xs">{payloads.length}</Badge>
                            </h4>
                            {payloads.length === 0 ? (
                                <p className="py-4 text-center text-sm italic text-muted-foreground">
                                    No payloads received yet. Send a webhook to the URL above.
                                </p>
                            ) : (
                                <div className="space-y-2 max-h-[400px] overflow-y-auto">
                                    {payloads.map((p, i) => (
                                        <div key={i} className="rounded border border-border/70 bg-muted/40 p-3 dark:bg-zinc-800/40">
                                            <div className="flex justify-between items-center mb-2">
                                                <Badge variant="outline" className="text-xs">#{i + 1}</Badge>
                                                <span className="text-xs text-muted-foreground">{p.received_at ? new Date(p.received_at).toLocaleTimeString() : ''}</span>
                                            </div>
                                            <pre className="max-h-[200px] overflow-auto whitespace-pre-wrap text-xs font-mono text-foreground">
                                                {JSON.stringify(p.body, null, 2)}
                                            </pre>
                                        </div>
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

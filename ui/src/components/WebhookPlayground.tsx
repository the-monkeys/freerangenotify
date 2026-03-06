import React, { useState, useRef, useCallback } from 'react';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
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
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    Webhook Playground
                    <Badge variant="outline" className="text-xs font-normal">Beta</Badge>
                </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
                {!playgroundURL ? (
                    <div className="text-center py-6 space-y-3">
                        <p className="text-sm text-gray-500">
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
                                <Input value={playgroundURL} readOnly className="font-mono text-sm bg-gray-50" />
                                <Button variant="outline" size="sm" onClick={copyURL}>Copy</Button>
                            </div>
                            <p className="text-xs text-gray-500">
                                Use this URL as a webhook target when sending notifications. Payloads appear below in real-time.
                            </p>
                        </div>

                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                                <span className="relative flex h-2 w-2">
                                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
                                    <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
                                </span>
                                <span className="text-xs text-green-600 font-medium">Listening for payloads</span>
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
                                <p className="text-sm text-gray-400 italic py-4 text-center">
                                    No payloads received yet. Send a webhook to the URL above.
                                </p>
                            ) : (
                                <div className="space-y-2 max-h-[400px] overflow-y-auto">
                                    {payloads.map((p, i) => (
                                        <div key={i} className="bg-gray-50 border border-gray-200 rounded p-3">
                                            <div className="flex justify-between items-center mb-2">
                                                <Badge variant="outline" className="text-xs">#{i + 1}</Badge>
                                                <span className="text-xs text-gray-400">{p.received_at ? new Date(p.received_at).toLocaleTimeString() : ''}</span>
                                            </div>
                                            <pre className="whitespace-pre-wrap text-xs font-mono text-gray-800 overflow-auto max-h-[200px]">
                                                {JSON.stringify(p.body, null, 2)}
                                            </pre>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                )}
            </CardContent>
        </Card>
    );
}
